# 实现 ip 选择器，提供 ip 查询接口

## 基本信息

| 字段 | 值 |
|------|-----|
| 需求 ID | 1020451610135756883（短 ID：135756883） |
| 需求名称 | 实现 ip 选择器，提供 ip 查询接口 |
| 优先级 | High |
| 父需求 | 1020451610135732990（进程配置管理插件优化） |
| 创建时间 | 2026-07-02 20:46:28 |
| 原始需求文档 | docs/reqs/进程IP查询接口.md |
| 预估工时 | 16 人时（2 人天） |
| 价值规模 | 80（Reach=20, Impact=8, Confidence=100%, Effort=2 人天） |

> 评估结论：需求内聚（单个后端 inner 接口，2 个同源用户故事，大量复用现有 `ExpressionScope`/`ListProcess`），无需拆分。
> RICE 评分明细：
> - Reach=20（内部工具类，服务使用进程配置管理插件的运维/编排用户）
> - Impact=8（High 优先级，进程配置管理能力向 BSCP 收敛闭环的一环）
> - Confidence=100%（文档完善、方案明确、已核对两侧源码）
> - Effort=2 人天（16 人时 ÷ 8）
> - RICE = (20 × 8 × 1.0) / 2 = 80，属🟡中优先级，正常排期。

## 需求背景

### 业务背景

进程配置管理能力目前通过「标准运维（bk-sops）插件 + BSCP 后端」的方式对外提供。父需求（进程配置管理插件优化）要求 BSCP 侧能力逐步对齐 gsekit，替换掉早期临时方案。

标准运维上有一个**内置的、由 gsekit 实现的「IP 选择器」变量插件**：用户在变量表单里填写集群 / 模块 / 服务实例 / 进程名等条件后，插件在 `get_value()` 中据此构造 `expression_scope`，调用 **gsekit 的进程状态查询接口 `process/process_status`**，从返回的进程列表中提取内网 IP，最终渲染成一个 IP 变量供后续流程节点使用。

现状问题：该变量插件的数据来源是 gsekit。随着进程配置管理能力迁移到 BSCP，需要让这个 IP 选择器变量**改为调用 BSCP 提供的接口**获取 IP。为此，BSCP 需要提供一个过滤语义与 gsekit `process_status` 对齐的 IP 查询接口。经核对变量插件源码，其仅使用返回中的内网 IP，故 BSCP 接口简化为**直接返回去重后的内网 IP 列表**，使标准运维侧的变量插件可以平滑切换数据源。

不做的影响：变量插件继续依赖 gsekit，无法完成进程配置管理能力向 BSCP 的收敛，父需求的迁移目标无法闭环。

### 用户故事

作为 标准运维流程的编排者（使用「IP 选择器」变量的用户）
我想要 填写集群 / 模块 / 服务实例 / 进程名等条件后，由变量插件自动过滤出符合条件的主机内网 IP
以便于 在后续流程节点中以 IP 变量的形式使用这批机器，而无需手工维护 IP 列表

作为 标准运维「GSEKit IP 选择器」变量插件的维护方
我想要 BSCP 提供一个过滤语义对齐 gsekit `process/process_status`、直接返回去重 IP 列表的接口
以便于 将变量插件的数据来源从 gsekit 切换到 BSCP，改动量最小

### 需求来源

- **需求渠道**：技术优化（进程配置管理能力向 BSCP 收敛）
- **关联需求**：父需求 1020451610135732990（进程配置管理插件优化）
- **参考实现**：gsekit（bk-process-config-manager）`process/process_status` 接口；标准运维 bk-sops 内置 GSEKit IP 选择器变量插件
- **本仓库已有基础**：`pkg/protocol/core/process/process.proto`（`ProcessSearchCondition` 已含 `inner_ips`、`expression_scope`，`Process.spec.inner_ip` 已存在）、`internal/dal/dao/process.go`（`handleSearch` 已实现按表达式范围与 IP 过滤）、`cmd/config-server/service/process.go`（已有 `ListProcess`）

## 功能需求

