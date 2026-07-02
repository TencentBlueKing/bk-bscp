# 【项目和环境】分组及分组-服务绑定适配 project_id

## 基本信息

| 字段 | 值 |
|------|-----|
| 需求 ID | 1120451610135156099（短 ID: 135156099） |
| 需求名称 | 【项目和环境】分组及分组-服务绑定适配 project_id |
| 优先级 | High |
| 父需求 | 1020451610134444229（【重要】BSCP 承接游戏业务场景需求） |
| 创建时间 | 2026-06-12 10:24:46 |
| 原始需求文档 | docs/reqs/group适配project_id.md |
| **价值规模 (RICE)** | **320** |
| **预估工时** | **16 人时** |

### RICE 评分明细

| 参数 | 值 | 说明 |
|------|-----|------|
| Reach | 80 | 影响大部分 BSCP 用户 |
| Impact | 8 | 高优先级 — 核心架构适配 |
| Confidence | 100% | 方案清晰、代码已完成 |
| Effort | 2 人天 | Proto+Table+RPC+中间件+路由 |

**RICE Score = (80 x 8 x 1.0) / 2 = 320** — 紧急优先级 (>200)

### 拆分评估

**结论：无需拆分**。6 个功能点围绕同一业务目标，存在严格技术依赖（数据模型→RPC→中间件→路由），总工时 16 人时在合理单需求范围内。

## 需求背景

### 业务背景

BSCP 系统正在引入「项目」和「环境」两个新维度以承接游戏业务场景。此前 Hook 已完成 project_id 适配（新增 HookProjectVerified 中间件 + additional_bindings 路由），Group 作为与 Hook 同级的资源实体，需要同步进行 project_id 适配。

**核心问题**：
- 当前 Group 实体缺少 `project_id` 字段归属关系
- 无法在 API 层面校验 Group 是否属于指定的 Project
- 需要同时兼容旧路由（无 project_id）和新路由（带 project_id）

### 用户故事

作为 BSCP 平台开发者
我想要为 Group 资源新增 `project_id` 字段并配套路由校验中间件
以便于 Group 能正确归属于某个 Project，支持多项目场景下的资源隔离

### 需求来源

- **需求渠道**：产品规划（父需求：BSCP 承接游戏业务场景需求）
- **关联需求**：1120451610134444229
- **参考资料**：Hook 已完成的 project_id 适配实现模式

## 功能需求

### 核心功能点

| 功能编号 | 功能描述 | 优先级 | 涉及角色 | 备注 |
|---------|---------|--------|---------|------|
| F-001 | GroupAttachment 新增 project_id 字段（Proto + Table） | P0 | 开发者 | 必须 |
| F-002 | 新增 GetGroup RPC 接口（按 ID 获取 Group） | P0 | 开发者 | 必须 |
| F-003 | Group 相关接口新增带 project_id 的 additional_bindings 路由 | P0 | 开发者 | 必须 |
| F-004 | 新增 GroupProjectVerified 校验中间件 | P0 | 开发者 | 必须 |
| F-005 | API Server 注册新旧两套 Group 路由 | P0 | 开发者 | 必须 |
| F-006 | ConfigServer 实现 GetGroup 方法 | P0 | 开发者 | 必须 |

### 详细功能描述

#### [F-001] Group 数据模型新增 project_id

- **输入**：Proto 定义和 Table ORM 定义修改
- **处理逻辑**：
  1. 在 `pkg/protocol/core/group/group.proto` 的 `GroupAttachment` 消息中新增 `uint32 project_id = 2`
  2. 在 `pkg/dal/table/group.go` 的 `GroupAttachmentColumnDescriptor` 新增 `project_id` 列描述
  3. 在 `GroupAttachment` struct 中新增 `ProjectID uint32` 字段
- **输出**：数据模型支持 project_id 归属字段
- **边界条件**：project_id 为 uint32 类型，默认值为 0
- **异常处理**：无需额外校验（由上层业务保证）

> **明确不改动**：`group_app_binds` 不需要 project_id 字段

#### [F-002] 新增 GetGroup RPC

