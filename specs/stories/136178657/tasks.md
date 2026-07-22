# Tasks: 操作范围记录优化

**需求 ID**: 136178657（TAPD: 1020451610136178657）
**Input**: `specs/stories/136178657/` 下 `plan.md`（P1~P8 + 需求覆盖映射）、`spec.md`（用户故事/FR/AC）、`research.md`、`data-model.md`
**Prerequisites**: plan.md（required）、spec.md（required）、research.md、data-model.md

**Tests**: 采用 TDD——后端单元测试（`List2Expr`、两处 `buildOperateRange`、`convert` 透传、迁移转换/幂等）先写失败测试再实现。前端 `ui/` **无既有单测框架**（`package.json` 无 vitest/jest），按仓库约束不引入测试框架，前端逻辑改为 `tsc` 类型检查 + 分支走查验证。

**Organization**: 任务按用户故事分组。US1（记录并展示，含存量迁移）为 MVP；US2（点击跳转按表达式过滤）在 US1 展示能力之上叠加。

## Format: `[ID] [P?] [Story] Description`

- **[P]**: 可并行（不同文件、无未完成依赖）
- **[Story]**: 所属用户故事（US1 / US2）；Setup / Foundational / Polish 阶段无 Story 标签
- 每个任务含明确文件路径

## 用户故事映射（来自 spec.md）

- **US1（P1，MVP）**：操作范围重构为五段表达式字符串——服务端如实记录插件传入的表达式范围（覆盖进程 + 配置操作）、非插件路径拼命中进程表达式、存量数据迁移刷成表达式、任务详情按表达式单一路径展示——覆盖 FR-001/FR-002/FR-003/FR-004/FR-006/FR-007/FR-008、AC-001/AC-002/AC-003/AC-005/AC-T01/AC-T02/AC-T03。
- **US2（P2）**：点击「操作范围」跳转进程列表页并切表达式模式按记录的五段表达式过滤命中进程，命中空展示空列表——覆盖 FR-005、AC-004。

## Path Conventions

后端 Go 微服务（仓库根相对路径）+ 前端 Vue（`ui/`），单仓多模块，沿用现有目录分层，不新建包。所有代码路径不得越出 `specs/stories/136178657/context.md` 的 Code scope。

---

## Phase 1: Setup（共享前置）

**Purpose**: 校验后续 TDD、proto 生成、迁移与前端类型检查所需工具链可用（不改代码）

- [ ] T001 校验工具链可用：`gofmt`、`golangci-lint`（相关包）、`cd pkg/protocol && make`（protoc 生成链）、迁移可编译、`ui` 下 `tsc`/类型检查命令；记录基线

---

## Phase 2: Foundational（阻塞性前置）

**Purpose**: 数组→表达式能力 + 存储结构重构 + 展示协议/转换/通知，是记录链路、迁移与展示链路的共同前提

**⚠️ CRITICAL**: 本阶段完成前，任何用户故事任务不得开始

- [ ] T002 [P] 编写 `internal/expression/list2expr_test.go`：断言 `List2Expr(nil)`→`*`、`List2Expr(["a"])`→`a`、`List2Expr(["a","b"])`→`[a,b]`、`IDsToExpr([6,7,8])`→`[6-8]`、`IDsToExpr([6,8,9])`→`[6,8-9]`、`IDsToExpr(nil)`→`*`（先失败，对齐 gsekit parse_list2expr/compressed_list，TD-005）
- [ ] T003 在 `internal/expression/list2expr.go` 实现 `List2Expr([]string) string` 与 `IDsToExpr([]uint32) string`（去重、空→`*`、单个→原值、多个→`[..]`、连续数字压缩），使 T002 通过（FR-008/FR-007 基础）
- [ ] T004 在 `pkg/dal/table/task_batch.go` 将 `OperateRange` 五段由数组改为 `string`（`SetName/ModuleName/ServiceName/ProcessAlias/ProcessID`，json 键 `set_name/…/process_id`）（FR-006）
- [ ] T005 在 `pkg/protocol/core/task_batch/task_batch.proto` 将 `OperateRange` 五段由 `repeated`/`repeated uint32` 改为 `string`、键名统一 `set_name/module_name/service_name/process_alias/process_id`，执行 `cd pkg/protocol && make clean && make` 重新生成 `task_batch.pb.go`（勿手改生成物）（FR-006）
- [ ] T006 [P] 编写 `pkg/protocol/core/task_batch/convert_test.go`：断言 `PbTaskBatch` 将 `taskData.OperateRange` 五段字符串一一透传到 pb，缺省段随存储（先失败，AC-003 展示数据）
- [ ] T007 在 `pkg/protocol/core/task_batch/convert.go` `PbTaskBatch` 改为五段字符串透传（`SetName→SetName` 等），使 T006 通过并修复 P2 引发的编译错误（FR-006）
- [ ] T008 在 `internal/task/executor/common/common.go` `buildScopeText` 改为解析新五段字符串、用 `expression.GenExpression`（空段补 `*`）拼 `.` 分割串，替换仅读 `CCProcessID` 的旧逻辑（FR-004 通知侧一致）

