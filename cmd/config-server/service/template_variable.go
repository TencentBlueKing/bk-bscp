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

	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"

	cs "github.com/TencentBlueKing/bk-bscp/internal/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/meta"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbcs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/config-server"
	pbtv "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/template-variable"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// CreateTemplateVariable create a template variable
func (s *Service) CreateTemplateVariable(ctx context.Context, req *pbcs.CreateTemplateVariableReq) (
	*pbcs.CreateTemplateVariableResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	if !strings.HasPrefix(strings.ToLower(req.Name), constant.TemplateVariablePrefix) {
		return nil, errf.Errorf(errf.InvalidArgument, i18n.T(grpcKit,
			"template variable name must start with %s", constant.TemplateVariablePrefix))
	}

	r := &pbds.CreateTemplateVariableReq{
		Attachment: &pbtv.TemplateVariableAttachment{
			BizId: grpcKit.BizID,
		},
		Spec: &pbtv.TemplateVariableSpec{
			Name:       req.Name,
			Type:       req.Type,
			DefaultVal: req.DefaultVal,
			Memo:       req.Memo,
		},
	}
	rp, err := s.client.DS.CreateTemplateVariable(grpcKit.RpcCtx(), r)
	if err != nil {
		logs.Errorf("create template variable failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	resp := &pbcs.CreateTemplateVariableResp{
		Id: rp.Id,
	}
	return resp, nil
}

// DeleteTemplateVariable delete a template variable
func (s *Service) DeleteTemplateVariable(ctx context.Context, req *pbcs.DeleteTemplateVariableReq) (
	*pbcs.DeleteTemplateVariableResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	r := &pbds.DeleteTemplateVariableReq{
		Id: req.TemplateVariableId,
		Attachment: &pbtv.TemplateVariableAttachment{
			BizId: grpcKit.BizID,
		},
	}
	if _, err := s.client.DS.DeleteTemplateVariable(grpcKit.RpcCtx(), r); err != nil {
		logs.Errorf("delete template variable failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	return &pbcs.DeleteTemplateVariableResp{}, nil
}

// BatchDeleteTemplateVariable batch delete template variable
func (s *Service) BatchDeleteTemplateVariable(ctx context.Context, req *pbcs.BatchDeleteBizResourcesReq) (
	*pbcs.BatchDeleteResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	var ids []uint32
	ids = req.GetIds()
	if req.ExclusionOperation {
		result, err := s.client.DS.TemplateVariableFetchIDsExcluding(grpcKit.RpcCtx(),
			&pbds.TemplateVariableFetchIDsExcludingReq{
				BizId: req.GetBizId(),
				Ids:   req.GetIds(),
			})
		if err != nil {
			return nil, err
		}
		ids = result.GetIds()
	}

	idsLen := len(ids)
	if idsLen == 0 || idsLen > constant.ArrayInputLenLimit {
		return nil, errf.Errorf(errf.InvalidArgument, i18n.T(grpcKit,
			"the length of template variable ids is %d, it must be within the range of [1,%d]",
			idsLen, constant.ArrayInputLenLimit))
	}

	eg, egCtx := errgroup.WithContext(grpcKit.RpcCtx())
	eg.SetLimit(10)

	var successfulIDs, failedIDs []uint32
	var mux sync.Mutex

	// 使用 data-service 原子接口
	for _, v := range ids {
		v := v
		eg.Go(func() error {
			r := &pbds.DeleteTemplateVariableReq{
				Id: v,
				Attachment: &pbtv.TemplateVariableAttachment{
					BizId: req.BizId,
				},
			}
			if _, err := s.client.DS.DeleteTemplateVariable(egCtx, r); err != nil {
				logs.Errorf("delete template variable failed, err: %v, rid: %s", err, grpcKit.Rid)

				// 错误不返回异常，记录错误ID
				mux.Lock()
				failedIDs = append(failedIDs, v)
				mux.Unlock()
				return nil
			}

			mux.Lock()
			successfulIDs = append(successfulIDs, v)
			mux.Unlock()

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		logs.Errorf("batch delete failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, errf.Errorf(errf.Aborted, i18n.T(grpcKit, "batch delete failed"))
	}

	// 全部失败, 当前API视为失败
	if len(failedIDs) == len(ids) {
		return nil, errf.Errorf(errf.Aborted, i18n.T(grpcKit, "batch delete failed"))
	}

	return &pbcs.BatchDeleteResp{SuccessfulIds: successfulIDs, FailedIds: failedIDs}, nil
}

// UpdateTemplateVariable update a template variable
func (s *Service) UpdateTemplateVariable(ctx context.Context, req *pbcs.UpdateTemplateVariableReq) (
	*pbcs.UpdateTemplateVariableResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	r := &pbds.UpdateTemplateVariableReq{
		Id: req.TemplateVariableId,
		Attachment: &pbtv.TemplateVariableAttachment{
			BizId: grpcKit.BizID,
		},
		Spec: &pbtv.TemplateVariableSpec{
			DefaultVal: req.DefaultVal,
			Memo:       req.Memo,
		},
	}
	if _, err := s.client.DS.UpdateTemplateVariable(grpcKit.RpcCtx(), r); err != nil {
		logs.Errorf("update template variable failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	return &pbcs.UpdateTemplateVariableResp{}, nil
}

// ListTemplateVariables list template variables
func (s *Service) ListTemplateVariables(ctx context.Context, req *pbcs.ListTemplateVariablesReq) (
	*pbcs.ListTemplateVariablesResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	r := &pbds.ListTemplateVariablesReq{
		BizId:        grpcKit.BizID,
		SearchFields: req.SearchFields,
		SearchValue:  req.SearchValue,
		Start:        req.Start,
		Limit:        req.Limit,
		All:          req.All,
		TopIds:       req.TopIds,
	}

	rp, err := s.client.DS.ListTemplateVariables(grpcKit.RpcCtx(), r)
	if err != nil {
		logs.Errorf("list template variables failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	resp := &pbcs.ListTemplateVariablesResp{
		Count:   rp.Count,
		Details: rp.Details,
	}
	return resp, nil
}

// ImportTemplateVariables import template variables
func (s *Service) ImportTemplateVariables(ctx context.Context, req *pbcs.ImportTemplateVariablesReq) (
	*pbcs.ImportTemplateVariablesResp, error) {
	grpcKit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(grpcKit, res...); err != nil {
		return nil, err
	}

	// validate params
	const whiteSpace string = "white-space"
	if req.Variables == "" {
		return nil, errors.New("variables can't be empty")
	}
	if strings.Contains(req.Separator, "\n") {
		return nil, errors.New("separator can't contain char '\\n'")
	}
	if req.Separator == "" {
		req.Separator = whiteSpace
	}

	vars := make([]*pbtv.TemplateVariableSpec, 0)
	lines := strings.Split(req.Variables, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var fields []string
		if req.Separator == whiteSpace {
			fields = strings.Fields(line)
		} else {
			fields = strings.SplitN(line, req.Separator, 4)
		}

		// validate variables content
		if len(fields) < 3 {
			return nil, fmt.Errorf("the line [%s] is not valid, minimum is 3 fields", line)
		}
		if !strings.HasPrefix(strings.ToLower(fields[0]), constant.TemplateVariablePrefix) {
			return nil, fmt.Errorf("template variable name must start with %s", constant.TemplateVariablePrefix)
		}

		v := &pbtv.TemplateVariableSpec{
			Name:       strings.TrimSpace(fields[0]),
			Type:       strings.TrimSpace(fields[1]),
			DefaultVal: strings.TrimSpace(fields[2]),
		}

		if len(fields) > 3 {
			v.Memo = strings.TrimSpace(strings.Join(fields[3:], " "))
		}

		vars = append(vars, v)
	}

	r := &pbds.ImportTemplateVariablesReq{
		BizId: req.BizId,
		Specs: vars,
	}
	rp, err := s.client.DS.ImportTemplateVariables(grpcKit.RpcCtx(), r)
	if err != nil {
		logs.Errorf("create template variable failed, err: %v, rid: %s", err, grpcKit.Rid)
		return nil, err
	}

	resp := &pbcs.ImportTemplateVariablesResp{
		VariableCount: rp.VariableCount,
		Ids:           rp.Ids,
	}
	return resp, nil
}

// ImportOtherFormatTemplateVariables 导入其他格式的模板变量
func (s *Service) ImportOtherFormatTemplateVariables(ctx context.Context,
	req *pbcs.ImportOtherFormatTemplateVariablesReq) (*pbcs.ImportOtherFormatTemplateVariablesResp, error) {
	kit := kit.FromGrpcContext(ctx)

	res := []*meta.ResourceAttribute{
		{Basic: meta.Basic{Type: meta.Biz, Action: meta.FindBusinessResource}, BizID: req.BizId},
	}
	if err := s.authorizer.Authorize(kit, res...); err != nil {
		return nil, err
	}

	// format：json、yaml
	var variablesMap map[string]interface{}
	switch req.Format {
	case cs.JsonFormat:
		if !json.Valid([]byte(req.GetData())) {
			return nil, errors.New(i18n.T(kit, "not legal JSON data"))
		}
		if err := json.Unmarshal([]byte(req.GetData()), &variablesMap); err != nil {
			return nil, errors.New(i18n.T(kit, "json format error, err: %v", err))
		}
	case cs.YamlFormat:
		if err := yaml.Unmarshal([]byte(req.GetData()), &variablesMap); err != nil {
			return nil, errors.New(i18n.T(kit, "yaml format error, err: %v", err))
		}
	default:
		return nil, errors.New(i18n.T(kit, "%s type not supported", req.Format))
	}

	variables, err := handleVariables(kit, variablesMap)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.DS.ImportTemplateVariables(kit.RpcCtx(), &pbds.ImportTemplateVariablesReq{
		BizId: req.BizId,
		Specs: variables,
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.ImportOtherFormatTemplateVariablesResp{Ids: resp.Ids}, nil
}

func handleVariables(kit *kit.Kit, variables map[string]interface{}) ([]*pbtv.TemplateVariableSpec, error) {

	templateVariables := make([]*pbtv.TemplateVariableSpec, 0)

	for key, value := range variables {
		entry, ok := value.(map[string]interface{})
		if ok {
			variableType, err := checkVariableType(kit, key, entry)
			if err != nil {
				return nil, err
			}

			variableValue, err := checkVariableValue(kit, variableType, key, entry)
			if err != nil {
				return nil, err
			}

			memo, _ := entry["memo"].(string)
			templateVariables = append(templateVariables, &pbtv.TemplateVariableSpec{
				Name:       key,
				Type:       variableType,
				DefaultVal: variableValue,
				Memo:       memo,
			})
		}
	}

	return templateVariables, nil
}

// 检测变量类型
func checkVariableType(kit *kit.Kit, key string, entry map[string]interface{}) (string, error) {

	kvType, ok := entry["variable_type"].(string)
	if !ok {
		return "", errors.New(i18n.T(kit, "template variable %s type error", key))
	}

	if err := validateVariableType(kit, kvType); err != nil {
		return kvType, err
	}

	return kvType, nil
}

// 验证变量类型
func validateVariableType(kit *kit.Kit, variableType string) error {
	switch variableType {
	case string(table.StringVar):
	case string(table.NumberVar):
	case string(table.TextVar):
	default:
		return errf.Errorf(errf.InvalidArgument, i18n.T(kit, "unsupported variable type: %s", variableType))
	}

	return nil
}

// 检测变量的值
func checkVariableValue(kit *kit.Kit, kvType, key string, entry map[string]interface{}) (string, error) {

	kvValue, okVal := entry["value"]
	if !okVal {
		return "", errors.New(i18n.T(kit, "format error, please check the key: %s", key))
	}
	val := fmt.Sprintf("%v", kvValue)
	// json 和 yaml 都需要格式化
	if kvType == string(table.KvJson) {

		v, ok := kvValue.(string)
		if !ok {
			return "", errors.New(i18n.T(kit, "config item %s format error", key))
		}

		mv, err := json.Marshal(v)
		if err != nil {
			return "", errors.New(i18n.T(kit, "config item %s json format error", key))
		}

		// 需要处理转义符
		var data interface{}
		err = json.Unmarshal(mv, &data)
		if err != nil {
			return "", errors.New(i18n.T(kit, "config item %s json format error", key))
		}
		val, ok = data.(string)
		if !ok {
			return "", errors.New(i18n.T(kit, "config item %s format error", key))
		}
	} else if kvType == string(table.KvYAML) {
		_, ok := kvValue.(string)
		if !ok {
			ys, err := yaml.Marshal(kvValue)
			if err != nil {
				return "", errors.New(i18n.T(kit, "config item %s yaml format error", key))
			}
			val = string(ys)
		}
	}

	return val, nil
}
