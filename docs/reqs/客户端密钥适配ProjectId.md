# 【项目和环境】客户端密钥全链路适配project_id

## 基本信息

| 字段 | 值 |
|------|-----|
| 需求 ID | 1120451610136020732 (短 ID: 136020732) |
| 需求名称 | 【项目和环境】客户端密钥全链路适配project_id |
| 优先级 | High |
| 父需求 | 1020451610134444229 |
| 创建时间 | 2026-07-10 15:18:18 |
| 原始需求文档 | docs/reqs/客户端密钥适配ProjectId.md |

> 脱敏要求：基本信息只保留 TAPD 数字 ID（完整 19 位 ID 与短 ID），不写入处理人/负责人等真实人名，
> 也不写入 TAPD 内网域名链接。

## 需求背景

### 业务背景

当前 b-bscp 系统中的 **credentials（客户端密钥）** 和 **credential_scopes（密钥作用域）** 表虽然数据库层面已存在 `project_id` 字段，但全链路（Proto → Service → DAO）并未真正使用该字段进行数据隔离。这导致：

- 数据查询未按项目维度过滤，存在跨项目数据泄露风险
- 无法支持多项目场景下的密钥管理
- 与系统中其他已适配 project_id 的表（如 clients）行为不一致

### 用户故事

作为 **系统管理员**
我想要 **credentials 和 credential_scopes 全链路适配 project_id**
以便于 **实现多项目环境下的客户端密钥隔离管理**

### 需求来源

- **需求渠道**：技术优化/架构改进
- **关联需求**：父需求 1020451610134444229
- **参考资料**：client 表的 project_id 适配实现

## 功能需求

### 核心功能点

| 功能编号 | 功能描述 | 优先级 | 涉及角色 | 备注 |
|---------|---------|--------|---------|------|
| F-001 | Proto 层：CredentialAttachment 和 CredentialScopeAttachment 新增 project_id 字段 | P0 | 开发者 | 必须 |
| F-002 | Config-Server 路由层：credentials 和 credential_scopes 的 RPC 方法配置 additional_bindings | P0 | 开发者 | 必须 |
| F-003 | Config-Server Service 层：传递 ProjectId 到 Data-Service (grpcKit.ResolvedProjectID) | P0 | 开发者 | 必须 |
| F-004 | Data-Service Service 层：接收并使用 ProjectId 参数 | P0 | 开发者 | 必须 |
| F-005 | DAO 层：Credential 和 CredentialScope 所有查询方法增加 project_id 过滤条件 | P0 | 开发者 | 必须 |

### 详细功能描述

#### [F-001] Proto 消息定义扩展

- **输入**：修改 credential.proto 和 credential-scope.proto
- **处理逻辑**：
  1. 在 `CredentialAttachment` 中新增 `uint32 project_id = 2;` 字段
  2. 在 `CredentialScopeAttachment` 中新增 `uint32 project_id = 3;` 字段
- **输出**：重新生成的 PB Go 代码
- **边界条件**：字段位置需要考虑兼容性
- **异常处理**：无

#### [F-002] Config-Server 路由配置 (additional_bindings)

- **输入**：修改 config_service.proto 的 RPC 方法配置
- **处理逻辑**：
  1. 为以下 RPC 方法新增 `additional_bindings` 配置：
     - `CreateCredential`
     - `ListCredentials`
     - `DeleteCredential`
     - `UpdateCredential`
     - `CheckCredentialName`
     - `ListCredentialScopes`
     - `UpdateCredentialScopes`
     - `CredentialScopePreview`
  2. 配置格式参考 client 相关方法：
     ```protobuf
     option (google.api.http) = {
       post: "/v1/projects/{project_id}/[路径]"
       body: "*"
     };
     ```
  3. 确保 Req 消息中包含 `project_id` 字段
- **输出**：更新后的 proto 定义和重新生成的路由代码
- **边界条件**：所有涉及 credentials 和 credential_scopes 的方法都必须配置
- **异常处理**：无

#### [F-003] Config-Server → Data-Service 项目 ID 传递

- **输入**：修改 cmd/config-server/service/credential.go 和 credential_scope.go
- **处理逻辑**：
  1. 在调用 data-service gRPC 客户端时，添加参数：
     ```go
     ProjectId: grpcKit.ResolvedProjectID(req.ProjectId)
     ```
  2. 参考现有 client 实现模式
- **输出**：Config-Service 能正确传递项目 ID
- **边界条件**：所有 RPC 方法调用都必须传递
- **异常处理**：无

#### [F-004] Data-Service 接收项目 ID

- **输入**：修改 cmd/data-service/service/credential.go 和 credential_scope.go
- **处理逻辑**：
  1. 从请求中提取 `ProjectId` 参数
  2. 将其传递给 DAO 层查询方法
- **输出**：Data-Service 使用项目 ID 进行业务逻辑处理
- **边界条件**：需验证 project_id 有效性
- **异常处理**：无效 project_id 返回错误

