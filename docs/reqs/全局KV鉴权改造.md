# 移除全局配置 KV 管理权限点

## 基本信息

| 字段 | 值 |
|------|-----|
| 需求 ID | 1020451610135532948（短 ID：135532948） |
| 需求名称 | 移除全局配置 KV 管理权限点 |
| 优先级 | Low |
| 父需求 | 无 |
| 创建时间 | 2026-06-25 16:08:56 |
| 原始需求文档 | docs/reqs/全局KV鉴权改造.md |
| 预估工时 | 20 人时（2.5 人天） |
| 价值规模 | 3（RICE：Reach=5, Impact=2, Confidence=75%, Effort=2.5 人天） |

## 需求评估

### 工时预估（高级工程师视角）

| 工作项 | 内容 | 工时 |
|--------|------|------|
| F-001 鉴权改造 | 用 `PlatformAppKeyAuthentication` 替换 `manage_config_kv` 路由中间件、接口经网关 `/inner/` 暴露、移除 handler `Authorize` 校验、补单测 | 6 人时 |
| F-002 移除权限点 | 删除 IAM meta 常量 / 静态注册（action、分组、推荐权限引用）/ auth-server 适配、处理 IAM action 删除的注册与迁移、补单测 | 10 人时 |
| 联调与缓冲 | 网关鉴权验证、IAM 平台同步验证、技术不确定性缓冲 | 4 人时 |
| **合计** | | **20 人时（2.5 人天）** |

> 说明：工时含开发 + 自测 + 代码审查 + 文档，不含需求澄清、方案评审、上线部署。
> IAM action 删除通常需在注册流程做迁移/下线处理，属主要不确定性来源。

### RICE 价值规模评分

| 参数 | 取值 | 依据 |
|------|------|------|
| Reach | 5 | 内部运维工具类接口，仅运维人员使用，触达面极小 |
| Impact | 2 | 运维效率优化 + 非必要权限点收敛，属一般功能增强/治理；非核心流程、非线上漏洞 |
| Confidence | 75% | 需求基本明确、方案复用已有中间件；但仍有 5 项待确认（Q-001~Q-005） |
| Effort | 2.5 人天 | 预估 20 人时 ÷ 8 |
| **RICE** | **3** | (5 × 2 × 0.75) / 2.5 = 3 |

> 评分解读：RICE < 20，属 ⚪ 极低区间。该需求价值规模小、影响面窄，属运维便利与权限治理类改进，
> 建议资源充裕时排期，非紧急。优先级定为 **Low**。

## 需求背景

### 业务背景

全局 KV 配置管理接口（`ManageConfigKV`，`POST /api/v1/config/manage_config_kv`）当前是对
`configs` 表做通用 KV 读写的系统级接口，已知的核心用途是维护「进程与配置管理可见性业务白名单」
（固定 key `pcv_biz`，值为逗号分隔的业务 ID 列表）。运维人员通过该接口把业务加入白名单，
从而开启该业务的进程与配置管理能力。

当前该接口存在两个使用痛点：

1. **鉴权方式不便于运维直连**：接口鉴权依赖 Cookie/Session（或蓝鲸网关 JWT）身份认证。
   该接口不在前端页面使用，仅运维人员使用，但每次请求都需要手动复制 Cookie，操作繁琐、
   易出错，不适合运维脚本化/工具化调用。

2. **权限点扩大了申请面**：接口在 IAM 注册了权限点 `manage_global_config_kv`（资源类型
   `global_config_kv`，中文名"全局配置 KV 管理操作"），并被归入"全局配置 KV 管理"动作分组、
   纳入"业务运维"推荐权限。这导致除管理员外，其他业务方也可以在权限中心申请到该接口权限。
   而该接口本质是平台级运维操作，不应对业务方开放申请，注册权限点属于非必要设计。

### 用户故事

作为**平台运维人员**
我想要**通过携带 app_code/app_secret 直接调用全局 KV 配置管理接口**
以便于**脚本化维护进程配置可见性白名单，不必每次手动复制 Cookie**。

作为**平台管理员**
我想要**移除该接口在权限中心的可申请权限点**
以便于**避免业务方误申请到本不该开放的平台级运维接口权限**。

### 需求来源

- **需求渠道**：技术优化 / 运维反馈
- **关联需求**：无
- **参考资料**：仓库现状代码（见"外部依赖与集成"）

## 功能需求

### 核心功能点

