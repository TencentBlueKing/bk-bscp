# Implementation Plan：操作范围记录优化

**需求 ID**：136178657（TAPD: 1020451610136178657） | **日期**：2026-07-22 | **Spec**：`specs/stories/136178657/spec.md`
**输入**：`spec.md`、`req.md`（技术澄清 TD-001~TD-005）、`research.md`、`data-model.md`
**分支**：沿用当前分支（不创建新分支）

## Summary

把进程配置管理「操作范围」**彻底重构为表达式字符串结构**并打通「记录 → 展示 → 跳转过滤」三段，
对齐 gsekit `expression_scope` 的存储/展示：

1. **存储**：`table.OperateRange` 五段由数组改为 `string`（缺省 `*`）；`task_batch.proto` `OperateRange`
   由 `repeated` 改单字符串、键名对齐 `set_name/…/process_id`，重新生成 pb.go。
2. **能力**：`internal/expression` 新增 `List2Expr`/`IDsToExpr`（数组→表达式，含数字范围压缩），对齐
   gsekit `parse_list2expr`。
3. **记录**：进程 `process.go`、配置 `config_instance.go` 两条建批次链路——插件路径**原样存**请求
   `expression_scope`+`environment`；非插件路径用 `IDsToExpr` 把命中进程 CcProcessID 拼进 `process_id`、其余 `*`。
4. **消费**：`convert.go` 五段字符串透传；`common.go buildScopeText` 改用 `GenExpression` 拼展示串。
5. **迁移**：新增一次性幂等数据迁移，把存量 `task_batches.task_data` 旧数组刷成表达式字符串。
6. **前端**：`ui/types/task.ts`/`store/task.ts` 类型字符串化；`info.vue mergeOpRange` 单一路径展示；
   `filter-process.vue` 跳转分支切表达式模式按 `expression_scope` 过滤。

技术选型无新增依赖，表达式内核复用 `internal/expression`（135740005 成果）。

## Technical Context

- **Language/Version**：Go（后端，遵循根 `.golangci.yml`）；Vue 3 + TypeScript（前端 `ui/`）。
- **Primary Dependencies**：无新增。表达式生成/匹配复用 `internal/expression`（仅同包新增 `List2Expr`）；
  协议生成用 `pkg/protocol` 下 `make`（protoc 链，见根 `Makefile`）；迁移用 `cmd/data-service/db-migration`。
- **Storage**：MySQL `task_batches.task_data`（JSON blob 文本列）；**无 DDL 变更**，仅数据迁移刷新 JSON 值。
- **Testing**：Go 单包测试（`internal/expression`、`pkg/protocol/core/task_batch`、`cmd/data-service/service`、
  迁移包）；前端 `tsc` 类型检查 + 分支走查。TDD：先写失败测试再实现。
- **Project Type**：Web 服务（Go 微服务后端 + Vue 前端），单仓多模块。
- **Constraints**：不改请求侧协议；不改鉴权与 `biz_id` 隔离；不重复实现表达式内核；迁移幂等、Down 空操作；
  改动小、可追溯。
- **Scale/Scope**：轻量字段读写；数据迁移分页遍历 `task_batches`（存量量级小，无性能敏感路径）。

## Constitution Check

- **宪章状态**：`ai-practice/.specify/memory/constitution.md` 为未填充模板占位，仓库根不存在
  `.specify/memory/constitution.md`。按自检规则**跳过宪章门禁**，并在 `plan-report.md` 注明。
- **仓库规则替代基线**：以 `AGENTS.md`、`.golangci.yml`、`.claude/skills/bk-security-redlines/SKILL.md` 为基线：
  - 改动小、边界清晰、每处改动可追溯到 FR；不引入不必要抽象/配置层。重构 `OperateRange` 为表达式字符串是
    需求核心（FR-006），数据迁移是消除应用层双分支的必要一次性动作（FR-007），均可追溯。
  - 修改 Go 文件后运行 `gofmt`；新增行为优先补单包测试（`List2Expr`、两处 build、convert、迁移）。
  - 不改鉴权与业务隔离（安全红线：鉴权沿用现有；输入校验沿用 135740005；表达式为非敏感业务数据）。

