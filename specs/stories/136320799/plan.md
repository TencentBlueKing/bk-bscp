# Implementation Plan — Story 136320799：拓扑字段同步 CMDB 状态

**需求 ID**：136320799（TAPD: 1020451610136320799）
**输入**：`specs/stories/136320799/spec.md`
**关联**：`research.md`（技术调研）、`data-model.md`（数据模型）
**开发模式**：测试驱动开发（TDD）
**分支**：不在本阶段创建 git 分支

## 概述

修复 bscp 一键同步时两个字段（`service_name`/`environment`）的增量更新缺陷（F-001）；
产出 gsekit vs bscp CMDB 同步能力对比清单（F-002）；补齐清单确认的缺口（F-003，范围以对比结论为准）。
核心改动集中在 `internal/processor/cmdb/sync_cmdb.go` 的 `BuildProcessChanges` 一处，
无数据模型变更、无新增依赖、无新增 CMDB 接口。

> **范围决策（2026-07-21，用户确认）**：集群名称（`set_name`）、模块名称（`module_name`）
> **不做 diff、不更新**。本期只同步集群环境类型（`environment`）与服务实例名称（`service_name`）。

## 技术上下文（Technical Context）

| 项 | 值 |
|----|----|
| 语言/运行时 | Go（仓库现有版本） |
| 主要改动文件 | `internal/processor/cmdb/sync_cmdb.go`（`BuildProcessChanges`） |
| 测试文件 | `internal/processor/cmdb/sync_cmdb_ostype_test.go` 同目录新增/扩展单测（复用 fake DAO 模式） |
| 数据模型 | 无变更（四字段已存在于 `ProcessSpec`，见 `data-model.md`） |
| 外部依赖 | 复用现有 `bkcmdb.Service` 客户端，不新增接口（FR-008） |
| 交付文档 | F-002 对比清单落 `docs/`（路径实现时确定） |
| 未决项 | 无技术阻塞；Q-001（排期）/Q-003（F-003 范围）非阻塞 |

无 NEEDS CLARIFICATION：Q-002 已澄清（直接覆盖），Q-001/Q-003 非阻塞（见 `questions.md`）。

## 合规门禁检查（Constitution Check）

本仓库无 `.specify/memory/constitution.md`，以 `AGENTS.md` Go 代码要求 + 工作区约束 + `.golangci.yml`
+ 安全红线为硬约束：

| 约束 | 是否满足 | 说明 |
|------|----------|------|
| 改动小、边界清晰、可验证（karpathy） | ✅ | 单函数扩展 + 单包单测 |
| 不引入不必要抽象/配置/兼容层 | ✅ | 四字段直接覆盖，无兜底层（Q-002 决策） |
| funlen(120)/gocyclo(30) | ✅ | `BuildProcessChanges` 已带 `nolint: funlen,gocyclo`，增量可控 |
| gofmt | ✅（实现期执行） | 修改后运行 `gofmt` |
| goheader（新文件 MIT 头） | ✅（实现期执行） | 新测试文件带标准头 |
| 优先补测试、可单包验证 | ✅ | `go test ./internal/processor/cmdb/` |
| 安全三大红线 | ✅ | 内部 diff 字段比较，无外部输入/鉴权/加密面变化 |
| 不新增 CMDB 接口/触发入口（FR-008） | ✅ | 仅扩展 diff 检测与写回 |

结论：无门禁违规。

## 项目结构（本需求触达）

```
internal/processor/cmdb/
  sync_cmdb.go                 # 改：BuildProcessChanges 变更检测 + 写回 + reusable 分支
  sync_cmdb_ostype_test.go     # 参考模式（fake DAO）
  <新增/扩展>_test.go          # 四字段增量更新表驱动单测
pkg/dal/table/process.go       # 只读：ProcessSpec 四字段定义（不改）
docs/                          # F-002 gsekit vs bscp 对比清单（实现期产出）
```

## Phase 0：技术调研（已完成，见 research.md）

要点：
1. F-001 根因 = `BuildProcessChanges` 早退守卫（L1633）未纳入四个拓扑字段。
2. 四字段在进程新增路径已从 `process_related_info` 正确填充，diff 侧只差「检测 + 写回」。
3. 四字段均为 `ProcessSpec` 已有 `string`，无 DDL。
4. gsekit `sync_biz_process` 以 `bulk_update` 直接覆盖（`bk_set_env` 单独 + 名称随 `expression`），
   作为 F-002 基准与 Q-002 空值语义依据。
5. TR-001：reusable 恢复分支需显式回填四字段。

## Phase 1：设计（见 data-model.md）

- 无表结构变更；覆盖语义为「直接以 CMDB 值覆盖」（含空）。
- 无对外接口契约变更（内部同步逻辑，无新增 API / CLI）。

## 实现方案（TDD 落地步骤）

### 改动点 1：`BuildProcessChanges` 变更检测扩展（F-001 / FR-001~004）

