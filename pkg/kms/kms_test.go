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

package kms

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestLoadNotEnabled(t *testing.T) {
	secrets, err := Load("", "")
	require.NoError(t, err)
	require.Nil(t, secrets)
}

func TestLoadRejectsHalfInput(t *testing.T) {
	_, err := Load("/tmp/envelope", "")
	require.Error(t, err)

	_, err = Load("", "private-key")
	require.Error(t, err)
}

func TestLoadDecryptFailure(t *testing.T) {
	dir := t.TempDir()
	envelopeFile := filepath.Join(dir, "envelope")
	require.NoError(t, os.WriteFile(envelopeFile, []byte("not-an-envelope"), 0o600))

	_, err := Load(envelopeFile, "not-a-private-key")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "not-a-private-key")
	require.NotContains(t, err.Error(), "not-an-envelope")
}

func TestApplyOverlay(t *testing.T) {
	conf := map[string]interface{}{}
	require.NoError(t, yaml.Unmarshal([]byte(`
baseConf:
  app_code: old-code
  app_secret: old-secret
esb:
  appCode: old-code
  appSecret: old-secret
iam:
  appCode: old-code
  appSecret: old-secret
  v4:
    app_code: old-code
    app_secret: old-secret
itsm:
  appCode: old-code
  appSecret: old-secret
cmdb:
  appCode: old-code
  appSecret: old-secret
base_conf:
  app_code: old-code
  app_secret: old-secret
sharding:
  adminDatabase:
    user: old-user
    password: old-password
repository:
  redisCluster:
    password: old-redis
  bkRepo:
    username: old-user
    password: old-password
  s3:
    accessKeyID: old-ak
    secretAccessKey: old-sk
  slave:
    bkRepo:
      username: old-user
      password: old-password
    s3:
      accessKeyID: old-ak
      secretAccessKey: old-sk
redisCluster:
  password: old-redis
componentRateLimit:
  redisCluster:
    password: old-limit
bcs:
  token: old-token
pushProvider:
  config:
    token: old-push
credential:
  master_key: old-master
`), &conf))

	secrets := &RuntimeSecrets{
		BKAppIDSecret: &BKAppIDSecret{
			Default: &AppCredential{AppCode: strPtr("bk-bscp"), AppSecret: strPtr("app-secret")},
			CMDB:    &AppCredential{AppCode: strPtr("cmdb-code"), AppSecret: strPtr("cmdb-secret")},
		},
		MySQL: &MySQLSecrets{Default: &UsernamePassword{Username: strPtr("db-user"), Password: strPtr("db-pass")}},
		Redis: &RedisSecrets{
			Default:   &RedisCredential{Password: strPtr("redis-pass")},
			RateLimit: &RedisCredential{Password: strPtr("limit-pass")},
		},
		BKRepo: &BKRepoSecrets{
			Default: &UsernamePassword{Username: strPtr("repo-user"), Password: strPtr("repo-pass")},
			Slave:   &UsernamePassword{Username: strPtr("slave-repo-user"), Password: strPtr("slave-repo-pass")},
		},
		S3: &S3Secrets{
			Default: &S3Credential{AccessKeyID: strPtr("ak"), SecretAccessKey: strPtr("sk")},
			Slave:   &S3Credential{AccessKeyID: strPtr("slave-ak"), SecretAccessKey: strPtr("slave-sk")},
		},
		BCS: &BCSSecrets{
			Default:     &TokenCredential{Token: strPtr("bcs-token")},
			PushManager: &TokenCredential{Token: strPtr("push-token")},
		},
		Encryption: &Encryption{MasterKey: strPtr("master-key")},
	}

	paths := Apply(conf, secrets)
	require.ElementsMatch(t, []string{
		"baseConf.app_code",
		"baseConf.app_secret",
		"esb.appCode",
		"esb.appSecret",
		"iam.appCode",
		"iam.appSecret",
		"iam.v4.app_code",
		"iam.v4.app_secret",
		"itsm.appCode",
		"itsm.appSecret",
		"cmdb.appCode",
		"cmdb.appSecret",
		"base_conf.app_code",
		"base_conf.app_secret",
		"sharding.adminDatabase.user",
		"sharding.adminDatabase.password",
		"repository.redisCluster.password",
		"redisCluster.password",
		"componentRateLimit.redisCluster.password",
		"repository.bkRepo.username",
		"repository.bkRepo.password",
		"repository.slave.bkRepo.username",
		"repository.slave.bkRepo.password",
		"repository.s3.accessKeyID",
		"repository.s3.secretAccessKey",
		"repository.slave.s3.accessKeyID",
		"repository.slave.s3.secretAccessKey",
		"bcs.token",
		"pushProvider.config.token",
		"credential.master_key",
	}, paths)

	require.Equal(t, "bk-bscp", lookup(t, conf, "baseConf", "app_code"))
	require.Equal(t, "app-secret", lookup(t, conf, "esb", "appSecret"))
	require.Equal(t, "app-secret", lookup(t, conf, "iam", "v4", "app_secret"))
	require.Equal(t, "cmdb-code", lookup(t, conf, "cmdb", "appCode"))
	require.Equal(t, "cmdb-secret", lookup(t, conf, "cmdb", "appSecret"))
	require.Equal(t, "db-user", lookup(t, conf, "sharding", "adminDatabase", "user"))
	require.Equal(t, "limit-pass", lookup(t, conf, "componentRateLimit", "redisCluster", "password"))
	require.Equal(t, "slave-sk", lookup(t, conf, "repository", "slave", "s3", "secretAccessKey"))
	require.Equal(t, "push-token", lookup(t, conf, "pushProvider", "config", "token"))
	require.Equal(t, "master-key", lookup(t, conf, "credential", "master_key"))
}

