# Data Model — Story 136320799

## 结论：无表结构变更（无 DDL）

本需求不涉及数据模型变更。四个拓扑字段均已存在于 `pkg/dal/table/process.go` 的 `ProcessSpec`
（L98-101，均为 `string`），仅补齐其在增量同步中的「检测 + 写回」行为。

## 涉及实体

### 进程主记录 `ProcessSpec`（bscp `processes` 表）

本需求关注字段（均已存在，类型 `string`，无变更）：

| bscp 字段 | gorm 列 | 语义 | CMDB 来源 | 取值约束 |
|-----------|---------|------|-----------|----------|
| `ServiceName` | `service_name` | 服务实例名称 | 服务实例 `name` | 直接覆盖（含空） |
| `Environment` | `environment` | 集群环境类型 | `bk_set_env` | "1"测试 / "2"体验 / "3"正式（与 CMDB 一致，FR-003） |
| `SetName` | `set_name` | 集群名称 | `bk_set_name` | 直接覆盖（含空） |
| `ModuleName` | `module_name` | 模块名称 | `bk_module_name` | 直接覆盖（含空） |

**填充路径（已有，不改动）**：`buildProcessEntities`（sync_cmdb.go L261-266）、
`buildProcessesFromSets`（L1169-1172）从 `bkcmdb.ProcessRelatedInfoItem` 解析填充 new 进程 Spec。

### CMDB 进程相关信息 `process_related_info`（数据源，只读）

权威数据源，bscp 已有客户端 `bkcmdb.Service`，本需求不新增接口（FR-008）。结构见
`internal/processor/cmdb/cc_topo_types.go`：`ProcessSetInfo{BkSetName,BkSetEnv}` /
`ProcessModuleInfo{BkModuleName}` / `ProcessServiceInstInfo{Name}`。

## 状态/覆盖语义

- 覆盖语义：**直接以 CMDB 值覆盖**，含 CMDB 侧为空时覆盖为空（FR-002 / Q-002），无空值保护分支。
- 与 `os_type`/`agent_status` 的区别：后者来自独立且可能失败的接口（list_hosts / gse），
  故有 `resolveOsType` / `agentStatusChanged != ""` 空值保护；四字段同源于 `process_related_info`
  权威数据，不套用空值保护（否则违背「与 CMDB 保持一致」的 AC-001~003）。

## F-002 / F-003 数据交付物（非表结构）

- F-002：gsekit vs bscp CMDB 同步能力对比清单（Markdown），落 `docs/`（路径实现时确定），
  骨架见 `research.md` 调研方向 4。
- F-003：以 F-002 结论 + 用户确认为准；当前已识别缺口即四个拓扑字段，落在 F-001 修复范围内。
