# Implementation Plan — Story 135633598（bscp MCP 支持文件型配置）

**Spec**：`specs/stories/135633598/spec.md`
**Research**：`specs/stories/135633598/research.md`
**Data model**：`specs/stories/135633598/data-model.md`
**开发模式**：测试驱动开发（TDD，先写失败测试再实现）

## Technical Context

- **语言/运行时**：Go（单体多服务仓库 bk-bscp）。
- **规范基线**：`.golangci.yml`；改 Go 文件后必须 `gofmt`；中文文档/注释（CLAUDE.md）。
- **主要落点**：
  - `cmd/api-server/service/repo.go`：新增下载 URL handler（与 `DownloadFile` 并列）。
  - `cmd/api-server/service/routers.go`：`/api/v1/biz/{biz_id}/content` 组内新增下载 URL 路由。
  - `internal/dal/repository/bkrepo.go`：导出有效期常量 `TempDownloadURLExpireSeconds`（供 handler 引用，避免魔法数字）。
  - `.claude/skills/bscp-file-config/SKILL.md`：新增文件型配置 skill（文档产物）。
  - 文档生成：`make docs`（`markdown_docs` → swag/mixin/inject）纳入网关，进入 MCP。
- **不改动**：config-server 配置项/版本/发布接口（F-001~F-005 纯 skill 编排）；现有
  `/content/download` 流式返回内容接口（FR-010 兼容，保留）。
- **宪章**：`.specify/memory/constitution.md` 为未填充模板 → 以 CLAUDE.md Go/安全约束为准（详见 research.md）。

## Constitution Check（以 CLAUDE.md 为基线）

| 约束 | 结论 |
|------|------|
| 不引入不必要抽象/配置项/兼容层 | ✅ 仅新增 1 handler + 1 路由 + 导出 1 常量；复用既有 Provider/鉴权链 |
| 改 Go 文件后 gofmt + 符合 .golangci.yml | ✅ 计划任务含 gofmt/golangci 校验 |
| 优先补单包测试 | ✅ 新增 `repo_test.go` 单包测试覆盖新 handler 全分支 |
| 安全三大红线 | ✅ 输入强校验 + 双要素鉴权 + 临时 URL 到期失效（research.md R3/R4） |
| 向后兼容 | ✅ 老下载接口不变，新增独立路由（FR-010） |

**Gate 结论**：无违规，可进入实现。

## 需求 → 实现落点对照

| 需求 | 实现要点 | 落点 |
|------|---------|------|
| FR-001（F-001 查询） | skill 编排现有查询接口 | `bscp-file-config/SKILL.md` |
| FR-002/003（F-002 增删改 + 未上传报错） | skill 编排 CRUD + sign 存在性引导 | `bscp-file-config/SKILL.md` |
| FR-004（F-003 生成版本） | skill 编排 `CreateRelease` | `bscp-file-config/SKILL.md` |
| FR-005（F-004 全量发布） | skill 编排 `Publish all=true` | `bscp-file-config/SKILL.md` |
| FR-006（F-005 灰度发布） | skill 编排 `Publish all=false + 灰度参数` | `bscp-file-config/SKILL.md` |
| FR-007/008（F-006 下载 URL 核心接口） | 新 handler：预检 Metadata → DownloadLink → 取首个 URL + 3600s | `repo.go` / `routers.go` / `bkrepo.go`(常量) |
| FR-009（鉴权一致） | 路由中间件链 + handler 内 IAM Authorize | `routers.go` / `repo.go` |
| FR-010（兼容） | 老 `/content/download` 不动，新增独立 `/content/download_url` | `routers.go` |
| FR-011（F-008 网关注册） | swag 注解 + `make docs` 生成 + inject | `repo.go`(注解) + 生成命令 |
| FR-012（F-007 skill） | 文件型 skill 文档 | `bscp-file-config/SKILL.md` |

## 实现阶段（按 TDD 顺序）

### Phase 0：常量导出（前置，小改动）

- 将 `internal/dal/repository/bkrepo.go` 的 `tempDownloadURLExpireSeconds` 导出为
  `TempDownloadURLExpireSeconds`（值不变，3600），更新包内引用。
- 目的：handler 引用同一常量，避免响应体 `expire_seconds` 硬编码 3600 与存储层重复。
- 验证：`go build ./internal/dal/repository/...` + `gofmt`。

### Phase 1：下载 URL handler（TDD 核心）

1. **先写失败单测** `cmd/api-server/service/repo_test.go`：
   - 定义 stub `Provider`（嵌入 `repository.Provider` 接口后覆写 `Metadata`/`DownloadLink`）
     与 stub `auth.Authorizer`（`Authorize` 返回 nil）。
   - 用 `httptest.NewRequest` + `kit.WithKit` 注入 `kt`（含 BizID/AppID），构造
     `repoService{authorizer, provider}` 调用新 handler。
   - 断言用例：
     - **正常**：Metadata 命中 + DownloadLink 返回 `["https://x/y"]` → 响应
       `{download_url:"https://x/y", expire_seconds:3600}`，且响应体不含文件字节（AC-P01/AC-005）。
     - **sign 缺失/非法**：不设或设非法 `X-Bkapi-File-Content-Id` → `GetFileSign` 报错 → 400。
     - **内容未上传**：Metadata 返回 `errf.ErrFileContentNotFound` → 「内容未上传」错误，
       且**不调用** DownloadLink（AC-T01）。
     - **DownloadLink 失败**：返回 error → 400。
     - **多副本取首个**：DownloadLink 返回 `["u1","u2"]` → `download_url="u1"`，`expire_seconds=3600`（AC-T02）。
     - **空切片**：DownloadLink 返回 `[]` → 错误（防越界）。
   - 运行确认测试**失败**（handler 尚未实现）。
