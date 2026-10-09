---
name: bscp-file-config
slug: bscp-file-config
version: 2.1.0
description: |
  bscp（蓝鲸基础配置平台）文件型配置只读查看指引。为挂载了 bk-bscp-prod-file-manage 的模型补充
  MCP 工具 schema 表达不了的领域知识：文件型（config_type=file）配置的领域模型、只读查看编排、
  参数获取、字段级业务约束与报错处置，帮助模型正确完成"定位服务 → 查询配置项元数据（含模版导入项）
  → 按版本/草稿态查询 → 取下载 URL 查看文件内容"闭环。覆盖 app 自身配置项与配置模版导入配置项两类。
  Use this skill whenever the user asks to 查看 bscp 文件配置, 看某文件型服务的配置,
  查询文件配置项, 看模版导入的配置项, 看文件配置内容, 按版本名查看已发布配置, 取文件下载 URL 查看文件内容,
  or invokes bk-bscp-prod-file-manage 的文件型配置项查询 / 模版绑定版本查询 / 服务版本查询 / 文件下载 URL 工具。
metadata:
  requires:
    mcps: ["bk-bscp-prod-file-manage"]
---

# bscp 文件型配置只读查看指引

## 定位

本 skill 只补充 MCP schema 表达不了的知识（跨工具编排、字段间业务约束、错误语义、领域模型），
**不重复** MCP 工具已有的单字段描述——填参时字段含义以 MCP 工具 schema 为准。文中的业务
约束来自 bscp 服务端的实际校验规则，会随 bscp 版本演进；若与调用返回的报错不一致，以服务端返回为准。

适用范围：**只读查看** `config_type=file` 服务的文件型配置——**定位服务 → 查询配置项元数据 →
用下载 URL 查看文件内容**。查询覆盖**两类配置项**：app 自身直接创建的「非模版配置项」，以及从
配置模版套餐导入的「模版导入配置项」；两类都支持查**草稿态（未命名版本）**与**指定已发布版本**。
**本 skill 只支持查看，不支持任何写操作**：不做配置项增删改、不做批量导入、不生成版本、不发布/灰度发布。
文件内容上传与配置项变更、发版仍走 UI / SDK。

## 前置条件

本 skill 依赖 `bk-bscp-prod-file-manage`（蓝鲸 API 网关提供的 bscp **文件型专用** MCP Server）暴露的工具。
只读查看所需的工具（服务定位、文件配置项查询、下载 URL）都由这一个 MCP 提供，与 KV 型的
`bk-bscp-prod-server-mcp` 相互独立。**开始操作前必须先确认这些工具已可用**：

1. 检查当前会话是否已挂载 `bk-bscp-prod-file-manage`，能看到 `Config_ListAppsBySpaceRest` /
   `Config_GetAppByName` 等服务定位工具。
2. 只读查看还需要以下几组工具，同样由 `bk-bscp-prod-file-manage` 提供：
   - 非模版配置项查询：`Config_ListConfigItems` / `Config_GetConfigItem`（草稿态）、
     `Config_ListReleasedConfigItems` / `Config_GetReleasedConfigItem`（已发布）。
   - 模版导入配置项查询：`Config_ListAppBoundTmplRevisions`（草稿态）、
     `Config_ListReleasedAppBoundTmplRevisions`（已发布）。
   - 版本查询：`Config_ListReleases`（列版本）、`Config_GetReleaseByName`（按版本名取 releaseId）。
   - 文件下载 URL：`get_content_download_url`。
   **这些工具须在蓝鲸 API 网关注册并对应用授权后才会出现在 MCP 工具集里并可调用**；缺哪组就退化到可用工具的编排。
3. **若某个工具不存在**：不要臆造工具名或伪造调用结果，直接告知用户"未检测到 `bk-bscp-prod-file-manage`
   或对应工具，可能尚未挂载 / 在网关注册"，并说明缺失的能力，然后停下等待用户确认接入，或退化为对可用工具的编排。
4. 工具可用后再按后续章节执行。

> 说明：文件下载 URL 接口只返回临时预签名 URL 与有效期，不透传文件字节；网关注册后自动纳入
> `bk-bscp-prod-file-manage` 工具集，用于查看文件内容。

## 交互引导（面向不熟悉闭环的用户）

