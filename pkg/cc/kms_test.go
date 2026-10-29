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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/TencentBlueKing/bk-bscp/pkg/kms"
)

func TestOverlayKMSDisabled(t *testing.T) {
	conf := map[string]interface{}{
		"kms":      map[string]interface{}{"enabled": false, "envelopeFile": "/missing"},
		"baseConf": map[string]interface{}{"app_secret": "keep"},
	}
	require.NoError(t, overlayKMS(conf))
	require.Equal(t, "keep", conf["baseConf"].(map[string]interface{})["app_secret"])
}

func TestOverlayKMSFailureDoesNotContinue(t *testing.T) {
	const privateKey = "secret-key-should-not-leak"
	t.Setenv(kms.PrivateKeyEnv, privateKey)
	conf := map[string]interface{}{
		"kms": map[string]interface{}{
			"enabled":      true,
			"envelopeFile": filepath.Join(t.TempDir(), "missing"),
		},
		"baseConf": map[string]interface{}{"app_secret": ""},
	}
	err := overlayKMS(conf)
	require.Error(t, err)
	require.NotContains(t, err.Error(), privateKey)
	require.Empty(t, conf["baseConf"].(map[string]interface{})["app_secret"])
}

func TestLoadFromFileErrorOmitsConfig(t *testing.T) {
	InitService(APIServerName)
	const secret = "plaintext-master-key-should-not-leak"
	_, err := loadFromFile([]byte("network:\n  httpPort: not-a-port\ncredential:\n  master_key: " + secret + "\n"))
	require.Error(t, err)
	require.NotContains(t, err.Error(), secret)
	require.NotContains(t, err.Error(), "credential:")
	require.Contains(t, err.Error(), "unmarshal Setting yaml failed")
}
