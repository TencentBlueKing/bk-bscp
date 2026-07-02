# bscp mcp 支持文件型

## 基本信息

|字段|值|
|:-:|:-:|
|需求 ID|1020451610135633598（短 ID：135633598）|
|需求名称|bscp mcp 支持文件型|
|工作空间|20451610|
|价值规模|24（RICE：Reach=20, Impact=8, Confidence=75%, Effort=5 人天）|
|预估工时|40 人时|
|父需求|无|
|原始需求文档|docs/reqs/文件型配置MCP.md|

## 需求背景

### 业务背景

bscp（蓝鲸基础配置平台）当前已经具备 **KV 型配置**的完整 AI 操作能力：
- MCP：`bk-bscp-prod-server-mcp`，由蓝鲸 API 网关注册的 bscp 接口**自动生成**，暴露 `Config_*` 系列工具；
- Skill：`bscp-kv-config`，为模型补充 MCP schema 表达不了的领域知识，支撑「改 KV → 生成版本 → 发布 → 验证」闭环。

但**文件型配置**（`config_type=file`）尚无对应的 MCP + skill 支撑，AI/用户无法像操作 KV 一样，通过对话完成文件型配置的查询、版本生成与发布闭环。

同时存在一个明确的性能痛点：文件型配置的内容可能较大，若操作链路中直接返回文件内容，会对 bscp 与蓝鲸 API 网关造成明显带宽负载。因此需要一个**只返回下载 URL、不返回文件内容**的接口，让调用方按需自行下载。

### 用户故事

- 作为 **配置管理者/SRE**，我想要通过对话（MCP + skill）查询某文件型服务的配置项与已发布内容，以便快速了解配置现状而无需登录 UI。
- 作为 **配置管理者/SRE**，我想要通过对话对文件型配置项做增删改（元数据）、生成版本、全量或灰度发布，以便完成一次文件型配置变更闭环。
- 作为 **配置管理者/SRE**，我想要在发布后拿到文件的**下载 URL**（而非文件内容本身）并据此验证发布结果，以便避免大文件内容穿透管理面造成带宽负载。
- 作为 **AI Agent**，我想要有一份文件型配置的 skill 指引，以便正确编排调用顺序、规避字段级业务约束和常见报错。

### 需求来源

- **需求渠道**：技术优化 / 平台能力补齐
- **关联需求**：无
- **参考资料**：
	- 现有 KV skill：`.claude/skills/bscp-kv-config/SKILL.md`
	- 下载 URL 参考实现：`bscp-go/internal/downloader/downloader.go`（及 `internal/upstream/api.go` 的 `GetDownloadURL`）

## 功能需求

### 核心功能点

|功能编号|功能描述|优先级|涉及角色|备注|
|:-:|:-:|:-:|:-:|:-:|
|F-001|文件型配置查询（配置项列表/详情、已发布配置项）|P0|配置管理者/AI|必须|
|F-002|文件型配置项元数据增删改（引用已上传内容 sign）|P0|配置管理者/AI|必须|
|F-003|生成版本（CreateRelease）|P0|配置管理者/AI|必须|
|F-004|全量发布（Publish all=true）|P0|配置管理者/AI|必须|
|F-005|灰度发布（按分组/标签）|P1|配置管理者/AI|应该有|
|F-006|新增「仅返回下载 URL」管理面接口|P0|平台|必须（本需求核心接口）|
|F-007|文件型配置 skill（领域模型、调用编排、字段约束、报错处置）|P0|AI Agent|必须|
|F-008|新接口在蓝鲸 API 网关注册，使其进入自动生成的 MCP|P0|平台|必须|

### 详细功能描述

#### [F-001] 文件型配置查询

- **输入**：bizId、服务名/appId（校验 `config_type=file`），可选配置项路径/名称、releaseId
- **处理逻辑**：
	1. 定位服务，读取 `config_type` 辨别 file 型（`ListAppsBySpaceRest` / `GetAppByName`）
	2. 查询草稿态配置项：`ListConfigItems` / `GetConfigItem`
	3. 查询已发布配置项：`ListReleasedConfigItems` / `GetReleasedConfigItem`
