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

package cc

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/TencentBlueKing/bk-bscp/pkg/config"
)

func TestIAMVersionDefaultsToV3(t *testing.T) {
	iam := IAM{APIURL: "http://iam", AppCode: "code", AppSecret: "secret"}
	iam.trySetDefault()

	require.Equal(t, IAMVersionV3, iam.Version)
	require.False(t, iam.IsV4())
	require.NoError(t, iam.validate())
}

// V4 未单独配置应用凭证时应回退到外层的，避免同一套凭证配两遍。
func TestIAMV4CredentialFallback(t *testing.T) {
	iam := IAM{
		Version:   IAMVersionV4,
		AppCode:   "outer-code",
		AppSecret: "outer-secret",
		V4:        IAMV4{GatewayURL: "https://gw/prod"},
	}
	iam.trySetDefault()

	require.Equal(t, "outer-code", iam.V4.AppCode)
	require.Equal(t, "outer-secret", iam.V4.AppSecret)
	require.Equal(t, defaultIAMV4SystemID, iam.V4.SystemID)
	require.NoError(t, iam.validate())
}

func TestIAMV4ExplicitCredentialWins(t *testing.T) {
	iam := IAM{
		Version:   IAMVersionV4,
		AppCode:   "outer-code",
		AppSecret: "outer-secret",
		V4: IAMV4{
			GatewayURL: "https://gw/prod",
			AppCode:    "v4-code",
			AppSecret:  "v4-secret",
			SystemID:   "custom-system",
		},
	}
	iam.trySetDefault()

	require.Equal(t, "v4-code", iam.V4.AppCode)
	require.Equal(t, "v4-secret", iam.V4.AppSecret)
	require.Equal(t, "custom-system", iam.V4.SystemID)
}

func TestIAMV4RuntimeDefaults(t *testing.T) {
	iam := IAM{Version: IAMVersionV4, AppCode: "c", AppSecret: "s",
		V4: IAMV4{GatewayURL: "https://gw/prod"}}
	iam.trySetDefault()

	require.Equal(t, defaultIAMV4AuthCacheTTLSeconds, iam.V4.AuthCacheTTLSeconds)
	require.Equal(t, defaultIAMV4AuthCacheSize, iam.V4.AuthCacheSize)
	require.Equal(t, defaultIAMV4AuthConcurrency, iam.V4.AuthConcurrency)
}

// 校验只针对当前 Version 那一套配置，另一套允许缺省。
func TestIAMValidateScopedToVersion(t *testing.T) {
	// v3 模式下不要求 V4 配置
	v3 := IAM{Version: IAMVersionV3, APIURL: "http://iam", AppCode: "c", AppSecret: "s"}
	require.NoError(t, v3.validate())

	// v4 模式下不要求 V3 的 api_url
	v4 := IAM{Version: IAMVersionV4, V4: IAMV4{GatewayURL: "https://gw/prod",
		AppCode: "c", AppSecret: "s"}}
	require.NoError(t, v4.validate())

	// v4 模式缺网关地址应报错
	missing := IAM{Version: IAMVersionV4, V4: IAMV4{AppCode: "c", AppSecret: "s"}}
	require.ErrorContains(t, missing.validate(), "gateway url")

	// v3 模式缺 api_url 应报错
	noURL := IAM{Version: IAMVersionV3, AppCode: "c", AppSecret: "s"}
	require.ErrorContains(t, noURL.validate(), "api url")
}

func TestIAMRejectsUnknownVersion(t *testing.T) {
	iam := IAM{Version: "v5", APIURL: "http://iam", AppCode: "c", AppSecret: "s"}
	require.ErrorContains(t, iam.validate(), "unsupported iam version")
}

// 配置文件未指定版本时，允许通过环境变量 BK_BSCP_IAM_VERSION 开启 iam v4。
func TestIAMVersionFromEnv(t *testing.T) {
	t.Setenv(config.IAMVersionEnv, "v4")
	iam := IAM{AppCode: "c", AppSecret: "s", V4: IAMV4{GatewayURL: "https://gw/prod"}}
	iam.trySetDefault()

	require.True(t, iam.IsV4())
	require.NoError(t, iam.validate())
}

// 配置文件显式指定的版本优先于环境变量。
func TestIAMVersionConfigFileWinsOverEnv(t *testing.T) {
	t.Setenv(config.IAMVersionEnv, "v4")
	iam := IAM{Version: IAMVersionV3, APIURL: "http://iam", AppCode: "c", AppSecret: "s"}
	iam.trySetDefault()

	require.False(t, iam.IsV4())
	require.NoError(t, iam.validate())
}

// 环境变量取值非法时应报错，而不是静默回退 v3。
func TestIAMVersionEnvInvalid(t *testing.T) {
	t.Setenv(config.IAMVersionEnv, "v5")
	iam := IAM{AppCode: "c", AppSecret: "s"}
	iam.trySetDefault()

	require.ErrorContains(t, iam.validate(), "unsupported iam version")
}

// 确认 yaml 标签与方案 §5.3 约定的配置形态一致。
func TestIAMYamlUnmarshal(t *testing.T) {
	const conf = `
version: v4
api_url: "http://iam.example.com"
appCode: "bk-bscp"
appSecret: "v3-secret"
v4:
  gateway_url: "https://bkiam.example.com/prod"
  system_id: "bk-bscp"
  app_code: "bk-bscp-v4"
  app_secret: "v4-secret"
  callback_host: "http://bscp.example.com"
  auth_cache_ttl_seconds: 60
  auth_cache_size: 1000
  auth_concurrency: 8
`

	iam := new(IAM)
	require.NoError(t, yaml.Unmarshal([]byte(conf), iam))

	require.True(t, iam.IsV4())
	require.Equal(t, "http://iam.example.com", iam.APIURL)
	require.Equal(t, "https://bkiam.example.com/prod", iam.V4.GatewayURL)
	require.Equal(t, "bk-bscp", iam.V4.SystemID)
	require.Equal(t, "bk-bscp-v4", iam.V4.AppCode)
	require.Equal(t, "v4-secret", iam.V4.AppSecret)
	require.Equal(t, "http://bscp.example.com", iam.V4.CallbackHost)
	require.Equal(t, 60, iam.V4.AuthCacheTTLSeconds)
	require.Equal(t, 1000, iam.V4.AuthCacheSize)
	require.Equal(t, 8, iam.V4.AuthConcurrency)
	require.NoError(t, iam.validate())
}
