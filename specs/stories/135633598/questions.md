# Clarification Questions — Story 135633598

## Q1 [dropped] — 来源：技术澄清
**问题**：需求优先级未在 TAPD 中标注，需补充（High/Middle/Low）。
**影响**：纯项目管理属性，与技术实现方案无关；非阻塞。
**建议候选**：无
**提出方**：主会话 / attempt=1 / round=1 / ts=2026-07-02T14:22:29+08:00
**放弃理由**：非技术问题，不影响技术方案与 DoR 判定；由 PO 在 TAPD 侧补充标注即可，不阻塞技术澄清。

## Q2 [resolved_by_doc] — 来源：技术澄清
**问题**：本期不含上传，但创建/更新文件型配置项需引用内容 sign；当引用的内容尚未上传时，skill 的处置策略如何确定？
**影响**：影响 req.md「技术澄清 → 安全与合规 / 测试策略」及 F-002/F-007 的报错处置编排；非阻塞。
**建议候选**：
- A. 明确报错并引导用户走 UI/SDK 上传（推荐：与 req.md 已述默认一致，且可复用现有 metadata 接口预检）
- B. skill 内静默跳过
**提出方**：主会话 / attempt=1 / round=1 / ts=2026-07-02T14:22:29+08:00
**答复**：采用 A。技术上可在增删改前复用现有 `GET /api/v1/biz/{biz_id}/content/metadata`（`repoService.FileMetadata` → `provider.Metadata`，返回 `{exists, metadata}`）对内容 sign 做存在性预检；不存在时由 skill 明确报错并引导用户走 UI/SDK 上传，不在本期实现二进制上传编排。
**答复方**：subagent(自答) / ts=2026-07-02T14:22:29+08:00
**文档来源**：docs/reqs/文件型配置MCP.md；cmd/api-server/service/repo.go（FileMetadata / MetadataResponse）

## Q3 [resolved_by_doc] — 来源：技术澄清
**问题**：下载 URL 有效期是否沿用 3600s，或需可配置/更短？
**影响**：影响 F-006 返回体 `expire_seconds` 取值与「非功能需求 → 下载 URL 有效期」；非阻塞。
**建议候选**：
- A. 沿用存储层现有 3600s（推荐：复用 `DownloadLink` 既有能力，零新增配置）
- B. 新增可配置项 / 更短有效期
**提出方**：主会话 / attempt=1 / round=1 / ts=2026-07-02T14:22:29+08:00
**答复**：采用 A。存储层预签名有效期已固定为 3600s——bkrepo 走 `GenerateTempDownloadURL` 的 `ExpireSeconds=tempDownloadURLExpireSeconds`，cos 走 `GetPresignedURL(..., time.Hour, ...)`。新接口直接复用 `Provider.DownloadLink`，`expire_seconds` 返回 3600，本期不新增有效期配置项。
**答复方**：subagent(自答) / ts=2026-07-02T14:22:29+08:00
**文档来源**：internal/dal/repository/bkrepo.go（DownloadLink）；internal/dal/repository/cos.go（DownloadLink）

## Q4 [resolved_by_doc] — 来源：技术澄清
**问题**：下载 URL 接口的落点（config-server 还是 api-server）与返回体字段命名如何确定？
**影响**：影响 F-006 的技术方案（服务/包/路由落点）、F-008 网关注册路径与 skill 调用编排；阻塞技术方案定稿。
**建议候选**：
- A. 落 api-server `repoService`，与现有 `DownloadFile` 并列注册于 `/api/v1/biz/{biz_id}/content` 路由组，复用 `GetFileSign(X-Bkapi-File-Content-Id)` + `Provider.DownloadLink` + 现有鉴权链（推荐）
- B. 落 config-server（需另建预签名调用链与鉴权，重复造轮子）
**提出方**：主会话 / attempt=1 / round=1 / ts=2026-07-02T14:22:29+08:00
**答复**：采用 A。依据白名单调研：`repoService` 已持有 `provider repository.Provider` 与 `authorizer`；`Provider.DownloadLink(kt, sign, fetchLimit)` 已由 bkrepo/cos/ha 实现，直接产出临时预签名 URL；`/api/v1/biz/{biz_id}/content` 组下 download/metadata 已挂 `UnifiedAuthentication + BizVerified + ContentVerified` 鉴权链，新增「下载 URL」handler 作为同组兄弟路由即可完整复用鉴权与服务/内容校验，无需在 config-server 另起链路。返回体命名为 `{ download_url, expire_seconds }`：`DownloadLink` 返回 `[]string`，取首个作为 `download_url`；`expire_seconds=3600`。管理面单次验证场景 `fetchLimit` 取 1。
**答复方**：subagent(自答) / ts=2026-07-02T14:22:29+08:00
**文档来源**：cmd/api-server/service/repo.go；cmd/api-server/service/routers.go；internal/dal/repository/repository.go（DownloadLink/GetFileSign）
