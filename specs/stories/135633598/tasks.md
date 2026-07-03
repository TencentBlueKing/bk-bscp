# Tasks: bscp MCP 支持文件型配置（Story 135633598）

**Input**: 设计文档来自 `specs/stories/135633598/`
**Prerequisites**: plan.md（必需）、spec.md（用户故事）、research.md、data-model.md

**Tests**: 本需求显式要求 TDD（plan.md「开发模式：测试驱动开发」）。核心 handler 的单元测试为必做项，先写失败测试再实现。skill 与网关注册为文档/生成物产物，验收靠结构完整性与生成 diff / 环境验证（非 Go 单测）。

**Organization**: 任务严格按 plan.md 的 Phase 0~4 顺序编排；同时标注所服务的用户故事（US1~US5）与需求追溯（FR/AC）。

## Format: `[ID] [P?] [Story?] Description`

- **[P]**：可与同阶段其它 [P] 任务并行（不同文件、无未完成依赖）
- **[Story]**：所服务的用户故事（US1~US5）；纯基础/静态任务不带 Story 标签
- 每条任务包含：目标文件路径、验证/验收方式、FR/AC 追溯

## Code scope（白名单，改动只允许触达以下路径）

- `cmd/api-server/service/repo.go`（新 handler + swag 注解 + 响应 DTO）
- `cmd/api-server/service/repo_test.go`（handler 单测，TDD）
- `cmd/api-server/service/routers.go`（`/content/download_url` 路由）
- `internal/dal/repository/bkrepo.go`（导出常量 `TempDownloadURLExpireSeconds`）
- `.claude/skills/bscp-file-config/SKILL.md`（文件型配置 skill，文档产物）
- `docs/swagger/**`（`make docs` 生成物，检查 diff）

---

## Phase 0: 常量导出（前置基础，阻塞 handler）

**Purpose**：将有效期常量导出，供 handler 引用，避免 `expire_seconds` 魔法数字漂移；这是 Phase 1 handler 引用的前置项。

- [ ] T001 将 `internal/dal/repository/bkrepo.go` 中未导出常量 `tempDownloadURLExpireSeconds`（值 3600，不变）导出为 `TempDownloadURLExpireSeconds`，并同步更新 `repository` 包内所有原引用点。验证：`go build ./internal/dal/repository/...` 通过 + `gofmt -l internal/dal/repository/bkrepo.go` 无输出。追溯：FR-008、data-model §常量。

**Checkpoint**：常量已导出且包内编译通过，handler 可引用 `repository.TempDownloadURLExpireSeconds`。

---

## Phase 1: 下载 URL handler（TDD 核心）— User Story 3（P1）

**Goal**：新增管理面「下载 URL」接口，输入内容 sign，只返回临时预签名下载 URL 与有效期，响应体不含文件字节。

**Independent Test**：对一个已发布文件配置项的内容 sign 调用新接口，响应体只含 `download_url` 与 `expire_seconds=3600`、不含文件字节；sign 未上传时报「内容未上传」且不产生指向空对象的 URL；未鉴权不返回 URL。

### Tests for User Story 3（先写，必须先失败）⚠️

- [ ] T002 [US3] 编写失败单测 `cmd/api-server/service/repo_test.go`：定义 stub `Provider`（`struct{ repository.Provider }` 嵌入接口后覆写 `Metadata`/`DownloadLink`）与 stub `auth.Authorizer`（`Authorize` 返回 nil），用 `httptest.NewRequest` + `kit.WithKit` 注入含 `BizID`/`AppID` 的 `kt`，构造 `repoService{authorizer, provider}` 调用新 handler。覆盖 6 类分支断言：(1) 正常：Metadata 命中 + DownloadLink 返回 `["https://x/y"]` → 响应 `{download_url:"https://x/y", expire_seconds:3600}` 且响应体不含文件字节；(2) sign 缺失/非法（不设或设非法 `X-Bkapi-File-Content-Id`）→ `GetFileSign` 报错 → 400；(3) 内容未上传：Metadata 返回 `errf.ErrFileContentNotFound` → 「内容未上传」错误且**不调用** DownloadLink；(4) DownloadLink 返回 error → 400；(5) 多副本：DownloadLink 返回 `["u1","u2"]` → `download_url="u1"`、`expire_seconds=3600`；(6) 空切片：DownloadLink 返回 `[]` → 报错（防越界）。验证：`go test ./cmd/api-server/service/...` 因 handler/DTO 未定义而**失败**。追溯：FR-007/FR-008、AC-P01/AC-T01/AC-T02/AC-005/AC-S01。