**Checkpoint**: 能力、存储结构、协议/转换/通知就绪——记录链路、迁移与展示链路可开工

---

## Phase 3: User Story 1 - 如实记录、迁移并按表达式展示操作范围（Priority: P1）🎯 MVP

**Goal**: 服务端把插件请求携带的五段表达式范围 + 环境如实写入任务批次（非插件路径拼命中进程表达式）；存量数据迁移刷成表达式；任务详情按表达式展示（缺省段 `*`）。

**Independent Test**: 用携带 `expression_scope` 的进程/配置请求触发建批次，断言记录的五段字符串等于请求；非插件请求断言 `process_id` 为压缩表达式；运行迁移把旧数组样本刷成表达式且幂等；前端对五段字符串展示 gsekit 风格串。

### Tests for User Story 1（TDD——先写失败测试）⚠️

- [ ] T009 [P] [US1] 编写 `cmd/data-service/service/process_test.go`：给定携带 `OperateRange.expression_scope` 五段表达式且单值字段为空的 `OperateProcessReq`，断言构建的五段字符串等于请求、未读任何单值字段（AC-001/AC-T01）；给定非插件请求（`OperateRange==nil`）断言 `process_id` 为命中进程 CcProcessID 的压缩表达式、其余段 `*`（AC-T03）（先失败）
- [ ] T010 [P] [US1] 编写 `cmd/data-service/service/config_instance_test.go`：给定携带 `expression_scope` 的插件配置请求（`pluginMode==true`）断言记录五段字符串与请求一致（AC-002）；非插件模式断言 `process_id` 压缩表达式、其余 `*`（AC-T03）（先失败）

### Implementation for User Story 1（后端记录）

- [ ] T011 [US1] 在 `cmd/data-service/service/process.go` 改造 `buildOperateRange`：插件路径从 `req.OperateRange.GetExpressionScope()` 原样取五段（空段补 `*`）、environment 取 `GetEnvironment()`，不读单值字段；非插件路径 `ProcessID = expression.IDsToExpr(命中 CcProcessID)`、其余段 `*`；使 T009 通过（FR-001/FR-002/FR-008）
- [ ] T012 [US1] 在 `cmd/data-service/service/config_instance.go`：`GenerateConfig`/`CheckConfig` 透传 `req.GetOperateRange()` 给 `runConfigTask`；`buildOperateRange` 插件模式原样存请求 `expression_scope`+`environment`、非插件模式 `ProcessID = expression.IDsToExpr(命中 CcProcessID)` 其余 `*`；使 T010 通过（FR-003/FR-008/TD-003）

### Implementation for User Story 1（存量迁移）

- [ ] T013 [US1] 新增 `cmd/data-service/db-migration/migrations/<ts>_migrate_task_batch_operate_range_to_expression.go`（`GormMode`）：`Up` 分页遍历 `task_batches`，旧数组 `task_data.operate_range` → 五段表达式字符串（复用 `expression.List2Expr`/`IDsToExpr`）重写并 UPDATE，已迁移跳过（幂等）；`Down` 空操作注明不可逆；`init()` 中 `AddMigration` 注册（FR-007/AC-005/AC-T02）
- [ ] T014 [US1] 迁移转换逻辑抽为可测函数并补单测（或在迁移包测试中）覆盖：旧数组样本→表达式字符串正确、空数组→`*`、连续数字→`[6-8]`、重复运行不变（幂等）（AC-T02）

### Implementation for User Story 1（前端展示）

- [ ] T015 [P] [US1] 在 `ui/types/task.ts` 将 `IOperateRange` 改为五段字符串（`set_name/module_name/service_name/process_alias/process_id`），更新 `task_data.operate_range` 类型
- [ ] T016 [P] [US1] 在 `ui/src/store/task.ts` 将 `taskDetail.operate_range` 默认值改为五段空字符串
- [ ] T017 [US1] 在 `ui/src/views/space/task/detail/info.vue` 改造 `mergeOpRange` 为单一路径：五段 `|| '*'` 以 `.` 拼接，移除 `OP_RANGE_ORDER` 与数组分支（FR-004/FR-006）
- [ ] T018 [US1] 对 `info.vue`/`task.ts`/`store/task.ts` 做走查 + `ui` `tsc` 类型检查，验证五段字符串展示正确、缺省段显示 `*`（前端无单测框架，AC-003）

**Checkpoint**: 进程 + 配置两类任务如实记录表达式、非插件路径拼表达式、存量迁移完成、任务详情按表达式展示——US1 可独立验收（MVP）

---

## Phase 4: User Story 2 - 点击操作范围按表达式过滤跳转（Priority: P2）

