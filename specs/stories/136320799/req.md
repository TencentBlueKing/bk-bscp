# 【进程配置管理】集群环境类型、服务实例名同步 cmdb 状态

## 基本信息

| 字段 | 值 |
|------|-----|
| 需求 ID | 1020451610136320799 |
| 短 ID | 136320799 |
| 需求名称 | 【进程配置管理】集群环境类型、服务实例名同步 cmdb 状态 |
| 优先级 | 待确认 |
| 价值规模 | 50（Reach=20, Impact=10, Confidence=75%, Effort=3 人天） |
| 预估工时 | 24 人时 |
| 父需求 | 无 |
| 迭代 ID | 1020451610002264573 |
| 创建时间 | 2026-07-20 17:42:44 |
| 原始需求文档 | docs/reqs/进程配置字段同步.md |

## 需求背景

### 业务背景

bscp 的「进程配置管理」模块提供「一键同步状态」能力，从 CMDB 拉取业务下的进程拓扑并落库，
供用户在进程列表中查看集群、模块、服务实例、进程等信息。该能力对标蓝鲸已有产品 gsekit 的
「一键状态同步」。

当前存在的问题：用户在 CMDB 中修改了服务实例名称或集群环境类型后，在 bscp 点击「一键同步」，
进程列表中的这两个字段仍保持旧值，无法同步更新。对照 gsekit，这些字段在其一键同步中是能够
随 CMDB 变化而更新的，形成了 bscp 与 gsekit 的能力差距。

除该显性问题外，需要系统性对比 gsekit 同步 CMDB 数据的能力范围，梳理 bscp 尚不支持或存在
差距的同步项，形成对比清单，并在本期补齐已确认的缺口。

### 用户故事

作为 进程配置管理的使用者
我想要 在 CMDB 修改集群环境类型、服务实例名称、集群名称、模块名称后，通过 bscp 一键同步让
进程列表中的这些字段同步为最新值
以便于 bscp 展示的进程拓扑信息与 CMDB 保持一致，不再出现陈旧数据

作为 产品/研发负责人
我想要 得到一份 gsekit 与 bscp 在 CMDB 同步能力上的差异对比清单
以便于 明确 bscp 还有哪些同步项不支持，规划后续补齐

### 需求来源

- **需求渠道**：用户反馈 / 产品对标
- **对标对象**：蓝鲸 gsekit（`bk-process-config-manager`）的一键状态同步
- **参考资料**：
  - gsekit 同步入口：`bk-process-config-manager/apps/gsekit/process/handlers/process.py` 的 `sync_biz_process()`
  - gsekit 一键同步 API：`flush_process`（`apps/gsekit/process/views/process.py`）
  - bscp 一键同步入口：`internal/processor/cmdb/sync_cmdb.go` 的 `SyncProcessData` / `diffProcesses` / `BuildProcessChanges`

## 功能需求

### 核心功能点

| 功能编号 | 功能描述 | 优先级 | 涉及角色 | 备注 |
|---------|---------|--------|---------|------|
| F-001 | 一键同步时，CMDB 已存在进程的「服务实例名称、集群环境类型、集群名称、模块名称」变更能同步更新到 bscp | P0 | 进程配置使用者 | 修复现有缺陷 |
| F-002 | 输出 gsekit 与 bscp 在 CMDB 同步能力上的差异对比清单（Markdown） | P0 | 研发/产品 | 调研产出 |
| F-003 | 对差异清单中已确认的缺口进行本期补齐 | P1 | 研发 | 缺口范围以 F-002 结论为准 |

### 详细功能描述

#### [F-001] 拓扑展示字段的增量同步更新

- **输入**：用户在进程配置管理页点击「一键同步状态」；此前 CMDB 中对应进程所属集群/模块/服务实例的
  以下字段发生了变化：
  - 服务实例名称（CMDB 服务实例 `name`，bscp 字段 `service_name`）
  - 集群环境类型（CMDB `bk_set_env`，bscp 字段 `environment`；取值 1:测试 / 2:体验 / 3:正式）
  - 集群名称（CMDB `bk_set_name`，bscp 字段 `set_name`）
  - 模块名称（CMDB `bk_module_name`，bscp 字段 `module_name`）
- **处理逻辑**：
  1. 同步拉取 CMDB 最新进程相关信息
  2. 对 CMDB 中已存在（进程主键匹配）的进程记录，识别上述四个字段的变化
  3. 将变化后的值更新到 bscp 进程记录