用户往往只抛一个模糊意图（如"看看某文件服务的配置 / 看下文件内容"）而不知道要给哪些参数。
**不要一次性罗列一堆参数把用户劝退，也不要臆造参数**；按下面的方式**分步反问，一次只问当前缺的一个关键信息**。

### 通用引导步骤（任何意图先做）

1. **确认业务 ID（bizId）**：用户没给就先问"请提供业务 ID（bizId）"。
2. **确认服务名 → 解析 appId**：拿到服务名后调 `Config_GetAppByName`（或先 `Config_ListAppsBySpaceRest`
   让用户从列表里挑）得到 `appId`，并**校验 `config_type=file`**；若不是 file 型（如 kv 型），
   直接告知"该服务不是文件型服务"并停止（R-002）。
3. **确认查草稿态还是已发布版本**：默认查草稿态（未命名版本）；用户提到"某个版本 / 已发布 / 版本名"
   时走已发布链路，用 `Config_ListReleases` 让用户挑，或用 `Config_GetReleaseByName` 把版本名换成 `releaseId`。
4. 参数齐了再进入对应意图的动作；能从上下文推断的（如上一步已拿到的 appId / releaseId）不要重复问。

> ⚠️ 一个服务的配置项通常**分两处**：`Config_ListConfigItems` 只返回非模版配置项，模版导入的配置项
> 要另调 `Config_ListAppBoundTmplRevisions`。**若发现 `Config_ListConfigItems` 返回的 `count`
> 小于 `total_quantity`，差额就是模版导入项**，务必补查模版接口，否则会漏掉大部分配置。

### 查看意图的最小引导

| 用户说 | 最少还需要问 | 拿齐后动作 |
|--------|-------------|-----------|
| 查询文件配置 / 看某文件服务配置（草稿态） | bizId、服务名 | `Config_ListConfigItems`（非模版）+ `Config_ListAppBoundTmplRevisions`（模版导入）合并展示 sign / path / name / byte_size |
| 看某已发布版本的配置 | bizId、服务名、版本名或 releaseId | 先解析 releaseId，再 `Config_ListReleasedConfigItems` + `Config_ListReleasedAppBoundTmplRevisions` 合并展示 |
| 看某版本里某个具体配置项 | bizId、服务名、releaseId、目标 `config_item_id` | `Config_GetReleasedConfigItem`（注意填**原始 config_item_id**，见核心规则 6） |
| 查看某个文件的内容 | bizId、服务名、目标内容 sign（或先列配置项找到 sign） | 用 `get_content_download_url` 取临时 URL，交给用户直连存储下载查看（模版项见规则 7、8） |

### 遇到写操作诉求如何处置

若用户要求**新增/更新/删除配置项、批量导入、生成版本、发布/灰度发布**等写操作：
**本 skill 不执行这些操作**。直接说明"当前文件型 skill 仅支持查看，写操作请走 UI / SDK 或对应的写工具"，
不要臆造或调用写工具，也不要伪造执行结果。

## ⚠️ 核心规则

1. **本 skill 仅只读查看**：不做配置项增删改、批量导入、生成版本、发布/灰度发布等任何写操作。
2. **文件型操作只适用于 `config_type=file` 的 app**；对 KV 型 app 操作文件接口会报错，务必先校验（R-002）。
3. **下载 URL 是一次性临时预签名 URL**：响应只含 `download_url` 与 `expire_seconds`（约 1 小时），不透传文件字节；
   **每个 URL 只能成功下载一次，用过即失效**——复用同一 URL 会返回 `400 Bad Request`（客户端表现为下载到 0 字节空文件），
   过期或用尽后须重新调 `get_content_download_url` 取新 URL。由用户用该 URL 直连存储下载查看，避免大文件穿透管理面/网关。
   **模型不要为"核对内容"先把要给用户的 URL 自己下载一遍**：消费后用户再下就会拿到空文件；确需自查另取一个 URL 自用。
4. **查看内容依赖已上传的内容 sign**：sign 指向的内容须已由 UI/SDK 上传；未上传会报"内容未上传"，
   此时说明内容尚未上传，不产生指向空对象的引用（R-004）。
