# 操作范围记录优化

## 基本信息

| 字段 | 值 |
|------|-----|
| 需求 ID | 1020451610136178657（短 ID：136178657） |
| 需求名称 | 操作范围记录优化 |
| 优先级 | High |
| 父需求 | 1020451610135732990（进程配置管理插件优化） |
| 创建时间 | 2026-07-15 17:36:50 |
| 原始需求文档 | docs/reqs/操作范围表达式记录.md |
| 预估工时 | 24 人时（3 人天） |
| 价值规模 | 53（RICE） |

## 需求背景

### 业务背景

在 bscp（蓝鲸基础配置平台）「进程配置管理」场景下，进程操作与配置操作会以任务批次
（`task_batch`）的形式记录一次操作的「操作范围」。页面在任务详情中展示该操作范围，
并支持点击「操作范围」跳转到进程列表页，按范围过滤出本次操作实际作用到的进程，便于
用户回溯"这次操作影响了哪些进程"。

当前问题：从进程配置管理插件（`bscp-proc-cfg`）发起的请求，其入参已由旧的**单值等值**
字段变更为 **gsekit 风格的五段表达式范围**（`OperateRange.expression_scope`：集群名 /
模块名 / 服务实例名 / 进程别名 / CC 进程 ID 五段表达式 + 环境类型）。但服务端记录操作范围时仍读取
旧的单值字段，这些字段在新入参下恒为空，导致：

- 记录进 `task_batch` 的操作范围为空；
- 前端展示时各段均回退为 `*`，操作范围恒显示为 `*.*.*.*.*`；
- 点击「操作范围」跳转后无法带上真实过滤条件，无法还原本次操作实际命中的进程。

参照 gsekit（`bk-process-config-manager/apps/gsekit/process/handlers/process.py`）：操作范围
以「表达式范围」形式记录与还原，从而支持按表达式过滤出命中进程。bscp 需对齐这一做法，
把插件真实传入的表达式范围完整记录下来，恢复"操作范围可读、可点击回溯进程"的能力。

### 用户故事

作为使用进程配置管理插件发起进程/配置操作的用户
我想要任务详情里的「操作范围」如实反映我这次操作圈定的表达式范围
以便于确认本次操作实际作用的范围，而不是永远看到 `*`。

作为在 bscp 页面回溯任务的用户
我想要点击任务详情的「操作范围」直接跳转到进程列表页并按该范围过滤出命中的进程
以便于快速核对"这次操作影响了哪些进程"。

### 需求来源

- **需求渠道**：缺陷修复 / 进程配置管理插件优化配套
- **关联需求**：
  - 父需求 1020451610135732990（进程配置管理插件优化）
  - 关联需求 1020451610135740005（支持通配符扩展的表达式语法过滤进程，文档见
    `docs/reqs/进程表达式过滤.md`）——本需求依赖其在进程列表页/服务端提供的表达式过滤能力
- **参考资料**：
  - gsekit 表达式范围记录与还原：`bk-process-config-manager/apps/gsekit/process/handlers/process.py`
    （`scope_to_expression_scope` / `expression_scope_to_scope`）
  - bscp 现状：
    - 插件入参构建：`bscp-proc-cfg/versions/v100/plugin.go`（`buildOperateRange` / `buildExpressionScope`）
    - 服务端记录：`cmd/data-service/service/process.go`（`buildOperateRange` / `createTaskBatch`）
    - 表结构：`pkg/dal/table/task_batch.go`（`OperateRange` / `TaskExecutionData`）
    - 协议：`pkg/protocol/core/process/process.proto`（`OperateRange` / `ExpressionScope`）、
      `pkg/protocol/core/task_batch/task_batch.proto`
    - 前端展示与跳转：`ui/src/views/space/task/detail/info.vue`（`mergeOpRange` / `handleGoProcess`）

## 功能需求

### 核心功能点