### 核心功能点

| 功能编号 | 功能描述 | 优先级 | 涉及角色 | 备注 |
|---------|---------|--------|---------|------|
| F-001 | 提供 IP 查询接口，按 `expression_scope`（环境 + 集群/模块/服务实例/进程名/进程ID 表达式）过滤命中进程，直接返回去重后的内网 IP 列表 | P0 | SOPS 变量插件 | 过滤语义对齐 gsekit `process_status`，返回结构简化为 IP 列表 |
| F-002 | 接口以 inner（内部调用、不经用户认证）方式暴露，供标准运维变量插件调用 | P0 | SOPS 变量插件 | 对齐父需求 TODO：补齐 inner 接口 |
| F-003 | 支持一次性返回全部命中结果（不依赖分页），满足变量插件"过滤出全部 IP"的诉求 | P0 | SOPS 变量插件 | 对齐 gsekit 使用方式 |

### 详细功能描述

#### [F-001] IP 查询接口（过滤语义对齐 gsekit process_status）

- **输入**：业务 ID + 过滤范围。过滤范围对齐 gsekit `process_status` 的 `expression_scope`：
  - 环境类型（对应 gsekit `bk_set_env`：1 测试 / 2 体验 / 3 正式）
  - 集群名表达式、模块名表达式、服务实例名表达式、进程别名表达式、进程 ID 表达式（缺省均为 `*`，语义与 gsekit 表达式一致：通配符、枚举、范围、切片等）
- **处理逻辑**：
  1. 校验业务 ID 与过滤参数（环境类型在表达式范围下必填，对齐现有实现约束）。
  2. 基于 BSCP **本地已同步的进程数据**（`processes` 表），按 `expression_scope` 匹配出命中的进程集合（复用现有 `ExpressionScope` 匹配逻辑）。
  3. 从命中进程集合中提取内网 IP，**去重**后返回。
- **输出**：**去重后的内网 IP 列表**（而非进程对象列表）。
  - 简化说明：经核对 bk-sops 侧「GSEKit IP 选择器」变量插件源码，其仅从 `process_status` 返回中取每条进程的 `bk_host_innerip` 后逗号拼接，其它进程字段（cloud_id、拓扑、进程状态等）全部丢弃。由于本需求本就要修改该变量插件改调 BSCP，因此 BSCP 接口直接返回 IP 列表即可，无需暴露完整进程对象；变量插件相应改为直接消费该 IP 列表（拼接逻辑不变）。
- **数据来源**：BSCP 本地进程数据（对齐 gsekit 查本地进程表，不实时回源 CMDB）。
- **边界条件**：
  - 表达式命中为空 → 返回空列表（不降级为全选）。
  - 对内网 IP **去重**：同一主机命中多个进程时该 IP 只返回一次（这是相对 gsekit 现状的行为改进——gsekit 侧不去重、逗号串中会重复出现）。
- **异常处理**：
  - 业务 ID 为空 / 非法 → 返回参数错误。
  - 表达式范围下环境类型缺失 → 返回参数错误（对齐现有 `environment is required for expression scope` 约束）。

#### [F-002] inner 接口暴露

- 接口以 inner（内部调用、不走用户认证）方式提供，供标准运维变量插件服务端调用。
- 对齐父需求既定 TODO：ListProcess 早期临时以网关方式暴露并临时关闭用户认证，后续补齐 inner 接口；本需求即落地"以 inner 接口方式提供进程 / IP 查询"。

#### [F-003] 全量返回

- 一次性返回全部命中 IP，不分页、不要求调用方翻页。
- 说明：gsekit `process_status` 自身按 `page`/`pagesize` 分页（默认 10、最大 1000），变量插件通过传大 `pagesize` / 翻页拿全量；BSCP 侧直接全量返回 IP 列表，简化变量插件的调用。

## 非功能需求

### 性能需求

- **响应时间**：单业务全量进程查询在正常数据规模下 P95 ≤ 2s（待确认：需结合业务最大进程规模复核指标）。
- **数据规模**：需支持单业务万级进程记录的过滤与返回（待确认具体上限）。

