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

package migrator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/TencentBlueKing/bk-bscp/cmd/gsekit-migration/config"
)

// CheckResult holds the result of a single connectivity check
type CheckResult struct {
	Name    string
	Pass    bool
	Latency time.Duration
	Details string
}

// PreflightReport holds all connectivity check results
type PreflightReport struct {
	Success bool
	Checks  []CheckResult
}

// Preflight runs connectivity checks for all configured external dependencies
func Preflight(cfg *config.Config) *PreflightReport {
	report := &PreflightReport{Success: true}

	report.Checks = append(report.Checks, checkMySQL(cfg.Source.MySQL, "Source MySQL (GSEKit)"))
	report.Checks = append(report.Checks, checkMySQL(cfg.Target.MySQL, "Target MySQL (BSCP)"))

	if cfg.CMDB.Endpoint == "" {
		report.Checks = append(report.Checks, CheckResult{
			Name:    "CMDB API",
			Details: "cmdb.endpoint is not configured",
		})
	} else {
		report.Checks = append(report.Checks, checkCMDB(&cfg.CMDB))
	}

	if cfg.GSEKit.Endpoint == "" {
		report.Checks = append(report.Checks, CheckResult{
			Name:    "GSEKit API",
			Details: "gsekit.endpoint is not configured",
		})
	} else {
		report.Checks = append(report.Checks, checkGSEKit(&cfg.GSEKit))
	}

	if cfg.Repository.StorageType == "" {
		report.Checks = append(report.Checks, CheckResult{
			Name:    "Repository",
			Details: "repository.storage_type is not configured",
		})
	} else {
		report.Checks = append(report.Checks, checkRepository(&cfg.Repository))
	}

	for _, c := range report.Checks {
		if !c.Pass {
			report.Success = false
			break
		}
	}

	return report
}

// PrintPreflightReport prints the preflight report to stdout
func PrintPreflightReport(report *PreflightReport) {
	fmt.Println("\n========== Preflight Check Report ==========")
	fmt.Printf("Status: %s\n", boolToStatus(report.Success))
	fmt.Println("\nChecks:")
	for _, c := range report.Checks {
		status := "PASS"
		if !c.Pass {
			status = "FAIL"
		}
		fmt.Printf("  [%s] %s (latency: %v)\n", status, c.Name, c.Latency)
		if c.Details != "" {
			fmt.Printf("         %s\n", c.Details)
		}
	}
	fmt.Println("==============================================")
}

// --- individual checks ---

func checkMySQL(mysqlCfg config.MySQLConfig, name string) CheckResult {
	start := time.Now()
	result := CheckResult{Name: name}

	db, err := gorm.Open(mysql.Open(mysqlCfg.DSN()),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		result.Latency = time.Since(start)
		result.Details = fmt.Sprintf("connection failed: %v", err)
		return result
	}

	sqlDB, err := db.DB()
	if err != nil {
		result.Latency = time.Since(start)
		result.Details = fmt.Sprintf("get db handle failed: %v", err)
		return result
	}
	defer sqlDB.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		result.Latency = time.Since(start)
		result.Details = fmt.Sprintf("ping failed: %v", err)
		return result
	}

	result.Latency = time.Since(start)
	result.Pass = true
	result.Details = fmt.Sprintf("database=%s, endpoints=%v", mysqlCfg.Database, mysqlCfg.Endpoints)
	return result
}

func checkCMDB(cfg *config.CMDBConfig) CheckResult {
	start := time.Now()
	result := CheckResult{Name: "CMDB API"}

	client := &http.Client{Timeout: 15 * time.Second}
	endpoint := strings.TrimRight(cfg.Endpoint, "/")

	url := fmt.Sprintf("%s/api/v3/find/objectattr", endpoint)
	body := strings.NewReader(`{"bk_obj_id":"process","page":{"start":0,"limit":1}}`)

	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		result.Latency = time.Since(start)
		result.Details = fmt.Sprintf("create request failed: %v", err)
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bkapi-Authorization", fmt.Sprintf(
		`{"bk_app_code":"%s","bk_app_secret":"%s","bk_username":"%s"}`,
		cfg.AppCode, cfg.AppSecret, cfg.Username))

	resp, err := client.Do(req)
	if err != nil {
		result.Latency = time.Since(start)
		result.Details = fmt.Sprintf("request failed: %v", err)
		return result
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	result.Latency = time.Since(start)

	var baseResp struct {
		Result  bool   `json:"result"`
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(respBody, &baseResp); err != nil {
		result.Details = fmt.Sprintf("HTTP %d, unmarshal failed: %v", resp.StatusCode, err)
		return result
	}

	if !baseResp.Result {
		result.Details = fmt.Sprintf("API returned error: code=%d, message=%s", baseResp.Code, baseResp.Message)
		return result
	}

	result.Pass = true
	result.Details = fmt.Sprintf("endpoint=%s", endpoint)
	return result
}

func checkRepository(cfg *config.RepositoryConfig) CheckResult {
	switch strings.ToUpper(cfg.StorageType) {
	case "BKREPO":
		return checkBkRepo(&cfg.BkRepo)
	case "S3", "COS":
		return checkS3(&cfg.S3)
	default:
		return CheckResult{
			Name:    fmt.Sprintf("Repository (%s)", cfg.StorageType),
			Details: fmt.Sprintf("unknown storage_type: %s", cfg.StorageType),
		}
	}
}

func checkBkRepo(cfg *config.BkRepoConfig) CheckResult {
	start := time.Now()
	result := CheckResult{Name: "BKRepo"}

	host := strings.TrimRight(cfg.Endpoint, "/")
	url := fmt.Sprintf("%s/repository/api/repo/info/bscp-preflight-check/nonexistent", host)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		result.Latency = time.Since(start)
		result.Details = fmt.Sprintf("create request failed: %v", err)
		return result
	}
	req.SetBasicAuth(cfg.Username, cfg.Password)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		result.Latency = time.Since(start)
		result.Details = fmt.Sprintf("request failed: %v", err)
		return result
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	result.Latency = time.Since(start)

	var bkResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(respBody, &bkResp); err != nil {
		if resp.StatusCode == http.StatusUnauthorized {
			result.Details = "authentication failed (HTTP 401): check username/password"
			return result
		}
		result.Details = fmt.Sprintf("HTTP %d, unmarshal failed: %v", resp.StatusCode, err)
		return result
	}

	// 0 = success (unexpected for fake project), 251005 = project not exist, 251006 = repo not exist
	// All indicate the API is reachable and auth is valid
	if bkResp.Code == 0 || bkResp.Code == 251005 || bkResp.Code == 251006 {
		result.Pass = true
		result.Details = fmt.Sprintf("endpoint=%s", host)
		return result
	}

	if resp.StatusCode == http.StatusUnauthorized {
		result.Details = "authentication failed (HTTP 401): check username/password"
		return result
	}

	result.Pass = true
	result.Details = fmt.Sprintf("endpoint=%s (code=%d, msg=%s)", host, bkResp.Code, bkResp.Message)
	return result
}

