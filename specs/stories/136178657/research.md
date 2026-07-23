# Phase 0 Research：操作范围记录优化（Story 136178657）

**输入**：`specs/stories/136178657/spec.md`、`req.md` 技术澄清章节（TD-001~TD-005）
**结论**：无遗留 `NEEDS CLARIFICATION`。请求侧协议与表达式过滤内核已由关联需求 135740005 就绪，
本需求把操作范围**彻底重构为表达式字符串**并**迁移存量数据**，打通「记录 → 展示 → 跳转过滤」三段，
技术选型无新增依赖。

---

## R-0：gsekit 存储对标（决定重构与迁移形态）

- **发现**（`bk-process-config-manager/apps/gsekit/job/models.py`、`handlers.py`、
  `apps/gsekit/utils/expression_utils/parse.py`）：
  - gsekit `Job` 同时存 `scope`（id 数组，DB 精确筛选）与 `expression_scope`（**每段单个表达式字符串**，
    缺省 `*`）。`expression_scope` 键为 `bk_set_name/bk_module_name/service_instance_name/bk_process_name/
    bk_process_id` + `bk_set_env`。
  - 创建 Job 时（`create_job`）：**API/表达式请求原样存 `expression_scope`**（不解析）、反解析出 `scope`；
    **页面请求存 `scope`（id 数组）并用 `scope_to_expression_scope` → `parse_list2expr` 派生 `expression_scope`**。
  - `parse_list2expr`：去重后，单个 → 原值（不加括号）、多个 → `[..]`（`compressed_list` 把连续数字压成 `6-8`）、
    空 → `*`。
- **Decision**：bscp 只保留表达式表示（不像 gsekit 双存 scope 数组），因为插件请求本就只传 `expression_scope`、
  进程列表过滤已按表达式（135740005）而非 id 数组做 DB 筛选。故把 `table.OperateRange` 五段由数组重构为
  单字符串（缺省 `*`），并把存量数组数据迁移为等价表达式字符串。
- **Rationale**：与 gsekit 存储语义一致、去掉冗余数组、消除应用层新旧双分支；数组 → 表达式为无损前向转换。

## R-1：操作范围承载形态（回答 TD-001 / TD-004）

- **Decision**：`pkg/dal/table/task_batch.go` 的 `OperateRange` 五段字段 `[]string`/`[]uint32` → `string`
  （`SetName/ModuleName/ServiceName/ProcessAlias/ProcessID`，json 键单数化），环境类型复用
  `TaskExecutionData.Environment`。不新增并存字段、不保留旧数组字段。
- **Rationale**：
  - 现有数组无法承载 `4[6,8,9]`、`[1-1000]` 等表达式串与切片语义。
  - `task_data` 为 JSON blob（`TaskExecutionData.String()` 走 `json.Marshal`），**无 DDL 变更**；
    存量旧数组 JSON 的键（`set_names` 等复数）与新键（`set_name` 等单数）不一致，故需数据迁移刷新（见 R-6）。
- **Alternatives considered**：
  - ①新增 `ExpressionScope` 字段与旧数组并存 + 转换层派生：保留冗余数组、convert 层需长期维护派生分支，
    与「统一表达式、改动可追溯」冲突，否决。
  - ②新增 schema 版本号区分：引入不必要兼容层，否决。

## R-2：表达式范围的规范载体（回答 TD-002）

- **Decision**：记录/展示用与协议 `OperateRange`（task_batch）对齐的五段字符串
  （`set_name/module_name/service_name/process_alias/process_id`）作为规范载体；内存匹配用
  `internal/expression.Scope`，二者一一对应，需要匹配时互转，不持久化 `Scope`。
- **Rationale**：`internal/expression.Scope` 含匹配算法所需字段（`Candidate`、切片处理），是内存态结构，
  不宜作为持久化/协议字段；五段字符串职责更清晰。
- **Alternatives considered**：直接持久化 `internal/expression.Scope`——语义混淆、耦合匹配内核，否决。