- **输出**：进程列表中上述四个字段展示为 CMDB 最新值
- **边界条件**：
  - 仅涉及这四个拓扑展示字段的更新，不改变进程实例扩缩容、进程状态、别名/进程属性等既有同步行为
  - 字段值为空时的处理方式：待确认（见未解决问题 Q-002）
- **异常处理**：
  - 同步过程中单条记录处理失败 → 沿用现有一键同步的错误处理与任务结果反馈机制（不在本需求新增机制）

> 现状说明（供实现参考，非新增设计）：现有增量更新的变更检测只覆盖别名、进程属性、实例数、
> os_type、agent 状态，未包含服务实例名称/集群环境类型/集群名称/模块名称，因此这些字段仅在
> 进程首次新增时写入、后续不再刷新。

> **范围更新（2026-07-21，用户确认）**：F-001 本期只同步**集群环境类型（`environment`）**与
> **服务实例名称（`service_name`）**两个字段；**集群名称（`set_name`）、模块名称（`module_name`）
> 不做 diff、不更新**（虽 gsekit 会随 expression 刷新这两项，bscp 本期有意不对齐，如需再作独立需求评估）。

#### [F-002] gsekit vs bscp CMDB 同步能力对比清单

- **输入**：对 gsekit 与 bscp 两侧 CMDB 同步实现的梳理
- **处理逻辑**：逐项对比两侧在「同步哪些实体/字段」「新增与增量更新分别覆盖哪些字段」上的差异
- **输出**：一份 Markdown 对比清单，逐项标注 bscp 是否支持、与 gsekit 的差异点
- **产出位置**：`docs/` 下（具体路径实现时确定）

#### [F-003] 已确认缺口补齐

- **输入**：F-002 对比清单中标注为「bscp 缺失且需补齐」的项
- **处理逻辑**：对齐 gsekit 一键同步的行为进行补齐
- **输出**：缺口项在 bscp 一键同步中生效
- **边界**：补齐范围以 F-002 结论 + 用户确认为准，避免超出对标范围引入额外能力

## 业务规则

### 对标基准

- **规则 R-001**：以 gsekit 一键同步（`sync_biz_process`）的字段同步行为作为对标基准。
  经观测，gsekit 在一键同步中支持同步服务实例名称、集群名称、模块名称与集群环境类型
  （其中集群/模块/服务实例名称随实例表达式 `expression` 一并更新，集群环境类型 `bk_set_env` 单独更新）。
- **规则 R-002**：bscp 一键同步、定时全量同步、CMDB watch 增量同步三条链路共用同一套进程 diff 逻辑；
  本需求以「一键同步」为验收入口，diff 逻辑修复后其余两条链路一并受益，不为其单独设计。

### 数据校验规则

- 集群环境类型取值范围：`1`（测试）/ `2`（体验）/ `3`（正式），与 CMDB `bk_set_env` 一致

## 外部依赖与集成

### 外部系统集成

| 系统名称 | 交互方式 | 接口说明 | 备注 |
|---------|---------|---------|------|
| 蓝鲸 CMDB | HTTP（APIGW） | 进程相关信息聚合查询（`process_related_info`）等 | bscp 已有客户端，本需求不新增接口 |

### 数据模型（涉及字段）

进程主记录（bscp `processes` 表 / `ProcessSpec`）中本需求关注的字段：

| bscp 字段 | 含义 | CMDB 来源 |
|-----------|------|-----------|
| `service_name` | 服务实例名称 | 服务实例 `name` |
| `environment` | 集群环境类型（1/2/3） | `bk_set_env` |
| `set_name` | 集群名称 | `bk_set_name` |
| `module_name` | 模块名称 | `bk_module_name` |

## 验收标准

### 功能验收

- [ ] **AC-001**：Given CMDB 中某进程所属集群的环境类型由「测试」改为「正式」
  When 用户在 bscp 进程配置管理页点击「一键同步状态」并等待同步完成
  Then 该进程在 bscp 进程列表中的集群环境类型展示为「正式」，与 CMDB 当前值一致
- [ ] **AC-002**：Given CMDB 中某进程所属服务实例名称被修改
  When 用户点击「一键同步状态」并等待同步完成
  Then 该进程在 bscp 进程列表中的服务实例名称展示为修改后的名称
