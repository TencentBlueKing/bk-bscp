# Feature Specification: 操作范围记录优化

**需求 ID**: 136178657（TAPD: 1020451610136178657）
**Feature Directory**: `specs/stories/136178657`
**创建时间**: 2026-07-22
**状态**: Draft
**来源**: `specs/stories/136178657/req.md`；澄清结论见 `req.md` 技术澄清章节（TD-001~TD-005）

## 概述

bscp「进程配置管理」在任务批次（`task_batch`）里记录一次操作的「操作范围」，任务详情展示该范围
并支持点击跳转进程列表页、按范围过滤出本次操作命中的进程，用于回溯"这次操作影响了哪些进程"。

当前缺陷：进程配置管理插件（`bscp-proc-cfg`）发起请求时，入参已由旧的**单值等值**字段改为
gsekit 风格的**五段表达式范围**（`OperateRange.expression_scope`：集群名 / 模块名 / 服务实例名 /
进程别名 / CC 进程 ID 五段表达式 + 环境类型 `environment`）。但服务端记录时仍读取旧单值字段，
新入参下恒为空，导致操作范围记录为空、前端恒显 `*.*.*.*.*`、点击跳转无法带上真实过滤条件。

本需求对齐 gsekit `expression_scope` 的存储/展示做法，把操作范围**彻底重构为表达式字符串结构**：
存储层 `OperateRange` 五段由数组改为单个表达式字符串（缺省 `*`），协议/转换/通知/前端全链路统一按
表达式处理；并通过一次性**数据迁移**把存量旧数组记录刷成等价表达式字符串，消除应用层新旧双分支。
打通「记录 → 展示 → 跳转过滤」三段，恢复"操作范围可读、可点击回溯进程"的能力，覆盖进程操作与
配置操作两类任务。

> **范围决策**：表达式解析/匹配内核、进程列表页的表达式过滤能力由关联需求 135740005 提供，本需求
> 不重复实现；「数组 → 表达式字符串」的拼接（`List2Expr`，含数字范围压缩）对齐 gsekit `parse_list2expr`，
> 作为独立能力补充到 `internal/expression`，供**非插件路径记录**与**存量迁移**复用。

## User Scenarios & Testing

### 主要用户故事

作为使用进程配置管理插件发起进程/配置操作的用户，我希望任务详情里的「操作范围」如实反映我这次
操作圈定的表达式范围，以便确认本次操作实际作用的范围，而不是永远看到 `*`。

作为在 bscp 页面回溯任务的用户，我希望点击任务详情的「操作范围」直接跳转到进程列表页并按该范围
过滤出命中的进程，以便快速核对"这次操作影响了哪些进程"。

### 验收场景（Acceptance Scenarios）

1. **AC-001（进程操作如实记录表达式范围）**：
   **Given** 插件以五段表达式范围（如集群名=`[管控平台, PaaS平台]`、进程 ID=`4[6, 8, 9]`）发起进程操作，
   **When** 服务端生成任务批次，
   **Then** 任务批次记录的 `OperateRange` 五段字符串与请求的 `expression_scope` 一致（原样存），
   不再恒为 `*.*.*.*.*`。

2. **AC-002（配置操作如实记录表达式范围）**：
   **Given** 插件发起配置操作（generate/check）并携带表达式范围，
   **When** 服务端生成任务批次，
   **Then** 操作范围同样按请求 `expression_scope` 原样记录（与进程操作路径一致）。

3. **AC-003（缺省段显示 `*`，有值段显示表达式）**：
   **Given** 任务详情展示操作范围，
   **When** 某段表达式为空，
   **Then** 该段显示 `*`；各段有值时展示对应的 gsekit 风格表达式串。

4. **AC-004（点击操作范围按表达式过滤跳转）**：
   **Given** 用户点击任务详情的「操作范围」，
   **When** 跳转到进程列表页（`process-management`），
   **Then** 进程列表页切到表达式模式并按记录的五段表达式过滤，展示表达式命中的**全部进程**
   （不区分运行/托管状态）；命中为空时展示空列表（不回退为全选）。

