# Phase 1 数据模型：操作范围记录优化（Story 136178657）

> 本需求**无数据库 DDL 变更**：任务批次执行数据以 JSON blob 存于 `task_batches.task_data` 文本列。
> 本需求把 `OperateRange` 五段由数组重构为表达式字符串（JSON 值形态变化），并用一次性数据迁移把存量
> 旧数组值刷成表达式字符串。以下描述持久化结构体、协议消息、前端类型、迁移与新增能力四层。

## 实体一：操作范围 OperateRange —— 由数组重构为五段表达式字符串

五段表达式字符串，语义以 gsekit `expression_scope` 为基准；缺省段为 `*`（匹配任意）。环境类型不在本
结构体，复用 `TaskExecutionData.Environment`。

| 字段 | 类型 | json 键 | 说明 | 约束 |
|------|------|---------|------|------|
| SetName | string | set_name | 集群名称表达式 | 缺省 `*` |
| ModuleName | string | module_name | 模块名称表达式 | 缺省 `*` |
| ServiceName | string | service_name | 服务实例名称表达式 | 缺省 `*` |
| ProcessAlias | string | process_alias | 进程别名表达式 | 缺省 `*` |
| ProcessID | string | process_id | CC 进程 ID 表达式（支持切片） | 缺省 `*` |

- 五段与 `internal/expression.Scope`（`SetName/ModuleName/ServiceName/ProcessAlias/ProcessID`）一一对应，
  匹配时用 `Scope`，持久化/协议用五段字符串（TD-002）。

### 1.1 持久化结构体（`pkg/dal/table/task_batch.go`）

**变更前**（数组）：

```go
type OperateRange struct {
    SetNames     []string `json:"set_names"`
    ModuleNames  []string `json:"module_names"`
    ServiceNames []string `json:"service_names"`
    ProcessAlias []string `json:"process_alias"`
    CCProcessID  []uint32 `json:"cc_process_ids"`
}
```

**变更后**（五段表达式字符串）：

```go
// OperateRange 操作范围（gsekit 风格五段表达式，缺省段为 "*"）。
type OperateRange struct {
    SetName      string `json:"set_name"`
    ModuleName   string `json:"module_name"`
    ServiceName  string `json:"service_name"`
    ProcessAlias string `json:"process_alias"`
    ProcessID    string `json:"process_id"`
}
```

- `TaskExecutionData` 结构不变（仍为 `Environment` + `OperateRange` + `ConfigTemplateIDs`），
  `String()`/`GetTaskExecutionData()` 逻辑不变（走 `json.Marshal`/`Unmarshal`）。
- **兼容**：json 键由复数改单数、值由数组改字符串，未迁移的旧 JSON 反序列化后五段为空 → 展示 `*.*.*.*.*`；
  数据迁移把存量刷成真实表达式（见实体四）。

## 实体二：任务批次执行数据（TaskExecutionData）—— 承载点

- **写入链路**：
  - 进程操作：`process.go` `buildOperateRange` → `createTaskBatch` 写入五段字符串。
  - 配置操作：`config_instance.go` `runConfigTask` 写入五段字符串。
- **记录规则**：
  - 插件路径：五段 = 请求 `expression_scope`（原样存，空段补 `*`）。
  - 非插件路径：`ProcessID = expression.IDsToExpr(命中进程 CcProcessID)`，其余段 `*`（FR-008）。

## 实体三：展示协议（`pkg/protocol/core/task_batch/task_batch.proto` `OperateRange`）

**变更前**：`repeated string set_names/module_names/service_names/cc_process_names` + `repeated uint32 cc_process_ids`。

**变更后**（重新生成 `task_batch.pb.go`）：

```proto
// OperateRange 操作范围（gsekit 风格五段表达式，缺省段为 "*"）
message OperateRange {
  string set_name = 1 [... { description: "集群名称表达式" }];
  string module_name = 2 [... { description: "模块名称表达式" }];
  string service_name = 3 [... { description: "服务实例名称表达式" }];
  string process_alias = 4 [... { description: "进程别名表达式" }];
  string process_id = 5 [... { description: "CC进程ID表达式" }];
}
```

