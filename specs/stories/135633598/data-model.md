# Data Model — Story 135633598

> 本需求**不新增/修改任何数据库表、字段或索引，不涉及数据迁移**（承接 spec「架构影响：
> 数据模型变更：无」）。此处仅登记：1）新增的 HTTP 响应 DTO；2）本期只读引用的既有领域实体。

## 1. 新增数据传输对象（DTO）

### DownloadURLResponse（下载 URL 接口响应体）

新下载 URL 接口（FR-007/FR-008）的响应结构，**只含 URL 与有效期，不含文件字节**。

| 字段 | JSON | 类型 | 说明 | 约束 |
|------|------|------|------|------|
| DownloadURL | `download_url` | string | 临时预签名下载 URL（取 `Provider.DownloadLink` 返回切片的首个非空元素） | 非空；到期失效 |
| ExpireSeconds | `expire_seconds` | int | URL 有效期秒数 | 固定 `3600`（引用 `repository.TempDownloadURLExpireSeconds`） |

- **来源**：`Provider.DownloadLink(kt, sign, fetchLimit=1)` → `[]string`；多副本（ha）取首个（TR-001/AC-T02）。
- **放置**：service 包内定义（或就近置于 `repository` 包，与既有 `MetadataResponse`/`ObjectMetadata`
  同风格），实现时取其一保持一致；随 `rest.OKRender` 包裹返回。
- **不持久化**：纯响应对象，无存储。

### 常量（非新增数据，仅可见性变更）

- `repository.TempDownloadURLExpireSeconds = 3600`：由 `bkrepo.go` 现有未导出常量
  `tempDownloadURLExpireSeconds` 导出而来，供 handler 引用，避免 `expire_seconds` 魔法数字。

## 2. 本期只读引用的既有领域实体（无变更）

| 实体 | 载体 | 本期使用方式 |
|------|------|-------------|
| biz（业务） | `kt.BizID`（path `{biz_id}`） | 鉴权 + 存储路径归属 |
| app（服务，config_type=file） | `kt.AppID` / header `X-Bscp-App-Id` | 鉴权（ContentVerified）+ skill 侧 config_type 校验 |
| config_item（草稿态配置项） | config-server 既有表 | 仅 skill 编排既有接口 CRUD，无服务端改动 |
| content（内容对象，按 sign 标识） | 对象存储 bkrepo/cos，`sign`=SHA256（header `X-Bkapi-File-Content-Id`） | 只读引用：`Metadata` 预检存在性 + `DownloadLink` 生成 URL；本期不上传 |
| release（不可变版本快照） | config-server 既有表 | 仅 skill 编排 `CreateRelease`/`Publish`，无服务端改动 |

## 3. 关系

```
biz ──1:N── app(config_type=file) ──1:N── config_item(sign + 元数据)
                                              │
                              content(sign, 已上传, 只读) ──DownloadLink──▶ DownloadURLResponse
config_item 草稿态 ──CreateRelease──▶ release(快照) ──Publish──▶ 生效
```

## 4. 校验规则（服务端强约束，落在新 handler）

- `sign`：`GetFileSign` 校验为 64 位 SHA256（长度=64），否则 400（安全红线 1）。
- `content` 存在性：`Metadata` 命中 `errf.ErrFileContentNotFound` → 「内容未上传」错误，不生成 URL（AC-T01）。
- 归属/鉴权：`ContentVerified` 校验 sign 与 app/template_space 归属；handler 内 IAM Authorize（防越权，安全红线 2）。