### 安全需求

- **调用方式**：inner 接口仅供内部服务（标准运维变量插件后端）调用，不面向终端用户开放；不经用户身份认证，依赖内部调用鉴权 / 网络隔离（具体鉴权方式对齐 BSCP 现有 inner 接口规范）。
- **数据范围**：查询严格限定在入参 `biz_id` 对应业务范围内，不跨业务返回数据。

### 兼容性

- **过滤语义兼容**：过滤条件（`expression_scope`）语义对齐 gsekit `process/process_status`，确保变量插件切换数据源后命中的机器集合不变。
- **调用方改造**：BSCP 直接返回 IP 列表，bk-sops 侧「GSEKit IP 选择器」变量插件需相应从"取进程对象再提取 `bk_host_innerip`"改为"直接消费 BSCP 返回的 IP 列表"（改动小，且与切换 client 同步进行）。
- **本仓库兼容**：优先复用已有 `ProcessSearchCondition` / `ExpressionScope` 协议与过滤实现，避免新增重复的过滤逻辑。

## 业务规则

### 业务逻辑规则

- **规则 R-001**：过滤语义以 gsekit `process_status` 的 `expression_scope` 为准（五段表达式 + 环境类型），与 BSCP 已落地的进程表达式过滤保持一致。
- **规则 R-002**：数据来源为 BSCP 本地进程数据；本需求不引入实时回源 CMDB 的主机查询。
- **规则 R-003**：BSCP 接口返回**去重后的内网 IP 列表**；是否携带管控区域前缀（`cloud_id:ip`）、逗号拼接等最终 IP 变量渲染由标准运维变量插件负责。经核对，现有变量插件不做去重、不加 `cloud_id` 前缀，仅逗号拼接 `bk_host_innerip`；BSCP 侧去重为行为改进。

### 数据校验规则

- **必填字段**：`biz_id`；在表达式范围模式下 `environment`（环境类型）必填。
- **取值范围**：环境类型取值 1 / 2 / 3（测试 / 体验 / 正式），对齐 gsekit `bk_set_env`。

## 外部依赖与集成

### 外部系统集成

| 系统名称 | 交互方式 | 接口说明 | 认证方式 | 备注 |
|---------|---------|---------|---------|------|
| 标准运维 bk-sops（GSEKit IP 选择器变量插件） | HTTP（调用方） | 变量插件 `get_value()` 构造 `expression_scope` 后调用本接口获取去重 IP 列表 | inner 内部调用（不走用户认证） | 迁移前调用 gsekit `process/process_status` 再自行提取 IP，迁移后改调本接口直接拿 IP 列表 |
| gsekit（bk-process-config-manager） | 参考基准 | `POST /api/{bk_biz_id}/process/process_status/`，`expression_scope` 过滤，返回 `data.list` 进程对象数组（IP 在 `bk_host_innerip`） | —— | 仅作为过滤语义对齐基准，不在运行期依赖 |

### 接口契约

#### 过滤条件（对齐 gsekit process_status 的 expression_scope）

```json
{
  "expression_scope": {
    "bk_set_env": "3",
    "bk_set_name": "[管控平台, PaaS平台]",
    "bk_module_name": "*",
    "service_instance_name": "*",
    "bk_process_name": "*",
    "bk_process_id": "*"
  }
}
```

> 字段命名可沿用 BSCP 现有 `ExpressionScope` 协议（`set_name`/`module_name`/`service_name`/`process_alias`/`process_id` + `environment`），语义与上述 gsekit 表达式一一对应。

#### BSCP 响应（简化为去重 IP 列表）

```json
{
  "ips": ["127.0.0.1", "127.0.0.2"]
}
```

> BSCP 直接返回去重后的内网 IP 列表，不返回完整进程对象。字段名以最终协议为准。BSCP 现有 `ListProcess`（`POST /api/v1/config/biz_id/{bizId}/process/list`）已支持 `expression_scope` 过滤且进程对象含 `spec.inner_ip`，可作为过滤与取值的实现基础。

