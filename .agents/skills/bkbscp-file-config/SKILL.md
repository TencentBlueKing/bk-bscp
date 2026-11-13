---
name: bkbscp-file-config
slug: bkbscp-file-config
version: 2.1.1
description: |
  bscp（蓝鲸基础配置平台）文件型配置只读查看指引。为挂载了 bk-bscp-prod-file-config 的模型补充
  MCP 工具 schema 表达不了的领域知识：文件型（config_type=file）配置的领域模型、项目/环境与两套工具的选择、
  只读查看编排、参数获取、字段级业务约束与报错处置，帮助模型正确完成
  "定位项目环境 → 定位服务 → 查询配置项元数据 → 取下载 URL 查看文件内容"闭环。
  Use this skill whenever the user asks to 查看 bscp 文件配置, 看某文件型服务的配置,
  查询文件配置项, 看文件配置内容, 取文件下载 URL 查看文件内容, 看某个项目/环境下的文件配置,
  or invokes bk-bscp-prod-file-config 的工具（Config_ListConfigItems / Config_ListReleasedConfigItems /
  对应的 Config_Env_* 工具 / get_content_download_url / ListProjects / ListEnvironments 等）。
metadata:
  requires:
    mcps: ["bk-bscp-prod-file-config"]
---

# bscp 文件型配置只读查看指引

## 定位

本 skill 只补充 MCP schema 表达不了的知识（跨工具编排、字段间业务约束、错误语义、领域模型），
**不重复** MCP 工具已有的单字段描述——填参时字段含义以 MCP 工具 schema 为准。文中的业务
约束来自 bscp 服务端的实际校验规则，会随 bscp 版本演进；若与调用返回的报错不一致，以服务端返回为准。

适用范围：**只读查看** `config_type=file` 服务的文件型配置——**定位服务 → 查询配置项元数据 →
用下载 URL 查看文件内容**。**本 skill 只支持查看，不支持任何写操作**：不做配置项增删改、
不做批量导入、不生成版本、不发布/灰度发布。文件内容上传与配置项变更、发版仍走 UI / SDK。

## 前置条件

本 skill 依赖 `bk-bscp-prod-file-config`（蓝鲸 API 网关提供的 bscp **文件型专用** MCP Server）暴露的工具，
与 KV 型的 `bk-bscp-prod-kv-config` 相互独立。过渡期内旧 MCP `bk-bscp-prod-file-manage` 提供同名工具，同样可用。
**开始操作前必须先确认这些工具已可用**：

1. 检查当前会话是否已挂载 `bk-bscp-prod-file-config` 或 `bk-bscp-prod-file-manage`，能看到 `Config_GetAppByName`、
   `Config_ListConfigItems`、`get_content_download_url` 等工具；任一 MCP 提供所需工具即可继续。
2. **若工具不存在**：不要臆造工具名或伪造调用结果，直接告知用户"未检测到 `bk-bscp-prod-file-config`
   或对应工具，可能尚未挂载 / 在网关注册"，说明缺失的能力，然后停下等待用户确认接入，或退化为对可用工具的编排。
3. **若只有原有工具、没有 `Config_Env_*` 与项目 / 环境查询工具**：说明 MCP 尚未追加带项目环境的工具，
   只能查看默认项目的默认环境。用户要查看其他项目或环境时如实告知，不要拿原有工具硬调。
4. 工具可用后再按后续章节执行。

## 工具速查

| 用途 | 原有工具（默认环境） | 带项目环境的工具 |
|------|---------------------|------------------|
| 列服务 / 按名称取服务 | `Config_ListAppsBySpaceRest` / `Config_GetAppByName` | `Config_Env_ListAppsBySpaceRest` / `Config_Env_GetAppByName` |
| 草稿态配置项列表 / 详情 | `Config_ListConfigItems` / `Config_GetConfigItem` | `Config_Env_ListConfigItems` / `Config_Env_GetConfigItem` |
| 草稿态绑定的模板文件 | `Config_ListAppBoundTmplRevisions` | `Config_Env_ListAppBoundTmplRevisions` |
| 版本列表 / 按名称取版本 | `Config_ListReleases` / `Config_GetReleaseByName` | `Config_Env_ListReleases` / `Config_Env_GetReleaseByName` |
| 已发布配置项列表 / 详情 | `Config_ListReleasedConfigItems` / `Config_GetReleasedConfigItem` | `Config_Env_ListReleasedConfigItems` / `Config_Env_GetReleasedConfigItem` |
| 已发布版本绑定的模板文件 | `Config_ListReleasedAppBoundTmplRevisions` | `Config_Env_ListReleasedAppBoundTmplRevs` |
| 项目 / 环境查询 | — | `Config_ListProjects`、`Config_GetProjectByKey`、`Config_ListEnvironments`、`Config_GetEnvironmentByName` |
| 取文件下载 URL | `get_content_download_url`（不区分项目环境，见下） | 同左 |

