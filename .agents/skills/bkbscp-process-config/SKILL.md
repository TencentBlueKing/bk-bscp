---
name: bkbscp-process-config
slug: bkbscp-process-config
version: 1.0.1
description: |
  bscp（蓝鲸基础配置平台）进程配置管理操作指引。为挂载了 bk-bscp-prod-process-config 的模型补充
  MCP 工具 schema 表达不了的领域知识：业务级进程模型、gsekit 插件动作到 bscp 工具的映射、
  表达式范围、异步批次轮询、配置「生成后再下发」两段式，以及写操作确认。
  帮助模型完成「按表达式定位进程 → 启停/托管或生成/检查/下发配置 → 轮询批次」。调用方默认业务已迁移到 bscp。
  Use this skill whenever the user asks to 操作 bscp 进程, 启停进程, 托管/取消托管, 生成配置,
  下发配置, 检查配置, 刷新进程实例, 同步 CMDB 和 GSE, 按集群/模块/进程别名操作进程,
  gsekit 进程配置, 标准运维进程插件,
  or invokes any bk-bscp-prod-process-config tool
  (Config_OperateProcess / Config_GenerateConfig / Config_CheckConfig / Config_PushConfig /
  Config_GetTaskBatchDetail / Config_ListProcess / Config_SyncCmdbGseStatus 等).
metadata:
  requires:
    mcps: ["bk-bscp-prod-process-config"]
---

# bscp 进程配置管理操作指引

## 定位

本 skill 只补充 MCP schema 表达不了的知识（跨工具编排、gsekit 动作映射、表达式语义、异步批次），
**不重复** MCP 工具已有的单字段描述。约束来自 bscp 服务端与标准运维 gsekit 插件的实际行为；
与调用返回不一致时，以服务端返回为准。

适用范围：`bk-bscp-prod-process-config`，对齐标准运维插件 `gsekit_job_exec`（执行命令）和
`gsekit_flush_process`（刷新进程实例）。进程配置挂在**业务**上，用 CMDB 环境类型
（`1` 测试 / `2` 体验 / `3` 正式）圈定范围，**不是** KV / 文件型那套项目（project）和环境（env）。

不覆盖：配置模板的创建/编辑/删除、模板绑定、缩容清除（`Config_OperateDeleteProcess`）、
更新托管信息（`Config_OperateUpdateRegisterProcess`）、KV 与文件型配置。这些接口在网关上存在，
但不在本 MCP 里。不要改用 `Config_Inner_*`，inner 接口只给标准运维等内部系统。
调用方默认业务已经迁移到 bscp，不要查询进程配置可见性开关。

## 前置条件

1. 确认当前会话已挂载 `bk-bscp-prod-process-config`，能看到 `Config_ListProcess`、`Config_OperateProcess` 等工具。
2. **工具不存在**：不要臆造工具名或结果。告知用户需要接入 `bk-bscp-prod-process-config`
   （网关 MCP 地址、`X-Bkapi-Authorization`、业务 `bizId`），然后停下。

## 工具速查

| 用途 | 工具 | 对应插件 |
|------|------|----------|
| 列进程 | `Config_ListProcess` | 插件表单由人填写范围；MCP 用它先发现进程 |
| 去重内网 IP | `Config_ListProcessInnerIPs` | IP 选择器 `process_status` |
| 列配置模板 | `Config_ListConfigTemplate` | 表单「配置模版」下拉 |
| 启停 / 托管 | `Config_OperateProcess` | 操作对象 process |
| 生成配置 | `Config_GenerateConfig` | `generate` |
| 检查配置 | `Config_CheckConfig` | `diff` |
| 下发配置 | `Config_PushConfig` | `release` 的第二段，入参是生成批次的 batchId |
| 轮询批次 | `Config_GetTaskBatchDetail` | `job_status`，每 5 秒一次 |
| 触发 CMDB/GSE 同步 | `Config_SyncCmdbGseStatus` | `flush_process` |
| 查同步状态 | `Config_CmdbGseStatus` | flush 的轮询 |

插件文案到 bscp 动作：

| 插件 | bscp |
|------|------|
| 启动 start / 停止 stop / 重启 restart / 重载 reload | `operateType` 同名 |
| 强制停止 force_stop | `operateType=kill` |
| 托管 set_auto | `operateType=register` |
| 取消托管 unset_auto | `operateType=unregister` |
| 生成配置 generate | `Config_GenerateConfig`，不要下发 |
| 下发配置 release | 先 `Config_GenerateConfig`，批次 `succeed` 后再 `Config_PushConfig` |
| 检查配置 diff | `Config_CheckConfig` |

## 交互引导

一次只问当前缺的一个信息，能从上下文推断的不要重复问。

1. **bizId**：没有就问「请提供业务 ID（bizId）」。MCP 不提供列业务的工具。调用方默认该业务已迁移到 bscp。
2. **环境类型**：`1` 测试、`2` 体验、`3` 正式。用户说「正式 / 线上 / prod」用 `"3"`，说「测试」用 `"1"`，说「体验」用 `"2"`。没说就问，不要默认正式。
3. **范围**：集群、模块、服务实例、进程别名、CC 进程 ID。未知的段用 `"*"`，但五段全是 `*` 等于该环境全部进程，必须先列出命中进程再让用户确认。
4. **动作**：启停托管、生成、检查，还是下发。下发还要问是否限定配置模板；不限定表示该业务全部模板的最新版。