#### bk-sops 变量插件适配（对照）

现状（迁移前，`var_gse_kit_ip_selector.py`）：

```python
process_status_result = client.process_status(bk_biz_id=bk_biz_id, expression_scope=expression_scope_kwargs)
ip_list = [p["bk_host_innerip"] for p in process_status_result]
ip_str = ",".join(ip_list)
```

迁移后（改调 BSCP，直接拿 IP 列表）：调用 BSCP IP 查询接口得到 `ips`，`ip_str = ",".join(ips)`。

### 数据模型

核心数据来源为 BSCP `processes` 表（`pkg/dal/table/process.go`）。过滤与取值涉及的关键字段：

| 字段 | 说明 |
|------|------|
| `inner_ip` | 主机内网 IP（接口返回值，去重） |
| `set_name` / `module_name` / `service_name` / `alias` | 拓扑 / 进程别名（`expression_scope` 过滤命中依据） |
| `environment` | 环境类型（过滤命中依据） |

## 验收标准

### 功能验收

- [ ] **AC-001**：Given 某业务下存在若干已同步进程记录，When 以对齐 gsekit 的 `expression_scope`（环境 + 集群/模块/服务实例/进程名/进程ID 表达式）调用本接口，Then 命中的进程集合与 gsekit `process_status` 在相同过滤条件下命中的进程集合一致（进程集合等价）。
- [ ] **AC-002**：Given 上述命中进程，When 接口返回 IP，Then 返回的 IP 集合与 gsekit 命中进程的 `bk_host_innerip` 去重后集合一致。
- [ ] **AC-003**：Given 同一主机命中多个进程，When 调用本接口，Then 该主机内网 IP 在返回列表中只出现一次（去重）。
- [ ] **AC-004**：Given 表达式命中为空，When 调用本接口，Then 返回空 IP 列表，不返回该业务全部 IP。
- [ ] **AC-005**：Given 表达式范围模式下未传环境类型，When 调用本接口，Then 返回参数错误提示。
- [ ] **AC-006**：Given 命中 IP 数超过单页默认条数，When 调用本接口，Then 一次性返回全部去重 IP，无需调用方翻页。
- [ ] **AC-007**：Given 传入某业务 ID，When 调用本接口，Then 返回结果不包含其他业务的 IP。

### 性能验收

- [ ] **AC-P01**：单业务全量进程查询在约定数据规模下 P95 ≤ 2s（指标待确认）。

### 安全验收

- [ ] **AC-S01**：本接口以 inner 方式暴露，终端用户无法直接经公开网关调用；内部调用鉴权对齐 BSCP 现有 inner 接口规范。

## 边界范围

### 本期包含

- 提供过滤语义对齐 gsekit `process_status` 的 IP 查询 inner 接口（本地数据源、表达式范围过滤、去重、全量返回 IP 列表）。
- 返回去重后的内网 IP 列表，满足标准运维「GSEKit IP 选择器」变量插件切换数据源的消费需求。

### 本期不包含

- BSCP 侧不新增 IP 选择器 UI 组件（IP 选择器是标准运维侧的变量插件，本需求只提供后端接口）。
- 不支持 IPv6。
- 不做基于 GSE Agent 在线 / 异常状态的 IP 筛选。
- 不接入 CMDB 动态分组。
- 不实时回源 CMDB 查询主机列表（仅用本地已同步进程数据）。
- 接口自身不做分页浏览（以全量返回满足诉求）。

## 约束条件

- **技术限制**：Go 实现需符合仓库 `.golangci.yml`；优先复用现有 `ProcessSearchCondition` / `ExpressionScope` 协议与过滤实现，不引入重复的过滤逻辑或不必要的抽象。
- **一致性约束**：过滤语义（`expression_scope`）必须以 gsekit `process_status` 为对齐基准，确保标准运维变量插件切换数据源后命中的机器集合不变；返回结构简化为去重 IP 列表，需同步改造 bk-sops 侧变量插件的取值逻辑。

