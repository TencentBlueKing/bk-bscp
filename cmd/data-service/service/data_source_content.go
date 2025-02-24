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
	"time"

	"google.golang.org/protobuf/types/known/structpb"
	"gorm.io/datatypes"
	"gorm.io/gorm"

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
			Count:   uint32(count),
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

// UpsertTableContent implements pbds.DataServer.
// 1. 获取表结构
// 2. 获取表中已存在数据
// 3. 通过数据中的主键对比，获取新增、编辑、删除的数据
func (s *Service) UpsertTableContent(ctx context.Context, req *pbds.UpsertTableContentReq) (
	*pbds.UpsertTableContentResp, error) {
	kit := kit.FromGrpcContext(ctx)

	// 1. 获取表结构
	tableStruct, err := s.dao.DataSourceMapping().
		GetDataSourceMappingByID(kit, req.DataSourceMappingId)
	if err != nil {
		return nil, errf.Errorf(errf.DBOpFailed,
			i18n.T(kit, "get the data source table failed, err: %v", err))
	}

	// 2. 获取表中已存在数据
	contents, _, err := s.dao.DataSourceContent().ListByDataSourceMappingID(kit, req.DataSourceMappingId,
		&types.BasePage{
			All: true,
		})
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errf.Errorf(errf.DBOpFailed,
			i18n.T(kit, "get the data list according to the data dource mapping id failed, err: %v", err))
	}

	// 3. 处理表格数据（验证表结构和数据）
	toCreate, toUpdate, delIDs, err := s.handleTableContent(kit, tableStruct, contents, req.GetContents())
	if err != nil {
		return nil, err
	}

	// 4. 创建、更新、删除数据
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

	if len(delIDs) != 0 {
		if err := s.dao.DataSourceContent().BatchFakeDeleteWithTx(kit, tx, req.DataSourceMappingId,
			delIDs); err != nil {
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

	return &pbds.UpsertTableContentResp{Ids: tools.MergeAndDeduplicate(createIds, updateIds)}, nil
}

func (s *Service) handleTableContent(kit *kit.Kit, tableStruct *table.DataSourceMapping,
	oldContents []*table.DataSourceContent, newContents []*structpb.Struct) (
	[]*table.DataSourceContent, []*table.DataSourceContent, []uint32, error) {
	toCreate := make([]*table.DataSourceContent, 0)
	toUpdate := make([]*table.DataSourceContent, 0)
	delIDs := []uint32{}

	var primary string
	// 1. 查询出主键，按主键来对比
	for _, v := range tableStruct.Spec.Columns_ {
		if v.Primary {
			primary = v.Name
		}
	}

	// 主键可能是数字或者字符串类型，所以统一转成字符串处理
	existingDatas := make(map[string]string, 0)
	existingIDs := map[string]uint32{}
	existingStates := make(map[string]string, 0)

	if len(oldContents) > 0 {
		for _, v := range oldContents {
			// 统一转成字符串处理
			data, err := json.Marshal(v.Spec.Content)
			if err != nil {
				return toCreate, toUpdate, delIDs, err
			}
			strValue := fmt.Sprintf("%v", v.Spec.Content[primary])
			existingDatas[strValue] = string(data)
			existingIDs[strValue] = v.ID
			existingStates[strValue] = v.Spec.Status
		}
	}

	// 处理提交的数据
	submitDatas := make(map[string]string, 0)
	for _, v := range newContents {
		// 统一转成字符串
		jsonData, err := json.Marshal(v.AsMap())
		if err != nil {
			return toCreate, toUpdate, delIDs, err
		}
		if v.AsMap()[primary] == nil {
			return toCreate, toUpdate, delIDs, errors.New(i18n.T(kit, "the primary key cannot be empty"))
		}
		strValue := fmt.Sprintf("%v", v.AsMap()[primary])
		submitDatas[strValue] = string(jsonData)
	}

	for k, v := range submitDatas {
		var jsonMap datatypes.JSONMap
		if err := json.Unmarshal([]byte(v), &jsonMap); err != nil {
			return toCreate, toUpdate, delIDs, err
		}
		base := &table.DataSourceContent{
			Attachment: &table.DataSourceContentAttachment{
				DataSourceMappingID: tableStruct.ID,
			},
			Spec: &table.DataSourceContentSpec{
				Content: jsonMap,
			},
			Revision: &table.Revision{
				UpdatedAt: time.Now().UTC(),
			},
		}
		// 通过主键判断数据是否存在
		_, ok := existingDatas[k]
		if ok {
			base.Spec.Status = existingStates[k]
			// 验证两个字符串是否相等
			if existingDatas[k] != v {
				switch existingStates[k] {
				case table.KvStateAdd.String(), table.KvStateDelete.String():
					base.Spec.Status = table.KvStateAdd.String()
				case table.KvStateRevise.String(), table.KvStateUnchange.String():
					base.Spec.Status = table.KvStateRevise.String()
				}
			}
			if existingDatas[k] == v && existingStates[k] == table.KvStateDelete.String() {
				base.Spec.Status = table.KvStateUnchange.String()
			}
			base.ID = existingIDs[k]
			base.Revision.Reviser = kit.User
			toUpdate = append(toUpdate, base)
		} else {
			base.Spec.Status = table.KvStateAdd.String()
			base.Revision.Creator = kit.User
			base.Revision.CreatedAt = time.Now().UTC()
			toCreate = append(toCreate, base)
		}
	}

	// 合并切片
	combined := append(toCreate, toUpdate...)

	if err := handleAutoIncrement(kit, tableStruct.Spec.Columns_, combined); err != nil {
		return toCreate, toUpdate, delIDs, err
	}

	if err := s.verifyTableData(kit, tableStruct.Spec.Columns_, combined); err != nil {
		return toCreate, toUpdate, delIDs, err
	}

	differenceData := getDifference(existingDatas, submitDatas)

	for _, v := range differenceData {
		delIDs = append(delIDs, existingIDs[v])
	}

	return toCreate, toUpdate, delIDs, nil
}

func getDifference(array1, array2 map[string]string) []string {
	var diff []string
	for key := range array1 {
		if _, exists := array2[key]; !exists {
			diff = append(diff, key)
		}
	}
	return diff
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
		// 检查是否有多余字段
		for fieldName := range row.Spec.Content {
			if _, ok := managers[fieldName]; !ok {
				// 数据行中存在未定义的字段
				return errors.New(i18n.T(kit, "row %d: field '%s' is not defined in column definitions", i+1, fieldName))
			}
		}
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

// 获取某列最大的值
func findMaxValues(kit *kit.Kit, fieldName string, rows []*table.DataSourceContent) (float64, error) {
	var maxID float64
	for _, row := range rows {
		if row.Spec.Content[fieldName] != "" && row.Spec.Content[fieldName] != nil {
			var numValue float64
			var err error
			switch value := row.Spec.Content[fieldName].(type) {
			case json.Number:
				numValue, err = value.Float64()
				if err != nil {
					return 0, errors.New(i18n.T(kit,
						"returns the number as a float64 failed, err: %v", err))
				}
			case int64:
				numValue = float64(value)
			case float64:
				numValue = value
			}

			if numValue > maxID {
				maxID = numValue
			}
		}
	}

	return maxID, nil
}
