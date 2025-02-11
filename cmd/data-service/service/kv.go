/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/gen"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbbase "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/base"
	pbkv "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/kv"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/tools"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

// CreateKv is used to create key-value data.
func (s *Service) CreateKv(ctx context.Context, req *pbds.CreateKvReq) (*pbds.CreateResp, error) {

	kt := kit.FromGrpcContext(ctx)

	// 检测配置项是否超出服务限制
	err := s.checkKVConfigItemExceedsAppLimit(kt, req.Attachment.BizId, req.Attachment.AppId, 1, 0)
	if err != nil {
		return nil, err
	}

	// GetByKvState get kv by KvState.
	_, err = s.dao.Kv().GetByKvState(kt, req.Attachment.BizId, req.Attachment.AppId, req.Spec.Key,
		[]string{string(table.KvStateAdd), string(table.KvStateUnchange), string(table.KvStateRevise)})
	if err != nil && !errors.Is(gorm.ErrRecordNotFound, err) {
		logs.Errorf("get kv (%d) failed, err: %v, rid: %s", req.Spec.Key, err, kt.Rid)
		return nil, errf.Errorf(errf.NotFound,
			i18n.T(kt, "get kv (%d) failed, err: %v", req.Spec.Key, err))
	}
	if !errors.Is(gorm.ErrRecordNotFound, err) {
		logs.Errorf("get kv (%d) failed, err: %v, rid: %s", req.Spec.Key, err, kt.Rid)
		return nil, errf.Errorf(errf.DBOpFailed,
			i18n.T(kt, "the config item %s under this service already exists and cannot be created again", req.Spec.Key))
	}
	// get app with id.
	app, err := s.dao.App().Get(kt, req.Attachment.BizId, req.Attachment.AppId)
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed, i18n.T(kt, "get app fail, key: %s, err: %v", req.Spec.Key, err))
	}
	if !checkKVTypeMatch(table.DataType(req.Spec.KvType), app.Spec.DataType) {
		return nil, errf.Errorf(errf.InvalidRequest,
			i18n.T(kt, "kv type does not match the data type defined in the application"))
	}

	if req.Spec.KvType == string(table.KvTab) &&
		req.Spec.ManagedTableId == 0 && req.Spec.ExternalSourceId == 0 {
		return nil, errors.New(i18n.T(kt, "table data types require config tables"))
	}

	var version int

	if req.Spec.ManagedTableId == 0 && req.Spec.ExternalSourceId == 0 {
		opt := &types.UpsertKvOption{
			BizID:  req.Attachment.BizId,
			AppID:  req.Attachment.AppId,
			Key:    req.Spec.Key,
			Value:  req.Spec.Value,
			KvType: table.DataType(req.Spec.KvType),
		}

		// UpsertKv 创建｜更新kv
		version, err = s.vault.UpsertKv(kt, opt)
		if err != nil {
			logs.Errorf("create kv failed, err: %v, rid: %s", err, kt.Rid)
			return nil, errf.Errorf(errf.DBOpFailed, i18n.T(kt, "create kv failed, err: %v", err))
		}
	}

	spec, err := req.Spec.KvSpec()
	if err != nil {
		return nil, err
	}
	kv := &table.Kv{
		Spec:       spec,
		Attachment: req.Attachment.KvAttachment(),
		Revision: &table.Revision{
			Creator: kt.User,
			Reviser: kt.User,
		},
		ContentSpec: &table.ContentSpec{
			Signature: tools.SHA256(req.Spec.Value),
			Md5:       tools.MD5(req.Spec.Value),
			ByteSize:  uint64(len(req.Spec.Value)),
		},
	}
	kv.Spec.Version = uint32(version)
	kv.KvState = table.KvStateAdd
	// Create one kv instance
	id, err := s.dao.Kv().Create(kt, kv)
	if err != nil {
		logs.Errorf("create kv failed, err: %v, rid: %s", err, kt.Rid)
		return nil, errf.Errorf(errf.DBOpFailed, i18n.T(kt, "create kv failed, err: %v", err))
	}

	resp := &pbds.CreateResp{Id: id}
	return resp, nil
}

