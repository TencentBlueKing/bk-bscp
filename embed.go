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

// Package bscp use embed ui
package bscp

import (
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/TencentBlueKing/bk-bscp/pkg/config"
)

//go:embed ui/dist
var frontendAssets embed.FS

//go:embed ui_old/dist
var frontendAssetsOld embed.FS

// FrontendVariant 前端变体，标识使用新 UI 还是旧 UI 的静态资源
type FrontendVariant string

const (
	// FrontendNew 新 UI（含项目/环境能力），对应 ui/dist
	FrontendNew FrontendVariant = "new"
	// FrontendOld 旧 UI（不含项目/环境能力），对应 ui_old/dist
	FrontendOld FrontendVariant = "old"
)

// ParseFrontendVariant 解析前端变体启动参数。空字符串默认新 UI（向后兼容），非法值返回错误。
func ParseFrontendVariant(s string) (FrontendVariant, error) {
	switch s {
	case "", string(FrontendNew):
		return FrontendNew, nil
	case string(FrontendOld):
		return FrontendOld, nil
	default:
		return "", fmt.Errorf("invalid frontend variant %q, expected %q or %q",
			s, FrontendNew, FrontendOld)
	}
}

var (
	allowCompressExtentions = map[string]bool{
		".js":  true,
		".css": true,
	}
)

// IndexConfig 前端配置
type IndexConfig struct {
	RunEnv               string
	StaticURL            string
	IAMHost              string
	CMDBHost             string
	APIURL               string
	SiteURL              string // vue 路由前缀
	EnableBKNotice       bool   // 是否启用蓝鲸通知中心
	Helper               string
	ProxyAPI             bool
	GrpcAddr             string // feed-server grpc 地址
	HttpAddr             string // feed-server http 地址
	BKSharedResBaseJSURL string // 规则是${bkSharedResUrl}/${目录名 aka app_code}/base.js
	NodeManHost          string
	UserManHost          string
	UserCenterURL        string // 用户中心(个人中心)跳转地址, 仅多租户环境配置
}

// EmbedWebServer 前端 web server
type EmbedWebServer interface {
	RenderIndexHandler(conf *IndexConfig) http.Handler
	Render403Handler(conf *IndexConfig) http.Handler
	FaviconHandler(w http.ResponseWriter, r *http.Request)
	StaticFileHandler(prefix string) http.Handler
}

type gzipFileInfo struct {
	contentType  string
	contentSize  string
	lastModified string
	filePath     string
}

// EmbedWeb ..
type EmbedWeb struct {
	assets   embed.FS
	baseDir  string
	dist     fs.FS
	tpl      *template.Template
	root     http.FileSystem
	fsServer http.Handler
}

// NewEmbedWeb 初始化模版和fs，默认使用新 UI（向后兼容）
func NewEmbedWeb() *EmbedWeb {
	return NewEmbedWebWithVariant(FrontendNew)
}

// NewEmbedWebWithVariant 按前端变体初始化模版和 fs，选择对应的静态资源
func NewEmbedWebWithVariant(variant FrontendVariant) *EmbedWeb {
	assets := frontendAssets
	baseDir := "ui/dist"
	if variant == FrontendOld {
		assets = frontendAssetsOld
		baseDir = "ui_old/dist"
	}

	// dist 路径
	dist, err := fs.Sub(assets, baseDir)
	if err != nil {
		panic(err)
	}

	// 模版路径
	tpl := template.Must(template.New("").ParseFS(assets, baseDir+"/*.html"))

	root := http.FS(dist)

	w := &EmbedWeb{
		assets:   assets,
		baseDir:  baseDir,
		dist:     dist,
		tpl:      tpl,
		root:     root,
		fsServer: http.FileServer(root),
	}
	return w
}

// FaviconHandler favicon Handler
func (e *EmbedWeb) FaviconHandler(w http.ResponseWriter, r *http.Request) {
	// 填写实际的 icon 路径
	r.URL.Path = "/favicon.ico"

	// 添加缓存
	w.Header().Set("Content-Type", "image/x-icon")
	w.Header().Set("Cache-Control", "max-age=86400, public")

	e.fsServer.ServeHTTP(w, r)
}

// readConfigFile 读取前端配置文件
func (e *EmbedWeb) readConfigFile() (map[string]string, error) {
	data, err := e.assets.ReadFile(e.baseDir + "/config.json")
	if err != nil {
		return nil, err
	}

	c := new(map[string]string)
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}

	return *c, nil
}

