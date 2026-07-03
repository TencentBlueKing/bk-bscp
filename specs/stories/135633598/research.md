# Research — Story 135633598（bscp MCP 支持文件型配置）

> 依据白名单（context.md）调研得出的技术选型与关键决策。每条采用
> Decision / Rationale / Alternatives 结构。范围聚焦本需求核心接口 F-006
> 与配套的网关注册 F-008、skill F-007；F-001~F-005 复用既有 config-server 接口。

## R1. 下载 URL 接口的落点：api-server 而非新增 config-server RPC

- **Decision**：在 `cmd/api-server/service/repo.go` 的 `repoService` 上新增一个
  REST handler（与现有 `DownloadFile` 并列），复用 `internal/dal/repository`
  的 `Provider.DownloadLink` 生成预签名 URL；不在 config-server 新增 gRPC/RPC。
- **Rationale**：
  1. 内容对象存储访问能力（`Provider` = bkrepo/cos）已经注入到 api-server 的
     `repoService`（见 `newRepoService` → `repository.NewProvider`），且现有
     `/api/v1/biz/{biz_id}/content` 组已承载 upload/download/metadata 全套内容接口，
     新接口是同一资源域（content by sign）的自然扩展。
  2. config-server 面向配置项元数据/版本/发布，不直接持有对象存储 provider；
     若在 config-server 新增会引入到存储层的新依赖与新鉴权链路，违背「不引入不必要
     抽象/兼容层」（CLAUDE.md）。
  3. 网关注册链路（`make markdown_docs` 的 swag → mixin → inject）已覆盖 api-server
     的 REST 注解 handler（现有 upload/download/metadata 均由此进入网关），F-008 零新增机制。
- **Alternatives considered**：
  - 复用 feed-server 的 gRPC `GetDownloadURL`（`cmd/feed-server/service/rpc_sidecar.go`）：
    该接口属客户端/sidecar 链路，鉴权用 credential/token（`CanMatchCI`）而非管理面
    IAM 用户级鉴权，语义与调用方不同；且非网关注册的管理面 REST，无法自动进入 MCP。仅作实现参考。
  - 在 config-server 新增 RPC：见 Rationale 3，成本更高、跨层依赖更重，弃用。

## R2. DownloadLink 与 Download 的差异（为何不做内容透传）

- **Decision**：新 handler 只调用 `Provider.DownloadLink(kt, sign, fetchLimit)` 返回
  预签名 URL，**不调用** `Provider.Download`、不做 `io.Copy` 内容透传。
- **Rationale**：
  - `Download`（`bkrepo.go`/`cos.go`）返回 `io.ReadCloser`，现有 `DownloadFile`
    handler 用 `io.Copy(w, body)` 把文件字节透传给调用方，大文件会穿透管理面/网关造成带宽负载
    （正是本需求要解决的痛点，FR-007/AC-P01）。
  - `DownloadLink` 生成对象存储侧的临时预签名 URL（bkrepo `GenerateTempDownloadURL`
    `type=DOWNLOAD`；cos `GetPresignedURL`），响应体只含 URL 字符串，调用方直连存储下载，
    管理面不接触字节。
- **有效期**：两实现均固定 3600 秒（bkrepo 常量 `tempDownloadURLExpireSeconds=3600`；
  cos `time.Hour`）。返回体 `expire_seconds` 取 3600。
  - 为避免魔法数字漂移，建议将 `repository` 包内 `tempDownloadURLExpireSeconds` 导出为
    `TempDownloadURLExpireSeconds`，handler 直接引用；否则 handler 需硬编码 3600（与存储层重复）。
- **多副本取值（TR-001/AC-T02）**：`DownloadLink` 返回 `[]string`。bkrepo/cos 当前都返回单元素
  切片；ha 模式可能返回多条。handler 取**首个非空** URL 作为 `download_url`（与 feed-server
  `GetDownloadURL` 的 `downloadLink[0]` 取值一致）。切片为空则报错。
- **fetchLimit（TR-002）**：管理面单次验证场景传 `fetchLimit=1`。bkrepo 将其作为
  `Permits` 生效，cos 忽略该参数；对下载验证无实质影响，文档标注即可。

## R3. 内容不存在（sign 未上传）的处置（AC-T01 / FR-003 / TR-003）