// check KV Type Match
func checkKVTypeMatch(kvType, appKvType table.DataType) bool {
	if appKvType == table.KvAny {
		return true
	}
	return kvType == appKvType
}

// UpdateKv is used to update key-value data.
func (s *Service) UpdateKv(ctx context.Context, req *pbds.UpdateKvReq) (*pbbase.EmptyResp, error) {

	kt := kit.FromGrpcContext(ctx)

	// GetByKvState get kv by KvState.
	kv, err := s.dao.Kv().GetByKvState(kt, req.Attachment.BizId, req.Attachment.AppId, req.Spec.Key,
		[]string{string(table.KvStateAdd), string(table.KvStateUnchange), string(table.KvStateRevise)})
	if err != nil {
		logs.Errorf("get kv (%d) failed, err: %v, rid: %s", req.Spec.Key, err, kt.Rid)
		return nil, errf.Errorf(errf.DBOpFailed,
			i18n.T(kt, "get kv (%d) failed, err: %v", req.Spec.Key, err))
	}

	var version int
	if req.Spec.ManagedTableId == 0 && req.Spec.ExternalSourceId == 0 {
		// UpsertKv 创建｜更新kv
		opt := &types.UpsertKvOption{
			BizID:  req.Attachment.BizId,
			AppID:  req.Attachment.AppId,
			Key:    kv.Spec.Key,
			Value:  req.Spec.Value,
			KvType: kv.Spec.KvType,
		}
		version, err = s.vault.UpsertKv(kt, opt)
		if err != nil {
			return nil, errf.Errorf(errf.DBOpFailed, i18n.T(kt, "update kv failed, err: %v", err))
		}
	}

	if kv.KvState == table.KvStateUnchange {
		kv.KvState = table.KvStateRevise
	}

	kv.Revision = &table.Revision{
		Reviser:   kt.User,
		UpdatedAt: time.Now().UTC(),
	}

	kv.Spec.Version = uint32(version)
	kv.Spec.Memo = req.Spec.Memo
	kv.ContentSpec = &table.ContentSpec{
		Signature: tools.SHA256(req.Spec.Value),
		Md5:       tools.MD5(req.Spec.Value),
		ByteSize:  uint64(len(req.Spec.Value)),
	}
	kv.Spec.SecretHidden = req.Spec.SecretHidden
	spec, err := req.Spec.KvSpec()
	if err != nil {
		return nil, err
	}
	kv.Spec.CertificateExpirationDate = spec.CertificateExpirationDate
	kv.Spec.ManagedTableID = spec.ManagedTableID
	kv.Spec.ExternalSourceID = spec.ExternalSourceID
	kv.Spec.FilterFields = spec.FilterFields
	kv.Spec.FilterCondition = spec.FilterCondition
	if e := s.dao.Kv().Update(kt, kv); e != nil {
		logs.Errorf("update kv failed, err: %v, rid: %s", e, kt.Rid)
		return nil, errf.Errorf(errf.DBOpFailed, i18n.T(kt, "update kv failed, err: %v", err))
	}

	return new(pbbase.EmptyResp), nil

}