- **输入**：GetGroupReq（biz_id, group_id, project_id）
- **处理逻辑**：
  1. ConfigServer 新增 `GetGroup` RPC 方法
  2. 内部调用 DataServer 的 `GetGroupByID` 接口
  3. 返回完整的 Group 信息
- **输出**：GetGroupResp（包含 pbgroup.Group data）

#### [F-003] Group 接口 additional_bindings 路由

以下 RPC 接口均需新增带 `{project_id}` 的 additional_bindings 路由：

| RPC | 原有路由 | 新增路由 (additional_bindings) |
|-----|---------|-------------------------------|
| CreateGroup | POST /biz/{biz_id}/groups | POST /biz/{biz_id}/projects/{project_id}/groups |
| DeleteGroup | DELETE /biz/{biz_id}/groups/{group_id} | DELETE /biz/{biz_id}/projects/{project_id}/groups/{group_id} |
| BatchDeleteGroups | POST .../groups/batch_delete | POST .../projects/{project_id}/groups/batch_delete |
| UpdateGroup | PUT /biz/{biz_id}/groups/{group_id} | PUT /biz/{biz_id}/projects/{project_id}/groups/{group_id} |
| ListAllGroups | GET /biz/{biz_id}/groups | GET /biz/{biz_id}/projects/{project_id}/groups |
| ListGroupReleasedApps | GET .../groups/{group_id}/released_apps | GET .../projects/{project_id}/groups/{group_id}/released_apps |
| GetGroupByName | 原有 + inner | **新增** GET /biz/{biz_id}/projects/{project_id}/groups/query/name/{group_name} |
| ListGroupSelector | GET .../groups/selector/{label_name} | GET .../projects/{project_id}/groups/selector/{label_name} |
| **GetGroup（新增）** | GET /biz/{biz_id}/groups/{group_id} | GET /biz/{biz_id}/projects/{project_id}/groups/{group_id} |

各 Request 消息新增 `uint32 project_id` 字段（标注为 additional_bindings 使用）。

#### [F-004] GroupProjectVerified 中间件

- **功能**：类似已有的 `HookProjectVerified`，校验 URL 中的 group_id 对应的 Group 是否属于 kt.ProjectID 指定的项目
- **前置条件**：kt.ProjectID 已被赋值（通过 VerifyProjectExists 或 checkOrCreateDefaultProjectEnv）
- **处理逻辑**：
  1. 从 URL 参数提取 group_id
  2. 调用 config-server GetGroup 接口（传入 biz_id, group_id, project_id）
  3. 若返回错误 → 返回 400 "group does not belong to the specified project"
  4. 若成功 → 放行到下一个 handler
- **错误响应**：400 Bad Request + 明确错误消息

#### [F-005] 路由注册策略

采用与 Hook 一致的双轨制路由：

| 路由类型 | 路径模式 | ProjectID 注入方式 | 中间件 |
|---------|---------|-------------------|--------|
| **新路由** | `/api/v1/config/biz/{biz_id}/projects/{project_id}/groups/...` | URL 参数 → VerifyProjectExists | GroupProjectVerified |
| **旧路由(兼容)** | `/api/v1/config/biz/{biz_id}/groups/{group_id}` | checkOrCreateDefaultProjectEnv | GroupProjectVerified |

- 新路由位于 `/biz/{biz_id}/projects/{project_id}/` 下，先经过 `VerifyProjectExists` 校验
- 旧路由保持原有路径不变，使用 `checkOrCreateDefaultProjectEnv` 自动注入默认 ProjectID
- 含 `{group_id}` 动态参数的路由挂载 `GroupProjectVerified` 中间件
- 批量操作等不含单个 group_id 的静态路由不挂载该中间件

#### [F-006] ConfigServer GetGroup 实现

```go
func (s *Service) GetGroup(ctx context.Context, req *pbcs.GetGroupReq) (*pbcs.GetGroupResp, error) {
    // 1. 业务鉴权（Biz FindBusinessResource）
    // 2. 调用 DS.GetGroupByID
    // 3. 封装返回 GetGroupResp{Data: rp}
}
```

