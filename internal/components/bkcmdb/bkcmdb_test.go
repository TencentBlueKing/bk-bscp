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
	"fmt"
	"testing"

	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/davecgh/go-spew/spew"
)

var cfg *cc.CMDBConfig

func init() {
	cfg = &cc.CMDBConfig{
		AppCode: "bk-bscp", AppSecret: "AWy9VhpNoEQ4U6STq3p5XRVxWcuOOceTTEVh", Host: "http://bkapi.sit.bktencent.com", UseEsb: false,
	}
}

func TestFindHostByTopo(t *testing.T) {
	cmdb, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.FindHostByTopo(context.Background(), HostListReq{
		BkBizID:  2,
		BkObjID:  "module",
		BkInstID: 2,
		// Fields:   []string{},
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
	cmdb, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.SearchBizInstTopo(context.Background(), BizTopoReq{
		BkBizID: 2,
	})

	if err != nil {
		t.Fatalf("FindHostByTopo error: %v", err)
	}
	// 打印结果
	t.Logf("结果: %v", spew.Sdump(resp))
}

func TestGetServiceTemplate(t *testing.T) {
	cmdb, err := New(cfg, nil)
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
	cmdb, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.ListServiceTemplate(context.Background(), ListServiceTemplateReq{
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
	cmdb, err := New(cfg, nil)
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
	cmdb, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.ListProcTemplate(context.Background(), ListProcTemplateReq{
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
	cmdb, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.FindHostBySetTemplate(context.Background(), FindHostBySetTemplateReq{
		BkBizID:          2,
		BkSetTemplateIDs: []int{},
		BkSetIDs:         []int{},
		Fields:           []string{},
		Page: &PageParam{
			Start: 0,
			Limit: 20,
		},
	})

	if err != nil {
		t.Fatalf("FindHostBySetTemplate error: %v", err)
	}
	// 打印结果
	t.Logf("结果: %v", spew.Sdump(resp))
}

func TestListSetTemplate(t *testing.T) {
	cmdb, err := New(cfg, nil)
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
	cmdb, err := New(cfg, nil)
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
	cmdb, err := New(cfg, nil)
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
	cmdb, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.FindModuleBatch(context.Background(), ModuleReq{
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
	cmdb, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.ListServiceInstance(context.Background(), ServiceInstanceListReq{
		BkBizID:    2,
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
	cmdb, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.FindSetBatch(context.Background(), SetListReq{
		BkBizID: 2,
		BkIDs:   []int{},
		Fields:  []string{},
	})

	if err != nil {
		t.Fatalf("FindSetBatch error: %v", err)
	}
	// 打印结果
	t.Logf("结果: %v", spew.Sdump(resp))
}

func TestFindHostTopoRelation(t *testing.T) {
	cmdb, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.FindHostTopoRelation(context.Background(), HostTopoReq{
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
	cmdb, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.FindModuleWithRelation(context.Background(), ModuleListReq{
		BkBizID:              2,
		BkSetIDs:             []int{},
		BkServiceTemplateIDs: []int{},
		Fields:               []string{},
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
	cmdb, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	resp, err := cmdb.SearchSet(context.Background(), SearchSetReq{
		BkSupplierAccount: "0",
		BkBizID:           2,
		Fields:            []string{},
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

func TestSynyCC(t *testing.T) {
	cmdb, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	// biz, err := cmdb.SearchBusinessByAccount(context.Background(), SearchSetReq{
	// 	BkSupplierAccount: "0",
	// })

	// if err != nil {
	// 	t.Fatalf("SearchBusinessByAccount error: %v", err)
	// }
	// var business Business
	// if err := biz.Decode(&business); err != nil {
	// 	panic(err)
	// }

	// for _, v := range business.Info {
	// 	fmt.Println(v.BkBizName, v.BkBizID)
	// }

	// setInfo, err := cmdb.SearchSet(context.Background(), SearchSetReq{
	// 	BkSupplierAccount: "0",
	// 	BkBizID:           3,
	// })

	// if err != nil {
	// 	t.Fatalf("SearchSet error: %v", err)
	// }

	// var set Sets

	// if err := setInfo.Decode(&set); err != nil {
	// 	panic(err)
	// }

	// for _, v := range set.Info {
	// 	fmt.Println(v.BkSetID, v.SetTemplateID, v.BkSetName) // 19 2 demo
	// }

	// searchModule, err := cmdb.SearchModule(context.Background(), SearchModuleReq{
	// 	BkSupplierAccount: "0",
	// 	BkBizID:           3,
	// 	BkSetID:           19,
	// })

	// if err != nil {
	// 	t.Fatalf("SearchSet error: %v", err)
	// }

	// var mod ModuleListResp

	// if err := searchModule.Decode(&mod); err != nil {
	// 	panic(err)
	// }

	// for _, v := range mod.Info {
	// 	fmt.Println(v.BkModuleName, v.BkModuleID) // node  52
	// }

	// host, err := cmdb.FindHostBySetTemplate(context.Background(), FindHostBySetTemplateReq{
	// 	BkBizID:          3,
	// 	BkSetTemplateIDs: []int{2},
	// 	BkSetIDs:         []int{19},
	// 	Fields: []string{
	// 		"bk_host_id",
	// 		"bk_cloud_id",
	// 		"bk_host_name",
	// 		"bk_host_innerip",
	// 	},
	// 	Page: &PageParam{
	// 		Start: 0,
	// 		Limit: 20,
	// 	},
	// })
	// if err != nil {
	// 	t.Fatalf("FindHostBySetTemplate error: %v", err)
	// }

	// var hosts FindHostBySetTemplateResp

	// if err := host.Decode(&hosts); err != nil {
	// 	panic(err)
	// }

	// for _, v := range hosts.Info {
	// 	fmt.Println(v.BkHostID, v.BkHostName, v.BkHostInnerIP)
	// 	// 746  172.16.20.3
	// 	// 947  10.0.0.151
	// }

	serviceInstance, err := cmdb.ListServiceInstance(context.Background(), ServiceInstanceListReq{
		BkBizID: 3,
		// BkModuleID: 52,
		BkHostIDs: []int{},
		Selectors: []Selector{},
		Page: &PageParam{
			Start: 0,
			Limit: 20,
		},
		SearchKey: "",
	})
	if err != nil {
		t.Fatalf("ListServiceInstance error: %v", err)
	}

	var serviceInstances ServiceInstanceResp

	if err := serviceInstance.Decode(&serviceInstances); err != nil {
		panic(err)
	}

	fmt.Println("serviceInstances:", serviceInstances.Count)

	for _, v := range serviceInstances.Info {
		fmt.Println(v.ID, v.Name)
		// 1 10.0.0.13_nginx-test 11
		// 2 10.0.0.151_test1 3
		// 3 172.16.20.3_test1 3
	}

	process, err := cmdb.ListProcessInstance(context.Background(), ListProcessInstanceReq{
		BkBizID:           3,
		ServiceInstanceID: 3,
	})
	if err != nil {
		t.Fatalf("ListProcessInstance error: %v", err)
	}

	var procs []ListProcessInstance

	if err := process.Decode(&procs); err != nil {
		panic(err)
	}

	for _, v := range procs {
		fmt.Println(v.Property.BkProcessName, v.Property.BkProcessID)
		// 1 10.0.0.13_nginx-test 11
		// 2 10.0.0.151_test1 3
		// 3 172.16.20.3_test1 3
	}

}
