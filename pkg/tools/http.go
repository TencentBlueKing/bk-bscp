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

package tools

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"k8s.io/klog/v2"

	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/constant"
)

var (
	// maskKeys 敏感参数和头部key
	maskKeys = map[string]struct{}{
		"bk_app_secret": {},
		"bk_token":      {},
		"Authorization": {},
	}
)

// RequestIDValue 获取 RequestId 值
func RequestIDValue(req *http.Request) string {
	for _, k := range constant.RidKeys {
		v := req.Header.Get(k)
		if v != "" {
			return v
		}
	}
	return ""
}

// reqToCurl curl 格式的请求日志
func reqToCurl(r *http.Request) string {
	headers := ""
	for key, values := range r.Header {
		for _, value := range values {
			if _, ok := maskKeys[key]; ok {
				value = "***"
			}
			headers += fmt.Sprintf(" -H %q", fmt.Sprintf("%s: %s", key, value))
		}
	}

	// 过滤掉敏感信息
	rawURL := *r.URL
	queryValue := rawURL.Query()
	for key := range queryValue {
		if _, ok := maskKeys[key]; ok {
			queryValue.Set(key, "<masked>")
		}
	}
	rawURL.RawQuery = queryValue.Encode()

	reqMsg := fmt.Sprintf("curl -X %s '%s'%s", r.Method, rawURL.String(), headers)
	if r.Body != nil {
		contentType := r.Header.Get("Content-Type")
		if RemoveSpace(contentType) == "application/json" ||
			RemoveSpace(contentType) == "application/json;charset=utf-8" {
			// 仅在内容为 JSON 时读取 body
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				reqMsg += " -d (error reading body)"
			} else {
				// 重新填充 Body，以便后续可以继续读取
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				reqMsg += fmt.Sprintf(" -d '%s'", string(bodyBytes))
			}
		} else {
			reqMsg += " -d (io.Reader)"
		}
	}

	if r.Form.Encode() != "" {
		encodeStr := r.Form.Encode()
		reqMsg += fmt.Sprintf(" -d %q", encodeStr)
		rawStr, _ := url.QueryUnescape(encodeStr)
		reqMsg += fmt.Sprintf(" -raw `%s`", rawStr)
	}

	return reqMsg
}

// respToCurl 返回日志
func respToCurl(resp *http.Response, st time.Time) string {
	respMsg := fmt.Sprintf("[%s] size=%s duration=%s\n",
		resp.Status, humanize.Bytes(uint64(resp.ContentLength)), time.Since(st))

	var responseHeaders []string
	for header, values := range resp.Header {
		for _, value := range values {
			responseHeaders = append(responseHeaders, fmt.Sprintf("%s: %s", header, value))
		}
	}
	responseHeaderStr := strings.Join(responseHeaders, "\n")

	respMsg += fmt.Sprintf("HTTP/1.1 %d %s\n%s\n",
		resp.StatusCode,
		http.StatusText(resp.StatusCode),
		responseHeaderStr,
	)

	// 读取并处理响应体
	if resp.Body != nil {
		// 检查 Content-Type
		contentType := resp.Header.Get("Content-Type")
		// 仅在内容为 JSON 或 HTML 或简单文本时添加到日志中
		if RemoveSpace(contentType) == "application/json" ||
			RemoveSpace(contentType) == "application/json;charset=utf-8" {
			// 读取响应体内容
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				respMsg += "(error reading body)"
			} else {
				// 重新填充 Body，以便后续可以继续读取
				resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

				respMsg += fmt.Sprintf(string(bodyBytes))
			}
		} else {
			respMsg += "(io.Reader)"
		}
	}

	return respMsg
}

// curlLogTransport print curl log transport
type curlLogTransport struct {
	Transport http.RoundTripper
}

// RoundTrip curlLog Transport
func (t *curlLogTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	st := time.Now()
	rid := RequestIDValue(req)
	klog.Infof("[%s] REQ: %s", rid, reqToCurl(req))

	resp, err := t.transport(req).RoundTrip(req)

	if err != nil {
		klog.Infof("[%s] RESP: [err] %s", rid, err)
	} else {
		klog.Infof("[%s] RESP: %s", rid, respToCurl(resp, st))
	}

	return resp, err
}

func (t *curlLogTransport) transport(req *http.Request) http.RoundTripper { //nolint:unparam
	if t.Transport != nil {
		return t.Transport
	}
	return http.DefaultTransport
}

// NewCurlLogTransport make a new curl log transport, default transport can be nil
func NewCurlLogTransport(transport http.RoundTripper) http.RoundTripper {
	return &curlLogTransport{Transport: transport}
}