- `get_content_download_url` 通过请求头里的服务 ID（或模板空间 ID）定位内容，服务端按服务反查项目环境，任何环境的服务都能用。
- 旧 MCP `bk-bscp-prod-file-manage` 工具集里的 `get_released_config_item` / `list_released_config_item` 是历史遗留工具，参数放在请求头里且只作用于默认环境，
  优先使用 `Config_GetReleasedConfigItem` / `Config_ListReleasedConfigItems` 及其 `Config_Env_*` 版本。

## 项目与环境：两套工具怎么选

bscp 在业务（biz）和服务（app）之间新增了项目（project）和环境（env）两级，每个服务归属于某个项目下的某个环境。
每个业务都有一个默认项目（`spec.is_default=true`，名称通常为"默认项目"），默认项目下有默认环境（名称 `Default`）。

- **原有工具**（如 `Config_ListConfigItems`）只作用于默认项目的默认环境。
- **`Config_Env_*` 工具**在路径中必填 `projectId`、`envId`，可查看任意项目环境下的服务；
  与原有工具一一对应，除 `projectId`/`envId` 外参数与约束相同。工具名一般是 `Config_Xxx` 对应 `Config_Env_Xxx`，
  唯一例外是 `Config_ListReleasedAppBoundTmplRevisions` 对应 `Config_Env_ListReleasedAppBoundTmplRevs`（受工具名长度限制缩写）。

选择规则：

1. **用户指定了项目或环境**（名称、标识或 ID 均可）：先解析出 `projectId`、`envId`（见下），后续全程使用 `Config_Env_*`。
2. **用户没提项目环境**：先用原有工具在默认环境里定位服务；按服务名找不到时，**不要直接判定"服务不存在"**，
   而是问用户该服务在哪个项目、哪个环境（可用 `Config_ListProjects` 列出项目供选择），再改用 `Config_Env_*`。
3. **同一条查看链路内不混用两套工具**：定位服务用了哪套，后续查配置项、查版本都用同一套，并沿用同一组 `projectId`/`envId`。
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

用户往往只抛一个模糊意图（如"看看某文件服务的配置 / 看下文件内容"）而不知道要给哪些参数。
**不要一次性罗列一堆参数把用户劝退，也不要臆造参数**；按下面的方式**分步反问，一次只问当前缺的一个关键信息**。

### 通用引导步骤（任何意图先做）

1. **确认业务 ID（bizId）**：用户没给就先问"请提供业务 ID（bizId）"。
2. **确认项目环境**：用户提到了项目或环境，就解析出 `projectId`/`envId` 并改用 `Config_Env_*`；
   没提就按默认环境走原有工具，不必主动追问（见「项目与环境」）。
3. **确认服务名 → 解析 appId**：拿到服务名后调 `Config_GetAppByName`（或先 `Config_ListAppsBySpaceRest`
   让用户从列表里挑）得到 `appId`，并**校验 `config_type=file`**；若不是 file 型（如 kv 型），
   直接告知"该服务不是文件型服务"并停止（R-002）。默认环境里找不到该服务时，按「项目与环境」的规则 2 询问项目环境。
4. 参数齐了再进入对应意图的动作；能从上下文推断的（如上一步已拿到的 appId、projectId、envId）不要重复问。

### 两类查看意图的最小引导

| 用户说 | 最少还需要问 | 拿齐后动作 |
|--------|-------------|-----------|
| 查询文件配置 / 看某文件服务配置 | bizId、服务名（可选：草稿态还是已发布、哪个版本） | 列配置项和绑定的模板文件，展示 name / path / signature / byte_size |
| 查看某个文件的内容 | bizId、服务名、文件名或路径 | 从配置项列表找到 signature，用 `get_content_download_url` 取临时 URL，交给用户直连存储下载查看 |

展示结果时写明所在的项目和环境（默认环境也要写明），避免用户误以为看的是另一个环境。

### 遇到写操作诉求如何处置

若用户要求**新增/更新/删除配置项、批量导入、生成版本、发布/灰度发布**等写操作：
**本 skill 不执行这些操作**。直接说明"当前文件型 skill 仅支持查看，写操作请走 UI / SDK"，
不要臆造或调用写工具，也不要伪造执行结果。

## ⚠️ 核心规则

