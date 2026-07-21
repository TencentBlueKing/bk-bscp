# Research — Story 136320799：拓扑字段同步 CMDB 状态

> 输入：`specs/stories/136320799/spec.md`、`req.md`、`questions.md`
> 白名单：`specs/stories/136320799/context.md`（仅在其列出的文件范围内调研）
> 目标：为 F-001（四字段增量同步修复）、F-002（gsekit vs bscp 对比清单）、F-003（缺口补齐）沉淀技术事实与决策。

## 调研方向 1：bscp 现有进程 diff 逻辑与变更检测早退守卫

**结论（Decision）**：F-001 根因是 `BuildProcessChanges` 的变更检测未覆盖四个拓扑字段，且早退守卫在「仅四字段变化」时直接返回空结果，导致进程不进入待更新集合。

**关键代码位置**（`internal/processor/cmdb/sync_cmdb.go`）：

- `BuildProcessChanges`（L1616 起，函数已带 `nolint: funlen,gocyclo`）当前仅计算 5 个变更标志：
  - `nameChanged := newP.Spec.Alias != oldP.Spec.Alias`（L1627）
  - `infoChanged := !equal`（`SourceData` 比较，L1628）
  - `numChanged := newP.Spec.ProcNum != oldP.Spec.ProcNum`（L1629）
  - `agentStatusChanged := newP.Spec.AgentStatus != "" && ...`（L1630，带空值保护）
  - `osTypeChanged := newP.Spec.OsType != "" && ...`（L1631，带空值保护）
- **早退守卫**（L1633）：
  `if !nameChanged && !infoChanged && !numChanged && !osTypeChanged && !agentStatusChanged { return result, nil }`
  → 四个拓扑字段（`ServiceName/Environment/SetName/ModuleName`）不在判断内，仅这些字段变化时进程被判为"无变更"直接返回，**印证 req.md「现状说明」**。
- 写回落点：`osTypeChanged`/`agentStatusChanged` 命中后立即写回 `oldP.Spec.*`（L1637-1643），最终在收尾 `toUpdate := &table.Process{... Spec: oldP.Spec ...}`（L1812-1819）汇总为 `ToUpdateProcess`。四字段写回可复用这一模式。

**当前覆盖字段**：Alias（`nameChanged`）/ SourceData（`infoChanged`）/ ProcNum（`numChanged`）/ AgentStatus / OsType。
**未覆盖字段**：`service_name` / `environment` / `set_name` / `module_name`（本需求补齐）。

**链路覆盖（R-002 核对）**：一键同步（`SyncBizProcesses` / `SyncSingleBiz`）与定时全量同步均经 `SyncProcessData → diffProcesses → BuildProcessChanges` 全量新旧对比；修复 `BuildProcessChanges` 一处，三条链路一并受益。CMDB watch 进程属性事件链路 `UpdateProcess` 基于事件构建 newSpec、事件不携带 set/module 拓扑信息，其拓扑刷新由全量一键同步兜底，符合 spec.md 边界。

**Alternatives considered**：在拉取/构建层（`buildProcessEntities`）特判——否决，diff 单点扩展改动最小、三链路统一受益、边界清晰（技术决策记录一致）。

## 调研方向 2：四个拓扑字段在 CMDB 拓扑数据中的来源与解析路径

**结论**：四字段在进程「新增」路径已从 `process_related_info` 正确解析填充，diff 侧只差「检测 + 写回」，无需改动 CMDB 拉取层。

- `buildProcessEntities`（`sync_cmdb.go` L261-266）从 `bkcmdb.ProcessRelatedInfoItem` 填充 new 进程 Spec：
  - `SetName: item.Set.BkSetName`
  - `ModuleName: item.Module.BkModuleName`
  - `ServiceName: item.ServiceInstance.Name`
  - `Environment: item.Set.BkSetEnv`
- `buildProcessesFromSets`（L1169-1172）另一条构建路径同样填充 `set.Name / mod.Name / svc.Name / set.SetEnv`。
- 拓扑结构体定义见 `internal/processor/cmdb/cc_topo_types.go` / 解析见 `cc_topo.go`（`ProcessSetInfo.BkSetName/BkSetEnv`、`ProcessModuleInfo.BkModuleName`、`ProcessServiceInstInfo.Name`）。

