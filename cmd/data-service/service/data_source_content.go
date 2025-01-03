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
	"time"

	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbdsc "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/data-source-content"
	pbgroup "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/group"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/runtime/selector"
	"github.com/TencentBlueKing/bk-bscp/pkg/tools"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// CreateTableContent 创建表数据(暂时没有用到)
func (s *Service) CreateTableContent(ctx context.Context, req *pbds.CreateTableContentReq) (
	*pbds.CreateTableContentResp, error) {
	kit := kit.FromGrpcContext(ctx)

	// 1. 拿到表结构
	tableStruct, err := s.dao.DataSourceMapping().
		GetDataSourceMappingByID(kit, req.DataSourceMappingId)
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed,
			i18n.T(kit, "get the data source table failed, err: %v", err))
	}

	contents, count, err := s.dao.DataSourceContent().ListByDataSourceMappingID(kit, req.DataSourceMappingId,
		&types.BasePage{
			All: true,
		})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errf.Errorf(errf.DBOpFailed,
			i18n.T(kit, "get the data list according to the data dource mapping id failed, err: %v", err))
	}

	rows := make([]*table.DataSourceContent, 0)

	// 2. 验证新增和已存在的数据是否符合表结构
	if count > 0 {
		rows = append(rows, contents...)
	}
	for _, v := range req.GetContent() {
		rows = append(rows, &table.DataSourceContent{
			Attachment: &table.DataSourceContentAttachment{
				DataSourceMappingID: req.DataSourceMappingId,
			},
			Spec: &table.DataSourceContentSpec{
				Content: v.AsMap(),
				Status:  table.KvStateAdd.String(),
			},
			Revision: &table.Revision{
				Creator:   kit.User,
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			},
		})
	}

	if err = s.verifyTableData(kit, tableStruct.Spec.Columns_, rows); err != nil {
		return nil, err
	}

	// 获取需要新增的数据
	newData := make([]*table.DataSourceContent, 0, len(req.GetContent()))
	for _, v := range rows {
		if v.ID != 0 {
			continue
		}
		newData = append(newData, v)
	}

	if err = s.dao.DataSourceContent().BatchCreate(kit, newData); err != nil {
		return nil, err
	}

	ids := []uint32{}
	for _, v := range newData {
		ids = append(ids, v.ID)
	}

	return &pbds.CreateTableContentResp{Ids: ids}, nil
}

// ListTableContent 获取表数据列表
func (s *Service) ListTableContent(ctx context.Context, req *pbds.ListTableContentReq) (
	*pbds.ListTableContentResp, error) {
	kit := kit.FromGrpcContext(ctx)

	var filterCondition *selector.Selector
	var err error
	if req.GetFilterCondition() != nil && len(req.GetFilterCondition().AsMap()) != 0 {
		filterCondition, err = pbgroup.UnmarshalSelector(req.GetFilterCondition())
		if err != nil {
			return nil, err
		}
	}

	items, count, err := s.dao.DataSourceContent().List(kit, req.DataSourceMappingId, filterCondition,
		req.FilterFields, &types.BasePage{
			Start: req.Start,
			Limit: uint(req.Limit),
			All:   req.All,
		})
	if err != nil {
		return nil, err
	}

	// 1. 获取表结构
	tableStruct, err := s.dao.DataSourceMapping().
		GetDataSourceMappingByID(kit, req.DataSourceMappingId)
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed,
			i18n.T(kit, "get the data source table failed, err: %v", err))
	}
	filterFieldsMap := make(map[string]struct{})
	for _, field := range req.FilterFields {
		filterFieldsMap[field] = struct{}{}
	}

	fields := make([]*pbdsc.Field, 0)
	for _, v := range tableStruct.Spec.Columns_ {
		// 如果 v.Name 在 filterFieldsMap 中，跳过追加
		if _, exists := filterFieldsMap[v.Name]; exists {
			continue
		}
		fields = append(fields, &pbdsc.Field{
			Name:       v.Name,
			Alias:      v.Alias,
			ColumnType: string(v.ColumnType),
			Primary:    v.Primary,
			EnumValue:  v.EnumValue,
			Selected:   v.Selected,
		})
	}

	return &pbds.ListTableContentResp{
			Details: pbdsc.PbDataSourceContents(items),
			Count:   count,
			Fields:  fields,
		},
		nil
}

// CheckTableField 检测表字段是否存在以及是否有值
func (s *Service) CheckTableField(ctx context.Context, req *pbds.CheckTableFieldReq) (
	*pbds.CheckTableFieldResp, error) {
	kit := kit.FromGrpcContext(ctx)

	_, count, err := s.dao.DataSourceContent().ListByJsonField(kit, req.DataSourceMappingId, req.FieldName,
		table.StringColumn, &types.BasePage{
			All: true,
		})
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed,
			i18n.T(kit, "get the data list according to the data dource mapping id failed, err: %v", err))
	}

	return &pbds.CheckTableFieldResp{
		Exist: func() bool {
			return count > 0
		}(),
	}, nil
}

