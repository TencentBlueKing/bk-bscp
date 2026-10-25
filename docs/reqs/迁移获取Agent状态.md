# gsekit 数据迁移工具获取 agent 状态

## 基本信息

| 字段 | 值 |
|------|-----|
| 需求 ID | 1020451610135627475（短 ID 135627475） |
| 需求名称 | gsekit 数据迁移工具获取 agent 状态 |
| 优先级 | 待确认 |
| 父需求 | 无 |
| 创建时间 | 2026-06-29 19:19:57 |
| 原始需求文档 | docs/reqs/迁移获取Agent状态.md |
| 预估工时 | 16 人时（2 人天） |
| 价值规模 | 37.5（Reach=20, Impact=5, Confidence=75%, Effort=2 人天） |

> 工时与评分说明：
> - 全量工作 1 位高级工程师完成工时预估：16 人时（2 人天）
> - 全量工作 1 位中级工程师完成工时预估：约 22 人时（高级的 1.4 倍）
> - RICE 评分明细：Reach=20（迁移场景/特定模块，影响被迁移进程同步就绪）、
>   Impact=5（重要功能改进，消除迁移后同步空窗期，存在周期同步自愈兜底）、
>   Confidence=75%（需求清晰、方案明确，GSE 鉴权参数等技术细节待确认）、
>   Effort=2 人天；RICE=(20×5×0.75)/2=37.5，处于 🟢 低优先级区间。

## 需求背景

### 业务背景

`cmd/gsekit-migration/` 是把 GSEKit（蓝鲸进程配置管理，`bk-process-config-manager`）
的 MySQL 数据迁移到 BSCP 的独立命令行工具，按 `biz_id` 粒度迁移进程、进程实例、
配置模板/版本、配置实例等数据。

当前迁移逻辑在写入 `processes` 表（`cmd/gsekit-migration/migrator/process.go`
的 `INSERT INTO processes`）时**不写入 `agent_status` 字段**，迁移后该字段为空/
数据库默认值。

而 BSCP 的 GSE 进程状态同步（`internal/processor/gse/sync_gse.go` 的
`filterSyncableProcesses`）只处理 `agent_status == normal` 的进程：

- 迁移完成后，新迁移进程的 `agent_status` 不是 `normal`，会被 GSE 进程状态同步跳过；
- 必须等到下一次周期性 CMDB 同步（`internal/processor/cmdb/sync_cmdb.go` 的
  `buildProcessEntities` → `s.gseSvc.ListAgentState`）把 `agent_status` 填充为
  `normal` 后，进程才会被纳入 GSE 进程状态同步。

这导致迁移后存在一段"进程已迁移但无法参与进程状态同步"的空窗期，依赖周期任务的
触发时机。

### 用户故事

作为 GSEKit→BSCP 数据迁移的执行者
我想要 迁移工具在迁移进程时即获取并写入每个进程的 agent 状态
以便于 迁移完成后进程能立即被 GSE 进程状态同步纳入，无需等待下一次 CMDB 周期同步

### 需求来源

- **需求渠道**：技术优化（GSEKit→BSCP 迁移配套能力）
- **关联需求**：无
- **参考资料**：
  - `cmd/gsekit-migration/README.md`
  - `docs/reqs/进程状态同步修复.md`（说明 `agent_status` 由 CMDB 同步写入、GSE 同步据此过滤）

## 功能需求

### 核心功能点

| 功能编号 | 功能描述 | 优先级 | 涉及角色 | 备注 |
|---------|---------|--------|---------|------|
| F-001 | 迁移进程时调用 GSE 批量查询 agent 状态 | P0 | 迁移工具 | 必须 |
| F-002 | 按统一规则将 agent 状态映射并写入 `processes.agent_status` | P0 | 迁移工具 | 必须 |
| F-003 | 新增 GSE API 接入配置（必填） | P0 | 迁移工具 | 必须 |
| F-004 | GSE 查询失败时不阻断迁移，按 normal 兜底并告警 | P0 | 迁移工具 | 必须 |

### 详细功能描述

#### [F-001] 迁移进程时获取 agent 状态

- **输入**：当前迁移批次内进程对应的 `bk_agent_id` 列表（来源 `gsekit_process.bk_agent_id`）。
- **处理逻辑**：
  1. 收集当前批次内非空的 `bk_agent_id`；
  2. 调用 GSE `list_agent_state` 接口（`internal/components/gse` 的
     `ListAgentState`）批量查询 agent 状态；
  3. 构建 `agent_id → status_code` 映射，供 F-002 使用。