- [ ] **AC-003**：Given CMDB 中某进程所属集群名称、模块名称被修改
  When 用户点击「一键同步状态」并等待同步完成
  Then 该进程在 bscp 进程列表中的集群名称、模块名称展示为修改后的值
- [ ] **AC-004**：Given 上述四个字段均未变化
  When 用户点击「一键同步状态」
  Then 不产生无意义的更新，其余同步行为（实例扩缩容、进程状态、别名/进程属性等）与现状一致
- [ ] **AC-005**：Given 已完成两侧梳理
  When 查看交付物
  Then 存在一份 gsekit vs bscp CMDB 同步能力对比清单，逐项标注 bscp 是否支持及差异
- [ ] **AC-006**：Given 对比清单中标注为「需补齐」的缺口
  When 完成本期开发
  Then 这些缺口在 bscp 一键同步中生效，并可通过对应用例验证

## 边界范围

### 本期包含

- 一键同步时服务实例名称、集群环境类型、集群名称、模块名称的增量更新（F-001）
- gsekit 与 bscp CMDB 同步能力对比清单（F-002）
- 对比清单中已确认缺口的补齐（F-003，范围以对比结论为准）

### 本期不包含

- 进程状态、进程实例扩缩容、别名、进程属性等既有同步逻辑的改造（除非属于 F-003 确认缺口）
- 新增 CMDB 接口调用或新的同步触发入口
- gsekit 侧任何改动

## 约束条件

- **技术限制**：复用 bscp 现有 CMDB 同步链路与客户端，不引入新的同步机制或抽象
- **对标限制**：补齐范围不超出 gsekit 一键同步的能力边界

## 人力与工时

- 全量工作 1 位高级工程师完成工时预估：24 人时（3 人天）
- 全量工作 1 位中级工程师完成工时预估：约 34 人时（高级工程师的 1.4 倍）

工时拆解（供参考）：
- F-001 四字段增量同步修复（含单元测试）：约 12 人时
- F-002 gsekit vs bscp 同步能力对比清单：约 6 人时
- F-003 已确认缺口补齐（假设除四字段外缺口有限）：约 6 人时

## RICE 评分明细

| 参数 | 值 | 说明 |
|------|-----|------|
| Reach | 20 | 影响进程配置管理模块的特定使用者（少部分用户 / 特定模块） |
| Impact | 10 | 核心为修复一键同步字段不更新的缺陷（解决 Bug） |
| Confidence | 75% | F-001 根因明确、方案清晰；F-003 缺口范围与空值策略（Q-002/Q-003）待确认 |
| Effort | 3 人天 | 24 人时 ÷ 8 |
| **RICE Score** | **50** | 🟡 中：正常排期，按迭代计划推进 |

> Effort 中 F-003 范围存在不确定性，若对比清单发现较多额外缺口，需重新评估工时与 RICE。

## 未解决问题

| 问题 ID | 问题描述 | 状态 |
|--------|---------|------|
| Q-001 | 需求优先级（High/Middle/Low）未在 TAPD 标注，需产品确认 | 非阻塞（产品排期决策，见技术澄清） |
| Q-002 | 当 CMDB 侧字段被清空（如服务实例名称为空）时，bscp 是覆盖为空还是保留旧值？ | 已澄清（技术结论：直接覆盖，对齐 gsekit，见技术澄清） |
| Q-003 | F-003 缺口补齐的具体范围，需在 F-002 对比清单产出后再次确认 | 非阻塞（待 F-002 产出后确认） |

---

## 原需求描述

> 需要对比 gsekit 的一键状态同步

## 澄清记录

### 第 1 轮澄清

**提问时间**：2026-07-21 20:40

**补充背景（用户）**：
- gsekit 同步进程配置可以同步到集群的环境类型和更新服务实例名称；bscp 点击一键同步后依然无法
  更新服务实例名称和集群环境类型。
- 需要评估 gsekit 在同步 CMDB 数据时，bscp 还有哪些是不支持的。
- 参考代码：`bk-process-config-manager/apps/gsekit/utils/solution_maker.py` 第 71-79 行
  （`SyncProcessSolutionMaker`，动作「同步CMDB进程配置」）。

**Agent 提问与用户回复**：

1. 需求整体范围？
   - 回复：两部分都做——修复一键同步无法更新「服务实例名称/集群环境类型」的 bug；
     系统评估 gsekit 同步 CMDB 的能力，梳理 bscp 缺口并本期补齐。
