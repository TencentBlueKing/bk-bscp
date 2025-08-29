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

// Package crontab example Synchronize the online status of the client
package crontab

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/metadata"

	"github.com/TencentBlueKing/bk-bscp/internal/components/itsm"
	"github.com/TencentBlueKing/bk-bscp/internal/components/itsm/api"
	v4 "github.com/TencentBlueKing/bk-bscp/internal/components/itsm/v4"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/shutdown"
	"github.com/TencentBlueKing/bk-bscp/internal/serviced"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

const (
	defaultRegisterTenantTemplatesInterval = 20 * time.Second
)

// RegisterTenantTemplates register tenant templates
func RegisterTenantTemplates(set dao.Set, sd serviced.Service) ItsmTenantRegistry {
	return ItsmTenantRegistry{
		set:   set,
		state: sd,
		itsm:  itsm.NewITSMService(),
	}
}

type ItsmTenantRegistry struct {
	set   dao.Set
	state serviced.Service
	mutex sync.Mutex
	itsm  itsm.Service
}

func (i *ItsmTenantRegistry) Run() {
	logs.Infof("start itsm multi-tenant template registration")

	notifier := shutdown.AddNotifier()
	go func() {
		ticker := time.NewTicker(defaultRegisterTenantTemplatesInterval)
		defer ticker.Stop()
		for {
			kt := kit.New()
			ctx, cancel := context.WithCancel(kt.Ctx)
			kt.Ctx = ctx

			select {
			case <-notifier.Signal:
				logs.Infof("stop itsm multi-tenant template registration")
				cancel()
				notifier.Done()
				return
			case <-ticker.C:
				if !i.state.IsMaster() {
					logs.Infof("current service instance is slave, skip itsm multi-tenant template registration")
					continue
				}

				i.registerTenantTemplates(kt)
			}
		}
	}()
}

func (i *ItsmTenantRegistry) registerTenantTemplates(kt *kit.Kit) {
	i.mutex.Lock()
	defer func() {
		i.mutex.Unlock()
	}()

	// 获取租户ID
	tenantIDs, err := i.set.App().GetDistinctTenantIDs(kt)
	if err != nil {
		logs.Errorf("get the tenant ID list. failed, err: %s", err.Error())
		return
	}

	if len(tenantIDs) == 0 {
		return
	}

	keys := []string{}
	for _, v := range tenantIDs {
		if v.Spec.TenantID == "" {
			continue
		}
		keys = append(keys, fmt.Sprintf("%s-%s", v.Spec.TenantID, constant.CreateApproveItsmWorkflowID))
	}

	// 通过租户ID获取已经注册的租户
	itsmConfigs, err := i.set.Config().ListConfigByKeys(kt, keys)
	if err != nil {
		logs.Errorf("get the configuration list by %v failed, err: %s", keys, err.Error())
		return
	}
	// 过滤没有注册的租户
	missing := diffKeys(keys, itsmConfigs)

	for _, v := range missing {
		// 去掉后缀，只保留 TenantID 部分
		tenantID := strings.TrimSuffix(v, "-"+constant.CreateApproveItsmWorkflowID)

		md := metadata.MD{
			strings.ToLower(constant.BkTenantID): []string{tenantID},
		}
		ctx := metadata.NewIncomingContext(kt.Ctx, md)

		resp, err := v4.ItsmV4SystemMigrate(ctx)
		if err != nil {
			logs.Errorf("init approve itsm services failed, err: %s\n", err.Error())
			return
		}

		workflow, err := i.itsm.ListWorkflow(ctx, api.ListWorkflowReq{
			WorkflowKeys: resp.CreateApproveItsmWorkflowID.Value,
		})
		if err != nil {
			logs.Errorf("itsm list workflows failed, err: %s\n", err.Error())
			return
		}
		// 存入配置表
		itsmConfigs := []*table.Config{
			{
				Key:   fmt.Sprintf("%s-%s", tenantID, constant.CreateApproveItsmWorkflowID),
				Value: resp.CreateApproveItsmWorkflowID.Value,
			}, {
				Key:   fmt.Sprintf("%s-%s", tenantID, constant.CreateCountSignApproveItsmStateID),
				Value: workflow[constant.ItsmApproveCountSignType],
			}, {
				Key:   fmt.Sprintf("%s-%s", tenantID, constant.CreateOrSignApproveItsmStateID),
				Value: workflow[constant.ItsmApproveOrSignType],
			},
		}
		if err = i.set.Config().UpsertConfig(kt, itsmConfigs); err != nil {
			logs.Errorf("itsm multi-tenant template registration failed, err: %s\n", err.Error())
			return
		}
	}
}

// diffKeys 返回 keys 中存在但 itsmConfigs 中不存在的 key
func diffKeys(keys []string, itsmConfigs []*table.Config) []string {
	exist := make(map[string]struct{}, len(itsmConfigs))
	for _, c := range itsmConfigs {
		exist[c.Key] = struct{}{}
	}

	missing := make([]string, 0)
	for _, k := range keys {
		if _, ok := exist[k]; !ok {
			missing = append(missing, k)
		}
	}
	return missing
}