1. 新增拓扑变更标志（放在现有 5 个标志之后）：
   ```go
   topoChanged := newP.Spec.ServiceName != oldP.Spec.ServiceName ||
       newP.Spec.Environment != oldP.Spec.Environment
   ```
   → 仅比较服务实例名称与集群环境类型两个字段；`set_name`/`module_name` 不纳入（范围决策）。
2. 纳入早退守卫（L1633）：追加 `&& !topoChanged`。→ 满足 FR-004/AC-004（两字段不变不产生更新）。
3. 守卫通过后统一写回（仿 osType 写回位置，在 nameChanged 分支之前）：
   ```go
   if topoChanged {
       oldP.Spec.ServiceName = newP.Spec.ServiceName
       oldP.Spec.Environment = newP.Spec.Environment
   }
   ```
   → 覆盖主更新路径（infoChanged/numChanged）与「安全原地改别名」路径，收尾 `toUpdate` 使用 `oldP.Spec` 生效。
   直接覆盖含空值（FR-002 / Q-002），不加空值保护。不写回 `set_name`/`module_name`。

### 改动点 2：reusable 恢复分支同步刷新（F-005 / TR-001）

在 `reusableProc` 恢复块（L1667-1675 附近）显式回填两个字段：
```go
reusableProc.Spec.ServiceName = newP.Spec.ServiceName
reusableProc.Spec.Environment = newP.Spec.Environment
```
→ 别名 + 这两个字段同时变更并命中复用记录时不残留旧值。重建分支（`toAdd.Spec = newP.Spec`）已自然携带最新值，无需改动。

### 改动点 3：F-002 对比清单（FR-006）

基于 `research.md` 调研方向 4 的骨架，产出完整 gsekit vs bscp CMDB 同步能力对比清单到 `docs/`，
逐项标注 bscp 是否支持及差异点。

### 改动点 4：F-003 缺口补齐（FR-007）

以 F-002 结论 + 用户确认为准。当前已识别缺口即四个拓扑字段，已由改动点 1/2 覆盖；若 F-002 发现
额外缺口，需回需求侧补充范围并重估（Q-003 / spec.md Assumptions）。**不超出 gsekit 一键同步能力边界。**

## 测试策略（TDD）

单包验证：`go test ./internal/processor/cmdb/`，复用 `sync_cmdb_ostype_test.go` 的 fake DAO 模式
（`fakeReusableProcessDao` / `fakeEmptyInstanceDao` / `fakeReusableDaoSet` / `SyncContext` 构造）。

表驱动用例（先写测试，后改实现）：

| 用例 | 场景 | 断言 | 对应 |
|------|------|------|------|
| T1 | 仅 `service_name` 变更 | `ToUpdateProcess` 非空且值=新值 | FR-001/AC-002 |
| T2 | 仅 `environment` 变更 | 同上 | FR-001/FR-003/AC-001 |
| T3 | 仅 `set_name`/`module_name` 变更（环境类型/服务实例名不变） | `ToUpdateProcess` 为 nil，不触发更新 | FR-001a/AC-003 |
| T4 | 环境类型/服务实例名均不变（其余也不变） | `ToUpdateProcess` 为 nil，无更新 | FR-004/AC-004 |
| T5 | 两字段变更为空 | 覆盖写空 | FR-002/Q-002 |
| T6 | 别名 + 两字段同时变更且命中 reusable | 恢复进程两字段=新值 | FR-005/TR-001 |

端到端：以「一键同步」为入口人工/联调验证 AC-001~004（超出单测范围，验收阶段执行）。

## 需求覆盖对照（Traceability）

| 需求 | 覆盖手段 |
|------|----------|
| FR-001 | 改动点 1（检测+写回 service_name/environment）+ T1~T2 |
| FR-001a | `topoChanged` 不含 set_name/module_name + T3（变化不触发更新）|
| FR-002 | 改动点 1 直接覆盖语义 + T5 |
| FR-003 | `environment` 存 CMDB `bk_set_env` 原值，无映射 + T2 |
| FR-004 | 早退守卫纳入 `topoChanged` + T4 |
| FR-005 | 改动点 2（reusable 分支）+ T6 |
| FR-006 | 改动点 3（docs 对比清单）|
| FR-007 | 改动点 4（缺口补齐，范围以 F-002 为准）|
| FR-008 | 不新增接口/触发入口，仅扩展 diff |

## 风险与应对

| 风险 | 应对 |
|------|------|
| TR-001 reusable 分支残留旧值 | 改动点 2 显式回填 + T6 覆盖 |
| TR-002 空值语义被改判为保留旧值 | 当前直接覆盖（Q-002）；若改判再引入类 `resolveOsType` 兜底 |
| F-003 范围膨胀 | F-002 产出后若发现额外缺口，回需求侧补充范围并重估工时（Q-003）|

## 交付物清单

- 代码：`internal/processor/cmdb/sync_cmdb.go`（改动点 1/2）
- 测试：`internal/processor/cmdb/` 下四字段增量更新单测（T1~T6）
- 文档：`docs/` gsekit vs bscp CMDB 同步能力对比清单（F-002）