- `ProcessTaskData` 仍为 `environment` + `operate_range`（类型指向新 `OperateRange`）。
- `convert.go` `PbTaskBatch`：五段字符串一一透传（`taskData.OperateRange.SetName` → `pb.OperateRange.SetName` 等），
  无派生逻辑。

## 实体四：数据迁移（`cmd/data-service/db-migration/migrations/`）

新增迁移 `<ts>_migrate_task_batch_operate_range_to_expression.go`（`migrator.GormMode`）：

- **Up**：分页遍历 `task_batches`，逐行 `json.Unmarshal(task_data)` 到一个**宽松结构**（同时可读旧数组键
  与新字符串键）；若识别为旧数组格式，则用 `expression.List2Expr`/`IDsToExpr` 把五段转字符串，重写
  `operate_range` 并 `UPDATE task_data`。已是字符串格式（新键存在/数组键缺失）则跳过（**幂等**）。
  - 映射：`set_names[]→set_name`、`module_names[]→module_name`、`service_names[]→service_name`、
    `process_alias[]→process_alias`、`cc_process_ids[]→process_id`（`IDsToExpr`）。
  - 空数组 → `*`；单个 → 原值；多个 → `[..]`（连续数字压缩 `[6-8]`）。
- **Down**：空操作（`return nil`），注明表达式 → 数组不可逆。
- 迁移文件内定义**本地** old/new 解析结构（不依赖 `table.OperateRange` 当前形态），仅复用
  `internal/expression.List2Expr`/`IDsToExpr`，避免与被迁移结构耦合。

## 实体五：List2Expr 新增能力（`internal/expression`）

新增 `list2expr.go`（对齐 gsekit `parse_list2expr` + `compressed_list`）：

```go
// List2Expr 把去重后的字符串列表拼成表达式：空→"*"、单个→原值、多个→"[..]"（连续数字压成 a-b）。
func List2Expr(values []string) string

// IDsToExpr 把 CC 进程 ID 列表转成表达式（uint32→string 后走 List2Expr）。
func IDsToExpr(ids []uint32) string
```

- 单测覆盖：空列表→`*`、单值→原值、多值枚举→`[a,b]`、连续数字→`[6-8]`、混合→`[6,8-9]` 等，
  语义对齐 gsekit `expression_utils` 测试。

## 实体六：前端类型（`ui/types/task.ts` / `ui/src/store/task.ts`）

**变更前** `IOperateRange`（数组）→ **变更后**（五段字符串）：

```ts
export interface IOperateRange {
  set_name: string;
  module_name: string;
  service_name: string;
  process_alias: string;
  process_id: string;
}
// task_data: { environment: string; operate_range: IOperateRange };
```

- `store/task.ts` `taskDetail.operate_range` 默认值改为五段空字符串。
- `info.vue` `mergeOpRange`：`[set_name, module_name, service_name, process_alias, process_id]`
  逐段 `|| '*'` 后以 `.` 拼接（单一路径，移除 `OP_RANGE_ORDER`）。
- `filter-process.vue` `filterFlag` 分支：切表达式模式，用五段字符串填 `expressionValues`
  （`set_name→sets`、`module_name→modules`、`service_name→service_instances`、`process_alias→process_aliases`、
  `process_id→cc_process_ids`），`triggerSearch` 发 `expression_scope` 过滤。

## 请求侧（复用，不改动）

- `pkg/protocol/core/process/process.proto`：`OperateRange.expression_scope`（field 9，`ExpressionScope` 五段
  `set_name/module_name/service_name/process_alias/process_id`）+ `environment`（field 1）已由 135740005 就绪；
  `ProcessSearchCondition.expression_scope`（field 12）为进程列表页附加过滤条件，跳转过滤直接复用。
  **本需求不改请求契约**。

## 状态流转

- 无新增状态机；任务批次状态（running/succeed/failed/partly_failed）沿用现状。操作范围仅为记录/展示数据。
