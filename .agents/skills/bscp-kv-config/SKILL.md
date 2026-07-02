---
name: bscp-kv-config
slug: bscp-kv-config
version: 1.0.0
description: |
  bscp（蓝鲸基础配置平台）KV 配置操作指引。为挂载了 bk-bscp-prod-server-mcp 的模型补充
  MCP 工具 schema 表达不了的领域知识：领域模型、端到端调用编排、参数获取、字段级业务
  约束、报错处置与场景化示例，帮助模型正确完成"改 KV → 生成版本 → 发布 → 验证"闭环。
  Use this skill whenever the user asks to 操作 bscp 配置, 改 bscp KV, 新增/更新/删除 KV,
  发布 bscp 版本, 灰度发布, kv 配置查询/更新/发布, 看看某个服务的配置, 帮我发版,
  or invokes any bk-bscp-prod-server-mcp Config_* tool
  (CreateKv/UpdateKv/DeleteKv/BatchUpsertKvs/CreateRelease/Publish 等).
metadata:
  requires:
    mcps: ["bk-bscp-prod-server-mcp"]
---

# bscp KV 配置操作指引

## 定位

本 skill 只补充 MCP schema 表达不了的知识（跨工具编排、字段间业务约束、错误语义、领域模型），
**不重复** MCP 工具已有的单字段描述——填参时字段含义以 MCP 工具 schema 为准。文中的业务
约束来自 bscp 服务端的实际校验规则，会随 bscp 版本演进；若与调用返回的报错不一致，以服务端返回为准。

适用范围：`bk-bscp-prod-server-mcp` 当前的 13 个工具，聚焦 `config_type=kv` 服务的 KV 配置闭环。
不覆盖模板、分组管理、客户端等 KV 闭环以外的接口。

## 前置条件

本 skill 依赖 `bk-bscp-prod-server-mcp`（蓝鲸 API 网关提供的 bscp MCP Server）暴露的 `Config_*` 工具。
**开始操作前必须先确认这些工具已可用**：

1. 检查当前会话是否已挂载 `bk-bscp-prod-server-mcp`，能看到 `Config_ListAppsBySpaceRest`/`Config_CreateKv` 等工具。
2. **若工具不存在**：不要臆造工具名或伪造调用结果，直接告知用户"未检测到 bscp MCP，需要先接入"，并给出下述接入指引，然后停下等待用户完成配置。
3. 工具可用后再按后续章节执行。

### 接入指引（当工具缺失时告知用户）

请在你的 AI IDE（Cursor / Claude 等）的 MCP 配置中添加 `bk-bscp-prod-server-mcp`，通常需要准备：

- **MCP Server 地址**：蓝鲸 API 网关上 bscp MCP Server 的接入地址；
- **鉴权信息**：调用网关所需的鉴权（如网关应用凭证 / `X-Bkapi-Authorization`）；
- **业务上下文**：目标业务的 `bizId`（来自 CMDB 业务 / 空间）。

具体的接入地址与凭证请向 bscp 平台或蓝鲸 API 网关管理员获取；配置完成后重新加载 MCP，确认 `Config_*` 工具出现即可。

## 交互引导（面向不熟悉闭环的用户）

用户往往只会抛一个模糊意图（如"kv 配置查询 / 更新 / 发布"）而不知道要给哪些参数、要走什么闭环。
**不要一次性罗列一堆参数把用户劝退，也不要臆造参数**；按下面的方式**分步反问，一次只问当前缺的一个关键信息**，
把用户缺的参数逐步补齐后再执行。

### 通用引导步骤（任何意图先做）

1. **确认业务 ID（bizId）**：用户没给就先问"请提供业务 ID（bizId）"。
2. **确认服务名 → 解析 appId**：拿到服务名后调 `Config_GetAppByName`（或先 `Config_ListAppsBySpaceRest` 让用户从列表里挑）
   得到 `appId`，并**校验 `config_type=kv`**；若不是 kv 型，直接告知"该服务不是 KV 型服务"并停止。
3. 参数齐了再进入对应意图的动作；能从上下文推断的（如上一步已拿到的 appId）不要重复问。

### 三类意图的最小引导

