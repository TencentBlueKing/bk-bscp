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

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitResBaseJSURL(t *testing.T) {
	conf := defaultUIConf()
	err := conf.initResBaseJSURL("bk-bscp")
	assert.NoError(t, err)
	assert.Equal(t, "", conf.Host.BKSharedResBaseJSURL)

	conf.Host.BKSharedResURL = "http://repo.test.com"
	err = conf.initResBaseJSURL("bk-bscp")
	assert.NoError(t, err)
	assert.Equal(t, "http://repo.test.com/bk_bscp/base.js", conf.Host.BKSharedResBaseJSURL)
}

func TestHostConfGetFromEnv(t *testing.T) {
	// 字段为空时, 从环境变量补充
	t.Run("fill from env when empty", func(t *testing.T) {
		t.Setenv(NewUIURLEnv, "https://new-ui.example.com")
		t.Setenv(OldUIURLEnv, "https://old-ui.example.com")

		h := &HostConf{}
		h.getFromEnv()
		assert.Equal(t, "https://new-ui.example.com", h.NewUIURL)
		assert.Equal(t, "https://old-ui.example.com", h.OldUIURL)
	})

	// 字段非空时, 显式配置优先, 不被环境变量覆盖
	t.Run("keep explicit config", func(t *testing.T) {
		t.Setenv(NewUIURLEnv, "https://new-ui.example.com")
		t.Setenv(OldUIURLEnv, "https://old-ui.example.com")

		h := &HostConf{
			NewUIURL: "https://explicit-new.example.com",
			OldUIURL: "https://explicit-old.example.com",
		}
		h.getFromEnv()
		assert.Equal(t, "https://explicit-new.example.com", h.NewUIURL)
		assert.Equal(t, "https://explicit-old.example.com", h.OldUIURL)
	})

	// iam 版本为空时, 从环境变量补充
	t.Run("fill iam version from env", func(t *testing.T) {
		t.Setenv(IAMVersionEnv, "v4")

		h := &HostConf{}
		h.getFromEnv()
		assert.Equal(t, "v4", h.IAMVersion)
		assert.True(t, h.IsIAMV4())
	})
}

func TestHostConfIAMHost(t *testing.T) {
	h := &HostConf{BKIAMHost: "https://iam-v3.example.com"}

	// 未开启 v4 时, 即使配置了 bk_iam_v4_host 也使用 bk_iam_host
	t.Run("fallback when v4 disabled", func(t *testing.T) {
		h.BKIAMV4Host = "https://iam-v4.example.com"
		h.IAMVersion = ""
		assert.Equal(t, "https://iam-v3.example.com", h.IAMHost())
	})

	// 环境变量开启 v4 且配置了 bk_iam_v4_host 时使用 v4 地址
	t.Run("use v4 host when enabled by env", func(t *testing.T) {
		h.BKIAMV4Host = "https://iam-v4.example.com"
		h.IAMVersion = "v4"
		assert.Equal(t, "https://iam-v4.example.com", h.IAMHost())
	})

	// 开启 v4 但未配置 bk_iam_v4_host 时回退 bk_iam_host
	t.Run("fallback when v4 host missing", func(t *testing.T) {
		h.BKIAMV4Host = ""
		h.IAMVersion = "v4"
		assert.Equal(t, "https://iam-v3.example.com", h.IAMHost())
	})
}