### Implementation for User Story 3

- [ ] T003 [US3] 在 `cmd/api-server/service/repo.go`（与 `MetadataResponse` 就近，保持一处一致）定义响应 DTO `DownloadURLResponse{ DownloadURL string \`json:"download_url"\`; ExpireSeconds int \`json:"expire_seconds"\` }`。验证：随 T006 单测编译通过。追溯：FR-007/FR-008、data-model §1。
- [ ] T004 [US3] 在 `cmd/api-server/service/repo.go` 实现 handler `DownloadFileURL`（与 `DownloadFile` 并列）：`kt := kit.MustGetKit(r.Context())` → IAM Authorize（Biz `FindBusinessResource` + App `View`，与 `DownloadFile` 相同 `res` 结构）→ `sign, err := repository.GetFileSign(r)`（err → `rest.BadRequest`）→ `Metadata(kt, sign)` 命中 `errf.ErrFileContentNotFound` → 返回明确「内容未上传」错误并**不继续**调用 DownloadLink → `links, err := s.provider.DownloadLink(kt, sign, 1)`（err → `rest.BadRequest`；取首个非空为 `download_url`；空切片 → 报错）→ `render.Render(w, r, rest.OKRender(&DownloadURLResponse{DownloadURL: links[0], ExpireSeconds: repository.TempDownloadURLExpireSeconds}))`。验证：随 T006 单测转绿。追溯：FR-007/FR-008/FR-009、AC-T01/AC-T02/AC-S01、data-model §4。
- [ ] T005 [US3] 为 `DownloadFileURL` 补 swag godoc 注解（对标 `DownloadFile`/`FileMetadata`）：`@Summary 获取文件内容下载URL`、`@Tags 文件相关`、四个 header/path 参数、`@Success ... data=DownloadURLResponse`、`@Router /api/v1/biz/{biz_id}/content/download_url [get]`、`@ID get_content_download_url`，写入 `cmd/api-server/service/repo.go`。验证：注解齐全，供 Phase 3 `make docs` 抓取。追溯：FR-011（网关注册前置）。
- [ ] T006 [US3] 运行单测转绿并静态校验：`go test ./cmd/api-server/service/...` 全绿（覆盖 T002 全部 6 类分支）+ `gofmt -l` 对改动文件无输出 + `golangci-lint run`（改动文件）无新增告警。验证：命令输出全绿/无告警。追溯：FR-007/FR-008/FR-009。

**Checkpoint**：下载 URL handler 全分支单测通过、静态检查干净，US3 服务端能力就绪。

---

## Phase 2: 路由注册 — User Story 3（P1）

**Goal**：将 `DownloadFileURL` 挂到 `/content` 下载组，复用与老下载接口一致的鉴权中间件链，且不改变老接口行为。

**Independent Test**：新路由 `/api/v1/biz/{biz_id}/content/download_url` 可达且经统一认证 + Biz + Content 校验；鉴权失败返回预期错误、不泄露 URL；老 `/download`、`/metadata` 行为不变。

- [ ] T007 [US3] 在 `cmd/api-server/service/routers.go` 的 `/api/v1/biz/{biz_id}/content` 下载组（`UnifiedAuthentication + BizVerified + ContentVerified` 那个 `r.Group`）内新增子路由：`r.Route("/download_url", func(r chi.Router){ r.Use(p.HttpServerHandledTotal("", "DownloadURL")); r.Get("/", p.repo.DownloadFileURL) })`；保持老 `/download`、`/metadata` 路由不变。验证：`go build ./cmd/api-server/...` 通过。追溯：FR-009/FR-010。
- [ ] T008 [US3] 集成验证（可跑范围内）：确认新路由挂载生效、中间件链（`UnifiedAuthentication/BizVerified/ContentVerified`）覆盖新路由，鉴权失败返回预期错误且响应体不含下载 URL。受外部鉴权依赖限制，以最小可跑范围为准。验证：可跑范围内的路由/鉴权用例通过。追溯：FR-009、AC-S01。