| 用户说 | 还要确认 | 然后 |
|--------|----------|------|
| 看看有哪些进程 / 哪些机器 | 环境类型，可选过滤条件 | `Config_ListProcess` 或 `Config_ListProcessInnerIPs` |
| 启动 / 停止 / 重启 / 重载 / 托管 / 取消托管 / 强制停止 | 环境类型和范围 | 列出命中进程，用户确认后 `Config_OperateProcess`，再轮询 |
| 生成配置 | 环境类型、范围、可选模板 | 确认后 `Config_GenerateConfig`，轮询到 `succeed` 即停 |
| 检查配置 | 同上 | `Config_CheckConfig`，轮询后看 `compareStatus` |
| 下发配置 | 同上 | 生成 → 轮询成功 → `Config_PushConfig` → 再轮询 |
| 刷新进程 / 同步 CMDB 和 GSE | 仅 bizId；这是整业务同步 | 确认后 `Config_SyncCmdbGseStatus`，再轮询 `Config_CmdbGseStatus` |

### 写操作执行前必须二次确认

`Config_OperateProcess`、`Config_GenerateConfig`、`Config_CheckConfig`、`Config_PushConfig`、
`Config_SyncCmdbGseStatus` 会作用到现网进程或整业务同步。执行前展示：业务 ID、环境类型、表达式五段、
动作、命中进程（别名 / 集群 / 模块 / 服务实例 / IP / CC 进程 ID）、以及是否限定模板。
用户未确认不调用。正式环境（`environment=3`）单独写明。

## 核心规则

1. **范围用 `operateRange`**：`environment` + `expressionScope`。不要同时传 `processIds`，避免两套范围叠在一起。
2. **空段等于 `*`**。不要把没问到的段留空后当成「只操作用户说的那一个」；展示给用户的确认信息里，没指定的段要写成 `*`。
3. **下发不是一个接口**。`Config_PushConfig` 只接受生成配置返回的 `batchId`。生成批次不是 `succeed` 时不要 push。
4. **启停、生成、检查、下发都是异步批次**。拿到批次 ID 后轮询 `Config_GetTaskBatchDetail`，直到 `task_batch.status` 为终态。
5. **`partly_failed` 是失败**。向用户报告失败明细，不要说成成功。
6. **列表要分页**。`Config_ListProcess`、`Config_ListConfigTemplate` 传 `all:true`，或 `limit`（1–1000）加 `start`。不传 `limit` 会被当成 0。`Config_GetTaskBatchDetail` 没有 `all`，必须传 `start` 和 `limit`。
7. **进程操作轮询带 `ignoreErrorCodes: [828, 829]`**。828 是重复启动，829 是无需停止，标准运维插件把它们当成幂等成功。
8. **返回字段名不一致**：`Config_OperateProcess` 的批次 ID 是 `batchID`；生成 / 检查 / 下发是 `batchId`。两个都要认。
9. **不要用项目环境工具**。这里没有 `Config_Env_*`。用户说的「环境」是 CMDB 环境类型 1/2/3。

## 领域模型

```
biz（业务，调用方默认已迁移）
  → environment（1 测试 / 2 体验 / 3 正式）
    → 进程（集群.模块.服务实例.进程别名.CC进程ID）
    → 配置模板（可选，限定生成/检查/下发）
    → task_batch（异步批次：running / succeed / failed / partly_failed）
```

进程实例上常用字段：`spec.alias`、`spec.setName`、`spec.moduleName`、`spec.serviceName`、
`spec.innerIp`、`spec.status`（running / stopped）、`spec.managedStatus`（managed / unmanaged）、
`spec.environment`、`attachment.ccProcessId`。

## 表达式范围

`operateRange`：

```
environment: "1" | "2" | "3"
expressionScope:
  setName: 集群名
  moduleName: 模块名
  serviceName: 服务实例名
  processAlias: 进程别名
  processId: CC 进程 ID
configTemplateIds: [数字模板 ID]   # 仅生成/检查/下发；不传 = 全部模板最新版
```

插件字段对应：`bk_set_env`→`environment`，`bk_set_name`→`setName`，`bk_module_name`→`moduleName`，
`service_instance_name`→`serviceName`，`bk_process_name`→`processAlias`，`bk_process_id`→`processId`。

单段语法与 gsekit 一致，缺省或空为 `*`：

- `*` 任意
- 字面量：`gamesvr`
- 枚举：`[a,b]`
- 逗号列表：`a,b`
- 范围：数字 `1-3`，或单字符字母 `a-c`
- 排除：`!x`
- 仅 `processId` 支持结果切片，Python 切片语义，如 `[0:2]`

先 `Config_ListProcess`，`search.environment` 加上同样的 `search.expressionScope`，把命中列表给用户。
只要 IP 时用 `Config_ListProcessInnerIPs`，读返回的 `ips`。

