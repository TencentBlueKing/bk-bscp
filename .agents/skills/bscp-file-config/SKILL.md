---
name: bscp-file-config
slug: bscp-file-config
version: 2.0.0
description: |
  bscp（蓝鲸基础配置平台）文件型配置只读查看指引。为挂载了 bk-bscp-prod-file-manage 的模型补充
  MCP 工具 schema 表达不了的领域知识：文件型（config_type=file）配置的领域模型、只读查看编排、
  参数获取、字段级业务约束与报错处置，帮助模型正确完成"定位服务 → 查询配置项元数据 →
  取下载 URL 查看文件内容"闭环。
  Use this skill whenever the user asks to 查看 bscp 文件配置, 看某文件型服务的配置,
  查询文件配置项, 看文件配置内容, 取文件下载 URL 查看文件内容,
  or invokes bk-bscp-prod-file-manage 的文件型配置项查询 / 文件下载 URL 工具。
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
用下载 URL 查看文件内容**。**本 skill 只支持查看，不支持任何写操作**：不做配置项增删改、
不做批量导入、不生成版本、不发布/灰度发布。文件内容上传与配置项变更、发版仍走 UI / SDK。

## 前置条件

本 skill 依赖 `bk-bscp-prod-file-manage`（蓝鲸 API 网关提供的 bscp **文件型专用** MCP Server）暴露的工具。
只读查看所需的工具（服务定位、文件配置项查询、下载 URL）都由这一个 MCP 提供，与 KV 型的
`bk-bscp-prod-server-mcp` 相互独立。**开始操作前必须先确认这些工具已可用**：

1. 检查当前会话是否已挂载 `bk-bscp-prod-file-manage`，能看到 `Config_ListAppsBySpaceRest` /
   `Config_GetAppByName` 等服务定位工具。
2. 只读查看还需要「文件配置项查询」工具与「文件下载 URL」工具，同样由 `bk-bscp-prod-file-manage`
   提供。**这些工具须在蓝鲸 API 网关注册后才会出现在 MCP 工具集里**。
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
3. 参数齐了再进入对应意图的动作；能从上下文推断的（如上一步已拿到的 appId）不要重复问。

### 两类查看意图的最小引导

| 用户说 | 最少还需要问 | 拿齐后动作 |
|--------|-------------|-----------|
| 查询文件配置 / 看某文件服务配置 | bizId、服务名 | 列草稿态配置项或已发布配置项，展示 sign / path / name / byte_size |
| 查看某个文件的内容 | bizId、服务名、目标内容 sign（或先列配置项找到 sign） | 用下载 URL 工具取临时 URL，交给用户直连存储下载查看 |

### 遇到写操作诉求如何处置

若用户要求**新增/更新/删除配置项、批量导入、生成版本、发布/灰度发布**等写操作：
**本 skill 不执行这些操作**。直接说明"当前文件型 skill 仅支持查看，写操作请走 UI / SDK 或对应的写工具"，
不要臆造或调用写工具，也不要伪造执行结果。

## ⚠️ 核心规则

1. **本 skill 仅只读查看**：不做配置项增删改、批量导入、生成版本、发布/灰度发布等任何写操作。
2. **文件型操作只适用于 `config_type=file` 的 app**；对 KV 型 app 操作文件接口会报错，务必先校验（R-002）。
3. **下载 URL 只返回临时预签名 URL 与有效期，不透传文件字节**；由用户用该 URL 直连存储下载查看，
   避免大文件穿透管理面/网关。
4. **查看内容依赖已上传的内容 sign**：sign 指向的内容须已由 UI/SDK 上传；未上传会报"内容未上传"，
   此时说明内容尚未上传，不产生指向空对象的引用（R-004）。

## 领域模型速览（F-001）

```
biz（业务） → app（服务，config_type=file） → config_item（配置项：sign + 元数据） → release（不可变版本快照）
                                                    │
                          content（内容对象，按 sign 标识，已上传，只读引用）
```

- 文件型配置项由「内容 sign（SHA256）+ 元数据（path / name / byte_size / 权限等）」组织，
  **区别于 KV 型的 key / kvType / value**。
- 草稿态配置项与已发布配置项都可查询；本 skill 只读，不改变任何状态。
- content 是对象存储里以 sign 标识的已上传文件内容，本 skill **只读引用**，不上传、不透传字节。