#### [F-005] DAO 层 project_id 过滤

- **输入**：修改 internal/dal/dao/credential.go 和 credential_scope.go
- **处理逻辑**：
  1. 所有 List/Delete/Update 方法签名新增 `projectId uint32` 参数
  2. 在 SQL 查询条件中添加 `WHERE project_id = ?` 过滤
  3. 参考 client dao 的实现方式：
     ```go
     func (d *CredentialDAO) List(ctx context.Context, projectId uint32, bizId uint32, ...) ([]*table.Credential, error) {
         // 添加 project_id 条件
     }
     ```
- **输出**：DAO 层所有查询都基于项目维度隔离
- **边界条件**：
  - Create 操作需要在插入时设置 project_id
  - Delete/Update 必须同时匹配 project_id 防止误操作
- **异常处理**：无

## 非功能需求

### 性能需求

- **响应时间**：新增 project_id 过滤不应影响现有查询性能（< 100ms）
- **并发能力**：支持高并发场景下的项目级别数据隔离

### 安全需求

- **权限控制**：确保不同项目间的数据完全隔离
- **数据保护**：防止跨项目数据访问

### 兼容性

- **接口兼容**：Proto 变更需保证向后兼容（新增字段，非删除）
- **数据兼容**：已有数据的 project_id 字段需要有合理的默认值或迁移策略

## 业务规则

### 业务逻辑规则

- **规则 R-001**：所有 credentials 操作必须在指定项目上下文中执行
- **规则 R-002**：credential_scopes 不再通过 credential_id 外键直接关联，而是通过 project_id + credential_id 组合定位
- **规则 R-003**：项目 ID 从 API Gateway 经 Config-Server 透传至 Data-Service，最终用于 DAO 层查询

### 数据校验规则

- **必填字段**：project_id（所有操作）
- **取值范围**：project_id > 0

## 验收标准

### 功能验收

- [ ] **AC-001**：Given 有效的项目 ID When 调用 CreateCredential Then 成功创建且 project_id 正确设置
- [ ] **AC-002**：Given 项目 A 有凭证 1,2 When 在项目 B 下查询凭证列表 Then 返回空列表（不显示项目 A 的数据）
- [ ] **AC-003**：Given 项目 A 有凭证 1 When 在项目 B 下尝试删除凭证 1 Then 返回错误（不存在或无权限）
- [ ] **AC-004**：Given 有效的项目 ID When 调用 ListCredentialScopes Then 只返回该项目下的 scope 列表
- [ ] **AC-005**：When 通过 REST API 调用 credentials 相关接口 Then URL 包含 /v1/{project_id}/ 路径
- [ ] **AC-006**：When Config-Server 调用 Data-Service Then 请求中包含正确的 ProjectId 字段
- [ ] **AC-007**：When DAO 层执行查询 Then SQL 语句包含 project_id 过滤条件

### 安全验收

- [ ] **AC-S01**：Given 用户只有项目 A 的权限 When 尝试访问项目 B 的凭证 Then 返回权限不足错误

## 边界范围

### 本期包含

- credentials 全链路（Proto → Config-Server → Data-Server → DAO）适配 project_id
- credential_scopes 全链路适配 project_id
- 路由层 additional_bindings 配置（无需新增中间件）
- DAO 层所有 CRUD 方法的 project_id 过滤

### 本期不包含

- 新增独立中间件
- 已有数据的 project_id 回填迁移脚本
- 其他未提及表的 project_id 适配

## 约束条件

- **技术限制**：必须遵循现有的架构模式（参考 client 表的实现）
- **时间限制**：无特殊时间要求
- **资源限制**：无特殊资源限制

## 未解决问题

| 问题 ID | 问题描述 | 截止日期 | 状态 |
|--------|---------|---------|------|
| Q-001 | 已有数据的 project_id 默认值策略 | 待确认 | 待确认 |

---

## 原需求描述

> (无描述内容)

## 澄清记录

### 第 1 轮澄清

**提问时间**：2026-07-10 15:33

**Agent 提问**：
1. （跳过 - 用户需求已明确）

**用户回复**：
用户提供了非常详细的需求说明，包含四个核心要点：
1. credentials 表存在项目 ID，credential_scopes 的路由都需要改造，也就是在表不需要通过 credential_id 外键
2. 不需要新增中间涉及到 credentials 和 credential_scopes 的路由都需要改造，也就是在 additional_bindings 新增新路由，注意 req 参数也需要加上
3. 需要从 config-server 把项目 id 传到 data-service, 也就是 ProjectId: grpcKit.ResolvedProjectID(req.ProjectId)
4. credentials 增删改查也需要有项目 ID，Credential dao 层查询都需要加上 projectID，可以参考其他

**确认状态**：✅ 用户确认需求描述充分，无需额外澄清