## 调用编排

### 启停或托管

```
Config_ListProcess {bizId, all:true, search:{environment, expressionScope}}
# 展示命中进程，等用户确认
Config_OperateProcess {bizId} body:{operateType, operateRange:{environment, expressionScope}}
  → batchID
Config_GetTaskBatchDetail {bizId, batchId:batchID} body:{start:0, limit:100, ignoreErrorCodes:[828,829]}
```

`operateType`：`start` `stop` `restart` `reload` `kill` `register` `unregister`。

### 生成配置

```
Config_ListConfigTemplate {bizId, all:true}    # 用户要限定模板时，用返回的 id
Config_GenerateConfig {bizId} body:{operateRange}
  → batchId
# 轮询到 task_batch.status=succeed 结束。这一步没有下发到机器。
```

### 下发配置

```
Config_GenerateConfig → batchId
# 轮询到 succeed
Config_PushConfig {bizId} body:{batchId:<生成批次的 batchId>} → 新的 batchId
# 轮询新批次到 succeed
```

### 检查配置

```
Config_CheckConfig {bizId} body:{operateRange} → batchId
# 轮询结束后看任务详情的 compareStatus：
# SAME 一致，DIFFERENT 不一致，NEVER_PUBLISHED 从未下发，UNKNOWN 未知
```

### 轮询

读 `task_batch.status`：

| status | 处理 |
|--------|------|
| `running` | 继续查。间隔约 5 秒，把当前状态告诉用户 |
| `succeed` | 终态成功 |
| `failed` / `partly_failed` | 终态失败。用 `status=FAILURE` 翻页把 `tasks[].message` 列出来 |

子任务 `tasks[].status`：`INIT`、`RUNNING`、`SUCCESS`、`IGNORED`、`FAILURE`。
`IGNORED` 表示目标态已经满足（重复启动或无需停止），不是失败。

批次详情没有 `all`。看失败明细时 `limit` ≤ 1000，用 `start` 翻页。

### 刷新进程实例

同步的是整个业务，不是某个表达式范围。

```
Config_SyncCmdbGseStatus {bizId}
Config_CmdbGseStatus {bizId}
```

`status`：`Running` 继续查，`Success` 结束，`Failure` 告知失败。字段还有 `lastSyncTime`。

## 报错 → 处置

| 现象 | 处置 |
|------|------|
| `page.limit value should >= 1` | 列表改 `all:true`，或显式 `limit`（1–1000） |
| 批次 `failed` / `partly_failed` | 拉 `FAILURE` 子任务的 `message`，不要重试同一范围除非用户要求 |
| 子任务 `IGNORED`，或错误码 828 / 829 | 幂等成功：已经在跑，或已经停着 |
| 生成尚未成功就想下发 | 继续轮询生成批次；不要拿未完成的 batchId 去 push |
| 工具不存在 | 说明 MCP 未挂载，不要改叫 `Config_Inner_*` |

## 场景

### 正式环境重启某个进程别名

用户说「业务 100，正式环境，集群 game、模块 lobby 上的 gamesvr 重启」：

```
Config_ListProcess {bizId:100, all:true, search:{
  environment:"3",
  expressionScope:{setName:"game", moduleName:"lobby", serviceName:"*", processAlias:"gamesvr", processId:"*"}
}}
# 列出命中进程并确认。确认信息写明「正式环境」
Config_OperateProcess {bizId:100} body:{
  operateType:"restart",
  operateRange:{environment:"3", expressionScope:{setName:"game", moduleName:"lobby", serviceName:"*", processAlias:"gamesvr", processId:"*"}}
} → batchID
Config_GetTaskBatchDetail {bizId:100, batchId:batchID} body:{start:0, limit:100, ignoreErrorCodes:[828,829]}
```

### 下发全部模板到指定模块

```
Config_GenerateConfig {bizId} body:{operateRange:{environment:"1", expressionScope:{setName:"*", moduleName:"lobby", serviceName:"*", processAlias:"*", processId:"*"}}}
# 不传 configTemplateIds = 全部模板最新版
# 轮询 batchId 到 succeed
Config_PushConfig {bizId} body:{batchId:<上一步的 batchId>}
# 轮询新 batchId
```

### 只生成、不下发，并限定模板

```
Config_ListConfigTemplate {bizId, all:true} → 按 spec.templateName 匹配，取 id
Config_GenerateConfig body:{operateRange:{environment, expressionScope, configTemplateIds:[id]}}
# succeed 后停止，不要 PushConfig
```

## 和 KV / 文件型的差别

| | 进程配置（本 skill） | KV | 文件型 |
|--|----------------------|----|--------|
| 范围 | 业务 + CMDB 环境类型 1/2/3 + 五段表达式 | 项目 / 环境 / 服务 | 同 KV |
| 生效 | 异步批次，下发要先生成再 push | 草稿 → 版本 → 发布 | 本 skill 只读 |
| MCP | `bk-bscp-prod-process-config` | `bk-bscp-prod-kv-config` | `bk-bscp-prod-file-config` |

## 说明

领域约束会随 bscp 版本变化。以工具返回和报错为准。