2. 「修复同步更新」需覆盖哪些字段？
   - 回复：服务实例名称 + 集群环境类型 + 集群名称 + 模块名称（拓扑展示类字段全覆盖）。
3. 字段更新需在哪些同步链路生效？
   - 回复：对比 gsekit，目前观测 gsekit 也只是在一键同步中支持同步实例名称和集群环境类型；
     以对齐 gsekit、保证一键同步为准（三条链路共用 diff 逻辑，修复后一并受益）。
4. 差异评估的产出形态？
   - 回复：一份 gsekit vs bscp CMDB 同步能力对比清单（Markdown）。
5. 验收基准？
   - 回复：一键同步后，bscp 进程列表相关字段与 CMDB 当前值实时一致。

---

## 技术澄清

> 澄清日期：2026-07-21
> 需求复杂度：中等（CMDB 同步 diff 逻辑修改 + 跨系统对标调研）
> 澄清轮次：1

### 技术审查结论

- **技术可行性**：✅ 可行
- **技术风险等级**：低
- **审查说明**：F-001 根因明确——现有进程 diff 变更检测未覆盖四个拓扑展示字段；仓库已有完全同构的先例（os_type 增量同步：`mapOsType`/`buildHostOsTypeIndex`/`resolveOsType` + `BuildProcessChanges` 变更检测），可直接复用同样的实现与单测模式，无新技术、无架构变更。

### 技术方案概述

- **实现方式**：在进程 diff 变更检测中新增对「服务实例名称 `service_name`、集群环境类型 `environment`、集群名称 `set_name`、模块名称 `module_name`」四个字段的比较；命中变化时把新值写回旧进程 Spec 并纳入待更新集合。
- **根因定位**：`internal/processor/cmdb/sync_cmdb.go` 的 `BuildProcessChanges`（L1610-1820）当前仅检测 `nameChanged`(Alias)/`infoChanged`(SourceData)/`numChanged`(ProcNum)/`agentStatusChanged`/`osTypeChanged`。当仅这四个拓扑字段变化时，早退守卫（L1633：`if !nameChanged && !infoChanged && !numChanged && !osTypeChanged && !agentStatusChanged { return result, nil }`）直接返回空结果 → 进程不进入 `ToUpdateProcesses`，因此这四个字段仅在进程首次新增（`buildProcessEntities`/`buildProcessesFromSets`）时写入、后续不再刷新，与 req.md「现状说明」一致。
- **改动点**：
  1. `BuildProcessChanges` 新增四字段变更标志，加入早退守卫的判断条件；
  2. 命中变化时将 `oldP.Spec.SetName/ModuleName/ServiceName/Environment` 更新为 `newP.Spec.*`（收尾处已有 `toUpdate := &table.Process{... Spec: oldP.Spec ...}` 汇总为 `ToUpdateProcess`，L1812-1819，沿用即可）。
- **数据来源确认**：四字段均已在 `buildProcessEntities`（L262-265）/`buildProcessesFromSets`（L1169-1172）从 `process_related_info` 的 `set.BkSetName`/`module.BkModuleName`/`serviceInstance.Name`/`set.BkSetEnv` 正确填充到 new 进程，diff 侧只差「检测 + 写回」，无需改动 CMDB 拉取层。
- **技术选型**：无新增依赖，复用现有 CMDB 同步链路与客户端。

### 架构影响

- **新增组件**：无。
- **变更组件**：仅 `internal/processor/cmdb/sync_cmdb.go` 的 `BuildProcessChanges`（可能顺带补一个空值语义辅助点，见风险 TR-002）。
- **数据模型变更**：无。四字段（`set_name`/`module_name`/`service_name`/`environment`）均已存在于 `pkg/dal/table/process.go` 的 `ProcessSpec`（L98-101，均为 `string` 类型，`environment` 存 "1"/"2"/"3"），无 DDL。
- **向后兼容性**：兼容。仅补齐已存在字段的更新行为，不改变新增/删除/扩缩容/别名/进程属性等既有语义。
- **链路覆盖（R-002 核对）**：一键同步（新模式 `SyncBizProcesses` / 旧模式 `SyncSingleBiz`）、定时全量同步均经 `SyncProcessData → diffProcesses → BuildProcessChanges` 走全量新旧对比，修复后一并生效；以「一键同步」为验收入口成立。注：CMDB watch 进程属性事件链路 `UpdateProcess`（L983-）基于事件构建 newSpec，事件不携带 set/module 拓扑信息，其拓扑字段刷新仍由全量一键同步兜底，符合 req.md 边界。