| 功能编号 | 功能描述 | 优先级 | 涉及角色 | 备注 |
|---------|---------|--------|---------|------|
| F-001 | 服务端记录操作范围时，如实采集插件传入的五段表达式范围 + 环境类型（对齐 gsekit expression_scope），不再依赖已废弃的单值字段 | P0 | 插件调用方/页面用户 | 必须 |
| F-002 | 记录范围覆盖进程操作与配置操作两类插件任务（凡携带 ExpressionScope 的任务批次） | P0 | 插件调用方 | 必须 |
| F-003 | 前端任务详情按表达式范围展示操作范围（缺省段显示 `*`，保持 gsekit 风格可读串） | P0 | 页面用户 | 必须 |
| F-004 | 点击「操作范围」跳转进程列表页，并用记录的表达式范围过滤出命中进程 | P0 | 页面用户 | 必须 |
| F-005 | 操作范围统一表达式字符串：存储/协议/前端把 OperateRange 五段由数组重构为表达式字符串，并用一次性幂等数据迁移把存量旧数组刷成等价表达式，消除应用层新旧双分支 | P1 | 页面用户 | 应该有 |

### 详细功能描述

#### [F-001] 服务端如实记录表达式范围

- **输入**：插件请求携带的 `OperateRange.expression_scope`（集群名 / 模块名 / 服务实例名 /
  进程别名 / CC 进程 ID 五段表达式）与环境类型（`environment`）。
- **处理逻辑**：
  1. 记录进 `task_batch` 的操作范围以「表达式范围」形式保存五段表达式字符串 + 环境类型；
  2. 每段缺省为 `*`（匹配任意），与 gsekit `expression_scope` 语义一致；
  3. 不再从已废弃的单值字段（`set_name`/`module_name`/… ）构建操作范围。
- **输出**：任务批次中记录到与本次插件请求一致的表达式范围。
- **边界条件**：某段为空 → 记录为 `*`；环境类型在表达式路径必填。
- **异常处理**：表达式合法性沿用关联需求 135740005 的解析/校验语义，本需求不重复定义。

#### [F-002] 记录范围覆盖进程与配置两类任务

- **输入**：进程操作类任务（start/stop/restart/reload/kill/register/unregister）与配置操作类
  任务（generate/release/diffcfg）——两类均通过 `ExpressionScope` 传入范围。
- **处理逻辑**：两类任务在创建任务批次时都按 F-001 记录真实表达式范围。
- **输出**：两类任务的操作范围记录均可读、可回溯。
- **边界条件**：同步类任务（sync_status）不涉及操作范围，保持现状。

#### [F-003] 前端按表达式展示操作范围

- **输入**：任务详情返回的表达式范围（五段 + 环境）。
- **处理逻辑**：以 gsekit 风格拼接为可读表达式串展示，缺省段显示 `*`。
- **输出**：操作范围文本，如实反映五段表达式而非恒为 `*.*.*.*.*`。
- **边界条件**：超长表达式的展示截断/悬浮提示沿用现有交互组件。

#### [F-004] 点击操作范围按表达式过滤跳转

- **输入**：用户在任务详情点击「操作范围」。
- **处理逻辑**：跳转进程列表页（`process-management`），并携带该任务记录的表达式范围
  作为过滤条件，由进程列表页按表达式过滤（复用关联需求 135740005 的表达式过滤能力），
  展示表达式命中的全部进程。
- **输出**：进程列表页展示本次操作范围命中的进程。
- **边界条件**：
  - "当前正在使用的进程"= 表达式命中的**全部进程**，不区分运行/托管状态；
  - 命中为空 → 展示空列表（不回退为全选）。

#### [F-005] 操作范围统一表达式字符串（含存量数据迁移）

- **输入**：历史任务批次（旧数组结构记录）与新任务批次（表达式字符串记录）。
- **处理逻辑**：把 `OperateRange` 五段在存储（`table`）、协议（`task_batch.proto`）、前端类型统一重构为
  表达式**字符串**（缺省 `*`）；对存量 `task_batches.task_data` 提供一次性**幂等**数据迁移，把旧数组无损
  转换为等价表达式字符串（`[a,b]`、连续数字 `[6-8]`、空段 `*`）；应用层（convert/common/前端）仅按表达式
  单一路径处理，不维护新旧双分支（与 gsekit 只用 `expression_scope` 记录一致）。
- **输出**：新旧任务详情均按表达式统一展示，可点击按表达式过滤跳转。
- **边界条件**：旧数组全空时五段均为 `*`；旧数组为离散枚举，数组→表达式段为无损前向转换；迁移可重复运行
  （幂等），`Down` 为空操作（表达式→数组不可逆）。

## 非功能需求

### 性能需求

- 操作范围的记录/展示为轻量字段读写，不引入额外性能敏感路径；不设专门性能指标。

### 安全需求

