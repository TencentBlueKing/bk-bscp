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

// Package pushmanager provides bcs push manager api client.
package pushmanager

import (
	"context"

	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
)

// New push manager service
func New(cfg cc.PushProvider) (Service, error) {
	if cfg.Type == "bcs-push-manager" {
		// 从 BCS 全局配置中补齐 push manager 依赖的参数
		if cfg.Config.Host == "" {
			cfg.Config.Host = cc.G().BCS.Host
		}
		if cfg.Config.Token == "" {
			cfg.Config.Token = cc.G().BCS.Token
		}
	}

	if err := cfg.ValidateBCSPPushManager(); err != nil {
		return nil, err
	}

	return &pushManagerService{cfg.Config}, nil
}

// Service PushManager 对外服务接口
type Service interface {
	// CreatePushEvent 创建推送事件
	CreatePushEvent(ctx context.Context, req *CreatePushEventRequest) (*CreatePushEventResponse, error)
	// DeletePushEvent 删除推送事件
	DeletePushEvent(ctx context.Context, domain, eventID string) (*BaseResponse, error)
	// GetPushEvent 获取单个推送事件
	GetPushEvent(ctx context.Context, domain, eventID string) (*GetPushEventResponse, error)
	// ListPushEvents 列出推送事件
	ListPushEvents(ctx context.Context, domain string, query *ListPushEventsRequest) (*ListPushEventsResponse, error)
	// UpdatePushEvent 更新推送事件
	UpdatePushEvent(ctx context.Context, domain, eventID string, req *UpdatePushEventRequest) (*BaseResponse, error)

	// CreatePushWhitelist 创建推送白名单
	CreatePushWhitelist(ctx context.Context, req *CreatePushWhitelistRequest) (*BaseResponse, error)
	// DeletePushWhitelist 删除推送白名单
	DeletePushWhitelist(ctx context.Context, domain, whitelistID string) (*BaseResponse, error)
	// GetPushWhitelist 获取单个推送白名单
	GetPushWhitelist(ctx context.Context, domain, whitelistID string) (*GetPushWhitelistResponse, error)
	// ListPushWhitelists 列出推送白名单
	ListPushWhitelists(ctx context.Context, domain string, query *ListPushWhitelistsRequest) (*ListPushWhitelistsResponse, error)
	// UpdatePushWhitelist 更新推送白名单
	UpdatePushWhitelist(ctx context.Context, domain, whitelistID string, req *UpdatePushWhitelistRequest) (*BaseResponse, error)

	// CreatePushTemplate 创建推送模板
	CreatePushTemplate(ctx context.Context, req *CreatePushTemplateRequest) (*BaseResponse, error)
	// DeletePushTemplate 删除推送模板
	DeletePushTemplate(ctx context.Context, domain, templateID string) (*BaseResponse, error)
	// GetPushTemplate 获取推送模板
	GetPushTemplate(ctx context.Context, domain, templateID string) (*GetPushTemplateResponse, error)
	// ListPushTemplates 列出推送模板
	ListPushTemplates(ctx context.Context, domain string, query *ListPushTemplatesRequest) (*ListPushTemplatesResponse, error)
	// UpdatePushTemplate 更新推送模板
	UpdatePushTemplate(ctx context.Context, domain, templateID string, req *UpdatePushTemplateRequest) (*BaseResponse, error)
}