// UpdateTableContent implements pbds.DataServer.
func (s *Service) UpdateTableContent(ctx context.Context, req *pbds.UpdateTableContentReq) (
	*pbds.UpdateTableContentResp, error) {
	kit := kit.FromGrpcContext(ctx)

	// 1. 拿到表结构
	tableStruct, err := s.dao.DataSourceMapping().
		GetDataSourceMappingByID(kit, req.DataSourceMappingId)
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed,
			i18n.T(kit, "get the data source table failed, err: %v", err))
	}

	alreadyExistIDs := map[uint32]bool{}

	toCreate := make([]*table.DataSourceContent, 0)
	toUpdate := make([]*table.DataSourceContent, 0)
	for _, v := range req.GetContents() {
		base := &table.DataSourceContent{
			Attachment: &table.DataSourceContentAttachment{
				DataSourceMappingID: req.DataSourceMappingId,
			},
			Spec: &table.DataSourceContentSpec{
				Content: v.GetContent().AsMap(),
			},
			Revision: &table.Revision{
				UpdatedAt: time.Now().UTC(),
			},
		}

		if v.TableContentId == 0 {
			base.Spec.Status = table.KvStateAdd.String()
			base.Revision.CreatedAt = time.Now().UTC()
			base.Revision.Creator = kit.User
			toCreate = append(toCreate, base)
		} else {
			base.ID = v.TableContentId
			base.Spec.Status = table.KvStateRevise.String()
			base.Revision.Reviser = kit.User
			toUpdate = append(toUpdate, base)
			alreadyExistIDs[v.TableContentId] = true
		}
	}

	contents, count, err := s.dao.DataSourceContent().ListByDataSourceMappingID(kit, req.DataSourceMappingId,
		&types.BasePage{
			All: true,
		})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errf.Errorf(errf.DBOpFailed,
			i18n.T(kit, "get the data list according to the data dource mapping id failed, err: %v", err))
	}
	alreadyExists := make([]*table.DataSourceContent, 0)
	if count > 0 {
		for _, v := range contents {
			if !alreadyExistIDs[v.ID] {
				alreadyExists = append(alreadyExists, v)
			}
		}
	}

	// 合并切片
	combined := append(toCreate, toUpdate...)
	combined = append(combined, alreadyExists...)

	if err = handleAutoIncrement(kit, tableStruct.Spec.Columns_, combined); err != nil {
		return nil, err
	}

	if err = s.verifyTableData(kit, tableStruct.Spec.Columns_, combined); err != nil {
		return nil, err
	}

	// 2. 创建、更新、删除数据
	tx := s.dao.GenQuery().Begin()

	if len(toCreate) != 0 {
		if err := s.dao.DataSourceContent().BatchCreateWithTx(kit, tx, toCreate); err != nil {
			if rErr := tx.Rollback(); rErr != nil {
				logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kit.Rid)
			}
			return nil, err
		}
	}

	if len(toUpdate) != 0 {
		if err := s.dao.DataSourceContent().BatchUpdateWithTx(kit, tx, toUpdate); err != nil {
			if rErr := tx.Rollback(); rErr != nil {
				logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kit.Rid)
			}
			return nil, err
		}
	}

	if len(req.GetDelIds()) != 0 {
		if err := s.dao.DataSourceContent().BatchDeleteWithTx(kit, tx, req.DataSourceMappingId,
			req.GetDelIds()); err != nil {
			if rErr := tx.Rollback(); rErr != nil {
				logs.Errorf("transaction rollback failed, err: %v, rid: %s", rErr, kit.Rid)
			}
			return nil, err
		}
	}

	if e := tx.Commit(); e != nil {
		logs.Errorf("commit transaction failed, err: %v, rid: %s", e, kit.Rid)
		return nil, e
	}

	updateIds, createIds := []uint32{}, []uint32{}
	for _, v := range toUpdate {
		updateIds = append(updateIds, v.ID)
	}
	for _, v := range toCreate {
		createIds = append(createIds, v.ID)
	}

	return &pbds.UpdateTableContentResp{Ids: tools.MergeAndDeduplicate(createIds, updateIds)}, nil
}

// 根据字段规则，验证数据
func (s *Service) verifyTableData(kit *kit.Kit, columns datatypes.JSONSlice[*table.Columns_],
	rows []*table.DataSourceContent) error {
	managers := map[string]*RuleManager{}

	// 为每列添加验证规则
	for _, col := range columns {
		managers[col.Name] = BuildRuleManager(col)
	}

	for i, row := range rows {
		for _, col := range columns {
			defaultVal, err := managers[col.Name].Validate(kit, row.Spec.Content[col.Name], col,
				row.Spec.Content)
			if err != nil {
				return errors.New(i18n.T(kit, "row %d: %v", i+1, err))
			}
			row.Spec.Content[col.Name] = defaultVal
		}
	}

	return nil
}

// 处理自增字段数据
func handleAutoIncrement(kit *kit.Kit, columns datatypes.JSONSlice[*table.Columns_],
	rows []*table.DataSourceContent) error {
	// 查询出自增的字段
	for _, column := range columns {
		if column.AutoIncrement {
			maxID, err := findMaxValue(kit, column.Name, rows)
			if err != nil {
				return err
			}
			for _, row := range rows {
				if row.Spec.Content[column.Name] == "" || row.Spec.Content[column.Name] == nil {
					maxID++
					row.Spec.Content[column.Name] = maxID
				}
			}
		}
	}

	return nil
}

// 获取某列最大的值
func findMaxValue(kit *kit.Kit, fieldName string, rows []*table.DataSourceContent) (int64, error) {
	var maxID int64
	for _, row := range rows {
		if row.Spec.Content[fieldName] != "" || row.Spec.Content[fieldName] == nil {
			// 尝试转换为不同的数字类型
			var numValue int64
			var err error
			switch value := row.Spec.Content[fieldName].(type) {
			case json.Number:
				numValue, err = value.Int64()
				if err != nil {
					floatValue, err := value.Float64()
					if err != nil {
						return 0, errors.New(i18n.T(kit,
							"returns the number as a float64 failed, err: %v", err))
					}
					numValue = int64(floatValue)
				}
			case int64:
				numValue = value
			case float64:
				numValue = int64(value)
			}

			if numValue > maxID {
				maxID = numValue
			}
		}
	}

	return maxID, nil
}
