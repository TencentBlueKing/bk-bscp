---
name: bscp-kv-config
slug: bscp-kv-config
version: 1.1.0
description: |
  bscp（蓝鲸基础配置平台）KV 配置操作指引。为挂载了 bk-bscp-prod-server-mcp 的模型补充
  MCP 工具 schema 表达不了的领域知识：领域模型、项目/环境与两套工具的选择、端到端调用编排、
  参数获取、字段级业务约束、报错处置与场景化示例，帮助模型正确完成
  "定位项目环境 → 改 KV → 生成版本 → 发布 → 验证"闭环。
  Use this skill whenever the user asks to 操作 bscp 配置, 改 bscp KV, 新增/更新/删除 KV,
  发布 bscp 版本, 灰度发布, kv 配置查询/更新/发布, 看看某个服务的配置, 帮我发版,
  改某个项目/环境下的 KV,
  or invokes any bk-bscp-prod-server-mcp Config_* tool
  (CreateKv/UpdateKv/DeleteKv/BatchUpsertKvs/CreateRelease/Publish 及对应的 Config_Env_* 工具,
  ListProjects/ListEnvironments 等).
metadata:
  requires:
    mcps: ["bk-bscp-prod-server-mcp"]
---

# bscp KV 配置操作指引

## 定位

本 skill 只补充 MCP schema 表达不了的知识（跨工具编排、字段间业务约束、错误语义、领域模型），
**不重复** MCP 工具已有的单字段描述——填参时字段含义以 MCP 工具 schema 为准。文中的业务
约束来自 bscp 服务端的实际校验规则，会随 bscp 版本演进；若与调用返回的报错不一致，以服务端返回为准。

适用范围：`bk-bscp-prod-server-mcp` 的两套 KV 工具（不带项目环境的原有工具、带项目环境的
`Config_Env_*` 工具）及项目 / 环境查询工具，聚焦 `config_type=kv` 服务的 KV 配置闭环。
不覆盖模板、分组管理、客户端等 KV 闭环以外的接口。

## 前置条件

本 skill 依赖 `bk-bscp-prod-server-mcp`（蓝鲸 API 网关提供的 bscp MCP Server）暴露的 `Config_*` 工具。
**开始操作前必须先确认这些工具已可用**：

1. 检查当前会话是否已挂载 `bk-bscp-prod-server-mcp`，能看到 `Config_ListAppsBySpaceRest`/`Config_CreateKv` 等工具。
2. **若工具不存在**：不要臆造工具名或伪造调用结果，直接告知用户"未检测到 bscp MCP，需要先接入"，并给出下述接入指引，然后停下等待用户完成配置。
3. **若只有原有工具、没有 `Config_Env_*` 与项目 / 环境查询工具**：说明 MCP 尚未追加带项目环境的工具，
   只能操作默认项目的默认环境。用户要操作其他项目或环境时如实告知，不要拿原有工具硬调。
4. 工具可用后再按后续章节执行。

### 接入指引（当工具缺失时告知用户）

请在你的 AI IDE（Cursor / Claude 等）的 MCP 配置中添加 `bk-bscp-prod-server-mcp`，通常需要准备：

- **MCP Server 地址**：蓝鲸 API 网关上 bscp MCP Server 的接入地址；
- **鉴权信息**：调用网关所需的鉴权（如网关应用凭证 / `X-Bkapi-Authorization`）；
- **业务上下文**：目标业务的 `bizId`（来自 CMDB 业务 / 空间）。

具体的接入地址与凭证请向 bscp 平台或蓝鲸 API 网关管理员获取；配置完成后重新加载 MCP，确认 `Config_*` 工具出现即可。

## 项目与环境：两套工具怎么选

bscp 在业务（biz）和服务（app）之间新增了项目（project）和环境（env）两级，每个服务归属于某个项目下的某个环境。
每个业务都有一个默认项目（`spec.is_default=true`，名称通常为"默认项目"），默认项目下有默认环境（名称 `Default`）。
MCP 因此同时提供两套功能相同的 KV 工具：

| 工具集 | 示例 | 作用范围 |
|--------|------|---------|
| 原有工具（不带项目环境） | `Config_ListKvs`、`Config_Publish` | 只作用于默认项目的默认环境 |
| 带项目环境的工具 | `Config_Env_ListKvs`、`Config_Env_Publish` | 路径中必填 `projectId`、`envId`，可操作任意项目环境下的服务 |