- **输出**：批次内各 agent 的状态码映射。
- **边界条件**：
  - `bk_agent_id` 为空的进程不参与查询；
  - 批次内无任何非空 `bk_agent_id` 时跳过查询。
- **异常处理**：见 F-004。

#### [F-002] 映射并写入 agent_status

- **输入**：F-001 得到的 `agent_id → status_code` 映射、当前批次进程。
- **处理逻辑**：对每个进程按以下规则确定 `agent_status`，并写入 `processes.agent_status`：
  - GSE `status_code == 2`（运行中）→ `normal`；
  - 其余状态码 / 无 `agent_id` / 未命中查询结果 → `abnormal`；
  - GSE 查询整体失败时的兜底处理见 F-004（兜底 `normal`）。
- **输出**：`processes` 表记录的 `agent_status` 字段被正确赋值。
- **边界条件**：映射规则与运行时 `buildProcessEntities` 保持完全一致，避免迁移值与
  周期同步值产生口径差异。

#### [F-003] 新增 GSE API 接入配置

- **输入**：`migration.yaml` 配置文件。
- **处理逻辑**：
  1. 在配置结构（`cmd/gsekit-migration/config/config.go`）中新增独立 `gse` 配置块，
     包含访问 GSE 网关所需字段（如 `endpoint`、`app_code`、`app_secret` 等，按
     `internal/components/gse` 客户端实际需要的鉴权参数确定）；
  2. `gse` 配置为**必填项**：缺失或不完整时，配置校验阶段直接报错并终止。
- **输出**：迁移工具具备调用 GSE `list_agent_state` 的能力。

#### [F-004] GSE 查询失败处理

- **输入**：GSE 查询过程中的接口报错/超时。
- **处理逻辑**：查询失败**不阻断迁移**；失败覆盖到的进程 `agent_status` 兜底写
  `normal`，并记录告警日志（含失败原因与受影响 agent 数量）。
- **兜底为 `normal` 的理由**：生产环境机器的 agent 正常情况下均处于运行中；若失败时
  兜底为 `abnormal`，会导致迁移后用户页面出现大量"agent 异常"误报、引发用户恐慌。
  迁移属一次性快照，后续周期性 CMDB 同步会自动纠正为真实状态。
- **输出**：迁移继续进行；后续 BSCP 周期性 CMDB 同步会自动纠正 `agent_status`。
- **说明**：F-003 的"配置必填"是启动期约束（连接信息必须配置），与本条"运行期单次
  查询失败兜底不中断"属于不同层面，不冲突。
- **与运行时差异**：运行时 `buildProcessEntities` 查询失败兜底为 `abnormal`，本迁移
  工具有意选择兜底 `normal`（避免误报）；该差异为快照值差异，由周期同步收敛。

## 非功能需求

### 性能需求

- agent 状态查询按迁移批次（`migration.batch_size`，默认 500）随进程迁移流程批量
  进行，不显著增加迁移整体耗时（待确认是否需要额外的查询并发/分批上限）。

### 安全需求

- GSE `app_secret` 等鉴权信息仅存放于迁移工具配置文件，不落入仓库文档与日志。

### 可用性与稳定性

- 单次 GSE 查询失败不影响整体迁移成功；通过 normal 兜底 + 周期同步纠正保证最终一致。

### 兼容性

- 映射规则与运行时 CMDB 同步一致，迁移写入值与后续周期同步值口径统一。

## 业务规则

### 业务逻辑规则

- **规则 R-001**：`agent_status` 取值仅 `normal` / `abnormal`（`pkg/dal/table/process.go`
  中 `AgentStatus` 约束）。
- **规则 R-002**：仅 GSE `status_code == 2` 映射为 `normal`，其余一律 `abnormal`。
- **规则 R-003**：迁移阶段写入的 `agent_status` 为快照值，后续以 BSCP 周期性 CMDB
  同步为准。

### 数据校验规则

- **必填配置**：`gse` 配置块（含 GSE 网关地址与鉴权信息）。

## 外部依赖与集成

### 外部系统集成

| 系统名称 | 交互方式 | 接口说明 | 认证方式 | 文档链接 |
|---------|---------|---------|---------|---------|
| GSE | HTTP POST | `/api/v2/cluster/list_agent_state` 批量查询 agent 状态 | 蓝鲸网关鉴权（待确认具体参数） | 待确认 |