// ListKvs is used to list key-value data.
func (s *Service) ListKvs(ctx context.Context, req *pbds.ListKvsReq) (*pbds.ListKvsResp, error) {
	// FromGrpcContext used only to obtain Kit through grpc context.
	kt := kit.FromGrpcContext(ctx)

	if len(req.Sort) == 0 {
		req.Sort = "key"
	}
	page := &types.BasePage{
		Start: req.Start,
		Limit: uint(req.Limit),
		Sort:  req.Sort,
		Order: types.Order(req.Order),
	}
	opt := &types.ListKvOption{
		BizID:     req.BizId,
		AppID:     req.AppId,
		Key:       req.Key,
		SearchKey: req.SearchKey,
		All:       req.All,
		Page:      page,
		KvType:    req.KvType,
		TopIDs:    req.TopIds,
		Status:    req.Status,
	}

	// 该方法被生成版本接口调用。移至到查询列表前面提前返回判断
	uncitedCount, err := s.dao.Kv().CountNumberUnDeleted(kt, req.BizId, opt)
	if err != nil {
		return nil, err
	}

	_, expirationNumber, err := s.dao.Kv().FindNearExpiryCertKvs(kt, req.BizId, req.AppId, 0,
		&types.BasePage{All: true})
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed,
			i18n.T(kt, "get a list of expired certificates failed, err: %v"), err)
	}

	po := &types.PageOption{
		EnableUnlimitedLimit: true,
	}
	if err = opt.Validate(po); err != nil {
		return nil, err
	}

	details, count, err := s.dao.Kv().List(kt, opt)
	if err != nil {
		logs.Errorf("list kv failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	kvs, err := s.setKvTypeAndValue(kt, details)
	if err != nil {
		return nil, err
	}

	resp := &pbds.ListKvsResp{
		Count:          uint32(count),
		Details:        kvs,
		ExclusionCount: uint32(uncitedCount),
		IsCertExpired: func() bool {
			return expirationNumber > 0
		}(),
	}

	return resp, nil
}

// set Kv Type And Value
func (s *Service) setKvTypeAndValue(kt *kit.Kit, details []*table.Kv) ([]*pbkv.Kv, error) {
	// 预分配切片，确保没有扩展
	kvs := make([]*pbkv.Kv, len(details))
	eg, _ := errgroup.WithContext(kt.RpcCtx())
	eg.SetLimit(10)
	var mux sync.Mutex

	for i, one := range details {
		one := one
		i := i
		eg.Go(func() error {

			var kvValue, name string

			var err error
			// 处理kv表格型数据
			if one.Spec.KvType == table.KvTab {
				name, err = s.getKvTableConfigPreviewName(kt, one.Attachment.BizID, one.Spec.ManagedTableID, one.Spec.ExternalSourceID)
				if err != nil {
					return err
				}
				kvValue, err = s.getKvTableConfigValue(kt, one)
				if err != nil {
					return err
				}
			}

			if one.Spec.KvType != table.KvTab {
				_, kvValue, err = s.getKv(kt, one.Attachment.BizID, one.Attachment.AppID, one.Spec.Version, one.Spec.Key)
				if err != nil {
					return err
				}
			}
			// 锁住关键部分，避免并发修改切片
			mux.Lock()
			// 保证按顺序写入
			kvs[i], err = pbkv.PbKv(one, kvValue, name)
			if err != nil {
				return err
			}
			mux.Unlock()

			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, errf.Errorf(errf.Aborted, i18n.T(kt, "get key value failed, err: %v"), err)
	}

	return kvs, nil

}

// DeleteKv is used to delete key-value data.
func (s *Service) DeleteKv(ctx context.Context, req *pbds.DeleteKvReq) (*pbbase.EmptyResp, error) {

	kt := kit.FromGrpcContext(ctx)

	kv, err := s.dao.Kv().GetByID(kt, req.Attachment.BizId, req.Attachment.AppId, req.Id)
	if err != nil {
		logs.Errorf("get kv (%d) failed, err: %v, rid: %s", req.Spec.Key, err, kt.Rid)
		return nil, err
	}

	if kv.KvState == table.KvStateAdd {
		if e := s.dao.Kv().Delete(kt, kv); e != nil {
			logs.Errorf("delete kv failed, err: %v, rid: %s", e, kt.Rid)
			return nil, e
		}
	} else {
		kv.KvState = table.KvStateDelete
		kv.Revision.Reviser = kt.User
		if e := s.dao.Kv().Update(kt, kv); e != nil {
			logs.Errorf("delete kv failed, err: %v, rid: %s", e, kt.Rid)
			return nil, e
		}
	}

	return new(pbbase.EmptyResp), nil
}

// BatchUpsertKvs is used to insert or update key-value data in bulk.
// 1.键存在则更新, 类型不一致直接提示错误
// 2.键不存在则新增
// replace_all为true时，清空表中的数据，但保证前面两条逻辑
// nolint:funlen
func (s *Service) BatchUpsertKvs(ctx context.Context, req *pbds.BatchUpsertKvsReq) (*pbds.BatchUpsertKvsResp, error) {

	// FromGrpcContext used only to obtain Kit through grpc context.
	kt := kit.FromGrpcContext(ctx)

	app, err := s.dao.App().Get(kt, req.BizId, req.AppId)
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed, i18n.T(kt, "get app failed, err: %v", err))
	}
	if app.Spec.ConfigType != table.KV {
		return nil, errors.New(i18n.T(kt, "not a KV type service"))
	}
	for _, kv := range req.Kvs {
		if !checkKVTypeMatch(table.DataType(kv.KvSpec.KvType), app.Spec.DataType) {
			return nil, errors.New(i18n.T(kt, "kv type does not match the data type defined in the application"))
		}
	}

	kvStateArr := []string{
		string(table.KvStateUnchange),
		string(table.KvStateAdd),
		string(table.KvStateRevise),
	}

	// 1. 查询过滤删除后的kv
	kvs, err := s.dao.Kv().ListAllByAppID(kt, req.GetAppId(), req.GetBizId(), kvStateArr)
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed, i18n.T(kt, "list kv failed, err: %v", err))
	}

	isRollback := true
	tx := s.dao.GenQuery().Begin()
	defer func() {
		if isRollback {
			if rErr := tx.Rollback(); rErr != nil {
				logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kt.Rid)
			}
		}
	}()
	// 2. 检测服务配置项类型（相同的key类型是否一致）
	if err = s.checkKVConfigItemTypes(kt, req, kvs); err != nil {
		return nil, err
	}

	// 3. 清空草稿区域
	if err = s.clearDraftKVStore(kt, tx, req, kvs); err != nil {
		return nil, err
	}

	// 4. 在vault中执行更新
	versionMap, err := s.doBatchUpsertVault(kt, req)
	if err != nil {
		return nil, errors.New(i18n.T(kt, "batch import of KV config failed, err: %v", err))
	}

	// 5. 处理需要编辑和创建的数据
	toUpdate, toCreate, err := s.checkKvs(kt, tx, req, versionMap, kvStateArr)
	if err != nil {
		return nil, err
	}

	// 检测kv配置项是否超出服务限制
	err = s.checkKVConfigItemExceedsAppLimit(kt, req.BizId, req.AppId, int64(len(toCreate)), int64(len(toUpdate)))
	if err != nil {
		return nil, err
	}

	// 5. 创建或更新kv等操作
	if len(toCreate) > 0 {
		if err = s.dao.Kv().BatchCreateWithTx(kt, tx, toCreate); err != nil {
			return nil, errf.Errorf(errf.DBOpFailed, i18n.T(kt, "batch import of KV config failed, err: %v", err))
		}
	}

	if len(toUpdate) > 0 {
		if err = s.dao.Kv().BatchUpdateWithTx(kt, tx, toUpdate); err != nil {
			return nil, errf.Errorf(errf.DBOpFailed, i18n.T(kt, "batch import of KV config failed, err: %v", err))
		}
	}

	if e := tx.Commit(); e != nil {
		logs.Errorf("commit transaction failed, err: %v, rid: %s", e, kt.Rid)
		return nil, errf.Errorf(errf.DBOpFailed, i18n.T(kt, "batch import of KV config failed, err: %v", e))
	}
	isRollback = false

	createIds, updateIds := []uint32{}, []uint32{}
	for _, item := range toCreate {
		createIds = append(createIds, item.ID)
	}
	for _, item := range toUpdate {
		updateIds = append(updateIds, item.ID)
	}

	return &pbds.BatchUpsertKvsResp{
		Ids: tools.MergeAndDeduplicate(createIds, updateIds),
	}, nil
}

