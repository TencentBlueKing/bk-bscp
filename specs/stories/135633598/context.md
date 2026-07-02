# Context for Story 135633598

## Stage
validate

## Source artifacts
- specs/stories/135633598/req.md                     # 所有阶段必读：原始需求 + 技术澄清章节
- specs/stories/135633598/spec.md                     # 技术规范
- specs/stories/135633598/plan.md                     # 实现计划（TDD 顺序、落点、Phase 0~5）
- specs/stories/135633598/research.md                 # 技术调研（落点/差异/网关链路依据）
- specs/stories/135633598/data-model.md               # 数据模型/DTO 定义
- specs/stories/135633598/tasks.md                    # 任务清单 T001~T011
- specs/stories/135633598/validate-arch-report.md        # validate fix 阶段必读（若存在）
- specs/stories/135633598/validate-security-report.md    # 同上
- specs/stories/135633598/validate-codereview-report.md  # 同上

## Project background
- docs/reqs/文件型配置MCP.md                          # 用途：文件型 MCP 需求原始文档（背景与调研结论）
- .claude/skills/bscp-kv-config/SKILL.md              # 用途：现有 KV skill 参考（领域模型/编排结构/字段约束）
- .claude/skills/bk-security-redlines/SKILL.md        # 用途：三大安全红线（下载 URL 接口鉴权/预签名安全）
- pkg/protocol/config-server/config_service.proto     # 用途：配置项/版本/发布接口契约与 visibility(INTERNAL/BKAPIGW)
- pkg/protocol/feed-server/feed_server.proto          # 用途：GetDownloadURL gRPC 契约（下载 URL 参考）
- cmd/api-server/service/repo.go                       # 用途：现有管理面 DownloadFile 流式下载实现（新接口落点参考）
- cmd/api-server/service/routers.go                    # 用途：管理面 content 路由与鉴权链注册
- internal/dal/repository/repository.go               # 用途：ObjectDownloader.DownloadLink 预签名接口 + GetFileSign
- internal/dal/repository/bkrepo.go                    # 用途：bkrepo 预签名 URL 实现（3600s 有效期）
- internal/dal/repository/cos.go                       # 用途：cos 预签名 URL 实现
- cmd/feed-server/service/rpc_sidecar.go               # 用途：feed-server GetDownloadURL handler（复用 DownloadLink 参考）
- bscp-go/internal/downloader/downloader.go           # 用途：客户端下载 URL 编排参考
- bscp-go/internal/upstream/api.go                     # 用途：GetDownloadURL 客户端封装参考
- scripts/bk_gateway/inject_bk_gateway.py             # 用途：网关注册注入脚本（F-008 MCP 生成）
- Makefile                                            # 用途：swagger/bkapigw 文档生成命令（make sg）
- CLAUDE.md                                           # 用途：仓库协作规则（中文文档、Go 规范、gofmt、golangci）

## Code scope
- cmd/api-server/service/repo.go                      # 新增下载 URL handler + swag 注解 + 响应 DTO
- cmd/api-server/service/repo_test.go                 # 新增 handler 单元测试（TDD）
- cmd/api-server/service/routers.go                   # 新增 /content/download_url 路由
- internal/dal/repository/bkrepo.go                   # 导出有效期常量 TempDownloadURLExpireSeconds
- .claude/skills/bscp-file-config/SKILL.md            # 新增文件型配置 skill（文档产物）
- docs/swagger/**                                     # make docs 生成物（网关注册，检查 diff）

## Improvement notes
无