5. **配置项分两类、须合并查**：非模版配置项（`Config_ListConfigItems` 等）与模版导入配置项
   （`Config_ListAppBoundTmplRevisions` 等）来自不同接口；只查其一会漏配置项。用 `Config_ListConfigItems`
   返回的 `total_quantity` 与 `count` 之差判断是否有模版导入项需要补查。
6. **`Config_GetReleasedConfigItem` 的 id 坑**：`configItemId` 参数要填**原始 `config_item_id`**
   （如 `45959`），**不是** `Config_ListReleasedConfigItems` 返回里的那个 `id`（已发布记录主键，如 `370179`）；
   填错会报 `record not found`。
7. **模版导入项在已发布版本里可能被变量渲染**：已发布模版项返回中 `signature`（渲染后实际内容）
   可能 ≠ `origin_signature`（模版原始内容），`byte_size` 同理。要下载**该版本实际生效的内容**用
   `signature`；要看模版原文用 `origin_signature`。非模版项与无变量的模版项两者相同。
8. **下载 URL 的归属 header 分模版/非模版**：`get_content_download_url` 除必填
   `X-Bkapi-File-Content-Id`（即 sign）外，非模版配置项带 `X-Bscp-App-Id`，模版导入配置项带
   `X-Bscp-Template-Space-Id`（取模版项所属 `template_space_id`）。

## 领域模型速览（F-001）

```
biz（业务）
  └ app（服务，config_type=file）
      ├ config_item（非模版配置项：sign + 元数据）─────────────┐
      ├ app_template_binding → template_set（模版套餐）        ├→ release（不可变版本快照）
      │      └ template_revision（模版导入配置项：sign + 元数据）┘
      └ content（内容对象，按 sign 标识，已上传，只读引用）
```

- 文件型配置项由「内容 sign（SHA256）+ 元数据（path / name / byte_size / 权限等）」组织，
  **区别于 KV 型的 key / kvType / value**。
- **一个 app 的文件配置项分两类**：
  - **非模版配置项**：直接建在 app 上，存于 `config_items`，用 `Config_ListConfigItems` 系列查。
  - **模版导入配置项**：app 绑定了模版套餐（`app_template_binding` → `template_set` → `template_revision`），
    这些配置项**不在** `config_items` 里，须用 `Config_ListAppBoundTmplRevisions` 系列查，返回按套餐分组，
    每项带 `template_space_id` / `template_set_id` / `template_revision_id` 及 sign 元数据。
- 草稿态（未命名版本）与已发布版本都可查询；`release` 是一次发布的不可变快照，用 `releaseId` 定位，
  版本名可用 `Config_GetReleaseByName` 换成 `releaseId`。本 skill 只读，不改变任何状态。
- content 是对象存储里以 sign 标识的已上传文件内容，本 skill **只读引用**，不上传、不透传字节。

## 只读查看调用编排（F-001 / F-006）

标准查看链路：

1. **定位服务** → 拿 `appId`
   - 已知业务：`Config_ListAppsBySpaceRest`（按 bizId 列 app）
   - 已知服务名：`Config_GetAppByName`
   - 从返回结果的 `config_type` 字段辨别 **file 型** app，非 file 型直接停止（R-002）
2. **（可选）确定版本** → 拿 `releaseId`（只在查已发布版本时需要）
   - 列版本让用户挑：`Config_ListReleases`（返回各版本 `id` / `name` / 发布状态）
   - 已知版本名：`Config_GetReleaseByName` → 拿 `release.id`
3. **查询配置项**（F-001）——**非模版 + 模版导入两类都要查，合并结果**
   - 草稿态（未命名版本）：
     - 非模版：`Config_ListConfigItems` / `Config_GetConfigItem`
     - 模版导入：`Config_ListAppBoundTmplRevisions`
   - 已发布版本（带 `releaseId`）：
     - 非模版：`Config_ListReleasedConfigItems` / `Config_GetReleasedConfigItem`（后者填原始 `config_item_id`，见规则 6）
     - 模版导入：`Config_ListReleasedAppBoundTmplRevisions`
   - 读 sign / path / name / byte_size；需要看内容时先拿到目标 sign（模版已发布项注意用渲染后 `signature`，见规则 7）