// 检测键值对配置项类型
func (s *Service) checkKVConfigItemTypes(kt *kit.Kit, req *pbds.BatchUpsertKvsReq, kvs []*table.Kv) error {

	existsKvs := map[string]string{}
	for _, v := range kvs {
		existsKvs[v.Spec.Key] = string(v.Spec.KvType)
	}

	for _, v := range req.GetKvs() {
		kvType, exist := existsKvs[v.KvSpec.Key]
		if exist && v.KvSpec.KvType != kvType {
			return errors.New(i18n.T(kt, "the type of config item %s is incorrect", v.KvSpec.Key))
		}
	}

	return nil
}

// 清空键值对草稿区域
func (s *Service) clearDraftKVStore(kt *kit.Kit, tx *gen.QueryTx, req *pbds.BatchUpsertKvsReq,
	kvs []*table.Kv) error {

	if !req.ReplaceAll {
		return nil
	}

	reallyDelete := []uint32{}
	fakeDelete := make([]*table.Kv, 0)
	for _, v := range kvs {
		// 如果是新增类型需要真删除, 否则假删除
		if v.KvState == table.KvStateAdd {
			reallyDelete = append(reallyDelete, v.ID)
		} else {
			v.Revision.Reviser = kt.User
			v.Revision.UpdatedAt = time.Now().UTC()
			v.KvState = table.KvStateDelete
			fakeDelete = append(fakeDelete, v)
		}
	}

	if err := s.dao.Kv().BatchDeleteWithTx(kt, tx, req.GetBizId(), req.GetAppId(), reallyDelete); err != nil {
		if rErr := tx.Rollback(); rErr != nil {
			logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kt.Rid)
		}
		return errf.Errorf(errf.DBOpFailed, i18n.T(kt, "clearing draft area failed, err: %v", err))
	}

	if err := s.dao.Kv().BatchUpdateWithTx(kt, tx, fakeDelete); err != nil {
		if rErr := tx.Rollback(); rErr != nil {
			logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kt.Rid)
		}
		return errf.Errorf(errf.DBOpFailed, i18n.T(kt, "clearing draft area failed, err: %v", err))
	}

	return nil
}