**含义**：new 进程 Spec 已携带 CMDB 最新四字段值；缺陷完全落在 diff 的「检测 + 写回」环节。

## 调研方向 3：进程表字段定义（ProcessSpec）

**结论**：四字段均已存在于数据模型，均为 `string`，无需 DDL / 表结构变更。

`pkg/dal/table/process.go`（`ProcessSpec`，L98-101）：

| 字段 | 定义 | 语义 | CMDB 来源 |
|------|------|------|-----------|
| `SetName` | `gorm:"column:set_name" json:"set_name"` string | 集群名称 | `bk_set_name` |
| `ModuleName` | `gorm:"column:module_name" json:"module_name"` string | 模块名称 | `bk_module_name` |
| `ServiceName` | `gorm:"column:service_name" json:"service_name"` string | 服务实例名称 | 服务实例 `name` |
| `Environment` | `gorm:"column:environment" json:"environment"` string | 环境类型（"1"/"2"/"3"） | `bk_set_env` |

`Environment` 以字符串存 CMDB `bk_set_env` 原值（1 测试 / 2 体验 / 3 正式），FR-003 取值范围与 CMDB 一致，无需额外映射（区别于 `os_type` 的 `mapOsType` 数字→语义转换）。

## 调研方向 4：gsekit 对标（F-002 对比清单 / F-003 缺口判定依据）

**结论**：gsekit `sync_biz_process` 在一键同步中对已存在进程执行 `bulk_update`，直接用 CMDB 值覆盖，无空值保护——作为 F-002 对比基准与 Q-002 空值语义依据。

`bk-process-config-manager/apps/gsekit/process/handlers/process.py` `sync_biz_process`（L472-548）：

- 从 `CCApi.list_process_related_info` 拉全量进程；按 `bk_process_id` 是否已存在分 `to_be_created` / `to_be_updated` / `to_be_deleted`。
- `bulk_update` 更新字段（L529-533）：`["bk_set_id", "bk_set_env", "bk_module_id", "bk_process_name", "expression"]`。
  - `bk_set_env` 单独字段直接覆盖 → 对应 bscp `environment`。
  - `expression` 由 `{bk_set_name}{splitter}{bk_module_name}{splitter}{service_instance_name}{splitter}{bk_process_name}{splitter}{bk_process_id}`（L507-515）拼成 → 集群/模块/服务实例名称随 `expression` 一并覆盖，对应 bscp `set_name/module_name/service_name`。
  - 覆盖语义为「直接以 CMDB 值覆盖」，无空值保留分支。
- `solution_maker.py` `SyncProcessSolutionMaker`（L71-79）：动作「同步CMDB进程配置」，仅为跳转提示，不含字段同步逻辑，佐证同步实体在 `sync_biz_process`。

**gsekit vs bscp 差异（F-002 清单骨架，实现期落 `docs/`）**：

| 对比维度 | gsekit `sync_biz_process` | bscp（现状） | bscp 是否支持 |
|----------|---------------------------|--------------|---------------|
| 进程新增/删除 | to_be_created / to_be_deleted | 新增/删除分支 | 支持 |
| 别名/进程名 | `bk_process_name`（bulk_update） | `nameChanged`(Alias) | 支持 |
| 进程属性/SourceData | 进程属性另有链路 | `infoChanged`(SourceData) | 支持 |
| 实例扩缩容 | `create_process_inst` | `numChanged` + reconcile | 支持 |
| 集群环境类型 `bk_set_env` | bulk_update 直接覆盖 | **未检测**（本需求补齐） | ✗→补 |
| 集群名称 `bk_set_name` | 随 expression 覆盖 | **未检测**（本需求补齐） | ✗→补 |
| 模块名称 `bk_module_name` | 随 expression 覆盖 | **未检测**（本需求补齐） | ✗→补 |
| 服务实例名称 `name` | 随 expression 覆盖 | **未检测**（本需求补齐） | ✗→补 |
| os_type / agent 状态 | 无（gsekit 不覆盖此项） | bscp 已支持（额外能力） | bscp 领先 |

