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

// Package bkcmdb provides bkcmdb client.
package bkcmdb

import (
	"context"
	"os"
	"testing"

	"github.com/davecgh/go-spew/spew"

	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
)

// getCMDBConfig 从环境变量获取 CMDB 配置
// 需要设置以下环境变量：
//   - APP_CODE: CMDB 应用代码（可选，默认为 "bk-bscp"）
//   - APP_SECRET: CMDB 应用密钥（可选，默认为空）
//   - HOST: CMDB 服务地址（如果为空，测试会跳过）
//   - BK_USER_NAME: CMDB 用户名（可选，默认为空）
//   - USE_ESB: 是否使用 ESB（可选，默认为 false）
func getCMDBConfig(t *testing.T) *cc.CMDBConfig {
	appCode := os.Getenv("APP_CODE")
	if appCode == "" {
		appCode = "bk-bscp" // 默认值
	}

	appSecret := os.Getenv("APP_SECRET")
	cmdbHost := os.Getenv("CMDB_HOST")
	bkUserName := os.Getenv("BK_USER_NAME")
	useEsb := os.Getenv("USE_ESB")

	// 设置GobalSettings中的CMDBConfig
	cc.SetG(cc.GlobalSettings{
		FeatureFlags: cc.FeatureFlags{
			EnableMultiTenantMode: false,
		},
	})

	// 如果 Host 为空，跳过测试（因为无法进行真实的 API 调用）
	if cmdbHost == "" {
		t.Skip("Skipping test: HOST environment variable not set")
	}
	t.Logf("CMDB_HOST: %s, appCode: %s, appSecret: %s, bkUserName: %s, useEsb: %s", cmdbHost, appCode, appSecret, bkUserName, useEsb)

	cfg := &cc.CMDBConfig{
		AppCode:    appCode,
		AppSecret:  appSecret,
		Host:       cmdbHost,
		BkUserName: bkUserName,
		UseEsb:     useEsb == "true",
	}

	return cfg
}