func (s *Service) getKv(kt *kit.Kit, bizID, appID, version uint32, key string) (table.DataType, string, error) {
	opt := &types.GetKvByVersion{
		BizID:   bizID,
		AppID:   appID,
		Key:     key,
		Version: int(version),
	}

	return s.vault.GetKvByVersion(kt, opt)
}

// doBatchUpsertVault is used to perform bulk insertion or update of key-value data in Vault.
func (s *Service) doBatchUpsertVault(kt *kit.Kit, req *pbds.BatchUpsertKvsReq) (map[string]int, error) {
	var mux sync.Mutex
	eg, _ := errgroup.WithContext(context.Background())
	eg.SetLimit(10)
	versionMap := make(map[string]int)
	for _, kv := range req.Kvs {
		kv := kv
		eg.Go(func() error {
			opt := &types.UpsertKvOption{
				BizID:  req.BizId,
				AppID:  req.AppId,
				Key:    kv.KvSpec.Key,
				Value:  kv.KvSpec.Value,
				KvType: table.DataType(kv.KvSpec.KvType),
			}
			version, err := s.vault.UpsertKv(kt, opt)
			if err != nil {
				return err
			}
			mux.Lock()
			versionMap[kv.KvSpec.Key] = version
			mux.Unlock()

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, errf.Errorf(errf.Aborted, i18n.T(kt, "batch upsert vault failed, err: %v", err))
	}

	return versionMap, nil
}

func (s *Service) checkKvs(kt *kit.Kit, tx *gen.QueryTx, req *pbds.BatchUpsertKvsReq, versionMap map[string]int,
	kvStates []string) (toUpdate, toCreate []*table.Kv, err error) {

	// 通过事务获取指定状态的kv
	editingKvs, err := s.dao.Kv().ListAllByAppIDWithTx(kt, tx, req.GetAppId(), req.GetBizId(), kvStates)
	if err != nil {
		return nil, nil, errf.Errorf(errf.DBOpFailed, i18n.T(kt, "list kv failed, err: %v", err))
	}

	editingKvMap := make(map[string]*table.Kv)
	for _, kv := range editingKvs {
		editingKvMap[kv.Spec.Key] = kv
	}

	for _, kv := range req.Kvs {

		var version int
		var exists bool
		var editing *table.Kv

		if version, exists = versionMap[kv.KvSpec.Key]; !exists {
			return nil, nil, errors.New(i18n.T(kt, "save kv failed"))
		}
		sepc, err := kv.GetKvSpec().KvSpec()
		if err != nil {
			return nil, nil, err
		}
		sepc.Version = uint32(version)
		now := time.Now().UTC()

		kvAttachment := &table.KvAttachment{
			BizID: req.BizId,
			AppID: req.AppId,
		}
		contentSpec := &table.ContentSpec{
			Signature: tools.SHA256(kv.KvSpec.Value),
			Md5:       tools.MD5(kv.KvSpec.Value),
			ByteSize:  uint64(len(kv.KvSpec.Value)),
		}

		if editing, exists = editingKvMap[kv.KvSpec.Key]; exists {
			if editing.KvState == table.KvStateUnchange {
				editing.KvState = table.KvStateRevise
			}
			sepc.ManagedTableID = editing.Spec.ManagedTableID
			sepc.ExternalSourceID = editing.Spec.ExternalSourceID
			sepc.FilterCondition = editing.Spec.FilterCondition
			sepc.FilterFields = editing.Spec.FilterFields
			toUpdate = append(toUpdate, &table.Kv{
				ID:          editing.ID,
				KvState:     editing.KvState,
				Spec:        sepc,
				Attachment:  kvAttachment,
				Revision:    editing.Revision,
				ContentSpec: contentSpec,
			})
		} else {
			toCreate = append(toCreate, &table.Kv{
				KvState:    table.KvStateAdd,
				Spec:       sepc,
				Attachment: kvAttachment,
				Revision: &table.Revision{
					Creator:   kt.User,
					Reviser:   kt.User,
					CreatedAt: now,
					UpdatedAt: now,
				},
				ContentSpec: contentSpec,
			})
		}
	}

	return toUpdate, toCreate, nil
}

// UnDeleteKv Revert the deletion of the key-value pair by restoring it to the version before the last one.
func (s *Service) UnDeleteKv(ctx context.Context, req *pbds.UnDeleteKvReq) (*pbbase.EmptyResp, error) {

	kt := kit.FromGrpcContext(ctx)

	// 只有删除的才能恢复
	kv, err := s.dao.Kv().GetByKvState(kt, req.GetBizId(), req.GetAppId(), req.GetKey(), []string{
		string(table.KvStateDelete),
	})
	if err != nil {
		logs.Errorf("get kv (%s) failed, err: %v, rid: %s", req.GetKey(), err, kt.Rid)
		return nil, err
	}

	// 看该key是否有存在新增
	addKv, err := s.dao.Kv().GetByKvState(kt, req.GetBizId(), req.GetAppId(), req.GetKey(), []string{
		string(table.KvStateAdd),
	})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logs.Errorf("get kv (%s) failed, err: %v, rid: %s", req.GetKey(), err, kt.Rid)
		return nil, err
	}

	tx := s.dao.GenQuery().Begin()
	if addKv != nil && addKv.ID > 0 {
		if e := s.dao.Kv().DeleteWithTx(kt, tx, addKv); e != nil {
			if rErr := tx.Rollback(); rErr != nil {
				logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kt.Rid)
			}
			logs.Errorf("delete kv (%s) failed, err: %v, rid: %s", req.GetKey(), e, kt.Rid)
		}
	}

	toUpdate, err := s.getLatestReleasedKV(kt, req.GetBizId(), req.GetAppId(), kv)
	if err != nil {
		if rErr := tx.Rollback(); rErr != nil {
			logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kt.Rid)
		}
		return nil, err
	}

	if err = s.dao.Kv().UpdateWithTx(kt, tx, toUpdate); err != nil {
		if rErr := tx.Rollback(); rErr != nil {
			logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kt.Rid)
		}
		logs.Errorf("undelete kv (%s) failed, err: %v, rid: %s", req.GetKey(), err, kt.Rid)

	}

	if e := tx.Commit(); e != nil {
		logs.Errorf("commit transaction failed, err: %v, rid: %s", e, kt.Rid)
		return nil, e
	}

	return new(pbbase.EmptyResp), nil

}