> F-002 完整清单在实现期产出到 `docs/`（路径实现时确定）；F-003 缺口范围以此清单结论 + 用户确认为准（Q-003 非阻塞），当前已识别的缺口即四个拓扑字段，均落在 F-001 修复范围内。

## 调研方向 5：别名变更复用/恢复分支（TR-001）需同步刷新拓扑字段

**结论**：`nameChanged` 命中且存在可复用 deleted 记录（`reusableProc`）的恢复分支（L1665-1712）逐字段回填 `reusableProc.Spec.*`，但**未回填四个拓扑字段**，会残留旧值；修复需在该分支显式写入 `newP.Spec` 的四字段。

- 恢复分支现状：更新 `NewAlias/PrevData/SourceData/CcSyncStatus/ProcNum/OsType` 与 `Attachment`（L1667-1675），返回 `reusableProc` 作为 `ToUpdateProcess`。四个拓扑字段沿用 `reusableProc` 自身旧值 → TR-001 风险点。
- 重建分支（`!safe` 无可复用记录，L1715-1758）：`toAdd.Spec = newP.Spec`，已携带 CMDB 最新四字段，无需额外处理。
- 主更新路径（infoChanged / numChanged / 安全原地改别名）：收尾用 `oldP.Spec`，只要在早退守卫后统一写回四字段即可覆盖。

**修复落点小结**（范围决策 2026-07-21：仅 `ServiceName` + `Environment` 两个字段，`SetName`/`ModuleName` 不做 diff/不写回）：
1. 新增 `topoChanged` 标志（`ServiceName || Environment`）并纳入早退守卫；
2. 守卫后统一 `if topoChanged { oldP.Spec.ServiceName = newP.Spec.ServiceName; oldP.Spec.Environment = newP.Spec.Environment }`（覆盖主更新路径与安全改别名路径）；
3. 恢复分支内显式把 `reusableProc.Spec.ServiceName`/`Environment` 写为 `newP.Spec.*`（覆盖 TR-001）。

## 测试策略调研

**结论**：复用 `sync_cmdb_ostype_test.go` 的 fake DAO 单测模式，对 `BuildProcessChanges` 做表驱动单测，单包可验证（`go test ./internal/processor/cmdb/`）。

- 现成 fake 组件：`fakeReusableProcessDao`（`GetByCcProcessIDAndAliasTx`）/ `fakeEmptyInstanceDao`（`ListByProcessIDTx` 恒空）/ `fakeReusableDaoSet`，`SyncContext{Kit, Dao, Now, HostCounter, ModuleCounter}` 构造范式已有（见 `TestBuildProcessChangesReusableResolvesOsType`）。
- 覆盖用例：①仅 `service_name` 变更 →`ToUpdateProcess` 非空且值更新；②仅 `environment` 变更；③仅 `set_name`/`module_name` 变更；④四字段均不变 → 无更新（AC-004 / FR-004）；⑤四字段变更为空 → 覆盖写空（FR-002 / Q-002）；⑥别名 + 拓扑字段同时变更且命中 reusable → 恢复进程四字段刷新为新值（TR-001 / FR-005）。
- 符合 AGENTS.md「能用单包测试验证的不只依赖全量编译」。

## 合规与约束核对（AGENTS.md / .golangci.yml / 安全红线）

- **funlen**（120 行）/ **gocyclo**（30）：`BuildProcessChanges` 已带 `nolint: funlen,gocyclo`，四字段检测新增行数与圈复杂度可控，不新增违规。
- **goheader**：新增测试文件需带仓库标准 MIT 头（YEAR=20\d\d）。
- **gofmt**：修改后运行 `gofmt`（AGENTS.md 硬约束）。
- **安全红线**：本需求为内部 CMDB 同步 diff 的字段比较与写回，无新增外部输入入口、无鉴权/加密面变化，不触达三大红线；不新增 CMDB 接口调用（FR-008）。
- **不引入不必要抽象**：不新增配置项/兼容层；四字段直接覆盖（无 `resolveOsType` 式兜底），保持改动最小（Q-002 决策）。