**Goal**: 点击任务详情「操作范围」跳转进程列表页，切表达式模式携带记录的五段表达式过滤，展示命中的全部进程；命中为空展示空列表，不回退全选。

**Independent Test**: 在有表达式范围的任务详情点击操作范围，断言跳转 `process-management` 后进程列表切表达式模式、`expression_scope` 等于记录的五段。

### Implementation for User Story 2

- [ ] T019 [US2] 在 `ui/src/views/space/process/components/filter-process.vue` 改造 `filterFlag` 分支：切表达式模式（`filterType='expression'`），用 `taskDetail.operate_range` 五段字符串填 `expressionValues`（`set_name→sets`/`module_name→modules`/`service_name→service_instances`/`process_alias→process_aliases`/`process_id→cc_process_ids`），`triggerSearch` 发 `expression_scope` 过滤；移除旧数组填充 `filterValues`；命中空展示空列表、不回退全选（FR-005/AC-004）
- [ ] T020 [US2] 对 `filter-process.vue`（及 `info.vue handleGoProcess` 置 `filterFlag`）做逻辑走查 + `ui` `tsc` 类型检查，验证跳转携带的 `expression_scope` 与记录一致（前端无单测框架）

**Checkpoint**: US1 + US2 均可独立验收——记录、迁移、展示、跳转过滤全链路打通

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: 收尾校验，保障合规与无回归

- [ ] T021 [P] 对改动的 Go 文件执行 `gofmt -w` 并运行 `golangci-lint`（`internal/expression`、`pkg/dal/table`、`pkg/protocol/core/task_batch`、`internal/task/executor/common`、`cmd/data-service/service`、迁移包）
- [ ] T022 运行 `go test ./internal/expression/... ./pkg/protocol/core/task_batch/... ./cmd/data-service/service/... ./cmd/data-service/db-migration/...` 确认相关单包全绿
- [ ] T023 校对 `pkg/protocol/core/task_batch/task_batch.pb.go` 生成 diff 仅为 `OperateRange` 字段类型/键名变化（无手改），并在 `ui/` 运行类型检查确认前端无类型错误

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup（Phase 1）**：无依赖，可立即开始
- **Foundational（Phase 2）**：依赖 Setup——阻塞所有用户故事；内部顺序 T002/T003（能力）→ T004/T005（结构/协议）→ T006/T007（convert）→ T008（common）
- **US1（Phase 3）**：依赖 Foundational
- **US2（Phase 4）**：依赖 Foundational + US1 前端类型/store（T015/T016）
- **Polish（Phase 5）**：依赖所有用户故事完成

### User Story Dependencies

- **US1（P1）**：Foundational 后即可开始
- **US2（P2）**：Foundational + US1 前端类型链路后开始；与 US1 前端改动相邻文件，顺序进行避免冲突

### Within Each User Story

- 后端测试先写并 FAIL，再实现使其通过
- 能力（List2Expr）→ 结构/协议 → convert/common → 服务端记录 → 迁移 → 前端展示 → 前端跳转

### Parallel Opportunities

- Foundational 内 T002（expression 测试）可与文档校对并行；T006 依赖 T005 生成的 pb 结构
- US1 内 T009/T010（两条链路测试）可并行；T015/T016（前端类型/store）与后端实现（T011/T012/T013）可并行
- Polish 内 T021 与文档校对可并行

---

## Implementation Strategy

### MVP First（仅 US1）

1. 完成 Phase 1 Setup
2. 完成 Phase 2 Foundational（能力 + 结构 + 协议/转换/通知）
3. 完成 Phase 3 US1（记录 + 非插件拼表达式 + 存量迁移 + 展示）
4. **STOP & VALIDATE**：验证插件/非插件记录、迁移无损幂等、任务详情按表达式展示
5. 可交付演示（MVP：不再恒显 `*.*.*.*.*`）

### Incremental Delivery

1. Setup + Foundational → 结构与能力就绪
2. US1 → 独立验收 → 演示（MVP）
3. US2 → 独立验收 → 演示（点击可回溯进程）

---

## Notes

- [P] 任务 = 不同文件、无未完成依赖
- 后端 TDD：先确认测试 FAIL 再实现；改 Go 文件后 `gofmt`
- proto 改动必须用 `cd pkg/protocol && make clean && make` 重新生成，不手改 `.pb.go`
- 迁移必须幂等、`Down` 空操作（表达式→数组不可逆）；复用 `internal/expression.List2Expr`/`IDsToExpr`，不在迁移文件重复实现拼接
- 前端无单测框架，验证以 `tsc` 类型检查 + 分支走查替代，不引入新测试框架（遵循 AGENTS.md）
- 不改请求侧协议，不改鉴权与 `biz_id` 隔离模型（FR-009）
- `info.vue`（T017）与 `filter-process.vue`（T019）改动不同文件，但均属前端表达式路径，注意与 `store/task.ts` 类型一致
