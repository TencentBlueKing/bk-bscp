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

package cmdb

import (
	"context"
	"os"
	"testing"

	"github.com/davecgh/go-spew/spew"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
)

// getCMDBConfig 从环境变量获取 CMDB 配置
func getCMDBConfig(t *testing.T) *cc.CMDBConfig {
	appCode := os.Getenv("APP_CODE")
	appSecret := os.Getenv("APP_SECRET")
	host := os.Getenv("CMDB_HOST")
	bkUserName := os.Getenv("BK_USER_NAME")
	useEsb := os.Getenv("USE_ESB")

	if appCode == "" || appSecret == "" || host == "" {
		t.Skip("Skipping test: APP_CODE, APP_SECRET, or HOST environment variables not set")
	}

	cc.SetG(cc.GlobalSettings{
		FeatureFlags: cc.FeatureFlags{
			EnableMultiTenantMode: false,
		},
	})
	cfg := &cc.CMDBConfig{
		AppCode:    appCode,
		AppSecret:  appSecret,
		Host:       host,
		BkUserName: bkUserName,
		UseEsb:     useEsb == "true",
	}

	return cfg
}

// TestCCTopoXMLService_GetTopoTreeXML 测试 GetTopoTreeXML 方法
// 使用真实的 CMDB 服务
// 需要设置以下环境变量：
//   - APP_CODE: CMDB 应用代码
//   - APP_SECRET: CMDB 应用密钥
//   - HOST: CMDB 服务地址
//   - BK_USER_NAME: CMDB 用户名（可选，默认为空）
//   - USE_ESB: 是否使用 ESB（可选，默认为 false）
func TestCCTopoXMLService_GetTopoTreeXML(t *testing.T) {
	// 从环境变量获取配置
	cfg := getCMDBConfig(t)

	// 初始化真实的 CMDB 服务
	cmdbSvc, err := bkcmdb.New(cfg, nil)
	if err != nil {
		t.Fatalf("initialize cmdb service error: %v", err)
	}

	// 测试业务ID（可以根据实际情况修改）
	bizID := 2

	// 创建 CC 拓扑 XML 服务
	svc := NewCCTopoXMLService(bizID, cmdbSvc)

	ctx := context.Background()

	// 测试获取完整拓扑 XML（不过滤环境）
	t.Logf("=== 测试获取完整拓扑 XML（不过滤环境）===")
	xmlStr, err := svc.GetTopoTreeXML(ctx, "")
	if err != nil {
		t.Fatalf("GetTopoTreeXML failed: %v", err)
	}

	t.Logf("完整拓扑 XML 长度: %d 字符", len(xmlStr))
	t.Logf("\n%s", xmlStr)

	// 测试获取指定环境的拓扑 XML（正式环境 "3"）
	t.Logf("\n=== 测试获取指定环境的拓扑 XML（正式环境）===")
	xmlStrFiltered, err := svc.GetTopoTreeXML(ctx, "3")
	if err != nil {
		t.Fatalf("GetTopoTreeXML with setEnv filter failed: %v", err)
	}

	t.Logf("过滤后的拓扑 XML 长度: %d 字符", len(xmlStrFiltered))
	t.Logf("\n%s", xmlStrFiltered)

	// 测试获取测试环境的拓扑 XML
	t.Logf("\n=== 测试获取测试环境的拓扑 XML（测试环境）===")
	xmlStrTest, err := svc.GetTopoTreeXML(ctx, "1")
	if err != nil {
		t.Fatalf("GetTopoTreeXML with test env failed: %v", err)
	}

	t.Logf("测试环境的拓扑 XML 长度: %d 字符", len(xmlStrTest))
	t.Logf("\n%s", xmlStrTest)

	// 使用 spew 输出结构化数据（用于调试，只输出前 500 个字符）
	t.Logf("\n=== XML 内容预览（前 500 字符）===")
	if len(xmlStr) > 500 {
		t.Logf("%s...", xmlStr[:500])
	} else {
		t.Logf("%s", xmlStr)
	}

	// 输出完整 XML 的 spew dump（用于详细分析）
	t.Logf("\n=== 完整 XML 内容（spew dump）===")
	t.Logf("%s", spew.Sdump(xmlStr))
}