两套工具一一对应：`Config_Xxx` 对应 `Config_Env_Xxx`，除 `projectId`/`envId` 外参数与业务约束完全相同。
共 13 对：ListAppsBySpaceRest、GetAppByName、ListKvs、CreateKv、UpdateKv、DeleteKv、BatchUpsertKvs、
BatchDeleteKv、ListReleases、CreateRelease、Publish、ListReleasedKvs、GetReleasedKv。
本文后续章节和示例以原有工具名书写，操作非默认环境时换成对应的 `Config_Env_*` 并补上 `projectId`/`envId`。

选择规则：

1. **用户指定了项目或环境**（名称、标识或 ID 均可）：先解析出 `projectId`、`envId`（见下），后续全程使用 `Config_Env_*`。
2. **用户没提项目环境**：先用原有工具在默认环境里定位服务；按服务名找不到时，**不要直接判定"服务不存在"**，
   而是问用户该服务在哪个项目、哪个环境（可用 `Config_ListProjects` 列出项目供选择），再改用 `Config_Env_*`。
3. **同一条闭环内不混用两套工具**：定位服务用了哪套，后续改 KV、生成版本、发布、验证都用同一套，并沿用同一组 `projectId`/`envId`。
4. **不要依赖原有工具上的 `projectId`/`envId` 参数**：服务内的接口按默认环境校验服务归属，
   非默认环境的服务会被拒绝（报 `app does not belong to the specified project or environment`）。

### 解析 projectId / envId

- **项目**：已知项目标识（key）用 `Config_GetProjectByKey {bizId, projectKey}`；只知道名称或需要用户挑选时，
  用 `Config_ListProjects {bizId, all:true}`，按 `spec.name` 匹配。取返回的 `id` 作为 `projectId`。
- **环境**：已知环境名用 `Config_GetEnvironmentByName {bizId, projectId, envName}`；需要用户挑选时，
  用 `Config_ListEnvironments {bizId, projectId, all:true}`，返回按环境类型分组为
  `prod_environments`/`staging_environments`/`test_environments`/`dev_environments`。取 `id` 作为 `envId`。
- 匹配到多个或一个都没有时，把候选列给用户确认，不要自行猜测。
- 部分 `Config_Env_*` 工具的 body 里也有 `projectId`/`envId` 字段：不填，或填与路径相同的值，不要填不一致的值。

## 交互引导（面向不熟悉闭环的用户）

用户往往只会抛一个模糊意图（如"kv 配置查询 / 更新 / 发布"）而不知道要给哪些参数、要走什么闭环。
**不要一次性罗列一堆参数把用户劝退，也不要臆造参数**；按下面的方式**分步反问，一次只问当前缺的一个关键信息**，
把用户缺的参数逐步补齐后再执行。

### 通用引导步骤（任何意图先做）

1. **确认业务 ID（bizId）**：用户没给就先问"请提供业务 ID（bizId）"。
2. **确认项目环境**：用户提到了项目或环境，就解析出 `projectId`/`envId` 并改用 `Config_Env_*`；
   没提就按默认环境走原有工具，不必主动追问（见「项目与环境」）。
3. **确认服务名 → 解析 appId**：拿到服务名后调 `Config_GetAppByName`（或先 `Config_ListAppsBySpaceRest` 让用户从列表里挑）
   得到 `appId`，并**校验 `config_type=kv`**；若不是 kv 型，直接告知"该服务不是 KV 型服务"并停止。
   默认环境里找不到该服务时，按「项目与环境」的规则 2 询问项目环境。
4. 参数齐了再进入对应意图的动作；能从上下文推断的（如上一步已拿到的 appId、projectId、envId）不要重复问。

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
确认信息里要写明**项目和环境名称**（默认环境也要写明），目标是生产类环境时特别提示，避免改到错误的环境。

## ⚠️ 核心规则