## 只读查看调用编排（F-001 / F-006）

标准查看链路：

1. **定位服务** → 拿 `appId`
   - 已知业务：`Config_ListAppsBySpaceRest`（按 bizId 列 app）
   - 已知服务名：`Config_GetAppByName`
   - 从返回结果的 `config_type` 字段辨别 **file 型** app，非 file 型直接停止（R-002）
2. **查询配置项**（F-001）
   - 草稿态配置项列表 / 详情、已发布配置项列表（读 sign / path / name / byte_size）
   - 需要查看某文件内容时，先从配置项列表拿到目标内容 sign
3. **查看文件内容**（F-006）
   - 用**文件下载 URL 工具**对目标内容 sign 取临时预签名下载 URL（响应只含 `download_url` +
     `expire_seconds`，**不含文件字节**），再由用户用该 URL 直连存储下载查看。

## 参数获取

- `bizId`：来自蓝鲸平台上下文（CMDB 业务 / 空间）或请求头 `X-Bkapi-Biz-Id`；MCP **不提供列 biz 的工具**，需由用户/上下文给出。
- `appId`：通过 `Config_ListAppsBySpaceRest`（按 bizId 列 app）或 `Config_GetAppByName`（已知服务名）获取。
- **辨别 file 型 app**：从上述工具返回结果里读 `config_type` 字段，取 `config_type=file` 的 app。
- 内容 `sign`（SHA256，64 位十六进制）：从配置项列表 / 详情中读取，用于取下载 URL 查看内容。

## 报错 → 原因 → 处置

| 报错关键字 | 原因 | 处置 |
|-----------|------|------|
| `内容未上传` / `file content not uploaded` / `file content not found` | 引用的 sign 尚未上传到对象存储 | 说明该内容尚未上传，无法取下载 URL 查看；上传走 UI/SDK |
| `not a file type service` / 服务类型不符 | 对非 file 型 app 操作文件接口 | 确认目标 app 的 `config_type=file`（R-002） |
| 鉴权失败 / 无权限 | 未通过业务/服务（内容）鉴权 | 确认对该 biz/app 有权限；鉴权失败不会返回下载 URL（不泄露） |

## 场景化示例

以下为调用序列示意（`bizId` / `appId` / `sign` 用占位符，实际以获取到的值为准）。
工具名以当前 MCP 工具集实际暴露的为准；文件配置项查询与下载 URL 工具须已在网关注册。

### 1) 查看某文件型服务的配置项列表

```
Config_ListAppsBySpaceRest {bizId} → 从结果中找 config_type=file 的目标 app，取其 appId
列出文件配置项 {bizId, appId} → 展示每项的 sign / path / name / byte_size
```

### 2) 查看某个文件的内容

```
Config_GetAppByName {bizId, 服务名} → 校验 config_type=file，取 appId
列出文件配置项 {bizId, appId} → 找到目标文件的内容 sign
下载URL工具 {bizId, appId, sign} → {download_url, expire_seconds:3600}   // 只拿 URL, 不透传字节
// 由用户用 download_url 直连存储下载查看文件内容
```

### 3) 引用的内容未上传的处置

```
下载URL工具 {..., sign:"<未上传的 sha256>"} → 报"内容未上传"
→ 告知用户：该内容尚未上传，无法查看；上传走 UI/SDK
```

## 文件型 vs KV 型差异（速查）

| 维度 | 文件型（本 skill，只读查看） | KV 型（见 bscp-kv-config） |
|------|------------------|--------------------------|
| 配置对象 | config_item：sign（SHA256）+ 元数据（path/name/byte_size/权限） | kv：key / kvType / value |
| 类型校验 | 操作前校验 `config_type=file` | 操作前校验 `config_type=kv` |
| 支持能力 | **仅查看**：查询配置项元数据 + 下载 URL 查看内容 | 查询 + 增删改 + 发布 |
| 查看内容 | 用**下载 URL 接口**取临时 URL 直连存储查看（不透传字节） | `Config_ListReleasedKvs` 直读已发布值 |

## 说明

本文的领域约束与操作规范可能随 bscp 版本演进而变化。实际以工具调用的返回结果和报错信息为准；
遇到与本文不一致的情况，按报错对照表处置或咨询 bscp 平台。