- **输出**：配置项列表/详情（含内容 sign、byte_size、path、name 等元数据）
- **边界条件**：对非 file 型服务操作应报错并中止
- **异常处理**：服务不存在 / 非 file 型 → 明确提示并停止

#### [F-002] 文件型配置项元数据增删改

- **输入**：bizId、appId、配置项元数据（path、name、权限、已上传内容的 sign 与 byte_size 等）
- **处理逻辑**：`CreateConfigItem` `UpdateConfigItem` `DeleteConfigItem` / `BatchUpsertConfigItems`（草稿态）
- **输出**：变更后的配置项（草稿态，客户端不可见）
- **边界条件**：**本期不含文件内容上传**；创建/更新配置项时引用的内容 sign 需为**已存在（此前经 UI/SDK 上传）** 的内容
- **异常处理**：引用的内容 sign 不存在 → 明确提示"内容未上传"（详见未解决问题 Q-002）

#### [F-003] 生成版本

- **输入**：bizId、appId，可选版本名
- **处理逻辑**：`CreateRelease` → 产出 `releaseId`
- **输出**：`releaseId`

#### [F-004] 全量发布

- **输入**：bizId、appId、releaseId
- **处理逻辑**：`Publish`，`all=true`
- **输出**：发布结果
- **异常处理**：已有版本上线中 / 版本已废弃 → 按报错对照处置

#### [F-005] 灰度发布

- **输入**：bizId、appId、releaseId、灰度模式（按分组 `publish_by_groups` 或按标签 `publish_by_labels`）、groups/labels
- **处理逻辑**：`CreateRelease` → `Publish`，`all=false` + 灰度参数
- **边界条件**：`groups` 有值时 `all` 必须为 false

#### [F-006] 新增「仅返回下载 URL」管理面接口

- **现状**：管理面（api-server，网关注册路径）`GET /api/v1/biz/{biz_id}/content/download` 目前**直接流式返回文件内容**（`io.Copy`），带宽重；无"只返回下载 URL"的管理面接口。
- **已有可复用能力**：
	- feed-server 有 gRPC `GetDownloadURL`（返回临时下载 URL，bscp-go 下载即用它，属客户端/sidecar 链路，非管理面 REST）；
	- 底层存储 provider（bkrepo / cos）已支持生成**预签名临时下载 URL**（当前默认有效期 3600 秒）。
- **输入**：bizId、appId（或模板空间 id）、内容 sign（`X-Bkapi-File-Content-Id`，SHA256）
- **处理逻辑**：复用存储层预签名 URL 能力，生成临时下载 URL 并返回，**不返回文件内容**
- **输出**：`{ download_url, expire_seconds }`（字段以实现为准）
- **非功能约束**：见「非功能需求」中带宽与有效期

#### [F-007] 文件型配置 skill

- 对标 `bscp-kv-config`，补充文件型的领域模型、端到端调用编排、参数获取、字段级业务约束、报错→处置、场景化示例。
- 明确文件型与 KV 型差异：文件型配置项以「内容 sign + 元数据」组织，草稿态增删改需「生成版本 + 发布」后才对客户端生效。
- 覆盖 F-001~F-006 的调用编排；**不含内容上传**编排。

#### [F-008] 新接口网关注册与 MCP 生成

- F-006 的新接口需在蓝鲸 API 网关注册，使其被自动纳入 `bk-bscp-prod-server-mcp` 生成的工具集合，供 skill 调用。

## 非功能需求

### 性能需求

- **带宽**：文件型下载验证链路**不得**在管理面/网关直接透传文件内容；改为返回下载 URL，由调用方直连存储下载。
- **下载 URL 有效期**：默认沿用现有 **3600 秒**（如需变更见未解决问题）。

### 安全需求

- **权限控制**：下载 URL 接口需与现有下载鉴权一致（业务级 + 服务级鉴权，参考现有 `DownloadFile` 的 IAM 用户级鉴权）。
- **数据保护**：返回的下载 URL 为**临时预签名 URL**，到期失效。

### 兼容性

- **接口兼容**：新增下载 URL 接口不改变现有 `/content/download`（返回内容）行为，避免影响存量客户端；老接口保留。

## 业务规则