| 用户说 | 最少还需要问 | 拿齐后动作 |
|--------|-------------|-----------|
| kv 配置查询 / 看某服务配置 | bizId、服务名（可选 key） | `Config_ListKvs`（草稿）或 `Config_ListReleasedKvs`（已发布），展示结果 |
| kv 配置更新 / 新增 / 删除 | bizId、服务名、key（更新/新增再要 value，必要时 kvType） | `Config_CreateKv`/`UpdateKv`/`DeleteKv`（草稿态）；完成后**主动追问"是否现在生成版本并发布？"**，用户同意再走发布流程 |
| kv 配置发布 / 发版 | bizId、服务名 | 有草稿改动 → 生成版本前先捕获历史命名规则给出建议版本名并询问用户（见「版本命名」），确认后 `Config_CreateRelease` 拿 releaseId；或让用户从 `Config_ListReleases` 选已有版本，再 `Config_Publish` |

### 写操作执行前必须二次确认

对 **发布（`Config_Publish`）**、**覆盖式批量导入（`Config_BatchUpsertKvs` 且 `replaceAll=true`）**、
**删除（`Config_DeleteKv`/`Config_BatchDeleteKv`）** 这类会影响客户端生效内容或清空数据的操作，
执行前先**展示将影响的内容**（如将发布的版本、将被覆盖/删除的 key 列表）并等用户确认后再调用；用户未确认不执行。

## ⚠️ 核心规则

1. **KV 增删改都是草稿态**，客户端不可见；必须"生成版本 + 发布"后才生效。
2. **KV 只适用于 `config_type=kv` 的 app**；对 file 型 app 操作会报 `not a KV type service`。
3. **删除按 id/ids，不是按 key**：`DeleteKv`/`BatchDeleteKv` 需先 `ListKvs` 拿到 id。
4. **`Config_Publish` 只发布已存在的 releaseId**，不会从草稿自动生成版本。
5. **灰度字段在 `Config_Publish` 上**（`grayPublishMode`/`groups`/`labels`），需要先有 releaseId。
6. **列表接口必须显式给分页**：`Config_ListKvs`/`ListReleasedKvs`/`ListReleases`/`ListAppsBySpaceRest` 要么传 `all:true` 全量取，要么显式传 `limit`（1–1000，配合 `start` 翻页）。**不传 `limit` 会被服务端当成 0**，报 `page.limit value should >= 1` 而查询失败——这是这些接口"要重试才成功"的常见原因。查询本意是"拿全部"时优先直接 `all:true`。

## 领域模型速览（F-001）

```
biz（业务） → app（服务，带 config_type / data_type） → kv（草稿态配置项） → release（不可变版本快照）
```

- app 有 `config_type`（kv / file）和 `data_type`（app 级 KV 类型，可为 `any`）。
- KV 的每次增删改只改草稿区，不影响已发布内容；release 是一次生成后不可变的快照。

## 端到端调用编排（F-002）

标准闭环链路：

1. **定位服务** → 拿 `appId`
   - 已知业务：`Config_ListAppsBySpaceRest`（按 bizId 列 app）
   - 已知服务名：`Config_GetAppByName`
   - 从返回结果的 `config_type` 字段辨别 kv 型 app（该工具当前不支持 config_type 过滤）
2. **改配置（草稿态）**
   - 单条：`Config_CreateKv` / `Config_UpdateKv` / `Config_DeleteKv`
   - 批量：`Config_BatchUpsertKvs`（`replaceAll=true` 会先清空草稿区再写入）/ `Config_BatchDeleteKv`（按 ids）
   - 删除前先 `Config_ListKvs` 拿 id
3. **生成版本**
   - 用户未指定版本名时，先**捕获历史命名规则并询问用户**（见下节「版本命名」），拿到确认后的名字再调用
   - `Config_CreateRelease` → 产出 `releaseId`
4. **发布**
   - `Config_Publish`：发布已有 `releaseId`；`all=true` 全量，或灰度（见下）
5. **验证**
   - `Config_ListReleases` / `Config_GetReleasedKv` / `Config_ListReleasedKvs`

### 全量 vs 灰度发布

| 场景 | 推荐工具 | 关键参数 |
|------|---------|---------|
| 生成版本并全量发布 | 先 `Config_CreateRelease` 拿 `releaseId`，再 `Config_Publish` | `all=true` |
| 发布某个已有版本（全量） | `Config_Publish` | `releaseId` + `all=true` |
| 灰度发布 | 先 `Config_CreateRelease` 拿 `releaseId`，再 `Config_Publish` | `all=false` + `grayPublishMode`（`publish_by_groups`/`publish_by_labels`）+ `groups` 或 `labels` |

