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

package service

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/TencentBlueKing/bk-bscp/internal/audit"
	"github.com/TencentBlueKing/bk-bscp/internal/rest/view"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/handler"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
)

const (
	// uploadTimeout 整文件单次上传 PUT 的超时上限, 给宽松值避免误杀大文件慢网上传
	uploadTimeout = 30 * time.Minute
	// multipartTimeout 分块上传 init/upload/complete 单请求的超时上限
	multipartTimeout = 10 * time.Minute
)

// routers return router config handler
// nolint: funlen
func (p *proxy) routers() http.Handler {
	r := chi.NewRouter()
	r.Use(handler.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(handler.CORS(cc.ApiServer().CORSAllowedOrigins))
	r.Use(audit.Audit)
	// r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/-/healthy", p.HealthyHandler)
	r.Get("/-/ready", p.ReadyHandler)
	r.Get("/healthz", p.Healthz)
	r.Mount("/", handler.RegisterCommonHandler())

	// iam 回调接口
	r.Route("/api/v1/auth/iam/find/resource", func(r chi.Router) {
		r.Use(handler.RequestBodyLogger())
		r.Use(p.authorizer.IAMVerify)
		r.Mount("/", p.authSvrMux)
	})

	// 用户信息
	r.With(p.authorizer.UnifiedAuthentication).Get("/api/v1/auth/user/info", UserInfoHandler)
	r.With(p.authorizer.UnifiedAuthentication).Get("/api/v1/feature_flags", p.FeatureFlagsHandler)
	// 登入接口, 不带鉴权信息
	r.Get("/api/v1/logout", p.LogoutHandler)

	// authserver通用接口
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Use(p.authorizer.UnifiedAuthentication)
		r.Use(view.Generic(p.authorizer))
		r.Mount("/", p.authSvrMux)
	})

	r.Route("/api/v1/config/biz/{biz_id}/apps/query", func(r chi.Router) {
		r.Use(p.authorizer.UnifiedAuthentication)
		r.Use(p.authorizer.BizVerified)
		r.Use(p.checkOrCreateDefaultProjectEnv)
		r.Use(view.Generic(p.authorizer))
		r.Mount("/", p.cfgSvrMux)
	})

	// 规范后的路由，url 需要包含 {app_id} 变量, 使用 AppVerified 中间件校验和初始化 kit.SpaceID 变量
	r.Route("/api/v1/config/biz/{biz_id}/apps/{app_id}", func(r chi.Router) {
		r.Use(p.authorizer.UnifiedAuthentication)
		r.Use(p.authorizer.BizVerified)
		r.Use(p.authorizer.AppVerified)
		r.Use(p.checkOrCreateDefaultProjectEnv)
		r.Use(p.AppProjectEnvVerified) // 校验 App 归属于该项目+环境
		r.Use(view.Generic(p.authorizer))
		r.Mount("/", p.cfgSvrMux)
	})

	// 模版空间相关接口，检查当前业务下的默认空间，没有则创建
	r.Route("/api/v1/config/biz/{biz_id}/template_spaces", func(r chi.Router) {
		r.Use(p.authorizer.UnifiedAuthentication)
		r.Use(p.authorizer.BizVerified)
		r.Use(p.checkOrCreateDefaultProjectEnv)
		r.Use(view.Generic(p.authorizer))
		r.Use(p.CheckDefaultTmplSpace)
		r.Mount("/", p.cfgSvrMux)
	})

	// configs 表 KV 管理接口
	r.Route("/api/v1/config/manage_config_kv", func(r chi.Router) {
		r.Use(p.authorizer.UnifiedAuthentication)
		r.Mount("/", p.cfgSvrMux)
	})

	r.Route("/api/v1/config/", func(r chi.Router) {
		r.Use(p.authorizer.UnifiedAuthentication)
		r.Use(p.authorizer.BizVerified)
		r.Use(p.checkOrCreateDefaultProjectEnv)
		r.Use(p.HttpServerHandledTotal("", ""))
		r.Use(view.Generic(p.authorizer))

		// 项目和环境相关路由
		r.Route("/biz/{biz_id}/projects", func(r chi.Router) {
			r.Mount("/", p.cfgSvrMux)
			r.Route("/{project_id}", func(r chi.Router) {
				r.Use(p.authorizer.VerifyProjectExists) // 校验 Project 是否存在
				r.Mount("/", p.cfgSvrMux)
				// 环境相关
				r.Route("/envs/{env_id}", func(r chi.Router) {
					r.Use(p.authorizer.VerifyEnvExists) // 校验 Env 归属于该项目
					r.Mount("/", p.cfgSvrMux)
					r.Route("/apps/{app_id}", func(r chi.Router) {
						r.Use(p.AppProjectEnvVerified) // 校验 App 归属于该项目+环境
						r.Mount("/", p.cfgSvrMux)
					})
				})
				// 脚本相关
				r.Route("/hooks/{hook_id}", func(r chi.Router) {
					r.Use(p.HookProjectVerified) // 校验 Hook 归属于该项目
					r.Mount("/", p.cfgSvrMux)
				})
				// 分组相关
				r.Route("/groups/{group_id}", func(r chi.Router) {
					r.Use(p.GroupProjectVerified) // 校验 Group 归属于该项目
					r.Mount("/", p.cfgSvrMux)
				})
				// 模版空间相关
				r.Route("/template_spaces", func(r chi.Router) {
					r.Mount("/", p.cfgSvrMux)
					r.Route("/{template_space_id}", func(r chi.Router) {
						r.Use(p.TemplateSpaceProjectVerified) // 校验 TemplateSpace 归属于该项目
						r.Mount("/", p.cfgSvrMux)
					})
				})
			})
		})
		r.Mount("/", p.cfgSvrMux)
	})

	// repo 上传 API, 此处因兼容老版本而保留，后续统一使用新接口
	r.Route("/api/v1/api/create/content/upload", func(r chi.Router) {
		r.Use(p.authorizer.UnifiedAuthentication)
		r.With(p.authorizer.BizVerified, p.authorizer.AppVerified,
			p.HttpServerHandledTotal("", "Upload")).
			Put("/biz_id/{biz_id}/app_id/{app_id}",
				p.repo.UploadFile)

	})

	// repo 下载 API, 此处因兼容老版本而保留，后续统一使用新接口
	r.Route("/api/v1/api/get/content/download", func(r chi.Router) {
		r.Use(p.authorizer.UnifiedAuthentication)
		r.With(p.authorizer.BizVerified, p.authorizer.AppVerified,
			p.HttpServerHandledTotal("", "Download")).
			Get("/biz_id/{biz_id}/app_id/{app_id}",
				p.repo.DownloadFile)
	})

	// repo 获取二进制元数据 API, 此处因兼容老版本而保留，后续统一使用新接口
	r.Route("/api/v1/api/get/content/metadata", func(r chi.Router) {
		r.Use(p.authorizer.UnifiedAuthentication)
		r.With(p.authorizer.BizVerified, p.authorizer.AppVerified,
			p.HttpServerHandledTotal("", "Metadata")).
			Get("/biz_id/{biz_id}/app_id/{app_id}",
				p.repo.FileMetadata)
	})

	// 新的内容上传、下载相关接口，后续统一使用这组新接口
	// 服务下的配置项内容需要进行服务鉴权，模版空间下的模版配置项内容需要进行模版空间鉴权
	// app_id和template_space_id信息放在header中，用于鉴权，和sign保持一致
	r.Route("/api/v1/biz/{biz_id}/content", func(r chi.Router) {
		// 上传类接口: 叠加 app 凭证认证(绕网关直连) + 请求超时。
		// 超时中间件必须排在认证之前, 使 deadline 透传到 kt.Ctx 再传到 bkrepo 请求。
		r.Group(func(r chi.Router) {
			r.Use(middleware.Timeout(uploadTimeout))
			r.Use(p.authorizer.UploadAppKeyAuthentication)
			r.Use(p.authorizer.BizVerified)
			r.Use(p.authorizer.ContentVerified)
			// 内容上传API
			r.Route("/upload", func(r chi.Router) {
				r.Use(p.HttpServerHandledTotal("", "Upload"))
				r.Put("/", p.repo.UploadFile)
			})
		})
		// 分块内容上传API: 同样叠加 app 凭证认证 + 超时
		r.Group(func(r chi.Router) {
			r.Use(middleware.Timeout(multipartTimeout))
			r.Use(p.authorizer.UploadAppKeyAuthentication)
			r.Use(p.authorizer.BizVerified)
			r.Use(p.authorizer.ContentVerified)
			r.Route("/multipart", func(r chi.Router) {
				// 初始化分块上传
				r.With(p.HttpServerHandledTotal("", "InitMultipartUpload")).Post("/init", p.repo.InitMultipartUploadFile)
				// 分块上传
				r.With(p.HttpServerHandledTotal("", "MultipartUpload")).Put("/upload", p.repo.MultipartUploadFile)
				// 完成分块上传
				r.With(p.HttpServerHandledTotal("", "CompleteMultipartUpload")).Post("/complete", p.repo.CompleteMultipartUploadFile)
			})
		})
		// 下载、元数据接口: 保持原有统一认证, 不放开直连
		// (download 走 DownloadFile 内的 IAM 用户级鉴权, 不适合 app 凭证路径)
		r.Group(func(r chi.Router) {
			r.Use(p.authorizer.UnifiedAuthentication)
			r.Use(p.authorizer.BizVerified)
			r.Use(p.authorizer.ContentVerified)
			// 内容下载API
			r.Route("/download", func(r chi.Router) {
				r.Use(p.HttpServerHandledTotal("", "Download"))
				r.Get("/", p.repo.DownloadFile)
			})
			// 内容下载URL API: 只返回临时预签名下载URL, 不透传文件字节
			r.Route("/download_url", func(r chi.Router) {
				r.Use(p.HttpServerHandledTotal("", "DownloadURL"))
				r.Get("/", p.repo.DownloadFileURL)
			})
			// 获取二进制内容元数据API
			r.Route("/metadata", func(r chi.Router) {
				r.Use(p.HttpServerHandledTotal("", "Metadata"))
				r.Get("/", p.repo.FileMetadata)
			})
		})
	})

	// 导入模板压缩包
	r.Route("/api/v1/config/biz/{biz_id}/template_spaces/{template_space_id}/templates/import/{filename}", func(r chi.Router) {
		r.Use(p.authorizer.UnifiedAuthentication)
		r.Use(p.authorizer.BizVerified)
		r.Use(p.checkOrCreateDefaultProjectEnv)
		r.Use(p.HttpServerHandledTotal("", "TemplateConfigFileImport"))
		r.Post("/", p.configImportService.TemplateConfigFileImport)
	})

	// 导入模板压缩包v2
	r.Route("/api/v1/config/biz/{biz_id}/projects/{project_id}/template_spaces/{template_space_id}/templates/import/{filename}", func(r chi.Router) {
		r.Use(p.authorizer.UnifiedAuthentication)
		r.Use(p.authorizer.BizVerified)
		r.Use(p.authorizer.VerifyProjectExists)
		r.Use(p.HttpServerHandledTotal("", "TemplateConfigFileImport"))
		r.Post("/", p.configImportService.TemplateConfigFileImport)
	})

	// 导入配置压缩包
	r.Route("/api/v1/config/biz/{biz_id}/apps/{app_id}/config_item/import/{filename}", func(r chi.Router) {
		r.Use(p.authorizer.UnifiedAuthentication)
		r.Use(p.authorizer.BizVerified)
		r.Use(p.checkOrCreateDefaultProjectEnv)
		r.Use(p.HttpServerHandledTotal("", "ConfigFileImport"))
		r.Post("/", p.configImportService.ConfigFileImport)
	})

	// 导入配置压缩包v2
	r.Route("/api/v1/config/biz/{biz_id}/projects/{project_id}/envs/{env_id}/apps/{app_id}/config_item/import/{filename}", func(r chi.Router) {
		r.Use(p.authorizer.UnifiedAuthentication)
		r.Use(p.authorizer.BizVerified)
		r.Use(p.authorizer.VerifyProjectExists)
		r.Use(p.authorizer.VerifyEnvExists)
		r.Use(p.AppProjectEnvVerified)
		r.Use(p.HttpServerHandledTotal("", "ConfigFileImport"))
		r.Post("/", p.configImportService.ConfigFileImport)
	})

	// 导出配置压缩包
	r.Route("/api/v1/config/biz/{biz_id}/apps/{app_id}/releases/{release_id}/config_item/export", func(r chi.Router) {
		r.Use(p.authorizer.UnifiedAuthentication)
		r.Use(p.authorizer.BizVerified)
		r.Use(p.checkOrCreateDefaultProjectEnv)
		r.Use(p.HttpServerHandledTotal("", "ConfigFileExport"))
		r.Get("/", p.configExportService.ConfigFileExport)
	})

	// 导出配置压缩包v2
	r.Route("/api/v1/config/biz/{biz_id}/projects/{project_id}/envs/{env_id}/apps/{app_id}/releases/{release_id}/config_item/export", func(r chi.Router) {
		r.Use(p.authorizer.UnifiedAuthentication)
		r.Use(p.authorizer.BizVerified)
		r.Use(p.authorizer.VerifyProjectExists)
		r.Use(p.authorizer.VerifyEnvExists)
		r.Use(p.AppProjectEnvVerified)
		r.Use(p.HttpServerHandledTotal("", "ConfigFileExport"))
		r.Get("/", p.configExportService.ConfigFileExport)
	})

	// 获取通知中心通知列表
	r.Route("/api/v1/announcements", func(r chi.Router) {
		r.Get("/", p.bkNotice.GetCurrentAnnouncements)
	})

	// 导出版本kv
	r.Route("/api/v1/biz/{biz_id}/apps/{app_id}/releases/{release_id}/kvs/export", func(r chi.Router) {
		r.Use(p.authorizer.UnifiedAuthentication)
		r.Use(p.authorizer.BizVerified)
		r.Use(p.checkOrCreateDefaultProjectEnv)
		r.Get("/", p.kvService.Export)
	})

	// 导出版本kv v2
	r.Route("/api/v1/biz/{biz_id}/projects/{project_id}/envs/{env_id}/apps/{app_id}/releases/{release_id}/kvs/export", func(r chi.Router) {
		r.Use(p.authorizer.UnifiedAuthentication)
		r.Use(p.authorizer.BizVerified)
		r.Use(p.authorizer.VerifyProjectExists)
		r.Use(p.authorizer.VerifyEnvExists)
		r.Use(p.authorizer.AppVerified)
		r.Get("/", p.kvService.Export)
	})

	// 导出全局变量
	r.Route("/api/v1/config/biz/{biz_id}/variables/export", func(r chi.Router) {
		r.Use(p.authorizer.UnifiedAuthentication)
		r.Use(p.authorizer.BizVerified)
		r.Get("/", p.varService.ExportGlobalVariables)
	})
	// 导出未命名版本服务变量
	r.Route("/api/v1/config/biz/{biz_id}/apps/{app_id}/variables/export", func(r chi.Router) {
		r.Use(p.authorizer.UnifiedAuthentication)
		r.Use(p.authorizer.BizVerified)
		r.Use(p.authorizer.AppVerified)
		r.Get("/", p.varService.ExportAppVariables)
	})
	// 导出已命名版本服务变量
	r.Route("/api/v1/config/biz/{biz_id}/apps/{app_id}/releases/{release_id}/variables/export", func(r chi.Router) {
		r.Use(p.authorizer.UnifiedAuthentication)
		r.Use(p.authorizer.BizVerified)
		r.Use(p.authorizer.AppVerified)
		r.Get("/", p.varService.ExportReleasedAppVariables)
	})

	// 导出模板压缩包
	r.Route("/api/v1/config/biz/{biz_id}/template_spaces/{template_space_id}/templates/{template_id}/export", func(r chi.Router) {
		r.Use(p.authorizer.UnifiedAuthentication)
		r.Use(p.authorizer.BizVerified)
		r.Use(p.checkOrCreateDefaultProjectEnv)
		r.Use(p.HttpServerHandledTotal("", "TemplateExport"))
		r.Get("/", p.configExportService.TemplateExport)
	})

	// 导出模板压缩包v2
	r.Route("/api/v1/config/biz/{biz_id}/projects/{project_id}/template_spaces/{template_space_id}/templates/{template_id}/export", func(r chi.Router) {
		r.Use(p.authorizer.UnifiedAuthentication)
		r.Use(p.authorizer.BizVerified)
		r.Use(p.authorizer.VerifyProjectExists)
		r.Use(p.HttpServerHandledTotal("", "TemplateExport"))
		r.Get("/", p.configExportService.TemplateExport)
	})

	return r
}