// UndoKv Undo edited data and return to the latest published version
func (s *Service) UndoKv(ctx context.Context, req *pbds.UndoKvReq) (*pbbase.EmptyResp, error) {
	kt := kit.FromGrpcContext(ctx)

	// 只有编辑的才能撤回
	kvState := []string{
		string(table.KvStateRevise),
	}
	kv, err := s.dao.Kv().GetByKvState(kt, req.GetBizId(), req.GetAppId(), req.GetKey(), kvState)
	if err != nil {
		logs.Errorf("get kv (%s) failed, err: %v, rid: %s", req.GetKey(), err, kt.Rid)
		return nil, err
	}

	toUpdate, err := s.getLatestReleasedKV(kt, req.GetBizId(), req.GetAppId(), kv)
	if err != nil {
		return nil, err
	}

	if err = s.dao.Kv().Update(kt, toUpdate); err != nil {
		logs.Errorf("undo kv (%s) failed, err: %v, rid: %s", req.GetKey(), err, kt.Rid)
		return nil, err
	}

	return new(pbbase.EmptyResp), nil
}

func (s *Service) getLatestReleasedKV(kt *kit.Kit, bizID, appID uint32, kv *table.Kv) (*table.Kv, error) {

	// 获取该服务最新发布的 release_id
	release, err := s.dao.Release().GetReleaseLately(kt, bizID, appID)
	if err != nil {
		return nil, err
	}

	rkv, err := s.dao.ReleasedKv().Get(kt, bizID, appID, release.ID, kv.Spec.Key)
	if err != nil {
		return nil, err
	}

	// 获取最新发布的kv
	_, kvValue, err := s.getReleasedKv(kt, bizID, appID, rkv.Spec.Version, release.ID, kv.Spec.Key)
	if err != nil {
		return nil, err
	}
	opt := &types.UpsertKvOption{
		BizID:  bizID,
		AppID:  appID,
		Key:    kv.Spec.Key,
		Value:  kvValue,
		KvType: kv.Spec.KvType,
	}
	// UpsertKv 创建｜更新kv
	version, err := s.vault.UpsertKv(kt, opt)
	if err != nil {
		logs.Errorf("update kv failed, err: %v, rid: %s", err, kt.Rid)
		return nil, err
	}

	kv.KvState = table.KvStateUnchange
	kv.Spec.CertificateExpirationDate = rkv.Spec.CertificateExpirationDate
	kv.Revision = &table.Revision{
		Reviser:   kt.User,
		UpdatedAt: time.Now().UTC(),
	}

	kv.Spec.Version = uint32(version)
	kv.ContentSpec = &table.ContentSpec{
		Signature: tools.SHA256(kvValue),
		Md5:       tools.MD5(kvValue),
		ByteSize:  uint64(len(kvValue)),
	}

	return kv, nil
}