**Checkpoint**：新接口在管理面 REST 路由可达，鉴权链与老下载接口一致，US3 端到端服务端链路打通。

---

## Phase 3: 网关注册与 MCP — User Story 4（P1）

**Goal**：新接口注解经既有文档生成链路进入蓝鲸 API 网关文档，从而自动纳入 `bk-bscp-prod-server-mcp` 工具集。

**Independent Test**：`make docs` 生成的 swagger 中出现新路径 `/api/v1/biz/{biz_id}/content/download_url` 且带 `x-bk-apigateway-resource` 扩展。

- [ ] T009 [US4] 运行 `make docs`（或单独 `make markdown_docs`；**注意仓库无 `make sg` 目标**）重新生成 swagger：swag init 抓取 T005 注解 → mixin 合并 → `inject_bk_gateway.py` 注入网关扩展。检查 `docs/swagger/**` diff 中出现 `/api/v1/biz/{biz_id}/content/download_url` 且带 `x-bk-apigateway-resource`（`isPublic=true` + `authConfig{appVerifiedRequired,userVerifiedRequired,resourcePermissionRequired}=true`）。验证：生成 diff 含新路径与网关扩展。追溯：FR-011、AC-006（实际网关发布 + MCP 工具可见属环境/人工 E2E 验证）。

**Checkpoint**：新接口纳入网关文档生成物，为 MCP 工具集自动收录做好准备。

---

## Phase 4: 文件型配置 skill — User Story 1/2/4/5（文档产物，F-001~F-007）

**Goal**：提供对标 KV skill 的文件型配置 skill，覆盖 F-001~F-006 端到端编排（含用下载 URL 验证），使 AI 可对话式操作文件型配置。

**Independent Test**：AI 依据 skill 可完成「查询 → 元数据增删改 → 生成版本 → 全量/灰度发布 → 取下载 URL 验证」端到端编排，并正确区分文件型 vs KV 型、按报错→处置指引处理常见错误。

- [ ] T010 [US4] 新增 `.claude/skills/bscp-file-config/SKILL.md`，对标 `.claude/skills/bscp-kv-config/SKILL.md` 的结构与深度：**定位**；**前置条件**（依赖 `bk-bscp-prod-server-mcp`）；**交互引导**（先要 bizId → 服务名解析 appId → 操作前**校验 `config_type=file`**，对 KV 型报错，R-002）；**核心规则**（草稿态需生成版本+发布 R-001；仅 file 型 R-002；灰度字段在 Publish、groups 有值则 all=false R-003；引用 sign 须已上传 R-004）；**领域模型**（biz → app(file) → config_item(sign+元数据) → release，content 只读引用）；**端到端编排 F-001~F-006**（查询 → 增删改 → 生成版本 CreateRelease → 全量发布 Publish all=true / 灰度发布 all=false+灰度参数 → 用下载 URL 接口取 URL 验证）；**参数获取**；**报错→处置**（含「内容未上传」引导走 UI/SDK）；**场景化示例**；并明确**不含内容上传**编排与文件型 vs KV 型差异。验证：skill 结构完整覆盖上述章节；端到端编排闭环走得通（E2E/人工）。追溯：FR-001~FR-006/FR-012、AC-001~AC-004/AC-006、R-001~R-004。

**Checkpoint**：文件型 skill 就绪，配合网关注册后的 MCP 工具，US1/US2/US4/US5 的对话式操作能力可用。

---

## Phase 5: Polish & 跨领域收尾

- [ ] T011 全量静态收尾：对本次改动 Go 文件统一 `gofmt` + `golangci-lint run`（改动文件），确认无新增告警；确认老 `/content/download`、`/content/metadata` 行为未变（FR-010 回归）。验证：命令无输出/无告警；老接口回归通过。追溯：FR-010、CLAUDE.md Go 规范。

---