- **权限控制**：沿用 bscp 现有进程配置管理鉴权与业务隔离（`biz_id` 维度），本需求不改变权限模型。
- **数据保护**：表达式范围为业务拓扑名称/ID，不涉及敏感数据；记录时沿用现有存储与序列化方式。

### 兼容性

- **数据兼容**：`OperateRange` 五段由数组重构为表达式字符串（JSON 键单数化、值字符串化）；存量
  `task_batch` 记录由一次性幂等数据迁移刷成等价表达式字符串。迁移随发布执行；迁移前若新代码读到旧
  数组 JSON，五段回退 `*` 展示 `*.*.*.*.*`，不报错、不阻断。
- **语义兼容**：表达式范围记录/拼接语义以 gsekit `expression_scope` / `parse_list2expr` 为基准，与关联
  需求 135740005 对齐。

## 业务规则

### 业务逻辑规则

- **规则 R-001**：操作范围以「表达式范围」记录（五段表达式 + 环境类型），缺省段为 `*`。
- **规则 R-002**：记录内容以插件请求的表达式范围为准，不使用已废弃的单值字段。
- **规则 R-003**：点击操作范围跳转后的过滤，按记录的表达式范围过滤，命中"全部进程"（不区分运行状态）。
- **规则 R-004**：进程操作与配置操作两类插件任务均需正确记录操作范围。

### 数据校验规则

- **必填字段**：环境类型（`environment`）在表达式路径必填。
- **格式要求**：五段均为表达式字符串，缺省 `*`。

## 外部依赖与集成

### 外部系统集成

| 系统名称 | 交互方式 | 接口说明 | 认证方式 | 文档链接 |
|---------|---------|---------|---------|---------|
| 进程配置管理插件（bscp-proc-cfg） | HTTP/RPC | 以 `OperateRange.expression_scope` 五段表达式 + 环境发起进程/配置操作 | 沿用现有 | 见父需求 |

### 数据模型

- 操作范围记录位于任务批次的执行数据中（`TaskExecutionData.OperateRange`）：五段由数组重构为表达式
  字符串（`SetName/ModuleName/ServiceName/ProcessAlias/ProcessID`，缺省 `*`），环境类型复用
  `TaskExecutionData.Environment`。以 JSON blob 存于 `task_batches.task_data`，无 DDL 变更（见 TD-004/TD-005）。

## 验收标准

### 功能验收

- [ ] **AC-001**：Given 插件以五段表达式范围（如集群名=`[管控平台, PaaS平台]`、进程 ID=`4[6, 8, 9]`）
  发起进程操作，When 生成任务批次，Then 任务详情记录的操作范围与请求的五段表达式一致，
  不再恒为 `*.*.*.*.*`。
- [ ] **AC-002**：Given 插件发起配置操作（generate/release/diffcfg）并携带表达式范围，
  When 生成任务批次，Then 操作范围同样被如实记录。
- [ ] **AC-003**：Given 任务详情展示操作范围，When 某段表达式缺省，Then 该段显示 `*`；
  When 各段有值，Then 展示对应的 gsekit 风格表达式串。
- [ ] **AC-004**：Given 用户点击任务详情的「操作范围」，When 跳转到进程列表页，
  Then 进程列表按记录的表达式范围过滤，展示表达式命中的全部进程（不区分运行状态）；
  命中为空时展示空列表。
- [ ] **AC-005**：Given 存在历史旧格式任务批次记录，When 展示其任务详情操作范围，
  Then 仍能正常展示，新旧格式互不影响。

## 边界范围

### 本期包含

- 存储/协议/前端把 `OperateRange` 五段重构为表达式字符串（缺省 `*`）。
- 服务端如实记录插件传入的表达式范围（五段 + 环境），覆盖进程操作与配置操作两类任务。
- 非插件/页面路径按命中进程 CC 进程 ID 拼压缩表达式记录（对齐 gsekit 页面路径）。
- `internal/expression` 新增 `List2Expr`（含数字范围压缩），供非插件记录与迁移复用。
- 存量 `task_batch` 记录旧数组 → 表达式字符串的一次性幂等数据迁移。
- 前端按表达式展示操作范围，并支持点击跳转进程列表页按表达式过滤命中进程。

### 本期不包含

