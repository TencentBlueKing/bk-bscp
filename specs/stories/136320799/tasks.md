# Tasks — Story 136320799：集群环境类型、服务实例名等拓扑字段同步 CMDB 状态

**需求 ID**：136320799（TAPD: 1020451610136320799）
**输入**：`specs/stories/136320799/plan.md`、`research.md`、`data-model.md`、`spec.md`
**开发模式**：测试驱动开发（TDD，先写测试后写实现）
**Feature Directory**：`specs/stories/136320799`（`SPECIFY_FEATURE_DIRECTORY=specs/stories/136320799`）
**分支**：不在本阶段创建 git 分支

## 约定

- 任务格式：`- [ ] [TaskID] [P?] [Story?] 描述（含文件路径）`。`[P]` 表示可与同阶段其他 `[P]` 任务并行（不同文件、无未完成依赖）。
- 单包验证命令：`go test ./internal/processor/cmdb/`。
- 修改 Go 文件后必须 `gofmt`；新增文件需带仓库标准 MIT 头（`goheader`）。
- 覆盖语义：集群环境类型、服务实例名称两个字段直接以 CMDB 值覆盖（含覆盖为空），不加空值保护（FR-002 / Q-002）。
- **范围决策（2026-07-21，用户确认）**：集群名称（`set_name`）、模块名称（`module_name`）不做 diff、不更新。

## 用户故事与优先级映射

| Story | 对应需求 | 优先级 | 验收 |
|-------|----------|--------|------|
| US1 | F-001 集群环境类型/服务实例名称增量同步修复（改动点 1/2） | P1（MVP） | AC-001~004 / FR-001~005 / TR-001 |
| US2 | F-002 gsekit vs bscp CMDB 同步能力对比清单（改动点 3） | P2 | AC-005 / FR-006 |
| US3 | F-003 对比清单确认缺口补齐（改动点 4） | P3 | AC-006 / FR-007 |

---

## Phase 1：Setup（环境与基线确认）

- [X] T001 确认改动涉及文件与只读边界：阅读 `internal/processor/cmdb/sync_cmdb.go` 的 `BuildProcessChanges`（L1616 起）与 `internal/processor/cmdb/sync_cmdb_ostype_test.go` 的 fake DAO 模式（`fakeReusableProcessDao` / `fakeEmptyInstanceDao` / `fakeReusableDaoSet` / `SyncContext` 构造），确认 `pkg/dal/table/process.go` 的 `ProcessSpec` 四字段（`ServiceName`/`Environment`/`SetName`/`ModuleName`）为只读不改。
- [X] T002 建立基线：运行 `go test ./internal/processor/cmdb/` 确认现有单测通过，作为回归对照基线。

---

## Phase 2：Foundational（阻塞性前置）

> 本需求为单函数扩展，无表结构/依赖/接口变更（见 `data-model.md`、`plan.md` 合规门禁）。无独立的阻塞性前置任务；US1 的测试骨架即为后续实现前置，已并入 US1 阶段。

*（无 Foundational 任务）*

---

## Phase 3：US1 — F-001 集群环境类型/服务实例名称增量同步修复（P1 / MVP）

**故事目标**：一键同步时，对 CMDB 已存在进程识别并覆盖更新 `service_name`/`environment` 两个字段；两字段不变时不产生无意义更新；别名变更命中 reusable 恢复分支时同步刷新这两个字段。集群名称/模块名称不做 diff、不更新。

**独立验收标准**：`go test ./internal/processor/cmdb/` 中 T1~T6 表驱动用例全部通过，即等价满足 AC-001~004 与 TR-001（端到端 AC 由验收阶段人工/联调覆盖）。

### 测试先行（TDD：先写失败测试）

> 全部新增/扩展到 `internal/processor/cmdb/` 下同目录测试文件（复用 `sync_cmdb_ostype_test.go` 的 fake DAO 范式）。建议新增 `internal/processor/cmdb/sync_cmdb_topo_test.go`，带 MIT 文件头；表驱动组织 T1~T6。以下测试任务同文件、需串行落笔，故不标 `[P]`。