func TestApplyRateLimitFallsBackToDefault(t *testing.T) {
	conf := map[string]interface{}{
		"repository":         map[string]interface{}{"redisCluster": map[string]interface{}{"password": "old"}},
		"redisCluster":       map[string]interface{}{"password": "old"},
		"componentRateLimit": map[string]interface{}{"redisCluster": map[string]interface{}{"password": "old"}},
	}
	paths := Apply(conf, &RuntimeSecrets{
		Redis: &RedisSecrets{Default: &RedisCredential{Password: strPtr("redis-pass")}},
	})
	require.ElementsMatch(t, []string{
		"repository.redisCluster.password",
		"redisCluster.password",
		"componentRateLimit.redisCluster.password",
	}, paths)
	require.Equal(t, "redis-pass", lookup(t, conf, "componentRateLimit", "redisCluster", "password"))
}

func TestApplySkipsMissingPaths(t *testing.T) {
	conf := map[string]interface{}{
		"baseConf": map[string]interface{}{"app_code": "old", "app_secret": "old"},
	}
	paths := Apply(conf, &RuntimeSecrets{
		BKAppIDSecret: &BKAppIDSecret{
			Default: &AppCredential{AppCode: strPtr("bk-bscp"), AppSecret: strPtr("app-secret")},
		},
		MySQL:      &MySQLSecrets{Default: &UsernamePassword{Username: strPtr("db-user"), Password: strPtr("db-pass")}},
		Encryption: &Encryption{MasterKey: strPtr("master-key")},
	})
	require.ElementsMatch(t, []string{"baseConf.app_code", "baseConf.app_secret"}, paths)
	_, ok := conf["sharding"]
	require.False(t, ok)
	_, ok = conf["credential"]
	require.False(t, ok)
	_, ok = conf["cmdb"]
	require.False(t, ok)
}

func TestApplyIgnoresAbsentCredentialFields(t *testing.T) {
	conf := map[string]interface{}{
		"cmdb":     map[string]interface{}{"appCode": "keep-code", "appSecret": "keep-secret"},
		"baseConf": map[string]interface{}{"app_code": "old", "app_secret": "old"},
	}
	paths := Apply(conf, &RuntimeSecrets{
		BKAppIDSecret: &BKAppIDSecret{
			CMDB: &AppCredential{AppSecret: strPtr("cmdb-secret")},
		},
	})
	require.Equal(t, []string{"cmdb.appSecret"}, paths)
	require.Equal(t, "keep-code", lookup(t, conf, "cmdb", "appCode"))
	require.Equal(t, "cmdb-secret", lookup(t, conf, "cmdb", "appSecret"))
	require.Equal(t, "old", lookup(t, conf, "baseConf", "app_code"))
}

func strPtr(v string) *string {
	return &v
}

func lookup(t *testing.T, root map[string]interface{}, path ...string) string {
	t.Helper()
	var cur interface{} = root
	for _, key := range path {
		m, ok := cur.(map[string]interface{})
		require.Truef(t, ok, "path %v is not a map, got %T", path, cur)
		cur, ok = m[key]
		require.Truef(t, ok, "missing %s in %v", key, path)
	}
	s, ok := cur.(string)
	require.Truef(t, ok, "path %v is %T", path, cur)
	return s
}