*GATE 结论*：Phase 0 通过（无未决澄清）；Phase 1 设计复核通过（无新增违背项）。

## Project Structure

### Documentation（本需求）

```text
specs/stories/136178657/
├── spec.md            # 功能规范
├── req.md             # 需求原文 + 技术澄清（TD-001~TD-005）
├── plan.md            # 本文件
├── research.md        # Phase 0 输出
├── data-model.md      # Phase 1 输出
├── questions.md       # 澄清结论
└── plan-report.md     # 合规自检报告
```

### Source Code（改动点，仓库根相对路径）

```text
internal/expression/list2expr.go                  # 新增 List2Expr/IDsToExpr（对齐 gsekit parse_list2expr）
internal/expression/list2expr_test.go             # 新增 List2Expr 单测
pkg/dal/table/task_batch.go                        # OperateRange 五段 数组→string
pkg/protocol/core/task_batch/task_batch.proto      # OperateRange 五段 repeated→string，键名对齐
pkg/protocol/core/task_batch/task_batch.pb.go      # 由 make 重新生成（勿手改）
pkg/protocol/core/task_batch/convert.go            # PbTaskBatch 五段字符串透传
pkg/protocol/core/task_batch/convert_test.go       # 新增：五段透传单测
internal/task/executor/common/common.go            # buildScopeText 改读五段字符串 + GenExpression
cmd/data-service/service/process.go                # buildOperateRange：插件原样存/非插件 IDsToExpr
cmd/data-service/service/config_instance.go        # runConfigTask 透传请求 OperateRange，插件原样存/非插件 IDsToExpr
cmd/data-service/db-migration/migrations/<ts>_migrate_task_batch_operate_range_to_expression.go  # 新增数据迁移
ui/types/task.ts                                   # IOperateRange 数组→五段字符串
ui/src/store/task.ts                               # operate_range 默认值字符串化
ui/src/views/space/task/detail/info.vue            # mergeOpRange 单一路径展示
ui/src/views/space/process/components/filter-process.vue  # 跳转分支切表达式模式过滤

# 新增/改测试
internal/expression/list2expr_test.go
pkg/protocol/core/task_batch/convert_test.go
cmd/data-service/service/*_test.go                 # buildOperateRange / 配置链路记录 单测
```

**Structure Decision**：沿用现有目录与分层（expression / table / protocol / service / migration / ui），不新建包。
`cmd/config-server/service/process.go` 仅透传 `OperateRange`，不在改动范围。

## Phase 分解（实现顺序，TDD）

> 每步先补/改失败测试，再改实现使其通过；改 Go 文件后 `gofmt`，proto 改动后用仓库命令重新生成。

### P1 List2Expr 能力（TD-005 / FR-008 / FR-007 基础）
- `internal/expression/list2expr.go`：`List2Expr([]string)`、`IDsToExpr([]uint32)`，对齐 gsekit
  `parse_list2expr`（空→`*`、单个→原值、多个→`[..]`、连续数字压缩 `a-b`）。
- 测试 `list2expr_test.go`：空/单值/多值枚举/连续数字/混合。

### P2 存储结构重构（FR-006）
- `task_batch.go`：`OperateRange` 五段 `[]string`/`[]uint32` → `string`（json 键单数化）。
- `task_batch.proto`：`OperateRange` 五段 `repeated`→`string`，键名 `set_name/…/process_id`；
  `cd pkg/protocol && make clean && make` 重新生成 `task_batch.pb.go`。
- 该步会使下游引用暂时编译失败（convert/common/两处 build），在 P3~P5 修复。

### P3 展示协议转换与通知（FR-004 展示数据 / AC-003）
- `convert.go`：`PbTaskBatch` 五段字符串一一透传；`convert_test.go` 断言透传正确、缺省段随存储。
- `common.go buildScopeText`：解析新五段字符串，用 `GenExpression`（空段补 `*`）拼 `.` 分割串。

