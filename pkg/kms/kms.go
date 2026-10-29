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

// Package kms decrypts the aggregated runtime credential and overlays it onto service config.
// Callers pass the envelope path and private key in. This package does not read the environment.
package kms

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/TencentBlueKing/bk-kms-sdk/go/decrypt"
)

const (
	// PrivateKeyEnv is the environment variable holding the KMS private key.
	PrivateKeyEnv = "BSCP_KMS_PRIVATE_KEY"
	// EnvelopeFile is the mounted envelope path injected by the chart.
	EnvelopeFile = "/data/bscp/kms/envelope"
)

// AppCredential is one blueking application identity.
type AppCredential struct {
	AppCode   *string `json:"app_code"`
	AppSecret *string `json:"app_secret"`
}

// UsernamePassword is a username and password pair.
type UsernamePassword struct {
	Username *string `json:"username"`
	Password *string `json:"password"`
}

// RedisCredential is a redis password.
type RedisCredential struct {
	Password *string `json:"password"`
}

// S3Credential is an S3 access key pair.
type S3Credential struct {
	AccessKeyID     *string `json:"access_key_id"`
	SecretAccessKey *string `json:"secret_access_key"`
}

// TokenCredential is a bearer token.
type TokenCredential struct {
	Token *string `json:"token"`
}

// BKAppIDSecret holds the default identity and an optional CMDB identity.
type BKAppIDSecret struct {
	Default *AppCredential `json:"default"`
	CMDB    *AppCredential `json:"cmdb"`
}

// MySQLSecrets holds mysql credentials.
type MySQLSecrets struct {
	Default *UsernamePassword `json:"default"`
}

// RedisSecrets holds redis credentials. RateLimit falls back to Default when absent.
type RedisSecrets struct {
	Default   *RedisCredential `json:"default"`
	RateLimit *RedisCredential `json:"rate_limit"`
}

// BKRepoSecrets holds bkrepo credentials.
type BKRepoSecrets struct {
	Default *UsernamePassword `json:"default"`
	Slave   *UsernamePassword `json:"slave"`
}

// S3Secrets holds s3 credentials.
type S3Secrets struct {
	Default *S3Credential `json:"default"`
	Slave   *S3Credential `json:"slave"`
}

// BCSSecrets holds BCS tokens.
type BCSSecrets struct {
	Default     *TokenCredential `json:"default"`
	PushManager *TokenCredential `json:"push_manager"`
}

// Encryption holds the client-secret master key.
type Encryption struct {
	MasterKey *string `json:"master_key"`
}

// RuntimeSecrets is the bk_bscp_runtime_config JSON document.
// A nil field means the key was absent and must not be overlaid.
type RuntimeSecrets struct {
	BKAppIDSecret *BKAppIDSecret `json:"bkapp_id_secret"`
	MySQL         *MySQLSecrets  `json:"mysql"`
	Redis         *RedisSecrets  `json:"redis"`
	BKRepo        *BKRepoSecrets `json:"bkrepo"`
	S3            *S3Secrets     `json:"s3"`
	BCS           *BCSSecrets    `json:"bcs"`
	Encryption    *Encryption    `json:"encryption"`
}

// DefaultApp returns the default application identity fields that were present.
func (s *RuntimeSecrets) DefaultApp() (code, secret *string) {
	if s == nil || s.BKAppIDSecret == nil || s.BKAppIDSecret.Default == nil {
		return nil, nil
	}
	return s.BKAppIDSecret.Default.AppCode, s.BKAppIDSecret.Default.AppSecret
}

// Load reads an envelope file and decrypts it into runtime secrets.
// Both arguments empty means KMS is not enabled. One empty argument, a decrypt
// failure, or invalid JSON returns an error.
func Load(envelopeFile, privateKey string) (*RuntimeSecrets, error) {
	envelopeFile = strings.TrimSpace(envelopeFile)
	privateKey = strings.TrimSpace(privateKey)
	if envelopeFile == "" && privateKey == "" {
		return nil, nil
	}
	if envelopeFile == "" || privateKey == "" {
		return nil, errors.New("kms envelope file and private key must both be set")
	}

	plain, err := decryptCredential(envelopeFile, privateKey)
	if err != nil {
		return nil, err
	}

	var secrets RuntimeSecrets
	if err := json.Unmarshal([]byte(plain), &secrets); err != nil {
		return nil, fmt.Errorf("unmarshal kms runtime credential: %w", err)
	}
	return &secrets, nil
}

func decryptCredential(envelopeFile, privateKey string) (string, error) {
	raw, err := os.ReadFile(envelopeFile)
	if err != nil {
		return "", fmt.Errorf("read kms envelope: %w", err)
	}
	plain, err := decrypt.Decrypt(strings.TrimSpace(string(raw)), privateKey)
	if err != nil {
		return "", fmt.Errorf("decrypt kms envelope: %w", err)
	}
	return plain, nil
}

// Apply writes credential fields onto conf. A path missing from conf is skipped.
// Only JSON fields that are present are written. Returned paths are sorted.
func Apply(conf map[string]interface{}, secrets *RuntimeSecrets) []string {
	if conf == nil || secrets == nil {
		return nil
	}
	var paths []string
	paths = append(paths, applyApp(conf, secrets)...)
	paths = append(paths, applyMySQL(conf, secrets.MySQL)...)
	paths = append(paths, applyRedis(conf, secrets.Redis)...)
	paths = append(paths, applyBKRepo(conf, secrets.BKRepo)...)
	paths = append(paths, applyS3(conf, secrets.S3)...)
	paths = append(paths, applyBCS(conf, secrets.BCS)...)
	if secrets.Encryption != nil {
		paths = append(paths, setPath(conf, "credential.master_key", secrets.Encryption.MasterKey)...)
	}
	sort.Strings(paths)
	return paths
}