4. **查看文件内容**（F-006）
   - 用 `get_content_download_url` 对目标 sign 取临时预签名下载 URL（响应只含 `download_url` +
     `expire_seconds`，**不含文件字节**）；非模版项带 `X-Bscp-App-Id`、模版项带 `X-Bscp-Template-Space-Id`（规则 8）。
   - **把 URL 连同使用说明一起交给用户**（见下「返回 URL 后的使用说明」），由用户用该 URL 直连存储下载查看。
   - **不要自己先下载一遍再把同一 URL 给用户**：URL 一次性，被消费后用户再下就会拿到 `400`/空文件（规则 3）。

### 返回 URL 后的使用说明（务必随 URL 一起给用户）

拿到 `download_url` 后，除 URL 本身外，一并把下面几点告诉用户，避免"下载为空/失败"：

- **一次性**：该 URL 只能成功下载一次，用过即失效；再次使用会报 `400 Bad Request`（表现为下载到 0 字节空文件）。需要重下就回来重新取新 URL。
- **有效期**：默认约 1 小时（以 `expire_seconds` 为准），过期同样需重新取。
- **shell 里必须给 URL 加引号**：URL 带 `?token=...`，不加引号会被 shell 当通配符/参数拆断，只请求到半截而失败。
- **别用静默模式**：不要用 `wget -q`，改用能暴露 HTTP 错误的方式（如 `curl -f`），否则服务端报错时会静默写出空文件、看不到原因。
- **模版导入项区分内容**：用 `signature` 取的是该版本**渲染后**内容，用 `origin_signature` 取的是**模版原文**（规则 7），按需取对应 sign。

示例（下载到本地文件后查看内容）：

```bash
# curl：-f 让 HTTP 错误直接失败、不再静默生成空文件；URL 用单引号包住
curl -f -o <文件名> '<download_url>'
cat <文件名>

# 或 wget（去掉 -q 以便看到错误）
wget -O <文件名> '<download_url>'
```

## 参数获取

- `bizId`：来自蓝鲸平台上下文（CMDB 业务 / 空间）或请求头 `X-Bkapi-Biz-Id`；MCP **不提供列 biz 的工具**，需由用户/上下文给出。
- `appId`：通过 `Config_ListAppsBySpaceRest`（按 bizId 列 app）或 `Config_GetAppByName`（已知服务名）获取。
- **辨别 file 型 app**：从上述工具返回结果里读 `config_type` 字段，取 `config_type=file` 的 app。
- `releaseId`（查已发布版本才需要）：`Config_ListReleases` 列版本挑一个，或 `Config_GetReleaseByName`
  用版本名取 `release.id`。本 MCP 无"按 id 取版本"工具，但列表/按名都够用。
- `config_item_id`（非模版·`Config_GetReleasedConfigItem` 用）：从 `Config_ListConfigItems` /
  `Config_ListReleasedConfigItems` 返回里读 **`config_item_id`**（不是已发布记录的 `id`，见规则 6）。
- `template_space_id`（模版项下载用）：从 `Config_ListAppBoundTmplRevisions` /
  `Config_ListReleasedAppBoundTmplRevisions` 返回的套餐分组里读，配合 sign 走下载 URL。
- 内容 `sign`（SHA256，64 位十六进制）：从配置项列表 / 详情中读取，用于取下载 URL 查看内容。
  模版已发布项优先用渲染后的 `signature`（规则 7）。

## 报错 → 原因 → 处置

| 报错关键字 | 原因 | 处置 |
|-----------|------|------|
| `内容未上传` / `file content not uploaded` / `file content not found` | 引用的 sign 尚未上传到对象存储 | 说明该内容尚未上传，无法取下载 URL 查看；上传走 UI/SDK |
| 下载到 0 字节 / 空文件 / `400 Bad Request` | 该临时 URL 已被用过一次（一次性）或已过期 | 重新调 `get_content_download_url` 取新 URL 且只下载一次；shell 里给 URL 加引号、别用 `wget -q`（规则 3） |
| `record not found` | `Config_GetReleasedConfigItem` 的 `configItemId` 填成了已发布记录 `id` | 改填**原始 `config_item_id`**（规则 6） |
| `not a file type service` / 服务类型不符 | 对非 file 型 app 操作文件接口 | 确认目标 app 的 `config_type=file`（R-002） |
| `APP_NO_PERMISSION` / `App has no permission` / `bk_app_code=...` | 网关侧应用凭证对该 API 资源未授权（非用户级鉴权） | 属平台/网关配置：该接口未在网关注册或未对 app_code 授权/发布；告知用户，不臆造结果 |
| `Method Not Allowed` / `UNIMPLEMENTED` / `API_NOT_FOUND` | 网关注册的 HTTP method 与后端不一致 | 属平台/网关配置问题；告知用户由平台侧对齐 method 后重试 |
| 鉴权失败 / 无权限 | 未通过业务/服务（内容）鉴权 | 确认对该 biz/app 有权限；鉴权失败不会返回下载 URL（不泄露） |