func TestFindHostByTopo(t *testing.T) {
	cmdb, err := New(getCMDBConfig(t), nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.FindHostByTopo(context.Background(), HostListReq{
		BkBizID:  2,
		BkObjID:  "module",
		BkInstID: 2,
		Fields:   []string{"bk_host_id", "bk_host_name", "bk_host_innerip", "bk_cloud_id"},
		Page: &PageParam{
			Start: 0,
			Limit: 20,
		},
	})

	if err != nil {
		t.Fatalf("FindHostByTopo error: %v", err)
	}

	// 打印结果
	t.Logf("结果: %v", spew.Sdump(resp))
}

func TestSearchBizInstTopo(t *testing.T) {
	cmdb, err := New(getCMDBConfig(t), nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.SearchBizInstTopo(context.Background(), &BizTopoReq{
		BkBizID: 2,
	})

	if err != nil {
		t.Fatalf("FindHostByTopo error: %v", err)
	}
	// 打印结果
	t.Logf("结果: %v", spew.Sdump(resp))
}

func TestGetServiceTemplate(t *testing.T) {
	cmdb, err := New(getCMDBConfig(t), nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.GetServiceTemplate(context.Background(), ServiceTemplateReq{
		ServiceTemplateID: 10,
	})

	if err != nil {
		t.Fatalf("GetServiceTemplate error: %v", err)
	}
	// 打印结果
	t.Logf("结果: %v", spew.Sdump(resp))
}

func TestListServiceTemplate(t *testing.T) {
	cmdb, err := New(getCMDBConfig(t), nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.ListServiceTemplate(context.Background(), &ListServiceTemplateReq{
		BkBizID: 2,
		// ServiceCategoryID:  0,
		// Search:             "",
		// IsExact:            false,
		// ServiceTemplateIDs: []int{},
		Page: &PageParam{
			Start: 0,
			Limit: 20,
		},
	})

	if err != nil {
		t.Fatalf("ListServiceTemplate error: %v", err)
	}
	// 打印结果
	t.Logf("结果: %v", spew.Sdump(resp))
}

func TestGetProcTemplate(t *testing.T) {
	cmdb, err := New(getCMDBConfig(t), nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.GetProcTemplate(context.Background(), GetProcTemplateReq{
		// BkBizID:           2,
		ProcessTemplateID: 1,
	})

	if err != nil {
		t.Fatalf("GetProcTemplate error: %v", err)
	}
	// 打印结果
	t.Logf("结果: %v", spew.Sdump(resp))
}

func TestListProcTemplate(t *testing.T) {
	cmdb, err := New(getCMDBConfig(t), nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.ListProcTemplate(context.Background(), &ListProcTemplateReq{
		BkBizID:           2,
		ServiceTemplateID: 9,
		// ProcessTemplateID: 1,
	})

	if err != nil {
		t.Fatalf("ListProcTemplate error: %v", err)
	}
	// 打印结果
	t.Logf("结果: %v", spew.Sdump(resp))
}

func TestFindHostBySetTemplate(t *testing.T) {
	cmdb, err := New(getCMDBConfig(t), nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.FindHostBySetTemplate(context.Background(), FindHostBySetTemplateReq{
		BkBizID:          3,
		BkSetTemplateIDs: []int{2},
		Fields: []string{"bk_host_id",
			"bk_host_name",
			"bk_host_innerip",
			"bk_cloud_id"},
		Page: &PageParam{
			Start: 0,
			Limit: 100,
		},
	})

	if err != nil {
		t.Fatalf("FindHostBySetTemplate error: %v", err)
	}
	// 打印结果
	t.Logf("结果: %v", spew.Sdump(resp))
}

func TestListSetTemplate(t *testing.T) {
	cmdb, err := New(getCMDBConfig(t), nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.ListSetTemplate(context.Background(), ListSetTemplateReq{
		BkBizID:        2,
		SetTemplateIDs: []int{},
		Page: &PageParam{
			Start: 0,
			Limit: 20,
		},
	})

	if err != nil {
		t.Fatalf("ListSetTemplate error: %v", err)
	}
	// 打印结果
	t.Logf("结果: %v", spew.Sdump(resp))
}

func TestListProcessDetailByIds(t *testing.T) {
	cmdb, err := New(getCMDBConfig(t), nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.ListProcessDetailByIds(context.Background(), ProcessReq{
		BkBizID:      2,
		BkProcessIDs: []int{},
		Fields:       []string{},
	})

	if err != nil {
		t.Fatalf("ListProcessDetailByIds error: %v", err)
	}
	// 打印结果
	t.Logf("结果: %v", spew.Sdump(resp))
}

func TestListServiceInstanceBySetTemplate(t *testing.T) {
	cmdb, err := New(getCMDBConfig(t), nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.ListServiceInstanceBySetTemplate(context.Background(), ServiceInstanceReq{
		BkBizID:       2,
		SetTemplateID: 0,
		Page: &PageParam{
			Start: 0,
			Limit: 20,
		},
	})

	if err != nil {
		t.Fatalf("ListServiceInstanceBySetTemplate error: %v", err)
	}
	// 打印结果
	t.Logf("结果: %v", spew.Sdump(resp))
}

func TestFindModuleBatch(t *testing.T) {
	cmdb, err := New(getCMDBConfig(t), nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.FindModuleBatch(context.Background(), &ModuleReq{
		BkBizID: 0,
		BkIDs:   []int{},
		Fields:  []string{},
	})

	if err != nil {
		t.Fatalf("FindModuleBatch error: %v", err)
	}
	// 打印结果
	t.Logf("结果: %v", spew.Sdump(resp))
}

func TestListServiceInstance(t *testing.T) {
	cmdb, err := New(getCMDBConfig(t), nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.ListServiceInstance(context.Background(), &ServiceInstanceListReq{
		BkBizID:    3,
		BkModuleID: 0,
		BkHostIDs:  []int{},
		Selectors:  []Selector{},
		Page: &PageParam{
			Start: 0,
			Limit: 20,
		},
		SearchKey: "",
	})

	if err != nil {
		t.Fatalf("ListServiceInstance error: %v", err)
	}
	// 打印结果
	t.Logf("结果: %v", spew.Sdump(resp))
}

func TestFindSetBatch(t *testing.T) {
	cmdb, err := New(getCMDBConfig(t), nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	setInfos, err := cmdb.FindSetBatch(context.Background(), SetListReq{
		BkBizID: 2,
		BkIDs:   []int{},
		Fields:  []string{"bk_set_id", "bk_set_name", "bk_set_env", "bk_biz_id"},
	})

	if err != nil {
		t.Fatalf("FindSetBatch error: %v", err)
	}
	// 打印结果
	t.Logf("结果: %v", spew.Sdump(setInfos))
}

func TestFindHostTopoRelation(t *testing.T) {
	cmdb, err := New(getCMDBConfig(t), nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.FindHostTopoRelation(context.Background(), &HostTopoReq{
		BkBizID:     2,
		BkSetIDs:    []int{},
		BkModuleIDs: []int{},
		BkHostIDs:   []int{},
		Page: &PageParam{
			Start: 0,
			Limit: 20,
		},
	})

	if err != nil {
		t.Fatalf("FindHostTopoRelation error: %v", err)
	}
	// 打印结果
	t.Logf("结果: %v", spew.Sdump(resp))
}

func TestFindModuleWithRelation(t *testing.T) {
	cmdb, err := New(getCMDBConfig(t), nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.FindModuleWithRelation(context.Background(), ModuleListReq{
		BkBizID:              2,
		BkSetIDs:             []int{},
		BkServiceTemplateIDs: []int{},
		Fields:               []string{"bk_module_id", "bk_module_name", "bk_set_id", "bk_biz_id"},
		Page: &PageParam{
			Start: 0,
			Limit: 20,
		},
	})

	if err != nil {
		t.Fatalf("FindModuleWithRelation error: %v", err)
	}
	// 打印结果
	t.Logf("结果: %v", spew.Sdump(resp))
}

func TestSearchSet(t *testing.T) {
	cmdb, err := New(getCMDBConfig(t), nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.SearchSet(context.Background(), SearchSetReq{
		BkSupplierAccount: "0",
		BkBizID:           2,
		Fields:            []string{"bk_set_id", "bk_set_name", "bk_set_env", "bk_biz_id"},
		Page: &PageParam{
			Start: 0,
			Limit: 20,
		},
	})

	if err != nil {
		t.Fatalf("SearchSet error: %v", err)
	}
	// 打印结果
	t.Logf("结果: %v", spew.Sdump(resp))
}

func TestFindTopoBrief(t *testing.T) {
	cmdb, err := New(getCMDBConfig(t), nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.FindTopoBrief(context.Background(), 2)
	if err != nil {
		t.Fatalf("FindTopoBrief error: %v", err)
	}

	// 打印结果
	t.Logf("结果: %v", spew.Sdump(resp))
}

func TestSearchObjectAttr(t *testing.T) {
	cmdb, err := New(getCMDBConfig(t), nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.SearchObjectAttr(context.Background(), SearchObjectAttrReq{
		BkObjID: "module",
		BkBizID: 2,
	})
	if err != nil {
		t.Fatalf("SearchObjectAttr error: %v", err)
	}

	// 打印结果
	t.Logf("结果: %v", spew.Sdump(resp))
}