- [X] T003 [US1] 在 `internal/processor/cmdb/sync_cmdb_topo_test.go` 搭建表驱动测试骨架与用例 **T1（仅 `service_name` 变更）**：构造 old/new `ProcessSpec` 仅 `ServiceName` 不同，断言 `BuildProcessChanges` 结果 `ToUpdateProcess` 非空且 `ServiceName` 为新值。（FR-001 / AC-002 / AC-T01）
- [X] T004 [US1] 在 `internal/processor/cmdb/sync_cmdb_topo_test.go` 追加用例 **T2（仅 `environment` 变更）**：仅 `Environment` 不同（如 "1"→"3"），断言 `ToUpdateProcess` 非空且 `Environment` 为新值。（FR-001 / FR-003 / AC-001）
- [X] T005 [US1] 在 `internal/processor/cmdb/sync_cmdb_topo_test.go` 追加用例 **T3（仅 `set_name`/`module_name` 变更，环境类型/服务实例名不变）**：仅 `SetName`/`ModuleName` 不同，断言 `ToUpdateProcess` 为 nil，**不因这两个字段触发更新**（范围决策：不同步集群名称/模块名称）。（FR-001a / AC-003）
- [X] T006 [US1] 在 `internal/processor/cmdb/sync_cmdb_topo_test.go` 追加用例 **T4（环境类型/服务实例名均不变且其余不变）**：断言 `ToUpdateProcess` 为 nil、不因这两个字段产生更新。（FR-004 / AC-004）
- [X] T007 [US1] 在 `internal/processor/cmdb/sync_cmdb_topo_test.go` 追加用例 **T5（两字段变更为空）**：new 侧 `ServiceName`/`Environment` 为空、old 侧非空，断言覆盖写空（`ToUpdateProcess` 对应字段为空）。（FR-002 / Q-002）
- [X] T008 [US1] 在 `internal/processor/cmdb/sync_cmdb_topo_test.go` 追加用例 **T6（别名 + 两字段同时变更且命中 reusable）**：用 `fakeReusableProcessDao` 提供可复用 deleted 记录，断言恢复进程（`ToUpdateProcess`）`ServiceName`/`Environment` 刷新为 `newP.Spec` 新值、不残留旧值。（FR-005 / TR-001）
- [X] T009 [US1] 运行 `go test ./internal/processor/cmdb/ -run BuildProcessChanges` 确认 T1/T2/T5/T6 因缺陷 **失败**、T3/T4 通过（红灯基线，验证测试有效）。

### 实现（测试通过后收敛为绿灯）

- [X] T010 [US1] 改动点 1（检测）：在 `internal/processor/cmdb/sync_cmdb.go` 的 `BuildProcessChanges` 现有 5 个变更标志之后新增 `topoChanged`（`ServiceName || Environment` 任一不等，**不含 SetName/ModuleName**），并将其追加进早退守卫（L1633 `if !nameChanged && ... { return result, nil }` 追加 `&& !topoChanged`）。（FR-001 / FR-001a / FR-004 / AC-004）
- [X] T011 [US1] 改动点 1（写回）：在 `internal/processor/cmdb/sync_cmdb.go` 早退守卫之后、`nameChanged` 分支之前（仿 osType 写回位置）统一写回 `if topoChanged { oldP.Spec.ServiceName = newP.Spec.ServiceName; oldP.Spec.Environment = newP.Spec.Environment }`，直接覆盖含空值、不加空值保护，使主更新路径与安全原地改别名路径均生效。不写回 SetName/ModuleName。（FR-001 / FR-002 / FR-003 / Q-002）
- [X] T012 [US1] 改动点 2（reusable 恢复分支）：在 `internal/processor/cmdb/sync_cmdb.go` `reusableProc` 恢复块（L1667-1675 附近）显式回填 `reusableProc.Spec.ServiceName = newP.Spec.ServiceName; reusableProc.Spec.Environment = newP.Spec.Environment`；确认重建分支（`toAdd.Spec = newP.Spec`）已自然携带最新值无需改动。（FR-005 / TR-001）
- [X] T013 [US1] 对 `internal/processor/cmdb/sync_cmdb.go` 运行 `gofmt -w`，并运行 `go test ./internal/processor/cmdb/` 确认 T1~T6 全部通过（绿灯）、原有单测无回归。（FR-004 回归保障）

**检查点**：US1 完成后，四字段增量同步缺陷修复可独立验收；此即 MVP 交付范围。

---

## Phase 4：US2 — F-002 gsekit vs bscp CMDB 同步能力对比清单（P2）

**故事目标**：产出一份 gsekit 与 bscp 在 CMDB 同步能力上的差异对比清单（Markdown），逐项标注 bscp 是否支持及差异点。

**独立验收标准**：`docs/` 下存在对比清单，逐项覆盖「同步实体/字段」「新增 vs 增量更新覆盖字段」并标注 bscp 支持情况与差异（AC-005）。