5. **AC-005（存量历史记录经迁移后统一按表达式展示）**：
   **Given** 存在历史旧格式（数组结构）任务批次记录，
   **When** 执行本需求的数据迁移后展示任务详情操作范围，
   **Then** 该记录已被无损转换为等价表达式字符串（数组 `["a","b"]` → `[a,b]`，连续数字 `[6,7,8]` → `[6-8]`，
   空段 → `*`），与新表达式记录同走一条展示/跳转路径、互不报错。

6. **AC-T01（服务端只读表达式、不读单值字段）**：
   **Given** 插件请求携带 `OperateRange.expression_scope` 五段表达式且单值字段为空，
   **When** 服务端构建操作范围（`buildOperateRange`），
   **Then** 记录的 `OperateRange` 五段字符串等于请求 `expression_scope`，且不读取任何单值字段。

7. **AC-T02（存量迁移无损且幂等）**：
   **Given** 一批仅含旧数组结构（`set_names`/`module_names`/…/`cc_process_ids`）的历史 `task_data` JSON，
   **When** 运行数据迁移 `Up`，
   **Then** 每条记录的 `operate_range` 被重写为五段表达式字符串（`["a","b"]`→`[a,b]`、连续数字→`[6-8]`、
   空→`*`）；**重复运行迁移不改变已迁移记录**（幂等）。

8. **AC-T03（非插件路径按命中进程拼表达式记录）**：
   **Given** 页面/非插件路径对 N 个进程发起进程或配置操作，
   **When** 服务端生成任务批次，
   **Then** `OperateRange.process_id` 记录为命中进程 CC 进程 ID 的压缩表达式（如 `[6-8]`、`6`），
   其余四段为 `*`（对齐 gsekit 页面路径 `scope_to_expression_scope`）。

### 边界与异常情况（Edge Cases）

- **某段为空**：某段表达式为空时记录为 `*`（匹配任意），与 gsekit `expression_scope` 语义一致。
- **环境类型必填**：表达式路径下环境类型（`environment`）为必填，复用 `TaskExecutionData.Environment`。
- **同步类任务**：同步类任务（sync_status）不涉及操作范围，保持现状。
- **表达式合法性/空集**：解析与校验语义沿用关联需求 135740005（非法入参归类为 `InvalidParameter`），
  本需求不重复定义校验规则。
- **超长表达式展示**：截断/悬浮提示沿用现有交互组件。
- **未迁移即读取**：迁移随发布执行；迁移前若新代码读到旧数组 JSON（键不匹配），五段回退为 `*`，
  展示 `*.*.*.*.*`，不报错、不阻断——迁移完成后即恢复真实表达式。
- **迁移不可逆**：数组 → 表达式为无损前向转换，但表达式 → 数组不可逆（范围/切片无法还原），迁移
  `Down` 为空操作并注明不可逆。

## Requirements

### 功能需求（Functional Requirements）

- **FR-001**：服务端记录操作范围时，系统 MUST 如实采集插件请求携带的五段表达式范围
  （`OperateRange.expression_scope`）+ 环境类型（`environment`），以**五段表达式字符串**形式保存到
  `TaskExecutionData.OperateRange`；每段缺省保存为 `*`。（对应 req.md F-001 / R-001）

- **FR-002**：系统 MUST NOT 再从已废弃的单值字段（`set_name`/`module_name`/… ）构建操作范围。
  （对应 req.md R-002 / AC-T01）

- **FR-003**：记录范围 MUST 覆盖进程操作类任务（start/stop/restart/reload/kill/register/unregister）
  与配置操作类任务（generate/check）——插件路径均按 FR-001 原样记录请求表达式范围。
  （对应 req.md F-002 / R-004）

- **FR-004**：前端任务详情 MUST 按 `OperateRange` 五段表达式字符串以 gsekit 风格拼接为可读表达式串展示，
  缺省段显示 `*`，不再恒显 `*.*.*.*.*`。（对应 req.md F-003）