func checkS3(cfg *config.S3Config) CheckResult {
	start := time.Now()
	result := CheckResult{Name: "S3/COS"}

	scheme := "http"
	if cfg.UseSSL {
		scheme = "https"
	}
	host := fmt.Sprintf("%s://%s.%s", scheme, cfg.BucketName, cfg.Endpoint)

	req, err := http.NewRequest(http.MethodHead, host+"/", nil)
	if err != nil {
		result.Latency = time.Since(start)
		result.Details = fmt.Sprintf("create request failed: %v", err)
		return result
	}

	now := time.Now().UTC()
	startTime := now.Unix()
	endTime := now.Add(time.Hour).Unix()
	keyTime := fmt.Sprintf("%d;%d", startTime, endTime)

	mac := hmacSHA256([]byte(cfg.SecretAccessKey), []byte(keyTime))
	signKey := fmt.Sprintf("%x", mac)

	httpString := fmt.Sprintf("%s\n%s\n\n\n", req.Method, "/")
	httpStringSHA := fmt.Sprintf("%x", sha256Sum([]byte(httpString)))
	stringToSign := fmt.Sprintf("sha256\n%s\n%s\n", keyTime, httpStringSHA)

	finalSig := hmacSHA256([]byte(signKey), []byte(stringToSign))
	finalSignature := fmt.Sprintf("%x", finalSig)

	auth := fmt.Sprintf(
		"q-sign-algorithm=sha256&q-ak=%s&q-sign-time=%s&q-key-time=%s&q-header-list=&q-url-param-list=&q-signature=%s",
		cfg.AccessKeyID, keyTime, keyTime, finalSignature)
	req.Header.Set("Authorization", auth)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		result.Latency = time.Since(start)
		result.Details = fmt.Sprintf("request failed: %v", err)
		return result
	}
	defer resp.Body.Close()

	result.Latency = time.Since(start)

	if resp.StatusCode == http.StatusForbidden {
		result.Details = "authentication failed (HTTP 403): check access_key_id/secret_access_key"
		return result
	}

	result.Pass = true
	result.Details = fmt.Sprintf("endpoint=%s, HTTP %d", host, resp.StatusCode)
	return result
}

func checkGSEKit(cfg *config.GSEKitConfig) CheckResult {
	start := time.Now()
	result := CheckResult{Name: "GSEKit API"}

	endpoint := strings.TrimRight(cfg.Endpoint, "/")
	url := fmt.Sprintf("%s/api/0/config_version/preview/", endpoint)

	body := strings.NewReader(`{"content":"test","bk_process_id":0}`)
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		result.Latency = time.Since(start)
		result.Details = fmt.Sprintf("create request failed: %v", err)
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bkapi-Authorization", fmt.Sprintf(
		`{"bk_app_code":"%s","bk_app_secret":"%s","bk_ticket":"%s"}`,
		cfg.AppCode, cfg.AppSecret, cfg.BkTicket))

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		result.Latency = time.Since(start)
		result.Details = fmt.Sprintf("request failed: %v", err)
		return result
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	result.Latency = time.Since(start)

	// HTTP 401/403 with gateway error indicates auth failure
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		var gw struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		if json.Unmarshal(respBody, &gw) == nil && gw.Message != "" {
			result.Details = fmt.Sprintf("authentication failed (HTTP %d): %s", resp.StatusCode, gw.Message)
			return result
		}
		result.Details = fmt.Sprintf("authentication failed (HTTP %d): check app_code/app_secret/bk_ticket", resp.StatusCode)
		return result
	}

	// Any other response (including business errors like "biz not found") means
	// the gateway is reachable and auth passed.
	result.Pass = true
	result.Details = fmt.Sprintf("endpoint=%s", endpoint)
	return result
}
