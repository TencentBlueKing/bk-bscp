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
)

// TenantWorkflowData tenant workflow data
type TenantWorkflowData struct {
	CreateApproveItsmServiceID        *table.Config
	CreateApproveItsmWorkflowID       *table.Config
	CreateCountSignApproveItsmStateID *table.Config
	CreateOrSignApproveItsmStateID    *table.Config
}

// ItsmV4TemplateRender xxx
type ItsmV4TemplateRender struct {
	ServiceID  string `json:"ServiceID"`
	WorkflowID string `json:"WorkflowID"`
}

func generateTemplateId(tenant string, systemCode string, category string) string {
	return fmt.Sprintf("%s_%s_%s", tenant, systemCode, category)
}

// ItsmV4SystemMigrate 初始化模板
func ItsmV4SystemMigrate(ctx context.Context) (*TenantWorkflowData, error) {
	kit := kit.FromGrpcContext(ctx)
	tenantId := kit.TenantID
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
		ServiceID:  generateTemplateId(tenantId, systemCode, "formModel"),
		WorkflowID: generateTemplateId(tenantId, systemCode, "workflowCategory"),
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, values)
	if err != nil {
		return nil, err
	}
	content := buf.String()

	err = MigrateSystem(ctx, []byte(content))
	if err != nil {
		return nil, err
	}

	return &TenantWorkflowData{
		CreateApproveItsmServiceID: &table.Config{
			Key:   fmt.Sprintf("%s-%s", tenantId, constant.CreateApproveItsmServiceID),
			Value: values.ServiceID,
		},
		CreateApproveItsmWorkflowID: &table.Config{
			Key:   fmt.Sprintf("%s-%s", tenantId, constant.CreateApproveItsmWorkflowID),
			Value: values.WorkflowID,
		},
		CreateCountSignApproveItsmStateID: &table.Config{
			Key: fmt.Sprintf("%s-%s", tenantId, constant.CreateCountSignApproveItsmStateID),
		},
		CreateOrSignApproveItsmStateID: &table.Config{
			Key: fmt.Sprintf("%s-%s", tenantId, constant.CreateOrSignApproveItsmStateID),
		},
	}, nil

}

// GetAuthHeader 获取蓝鲸网关通用认证头
func GetAuthHeader(ctx context.Context) map[string]string {
	kit := kit.FromGrpcContext(ctx)

	return map[string]string{
		"Content-Type": "application/json",
		"X-Bkapi-Authorization": fmt.Sprintf(`{"bk_app_code": "%s", "bk_app_secret": "%s", "bk_username": "%s"}`,
			cc.DataService().Esb.AppCode, cc.DataService().Esb.AppSecret, cc.DataService().Esb.User),
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