## Dependencies & Execution Order

### Phase 依赖

- **Phase 0（常量导出）**：无前置，最先执行；**阻塞** Phase 1 handler（handler 引用导出常量）。
- **Phase 1（handler TDD）**：依赖 Phase 0；内部严格 TDD——T002（失败测试）→ T003/T004/T005（实现）→ T006（转绿+静态）。
- **Phase 2（路由）**：依赖 Phase 1 handler 存在（T004）。
- **Phase 3（网关生成）**：依赖 Phase 1 注解（T005）与 Phase 2 路由（T007）就绪。
- **Phase 4（skill）**：功能上引用下载 URL 接口（Phase 1~2 提供），文档编写本身可与 Phase 3 并行；端到端验证依赖前序完成。
- **Phase 5（收尾）**：依赖所有代码改动完成。

### 用户故事映射

- **US3（P1，下载 URL 接口）**：Phase 1 + Phase 2（核心新增代码，MVP）。
- **US4（P1，skill + MCP 工具可见）**：Phase 3 + Phase 4。
- **US1/US2/US5（P1/P2，查询 / 变更发布 / 灰度）**：由 Phase 4 skill 编排既有 config-server 接口实现，无服务端代码改动。

### TDD 内部顺序（Phase 1）

- T002 测试必须先写且先失败 → T003 DTO → T004 handler → T005 注解 → T006 转绿+静态。

### 并行机会

- T001（Phase 0）与 T010（Phase 4 skill 文档撰写）互不触碰同一文件，可并行起草；但 T010 的端到端验证需等 Phase 1~3 完成。
- Phase 1 内 T003/T004/T005 均改 `repo.go` 同一文件，**不可并行**。

---

## Implementation Strategy

### MVP First（US3）

1. Phase 0：导出常量。
2. Phase 1：TDD 完成下载 URL handler（先失败测试 → 实现 → 转绿）。
3. Phase 2：注册路由。
4. **STOP & VALIDATE**：单测全绿 + 路由可达 + 鉴权链一致，US3 独立可测。

### Incremental Delivery

1. Phase 0~2 → 下载 URL 接口可用（MVP）。
2. Phase 3 → 网关注册，接口进入 MCP 工具集（US4 前半）。
3. Phase 4 → 文件型 skill，打通 F-001~F-006 对话式编排（US4 后半 + US1/US2/US5）。
4. Phase 5 → 静态收尾 + 老接口回归。

---

## 需求覆盖（FR/AC 追溯汇总）

| 需求 | 任务 |
|------|------|
| FR-001（查询） | T010 |
| FR-002（增删改） | T010 |
| FR-003（内容未上传引导） | T010（skill）+ T004（handler 服务端预检纵深防御） |
| FR-004（生成版本） | T010 |
| FR-005（全量发布） | T010 |
| FR-006（灰度发布） | T010 |
| FR-007（下载 URL 核心接口） | T002/T003/T004 |
| FR-008（返回体 + 多副本取首个 + 3600s） | T001/T003/T004 |
| FR-009（鉴权一致） | T004/T007/T008 |
| FR-010（兼容老接口） | T007/T011 |
| FR-011（网关注册） | T005/T009 |
| FR-012（文件型 skill） | T010 |
| AC-P01（响应体不含字节） | T002/T004 |
| AC-T01（内容未上传不产生空对象 URL） | T002/T004/T010 |
| AC-T02（多副本取首个 + 3600s） | T002/T004 |
| AC-S01（鉴权失败不泄露 URL） | T002/T004/T008 |
| AC-005（下载 URL 验证发布） | T002/T004/T010 |
| AC-006（MCP 工具可见 + E2E 编排） | T009/T010 |

---

## Notes

- [P] = 不同文件、无依赖，可并行。
- Phase 1 严格 TDD：先确认测试失败再实现，转绿后再做静态检查。
- skill（T010）为纯文档产物，不产生 Go 代码、不纳入 Go 单测；验收靠结构完整性与 E2E 编排走通。
- `make sg` 在仓库不存在，网关文档生成一律用 `make docs` / `make markdown_docs`。
- 代码改动只允许触达上文 Code scope 白名单路径。