## 非功能需求

### 兼容性

- **向后兼容**：旧路由完全保留，行为不变
- **Proto 兼容性**：新增字段使用递增序号，不影响现有序列化
- **数据库**：需 migration 为 groups 表添加 project_id 列

## 业务规则

### 权限规则

- 所有 Group 接口维持现有 IAM 鉴权逻辑不变
- GroupProjectVerified 是额外的项目归属校验层，在 IAM 鉴权之后执行

### 数据规则

- GroupAttachment.project_id 标识 Group 归属的项目
- group_app_binds 表不涉及 project_id 变更

## 外部依赖与集成

### 内部组件依赖

| 组件 | 交互方式 | 说明 |
|------|---------|------|
| DataServer | gRPC | GetGroupByID 获取 Group 详情 |
| ConfigServer（自身） | gRPC | GetGroup 供中间件调用 |

## 验收标准

### 功能验收

- [ ] **AC-001**：Given groups 表已有 project_id 列 When 通过 Proto 序列化/反序列化 GroupAttachment Then project_id 字段正确读写
- [ ] **AC-002**：Given 已有 Group 数据 When 调用 GetGroup RPC 传入 biz_id 和 group_id Then 返回正确的 Group 详情（含 project_id）
- [ ] **AC-003**：Given 请求新路由 `/biz/{bid}/projects/{pid}/groups/{gid}` When group_id 属于 project_id 则请求正常放行
- [ ] **AC-004**：Given 请求新路由 `/biz/{bid}/projects/{pid}/groups/{gid}` When group_id 不属于 project_id 则返回 400 错误 "group does not belong to the specified project"
- [ ] **AC-005**：Given 请求旧路由 `/biz/{bid}/groups/{gid}` When 旧客户端未传 project_id 则通过 checkOrCreateDefaultProjectEnv 正常工作
- [ ] **AC-006**：Given 调用 CreateGroup 使用新路由（含 project_id）When 请求体包含 project_id 则创建的 Group 归属于指定项目
- [ ] **AC-007**：Given 调用 DeleteGroup/UpdateGroup/BatchDeleteGroups/ListAllGroups/ListGroupReleasedApps/GetGroupByName/ListGroupSelector 使用新路由 When URL 包含 {project_id} 则请求正常路由到对应处理器
- [ ] **AC-008**：Given group_app_binds 相关操作 When 不涉及 project_id 字段变更 则行为不受影响

## 边界范围

### 本期包含

- Group 数据模型新增 project_id 字段（Proto + Table + ColumnDescriptor）
- 新增 GetGroup RPC 接口（ConfigServer 实现）
- 9 个 Group 相关 RPC 接口的 additional_bindings 路由
- GroupProjectVerified 中间件
- 双轨制路由注册（新路由 + 旧路由兼容）
- 各 Request 消息的 project_id 字段扩展

### 本期不包含

- group_app_binds 的 project_id 字段变更
- 前端界面改造
- 数据库 migration 脚本（需 DBA 配合）
- Proto 文件的代码重新生成（make proto）

## 约束条件

- **技术限制**：必须复用 Hook 已有的 project_id 适配模式（中间件 + additional_bindings），保持架构一致性
- **兼容限制**：旧路由不可破坏，必须保持 checkOrCreateDefaultProjectEnv 行为不变

## 未解决问题

| 问题 ID | 问题描述 | 截止日期 | 状态 |
|--------|---------|---------|------|
| 无 | - | - | - |

---

## 原需求描述

> (无描述内容)

## 澄清记录

### 第 1 轮澄清

**提问时间**：2026-06-30 14:34

**Agent 提问**：
1. 用户已在对话中提供了完整的需求描述，包括：group 新增 project_id、group_app_binds 不需要、新增 GroupProjectVerified 中间件、旧路由保持 checkOrCreateDefaultProjectEnv、新路由使用 additional_bindings。

**用户回复**：
1. 需求描述充分清晰，基于已有代码改动确认无误，跳过交互式澄清，直接生成文档并回填 TAPD。