### 接口契约

- 请求：`{ "agent_id_list": ["<bk_agent_id>", ...] }`
- 响应数据项（`ListAgentStateData`）关键字段：
  - `bk_agent_id`：Agent ID
  - `status_code`：Agent 运行状态码（-1 未知 / 0 初始安装 / 1 启动中 / 2 运行中 /
    3 有损 / 4 繁忙 / 5 升级中 / 6 停止中 / 7 解除安装）

### 数据模型

- 写入目标：`processes.agent_status`（`varchar`，取值 `normal` / `abnormal`）。

## 验收标准

### 功能验收

- [ ] **AC-001**：Given 已正确配置 `gse` 配置块且 GSE 可用，When 执行迁移，Then 每个
  迁移进程的 `processes.agent_status` 按规则写入（`status_code==2` 的进程为 `normal`，
  其余为 `abnormal`）。
- [ ] **AC-002**：Given 迁移进程的 agent 处于运行中（`status_code==2`），When 迁移完成，
  Then 该进程在不依赖周期 CMDB 同步的情况下即可被 GSE 进程状态同步
  （`filterSyncableProcesses`）纳入处理。
- [ ] **AC-003**：Given 进程的 `bk_agent_id` 为空或未在 GSE 查询结果中命中，When 迁移
  完成，Then 该进程 `agent_status` 为 `abnormal`。
- [ ] **AC-004**：Given GSE `list_agent_state` 查询失败或超时，When 迁移执行，Then 迁移
  不中断，受影响进程 `agent_status` 兜底为 `normal`，并输出告警日志。
- [ ] **AC-005**：Given 未配置或配置不完整的 `gse` 配置块，When 启动迁移，Then 配置
  校验阶段报错并终止，提示缺失的 GSE 配置。

### 性能验收

- [ ] **AC-P01**：迁移整体耗时相比未接入 agent 状态查询前无显著恶化（具体阈值待确认）。

## 边界范围

### 本期包含

- 迁移工具在进程迁移阶段获取并写入 `agent_status`（F-001~F-004）。
- 新增 GSE 接入配置（必填）。
- 映射规则与运行时 CMDB 同步对齐。

### 本期不包含

- 修改运行时 CMDB / GSE 周期同步逻辑（`buildProcessEntities`、`sync_gse.go` 等）。
- 进程实例（`process_instances`）状态相关迁移逻辑的改动。
- GSEKit `compare-render` 等其他迁移子命令的改动。

## 约束条件

- **技术限制**：复用 `internal/components/gse` 的 `ListAgentState` 能力，不重复实现
  GSE 客户端；映射规则须与 `buildProcessEntities` 保持一致。
- **数据一致性**：迁移写入值为快照，最终以周期 CMDB 同步为准。

## 未解决问题

| 问题 ID | 问题描述 | 截止日期 | 状态 |
|--------|---------|---------|------|
| Q-001 | GSE 网关具体鉴权参数（app_code/app_secret 是否复用 CMDB 同一套、是否需要额外票据）需确认 | 待确认 | 待确认 |
| Q-002 | 是否需要为 GSE 查询设置额外的并发/分批上限及超时（超大 biz 场景） | 待确认 | 待确认 |
| Q-003 | 性能验收 AC-P01 的具体阈值 | 待确认 | 待确认 |

---

## 原需求描述

> (无描述内容)

## 澄清记录

### 第 1 轮澄清

**提问时间**：2026-06-29 19:27

**Agent 提问与用户回复**：

1. 核心目标 → 让迁移工具在迁移进程时调用 GSE 查询 agent 状态并写入 `agent_status`，
   使迁移完成后进程立即可参与 GSE 进程状态同步，无需等周期任务。
2. GSE 接入方式 → 在 `migration.yaml` 新增独立 `gse` 配置块（endpoint、app_code、
   app_secret 等），复用 `internal/components/gse` 的 `ListAgentState`。
3. 映射规则 → 与运行时 `buildProcessEntities` 完全一致：`status_code==2`→`normal`，
   其余/无 agent_id/未命中→`abnormal`。
4. 失败处理 → 查询失败不阻断迁移并告警，后续 CMDB 周期同步纠正。
   （补充修订：兜底值由 `abnormal` 调整为 `normal`，避免迁移后用户页面大量 agent
   异常误报；生产机器 agent 正常情况下均为运行中。）
5. 开关 → 始终开启，`gse` 配置作为必填项。
