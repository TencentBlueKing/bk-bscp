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

// Package itsm 在 ITSM 注册服务，包括：创建命名空间、更新命名空间、删除命名空间, 允许重复执行
package itsm

import (
	"context"
	"embed"
	"fmt"

	"github.com/TencentBlueKing/bk-bscp/internal/components/itsm"
	"github.com/TencentBlueKing/bk-bscp/internal/components/itsm/api"
	v4 "github.com/TencentBlueKing/bk-bscp/internal/components/itsm/v4"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
)

var (
	// nolint:unused
	daoSet dao.Set

	// WorkflowTemplates itsm templates
	//go:embed templates
	WorkflowTemplates embed.FS
)

// InitServices 初始化BSCP相关流程服务
func InitServices(ctx context.Context) error {

	// initial DAO set
	set, err := dao.NewDaoSet(cc.DataService().Sharding, cc.DataService().Credential, cc.DataService().Gorm)
	if err != nil {
		return fmt.Errorf("initial dao set failed, err: %v", err)
	}

	daoSet = set

	resp, err := v4.ItsmV4SystemMigrate(ctx)
	if err != nil {
		fmt.Printf("init approve itsm services failed, err: %s\n", err.Error())
		return err
	}

	itsm := itsm.NewITSMService()
	// 通过 workflow_keys 获取 activity_key
	workflow, err := itsm.ListWorkflow(ctx, api.ListWorkflowReq{
		WorkflowKeys: resp.CreateApproveItsmWorkflowID.Value,
	})
	if err != nil {
		fmt.Printf("itsm list workflows failed, err: %s\n", err.Error())
		return err
	}
	// 存入配置表
	itsmConfigs := []*table.Config{
		{
			Key:   constant.CreateApproveItsmWorkflowID,
			Value: resp.CreateApproveItsmWorkflowID.Value,
		}, {
			Key:   constant.CreateCountSignApproveItsmStateID,
			Value: workflow[constant.ItsmApproveCountSignType],
		}, {
			Key:   constant.CreateOrSignApproveItsmStateID,
			Value: workflow[constant.ItsmApproveOrSignType],
		},
	}

	return daoSet.Config().UpsertConfig(kit.New(), itsmConfigs)
}

// InitApproveITSMServices 初始化上线审批相关流程服务
func InitApproveITSMServices() error {
	// kt := kit.New()
	// 2. create itsm catalog
	// catalogID, err := createITSMCatalog(kt.Ctx)
	// if err != nil {
	// 	return err
	// }

	// services, err := itsm.ListServices(kt.Ctx, catalogID)
	// if err != nil {
	// 	return err
	// }

	// 3. import approve services
	// if err := importApproveService(kt, catalogID, services); err != nil {
	// 	return err
	// }
	return nil
}

// func createITSMCatalog(ctx context.Context) (uint32, error) {
// 	catalogs, err := itsm.ListCatalogs(ctx)
// 	if err != nil {
// 		return 0, err
// 	}

// 	var rootID uint32
// 	var parentID uint32
// 	for _, rootCatalog := range catalogs {
// 		if rootCatalog.Key == "root" {
// 			rootID = rootCatalog.ID
// 			for _, parentCatalog := range rootCatalog.Children {
// 				if parentCatalog.Name == "服务配置中心" {
// 					parentID = parentCatalog.ID
// 					for _, catalog := range parentCatalog.Children {
// 						if catalog.Name == "上线审批" {
// 							return catalog.ID, nil
// 						}
// 					}
// 				}
// 			}
// 		}
// 	}
// 	if rootID == 0 {
// 		return 0, fmt.Errorf("root catalog not found")
// 	}
// 	if parentID == 0 {
// 		parentID, err = itsm.CreateCatalog(ctx, itsm.CreateCatalogReq{
// 			ProjectKey: "0",
// 			ParentID:   rootID,
// 			Name:       "服务配置中心",
// 			Desc:       "服务配置中心相关流程",
// 		})
// 		if err != nil {
// 			return 0, err
// 		}
// 	}
// 	// create namespace catalog
// 	catalogID, err := itsm.CreateCatalog(ctx, itsm.CreateCatalogReq{
// 		ProjectKey: "0",
// 		ParentID:   parentID,
// 		Name:       "上线审批",
// 		Desc:       "服务配置上线操作",
// 	})
// 	if err != nil {
// 		return 0, err
// 	}
// 	return catalogID, nil
// }

