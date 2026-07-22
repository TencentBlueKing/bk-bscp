# gsekit vs bscp CMDB 同步能力对比清单

> 需求：#136320799 【进程配置管理】集群环境类型、服务实例名同步 cmdb 状态（F-002 交付物）
> 目的：逐项对比 gsekit 与 bscp 在「一键同步 CMDB 进程数据」上的能力，标注 bscp 是否支持、
> 与 gsekit 的差异点，作为 F-003 缺口补齐的范围判定依据。
> 结论基于源码核对（含文件:行号），非推测。

## 1. 对标范围与入口

| 项 | gsekit | bscp |
|----|--------|------|
| 一键同步入口 | `ProcessHandler.sync_biz_process`（`apps/gsekit/process/handlers/process.py:472`） | `SyncProcessData → diffProcesses → BuildProcessChanges`（`internal/processor/cmdb/sync_cmdb.go:1959 / :1394 / :1616`） |
| 数据源 | `CCApi.list_process_related_info`（`process.py:485`） | `bkcmdb` 客户端 `process_related_info`（`sync_cmdb.go` `buildProcessEntities:261`） |
| 存储模型 | `Process`（`apps/gsekit/process/models.py:21`） | `processes` 表 / `ProcessSpec`（`pkg/dal/table/process.go:97`） |
| 触发链路 | 一键同步 + 周期任务（`periodic_tasks/sync_process.py`） | 一键同步 + 定时全量同步 + CMDB watch 增量（三链路共用 `BuildProcessChanges`，R-002） |

## 2. 名称字段的存储与展示差异（理解对比的前提）

两侧对「集群名称/模块名称/服务实例名称」的**存储模型不同**，这是解读对比的关键：

- **gsekit**：不单独存 `bk_set_name / bk_module_name / service_instance_name` 三列，而是拼进
  `expression` 一列（`process.py:507-515`，格式
  `{bk_set_name}|{bk_module_name}|{service_instance_name}|{bk_process_name}|{bk_process_id}`）。
  另单独存 `bk_set_env`（集群环境类型）、`bk_set_id`、`bk_module_id`（`models.py:45/50/51/52`）。
  页面展示集群名/模块名/服务实例名时，**从 `expression` 解析**（`process_expression_to_name:69-77`、
  `fill_topo_name_to_process:81-90`）。
- **bscp**：`service_name / environment / set_name / module_name` 是 `ProcessSpec` 上的**四个独立列**
  （`process.go:98-101`），页面直接读这四列展示。

> 含义：gsekit「刷新名称」= 每次同步 `bulk_update` 覆盖 `expression`；bscp「刷新名称」= 必须在
> diff 时覆盖四个独立列。gsekit 支持更新集群名称/模块名称/服务实例名称，是通过刷新 `expression`
> 间接实现的。

## 3. 逐字段同步能力对比（已存在进程的增量更新）

> gsekit 已存在进程的增量更新字段集见 `process.py:529-533` `bulk_update(fields=[...])`：
> `["bk_set_id", "bk_set_env", "bk_module_id", "bk_process_name", "expression"]`。
> bscp 增量更新的变更检测见 `sync_cmdb.go:1627-1633`（早退守卫）。