- **FR-005**：用户点击「操作范围」时，系统 MUST 跳转进程列表页并携带该任务记录的五段表达式作为过滤
  条件，进程列表页 MUST 切到表达式模式按表达式过滤（复用 135740005 的 `ProcessSearchCondition.expression_scope`
  能力），展示命中的**全部进程**（不区分运行/托管状态）；命中为空展示空列表，不回退为全选。
  （对应 req.md F-004 / R-003）

- **FR-006**：系统 MUST 将「操作范围」在存储层、协议层、前端类型统一为**表达式字符串结构**
  （`OperateRange` 五段由数组改为 `string`；`task_batch.proto` `OperateRange` 由 `repeated` 改为
  单字符串、键名对齐 `set_name/module_name/service_name/process_alias/process_id`）。系统 MUST NOT
  在应用层维护新旧双分支。（对应 req.md F-005 / TD-004）

- **FR-007**：系统 MUST 提供一次性、**幂等**的数据迁移，将存量 `task_batches.task_data` 中旧数组格式的
  `operate_range` 无损转换为五段表达式字符串（数组 → `[a,b]`、连续数字压缩 → `[6-8]`、空 → `*`）。
  迁移 `Down` 为空操作（不可逆，注明）。（对应 req.md F-005 / TD-005 / AC-005 / AC-T02）

- **FR-008**：非插件/页面路径 MUST 把命中进程的 CC 进程 ID 拼成压缩表达式记录到 `OperateRange.process_id`
  （其余段 `*`），对齐 gsekit 页面路径 `scope_to_expression_scope` 的记录方式。（对应 req.md TD-005 / AC-T03）

- **FR-009**：本需求 MUST NOT 重复实现表达式解析/匹配内核（复用 `internal/expression`），MUST NOT 改变
  现有进程配置管理鉴权与 `biz_id` 业务隔离模型，MUST NOT 变更请求侧协议（`process.proto`）。
  （对应 req.md 边界范围 / 安全需求）

### 关键实体（Key Entities）

- **任务批次操作范围（`TaskExecutionData.OperateRange`）**：本需求承载点。现状为数组结构
  （枚举值数组），无法承载 `4[6,8,9]`/`[1-1000]` 等表达式与切片语义；重构为**五段表达式字符串**
  （`SetName/ModuleName/ServiceName/ProcessAlias/ProcessID`，缺省 `*`），与 `internal/expression.Scope`
  一一对应。以 JSON blob 存于 `task_batches.task_data`，**无表结构 DDL 变更**（仅 JSON 值形态变化，
  由数据迁移刷新）。（对应 TD-001/TD-004）

- **表达式范围（五段 + 环境）**：集群名 / 模块名 / 服务实例名 / 进程别名 / CC 进程 ID 五段表达式
  字符串 + 环境类型；语义以 gsekit `expression_scope` 为基准，缺省段为 `*`。记录/展示以协议
  `OperateRange`（task_batch）五段字符串为规范载体，与内存匹配结构 `internal/expression.Scope` 互转
  （不直接持久化匹配结构）。（对应 TD-002）

- **List2Expr（数组 → 表达式）**：`internal/expression` 新增能力，对齐 gsekit `parse_list2expr` +
  `compressed_list`：去重后，空 → `*`、单个 → 原值、多个 → `[..]`（连续数字压缩为 `a-b`）。供
  非插件路径记录与数据迁移复用。（对应 TD-005）

- **进程列表过滤条件（跳转目标）**：进程列表页按表达式范围过滤命中进程，复用关联需求 135740005 提供
  的 `ProcessSearchCondition.expression_scope` 过滤能力。

## Success Criteria

- **SC-001**：插件以五段表达式范围发起进程操作或配置操作后，任务批次记录的 `OperateRange` 五段字符串
  与请求 `expression_scope` 一致，不再恒为 `*.*.*.*.*`（对应 AC-001/AC-002/AC-T01）。