- **规则 R-001**：文件型配置项增删改均为草稿态，客户端不可见；必须「生成版本 + 发布」后才生效。
- **规则 R-002**：文件型操作只适用于 `config_type=file` 的 app；对 KV 型操作文件接口应报错。
- **规则 R-003**：灰度发布字段（grayPublishMode/groups/labels）在 Publish 上；`groups` 有值时 `all` 必须为 false。
- **规则 R-004**：本期文件内容**只读引用**——创建/更新配置项引用的内容 sign 须为已上传内容；上传动作不在本期范围。

## 外部依赖与集成

|系统名称|交互方式|接口说明|认证方式|备注|
|:-:|:-:|:-:|:-:|:-:|
|蓝鲸 API 网关|HTTP REST|注册 bscp 管理面接口并自动生成 MCP 工具|网关应用凭证 / X-Bkapi-Authorization|F-008|
|bscp config-server / api-server|HTTP REST|配置项/版本/发布/内容下载 URL 接口|统一鉴权 + 业务/服务鉴权|F-001~F-006|
|对象存储（bkrepo / cos）|SDK|生成预签名临时下载 URL（默认 3600s）|存储侧凭证|F-006 复用能力|
|feed-server `GetDownloadURL`|gRPC|客户端侧下载 URL 参考实现|客户端 token|仅作实现参考，非本期交付接口|

### 数据模型（速览）

```
biz（业务） → app（服务，config_type=file） → config_item（草稿态配置项：元数据 + 内容 sign）→ release（不可变版本快照）
```

## 验收标准

### 功能验收

- [ ] **AC-001**：Given 一个 `config_type=file` 的服务 When 通过 MCP+skill 查询其配置项与已发布内容 Then 返回配置项元数据（含 sign/byte_size/path/name）与已发布列表，且对非 file 型服务操作时明确报错并中止
- [ ] **AC-002**：Given 已上传内容对应的 sign When 通过 MCP+skill 创建/更新/删除文件型配置项（元数据）Then 变更成功且处于草稿态（客户端不可见）
- [ ] **AC-003**：Given 存在草稿态改动 When 执行「生成版本 → 全量发布」Then 生成 releaseId 并全量发布成功，已发布内容可查询到
- [ ] **AC-004**：Given 一个已生成的 releaseId When 执行灰度发布（按分组/标签，all=false）Then 仅目标灰度范围生效
- [ ] **AC-005**：Given 一个已发布文件配置项的内容 sign When 调用新增的「下载 URL」接口 Then 返回临时预签名下载 URL 且**不返回文件内容**，用该 URL 可成功下载到正确文件
- [ ] **AC-006**：Given 新增的下载 URL 接口已在网关注册 When 加载 `bk-bscp-prod-server-mcp` Then 该接口对应工具出现在 MCP 工具集合中，可被 skill 调用

### 性能验收

- [ ] **AC-P01**：文件型下载验证链路中，管理面/网关**不透传文件内容**；下载 URL 接口响应体不含文件字节。

### 安全验收

- [ ] **AC-S01**：下载 URL 接口鉴权与现有内容下载一致（业务 + 服务/IAM 鉴权）；返回的 URL 为临时 URL，到期失效。

## 边界范围

### 本期包含

- 文件型配置查询（列表/详情、已发布）
- 文件型配置项元数据增删改（引用已上传内容）
- 生成版本、全量发布、灰度发布
- 新增「仅返回下载 URL」管理面接口 + 网关注册 + MCP 生成
- 文件型配置 skill

### 本期不包含

- 文件内容**上传**（仍走 UI / SDK；AI 场景不上传二进制大文件）
- 模板型（template）文件配置的操作
- KV 型能力的改动

## 约束条件

- **技术限制**：MCP 由蓝鲸 API 网关注册接口自动生成，新增接口须先在网关注册；下载 URL 复用存储层（bkrepo/cos）预签名能力，不新造下载通道。
- **兼容限制**：不改变现有 `/content/download` 返回内容的行为，老接口保留。

## 未解决问题