| 功能编号 | 功能描述 | 优先级 | 涉及角色 | 备注 |
|---------|---------|--------|---------|------|
| F-001 | 将 `ManageConfigKV` 接口鉴权由 Cookie/JWT 改为校验 app_code/app_secret | P0 | 运维 | 必须 |
| F-002 | 移除该接口注册的 IAM 权限点 `manage_global_config_kv` | P0 | 管理员 | 必须 |

### 详细功能描述

#### [F-001] 接口鉴权改为 app_code/app_secret 校验

- **输入**：运维方经蓝鲸 API 网关调用 `POST /api/v1/inner/config/manage_config_kv`，
  按网关规范携带 bscp 自身的 app_code/app_secret。
- **处理逻辑**：
  1. 接口不再要求 Cookie/Session 登录态。
  2. 网关用 app_code/app_secret 验证调用方身份，验证通过后下发已验签的 `X-Bkapi-JWT`，
     并消费掉调用方原始的 `X-Bkapi-Authorization` 头（app_secret 不会透传到后端）。
  3. 后端比对 JWT 中的 app_code 是否为 bscp 自身（`baseConf.app_code`），是则放行执行原有
     KV 读写逻辑（upsert/delete/get/list/append/remove），否则拒绝。
- **输出**：鉴权通过时正常返回 KV 操作结果；不通过时返回鉴权失败。
- **边界条件**：
  - 未携带任何凭证 → 拒绝。
  - app_code/app_secret 不匹配（网关侧拦截）→ 拒绝。
  - 其他应用经网关调用（JWT 合法但 app_code 非 bscp 自身）→ 拒绝。
  - 携带 Cookie 登录态 → 拒绝，不回退登录态鉴权。
  - 绕开网关直连并携带 `X-Bkapi-Authorization` → 拒绝。
  - 本服务未配置 `baseConf.app_code` → 拒绝（fail-closed）。
- **异常处理**：
  - 鉴权失败 → 返回明确的未授权错误，不泄露内部信息。

> ✅ **已决策（鉴权凭证判定依据，对应 Q-001）**：采用「经网关鉴权 + 限定 app_code」的组合，
> 即 Q-001 选项 A 与 C 的结合。网关负责校验 app_code/app_secret 并签发 JWT，后端只比对 JWT 中的
> app_code 是否为 bscp 自身。相比纯选项 C（不限定 app_code），可避免任何获得网关权限的应用都能调用。

> ✅ **已决策（是否保留 Cookie 兼容，对应 Q-002）**：完全去掉 Cookie 依赖。进一步地，也不保留
> 绕开网关的直连方式——该接口只经网关暴露，凭证无需外发给调用方，收敛凭证扩散风险。

#### [F-002] 移除 IAM 权限点

- **输入**：无（属于平台侧鉴权模型调整）。
- **处理逻辑（预期行为）**：
  1. 移除 handler 中针对 `GlobalConfigKV / ManageGlobalConfigKV` 的 IAM `Authorize` 校验。
  2. 移除 IAM 静态注册中的该权限点（action 定义、动作分组、"业务运维"推荐权限中的引用）。
  3. 移除后，业务方在权限中心不再能看到/申请该权限点。
- **输出**：接口不再依赖该 IAM 权限；权限中心不再展示该权限点。
- **边界条件**：
  - 已有业务此前申请过该权限点 → 需明确回收/迁移策略（见 Q-003）。
- **异常处理**：
  - 权限点删除需保证不影响其他接口（该权限点仅 `ManageConfigKV` 使用，属独占）。

> 🟡 **待确认（清理彻底程度）**：当前文档假设**彻底移除**（action 定义 + 分组 + 推荐权限引用 +
> handler 校验）。若只想先去掉 handler 校验、暂留 IAM 定义，见"未解决问题 Q-003"。

## 非功能需求

### 安全需求

- **权限控制**：接口改造后，鉴权唯一依据为网关已验签 JWT 中的 app_code 等于 bscp 自身。需确保
  移除 IAM 权限点后不产生越权——即只有持有 bscp app_code/app_secret 的运维方能调用。
- **用户身份仅用于审计**：鉴权只依据 app_code。调用方自报的用户名（网关免用户认证时取自
  `X-Bkapi-User-Name` 请求头）随请求透传，用于审计与日志追溯，不作为鉴权依据。
- **网关资源可见性**：该接口对应的网关资源须设为不公开且不可申请权限，避免在权限中心重新形成
  申请面（与 F-002 移除 IAM 权限点的目的一致）。
- **数据保护**：鉴权失败响应不得泄露内部实现细节；凭证不落日志明文。

### 兼容性

- **接口兼容**：后端路径 `POST /api/v1/config/manage_config_kv` 与请求/响应体保持不变。
  调用方入口变为网关路径 `POST /api/v1/inner/config/manage_config_kv`，由网关改写回后端路径。