### 外部依赖

| 依赖项 | 类型 | 状态 | 接口文档 | 备注 |
|--------|------|------|---------|------|
| 蓝鲸 CMDB `list_process_related_info` | HTTP(APIGW) | ✅ 已确认 | bscp 已有客户端 `bkcmdb.Service` | 本需求不新增接口调用，四字段已在返回体内 |

### 技术风险

| 风险 ID | 风险描述 | 影响 | 概率 | 应对措施 |
|---------|---------|------|------|---------|
| TR-001 | 别名变更且复用 deleted 记录（`reusableProc`）分支（L1665-1712）恢复进程时，仅回填部分字段，未刷新四个拓扑字段，可能残留旧值 | 低 | 低 | 该分支 `reusableProc.Attachment = newP.Attachment` 已更新归属；实现时同步把四个拓扑字段写为 `newP.Spec.*` 值，并补边界用例覆盖「别名+拓扑字段同时变更」场景 |
| TR-002 | 空值语义若被产品改判为「空值保留旧值」，需新增类 `resolveOsType` 的兜底逻辑 | 低 | 低 | 当前结论为直接覆盖（见 Q-002/技术决策）；实现保持简单，若后续改判再引入兜底 |

### 技术决策记录

| 决策 | 选择方案 | 备选方案 | 选择理由 |
|------|---------|---------|---------|
| 四字段空值处理（Q-002） | 直接以 CMDB 值覆盖（含覆盖为空） | 空值保留旧值 | 对齐 gsekit 对标基准 R-001；四字段与别名/进程属性同源于 `process_related_info`（权威、不会因独立接口失败而误判为空），无需 os_type/agent_status 那类空值保护 |
| 变更检测落点 | 在 `BuildProcessChanges` 统一 diff 处扩展 | 在拉取/构建层特判 | 复用现有 diff 单点，三条链路一并受益，改动最小、边界清晰 |

### 测试策略

- **单元测试**：直接对 `BuildProcessChanges` 编写表驱动用例（复用 `sync_cmdb_ostype_test.go` 的 fake DAO 模式：`fakeReusableDaoSet`/`fakeEmptyInstanceDao`），覆盖：①仅 `service_name` 变更→进入 `ToUpdateProcess` 且值更新；②仅 `environment`(bk_set_env) 变更；③仅 `set_name`/`module_name` 变更；④四字段均不变→不产生更新（对应 AC-004）；⑤四字段变更值为空→按覆盖语义写空（对应 Q-002 结论）；⑥别名+拓扑字段同时变更（覆盖 TR-001）。
- **集成测试**：可选，依赖 DB 与 CMDB mock；核心逻辑已可单包单测验证，遵循仓库「能用单包测试验证的不只依赖全量编译」约束。
- **端到端测试**：以「一键同步」为入口，人工/联调验证 AC-001~004。
- **测试数据**：构造新旧 `table.Process` 对，Spec 仅差目标字段。

### 补充的验收标准

- [ ] **AC-T01**：Given 旧进程与 CMDB 新进程仅 `service_name`/`environment`/`set_name`/`module_name` 之一或多者不同 When 执行 `BuildProcessChanges` Then 返回的 `ToUpdateProcess` 非空且对应字段等于 CMDB 新值。
- [ ] **AC-T02**：Given CMDB 侧某字段变为空 When 执行同步 Then 该字段按「直接覆盖」写为空（对齐 Q-002 结论）。

### 待解决问题

| 问题 ID | 问题描述 | 负责人 | 截止日期 | 状态 |
|---------|---------|--------|---------|------|
| Q-001 | 需求优先级需产品在 TAPD 标注 | 产品 | - | ⚠️ 非阻塞（产品决策，见 questions.md Q1[dropped]） |
| Q-002 | 四字段空值处理策略 | - | - | ✅ 已澄清：直接覆盖（questions.md Q2[resolved_by_doc]） |
| Q-003 | F-003 缺口补齐范围 | - | - | ⚠️ 非阻塞：待 F-002 产出后确认（questions.md Q3[resolved_by_doc]） |