| # | 展示/业务字段 | CMDB 来源 | gsekit 同步（一键同步内） | bscp 同步（一键同步内） | bscp 是否支持 | 差异结论 |
|---|--------------|-----------|--------------------------|------------------------|--------------|---------|
| 1 | 集群环境类型 | `bk_set_env` | ✅ `bulk_update` 直接覆盖 `bk_set_env` | ❌ 未检测（`environment` 不在早退守卫） | ✗ | **bscp 缺失，本期补齐（F-001）** |
| 2 | 集群名称 | `bk_set_name` | ✅ 随 `expression` 覆盖刷新 | ❌ 未检测（`set_name` 不在早退守卫） | ✗ | 有差距，**本期有意不补齐**（范围决策 2026-07-21）|
| 3 | 模块名称 | `bk_module_name` | ✅ 随 `expression` 覆盖刷新 | ❌ 未检测（`module_name` 不在早退守卫） | ✗ | 有差距，**本期有意不补齐**（范围决策 2026-07-21）|
| 4 | 服务实例名称 | 服务实例 `name` | ✅ 随 `expression` 覆盖刷新 | ❌ 未检测（`service_name` 不在早退守卫） | ✗ | **bscp 缺失，本期补齐（F-001）** |
| 5 | 进程别名/进程名 | `bk_process_name` | ✅ `bulk_update` 覆盖 `bk_process_name` | ✅ `nameChanged`（`Alias`，`sync_cmdb.go:1627`） | ✓ | 一致 |
| 6 | 进程属性/配置数据 | 进程属性 | ⚠️ gsekit 无等价「进程属性快照」概念 | ✅ `infoChanged`（`SourceData` 比较，:1628） | ✓ | bscp 额外能力 |
| 7 | 进程实例数（扩缩容） | 进程 `proc_num` | ✅ 另经 `create_process_inst`（:546） | ✅ `numChanged` + `reconcileProcessInstances`（:1629/1775） | ✓ | 一致（实现路径不同） |
| 8 | 系统类型 os_type | 主机 os_type | ❌ gsekit 不存/不更此列 | ✅ `osTypeChanged`（:1631，带空值保护） | ✓ | **bscp 领先** |
| 9 | agent 状态 | GSE agent | ⚠️ 另经 `sync_biz_process_status`（:1155）独立链路 | ✅ `agentStatusChanged`（:1630） | ✓ | 一致（都单独处理） |
| 10 | 进程新增 / 删除 | 进程存在性 | ✅ `bulk_create` / `delete`（:528/534） | ✅ 新增/删除分支（`diffProcesses`） | ✓ | 一致 |
| 11 | 集群/模块归属（set_id/module_id） | `bk_set_id` / `bk_module_id` | ✅ `bulk_update` 覆盖 | ⚠️ bscp 以 `Attachment.ModuleID` 关联，跨模块迁移场景另议 | 部分 | 见 §5 备注，非本期范围 |

## 4. 结论：bscp 相对 gsekit 的缺口

- **本期补齐（F-001/F-003）**：集群环境类型、服务实例名称两个字段——gsekit 在一键同步中会随
  `bk_set_env` / `expression` 刷新，bscp 当前不刷新。这两项即本期需补齐的**全部缺口**，已落在
  F-001 修复范围内，无需额外任务。
- **有差距但本期有意不补齐（范围决策 2026-07-21，用户确认）**：集群名称（`bk_set_name`）、模块名称
  （`bk_module_name`）——gsekit 随 `expression` 刷新，bscp 不刷新。经用户确认，这两个字段
  **本期不做 diff、不更新**（`BuildProcessChanges` 的 `topoChanged` 不纳入 set_name/module_name）。
  如后续需要与 gsekit 完全对齐，作为独立需求评估。
- **bscp 不落后、甚至领先的项**：进程别名、进程属性快照（SourceData）、实例扩缩容、os_type、
  agent 状态、进程新增/删除——bscp 均已支持，其中 os_type/进程属性快照为 bscp 额外能力。
- **超出对标范围、本期不做**：集群/模块归属（set_id/module_id）在进程跨模块迁移时的处理（§5）。

## 5. 备注：未纳入本期的差异项

- **跨模块迁移（set_id/module_id 变更）**：gsekit 对已存在进程 `bulk_update` 覆盖 `bk_set_id`/
  `bk_module_id`；bscp 进程以 `CcProcessID` + `Attachment.ModuleID` 关联，进程在 CMDB 中改变
  集群/模块归属属于「拓扑结构变更」而非「展示名称变更」，与本需求「四个展示字段刷新」的目标不同，
  且改动面更大、风险更高。**建议不在本期处理**；若确有诉求，作为独立需求评估（对应 req.md Q-003
  的范围守卫）。
- **agent 状态 / 进程运行状态**：两侧都走独立链路，且 bscp 已支持，不属于缺口。

## 6. 对 F-001/F-003 的直接指引

1. F-001 修复**集群环境类型、服务实例名称**两个字段的增量更新；集群名称/模块名称本期不做
   （范围决策 2026-07-21）。
2. 覆盖语义与 gsekit 一致：`bulk_update` / 覆盖 `expression` 均为**直接以 CMDB 值覆盖**（无空值保护），
   故 bscp 采用「直接覆盖含空值」（对应 req.md Q-002 结论）。
3. F-003 无需在这两个字段之外新增补齐项；集群名称/模块名称、集群/模块归属迁移均不在本期范围。