2. **再实现 handler**（`repo.go`，命名如 `DownloadFileURL`）：
   - `kt := kit.MustGetKit(r.Context())`。
   - IAM Authorize：Biz `FindBusinessResource` + App `View`（与 `DownloadFile` 相同的 `res` 结构）。
   - `sign, err := repository.GetFileSign(r)`，err → `rest.BadRequest`。
   - `Metadata(kt, sign)`：命中 `errf.ErrFileContentNotFound` → 返回明确「内容未上传」错误（不继续）。
   - `links, err := s.provider.DownloadLink(kt, sign, 1)`：err → `rest.BadRequest`；
     取首个非空为 `download_url`，空 → 报错。
   - `render.Render(w, r, rest.OKRender(&DownloadURLResponse{DownloadURL: links[0], ExpireSeconds: repository.TempDownloadURLExpireSeconds}))`。
   - 加 swag godoc 注解（`@Summary 获取文件内容下载URL`、`@Tags 文件相关`、四个 header/path 参数、
     `@Success ... data=DownloadURLResponse`、`@Router /api/v1/biz/{biz_id}/content/download_url [get]`、
     `@ID get_content_download_url`）。
3. **响应 DTO**（见 data-model.md）：`DownloadURLResponse{ DownloadURL string `json:"download_url"`; ExpireSeconds int `json:"expire_seconds"` }`，
   置于 service 包（或 repository 包，与 `MetadataResponse` 就近）；实现时取其一并保持一致。
4. 运行单测确认**全绿**；`gofmt` + `golangci-lint`（新增/改动文件）。

### Phase 2：路由注册

- 在 `routers.go` 的 `/api/v1/biz/{biz_id}/content` 下载组（`UnifiedAuthentication +
  BizVerified + ContentVerified` 那个 `r.Group`）内新增：
  ```
  r.Route("/download_url", func(r chi.Router) {
      r.Use(p.HttpServerHandledTotal("", "DownloadURL"))
      r.Get("/", p.repo.DownloadFileURL)
  })
  ```
- 老 `/download`、`/metadata` 保持不变（FR-010）。
- 集成验证：路由可达、鉴权链生效、鉴权失败返回预期错误（在可跑范围内）。

### Phase 3：网关注册与 MCP（F-008）

- 运行 `make docs`（或 `make markdown_docs`）重新生成 swagger：swag init 抓取新 handler 注解 →
  mixin 合并 → `inject_bk_gateway.py` 注入 `x-bk-apigateway-resource`（DEFAULT_EXTENSIONS）。
- 检查生成 diff 中新路径 `/api/v1/biz/{biz_id}/content/download_url` 出现且带网关扩展。
- 说明：`make sg` 在仓库中不存在，须用 `make docs`/`make markdown_docs`（见 research.md R5）。
- 实际网关注册与 MCP 工具生效需平台侧发布，属环境验证（AC-006 E2E）。

### Phase 4：文件型 skill（F-007，文档产物）

- 新增 `.claude/skills/bscp-file-config/SKILL.md`，对标 `bscp-kv-config/SKILL.md`：
  - 定位 / 前置条件（依赖 `bk-bscp-prod-server-mcp`）/ 交互引导（先要 bizId → 服务名解析 appId →
    **校验 config_type=file**）。
  - 核心规则：草稿态需生成版本+发布（R-001）；仅 file 型（R-002）；引用 sign 须已上传（R-004）；
    灰度字段在 Publish、groups 有值则 all=false（R-003）。
  - 领域模型：biz → app(file) → config_item(sign+元数据) → release。
  - 端到端编排 F-001~F-006：查询 → 增删改 → 生成版本 → 全量/灰度发布 → **取下载 URL 验证**。
  - 参数获取 / 报错→处置（含「内容未上传」引导走 UI/SDK）/ 场景化示例。
  - 明确**不含内容上传**编排；文件型 vs KV 型差异。

## 测试计划汇总

| 层级 | 范围 | 判定 |
|------|------|------|
| 单元（必做，TDD） | `repo_test.go` 覆盖新 handler 6 类分支 | `go test ./cmd/api-server/service/...` 全绿 |
| 集成（尽力） | 路由挂载 + 中间件链 + 鉴权失败 | 可跑范围内通过，不足在 tasks 细化 |
| E2E（环境/人工） | skill 闭环 + 网关注册后 MCP 工具可见 | AC-005/AC-006，环境验证 |
| 静态 | `gofmt` + `golangci-lint`（改动文件） | 无新增告警 |

## 风险与应对（承接 spec 技术风险）

- **TR-001 多副本取值**：handler 取首个非空 URL；单测覆盖多元素切片（AC-T02）。
- **TR-002 cos 忽略 fetchLimit**：管理面传 1，行为差异无实质影响，skill/注释标注。
- **TR-003 sign 未上传指向空对象**：handler 内 `Metadata` 预检 + skill 侧纵深防御（AC-T01）。
- **常量导出影响面**：`TempDownloadURLExpireSeconds` 仅在 repository 包内使用，导出为向后兼容更名，无外部破坏。

## 交付物清单

- Go：`repo.go`（新 handler + 注解 + DTO）、`routers.go`（新路由）、`bkrepo.go`（常量导出）、`repo_test.go`（单测）。
- 生成物：`docs/swagger/**`（`make docs` 重新生成，含新接口，检查 diff）。
- 文档：`.claude/skills/bscp-file-config/SKILL.md`。