- 表达式解析/匹配内核本身（由关联需求 135740005 提供）。
- 进程列表页表达式过滤能力的新增实现（复用 135740005 的成果）。
- 请求侧协议 `OperateRange.expression_scope`/`ExpressionScope`/`environment` 的改动（已就绪）。

## 约束条件

- **一致性基准**：操作范围记录与还原语义以 gsekit `expression_scope` 为基准。
- **术语约束**：`expression_scope`、`OperateRange`、`cc_process_id`、`environment` 等术语保持原样，不翻译改名。

## 未解决问题

| 问题 ID | 问题描述 | 状态 |
|--------|---------|------|
| Q-001 | 表达式范围在任务批次记录中的具体承载形态，及新旧兼容的具体结构 | ✅ 已澄清（TD-001/TD-004：`OperateRange` 五段由数组重构为表达式字符串，缺省 `*`；TD-005：插件原样存、非插件与存量迁移用 `List2Expr` 拼表达式，一次性幂等迁移刷存量数据） |

---

## 技术澄清

> 澄清日期：2026-07-22
> 需求复杂度：中等
> 澄清轮次：1

### 技术方案概述

- **实现方式**：请求侧协议（`OperateRange.expression_scope` + `environment`）已由关联需求 135740005
  就绪。本需求把操作范围**彻底重构为表达式字符串**并打通「记录 → 展示 → 跳转过滤」三段：
  1. **重构**：`table.OperateRange`/`task_batch.proto`/前端类型五段由数组改为表达式字符串（缺省 `*`）；
  2. **记录**：两条建批次链路——插件路径原样存请求 `expression_scope`+`environment`，非插件路径用
     `internal/expression.IDsToExpr` 把命中进程 CC 进程 ID 拼进 `process_id`、其余段 `*`；
  3. **消费**：`convert.go` 五段字符串透传，`common.go buildScopeText` 用 `GenExpression` 拼展示串；
  4. **迁移**：一次性幂等数据迁移把存量旧数组刷成表达式字符串；
  5. **展示/跳转**：前端 `mergeOpRange` 单一路径展示；点击操作范围跳转进程列表页切表达式模式，复用
     `ProcessSearchCondition.expression_scope` 过滤能力（135740005 已提供）。
- **涉及模块**：
  - 能力：`internal/expression`（新增 `List2Expr`/`IDsToExpr`，对齐 gsekit `parse_list2expr`）。
  - 表结构：`pkg/dal/table/task_batch.go`（`OperateRange` 五段数组→字符串）。
  - 协议：`pkg/protocol/core/task_batch/task_batch.proto`（`OperateRange` 五段 `repeated`→`string`、键名对齐）；
    请求侧 `process.proto` 已含 `OperateRange.expression_scope`/`ExpressionScope`，无需改。
  - 互转/通知：`pkg/protocol/core/task_batch/convert.go`（五段字符串透传）、
    `internal/task/executor/common/common.go`（`buildScopeText` 改读五段字符串）。
  - 服务端：`cmd/data-service/service/process.go`、`cmd/data-service/service/config_instance.go`
    （两处 `buildOperateRange` 改造）。`cmd/config-server/service/process.go` 仅透传，无需改造。
  - 迁移：`cmd/data-service/db-migration/migrations/`（新增 `task_data` 数组→表达式迁移）。
  - 前端：`ui/types/task.ts`、`ui/src/store/task.ts`、`ui/src/views/space/task/detail/info.vue`、
    `ui/src/views/space/process/components/filter-process.vue`。
- **技术选型**：无新引入依赖；表达式生成/匹配语义沿用 `internal/expression`（135740005 成果），仅同包补
  `List2Expr`；记录/展示承载五段字符串，不重复实现解析内核。

### 架构影响

- **新增组件**：无独立新组件。
- **变更组件**：
  - `table.OperateRange` 五段由数组重构为表达式字符串（环境复用 `TaskExecutionData.Environment`）。
  - `task_batch.proto` `OperateRange` 五段由 `repeated` 改单字符串、键名对齐 `set_name/…/process_id`。
  - `convert.go` 五段字符串透传；`common.go buildScopeText` 改读五段字符串 + `GenExpression`。
  - 两处 `buildOperateRange`：插件路径原样记录请求表达式；非插件路径用 `IDsToExpr` 拼命中进程表达式。
  - `internal/expression` 新增 `List2Expr`/`IDsToExpr`；新增数据迁移刷存量。