1. **KV 增删改都是草稿态**，客户端不可见；必须"生成版本 + 发布"后才生效。
2. **KV 只适用于 `config_type=kv` 的 app**；对 file 型 app 操作会报 `not a KV type service`。
3. **删除按 id/ids，不是按 key**：`DeleteKv`/`BatchDeleteKv` 需先 `ListKvs` 拿到 id。
4. **`Config_Publish` 只发布已存在的 releaseId**，不会从草稿自动生成版本。
5. **灰度字段在 `Config_Publish` 上**（`grayPublishMode`/`groups`/`labels`），需要先有 releaseId。
6. **列表接口必须显式给分页**：`Config_ListKvs`/`ListReleasedKvs`/`ListReleases`/`ListAppsBySpaceRest`（及对应 `Config_Env_*`）、`ListProjects`/`ListEnvironments` 要么传 `all:true` 全量取，要么显式传 `limit`（1–1000，配合 `start` 翻页）。**不传 `limit` 会被服务端当成 0**，报 `page.limit value should >= 1` 而查询失败——这是这些接口"要重试才成功"的常见原因。查询本意是"拿全部"时优先直接 `all:true`。
7. **原有工具只作用于默认项目的默认环境**；操作其他项目环境下的服务必须用 `Config_Env_*`，且同一闭环不混用两套工具。

## 领域模型速览（F-001）

```
biz（业务） → project（项目） → env（环境） → app（服务，带 config_type / data_type） → kv（草稿态配置项） → release（不可变版本快照）
```

- 每个业务有一个默认项目，默认项目下有默认环境 `Default`；原有工具只作用于这个环境。
- app 有 `config_type`（kv / file）和 `data_type`（app 级 KV 类型，可为 `any`）。
- KV 的每次增删改只改草稿区，不影响已发布内容；release 是一次生成后不可变的快照。

## 端到端调用编排（F-002）

标准闭环链路（工具名以原有工具书写，非默认环境换成对应的 `Config_Env_*`）：

0. **确定项目环境**（用户指定了项目或环境时）→ 拿 `projectId`/`envId`
   - `Config_GetProjectByKey` / `Config_ListProjects` → `projectId`
   - `Config_GetEnvironmentByName` / `Config_ListEnvironments` → `envId`
1. **定位服务** → 拿 `appId`
   - 已知业务：`Config_ListAppsBySpaceRest`（按 bizId 列 app）
   - 已知服务名：`Config_GetAppByName`
   - 从返回结果的 `config_type` 字段辨别 kv 型 app（MCP schema 里有 `configType` 字段时也可直接传 `kv` 过滤）
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
- `projectId`：`Config_GetProjectByKey`（已知项目标识）或 `Config_ListProjects`（按名称匹配 / 让用户挑）返回的 `id`。
- `envId`：`Config_GetEnvironmentByName`（已知环境名）或 `Config_ListEnvironments`（让用户挑）返回的 `id`。
- `appId`：通过 `Config_ListAppsBySpaceRest`（按 bizId 列 app）或 `Config_GetAppByName`（已知服务名）获取；
  非默认环境用 `Config_Env_ListAppsBySpaceRest` / `Config_Env_GetAppByName`。
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
| `app does not belong to the specified project or environment` | 服务不在所传项目环境下；或用原有工具操作了非默认环境的服务 | 向用户确认服务所在的项目环境，重新解析 `projectId`/`envId`，改用 `Config_Env_*` |

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

### 6) 在指定项目环境下更新 KV 并发布

用户说"把 game 项目 test 环境里 svc-a 的 timeout 改成 30"：

```
Config_GetProjectByKey {bizId, projectKey:"game"} → projectId     // 只知道名称时改用 Config_ListProjects {bizId, all:true} 按 spec.name 匹配
Config_GetEnvironmentByName {bizId, projectId, envName:"test"} → envId
Config_Env_GetAppByName {bizId, projectId, envId, appName:"svc-a"} → 校验 config_type=kv，取 appId
Config_Env_UpdateKv {bizId, projectId, envId, appId, key:"timeout"} body: {value:"30"}
Config_Env_ListReleases {bizId, projectId, envId, appId, all:true}   // 捕获历史命名规则，建议版本名并询问用户
Config_Env_CreateRelease {bizId, projectId, envId, appId} body: {name:"<用户确认的版本名>"} → releaseId
Config_Env_Publish {bizId, projectId, envId, appId, releaseId} body: {all:true}   // 确认信息写明"game 项目 / test 环境"
```

## 说明

本文的领域约束与操作规范可能随 bscp 版本演进而变化。实际以工具调用的返回结果和报错信息为准；
遇到与本文不一致的情况，按报错对照表处置或咨询 bscp 平台。
