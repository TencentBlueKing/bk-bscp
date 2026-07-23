# Clarification Questions — Story 136178657

> 说明：本文件记录技术澄清阶段的问答。状态取值：open / answered / resolved_by_doc。
> resolved_by_doc 表示已从上下文白名单文档自答，结论已写入 req.md「技术澄清」章节。

## resolved_by_doc

### Q-001 表达式范围在任务批次记录中的具体承载形态
- **状态**：resolved_by_doc
- **来源**：req.md 未解决问题 Q-001
- **自答依据**：
  - `pkg/protocol/core/process/process.proto`：请求侧 `OperateRange.expression_scope`（field 9，
    `ExpressionScope` 五段字符串）+ `environment`（field 1）已由 135740005 就绪，请求契约无需再改。
  - `pkg/dal/table/task_batch.go`：`table.OperateRange` 为枚举值数组（`[]string`/`[]uint32`），
    无法承载 `4[6,8,9]`/`[1-1000]` 表达式与切片语义 → 需在 `TaskExecutionData` 新增表达式载体字段，
    不复用单值数组；`TaskExecutionData.String()` 用 `json.Marshal` 忽略零值，新旧字段天然共存（兼容）。
  - `pkg/protocol/core/task_batch/task_batch.proto` + `convert.go`：`ProcessTaskData` 需新增表达式范围
    字段，`PbTaskBatch` 增补 table→pb 搬运。
  - `internal/expression/scope.go`：`Scope` 五段与 proto `ExpressionScope` 一一对应，但为内存匹配结构，
    记录/展示应用 proto `ExpressionScope`（TD-002）。
- **结论落点**：req.md「技术澄清 / 技术决策记录」TD-001、TD-002。

### Q-002 服务端记录链路缺陷定位与修复方向
- **状态**：resolved_by_doc
- **自答依据**：
  - `cmd/data-service/service/process.go:461 buildOperateRange`：读 `req.OperateRange.GetSetName()` 等
    单值字段，新入参下恒空 → 记录空 → 前端恒显 `*.*.*.*.*`。修复：改读 `GetExpressionScope()` + `GetEnvironment()`。
  - `cmd/data-service/service/config_instance.go:1747 buildOperateRange`：插件模式按命中进程真实属性
    去重物化枚举值，丢失表达式与切片语义 → 统一改为记录请求 `expression_scope`（TD-003）。
  - `cmd/config-server/service/process.go:42`：仅透传 `OperateRange`，无需改造。
- **结论落点**：req.md「技术澄清 / 技术方案概述」「技术决策记录 TD-003」。

### Q-003 前端展示与跳转过滤落地
- **状态**：resolved_by_doc
- **自答依据**：
  - `ui/src/views/space/task/detail/info.vue`：`mergeOpRange` 现按旧数组五段拼接（恒回退 `*`）；
    `handleGoProcess` 现仅 `router.push` 到 process-management 并置 `filterFlag`，未携带过滤条件。
    需读取新表达式字段展示，并将记录的表达式范围带到进程列表页过滤。
  - `pkg/protocol/core/process/process.proto:110-113`：`ProcessSearchCondition.expression_scope`
    已作为附加过滤条件存在（135740005 提供），跳转过滤复用即可。
  - `ui/types/task.ts`：`IOperateRange` 现为数组类型，需补充表达式范围类型字段。
- **结论落点**：req.md「技术澄清 / 技术方案概述」「架构影响」。

## open

（无）

## answered

（无）