> 注意：灰度相关字段（`grayPublishMode`/`groups`/`labels`/`groupName`）在 `Config_Publish` 的 body 上，
> 需要发布时统一走 `CreateRelease` 拿 `releaseId` 再 `Publish` 两步。
> `groups`（分组 ID 列表）有值时 `all` 必须为 `false`。

### 版本命名：捕获历史规则后询问用户

`Config_CreateRelease` 的 `name`（版本名）**必填**，且**同一 app 内唯一**。用户往往不想每次都自己想名字，
但又希望沿用团队既有的命名习惯。因此**用户未显式给版本名时**，不要直接臆造，也不要直接用一个固定默认值，
按下面的流程做：

1. **拉历史版本名**：`Config_ListReleases {bizId, appId, all:true}`，取各 release 的 `name`（列表接口记得带 `all:true`，见核心规则 6）。
2. **识别命名规律**：从历史 name 里归纳规则，常见几类——
   - 递增序号：`v1`/`v2`/`v3`、`release-1`/`release-2`
   - 语义前缀 + 序号：`v-import-1`、`gray-2`
   - 日期类：`20260702`、`2026-07-02`、`2026-07-02-1`（同日多次带尾号）
3. **生成建议名**：按识别到的规律推下一个值（递增类要在最大序号上 +1，日期类用当天日期；**都要避免与已有 name 重名**）。
4. **询问用户是否采用**：把建议名给用户，例如「按历史命名规律，建议版本名 `v4`，是否使用？也可以自定义」，
   用户确认或给出自定义名后再 `Config_CreateRelease`。
5. **无历史或无明显规律**：给一个合理默认（如 `v1` 或当天日期），同样先问用户，不要静默替用户决定。

命名约束（服务端 `ValidateReleaseName` 强校验，与 key 类似）：长度 1–128；仅中文 / 英文 / 数字 / `_` / `-` / `.`；
首尾必须是中文、英文或数字；同 app 内**不可重名**（重名报 `release name ... already exists`）。

## 参数获取（F-003）

- `bizId`：来自蓝鲸平台上下文（CMDB 业务 / 空间）或请求头 `X-Bkapi-Biz-Id`；MCP **不提供列 biz 的工具**，需由用户/上下文给出。
- `appId`：通过 `Config_ListAppsBySpaceRest`（按 bizId 列 app）或 `Config_GetAppByName`（已知服务名）获取。
- **辨别 kv 型 app**：从上述工具返回结果里读 `config_type` 字段，取 `config_type=kv` 的 app。
- `id`（KV 主键）：删除 KV 前用 `Config_ListKvs` 查询获取。
- `releaseId`：由 `Config_CreateRelease` 返回，或从 `Config_ListReleases` 中选取未废弃的版本。

## 字段级业务约束（F-004）

以下为 MCP schema 无法表达的关联约束（schema 只描述单字段），由 bscp 服务端强校验：

- **key**
  - 长度 1–128
  - 仅中文 / 英文 / 数字 / `_` / `-`；**首尾必须是中文、英文或数字**
  - **禁止 `_bk` 前缀**（不区分大小写）
  - **不含 `.` 和 `/`**（保留字符）
- **value**
  - 非空；**上限 1MB**
  - 按 kvType 做格式校验：`json` 须合法 JSON；`yaml` 须合法 YAML；`xml` 须合法 XML；`number` 须为数字；`string` 不含换行符；`text`/`secret` 不做内容格式校验
- **kvType**
  - 单条 KV 的 kvType **不可填 `any`**（`any` 只用于 app 的 data_type）
  - 单条 KV 的 kvType 必须与 app 的 `data_type` 一致；**例外**：app 的 data_type 为 `any` 时允许任意 kvType
  - **`UpdateKv` 不能改 kvType**：更新时沿用已存储的类型，请求里的 kvType 不生效；要改类型需删掉重建
- **secret**
  - kvType=`secret` 时 `secretType` 必填，枚举：`password` / `certificate` / `secret_key` / `token` / `custom`
  - `certificate` 类型的 value 应为 X.509 PEM，可配合 `certificateExpirationDate`
  - `secretHidden` 控制明文是否隐藏
- **数量上限**
  - 单 app 未删除的配置项默认上限 **2000**（含模板+非模板），部分业务可能有不同上限

## 报错 → 原因 → 处置（F-005）