// KvFetchIDsExcluding Kv 获取指定ID后排除的ID
func (s *Service) KvFetchIDsExcluding(ctx context.Context, req *pbds.KvFetchIDsExcludingReq) (
	*pbds.KvFetchIDsExcludingResp, error) {
	kt := kit.FromGrpcContext(ctx)

	ids, err := s.dao.Kv().FetchIDsExcluding(kt, req.BizId, req.AppId, req.GetIds())
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed, i18n.T(kt, "get excluded kv failed, err: %s", err))
	}

	return &pbds.KvFetchIDsExcludingResp{
		Ids: ids,
	}, nil
}

// 检测kv服务项是否超出限制
// addQuantity 新增的数量
// subQuantity 减少的数量
func (s *Service) checkKVConfigItemExceedsAppLimit(kit *kit.Kit, bizID, appID uint32,
	addQuantity, subQuantity int64) error {
	// 获取未删除的kv数量
	count, err := s.dao.Kv().CountNumberUnDeleted(kit, bizID, &types.ListKvOption{
		AppID: appID,
	})
	if err != nil {
		return errf.Errorf(errf.DBOpFailed,
			i18n.T(kit, "count the number of kV files that have not been deleted failed, err: %v", err))
	}

	// 判断是否超出服务限制
	appConfigCnt := getAppConfigCnt(bizID)
	if count-subQuantity+addQuantity > int64(appConfigCnt) {
		return errf.New(errf.InvalidParameter,
			i18n.T(kit, `the total number of config items exceeded the limit %d`, appConfigCnt))
	}

	return nil
}