- **Decision**：handler 在生成预签名 URL 前，先调用 `Provider.Metadata(kt, sign)` 预检内容存在性；
  命中 `errf.ErrFileContentNotFound` 时返回明确的「内容未上传」错误，不再调用 `DownloadLink`，
  从而不产生指向空对象的 URL。
- **Rationale**：
  - bkrepo `DownloadLink` 仅拼接预签名 URL（`GenerateTempDownloadURL`），**不校验对象是否存在**；
    cos `GetPresignedURL` 同理。若不预检，会对未上传的 sign 返回一个「能返回 200 但指向空对象」
    的 URL，违反 AC-T01「不产生指向空对象的 URL」。
  - 现有 `FileMetadata` handler 已用 `Provider.Metadata` + `errf.ErrFileContentNotFound`
    判定存在性，直接复用同一能力，语义一致、无新增抽象。
- **Alternatives considered**：仅在 skill 侧预检（Q-002 提到 skill 先经 metadata 预检）——
  作为纵深防御保留，但**服务端仍须强校验**（bk-security-redlines 红线 1：外部输入进入高危操作前
  服务端强约束校验），不能只靠 skill/前端，故 handler 内也做存在性校验。

## R4. 鉴权链路（FR-009 / AC-S01 / 安全红线 2）

- **Decision**：新路由挂在 `/api/v1/biz/{biz_id}/content` 的下载组，中间件链与 `DownloadFile`
  一致：`UnifiedAuthentication → BizVerified → ContentVerified`；handler 内再做 IAM 用户级
  Authorize（Biz `FindBusinessResource` + App `View`），与 `DownloadFile` 完全对齐。
- **Rationale**：
  - 下载 URL 一旦泄露即可直连存储取内容，属敏感能力，须身份认证 + 权限校验双要素（红线 2）。
  - 与老下载接口同链路可保证「鉴权与现有内容下载一致」（FR-009），且 `ContentVerified`
    负责 sign 与 app/template_space 归属校验，防跨业务/跨服务越权读取（横向越权）。
  - 鉴权失败在返回 URL 之前拦截，不泄露 URL（AC-S01）。
- **注意**：不复用上传组的 `UploadAppKeyAuthentication`（app 凭证直连）路径——下载沿用统一
  认证 + IAM 用户级鉴权（routers.go 现有注释已说明 download 不放开直连）。

## R5. 网关注册与 MCP 生成链路（F-008 / FR-011）

- **Decision**：为新 handler 增加 swag godoc 注解（`@Router /api/v1/biz/{biz_id}/content/download_url [get]`
  等，对标 `DownloadFile`/`FileMetadata`），通过既有文档生成链路纳入网关文档，从而自动进入 MCP。
- **链路事实**（Makefile）：
  - `make docs` = `api_docs` + `bkapigw_docs` + `markdown_docs`。
  - `markdown_docs`：`swag init -g ./cmd/api-server/api_server.go`（抓取 REST handler 注解）→
    `swagger mixin bkapigw.swagger.json apiserver/swagger.json`（合并 proto 与 REST 文档）→
    `python3 scripts/bk_gateway/inject_bk_gateway.py .../bkapigw/swagger.json`（注入
    `x-bk-apigateway-resource`）→ 生成 markdown。
  - 新路径非 `/inner/`，命中 `inject_bk_gateway.py` 的 `DEFAULT_EXTENSIONS`：
    `isPublic=true` + `authConfig{appVerifiedRequired, userVerifiedRequired, resourcePermissionRequired}=true`，
    与本接口「IAM 用户级 + 资源权限」鉴权语义一致。
- **命名更正**：spec/req 提到 `make sg`，但仓库 Makefile **无 `sg` 目标**；实际目标为
  `make docs`（或单独 `make markdown_docs`）。tasks/skill 阶段须使用真实目标名，避免命令不可执行。
- **Alternatives considered**：手工编辑生成的 swagger.json——不可维护、易与代码漂移，弃用；一律走生成命令。

## R6. F-001~F-005 复用既有 config-server 接口（不新增服务端逻辑）

- **Decision**：查询/增删改/版本/发布全部复用现有 config-server 接口，由 F-007 skill 负责编排；
  本需求不新增/修改这些接口的服务端实现。