- [X] T014 [US2] 依据 `research.md` 调研方向 4 的对比骨架，在 `docs/` 下新建 gsekit vs bscp CMDB 同步能力对比清单（如 `docs/cmdb-sync-capability-gsekit-vs-bscp.md`），逐项对比进程新增/删除、别名/进程名、进程属性、实例扩缩容、`bk_set_env`/`bk_set_name`/`bk_module_name`/服务实例 `name`、os_type/agent 状态，并标注 bscp 是否支持及差异点。（FR-006 / AC-005）
- [X] T015 [US2] 校对对比清单：确认对 gsekit `sync_biz_process`（`bulk_update` 字段集 + `expression` 拼装）与 bscp `BuildProcessChanges` 现状描述与源码一致，避免夸大/遗漏。（FR-006 准确性）

**检查点**：US2 完成后对比清单可交付，并作为 US3 缺口范围判定依据。

---

## Phase 5：US3 — F-003 对比清单确认缺口补齐（P3）

**故事目标**：对 F-002 对比清单标注为「bscp 缺失且需补齐」的缺口进行本期补齐，使其在一键同步中生效；范围以对比结论 + 用户确认为准，不超出 gsekit 一键同步能力边界。

**独立验收标准**：对比清单中确认的缺口在一键同步中生效，并有对应用例验证（AC-006）。

- [X] T016 [US3] 依据 US2 对比清单结论核对已识别缺口：确认本期需补齐的缺口为集群环境类型、服务实例名称两个字段（集群名称/模块名称按范围决策不补齐），已由 US1（改动点 1/2）覆盖，且 T1~T6 已提供验证用例，据此判定 F-003 无额外代码改动。（FR-007 / AC-006）
- [X] T017 [US3] 若 US2 对比清单发现两字段之外的额外缺口：**不在本阶段私自扩范围**，在 `specs/stories/136320799/questions.md` 追加 open 条目回需求侧补充范围并重估工时（Q-003 / spec.md Assumptions），并以此为准决定是否新增任务。（FR-007 范围守卫）

**检查点**：US3 完成后 F-003 范围收敛，缺口补齐状态明确可查证。

---

## Phase 6：Polish & 跨切面收尾

- [X] T018 [P] 全量 `gofmt` 检查改动的 Go 文件（`internal/processor/cmdb/sync_cmdb.go` 及新增测试文件），确认无格式问题。
- [X] T019 [P] 运行 `golangci-lint`（或按 `.golangci.yml`）核对 `funlen(120)`/`gocyclo(30)` 等规则；确认 `BuildProcessChanges` 增量在 `nolint: funlen,gocyclo` 范围内、新测试文件带 MIT 头。
- [X] T020 运行 `go test ./internal/processor/cmdb/` 收尾回归，确认 T1~T6 + 原有单测全绿。
- [X] T021 需求覆盖对照复核（对照 `plan.md` Traceability 表）：FR-001（T010/T011+T1~T2）、FR-001a（T010+T3，set_name/module_name 变化不触发更新）、FR-002（T011+T5）、FR-003（T004）、FR-004（T010+T4）、FR-005（T012+T6）、FR-006（T014/T015）、FR-007（T016/T017）、FR-008（无新增接口/入口，仅扩展 diff）逐项确认已落地。

---

## 依赖关系与执行顺序

- **Setup（T001-T002）** → 先于所有阶段。
- **US1（P1，T003-T013）**：测试任务 T003-T009 先行（TDD 红灯），实现任务 T010-T013 其后（绿灯）。T010→T011 有序（同文件、写回依赖检测标志）；T012 与 T010/T011 同文件不同代码块，建议串行落笔。T013 依赖 T010-T012。
- **US2（P2，T014-T015）**：依赖 `research.md`（已具备），可在 US1 之后或并行推进（不同文件：`docs/`）。
- **US3（P3，T016-T017）**：依赖 US2 对比清单结论 + US1 代码落地。
- **Polish（T018-T021）**：依赖 US1/US2/US3 完成。

## 并行执行示例

- US1 内测试 T003-T008 为同一测试文件的表驱动用例，串行落笔更稳妥；如拆分到不同文件则可 `[P]`。
- US2（`docs/` 文档）与 US1（`internal/processor/cmdb/` 代码）落点不同，可并行：一人写代码修复，一人产出对比清单。
- Polish 中 T018/T019 可并行（格式检查与 lint 检查互不阻塞）。

## 实现策略（MVP 优先，增量交付）

1. **MVP = US1（F-001）**：四字段增量同步修复，根因明确、可独立实现与验收，优先交付。
2. **US2（F-002）**：对比清单交付，作为缺口判定依据。
3. **US3（F-003）**：以对比清单结论收敛缺口范围；当前已识别缺口即四字段（US1 已覆盖），无额外代码改动；若发现额外缺口回需求侧补充范围（不私自扩范围）。