### P4 进程操作记录修复（FR-001/FR-002/FR-008/AC-001/AC-T01/AC-T03）
- `process.go buildOperateRange`：插件路径从 `req.OperateRange.GetExpressionScope()` 原样取五段（空补 `*`）、
  environment 取 `GetEnvironment()`，不读单值字段；非插件路径 `ProcessID = IDsToExpr(命中 CcProcessID)`、其余 `*`。
- 测试 `process_test.go`：插件请求断言五段=请求且未读单值；非插件请求断言 `process_id` 为压缩表达式、其余 `*`。

### P5 配置操作记录统一（TD-003/FR-003/AC-002/AC-T03）
- `config_instance.go`：`GenerateConfig`/`CheckConfig` 透传 `req.GetOperateRange()` 给 `runConfigTask`；
  插件模式原样存请求 `expression_scope`+`environment`；非插件模式 `ProcessID = IDsToExpr(命中 CcProcessID)`、其余 `*`。
- 测试 `config_instance_test.go`：插件配置请求断言五段=请求；非插件断言 `process_id` 压缩表达式。

### P6 存量数据迁移（FR-007/AC-005/AC-T02）
- 新增迁移文件（`GormMode`）：`Up` 分页遍历 `task_batches`，旧数组 JSON → 五段表达式字符串重写 `task_data`，
  已迁移则跳过（幂等）；`Down` 空操作（注明不可逆）。复用 `internal/expression.List2Expr`/`IDsToExpr`。
- 测试：迁移单测（若迁移包已有测试脚手架则复用，否则以 `List2Expr` 单测 + 迁移逻辑函数级单测覆盖转换正确性/幂等）。

### P7 前端表达式单一路径展示与跳转（FR-004/FR-005/AC-003/AC-004）
- `ui/types/task.ts` `IOperateRange` 五段字符串；`store/task.ts` 默认值字符串化。
- `info.vue mergeOpRange`：五段 `|| '*'` 以 `.` 拼接，移除 `OP_RANGE_ORDER`。
- `filter-process.vue` `filterFlag` 分支：切表达式模式、五段填 `expressionValues`、`triggerSearch` 发
  `expression_scope`；命中空展示空列表、不回退全选。
- `tsc` 类型检查 + 分支走查。

### P8 收尾
- `gofmt` + `golangci-lint`（相关包）+ 前端类型检查；`go test` 相关单包；核对 proto 生成 diff 仅为
  `OperateRange` 字段类型/键名变化。

## 需求覆盖映射

| 需求 | 计划落点 |
|------|----------|
| FR-001（插件原样记录表达式+环境，缺省 `*`） | P4 + P5 |
| FR-002（禁读单值字段） | P4 |
| FR-003（覆盖进程 + 配置两类任务） | P4 + P5 |
| FR-004（前端表达式展示，缺省 `*`） | P3（展示数据）+ P7（mergeOpRange） |
| FR-005（点击跳转按表达式过滤，命中空不回退全选） | P7（filter-process 表达式模式 + 复用 135740005） |
| FR-006（存储/协议/前端统一表达式字符串） | P2 + P3 + P7 |
| FR-007（存量幂等数据迁移） | P6 |
| FR-008（非插件路径拼表达式记录） | P1（IDsToExpr）+ P4 + P5 |
| FR-009（不重复实现内核/不改鉴权/不改请求协议） | 全程约束 |
| AC-001/AC-T01/AC-T03 | P4 单测 |
| AC-002 | P5 单测 |
| AC-003 | P3 + P7 |
| AC-004 | P7 |
| AC-005/AC-T02 | P6 迁移单测 |

## Complexity Tracking

无需破例的复杂度。相较「新增字段并存 + convert 层长期派生」的备选，Scheme C 通过一次性数据迁移把兼容成本
收敛到发布期一处，应用层（convert/common/前端）保持表达式单一路径，长期更简单、更可追溯。数组→表达式的
拼接统一由 `internal/expression.List2Expr` 一处实现（非插件记录 + 迁移复用），避免重复逻辑。