// KvFetchKeysExcluding 获取指定keys后排除的keys
func (s *Service) KvFetchKeysExcluding(ctx context.Context, req *pbds.KvFetchKeysExcludingReq) (
	*pbds.KvFetchKeysExcludingResp, error) {
	kt := kit.FromGrpcContext(ctx)

	keys, err := s.dao.Kv().FetchKeysExcluding(kt, req.BizId, req.AppId, req.GetKeys())
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed, i18n.T(kt, "get excluded kv failed, err: %s", err))
	}

	return &pbds.KvFetchKeysExcludingResp{
		Keys: keys,
	}, nil
}

// FindNearExpiryCertKvs 查找临近到期证书
func (s *Service) FindNearExpiryCertKvs(ctx context.Context, req *pbds.FindNearExpiryCertKvsReq) (
	*pbds.FindNearExpiryCertKvsResp, error) {
	// FromGrpcContext used only to obtain Kit through grpc context.
	kit := kit.FromGrpcContext(ctx)

	details, count, err := s.dao.Kv().FindNearExpiryCertKvs(kit, req.BizId, req.AppId, req.Days, &types.BasePage{
		Start: req.Start,
		Limit: uint(req.Limit),
		All:   req.All,
	})
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed, i18n.T(kit, "get a list of expired certificates failed, err: %v"), err)
	}

	kvs, err := s.setKvTypeAndValue(kit, details)
	if err != nil {
		return nil, err
	}

	return &pbds.FindNearExpiryCertKvsResp{Details: kvs, Count: count}, nil
}

// 获取表格型预览数据
func (s *Service) getKvTableConfigPreviewName(kit *kit.Kit, bizID, managedTableId, externalSourceId uint32) (string, error) {
	// 表示托管表格
	if managedTableId != 0 {
		mappings, err := s.dao.DataSourceMapping().GetDataSourceMappingByID(kit, managedTableId)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", err
		}

		if mappings == nil {
			return "", nil
		}

		return mappings.Spec.TableName_, nil
	}

	if externalSourceId != 0 {
		var names []string
		info, err := s.dao.DataSourceInfo().Get(kit, bizID, externalSourceId)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", err
		}
		mappings, err := s.dao.DataSourceMapping().ListByDataSourceInfoId(kit, bizID, externalSourceId)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", err
		}

		if info != nil {
			names = append(names, info.Spec.Name)
		}

		if len(mappings) != 0 {
			for _, mapping := range mappings {
				names = append(names, mapping.Spec.TableName_)
			}
		}

		return strings.Join(names, ","), nil
	}

	return "", nil
}

func (s *Service) getKvTableConfigValue(kit *kit.Kit, kv *table.Kv) (string, error) {

	if kv == nil {
		return "", nil
	}
	var contents []*table.DataSourceContent
	var err error
	if kv.Spec.ManagedTableID != 0 {
		contents, _, err = s.dao.DataSourceContent().List(kit, kv.Spec.ManagedTableID, kv.Spec.FilterCondition, kv.Spec.FilterFields, &types.BasePage{All: true})
		if err != nil {
			return "", err
		}
	}

	if kv.Spec.ExternalSourceID != 0 {
		contents, _, err = s.dao.DataSourceContent().List(kit, kv.Spec.ExternalSourceID, kv.Spec.FilterCondition, kv.Spec.FilterFields, &types.BasePage{All: true})
		if err != nil {
			return "", err
		}
	}

	if len(contents) != 0 {
		result := make([]datatypes.JSONMap, 0)
		for _, v := range contents {
			result = append(result, v.Spec.Content)
		}

		// 将 result 切片转换为 JSON 格式的字符串
		contentBytes, err := json.Marshal(result)
		if err != nil {
			return "", fmt.Errorf("failed to marshal JSONMap slice: %w", err)
		}
		// 返回转换后的字符串
		return string(contentBytes), nil
	}

	return "", nil
}