1. **本 skill 仅只读查看**：不做配置项增删改、批量导入、生成版本、发布/灰度发布等任何写操作。
2. **文件型操作只适用于 `config_type=file` 的 app**；对 KV 型 app 操作文件接口会报错，务必先校验（R-002）。
3. **下载 URL 只返回临时预签名 URL 与有效期，不透传文件字节**；由用户用该 URL 直连存储下载查看，
   避免大文件穿透管理面/网关。
4. **查看内容依赖已上传的内容 signature**：signature 指向的内容须已由 UI/SDK 上传；未上传会报"内容未上传"，
   此时说明内容尚未上传，不产生指向空对象的引用（R-004）。
5. **列表接口必须显式给分页**：`ListAppsBySpaceRest`、`ListConfigItems`、`ListReleasedConfigItems`、`ListReleases`、
   `ListProjects`、`ListEnvironments` 要么传 `all:true`，要么显式传 `limit`（1–1000，配合 `start` 翻页），
   否则报 `page.limit value should >= 1`。
6. **服务的完整文件 = 非模板配置项 + 绑定的模板文件**：只列配置项会漏掉通过模板套餐绑定的文件，
   查"全部文件"时两类都要查。
7. **原有工具只作用于默认项目的默认环境**；查看其他项目环境下的服务必须用 `Config_Env_*`，且同一链路不混用两套工具。

## 领域模型速览（F-001）

```
biz（业务） → project（项目） → env（环境） → app（服务，config_type=file）
                                                 ├─ config_item（非模板配置项：元数据 + 内容 signature）
                                                 ├─ 绑定的模板文件（来自模板空间，带 template_space_id + signature）
                                                 └─ release（不可变版本快照，含当时的配置项与模板文件）
                          content（内容对象，按 signature 标识，已上传，只读引用）
```

- 每个业务有一个默认项目，默认项目下有默认环境 `Default`；原有工具只作用于这个环境。
- 文件型配置项由「内容 signature（SHA256）+ 元数据（path / name / byte_size / 权限等）」组织，
  **区别于 KV 型的 key / kvType / value**。
- 草稿态与已发布的配置项都可查询；本 skill 只读，不改变任何状态。
- content 是对象存储里以 signature 标识的已上传文件内容，本 skill **只读引用**，不上传、不透传字节。

## 只读查看调用编排（F-001 / F-006）

标准查看链路（工具名以原有工具书写，非默认环境换成对应的 `Config_Env_*` 并补上 `projectId`/`envId`）：

0. **确定项目环境**（用户指定了项目或环境时）→ 拿 `projectId`/`envId`，见「解析 projectId / envId」。
1. **定位服务** → 拿 `appId`
   - 已知业务：`Config_ListAppsBySpaceRest`（按 bizId 列 app，MCP schema 里有 `configType` 时可传 `file` 过滤）
   - 已知服务名：`Config_GetAppByName`
   - 从返回结果的 `config_type` 字段辨别 **file 型** app，非 file 型直接停止（R-002）
2. **查询配置项**
   - 草稿态：`Config_ListConfigItems`（非模板配置项）+ `Config_ListAppBoundTmplRevisions`（绑定的模板文件）；
     单个配置项详情用 `Config_GetConfigItem`
   - 已发布：先用 `Config_ListReleases` / `Config_GetReleaseByName` 拿 `releaseId`，再
     `Config_ListReleasedConfigItems` + `Config_ListReleasedAppBoundTmplRevisions`；单项详情用 `Config_GetReleasedConfigItem`
3. **查看文件内容**
   - 非模板配置项：`get_content_download_url`，请求头 `X-Bscp-App-Id=<appId>`、`X-Bkapi-File-Content-Id=<signature>`
   - 模板文件：请求头改为 `X-Bscp-Template-Space-Id=<template_space_id>`、`X-Bkapi-File-Content-Id=<signature>`
   - 响应只含临时下载 URL 与有效期，**不含文件字节**，由用户用该 URL 直连存储下载查看。

## 参数获取

- `bizId`：来自蓝鲸平台上下文（CMDB 业务 / 空间）或请求头 `X-Bkapi-Biz-Id`；MCP **不提供列 biz 的工具**，需由用户/上下文给出。
  `get_content_download_url` 的路径参数叫 `biz_id`，值相同。