|问题ID|问题描述|状态|
|:-:|:-:|:-:|
|Q-001|需求优先级未在 TAPD 中标注，需补充（High/Middle/Low）|待确认|
|Q-002|本期不含上传，但创建/更新文件型配置项需引用内容 sign；当引用的内容尚未上传时，skill 的处置策略（明确报错并引导用户走 UI/SDK 上传，还是其它）需确认|待确认|
|Q-003|下载 URL 有效期是否沿用 3600s，或需可配置/更短|已解决（沿用 3600s，见技术澄清）|
|Q-004|下载 URL 接口的落点（config-server 还是 api-server）与返回体字段命名，待技术方案阶段确定|已解决（落 api-server，见技术澄清）|

## 技术澄清

> 澄清日期：2026-07-02
> 需求复杂度：中等（偏复杂）
> 澄清轮次：1

### 技术审查结论

- **技术可行性**：✅ 可行
- **技术风险等级**：低
- **审查说明**：F-001~F-005 复用现有 config-server 配置项/版本/发布接口（与 KV 型编排同构，仅 config_type 差异）；F-006 核心接口可在 api-server `repoService` 复用既有 `Provider.DownloadLink` 预签名能力 + 现有鉴权链落地，无新造下载通道；F-008 沿用现有网关注册脚本与 swagger 生成链路；F-007 skill 为文档产物。无新技术引入、无数据模型变更。

### 技术方案概述

- **实现方式**：
  - **F-006（本需求核心接口）**：在 api-server `cmd/api-server/service/repo.go` 的 `repoService` 上新增「仅返回下载 URL」handler，与现有 `DownloadFile` 并列。输入沿用 `GetFileSign(r)`（读 `X-Bkapi-File-Content-Id` 头，SHA256）+ `biz_id`（path）+ app/template_space（header）；处理复用 `s.provider.DownloadLink(kt, sign, fetchLimit)` 生成临时预签名 URL；**不返回文件内容**。返回体 `{ download_url, expire_seconds }`：`DownloadLink` 返回 `[]string`，取首个为 `download_url`，`expire_seconds=3600`，管理面单次验证场景 `fetchLimit=1`。
  - **F-001~F-005**：不新增管理面接口，由 F-007 skill 编排现有 config-server 接口（查询：`ListAppsBySpaceRest`/`GetAppByName`/`ListConfigItems`/`GetConfigItem`/`ListReleasedConfigItems`/`GetReleasedConfigItem`；增删改：`CreateConfigItem`/`UpdateConfigItem`/`DeleteConfigItem`/`BatchUpsertConfigItems`；版本与发布：`CreateRelease`/`Publish`）。
  - **F-008**：F-006 新接口在蓝鲸 API 网关注册（`scripts/bk_gateway/inject_bk_gateway.py` 注入 + `make sg` 生成 swagger/bkapigw 文档），自动纳入 `bk-bscp-prod-server-mcp` 工具集。
  - **F-007**：对标 `.claude/skills/bscp-kv-config/SKILL.md` 编写文件型 skill，覆盖 F-001~F-006 编排、字段级约束（R-001~R-004）、报错处置。
- **涉及模块**：`cmd/api-server/service/repo.go`（新 handler）、`cmd/api-server/service/routers.go`（新路由注册）、`scripts/bk_gateway/inject_bk_gateway.py`（网关注册）、新增文件型 skill 目录。
- **技术选型**：无新引入框架/库；复用 `internal/dal/repository` 的 `Provider.DownloadLink` 与 `GetFileSign`。

### 架构影响

- **新增组件**：无独立服务/组件；仅在 api-server `repoService` 新增一个 REST handler + 路由。
- **变更组件**：`repoService` 增加下载 URL handler；`routers()` 在 `/api/v1/biz/{biz_id}/content` 组内新增兄弟路由。
- **数据模型变更**：无（不新增表/字段，不涉及数据迁移）。
- **向后兼容性**：✅ 兼容。现有 `/content/download`（流式返回内容）行为不变、老接口保留（兼容限制）；新增接口为独立路由。

### 外部依赖

