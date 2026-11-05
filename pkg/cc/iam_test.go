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
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
)

// 权限模型属于平台租户：多租户环境用 system，单租户环境用 default。
func TestResolveModelTenantID(t *testing.T) {
	require.Equal(t, constant.SystemTenantID, ResolveModelTenantID(true))
	require.Equal(t, constant.DefaultTenantID, ResolveModelTenantID(false))
}

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

// 未配置开关时默认关闭，保持改造前的业务维度授权逻辑。
func TestIAMProjectEnvAuthDefaultsToDisabled(t *testing.T) {
	t.Setenv(config.IAMProjectEnvAuthEnv, "")
	iam := IAM{Version: IAMVersionV3, APIURL: "http://iam", AppCode: "c", AppSecret: "s"}
	iam.trySetDefault()

	require.False(t, iam.ProjectEnvAuthEnabled())
	require.NoError(t, iam.validate())
}

// 配置文件显式开启时以文件为准。
func TestIAMProjectEnvAuthFromConfig(t *testing.T) {
	enabled := true
	iam := IAM{
		Version:                IAMVersionV3,
		APIURL:                 "http://iam",
		AppCode:                "c",
		AppSecret:              "s",
		ProjectEnvironmentAuth: IAMProjectEnvironmentAuth{Enabled: &enabled},
	}
	iam.trySetDefault()

	require.True(t, iam.ProjectEnvAuthEnabled())
	require.NoError(t, iam.validate())
}

// 配置文件未配置时允许通过环境变量开启，便于不改配置文件快速切换。
func TestIAMProjectEnvAuthFromEnv(t *testing.T) {
	t.Setenv(config.IAMProjectEnvAuthEnv, "true")
	iam := IAM{Version: IAMVersionV3, APIURL: "http://iam", AppCode: "c", AppSecret: "s"}
	iam.trySetDefault()

	require.True(t, iam.ProjectEnvAuthEnabled())
	require.NoError(t, iam.validate())
}

// 配置文件显式配置优先于环境变量，且可以显式关闭。
func TestIAMProjectEnvAuthConfigWinsOverEnv(t *testing.T) {
	t.Setenv(config.IAMProjectEnvAuthEnv, "true")
	disabled := false
	iam := IAM{
		Version:                IAMVersionV3,
		APIURL:                 "http://iam",
		AppCode:                "c",
		AppSecret:              "s",
		ProjectEnvironmentAuth: IAMProjectEnvironmentAuth{Enabled: &disabled},
	}
	iam.trySetDefault()

	require.False(t, iam.ProjectEnvAuthEnabled())
	require.NoError(t, iam.validate())
}

// 环境变量取值非法时应报错，而不是静默按关闭处理。
func TestIAMProjectEnvAuthEnvInvalid(t *testing.T) {
	t.Setenv(config.IAMProjectEnvAuthEnv, "yes")
	iam := IAM{Version: IAMVersionV3, APIURL: "http://iam", AppCode: "c", AppSecret: "s"}
	iam.trySetDefault()

	require.ErrorContains(t, iam.validate(), config.IAMProjectEnvAuthEnv)
}

// 确认 yaml 标签与配置形态一致。
func TestIAMYamlUnmarshalProjectEnvAuth(t *testing.T) {
	const conf = `
version: v3
api_url: "http://iam.example.com"
appCode: "bk-bscp"
appSecret: "secret"
project_environment_auth:
  enabled: true
`

	iam := new(IAM)
	require.NoError(t, yaml.Unmarshal([]byte(conf), iam))

	require.NotNil(t, iam.ProjectEnvironmentAuth.Enabled)
	require.True(t, iam.ProjectEnvAuthEnabled())
}

// 未配置资源拓扑时默认 auto，由 auth-server 启动时探测。
func TestIAMResourceTopologyDefaultsToAuto(t *testing.T) {
	t.Setenv(config.IAMResourceTopologyEnv, "")
	iam := IAM{Version: IAMVersionV3, APIURL: "http://iam", AppCode: "c", AppSecret: "s"}
	iam.trySetDefault()

	require.Equal(t, IAMResourceTopologyAuto, iam.ResolveTopology())
	require.NoError(t, iam.validate())
}

// 配置文件显式指定的拓扑优先于环境变量。
func TestIAMResourceTopologyFromConfig(t *testing.T) {
	t.Setenv(config.IAMResourceTopologyEnv, "legacy")
	iam := IAM{
		Version:   IAMVersionV3,
		APIURL:    "http://iam",
		AppCode:   "c",
		AppSecret: "s",
		ProjectEnvironmentAuth: IAMProjectEnvironmentAuth{
			Topology: IAMResourceTopologyHierarchical,
		},
	}
	iam.trySetDefault()

	require.Equal(t, IAMResourceTopologyHierarchical, iam.ResolveTopology())
	require.NoError(t, iam.validate())
}

// 配置文件未配置时允许通过环境变量指定拓扑。
func TestIAMResourceTopologyFromEnv(t *testing.T) {
	t.Setenv(config.IAMResourceTopologyEnv, "legacy")
	iam := IAM{Version: IAMVersionV3, APIURL: "http://iam", AppCode: "c", AppSecret: "s"}
	iam.trySetDefault()

	require.Equal(t, IAMResourceTopologyLegacy, iam.ResolveTopology())
	require.NoError(t, iam.validate())
}

// 环境变量取值非法时应报错，而不是静默回退 auto。
func TestIAMResourceTopologyEnvInvalid(t *testing.T) {
	t.Setenv(config.IAMResourceTopologyEnv, "tree")
	iam := IAM{Version: IAMVersionV3, APIURL: "http://iam", AppCode: "c", AppSecret: "s"}
	iam.trySetDefault()

	require.Equal(t, IAMResourceTopologyAuto, iam.ResolveTopology())
	require.ErrorContains(t, iam.validate(), config.IAMResourceTopologyEnv)
}

// 配置文件取值非法时同样报错。
func TestIAMResourceTopologyConfigInvalid(t *testing.T) {
	iam := IAM{
		Version:   IAMVersionV3,
		APIURL:    "http://iam",
		AppCode:   "c",
		AppSecret: "s",
		ProjectEnvironmentAuth: IAMProjectEnvironmentAuth{
			Topology: IAMResourceTopology("tree"),
		},
	}
	iam.trySetDefault()

	require.ErrorContains(t, iam.validate(), "unsupported iam resource topology")
}

// 确认拓扑字段的 yaml 标签与配置形态一致。
func TestIAMYamlUnmarshalResourceTopology(t *testing.T) {
	const conf = `
version: v3
api_url: "http://iam.example.com"
appCode: "bk-bscp"
appSecret: "secret"
project_environment_auth:
  enabled: false
  topology: hierarchical
`

	iam := new(IAM)
	require.NoError(t, yaml.Unmarshal([]byte(conf), iam))
	require.Equal(t, IAMResourceTopologyHierarchical, iam.ResolveTopology())
	require.NoError(t, iam.validate())
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