- **数据模型变更**：任务批次执行数据以 JSON blob（`task_batches.task_data`）存储，**无 DDL 变更**；
  `OperateRange` JSON 键单数化、值字符串化，存量由数据迁移刷新。
- **向后兼容性**：操作范围统一为表达式**字符串**。存量记录由一次性**幂等**数据迁移把旧数组无损转换为
  等价表达式字符串（`[a,b]`、连续数字 `[6-8]`、空段 `*`）；迁移随发布执行，迁移前新代码读旧数组 JSON 则
  五段回退 `*`，不报错。应用层（convert/common/前端）仅按表达式单一路径处理，不维护双分支
  （对应 F-005 / AC-005 / AC-T02；与 gsekit 只用 `expression_scope` 记录一致，见 TD-004/TD-005）。

### 外部依赖

| 依赖项 | 类型 | 状态 | 接口文档 | 备注 |
|--------|------|------|---------|------|
| 关联需求 135740005 表达式过滤能力 | 内部能力 | ✅ 已确认 | `docs/reqs/进程表达式过滤.md`、`internal/expression/scope.go` | 进程列表页/服务端 `expression_scope` 过滤复用其成果，本需求不重复实现 |
| 进程配置管理插件（bscp-proc-cfg） | HTTP/RPC | ✅ 已确认 | 见父需求 | 已按 `OperateRange.expression_scope` 五段表达式 + `environment` 发起请求 |

### 安全与合规

- **权限控制**：沿用现有进程配置管理鉴权（`config-server` `OperateProcess` 已校验 `ProcConfigMgmt/ProcessOperate`）
  与 `biz_id` 业务隔离，本需求不改权限模型。
- **审计要求**：无新增审计操作；任务批次本身即操作记录。
- **加密要求**：表达式范围为业务拓扑名称/ID，非敏感数据，沿用现有存储与序列化方式。
- **输入校验**：表达式合法性/空集/异常语义沿用 135740005 的解析校验（`GetByOperateRange` 已将非法入参
  归类为 `InvalidParameter`），本需求不重复定义校验规则。

### 测试策略

- **单元测试**：
  - 服务端 `buildOperateRange`（进程 + 配置两条链路）：给定携带 `expression_scope` 的请求，
    断言记录的表达式范围与请求一致，且不再读单值字段（覆盖 AC-001 / AC-002）。
  - `internal/expression.List2Expr`/`IDsToExpr`：空/单值/多值/连续数字压缩，对齐 gsekit（覆盖 TD-005）。
  - `convert.go` `PbTaskBatch`：五段字符串透传正确（覆盖 AC-003 展示数据）。
  - 数据迁移：旧数组样本 → 表达式字符串无损、重复运行幂等（覆盖 AC-005/AC-T02）。
- **集成测试**：任务批次创建 → 查询任务详情，验证操作范围如实返回（可用单包测试验证，不必全量编译）。
- **前端**：`mergeOpRange` 五段字符串展示、`filter-process` 跳转切表达式模式过滤的逻辑验证（覆盖 AC-003 / AC-004）。
- **测试数据**：参考 AC-001 示例（集群名=`[管控平台, PaaS平台]`、进程 ID=`4[6, 8, 9]`）构造表达式范围入参。

### 技术决策记录

