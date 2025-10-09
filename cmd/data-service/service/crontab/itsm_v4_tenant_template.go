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
	v4 "github.com/TencentBlueKing/bk-bscp/internal/components/itsm/v4"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/shutdown"
	"github.com/TencentBlueKing/bk-bscp/internal/serviced"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

const (
	defaultRegisterItsmV4TemplatesInterval = 20 * time.Second
)

// RegisterItsmV4Templates register itsm v4 templates
func RegisterItsmV4Templates(set dao.Set, sd serviced.Service) ItsmItsmV4Registry {
	return ItsmItsmV4Registry{
		set:   set,
		state: sd,
		itsm:  itsm.NewITSMService(),
	}
}

type ItsmItsmV4Registry struct {
	set   dao.Set
	state serviced.Service
	mutex sync.Mutex
	itsm  itsm.Service
}

func (i *ItsmItsmV4Registry) Run() {
	logs.Infof("start itsm itsm v4 template registration")
	// 单租户模式只跑一次
	notifier := shutdown.AddNotifier()
	go func() {
		ticker := time.NewTicker(defaultRegisterItsmV4TemplatesInterval)
		defer ticker.Stop()
		for {
			kt := kit.New()
			ctx, cancel := context.WithCancel(kt.Ctx)
			kt.Ctx = ctx

			select {
			case <-notifier.Signal:
				logs.Infof("stop itsm itsm v4 template registration")
				cancel()
				notifier.Done()
				return
			case <-ticker.C:
				if !i.state.IsMaster() {
					logs.Infof("current service instance is slave, skip itsm itsm v4 template registration")
					continue
				}

				i.registerItsmV4Templates(kt)
			}
		}
	}()
}

func (i *ItsmItsmV4Registry) registerItsmV4Templates(kt *kit.Kit) {
	i.mutex.Lock()
	defer func() {
		i.mutex.Unlock()
	}()

	tenantIDs := []string{}
	if cc.DataService().FeatureFlags.EnableMultiTenantMode {
		// 获取租户ID
		apps, err := i.set.App().GetDistinctTenantIDs(kt)
		if err != nil {
			logs.Errorf("get the tenant ID list. failed, err: %s", err.Error())
			return
		}
		if len(apps) == 0 {
			logs.Warnf("tenant list is empty")
			return
		}
		for _, v := range apps {
			// 多租户模式下就不允许租户ID为空
			if v.Spec.TenantID == "" {
				continue
			}
			tenantIDs = append(tenantIDs, v.Spec.TenantID)
		}
	} else {
		// 单租户模式，租户ID为空
		tenantIDs = append(tenantIDs, "")
	}

	keys := []string{}
	for _, v := range tenantIDs {
		prefix := ""
		if v != "" {
			prefix = fmt.Sprintf("%s-", v)
		}
		keys = append(keys, fmt.Sprintf("%s%s", prefix, constant.CreateApproveItsmWorkflowID))
	}

	// 获取已经注册的模板
	itsmConfigs, err := i.set.Config().ListConfigByKeys(kt, keys)
	if err != nil {
		logs.Errorf("get the configuration list by %v failed, err: %s", keys, err.Error())
		return
	}
	// 过滤还没有注册的
	missing := diffKeys(keys, itsmConfigs)

	for _, v := range missing {
		// 去掉后缀，只保留 TenantID 部分(兼容为空的情况)
		tenantID := strings.TrimSuffix(v, constant.CreateApproveItsmWorkflowID)
		tenantID = strings.TrimSuffix(tenantID, "-")
		md := metadata.MD{}
		prefix := ""
		if tenantID != "" {
			md.Set(strings.ToLower(constant.BkTenantID), tenantID)
			prefix = fmt.Sprintf("%s-", tenantID)
		}

		ctx := metadata.NewIncomingContext(kt.Ctx, md)
		resp, err := v4.ItsmV4SystemMigrate(ctx)
		if err != nil {
			logs.Errorf("ItsmV4SystemMigrate failed, err: %s\n", err.Error())
			return
		}

		workflow, err := v4.ListWorkflow(ctx, v4.ListWorkflowReq{
			WorkflowKeys: resp.CreateApproveItsmWorkflowID.Value,
		})
		if err != nil {
			logs.Errorf("itsm list workflows failed, err: %s\n", err.Error())
			return
		}
		// 存入配置表(如果有租户则以租户为前缀)
		itsmConfigs := []*table.Config{
			{
				Key:   fmt.Sprintf("%s%s", prefix, constant.CreateApproveItsmWorkflowID),
				Value: resp.CreateApproveItsmWorkflowID.Value,
			}, {
				Key:   fmt.Sprintf("%s%s", prefix, constant.CreateCountSignApproveItsmStateID),
				Value: workflow[constant.ItsmApproveCountSignType],
			}, {
				Key:   fmt.Sprintf("%s%s", prefix, constant.CreateOrSignApproveItsmStateID),
				Value: workflow[constant.ItsmApproveOrSignType],
			},
		}
		if err = i.set.Config().UpsertConfig(kt, itsmConfigs); err != nil {
			logs.Errorf("itsm itsm v4 template registration failed, err: %s\n", err.Error())
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
