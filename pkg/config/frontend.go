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

// Package config xxx
package config

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"
)

const (
	// NewUIURLEnv 新 UI 路径/地址对应的环境变量，用于不便于修改配置文件时通过环境变量覆盖
	NewUIURLEnv = "BK_BSCP_NEW_UI_URL"
	// OldUIURLEnv 旧 UI 路径/地址对应的环境变量，用于不便于修改配置文件时通过环境变量覆盖
	OldUIURLEnv = "BK_BSCP_OLD_UI_URL"
	// IAMVersionEnv 权限中心版本对应的环境变量，取值 v3/v4，
	// 用于不便于修改配置文件时通过环境变量开启 iam v4，各组件共用
	IAMVersionEnv = "BK_BSCP_IAM_VERSION"
)

// HostConf host conf
type HostConf struct {
	BKIAMHost            string `yaml:"bk_iam_host"`       // 权限中心
	BKIAMV4Host          string `yaml:"bk_iam_v4_host"`    // 权限中心 V4，为空时使用 bk_iam_host
	BKCMDBHost           string `yaml:"bk_cmdb_host"`      // 配置平台
	BSCPAPIURL           string `yaml:"bscp_api_url"`      // bscp api地址
	BKNODEMANHOST        string `yaml:"bk_nodeman_host"`   // 节点管理地址
	BKSharedResURL       string `yaml:"bk_shared_res_url"` // 对应运维公共变量bkSharedResUrl, PaaS环境变量BKPAAS_SHARED_RES_URL
	BKSharedResBaseJSURL string `yaml:"-"`                 // 规则是${bkSharedResUrl}/${目录名 aka app_code}/base.js
	UserManHost          string `yaml:"user_man_host"`     // 用户列表host
	UserCenterURL        string `yaml:"user_center_url"`   // 用户中心(个人中心)跳转地址
	NewUIURL             string `yaml:"new_ui_url"`        // 新 UI 路径/地址
	OldUIURL             string `yaml:"old_ui_url"`        // 旧 UI 路径/地址
	IAMVersion           string `yaml:"iam_version"`       // 权限中心版本, 取值 v3/v4, 为空时读环境变量 BK_BSCP_IAM_VERSION, 再为空时视为 v3
}

// getFromEnv 从环境变量补充配置，仅当对应字段为空时读取，避免覆盖显式配置
func (h *HostConf) getFromEnv() {
	if h.NewUIURL == "" {
		h.NewUIURL = os.Getenv(NewUIURLEnv)
	}
	if h.OldUIURL == "" {
		h.OldUIURL = os.Getenv(OldUIURLEnv)
	}
	if h.IAMVersion == "" {
		h.IAMVersion = os.Getenv(IAMVersionEnv)
	}
}

// IsIAMV4 判断是否启用权限中心 V4，仅由 iam_version(配置文件或环境变量)决定。
func (h *HostConf) IsIAMV4() bool {
	return h.IAMVersion == "v4"
}

// IAMHost 返回 UI 展示用的权限中心地址。
// 开启 IAM V4 且配置了 bk_iam_v4_host 时使用 V4 地址，
// 否则统一回退 bk_iam_host，关闭 V4 时即便残留 V4 地址也不会被使用。
func (h *HostConf) IAMHost() string {
	if h.IsIAMV4() && h.BKIAMV4Host != "" {
		return h.BKIAMV4Host
	}

	return h.BKIAMHost
}

// FrontendConf docs and host conf
type FrontendConf struct {
	Docs           map[string]string `yaml:"docs"`
	Host           *HostConf         `yaml:"hosts"`
	Helper         string            `yaml:"helper"`           // 白名单对接人员
	EnableBKNotice bool              `yaml:"enable_bk_notice"` // 是否启用蓝鲸通知中心
}

// defaultFrontendConf 默认配置
func defaultUIConf() *FrontendConf {
	c := &FrontendConf{
		Docs: map[string]string{},
		Host: &HostConf{},
	}
	return c
}

func (c *FrontendConf) initResBaseJSURL(appCode string) error {
	if c.Host.BKSharedResURL == "" {
		return nil
	}
	if appCode == "" {
		return fmt.Errorf("initResBaseJSURL: app_code is required")
	}

	// 规范: 统一使用下划线做目录名
	appCode = strings.ReplaceAll(appCode, "-", "_")

	u, err := url.Parse(c.Host.BKSharedResURL)
	if err != nil {
		return err
	}
	u.Path = path.Join(u.Path, appCode, "base.js")

	c.Host.BKSharedResBaseJSURL = u.String()
	return nil
}