## 未解决问题

| 问题 ID | 问题描述 | 状态 |
|--------|---------|------|
| Q-001 | 变量插件解析逻辑已核对（`var_gse_kit_ip_selector.py`：仅取 `bk_host_innerip` 逗号拼接，不去重、不加 cloud 前缀）；据此确定 BSCP 直接返回去重 IP 列表 | 已确认 |
| Q-002 | 是否直接复用并扩展现有 `ListProcess` 暴露为 inner 接口，还是新增独立 inner 接口，属实现方案，待技术设计阶段确定 | 待确认 |
| Q-003 | 性能指标（P95、单业务最大进程规模）需结合真实业务数据规模确认 | 待确认 |
| Q-004 | 去重为相对 gsekit 现状的行为改进（gsekit 侧不去重）；如需严格对齐现状不去重，请在技术设计阶段调整 | 待确认 |

---

## 原需求描述

> 参考 gsekit 提供 ip 查询接口
>
> 实现 ip 选择器，提供 ip 查询接口

## 澄清记录

### 第 1 轮澄清

**Agent 提问（要点）**：
1. IP 选择器的载体 / 使用场景（插件表单 / BSCP Web 前端 / 仅后端接口）
2. IP 查询接口数据来源（本地已同步数据 / 实时 CMDB / 两者结合）
3. 是否按拓扑 / 环境级联过滤候选 IP
4. IP 选择方式（下拉多选 / 手动输入粘贴 / 两者）
5. 返回展示字段范围
6. 是否分页 + 模糊搜索
7. 接口暴露层级与认证（网关 / inner）
8. 本期不做的范围（IPv6 / agent 状态 / 动态分组）

**用户回复（要点）**：
1. 载体是**标准运维内置、由 gsekit 实现的 IP 选择器变量插件**：它当前调用 gsekit 接口获取 IP，需要调整为使用 BSCP 的接口。BSCP 侧只负责提供接口。
2. 数据来源：**本地已同步的进程数据**。
3. 级联 / 过滤：**对齐 gsekit 的逻辑即可**（用户输入集群 / 模块 / 服务实例后过滤出 IP）。
4. 选择方式：变量插件由用户输入集群 / 模块 / 服务实例后调接口获得 IP，最终渲染成过滤出来的 IP；**这里只需对齐 gsekit 返回 IP**。
5. 返回字段：**对齐 gsekit**。
6. 分页：**对齐 gsekit，不需要分页，返回全部 IP**。
7. 暴露层级：**inner 接口**。
8. 本期不做：**不支持 IPv6**。

### 第 2 轮澄清

**Agent 提问（要点）**：
1. SOPS IP 选择器变量当前实际调用的 gsekit 接口是哪一个（对齐基准）
2. BSCP 新接口的返回格式（纯 IP 数组 / `cloud_id:ip` / 进程对象列表 / 以 SOPS 变量消费格式为准）

**用户回复（要点）**：
1. GSEKit IP 选择器调用的是 gsekit 的进程状态查询接口 `process/process_status`；变量在 `get_value()` 中构造 `expression_scope`（集群/模块/服务实例/进程名等表达式），然后调用 `client.process_status(...)`。
2. 返回格式**以 SOPS 变量插件实际消费格式为准**（Agent 复核确认：变量插件从 `process_status` 返回的进程列表中提取内网 IP，故 BSCP 接口返回对齐 `process_status` 的进程列表即可，最终 IP 渲染由变量插件负责）。

### 第 3 轮澄清

**Agent 提问（要点）**：
1. 核对 `var_gse_kit_ip_selector.py` 后确认：变量插件仅取 `bk_host_innerip` 逗号拼接（不去重、不加 cloud 前缀），其余进程字段丢弃；BSCP 是否可简化为直接返回 IP 列表？

**用户回复（要点）**：
1. 认可简化方案：BSCP 直接返回（去重后的）内网 IP 列表，不再返回完整进程对象；bk-sops 侧变量插件相应改为直接消费 BSCP 返回的 IP 列表。