## R-3：服务端记录链路缺陷定位与修复方向（回答 TD-003 / TD-005 / AC-T01 / AC-T03）

- **Decision**：
  - 进程操作链路 `cmd/data-service/service/process.go` `buildOperateRange`（L461）：
    - 插件路径（`req.OperateRange != nil`）：从 `req.OperateRange.GetExpressionScope()` 取五段字符串
      **原样存**（空段补 `*`），environment 取 `req.OperateRange.GetEnvironment()`；不再读
      `GetSetName()/GetModuleName()/…` 单值字段。
    - 非插件路径：`ProcessID = expression.IDsToExpr(命中进程 CcProcessID)`，其余四段 `*`。
  - 配置操作链路 `cmd/data-service/service/config_instance.go` `runConfigTask`（L1544）：由
    `GenerateConfig`/`CheckConfig` 透传请求 `*pbproc.OperateRange`；插件模式（`pluginMode==true`）
    **原样存**请求 `expression_scope`+`environment`；非插件模式 `ProcessID = IDsToExpr(命中进程 CcProcessID)`、
    其余段 `*`。替换现有 `buildOperateRange`（L1747）按命中进程真实属性物化数组的做法。
  - `cmd/config-server/service/process.go`（L42）仅透传 `OperateRange`，**无需改造**。
- **Rationale**：
  - 现状 `process.go` L471-486 读单值字段，新入参下恒空 → 记录空 → 前端恒显 `*.*.*.*.*`（缺陷根因）。
  - 插件路径原样存请求表达式，对齐 gsekit API 路径；非插件路径拼表达式，对齐 gsekit 页面路径。
- **Alternatives considered**：配置链路保留物化数组现状——违背 FR-002/FR-003/AC-002，否决。

## R-4：展示侧协议与转换（FR-006 / AC-003）

- **Decision**：
  - `pkg/protocol/core/task_batch/task_batch.proto` 的 `OperateRange` 五段由 `repeated string`/`repeated uint32`
    改为 `string`，键名统一为 `set_name/module_name/service_name/process_alias/process_id`；用仓库现有生成命令
    （`cd pkg/protocol && make clean && make`，见根 `Makefile`）重新生成 `task_batch.pb.go`。
  - `pkg/protocol/core/task_batch/convert.go` `PbTaskBatch`：直接透传五段字符串（table→pb 一一映射），
    无派生逻辑。
- **Rationale**：存储已是表达式字符串、存量由迁移刷新，故转换层单纯搬运即可；避免长期维护派生分支。
- **Note**：`task_batch.proto` `OperateRange` 为 bscp 内部展示协议（消费方仅 `convert.go` + 前端），
  键名/类型变更不影响请求侧 `process.proto`。

## R-5：通知链路（common.go buildScopeText）

- **Decision**：`internal/task/executor/common/common.go` `buildScopeText`（L581）改为解析新
  `table.OperateRange`（五段字符串），用 `internal/expression.GenExpression`（或等价 `.` 拼接、空段补 `*`）
  生成 `set.module.service.alias.processid`，替换现有仅读 `CCProcessID` 的逻辑。
- **Rationale**：`buildScopeText` 是 `OperateRange` 的实际消费方（任务通知），随结构重构必须同步；
  复用 `GenExpression` 与展示语义一致。

## R-6：存量数据迁移（FR-007 / AC-005 / AC-T02）

- **Decision**：新增 `cmd/data-service/db-migration/migrations/<ts>_migrate_task_batch_operate_range_to_expression.go`
  （`migrator.GormMode`）。`Up`：分页遍历 `task_batches`，逐行解析 `task_data` JSON；若 `operate_range` 为旧
  数组格式（含 `set_names` 等复数键或数组值），用 `List2Expr`/`IDsToExpr` 把各段转字符串并重写 `task_data`；
  已是字符串格式则跳过（**幂等**）。`Down` 为空操作（表达式 → 数组不可逆，注明）。