func applyApp(conf map[string]interface{}, secrets *RuntimeSecrets) []string {
	if secrets.BKAppIDSecret == nil {
		return nil
	}
	var paths []string
	if def := secrets.BKAppIDSecret.Default; def != nil {
		paths = append(paths, applyCred(conf, "baseConf", "app_code", "app_secret", def)...)
		paths = append(paths, applyCred(conf, "esb", "appCode", "appSecret", def)...)
		paths = append(paths, applyCred(conf, "iam", "appCode", "appSecret", def)...)
		paths = append(paths, applyCred(conf, "iam.v4", "app_code", "app_secret", def)...)
		paths = append(paths, applyCred(conf, "itsm", "appCode", "appSecret", def)...)
		paths = append(paths, applyCred(conf, "base_conf", "app_code", "app_secret", def)...)
		if secrets.BKAppIDSecret.CMDB == nil {
			paths = append(paths, applyCred(conf, "cmdb", "appCode", "appSecret", def)...)
		}
	}
	if secrets.BKAppIDSecret.CMDB != nil {
		paths = append(paths, applyCred(conf, "cmdb", "appCode", "appSecret", secrets.BKAppIDSecret.CMDB)...)
	}
	return paths
}

func applyCred(conf map[string]interface{}, parent, codeField, valueField string, cred *AppCredential) []string {
	if cred == nil {
		return nil
	}
	var paths []string
	paths = append(paths, setPath(conf, parent+"."+codeField, cred.AppCode)...)
	paths = append(paths, setPath(conf, parent+"."+valueField, cred.AppSecret)...)
	return paths
}

func applyMySQL(conf map[string]interface{}, mysql *MySQLSecrets) []string {
	if mysql == nil || mysql.Default == nil {
		return nil
	}
	var paths []string
	paths = append(paths, setPath(conf, "sharding.adminDatabase.user", mysql.Default.Username)...)
	paths = append(paths, setPath(conf, "sharding.adminDatabase.password", mysql.Default.Password)...)
	return paths
}

func applyRedis(conf map[string]interface{}, redis *RedisSecrets) []string {
	if redis == nil {
		return nil
	}
	var paths []string
	if redis.Default != nil {
		paths = append(paths, setPath(conf, "repository.redisCluster.password", redis.Default.Password)...)
		paths = append(paths, setPath(conf, "redisCluster.password", redis.Default.Password)...)
	}
	var rateLimit *string
	if redis.RateLimit != nil && redis.RateLimit.Password != nil {
		rateLimit = redis.RateLimit.Password
	} else if redis.Default != nil {
		rateLimit = redis.Default.Password
	}
	paths = append(paths, setPath(conf, "componentRateLimit.redisCluster.password", rateLimit)...)
	return paths
}

func applyBKRepo(conf map[string]interface{}, repo *BKRepoSecrets) []string {
	if repo == nil {
		return nil
	}
	var paths []string
	paths = append(paths, applyUserPass(conf, "repository.bkRepo", repo.Default)...)
	paths = append(paths, applyUserPass(conf, "repository.slave.bkRepo", repo.Slave)...)
	return paths
}

func applyS3(conf map[string]interface{}, s3 *S3Secrets) []string {
	if s3 == nil {
		return nil
	}
	var paths []string
	if s3.Default != nil {
		paths = append(paths, setPath(conf, "repository.s3.accessKeyID", s3.Default.AccessKeyID)...)
		paths = append(paths, setPath(conf, "repository.s3.secretAccessKey", s3.Default.SecretAccessKey)...)
	}
	if s3.Slave != nil {
		paths = append(paths, setPath(conf, "repository.slave.s3.accessKeyID", s3.Slave.AccessKeyID)...)
		paths = append(paths, setPath(conf, "repository.slave.s3.secretAccessKey", s3.Slave.SecretAccessKey)...)
	}
	return paths
}

func applyBCS(conf map[string]interface{}, bcs *BCSSecrets) []string {
	if bcs == nil {
		return nil
	}
	var paths []string
	if bcs.Default != nil {
		paths = append(paths, setPath(conf, "bcs.token", bcs.Default.Token)...)
	}
	if bcs.PushManager != nil {
		paths = append(paths, setPath(conf, "pushProvider.config.token", bcs.PushManager.Token)...)
	}
	return paths
}

func applyUserPass(conf map[string]interface{}, parent string, cred *UsernamePassword) []string {
	if cred == nil {
		return nil
	}
	var paths []string
	paths = append(paths, setPath(conf, parent+".username", cred.Username)...)
	paths = append(paths, setPath(conf, parent+".password", cred.Password)...)
	return paths
}

func setPath(root map[string]interface{}, path string, value *string) []string {
	if value == nil {
		return nil
	}
	parts := strings.Split(path, ".")
	cur := root
	for _, key := range parts[:len(parts)-1] {
		next, ok := cur[key]
		if !ok {
			return nil
		}
		child, ok := next.(map[string]interface{})
		if !ok {
			return nil
		}
		cur = child
	}
	leaf := parts[len(parts)-1]
	if _, ok := cur[leaf]; !ok {
		return nil
	}
	cur[leaf] = *value
	return []string{path}
}