- `projectId` / `envId`：见「解析 projectId / envId」。
- `appId`：通过 `Config_ListAppsBySpaceRest` 或 `Config_GetAppByName` 获取；非默认环境用对应的 `Config_Env_*`。
- **辨别 file 型 app**：从上述工具返回结果里读 `config_type` 字段，取 `config_type=file` 的 app。
- `releaseId`：`Config_ListReleases`（列版本让用户挑）或 `Config_GetReleaseByName`（已知版本名）返回的 `id`。
- 内容 `signature`（SHA256，64 位十六进制）：非模板配置项从 `commit_spec.content.signature` 读取，
  模板文件从模板版本列表的 `signature` 读取。
- `template_space_id`：模板版本列表里每项自带，取模板文件下载 URL 时使用。

## 报错 → 原因 → 处置

| 报错关键字 | 原因 | 处置 |
|-----------|------|------|
| `内容未上传` / `file content not uploaded` / `file content not found` | 引用的 signature 尚未上传到对象存储 | 说明该内容尚未上传，无法取下载 URL 查看；上传走 UI/SDK |
| `not a file type service` / 服务类型不符 | 对非 file 型 app 操作文件接口 | 确认目标 app 的 `config_type=file`（R-002） |
| `app does not belong to the specified project or environment` | 服务不在所传项目环境下；或用原有工具查看了非默认环境的服务 | 向用户确认服务所在的项目环境，重新解析 `projectId`/`envId`，改用 `Config_Env_*` |
| `page.limit value should >= 1` | 列表接口未传 `limit` 且 `all≠true` | 传 `all:true` 全量，或显式 `limit`(1–1000) |
| 鉴权失败 / 无权限 | 未通过业务/服务（内容）鉴权 | 确认对该 biz/app 有权限；鉴权失败不会返回下载 URL（不泄露） |

## 场景化示例

以下为调用序列示意（`bizId` / `appId` / `signature` 等用占位符，实际以获取到的值为准）。

### 1) 查看默认环境某文件型服务的全部草稿态文件

```
Config_GetAppByName {bizId, appName} → 校验 config_type=file，取 appId
Config_ListConfigItems {bizId, appId} body: {all:true}            → 非模板配置项的 name / path / signature / byte_size
Config_ListAppBoundTmplRevisions {bizId, appId}                   → 绑定的模板文件
```

### 2) 查看指定项目环境下某个已发布文件的内容

用户说"看下 game 项目 prod 环境里 svc-a 当前版本的 server.yaml"：

```
Config_GetProjectByKey {bizId, projectKey:"game"} → projectId     // 只知道名称时改用 Config_ListProjects {bizId, all:true}
Config_GetEnvironmentByName {bizId, projectId, envName:"prod"} → envId
Config_Env_GetAppByName {bizId, projectId, envId, appName:"svc-a"} → 校验 config_type=file，取 appId
Config_Env_ListReleases {bizId, projectId, envId, appId, all:true} → 让用户确认版本，取 releaseId
Config_Env_ListReleasedConfigItems {bizId, projectId, envId, appId, releaseId} body: {all:true}
  → 按 name 找到 server.yaml，读 commit_spec.content.signature
  // 不在非模板配置项里时，再查 Config_Env_ListReleasedAppBoundTmplRevs
get_content_download_url path: {biz_id:bizId} header: {X-Bscp-App-Id:appId, X-Bkapi-File-Content-Id:signature}
  → 临时下载 URL + 有效期    // 只拿 URL，不透传字节；回复里写明"game 项目 / prod 环境"
```

### 3) 引用的内容未上传的处置

```
get_content_download_url {..., X-Bkapi-File-Content-Id:"<未上传的 sha256>"} → 报"内容未上传"
→ 告知用户：该内容尚未上传，无法查看；上传走 UI/SDK
```

## 文件型 vs KV 型差异（速查）

| 维度 | 文件型（本 skill，只读查看） | KV 型（见 bkbscp-kv-config） |
|------|------------------|--------------------------|
| 配置对象 | config_item：signature（SHA256）+ 元数据（path/name/byte_size/权限），另有绑定的模板文件 | kv：key / kvType / value |
| 类型校验 | 操作前校验 `config_type=file` | 操作前校验 `config_type=kv` |
| 支持能力 | **仅查看**：查询配置项元数据 + 下载 URL 查看内容 | 查询 + 增删改 + 发布 |
| 查看内容 | 用 `get_content_download_url` 取临时 URL 直连存储查看（不透传字节） | `Config_ListReleasedKvs` 直读已发布值 |
| 项目环境 | 原有工具限默认环境；其他环境用 `Config_Env_*` | 同左 |

## 说明

本文的领域约束与操作规范可能随 bscp 版本演进而变化。实际以工具调用的返回结果和报错信息为准；
遇到与本文不一致的情况，按报错对照表处置或咨询 bscp 平台。