- **数据兼容**：`pcv_biz` 等 configs 表数据结构不变。

## 业务规则

### 权限规则

- 改造后，`ManageConfigKV` 仅面向平台运维（凭 bscp 自身 app_code/app_secret 经网关调用），
  不再对业务方开放权限申请。
- `GetProcessConfigView`（只读查询白名单是否开启）**不在本次范围内**，维持现有登录态鉴权（假设方案下）。

## 外部依赖与集成

### 相关现状代码（供实现参考，非本文档改动约束）

| 用途 | 路径 |
|------|------|
| 接口 Proto（HTTP 路由） | `pkg/protocol/config-server/config_service.proto`（`ManageConfigKV`） |
| HTTP 路由注册 | `cmd/api-server/service/routers.go`（`/api/v1/config/manage_config_kv`，挂 `PlatformAppKeyAuthentication`） |
| config-server handler | `cmd/config-server/service/process_config_view.go`（`ManageConfigKV`，含 `Authorize` 校验） |
| 身份认证中间件 | `internal/iam/auth/middleware.go`（`UnifiedAuthentication`、`AppKeyAuthentication`、`PlatformAppKeyAuthentication`） |
| 网关 JWT 解析 | `internal/runtime/gwparser/parser.go`（`Parse`，`validateUser` 控制是否校验用户） |
| 网关资源配置注入 | `scripts/bk_gateway/inject_bk_gateway.py`（`/inner/` 路径与不公开路径的扩展配置） |
| IAM 权限点定义 | `pkg/iam/meta/resource.go`、`pkg/iam/meta/action.go`、`pkg/iam/sys/types.go` |
| IAM 静态注册 | `pkg/iam/sys/initial_actions.go`、`initial_action_groups.go`、`initial_common_actions.go` |
| IAM 鉴权适配 | `cmd/auth-server/service/auth/gen_id.go`、`adaptor.go` |
| 白名单 key 定义 | `cmd/data-service/service/config_kv.go`（`pcv_biz`） |

## 验收标准

### 功能验收

- [ ] **AC-001**：Given 运维方携带 bscp 自身正确的 app_code/app_secret，When 经网关调用 `POST /api/v1/inner/config/manage_config_kv`，Then 无需 Cookie 即可鉴权通过并正常执行 KV 操作。
- [ ] **AC-002**：Given 请求未携带或携带错误的 app_code/app_secret，When 调用该接口，Then 返回鉴权失败，操作被拒绝。
- [ ] **AC-006**：Given 其他应用持有合法的网关凭证，When 经网关调用该接口，Then 因 JWT 中 app_code 非 bscp 自身而被拒绝。
- [ ] **AC-007**：Given 该接口对应的网关资源，When 业务方在网关文档/权限中心查看，Then 不可见且不可申请。
- [ ] **AC-003**：Given 权限点已移除，When 业务方在权限中心检索/申请权限，Then 不再出现"全局配置 KV 管理操作"权限点。
- [ ] **AC-004**：Given 权限点已移除，When 调用 `ManageConfigKV`，Then 接口不再触发 `manage_global_config_kv` 的 IAM 授权校验。
- [ ] **AC-005**：Given 改造完成，When 调用 `GetProcessConfigView`，Then 其鉴权行为与改造前一致（不受影响）。

### 安全验收

- [ ] **AC-S01**：改造后无 app_code/app_secret 无法调用该接口，不存在匿名可调用路径。
- [ ] **AC-S02**：Given 请求携带任意 `X-Bkapi-User-Name`，When 调用该接口，Then 放行与否只由 app_code
      决定，该用户名仅作审计记录，不影响鉴权结果。
- [ ] **AC-S03**：Given 携带浏览器登录态 Cookie，When 调用该接口，Then 被拒绝，不回退登录态鉴权。

## 边界范围

### 本期包含

- `ManageConfigKV` 接口鉴权方式改造（Cookie/JWT → app_code/app_secret）。
- 移除 IAM 权限点 `manage_global_config_kv`（action 定义、分组、推荐权限引用、handler 校验）。

### 本期不包含

- `GetProcessConfigView` 只读接口的鉴权调整。
- `configs` 表数据结构、`pcv_biz` 白名单语义的变更。
- 进程操作 / 配置实例（`process.go` / `config_instance.go`）相关的业务权限调整。

## 约束条件