| 决策 | 选择方案 | 备选方案 | 选择理由 |
|------|---------|---------|---------|
| TD-001：表达式范围在任务批次记录中的承载形态（回答 Q-001） | **把 `table.OperateRange` 五段由数组重构为表达式字符串**（`SetName/…/ProcessID`，缺省 `*`），环境复用 `Environment`；协议/前端同步字符串化 | ①新增表达式字段与旧数组并存 + 转换层派生；②新增 schema/版本区分 | 现有数组无法承载 `4[6,8,9]`/`[1-1000]` 等表达式与切片语义；与其新增并存字段并在 convert 层长期维护派生分支，不如彻底重构为字符串并一次性迁移存量（TD-005），应用层单一路径、更可追溯；JSON blob 无需 DDL |
| TD-002：表达式范围的规范载体 | 记录/展示用协议 `OperateRange`（task_batch）五段字符串消息 | 直接用 `internal/expression.Scope` 结构 | `Scope` 是内存匹配结构（含匹配算法所需字段），不宜直接作为持久化/协议字段；五段字符串与 `Scope` 一一对应可互转，职责更清晰 |
| TD-003：配置操作链路记录方式 | 插件模式下同样以请求 `expression_scope` 原样记录，与进程操作路径统一；非插件模式按命中进程 CC 进程 ID 拼表达式 | 保留现状（按命中进程真实属性去重物化数组） | 现状 `config_instance.go` 的 `buildOperateRange` 物化数组，丢失用户表达式与切片语义、且与进程路径不一致；统一按表达式记录才满足 F-002 / AC-002 |
| TD-004：新旧记录兼容方式（用户澄清后收敛为彻底重构） | **把 OperateRange 彻底重构为表达式字符串 + 一次性数据迁移刷存量**；应用层（convert/common/前端）单一表达式路径 | ①前端「优先表达式、回退旧数组」双分支；②后端 convert 层长期派生兼容 | gsekit 存储即为单字符串 `expression_scope`；旧数组均为离散枚举、数组→表达式无损。用迁移把兼容成本收敛到发布期一处，长期无双分支、无冗余数组，最贴合 gsekit 且最可维护（对应 F-005 / AC-005 / AC-T02） |
| TD-005：数组→表达式拼接与记录策略（用户澄清 gsekit 存储后新增；评审 Review-01 收敛压缩范围） | 对齐 gsekit：**插件/表达式路径原样存**请求 `expression_scope`（不解析）；**非插件/页面路径与存量迁移**用 `internal/expression.List2Expr`/`IDsToExpr` 由数组拼表达式（单个→原值、多个→`[..]`、空→`*`）。**数字区间压缩 `[6-8]` 仅施于进程 ID（`IDsToExpr`）**；名称段（`List2Expr`）为字面量枚举、不压数字、保留原值 | ①非插件路径统一记 `*.*.*.*.*`（不拼进程 id）；②多值不做数字范围压缩；③名称段也做数字压缩（对齐 gsekit） | gsekit `create_job`：API 路径原样存 `expression_scope`、页面路径 `scope_to_expression_scope`→`parse_list2expr` 派生；bscp 对齐可让页面操作也能按表达式精确回溯。进程 ID 恒为规范整数、压缩安全无损；名称段是离散标识符，若把 `01`/`02` 当数字压成 `[1-2]` 前导零丢失后无法匹配回原始 CMDB 名称，故名称段保留字面量枚举（比区间更精确、对匹配无损）。拼接统一由 `internal/expression` 一处实现，非插件记录与迁移复用（对应 FR-007 / FR-008 / AC-T03） |

### 补充的验收标准

- [ ] **AC-T01**：Given 插件请求携带 `OperateRange.expression_scope` 五段表达式且单值字段为空，
  When 服务端 `buildOperateRange` 构建操作范围，Then 记录的表达式范围等于请求 `expression_scope`，
  且不读取任何单值字段。
- [ ] **AC-T02**：Given 一批仅含旧数组结构的历史 `task_data` JSON，When 运行数据迁移 `Up`，
  Then 每条 `operate_range` 被重写为五段表达式字符串（`["a","b"]`→`[a,b]`、连续数字→`[6-8]`、空→`*`），
  且**重复运行迁移不改变已迁移记录**（幂等）。
- [ ] **AC-T03**：Given 页面/非插件路径对 N 个进程发起操作，When 生成任务批次，
  Then `OperateRange.process_id` 为命中进程 CC 进程 ID 的压缩表达式（如 `[6-8]`），其余四段为 `*`。

### 待解决问题

| 问题 ID | 问题描述 | 负责人 | 截止日期 | 状态 |
|---------|---------|--------|---------|------|
| —（无阻塞项） | 中等复杂度 DoR 三项（技术方案明确 / 外部依赖已识别 / 测试策略已定义）均已通过白名单文档自答满足 | — | — | ✅ 已满足 |

---

## 原需求描述

> TAPD 原始需求描述为空（无描述内容）。以下为需求提出人（用户）在会话中提供的口头背景，完整保留：
>
> 插件侧请求会记录更完整的操作范围。然后在页面点击操作范围可以跳转到进程列表页面通过
> 表达式过滤出当前正在使用的进程。但是当前从插件侧的请求记录的操作范围固定都是 *。
> 原因是插件使用的入参变更了。记录的操作范围需要参考 gsekit。
> （参考文件：bk-process-config-manager/apps/gsekit/process/handlers/process.py:1-1267）