// func importApproveService(kt *kit.Kit, catalogID uint32, services []itsmv4.Service) error {
// 	// check whether the service has been imported before
// 	// if not, import it, else update it.

// 	var serviceID int
// 	for _, v := range services {
// 		if v.Name == constant.ItsmApproveServiceName {
// 			serviceID = v.ID
// 		}
// 	}

// 	// 自定义模板分隔符为 [[ ]]，例如 [[ .Name ]]，避免和 ITSM 模板变量格式冲突
// 	tmpl, err := template.New("create_shared_approve.json.tpl").Delims("[[", "]]").
// 		ParseFS(WorkflowTemplates, "templates/create_shared_approve.json.tpl")
// 	if err != nil {
// 		return err
// 	}
// 	stringBuffer := &strings.Builder{}
// 	if err = tmpl.Execute(stringBuffer, map[string]string{
// 		"BCSPGateway": cc.DataService().ITSM.BscpGateway,
// 		"BkAppCode":   cc.DataService().Esb.AppCode,
// 		"BkAppSecret": cc.DataService().Esb.AppSecret,
// 	}); err != nil {
// 		return err
// 	}
// 	mp := map[string]interface{}{}
// 	if err = json.Unmarshal([]byte(stringBuffer.String()), &mp); err != nil {
// 		return err
// 	}
// 	importReq := itsm.ImportServiceReq{
// 		Key:             "request",
// 		Name:            constant.ItsmApproveServiceName,
// 		Desc:            constant.ItsmApproveServiceName,
// 		CatelogID:       catalogID,
// 		Owners:          "admin",
// 		CanTicketAgency: false,
// 		IsValid:         true,
// 		DisplayType:     "OPEN",
// 		DisplayRole:     "",
// 		Source:          "custom",
// 		ProjectKey:      "0",
// 		Workflow:        mp,
// 	}

// 	// 在itsm不存在
// 	if serviceID == 0 {
// 		serviceID, err = itsm.ImportService(kt.Ctx, importReq)
// 		if err != nil {
// 			return err
// 		}
// 	} else {
// 		err = itsm.UpdateService(kt.Ctx, itsm.UpdateServiceReq{
// 			ID:               serviceID,
// 			ImportServiceReq: importReq,
// 		})
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	workflowId, err := itsmv4.GetWorkflowByService(kt.Ctx, serviceID)
// 	if err != nil {
// 		return err
// 	}

// 	stateApproveId, err := itsm.GetStateApproveByWorkfolw(kt.Ctx, workflowId)
// 	if err != nil {
// 		return err
// 	}

// itsmConfigs := []*table.Config{
// 	{
// 		Key:   constant.CreateApproveItsmServiceID,
// 		Value: strconv.Itoa(serviceID),
// 	}, {
// 		Key:   constant.CreateApproveItsmWorkflowID,
// 		Value: strconv.Itoa(workflowId),
// 	}, {
// 		Key:   constant.CreateCountSignApproveItsmStateID,
// 		Value: strconv.Itoa(stateApproveId[constant.ItsmApproveCountSignType]),
// 	}, {
// 		Key:   constant.CreateOrSignApproveItsmStateID,
// 		Value: strconv.Itoa(stateApproveId[constant.ItsmApproveOrSignType]),
// 	},
// }
// return daoSet.Config().UpsertConfig(kt, itsmConfigs)
// }