- **技术限制**：Go 代码须符合仓库 `.golangci.yml`；涉及 proto/IAM 生成文件需用仓库既有生成命令重新生成。
- **兼容限制**：后端接口路径与请求/响应体不变，调用方入口改为网关 `/inner/` 路径。
- **上线依赖（代码之外的手工操作）**：
  1. 在蓝鲸 API 网关为 `bk_bscp` 应用授权调用其自身的网关资源，否则请求会在网关侧被拦截。
  2. 发布后执行 `api-server migrate init-apigateway` 同步网关资源定义。

## 未解决问题

| 问题 ID | 问题描述 | 状态 |
|--------|---------|------|
| Q-001 | 鉴权凭证判定依据：A) 校验请求头与平台固定 app_code/app_secret；B) 维护允许的 app_code 白名单逐个校验；C) 只要经蓝鲸网关鉴权通过即可，不限定具体 app_code。 | ✅ 已决策：A+C 组合。网关校验 app_code/app_secret 并签发 JWT，后端比对 JWT 中 app_code 是否为 bscp 自身 |
| Q-002 | 是否保留 Cookie/Session 登录方式作为兼容。 | ✅ 已决策：完全去掉；并且不保留绕开网关的直连方式，仅经网关暴露 |
| Q-003 | 权限点清理彻底程度：A) 彻底移除 action 定义+分组+推荐权限+handler 校验（当前假设）；B) 仅去 handler 校验、暂留 IAM 定义。以及对已授权业务是否需要回收/迁移。 | 待确认 |
| Q-004 | 本次范围是否仅限 `ManageConfigKV`（当前假设：是）。 | 待确认 |
| Q-005 | 需求优先级。评估阶段依据 RICE=3（极低区间）建议定为 Low。 | 已建议 Low |

---

## 原需求描述

> ### **背景**
>
> ### 需求
>
> ### 原型设计
>
> ### 设计稿

（TAPD 原始描述为空模板，实际需求由需求提出人在澄清会话中口述，见"澄清记录"。）

## 澄清记录

### 第 1 轮澄清

**提问时间**：2026-07-21

**需求提出人口述需求**：

> 全局 kv 配置管理接口当前用于注册进程配置管理白名单。
> 1、这个接口不在页面使用，仅运维人员使用，但是每次请求都需要去复制 cookie。可以考虑改为
>    校验是否提交了 app_code 和 app_secret。或者其他方案。
> 2、这个接口注册了权限点，导致除了管理员外，其他业务也可以申请这个接口。实际上这是非必须的。
>    所以需要移除这个权限点。

**Agent 提问**（基于代码现状梳理，一次性提出）：

1. 🔴 鉴权方案：改造后用什么凭证判定调用方合法？（复用平台固定 app_code/app_secret / app_code 白名单 / 网关鉴权通过即可 / 需方案对比）
2. 是否保留 Cookie 登录作为兼容？
3. 🔴 权限点移除的彻底程度？（彻底清理 vs 仅去 handler 校验；已授权业务是否回收）
4. 本次范围是否仅 `ManageConfigKV`？（`GetProcessConfigView` 不动）

**用户回复**：用户跳过提问。Agent 按研发最佳实践给出默认假设（见各章节"待确认"标注与"未解决问题"），
待需求提出人后续确认或修正。

### 第 2 轮澄清（F-001 实现阶段）

**时间**：2026-08-20

**触发点**：实现 F-001 时发现，直连 bscp 对外域名（`api.bscp.woa.com`）会被基础设施层的 OA 认证
拦截，返回登录页而非到达 api-server，原假设的「运维携带 app_code/app_secret 直连」在生产环境不可行。

**决策结论**：

| 问题 | 结论 | 理由 |
|------|------|------|
| Q-001 鉴权凭证判定依据 | 网关鉴权 + 限定 app_code 为 bscp 自身 | 纯选项 C 会让任何获得网关权限的应用都能调用；加上 app_code 限定后与选项 A 等强度，且凭证不必外发 |
| Q-002 是否保留 Cookie | 完全去掉，且不保留直连 | 契合「不用复制 Cookie」诉求；直连既不可用也会扩散凭证 |
| 网关资源可见性 | 公开路径与 `/inner/` 路径均设为不公开、公开路径不可申请权限 | 若按仓库默认生成公开可申请资源，等于在网关侧重建了 F-002 要消除的申请面 |
| 是否复用 `UnifiedAuthentication` + 路径特例 | 否，保留专用的 `PlatformAppKeyAuthentication` | `UnifiedAuthentication` 会回退 Cookie 与 Dev 环境变量分支（后者直接从 `BK_APP_CODE_FOR_TEST` 取 app_code），把端点特例塞进公共中间件会让安全性依赖多处路径列表，且默认是放行而非拒绝 |
