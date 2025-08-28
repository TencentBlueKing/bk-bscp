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

// Package itsmv4 xxx
package v4

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"text/template"

	"github.com/TencentBlueKing/bk-bscp/internal/components"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/tools"
)

// TenantWorkflowData tenant workflow data
type TenantWorkflowData struct {
	CreateApproveItsmWorkflowID *table.Config
}

// ItsmV4TemplateRender xxx
type ItsmV4TemplateRender struct {
	FormModel        string `json:"FormModel"`
	WorkflowCategory string `json:"WorkflowCategory"`
	Workflow         string `json:"Workflow"`
}

const (
	formModel        = "formmodel"
	workflowCategory = "workflowcategory"
	workflow         = "workflow"
)

func generateTemplateId(tenant string, systemCode string, category string) string {
	return tools.RandomString(fmt.Sprintf("%s_%s_%s_", tenant, systemCode, category), 8)
}

// ItsmV4SystemMigrate 初始化模板
func ItsmV4SystemMigrate(ctx context.Context, tenantID string) (*TenantWorkflowData, error) {
	ctx = context.WithValue(ctx, constant.BkTenantID, tenantID) // nolint: staticcheck
	// 读取模板文件内容
	templateContent, err := os.ReadFile(migrateItsm)
	if err != nil {
		return nil, err
	}
	// 解析模板
	tmpl, err := template.New("json").Parse(string(templateContent))
	if err != nil {
		return nil, err
	}

	values := ItsmV4TemplateRender{
		FormModel:        generateTemplateId(tenantID, systemCode, formModel),
		WorkflowCategory: generateTemplateId(tenantID, systemCode, workflowCategory),
		Workflow:         generateTemplateId(tenantID, systemCode, workflow),
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, values)
	if err != nil {
		return nil, err
	}
	content := buf.String()

	if err = MigrateSystem(ctx, []byte(content), tenantID); err != nil {
		return nil, err
	}

	return &TenantWorkflowData{
		CreateApproveItsmWorkflowID: &table.Config{
			Key:   fmt.Sprintf("%s-%s", tenantID, constant.CreateApproveItsmWorkflowID),
			Value: values.Workflow,
		},
	}, nil
}

// GetAuthHeader 获取蓝鲸网关通用认证头
func GetAuthHeader(ctx context.Context) map[string]string {
	kit := kit.FromGrpcContext(ctx)
	if len(kit.TenantID) == 0 {
		// 尝试直接从 ctx.Value 取
		if v := ctx.Value(constant.BkTenantID); v != nil {
			if s, ok := v.(string); ok {
				kit.TenantID = s
			}
		}
	}

	return map[string]string{
		"Content-Type": "application/json",
		"X-Bkapi-Authorization": fmt.Sprintf(`{"bk_app_code": "%s", "bk_app_secret": "%s", "bk_username": "%s"}`,
			cc.DataService().ITSM.AppCode, cc.DataService().ITSM.AppSecret, cc.DataService().ITSM.User),
		constant.BkTenantID: kit.TenantID,
	}
}

// ItsmRequest itsm request
func ItsmRequest(ctx context.Context, method, reqURL string, data any) ([]byte, error) {

	client := components.GetClient().R().
		SetContext(ctx).
		SetHeaders(GetAuthHeader(ctx))

	switch method {
	case http.MethodGet:
		resp, err := client.Get(reqURL)
		if err != nil {
			return nil, err
		}
		return resp.Body(), nil
	case http.MethodPost:
		resp, err := client.SetBody(data).Post(reqURL)
		if err != nil {
			return nil, err
		}
		return resp.Body(), nil
	default:
		return nil, fmt.Errorf("invalid method: %s", method)
	}
}