| 报错关键字 | 原因 | 处置 |
|-----------|------|------|
| `already exists ... cannot be created again` | key 已存在 | 改用 `Config_UpdateKv`，或换一个 key |
| `kv type does not match the data type defined in the application` | kvType 与 app 的 data_type 不一致 | 按 app.data_type 修正 kvType（app 为 any 时不受此限） |
| `the type of config item ... is incorrect` | 批量导入时同 key 类型与已有不一致 | 保持已有类型 |
| `not a KV type service` | 对非 kv 型 app 操作 KV | 确认目标 app 的 `config_type=kv` |
| `there are duplicate keys ...` | 同一批量请求内 key 重复 | 去重后重试 |
| `there is a release in publishing currently` | 已有版本正在上线 | 等待当前上线完成后再发布 |
| `release ... is deprecated` | 目标版本已废弃 | 换未废弃版本，或重新 `CreateRelease` |
| `exceeded the limit` | 配置项数超过上限（默认 2000） | 清理无用 KV 后重试 |
| `page.limit value should >= 1` | 列表接口未传 `limit` 且 `all≠true`（limit 默认为 0） | 传 `all:true` 全量，或显式 `limit`(1–1000) |
| `invalid page.limit max value: 1000` | `limit` 超过单页上限 | `limit` ≤ 1000，需要更多时用 `start` 翻页或改 `all:true` |
| `release ... already exists` | 版本名与同 app 已有版本重名 | 换一个不重名的 name（递增类在最大序号上 +1） |

## 场景化示例（F-006）

以下为调用序列示意（`bizId`/`appId` 用占位符，实际以获取到的值为准）。

### 1) 新增一个 json 配置并全量发布

```
Config_ListAppsBySpaceRest {bizId, all:true} → 从结果中找 config_type=kv 的目标 app，取其 appId  // 列表接口带 all:true
Config_CreateKv {bizId, appId} body: {key:"feature_flags", kvType:"json", value:"{\"beta\":true}"}
Config_ListReleases {bizId, appId, all:true}             // 捕获历史命名规则，据此建议版本名并询问用户
Config_CreateRelease {bizId, appId} body: {name:"<用户确认的版本名>"} → releaseId  // 生成版本
Config_Publish {bizId, appId, releaseId} body: {all:true} // 全量发布
Config_ListReleasedKvs {bizId, appId, all:true}          // 验证已发布内容（列表接口带 all:true）
```

### 2) 批量导入 KV（覆盖式，replaceAll）

```
Config_BatchUpsertKvs {bizId, appId} body: {replaceAll:true, kvs:[{key,kvType,value}, ...]}
Config_CreateRelease {bizId, appId} body: {name:"v-import-1"}   // 拿 releaseId
Config_Publish {bizId, appId, releaseId} body: {all:true}
```

### 3) 删除一个只知道 key 的 KV

```
Config_ListKvs {bizId, appId, key:["obsolete_key"], all:true}   // key 为数组；列表接口带 all:true；拿到该 KV 的 id
Config_DeleteKv {bizId, appId, id}                  // 按 id 删（草稿态）
Config_ListReleases {bizId, appId, all:true}        // 捕获历史命名规则，建议版本名并询问用户
Config_CreateRelease {bizId, appId} body: {name:"<用户确认的版本名>"} → releaseId  // 生成版本
Config_Publish {bizId, appId, releaseId} body: {all:true}  // 发布后才对客户端生效
```

### 4) 更新一个 secret 配置

```
Config_UpdateKv {bizId, appId} body: {key:"db_password", value:"<new>", secretType:"password", secretHidden:true}
// 注意：UpdateKv 不能改 kvType；secret 必填 secretType
Config_CreateRelease {bizId, appId} → releaseId
Config_Publish {bizId, appId, releaseId} body: {all:true}
```

### 5) 灰度发布到指定分组

```
Config_CreateRelease {bizId, appId} body: {name:"v-gray-1"} → releaseId
Config_Publish {bizId, appId, releaseId} body: {all:false, grayPublishMode:"publish_by_groups", groups:[<groupId>, ...]}
// 或按 labels：grayPublishMode:"publish_by_labels", labels:[{...}], 可选 groupName
```

## 说明

本文的领域约束与操作规范可能随 bscp 版本演进而变化。实际以工具调用的返回结果和报错信息为准；
遇到与本文不一致的情况，按报错对照表处置或咨询 bscp 平台。