- **Rationale**：json 键由复数改单数、数组改字符串，未迁移记录新代码读不到旧键 → 五段回退 `*`；迁移随发布
  执行把存量刷成真实表达式，彻底消除应用层兼容分支。参考同表迁移先例
  `20260311000000_fix_task_batch_start_at.go` 与 `migrator` 框架（`AddMigration`/`GormMode`/`Up`/`Down`）。
- **Alternatives considered**：不迁移、靠 convert 层长期派生——保留冗余数组与派生分支，与 Scheme C 目标冲突，否决。

## R-7：List2Expr 能力（回答 TD-005）

- **Decision**：`internal/expression` 新增 `List2Expr(values []string) string` 与 `IDsToExpr(ids []uint32) string`：
  去重 → 空返回 `*` → 单个返回原值 → 多个返回 `[` + 枚举 + `]`。
  - `List2Expr`（名称段：集群/模块/服务实例/进程别名）为**字面量枚举**（去重升序、不做数字区间压缩、保留原值）。
  - `IDsToExpr`（进程 ID 段）对齐 gsekit `compressed_list`，连续数字压成 `a-b`（`[6-8]`）。
  - 补单测覆盖 空/单个/多个/前导零名称/进程 ID 连续压缩。
- **Rationale**：`internal/expression` 现有 `GenExpression`（五段→点分串）、`ScopeToCcIDs`（表达式→id），
  缺「数组→表达式」反向能力；补齐后非插件记录与迁移复用同一实现。
  数字区间压缩仅施于进程 ID：进程 ID 恒为规范整数（无前导零），压缩安全无损；名称段是离散标识符，
  若把形如 `01`/`02` 的名称当数字压成 `[1-2]`，前导零丢失后无法匹配回原始 CMDB 名称（评审收敛，Review-01）。
- **Alternatives considered**：在 service 层/迁移文件各写一份拼接——重复实现、语义易漂移，否决。

## R-8：前端展示与跳转过滤（FR-004~FR-006）

- **Decision**：
  - `ui/types/task.ts` `IOperateRange` 由数组改为五段字符串（`set_name/module_name/service_name/
    process_alias/process_id`）；`task_data.operate_range` 类型随之更新。
  - `ui/src/store/task.ts` `taskDetail.operate_range` 默认值改为五段空字符串。
  - `ui/src/views/space/task/detail/info.vue` `mergeOpRange` 改为读五段字符串、空段补 `*`、`.` 拼接，
    移除 `OP_RANGE_ORDER` 数组逻辑；`handleGoProcess` 维持（置 `filterFlag` 跳转 `process-management`）。
  - `ui/src/views/space/process/components/filter-process.vue` 的 `filterFlag` 分支：改为切表达式模式
    （`filterType='expression'`），用五段字符串填 `expressionValues`（`set_name→sets` 等映射），
    `triggerSearch` 走 `expression_scope` 过滤；移除旧数组填充 `filterValues` 的逻辑。命中为空展示空列表。
- **Rationale**：满足 FR-004/FR-005；表达式过滤内核与页面过滤能力均由 135740005 就绪，前端只做展示与
  条件透传，且与后端表达式字符串结构对齐后为单一路径。

## R-9：技术上下文与依赖

- **语言/版本**：Go（后端，服从根 `.golangci.yml`）、Vue 3 + TypeScript（前端 `ui/`）。
- **存储**：MySQL `task_batches.task_data`（JSON blob 文本列），本需求无 DDL 变更，仅数据迁移刷新 JSON 值。
- **新增依赖**：无。表达式生成/匹配复用 `internal/expression`（135740005 成果），仅新增 `List2Expr` 同包能力。
- **测试**：Go 单包测试（`internal/expression`、`pkg/protocol/core/task_batch`、`cmd/data-service/service`、
  迁移包）；前端 `tsc` 类型检查 + 分支走查。参考 `internal/expression/*_test.go`、
  `cmd/data-service/service/process_ip_test.go` 现有测试风格。