- **依据**：`pkg/protocol/config-server/config_service.proto` 已提供
  `ListConfigItems`/`GetConfigItem`/`ListReleasedConfigItems`/`GetReleasedConfigItem`/
  `CreateConfigItem`/`UpdateConfigItem`/`DeleteConfigItem`/`BatchUpsertConfigItems`/
  `CreateRelease`/`Publish`，且这些接口已在网关注册、已作为 `Config_*` MCP 工具存在
  （现有 KV skill 即基于同一批工具，仅 config_type 维度不同）。
- **文件型 vs KV 型差异（供 skill 使用）**：文件型配置项以「内容 sign（SHA256）+ 元数据
  （path/name/byte_size/权限）」组织；KV 型以 key/kvType/value 组织。二者都遵循「草稿态增删改
  → 生成版本 → 发布」闭环（R-001）；文件型创建/更新须引用**已上传**内容 sign（R-004），本期不含上传。

## R7. 文件型 skill 覆盖范围与 KV skill 差异（F-007 / FR-012）

- **Decision**：新增 `.claude/skills/bscp-file-config/SKILL.md`，对标
  `.claude/skills/bscp-kv-config/SKILL.md` 的结构与深度（定位/前置条件/交互引导/核心规则/
  领域模型/端到端编排/参数获取/字段约束/报错处置/场景化示例）。
- **与 KV skill 的差异**：
  - 领域对象：config_item（sign + 元数据）替代 kv（key/value/kvType）。
  - 校验入口：操作前校验 app `config_type=file`（对 KV 型报错，R-002），与 KV skill 校验
    `config_type=kv` 对称。
  - 新增编排：发布后用**下载 URL 接口**（本需求新增，F-006）取 URL 验证，替代 KV 的
    ListReleasedKvs 直读值。
  - 明确边界：**不含内容上传**编排；引用 sign 未上传时报「内容未上传」并引导走 UI/SDK。
- **性质**：纯文档产物，不产生 Go 代码，不纳入 Go 单测；验收靠 skill 结构完整性与 E2E 编排走通。

## R8. 测试策略可行性（TDD）

- **单元测试**（`cmd/api-server/service` 新增 `repo_test.go`）：
  - 通过 stub 实现 `repository.Provider`（可用 `struct{ repository.Provider }` 嵌入接口后
    覆写 `Metadata`/`DownloadLink`，避免实现全部方法）与 stub `auth.Authorizer`（Authorize 返回 nil）
    构造 `repoService`，用 `httptest` + 注入 `kit`（`kit.WithKit`）驱动 handler。
  - 覆盖分支：正常返回 `{download_url, expire_seconds=3600}`；`GetFileSign` 失败（缺/非法 sign）→ 400；
    `Metadata` 返回 `ErrFileContentNotFound` → 「内容未上传」错误；`DownloadLink` 返回 error → 400；
    `DownloadLink` 返回空切片 → 错误；断言响应体**不含文件字节**（AC-P01）。
- **集成测试**：路由挂载与中间件链（`UnifiedAuthentication/BizVerified/ContentVerified`）生效、
  鉴权失败返回预期错误——受依赖注入与外部鉴权服务限制，以最小可跑范围为准，不足处在 tasks 阶段细化。
- **端到端**：F-007 skill 编排闭环（查询 → 增删改 → 生成版本 → 发布 → 取下载 URL 验证）+
  网关注册后 MCP 工具可见——属人工/环境验证，非 Go 单测。
- **R-002 归属澄清**：下载 URL handler 是**内容级**接口（按 sign 操作），本身不感知 app 的
  `config_type`；R-002（文件型 only）由 **skill 编排层**在操作前校验 `config_type=file` 落地。
  spec 测试策略中「handler 覆盖非 file 型报错分支」应理解为 skill/E2E 层职责，handler 单测不含该分支。

## 规范符合性基线

- **项目宪章**：`.specify/memory/constitution.md` 为**未填充模板**（占位符未替换）。按 pipeline 约定，
  以 `CLAUDE.md` 的 Go 规范与安全约束为准：中文文档/注释、`.golangci.yml`、改 Go 文件后 `gofmt`、
  优先补单包测试、不引入不必要抽象。
- **安全**：满足 bk-security-redlines 三大红线——输入校验（sign 长度/格式经 `GetFileSign` 强校验、
  内容存在性服务端预检）、鉴权（统一认证 + IAM + ContentVerified 防越权）、敏感数据（返回临时
  预签名 URL 3600s 到期失效，不落日志明文 URL、不在 URL 携带长期凭证）。
