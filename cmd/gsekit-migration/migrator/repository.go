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
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/TencentBlueKing/bk-bscp/cmd/gsekit-migration/config"
)

// hmacSHA256 computes HMAC-SHA256
func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data) // nolint
	return mac.Sum(nil)
}

// sha256Sum computes SHA256 digest
func sha256Sum(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// ContentUploader defines the interface for uploading content to a repository
type ContentUploader interface {
	// Upload uploads content and returns the upload result (signature, byte_size, md5)
	Upload(ctx context.Context, bizID uint32, content []byte) (*UploadResult, error)
	// Exists checks if content already exists in the repository
	Exists(ctx context.Context, bizID uint32, signature string) (bool, error)
}

// UploadResult contains the result of content upload
type UploadResult struct {
	Signature string
	ByteSize  uint64
	Md5       string
}

// ---------- BK-Repo Uploader ----------

// BkRepoUploader handles uploading content to BK-Repo (BlueKing artifact repository).
// Implementation follows bk-bscp/internal/dal/repository/bkrepo.go.
type BkRepoUploader struct {
	client      *http.Client
	host        string
	project     string
	tenantID    string
	repoCreated repoCreatedSet
}

// repoCreatedSet tracks which BK-Repo repositories have been created/verified
type repoCreatedSet struct {
	sync.Mutex
	created map[string]struct{}
}

func (s *repoCreatedSet) exist(name string) bool {
	s.Lock()
	defer s.Unlock()
	_, ok := s.created[name]
	return ok
}

func (s *repoCreatedSet) set(name string) {
	s.Lock()
	defer s.Unlock()
	s.created[name] = struct{}{}
}

// bkRepoAuthTransport adds Basic Auth to every request
type bkRepoAuthTransport struct {
	Username  string
	Password  string
	Transport http.RoundTripper
}

func (t *bkRepoAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(t.Username, t.Password)
	transport := t.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	return transport.RoundTrip(req)
}

// bkRepoBaseResp is the base response from BK-Repo API
type bkRepoBaseResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// bkRepoUploadResp is the response from BK-Repo upload API
type bkRepoUploadResp struct {
	bkRepoBaseResp
	Data *bkRepoUploadData `json:"data"`
}

type bkRepoUploadData struct {
	Size   int64  `json:"size"`
	Sha256 string `json:"sha256"`
}

// NewBkRepoUploader creates a new BK-Repo uploader.
// tenantID is used to build the project name in multi-tenant mode ({tenantID}.{project}).
func NewBkRepoUploader(conf *config.BkRepoConfig, tenantID string) *BkRepoUploader {
	transport := &bkRepoAuthTransport{
		Username:  conf.Username,
		Password:  conf.Password,
		Transport: http.DefaultTransport,
	}
	host := strings.TrimRight(conf.Endpoint, "/")
	return &BkRepoUploader{
		client: &http.Client{
			Timeout:   120 * time.Second,
			Transport: transport,
		},
		host:     host,
		project:  conf.Project,
		tenantID: tenantID,
		repoCreated: repoCreatedSet{
			created: make(map[string]struct{}),
		},
	}
}

// buildProject returns the project name. In multi-tenant mode: {tenantID}.{project}.
func (u *BkRepoUploader) buildProject() string {
	if u.tenantID != "" {
		return fmt.Sprintf("%s.%s", u.tenantID, u.project)
	}
	return u.project
}

// genRepoName returns the BK-Repo repository name: bscp-v1-biz-{bizID}
func genRepoName(bizID uint32) string {
	return fmt.Sprintf("bscp-v1-biz-%d", bizID)
}

// ensureRepo ensures the BK-Repo repository exists for the given bizID.
// Reference: bk-bscp/internal/dal/repository/bkrepo.go:ensureRepo
func (u *BkRepoUploader) ensureRepo(ctx context.Context, bizID uint32) error {
	repoName := genRepoName(bizID)
	if u.repoCreated.exist(repoName) {
		return nil
	}

	project := u.buildProject()

	// Create repository (ignore "already exists" error code 251007)
	createRepoReq := map[string]interface{}{
		"projectId": project,
		"name":      repoName,
		"type":      "GENERIC",
		"category":  "LOCAL",
		"configuration": map[string]string{
			"type": "local",
		},
		"description": fmt.Sprintf("bscp %d business repository", bizID),
	}

	reqBody, err := json.Marshal(createRepoReq)
	if err != nil {
		return fmt.Errorf("marshal create repo request failed: %w", err)
	}

	url := fmt.Sprintf("%s/repository/api/repo/create", u.host)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("create repo request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := u.client.Do(req)
	if err != nil {
		return fmt.Errorf("create repo failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Allow both 200 and 400 (BK-Repo uses 400 for some errors)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		return fmt.Errorf("create repo status %d, body: %s", resp.StatusCode, string(body))
	}

	var bkResp bkRepoBaseResp
	if err := json.Unmarshal(body, &bkResp); err != nil {
		return fmt.Errorf("unmarshal create repo response failed: %w", err)
	}

	// code 251007 = repo already exist, which is fine
	if bkResp.Code != 0 && bkResp.Code != 251007 {
		return fmt.Errorf("create repo failed: code=%d, message=%s", bkResp.Code, bkResp.Message)
	}

	u.repoCreated.set(repoName)
	return nil
}

// Upload uploads content to BK-Repo.
// Reference: bk-bscp/internal/dal/repository/bkrepo.go:Upload
func (u *BkRepoUploader) Upload(ctx context.Context, bizID uint32, content []byte) (*UploadResult, error) {
	// Ensure repo exists before uploading
	if err := u.ensureRepo(ctx, bizID); err != nil {
		return nil, fmt.Errorf("ensure repo failed: %w", err)
	}

	signature := byteSHA256(content)
	md5Hash := byteMD5(content)
	byteSize := uint64(len(content))

	// BK-Repo upload path: /generic/{project}/bscp-v1-biz-{bizID}/file/{sha256}
	project := u.buildProject()
	repoName := genRepoName(bizID)
	objectPath := fmt.Sprintf("/generic/%s/%s/file/%s", project, repoName, signature)
	rawURL := fmt.Sprintf("%s%s", u.host, objectPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, rawURL, bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("create bkrepo upload request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-BKREPO-OVERWRITE", "true")
	if u.tenantID != "" {
		req.Header.Set("X-Bk-Tenant-Id", u.tenantID)
	}
	req.ContentLength = int64(byteSize)

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload to bkrepo failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bkrepo upload status %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse response to verify success
	var uploadResp bkRepoUploadResp
	if err := json.Unmarshal(body, &uploadResp); err != nil {
		return nil, fmt.Errorf("unmarshal upload response failed: %w", err)
	}
	if uploadResp.Code != 0 {
		return nil, fmt.Errorf("bkrepo upload failed: code=%d, message=%s", uploadResp.Code, uploadResp.Message)
	}

	return &UploadResult{
		Signature: signature,
		ByteSize:  byteSize,
		Md5:       md5Hash,
	}, nil
}

// Exists checks if an object exists in BK-Repo via HEAD request.
// Reference: bk-bscp/internal/dal/repository/bkrepo.go:Metadata
func (u *BkRepoUploader) Exists(ctx context.Context, bizID uint32, signature string) (bool, error) {
	project := u.buildProject()
	repoName := genRepoName(bizID)
	objectPath := fmt.Sprintf("/generic/%s/%s/file/%s", project, repoName, signature)
	rawURL := fmt.Sprintf("%s%s", u.host, objectPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return false, fmt.Errorf("create bkrepo head request failed: %w", err)
	}
	if u.tenantID != "" {
		req.Header.Set("X-Bk-Tenant-Id", u.tenantID)
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("bkrepo head request failed: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// ---------- S3/COS Uploader ----------

// S3Uploader handles uploading content to S3-compatible storage (e.g., Tencent COS)
type S3Uploader struct {
	client *http.Client
	host   string
	conf   *config.S3Config
}

// NewS3Uploader creates a new S3/COS uploader
func NewS3Uploader(conf *config.S3Config) *S3Uploader {
	scheme := "http"
	if conf.UseSSL {
		scheme = "https"
	}
	host := fmt.Sprintf("%s://%s.%s", scheme, conf.BucketName, conf.Endpoint)
	return &S3Uploader{
		client: &http.Client{Timeout: 120 * time.Second},
		host:   host,
		conf:   conf,
	}
}

// Upload uploads content to S3/COS
func (u *S3Uploader) Upload(ctx context.Context, bizID uint32, content []byte) (*UploadResult, error) {
	signature := byteSHA256(content)
	md5Hash := byteMD5(content)
	byteSize := uint64(len(content))

	objectPath := fmt.Sprintf("/bscp-v1-biz-%d/file/%s", bizID, signature)
	rawURL := fmt.Sprintf("%s%s", u.host, objectPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, rawURL, bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("create s3 upload request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = int64(byteSize)

	u.signRequest(req, objectPath)

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload to s3 failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("s3 upload status %d, body: %s", resp.StatusCode, string(body))
	}

	return &UploadResult{
		Signature: signature,
		ByteSize:  byteSize,
		Md5:       md5Hash,
	}, nil
}

// Exists checks if an object exists in S3/COS
func (u *S3Uploader) Exists(ctx context.Context, bizID uint32, signature string) (bool, error) {
	objectPath := fmt.Sprintf("/bscp-v1-biz-%d/file/%s", bizID, signature)
	rawURL := fmt.Sprintf("%s%s", u.host, objectPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return false, fmt.Errorf("create s3 head request failed: %w", err)
	}

	u.signRequest(req, objectPath)

	resp, err := u.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("s3 head request failed: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// signRequest adds COS HMAC-SHA256 authorization to the request
func (u *S3Uploader) signRequest(req *http.Request, objectPath string) {
	now := time.Now().UTC()
	startTime := now.Unix()
	endTime := now.Add(time.Hour).Unix()
	keyTime := fmt.Sprintf("%d;%d", startTime, endTime)

	mac := hmacSHA256([]byte(u.conf.SecretAccessKey), []byte(keyTime))
	signKey := fmt.Sprintf("%x", mac)

	httpString := fmt.Sprintf("%s\n%s\n\n\n", req.Method, objectPath)
	httpStringSHA := fmt.Sprintf("%x", sha256Sum([]byte(httpString)))
	stringToSign := fmt.Sprintf("sha256\n%s\n%s\n", keyTime, httpStringSHA)

	finalSig := hmacSHA256([]byte(signKey), []byte(stringToSign))
	finalSignature := fmt.Sprintf("%x", finalSig)

	auth := fmt.Sprintf(
		"q-sign-algorithm=sha256&q-ak=%s&q-sign-time=%s&q-key-time=%s&q-header-list=&q-url-param-list=&q-signature=%s",
		u.conf.AccessKeyID, keyTime, keyTime, finalSignature)
	req.Header.Set("Authorization", auth)
}

// ---------- Helpers ----------

// NewContentUploader creates the appropriate uploader based on config.
// tenantID is passed to BkRepoUploader for multi-tenant project naming.
func NewContentUploader(conf *config.RepositoryConfig, tenantID string) ContentUploader {
	switch strings.ToUpper(conf.StorageType) {
	case "BKREPO":
		return NewBkRepoUploader(&conf.BkRepo, tenantID)
	case "S3", "COS":
		return NewS3Uploader(&conf.S3)
	default:
		log.Printf("Warning: unknown storage_type %q, defaulting to BKREPO", conf.StorageType)
		return NewBkRepoUploader(&conf.BkRepo, tenantID)
	}
}

// computeContentHashes computes hashes without uploading (for skip-upload mode)
func computeContentHashes(content []byte) *UploadResult {
	return &UploadResult{
		Signature: byteSHA256(content),
		ByteSize:  uint64(len(content)),
		Md5:       byteMD5(content),
	}
}

// logUploadStats logs upload statistics
func logUploadStats(uploaded, skipped, failed int) {
	log.Printf("  Upload stats: uploaded=%d, skipped(exists)=%d, failed=%d", uploaded, skipped, failed)
}