- **SC-002**：任务详情操作范围文本按五段字符串以 gsekit 风格展示，缺省段显示 `*`（对应 AC-003）。
- **SC-003**：点击操作范围跳转进程列表页后，切表达式模式按记录的五段表达式过滤出命中的全部进程
  （不区分运行状态），命中为空时展示空列表（对应 AC-004）。
- **SC-004**：数据迁移把存量旧数组记录无损、幂等地刷成等价表达式字符串，迁移后与新记录同走表达式
  单一路径展示/跳转（对应 AC-005/AC-T02）。
- **SC-005**：非插件路径记录的 `process_id` 段为命中进程的压缩表达式、其余段 `*`（对应 AC-T03）。

## Assumptions（假设与默认）

- `OperateRange` 五段直接由数组重构为 `string`（不新增并存字段）；环境类型复用 `TaskExecutionData.Environment`；
  JSON blob 存储无需 DDL，存量数据形态由数据迁移刷新（TD-004，已澄清）。
- 记录/展示的规范载体为协议 `OperateRange`（task_batch）五段字符串消息，而非内存匹配结构（TD-002，已澄清）。
- 插件路径**原样存**请求 `expression_scope`（不解析）；非插件路径与存量迁移用 `List2Expr` 由数组拼表达式
  （对齐 gsekit：API 路径原样存、页面路径由 scope 派生 expression_scope）（TD-003/TD-005，已澄清）。
- 请求侧协议（`OperateRange.expression_scope` + `environment`）与进程列表页表达式过滤能力已由关联
  需求 135740005 就绪，本需求直接复用，不新增/修改请求契约。
- 旧数组均为离散枚举（无范围/切片），故「数组 → 表达式段」为无损转换；连续数字压缩为范围仅缩短
  表达式，语义等价。

## 范围（Scope）

### 本期包含

- 存储/协议/转换/通知/前端把 `OperateRange` 统一重构为五段表达式字符串（FR-006）。
- 服务端如实记录插件传入的表达式范围（五段 + 环境），覆盖进程操作与配置操作两类任务（FR-001~FR-003）。
- 非插件路径按命中进程 CC 进程 ID 拼表达式记录（FR-008）。
- `internal/expression` 新增 `List2Expr`（含数字范围压缩），对齐 gsekit（TD-005）。
- 存量 `task_batches.task_data` 旧数组 → 表达式字符串的一次性幂等数据迁移（FR-007）。
- 前端按表达式展示操作范围，并支持点击跳转进程列表页按表达式过滤命中进程（FR-004~FR-005）。

### 本期不包含

- 表达式解析/匹配内核本身（由关联需求 135740005 提供）。
- 进程列表页表达式过滤能力的新增实现（复用 135740005 的成果）。
- 请求侧协议 `OperateRange.expression_scope`/`ExpressionScope`/`environment` 的改动（已就绪）。

## 依赖（Dependencies）

- **关联需求 135740005（表达式过滤能力）**：内部能力，已确认。进程列表页/服务端 `expression_scope`
  过滤与表达式解析/校验语义复用其成果（`docs/reqs/进程表达式过滤.md`、`internal/expression/scope.go`），
  本需求不重复实现。
- **进程配置管理插件（`bscp-proc-cfg`）**：HTTP/RPC，已确认。已按 `OperateRange.expression_scope`
  五段表达式 + `environment` 发起进程/配置请求。

## 约束条件（Constraints）

- **一致性基准**：操作范围记录、拼接与还原语义以 gsekit `expression_scope` / `parse_list2expr` 为基准。
- **术语约束**：`expression_scope`、`OperateRange`、`cc_process_id`、`environment` 等术语保持原样，不翻译改名。
- **权限/数据**：沿用现有进程配置管理鉴权与 `biz_id` 业务隔离；表达式范围为业务拓扑名称/ID，非敏感
  数据，沿用现有存储与序列化方式。