// mergeConfig 合并默认和自定义配置
func (e *EmbedWeb) mergeConfig() ([]byte, error) {
	c, err := e.readConfigFile()
	if err != nil {
		return nil, err
	}
	for k, v := range config.G.Frontend.Docs {
		c[k] = v
	}

	configBytes, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	return configBytes, nil
}

// RenderIndexHandler vue html 模板渲染
func (e *EmbedWeb) RenderIndexHandler(conf *IndexConfig) http.Handler {
	configBytes, err := e.mergeConfig()
	if err != nil {
		panic(fmt.Errorf("init bscp config err, %s", err))
	}
	bscpConfig := string(configBytes)

	fn := func(w http.ResponseWriter, r *http.Request) {
		tplData := map[string]interface{}{
			"BK_STATIC_URL":             conf.StaticURL,
			"RUN_ENV":                   conf.RunEnv,
			"BK_BCS_BSCP_API":           conf.APIURL,
			"BK_IAM_HOST":               conf.IAMHost,
			"BK_CC_HOST":                conf.CMDBHost,
			"BK_SHARED_RES_BASE_JS_URL": conf.BKSharedResBaseJSURL,
			"BK_BSCP_CONFIG":            bscpConfig,
			"SITE_URL":                  conf.SiteURL,
			"ENABLE_BK_NOTICE":          conf.EnableBKNotice,
			"HELPER":                    conf.Helper,
			"GRPC_ADDR":                 conf.GrpcAddr,
			"HTTP_ADDR":                 conf.HttpAddr,
			"BK_NODE_HOST":              conf.NodeManHost,
			"USER_MAN_HOST":             conf.UserManHost,
			"USER_CENTER_URL":           conf.UserCenterURL,
		}

		// 本地开发模式 / 代理请求
		if conf.ProxyAPI {
			tplData["BK_BCS_BSCP_API"] = "/bscp"
		}

		e.tpl.ExecuteTemplate(w, "index.html", tplData) //nolint
	}

	return http.HandlerFunc(fn)
}

// get403Msg base64 decode msg
func get403Msg(r *http.Request) string {
	rawMsg := r.URL.Query().Get("msg")
	if rawMsg == "" {
		return ""
	}

	b, err := base64.StdEncoding.DecodeString(rawMsg)
	if err != nil {
		return ""
	}

	return string(b)
}

// Render403Handler 403.html 页面
func (e *EmbedWeb) Render403Handler(conf *IndexConfig) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		tplData := map[string]string{
			"MSG":           get403Msg(r),
			"BK_STATIC_URL": conf.StaticURL,
		}

		e.tpl.ExecuteTemplate(w, "403.html", tplData) //nolint
	}

	return http.HandlerFunc(fn)
}

func (e *EmbedWeb) shouldCompress(r *http.Request) (bool, *gzipFileInfo) {
	// 必须包含 gzip 编码
	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		return false, nil
	}

	// 其他不支持的场景
	if strings.Contains(r.Header.Get("Connection"), "Upgrade") ||
		strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		return false, nil
	}

	upath := r.URL.Path
	fileExt := filepath.Ext(upath)
	if ok, exist := allowCompressExtentions[fileExt]; !exist || !ok {
		return false, nil
	}

	ctype := mime.TypeByExtension(fileExt)
	if ctype == "" {
		return false, nil
	}

	filePath := upath + ".gz"
	gzipFile, err := e.root.Open(filePath)
	if err != nil {
		return false, nil
	}

	fileInfo, err := gzipFile.Stat()
	if err != nil {
		return false, nil
	}

	info := &gzipFileInfo{
		filePath:     filePath,
		contentType:  ctype,
		contentSize:  strconv.FormatInt(fileInfo.Size(), 10),
		lastModified: fileInfo.ModTime().Format(http.TimeFormat),
	}

	return true, info
}

// StaticFileHandler 静态文件处理函数
func (e *EmbedWeb) StaticFileHandler(prefix string) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if ok, fileInfo := e.shouldCompress(r); ok {
			r.URL.Path = fileInfo.filePath

			w.Header().Add("Vary", "Accept-Encoding")
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Content-Length", fileInfo.contentSize)
			w.Header().Set("Content-Type", fileInfo.contentType)
			// 添加缓存
			w.Header().Set("Cache-Control", "max-age=86400, public")
			// issue https://github.com/golang/go/issues/44854
			// w.Header().Set("Last-Modified", fileInfo.lastModified)
			w.Header().Del("Transfer-Encoding")
		}

		e.fsServer.ServeHTTP(w, r)
	}

	return http.StripPrefix(prefix, http.HandlerFunc(fn))
}