## 场景化示例

以下为调用序列示意（`bizId` / `appId` / `sign` 用占位符，实际以获取到的值为准）。
工具名以当前 MCP 工具集实际暴露的为准；文件配置项查询与下载 URL 工具须已在网关注册。

### 1) 查看某文件型服务的完整配置项列表（草稿态，含模版导入项）

```
Config_GetAppByName {bizId, 服务名} → 校验 config_type=file，取 appId
Config_ListConfigItems {bizId, appId, all:true} → 非模版项（读 count 与 total_quantity）
// 若 count < total_quantity，说明有模版导入项，补查：
Config_ListAppBoundTmplRevisions {bizId, appId} → 模版导入项（按套餐分组，含 sign / template_space_id）
// 合并两边结果一起展示，条数应等于 total_quantity
```

### 2) 查看某个已发布版本的配置项（按版本名）

```
Config_GetReleaseByName {bizId, appId, releaseName:"v6"} → 取 release.id 作为 releaseId
Config_ListReleasedConfigItems {bizId, appId, releaseId, all:true} → 该版本非模版项
Config_ListReleasedAppBoundTmplRevisions {bizId, appId, releaseId} → 该版本模版导入项
// 合并展示；模版项注意 signature(渲染后) 可能 ≠ origin_signature(模版原文)
```

### 3) 取某版本里某个具体配置项详情

```
Config_ListReleasedConfigItems {...} → 找到目标项，记下它的 config_item_id（不是 id）
Config_GetReleasedConfigItem {bizId, appId, releaseId, configItemId:<config_item_id>} → 详情
// 若填成已发布记录 id 会报 record not found（规则 6）
```

### 4) 查看某个文件的内容（区分模版/非模版）

```
// 非模版项：
下载URL工具 {header: X-Bkapi-File-Content-Id=<sign>, X-Bscp-App-Id=<appId>, path: biz_id} → {download_url, expire_seconds}
// 模版导入项：
下载URL工具 {header: X-Bkapi-File-Content-Id=<signature>, X-Bscp-Template-Space-Id=<template_space_id>, path: biz_id} → {download_url, expire_seconds}
// 只拿 URL，不透传字节；URL 一次性、约 1 小时过期，随 URL 附带使用说明交给用户（见「返回 URL 后的使用说明」）
// 用户下载示例：curl -f -o <文件名> '<download_url>' && cat <文件名>（URL 必须加引号，勿用 wget -q）
```

### 5) 引用的内容未上传的处置

```
下载URL工具 {..., X-Bkapi-File-Content-Id:"<未上传的 sha256>"} → 报"内容未上传"
→ 告知用户：该内容尚未上传，无法查看；上传走 UI/SDK
```

## 文件型 vs KV 型差异（速查）

| 维度 | 文件型（本 skill，只读查看） | KV 型（见 bscp-kv-config） |
|------|------------------|--------------------------|
| 配置对象 | config_item：sign（SHA256）+ 元数据（path/name/byte_size/权限） | kv：key / kvType / value |
| 配置来源 | 两类：非模版配置项 + 模版套餐导入配置项（分不同接口查） | 单一：app 上的 kv |
| 类型校验 | 操作前校验 `config_type=file` | 操作前校验 `config_type=kv` |
| 支持能力 | **仅查看**：非模版 + 模版导入配置项元数据、草稿态/已发布版本、下载 URL 查看内容 | 查询 + 增删改 + 发布 |
| 查看内容 | 用**下载 URL 接口**取临时 URL 直连存储查看（不透传字节） | `Config_ListReleasedKvs` 直读已发布值 |

## 说明

本文的领域约束与操作规范可能随 bscp 版本演进而变化。实际以工具调用的返回结果和报错信息为准；
遇到与本文不一致的情况，按报错对照表处置或咨询 bscp 平台。