| 依赖项 | 类型 | 状态 | 接口文档 | 备注 |
|--------|------|------|---------|------|
| 蓝鲸 API 网关 | HTTP REST | ✅ 已确认 | scripts/bk_gateway/inject_bk_gateway.py；Makefile（make sg） | 新接口注册后自动生成 MCP 工具（F-008）|
| bscp config-server | HTTP REST | ✅ 已确认 | pkg/protocol/config-server/config_service.proto | F-001~F-005 配置项/版本/发布 |
| bscp api-server | HTTP REST | ✅ 已确认 | cmd/api-server/service/repo.go、routers.go | F-006 新接口落点 |
| 对象存储 bkrepo/cos | SDK | ✅ 已确认 | internal/dal/repository/{bkrepo,cos,ha}.go（DownloadLink） | 预签名 URL 3600s，F-006 复用 |
| feed-server GetDownloadURL | gRPC | ✅ 已确认（仅参考）| pkg/protocol/feed-server/feed_server.proto | 客户端链路，非本期交付接口 |

### 安全与合规

- **权限控制**：新下载 URL 接口挂现有鉴权链——`UnifiedAuthentication + BizVerified + ContentVerified`（与 `/content/download` 同组），业务级 + 服务/内容级鉴权一致，满足 AC-S01。参考现有 `DownloadFile` 的 IAM 用户级鉴权（Biz `FindBusinessResource` + App `View`），新接口按同一资源属性鉴权。
- **数据保护**：返回临时预签名 URL（bkrepo `type=DOWNLOAD` / cos `GetPresignedURL`），3600s 到期失效；响应体不含文件字节（满足 AC-P01）。
- **越权防护**：内容 sign 校验与 app/template_space 归属由 `ContentVerified` 中间件保证，避免跨业务/跨服务读取他人内容。

### 技术风险

| 风险 ID | 风险描述 | 影响 | 概率 | 应对措施 |
|---------|---------|------|------|---------|
| TR-001 | `Provider.DownloadLink` 在 ha 模式返回多条 URL（master/slave），需明确取值 | 低 | 中 | 取返回切片首个非空 URL 作为 `download_url`；plan 阶段确认多副本场景取值规则 |
| TR-002 | cos 实现忽略 `fetchLimit`（仅 bkrepo 生效 Permits） | 低 | 中 | 管理面验证场景传 `fetchLimit=1`，行为差异对下载验证无实质影响，文档标注 |
| TR-003 | 引用内容 sign 未上传导致预签名指向不存在对象 | 低 | 中 | 见 Q-002：skill 侧先经 metadata 接口预检存在性，缺失即报错引导上传 |

### 测试策略

- **单元测试**：新下载 URL handler——mock `repository.Provider`，覆盖：正常返回 `{download_url, expire_seconds}`、sign 缺失（`GetFileSign` 报错）、`DownloadLink` 返回空/错误、非 file 型服务（R-002）报错分支。
- **集成测试**：路由 + 鉴权链——校验新路由挂载于 `/api/v1/biz/{biz_id}/content` 组、`UnifiedAuthentication/BizVerified/ContentVerified` 生效、鉴权失败返回预期错误；响应体不含文件字节（AC-P01）。
- **端到端测试**：F-007 skill 编排闭环——查询（F-001）→ 元数据增删改（F-002，含 sign 未上传报错）→ 生成版本（F-003）→ 全量/灰度发布（F-004/F-005）→ 调用下载 URL 接口取 URL 并直连下载校验内容正确（AC-005）；网关注册后 MCP 工具可见（AC-006）。
- **测试数据**：准备一个 `config_type=file` 的 app 与一份已上传内容（含已知 sign/byte_size）；对照准备一个 KV 型 app 验证 R-002 报错。

### 补充的验收标准

- [ ] **AC-T01**：Given 传入的内容 sign 未上传 When 调用下载 URL 接口或经 skill 增删改配置项 Then 返回明确的"内容未上传"错误并引导走 UI/SDK 上传，不产生指向空对象的 URL（Q-002）
- [ ] **AC-T02**：Given ha 存储模式返回多条预签名 URL When 调用下载 URL 接口 Then 返回体 `download_url` 取首个有效 URL，且 `expire_seconds=3600`（TR-001/Q-003）

### 待解决问题

| 问题 ID | 问题描述 | 负责人 | 截止日期 | 状态 |
|---------|---------|--------|---------|------|
| —       | 技术澄清阶段无遗留阻塞问题（Q-001 dropped；Q-002/Q-003/Q-004 已由白名单文档自答，见 questions.md） | — | — | ✅ 已解决 |
