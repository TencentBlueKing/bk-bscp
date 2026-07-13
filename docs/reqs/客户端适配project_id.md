# 【项目和环境】客户端、客户端事件、客户端查询、事件全链路适配 project_id

## 基本信息

| 字段 | 值 |
|------|-----|
| 需求 ID | 1120451610135156257（短 ID: 135156257） |
| 需求名称 | 【项目和环境】客户端、客户端事件、客户端查询、事件 |
| 优先级 | High |
| 父需求 | 1020451610134444229（BSCP 承接游戏业务场景需求） |
| 创建时间 | 2026-06-12 10:27:16 |
| 原始需求文档 | docs/reqs/客户端适配project_id.md |

> 脱敏要求：基本信息只保留 TAPD 数字 ID（完整 19 位 ID 与短 ID），不写入处理人/负责人等真实人名，
> 也不写入 TAPD 内网域名链接。

## 需求背景

### 业务背景

BSCP 系统正在引入「项目」和「环境」两个新维度以承接游戏业务场景。此前 **Group**、**TemplateSpace** 和 **TemplateVariable** 已完成 `project_id` 适配。**Client** 作为核心资源实体（代表接入 BSCP 的客户端实例），需要同步进行 `project_id` 适配。

**当前状态**：
- ✅ `clients` 表已在数据库 migration 中新增了 `project_id` 列（类型 uint32，默认值 0）
- ✅ `client_querys` 表已在数据库 migration 中新增了 `project_id` 列（类型 uint32，默认值 0）
- ❌ Proto 定义中的 `ClientAttachment` 和 `ClientQueryAttachment` 尚未更新 `project_id` 字段
- ❌ 无项目归属校验中间件
- ❌ 无带 `project_id` 的新路由

**核心问题**：
- 客户端缺少项目归属关系的代码层校验
- 无法在 API 层面校验客户端资源是否属于指定的 Project
- 客户端事件（client_events）不直接存储 `project_id`，需要通过逻辑关联（关联 client 的 `project_id`）进行归属判断
- 需要同时兼容旧路由（无 `project_id`）和新路由（带 `project_id`）

### 用户故事

作为 BSCP 平台开发者
我想要为客户端、客户端查询新增项目归属校验能力（通过 ClientProjectVerified 中间件 + additional_bindings 路由）
以便于客户端能正确归属于某个 Project，支持多项目场景下的资源隔离和数据安全

### 需求来源

- **需求渠道**：产品规划（父需求：BSCP 承接游戏业务场景需求）
- **关联需求**：
  - 父需求：1020451610134444229（BSCP 承接游戏业务场景需求）
  - 兄弟需求：
    - 1120451610135156099（Group 适配 project_id）
    - 1120451610135948333（TemplateVariable 全链路适配 project_id）
    - 1120451610135611515（模板功能适配 project_id）
- **参考资料**：
  - 已完成的 Group/Hook/TemplateSpace/TemplateVariable 的 project_id 适配实现模式
  - docs/reqs/模板变量适配project_id.md（TemplateVariable 完整实现方案）

## 功能需求

### 核心功能点

| 功能编号 | 功能描述 | 优先级 | 涉及角色 | 备注 |
|---------|---------|--------|---------|------|
| F-001 | ClientAttachment Proto 新增 project_id 字段 | P0 | 开发者 | 必须（DB已有列） |
| F-002 | ClientQueryAttachment Proto 新增 project_id 字段 | P0 | 开发者 | 必须（DB已有列） |
| F-003 | pkg/dal/table/client.go 和 client_query.go 结构体新增 ProjectID 字段 | P0 | 开发者 | 必须 |
| F-004 | 新增 GetClient RPC 接口（供中间件调用） | P0 | 开发者 | 必须 |
| F-005 | 新增 ClientProjectVerified 校验中间件（统一覆盖 clients 和 client_querys） | P0 | 开发者 | 必须统一 |
| F-006 | Client 相关接口（10个）新增带 project_id 的 additional_bindings 路由 | P0 | 开发者 | 全部改造 |
| F-007 | ClientQuery 相关接口（5个）新增带 project_id 的 additional_bindings 路由 | P0 | 开发者 | 全部改造 |
| F-008 | ClientEvent 相关接口（2+个）新增带 project_id 的 additional_bindings 路由（逻辑关联校验） | P0 | 开发者 | 业务层校验 |
| F-009 | API Server 注册新旧两套路由（双轨制） | P0 | 开发者 | 兼容旧客户端 |

### 详细功能描述

#### [F-001] Client 数据模型更新（Proto 层）

- **输入**：Proto 定义修改
- **处理逻辑**：
  1. 在 `pkg/protocol/core/client/client.proto` 的 `ClientAttachment` 消息中新增 `uint32 project_id = 4`
  2. 在所有 Client 相关的 Request 消息中新增 `uint32 project_id` 字段（标注 additional_bindings 使用）
  3. 执行 `make proto` 重新生成 Proto 代码
- **输出**：数据模型支持 project_id 归属字段
- **边界条件**：project_id 为 uint32 类型，默认值为 0
- **异常处理**：无需额外校验（由上层业务保证）

> **明确说明**：数据库层面的 `clients.project_id` 列已通过 migration 添加并回填完成，本次仅更新代码层的 Proto/Table 定义使其对齐。

#### [F-002] ClientQuery 数据模型更新（Proto 层）

- **输入**：Proto 定义修改
- **处理逻辑**：
  1. 在 `pkg/protocol/core/client-query/client_query.proto` 的 `ClientQueryAttachment` 消息中新增 `uint32 project_id = 3`
  2. 在所有 ClientQuery 相关的 Request 消息中新增 `uint32 project_id` 字段（标注 additional_bindings 使用）
  3. 执行 `make proto` 重新生成 Proto 代码
- **输出**：数据模型支持 project_id 归属字段
- **边界条件**：project_id 为 uint32 类型，默认值为 0

#### [F-003] Table 结构体更新

- **输入**：Go 结构体修改
- **处理逻辑**：
  1. 在 `pkg/dal/table/client.go` 的相关 struct 中新增字段：
     ```go
     type ClientAttachment struct {
         BizID     uint32 `gorm:"column:biz_id" json:"biz_id"`
         AppID     uint32 `gorm:"column:app_id" json:"app_id"`
         Uid       string `gorm:"column:uid" json:"uid"`
         ProjectID uint32 `gorm:"column:project_id" json:"project_id"` // 新增
     }
     ```
  2. 在 `pkg/dal/table/client_query.go` 的相关 struct 中新增字段：
     ```go
     type ClientQueryAttachment struct {
         BizID     uint32 `gorm:"column:biz_id" json:"biz_id"`
         AppID     uint32 `gorm:"column:app_id" json:"app_id"`
         ProjectID uint32 `gorm:"column:project_id" json:"project_id"` // 新增
     }
     ```
  3. 确保 gorm tag 与数据库列名一致
- **输出**：ORM 层支持 project_id 字段的读写

#### [F-004] 新增 GetClient RPC 接口

- **输入**：GetClientReq（biz_id, client_id, project_id）
- **处理逻辑**：
  1. ConfigServer 新增 `GetClient` RPC 方法
  2. 内部调用 DataServer 或自身逻辑获取 Client 详情
  3. 校验返回的 Client.Attachment.ProjectID 是否与请求的 project_id 一致
  4. 返回完整的 Client 信息
- **输出**：GetClientResp（包含 pbclient.Client data）

> **设计说明**：该接口主要供 `ClientProjectVerified` 中间件调用，用于校验客户端的项目归属关系。
> 类似 TemplateVariable 的 GetTemplateVariable 实现。

#### [F-005] ClientProjectVerified 中间件

- **功能**：类似已有的 `TemplateSpaceProjectVerified` 和 `TemplateVariableProjectVerified`，
  统一校验 URL 中的 `client_id` 对应的 Client 是否属于 kt.ProjectID 指定的项目
- **前置条件**：kt.ProjectID 已被赋值（通过 VerifyProjectExists 或 checkOrCreateDefaultProjectEnv）
- **适用范围**：
  - ✅ 所有含 `{client_id}` 动态参数的新路由（clients 和 client_events 共用）
  - ✅ 所有含 `{client_query_id}` 动态参数的新路由（如果存在单个查询资源的接口）
  - ❌ 不适用于列表类等不含单个资源 ID 的静态路由（但 Request 中仍需携带 project_id 用于过滤）
- **处理逻辑**：
  1. 从 URL 参数提取 `client_id` 或 `client_query_id`
  2. 调用 config-server GetClient 接口（传入 biz_id, client_id, project_id）
  3. 若返回错误 → 返回 400 "client does not belong to the specified project"
  4. 若成功 → 放行到下一个 handler
- **错误响应**：400 Bad Request + 明确错误消息

> **设计说明**：Client 和 ClientQuery 共用同一个 `ClientProjectVerified` 中间件，
> 因为 ClientQuery 通过自身的 `project_id` 字段可以直接校验，也可以间接通过关联的 Client 来校验。
>
> **对于 ClientEvent（逻辑关联）**：
> ClientEvent 不单独挂载中间件，而是在业务层（如 PushEvent handler 内部）通过查询关联 Client 的 `project_id` 来进行归属校验。

#### [F-006] Client 接口 additional_bindings 路由

以下 **10 个** Client RPC 接口均需新增带 `{project_id}` 的 additional_bindings 路由：

| 序号 | RPC 方法名 | HTTP 方法 | 原有路由 | 新增路由 (additional_bindings) |
|-----|----------|----------|---------|-------------------------------|
| 1 | ListClients | GET | /biz/{biz_id}/apps/{app_id}/clients | GET .../projects/{project_id}/clients |
| 2 | ClientConfigVersionStatistics | POST | /biz/{biz_id}/apps/{app_id}/clients/config_version_statistics | POST .../projects/{project_id}/.../config_version_statistics |
| 3 | ClientPullTrendStatistics | POST | /biz/{bid}/apps/{aid}/clients/pull_trend_statistics | POST .../projects/{pid}/.../pull_trend_statistics |
| 4 | ClientPullStatistics | POST | /biz/{bid}/apps/{aid}/clients/pull_statistics | POST .../projects/{pid}/.../pull_statistics |
| 5 | ClientLabelStatistics | POST | /biz/{bid}/apps/{aid}/clients/label_statistics | POST .../projects/{pid}/.../label_statistics |
| 6 | ClientAnnotationStatistics | POST | /biz/{bid}/apps/{aid}/clients/annotation_statistics | POST .../projects/{pid}/.../annotation_statistics |
| 7 | ClientVersionStatistics | POST | /biz/{bid}/apps/{aid}/clients/version_statistics | POST .../projects/{pid}/.../version_statistics |
| 8 | ListClientLabelAndAnnotation | GET | /biz/{bid}/apps/{aid}/clients/labels_annotations | GET .../projects/{pid}/.../labels_annotations |
| 9 | ClientSpecificFailedReason | POST | /biz/{bid}/apps/{aid}/clients/specific_failed_reason | POST .../projects/{pid}/.../specific_failed_reason |
| 10 | RetryClients | POST | /biz/{bid}/apps/{aid}/clients/retry | POST .../projects/{pid}/.../retry |

各 Request 消息新增 `uint32 project_id` 字段（标注为 additional_bindings 使用）。

#### [F-007] ClientQuery 接口 additional_bindings 路由

以下 **5 个** ClientQuery RPC 接口均需新增带 `{project_id}` 的 additional_bindings 路由：

| 序号 | RPC 方法名 | HTTP 方法 | 原有路由 | 新增路由 (additional_bindings) |
|-----|----------|----------|---------|-------------------------------|
| 1 | ListClientQueries | GET | /biz/{biz_id}/apps/{app_id}/client_queries | GET .../projects/{project_id}/client_queries |
| 2 | ClientQueryTrendStatistics | POST | /biz/{bid}/apps/{aid}/client_queries/trend_statistics | POST .../projects/{pid}/.../trend_statistics |
| 3 | ClientQueryCostTimeRanking | POST | /biz/{bid}/apps/{aid}/client_queries/cost_time_ranking | POST .../projects/{pid}/.../cost_time_ranking |
| 4 | ClientQueryFailedReason | POST | /biz/{bid}/apps/{aid}/client_queries/failed_reason | POST .../projects/{pid}/.../failed_reason |
| 5 | ClientQuerySlowQueryTop | POST | /biz/{bid}/apps/{aid}/client_queries/slow_query_top | POST .../projects/{pid}/.../slow_query_top |

各 Request 消息新增 `uint32 project_id` 字段（标注为 additional_bindings 使用）。

#### [F-008] ClientEvent 接口 additional_bindings 路由（逻辑关联校验）

以下 **2 个及以上** ClientEvent RPC 接口均需新增带 `{project_id}` 的 additional_bindings 路由：

| 序号 | RPC 方法名 | HTTP 方法 | 原有路由 | 新增路由 (additional_bindings) | 校验方式 |
|-----|----------|----------|---------|-------------------------------|---------|
| 1 | PushClientEvent | POST | /biz/{bid}/apps/{aid}/client_events/push | POST .../projects/{pid}/.../push | 业务层查 Client 的 project_id |
| 2 | ListClientEvents | GET | /biz/{bid}/apps/{aid}/client_events | GET .../projects/{pid}/client_events | 业务层查 Client 的 project_id |

**特殊说明**：
- ClientEvent 的 Proto 定义 **不需要** 更新（因为采用逻辑关联，不直接存储 `project_id`）
- 各 Request 消息 **可选** 新增 `uint32 project_id` 字段（如果需要在 URL 中传递则添加；如果完全依赖业务层查询可不加）
- **推荐做法**：Request 消息也新增 `uint32 project_id` 字段以保持一致性，便于后续过滤和日志记录

#### [F-009] 路由注册策略

采用与 Group/Hook/TemplateSpace/TemplateVariable 一致的双轨制路由：

| 资源类型 | 路由类型 | 路径模式 | ProjectID 注入方式 | 中间件 |
|---------|---------|---------|-------------------|--------|
| **Client** | 新路由 | `/api/v1/config/biz/{biz_id}/apps/{app_id}/projects/{project_id}/clients/...` | URL 参数 → VerifyProjectExists | ClientProjectVerified（含 {client_id} 时） |
| **Client** | 旧路由(兼容) | `/api/v1/config/biz/{biz_id}/apps/{app_id}/clients/...` | checkOrCreateDefaultProjectEnv | ClientProjectVerified（含 {client_id} 时） |
| **ClientQuery** | 新路由 | `/api/v1/config/biz/{biz_id}/apps/{app_id}/projects/{project_id}/client_queries/...` | URL 参数 → VerifyProjectExists | 可选（根据是否有单个资源ID） |
| **ClientQuery** | 旧路由(兼容) | `/api/v1/config/biz/{biz_id}/apps/{app_id}/client_queries/...` | checkOrCreateDefaultProjectEnv | 同上 |
| **ClientEvent** | 新路由 | `/api/v1/config/biz/{biz_id}/apps/{app_id}/projects/{project_id}/client_events/...` | URL 参数 → VerifyProjectExists | 无（业务层校验） |
| **ClientEvent** | 旧路由(兼容) | `/api/v1/config/biz/{biz_id}/apps/{app_id}/client_events/...` | checkOrCreateDefaultProjectEnv | 无（业务层校验） |

路由注册代码示例（routers.go）：

```go
// 客户端相关
r.Route("/clients", func(r chi.Router) {
    r.Mount("/", p.cfgSvrMux)
    r.Route("/{client_id}", func(r chi.Router) {
        r.Use(p.ClientProjectVerified) // 校验 Client 归属于该项目
        r.Mount("/", p.cfgSvrMux)
    })
})

// 客户端查询相关
r.Route("/client_queries", func(r chi.Router) {
    r.Mount("/", p.cfgSvrMux)
    // 如有单个查询资源的路由，可选择性挂载中间件
})
```

关键规则：
- 新路由位于 `/apps/{app_id}/projects/{project_id}/` 下（注意：Client 路由比 TemplateVariable 多一层 app_id），先经过 `VerifyProjectExists` 校验
- 旧路由保持原有路径不变（如 `/api/v1/config/biz/{biz_id}/apps/{app_id}/clients`），使用 `checkOrCreateDefaultProjectEnv` 自动注入默认 ProjectID
- 含 `{client_id}` 动态参数的路由挂载 `ClientProjectVerified` 中间件
- 列表类等不含单个资源 ID 的静态路由不挂载该中间件（但 Request 中仍需携带 project_id 用于过滤）
- ClientEvent 的路由 **不挂载** ClientProjectVerified 中间件（改用业务层校验）

## 非功能需求

### 兼容性

- **向后兼容**：旧路由完全保留，行为不变
- **Proto 兼容性**：新增字段使用递增序号（ClientAttachment=4, ClientQueryAttachment=3），不影响现有序列化
- **数据库**：✅ 无需额外 migration（clients/project_id 和 client_querys/project_id 列已存在）

### 性能需求

- **响应时间**：新增的 ClientProjectVerified 中间件增加的单次 RPC 调用耗时 ≤ 10ms（P99）
- **并发能力**：无特殊要求，沿用现有连接池配置

### 安全需求

- **权限控制**：所有客户端接口维持现有 IAM 鉴权逻辑不变
- **归属校验**：ClientProjectVerified 是额外的项目归属校验层，在 IAM 鉴权之后执行
- **数据隔离**：确保不同项目间的客户端资源不可越权访问
- **ClientEvent 逻辑关联安全**：通过查询 Client 的 project_id 来确保事件归属于正确项目

## 业务规则

### 数据模型关系

```
clients 表:
  - 有 project_id 字段（直接存储）
  - 通过 client_id 关联 client_events

client_querys 表:
  - 有 project_id 字段（直接存储）
  - 可能通过某种方式与 client 关联

client_events 表:
  - ⚠️ 没有 project_id 字段
  - 通过 client_id → clients.project_id 进行逻辑关联
  - 归属校验时需先查 client 再判断 project_id
```

### 权限规则

- 所有 Client/ClientQuery/ClientEvent 接口维持现有 IAM 鉴权逻辑不变
- ClientProjectVerified 是额外的项目归属校验层，在 IAM 鉴权之后执行
- 校验失败时返回明确的错误信息，便于前端展示

### 数据规则

- ClientAttachment.project_id 标识 Client 归属的项目（直接存储）
- ClientQueryAttachment.project_id 标识 ClientQuery 归属的项目（直接存储）
- ClientEvent 不存储 project_id，通过关联 Client 的 project_id 确定归属
- ClientProjectVerified 统一处理 Client 和 ClientQuery 的校验（两者都有直接的 project_id 字段）

## 外部依赖与集成

### 内部组件依赖

| 组件 | 交互方式 | 说明 |
|------|---------|------|
| DataServer | gRPC | GetClientByID 获取 Client 详情（供中间件调用） |
| ConfigServer（自身） | gRPC | GetClient 供中间件调用 |

### 接口契约示例

**GetClient 请求/响应**：

```protobuf
// Request
message GetClientReq {
  uint32 biz_id = 1;
  uint32 client_id = 2;
  uint32 project_id = 3; // additional_bindings 使用
}

// Response
message GetClientResp {
  uint32 code = 1;
  string message = 2;
  pbclient.Client data = 3;
}
```

## 验收标准

### 功能验收

#### Client 相关（10个接口）

- [ ] **AC-001**：Given ClientAttachment Proto 定义已更新 When 序列化/反序列化 Then project_id 字段正确读写
- [ ] **AC-002**：Given 已有 Client 数据 When 调用 GetClient RPC 传入 biz_id 和 client_id Then 返回正确的 Client 详情（含 project_id）
- [ ] **AC-003**：Given 请求新路由 `/biz/{bid}/apps/{aid}/projects/{pid}/clients/{cid}` When cid 属于 pid 则请求正常放行
- [ ] **AC-004**：Given 请求新路由 `/biz/{bid}/apps/{aid}/projects/{pid}/clients/{cid}` When cid 不属于 pid 则返回 400 错误 "client does not belong to the specified project"
- [ ] **AC-005**：Given 请求旧路由 `/biz/{bid}/apps/{aid}/clients/{cid}` When 旧客户端未传 project_id 则通过 checkOrCreateDefaultProjectEnv 正常工作
- [ ] **AC-006**：Given 调用任意 Client 接口使用新路由 When URL 包含 {project_id} 则请求正常路由到对应处理器
- [ ] **AC-007**：Given 全部 10 个 Client RPC 接口 When 检查 Proto 定义 Then 每个 Req 消息都包含 `uint32 project_id` 字段
- [ ] **AC-008**：Given 旧客户端访问旧 Client 路由 When 不传 project_id Then 行为与改造前完全一致（向后兼容）

#### ClientQuery 相关（5个接口）

- [ ] **AC-009**：Given ClientQueryAttachment Proto 定义已更新 When 序列化/反序列化 Then project_id 字段正确读写
- [ ] **AC-010**：Given 调用任意 ClientQuery 接口使用新路由 When URL 包含 {project_id} 则请求正常路由到对应处理器且按 project_id 过滤
- [ ] **AC-011**：Given 全部 5 个 ClientQuery RPC 接口 When 检查 Proto 定义 Then 每个 Req 消息都包含 `uint32 project_id` 字段
- [ ] **AC-012**：Given 旧客户端访问旧 ClientQuery 路由 When 不传 project_id Then 行为与改造前完全一致（向后兼容）

#### ClientEvent 相关（2+个接口，逻辑关联）

- [ ] **AC-013**：Given 调用 PushClientEvent 使用新路由（含 {project_id}）When 传入的 client_id 属于该 project_id 则事件推送成功
- [ ] **AC-014**：Given 调用 PushClientEvent 使用新路由（含 {project_id}）When 传入的 client_id 不属于该 project_id 则返回 400 错误或拒绝操作
- [ ] **AC-015**：Given 调用 ListClientEvents 使用新路由 When URL 包含 {project_id} Then 返回的事件列表只包含属于该项目的客户端事件
- [ ] **AC-016**：Given ClientEvent 的 Proto 定义 When 检查 Then **不包含** project_id 字段（保持逻辑关联设计）
- [ ] **AC-017**：Given 旧客户端访问旧 ClientEvent 路由 When 不传 project_id Then 行为与改造前完全一致（向后兼容）

### 性能验收

- [ ] **AC-P01**：ClientProjectVerified 中间件的 P99 耗时 ≤ 10ms

### 安全验收

- [ ] **AC-S01**：不同项目间的客户端无法通过新路由跨项目访问
- [ ] **AC-S02**：不同项目间的客户端查询无法通过新路由跨项目访问
- [ ] **AC-S03**：IAM 鉴权逻辑不受影响，原有权限控制仍然生效
- [ ] **AC-S04**：ClientEvent 的逻辑关联校验能够正确拦截跨项目的事件操作

## 边界范围

### 本期包含

- ✅ ClientAttachment Proto 定义新增 project_id 字段（=4）
- ✅ ClientQueryAttachment Proto 定义新增 project_id 字段（=3）
- ✅ pkg/dal/table/client.go 和 client_query.go 结构体新增 ProjectID 字段
- ✅ 新增 GetClient RPC 接口（ConfigServer + DataServer 实现）
- ✅ 新增 ClientProjectVerified 校验中间件（统一覆盖 Client 和 ClientQuery）
- ✅ 全部 10 个 Client 相关 RPC 接口的 additional_bindings 路由
- ✅ 全部 5 个 ClientQuery 相关 RPC 接口的 additional_bindings 路由
- ✅ ClientEvent 相关接口（2+个）的 additional_bindings 路由 + 业务层逻辑关联校验
- ✅ 双轨制路由注册（新路由 + 旧路由兼容）
- ✅ 各 Request 消息的 project_id 字段扩展（Client 和 ClientQuery 必须加，ClientEvent 可选但建议加）
- ✅ Proto 代码重新生成（make proto）

### 本期不包含

- ❌ ClientEvent Proto 定义更新（保持逻辑关联设计，不直接存储 project_id）
- ❌ 前端界面改造
- ❌ 数据库 migration 脚本（clients/project_id 和 client_querys/project_id 列已存在）
- ❌ 其他资源的二次改造（已在兄弟需求中完成或不在范围）
- ❌ Client 与 App 的多对一关系变更（本次仅增加 project_id 维度）
- ❌ ClientEvent 的独立中间件（采用业务层校验方案）

### 涉及接口清单（共 17 个）

| 类别 | RPC 接口名称 | 数量 |
|------|-------------|------|
| **Client** | ListClients, ClientConfigVersionStatistics, ClientPullTrendStatistics, ClientPullStatistics, ClientLabelStatistics, ClientAnnotationStatistics, ClientVersionStatistics, ListClientLabelAndAnnotation, ClientSpecificFailedReason, RetryClients | **10** |
| **ClientQuery** | ListClientQueries, ClientQueryTrendStatistics, ClientQueryCostTimeRanking, ClientQueryFailedReason, ClientQuerySlowQueryTop | **5** |
| **ClientEvent** | PushClientEvent, ListClientEvents | **2**（可能更多待确认） |
| **合计** | | **17+** |

## 约束条件

- **技术限制**：必须复用 Group/Hook/TemplateSpace/TemplateVariable 已有的 project_id 适配模式（中间件 + additional_bindings），保持架构一致性
- **兼容限制**：旧路由不可破坏，必须保持 checkOrCreateDefaultProjectEnv 行为不变
- **依赖限制**：依赖 migration 已执行完毕（clients/project_id 和 client_querys/project_id 列已存在）
- **中间件复用约束**：Client 和 ClientQuery 必须共用同一个 ClientProjectVerified 中间件，不允许分别建两个
- **ClientEvent 特殊约束**：ClientEvent 采用逻辑关联方案，不在 Proto 层增加 project_id，不在路由层面挂载中间件，而是在业务层进行归属校验

## 未解决问题

| 问题 ID | 问题描述 | 截止日期 | 状态 |
|--------|---------|---------|------|
| Q-001 | ClientEvent 除了 PushClientEvent 和 ListClientEvents 外是否还有其他接口需要改造？ | 待确认 | 待确认 |
| Q-002 | ClientQuery 是否有类似 `{client_query_id}` 的单个资源路由需要挂载中间件？还是全部都是列表类接口？ | 待确认 | 待确认 |

---

## 原需求描述

> （无描述内容）

## 澄清记录

### 第 1 轮澄清

**提问时间**：2026-07-10 09:22

**Agent 提问**：

#### 🔴 阻塞性问题（需要先确认才能继续）

1. **[DataModel-001]** 数据模型细节确认
   > 您提到 `client_events` 通过"逻辑关联"，请确认具体的关联方式：
   > - A️⃣ client_events → 关联 client_id → 通过 client 的 `project_id` 确定归属（推荐）
   > - B️⃣ client_events → 关联其他字段（请说明）
   >
   > 同时请确认：
   > - `clients.project_id` 和 `client_querys.project_id` 字段的类型（uint32?）、是否允许为空（默认值 0 表示无归属？）
   > - 这两个字段是通过 migration 已经添加完成，还是本次需求的一部分？

2. **[Middleware-001]** 中间件命名与职责范围
   > 类似 TemplateVariableProjectVerified，计划新建中间件：
   > - A️⃣ 新建统一的 `ClientProjectVerified` 中间件，同时覆盖 `clients` 和 `client_querys` 两个资源的校验（推荐）
   > - B️⃣ 分别建两个中间件：`ClientProjectVerified` + `ClientQueryProjectVerified`
   >
   > 对于 `client_events`（逻辑关联）：
   > - C️⃣ 不单独建中间件，在业务层通过查询 client 的 project_id 来校验
   > - D️⃣ 也挂载 `ClientProjectVerified` 中间件（内部会先查 client 再校验）

#### 🟡 建议确认的问题（可以先用默认假设）

3. **[Interface-001]** 需要改造的 RPC 接口范围
   > 如果不确定，我将假设：**所有涉及 clients/client_querys/client_events 的读写接口都需要改造**
   >
   > 特别确认：
   > - CreateClient / UpdateClient / DeleteClient / GetClient / ListClients 是否都在范围内？
   > - ClientQuery 相关接口（ListClientQueries 等）是否也在范围内？
   > - ClientEvent 相关接口（PushEvent 等）是否在范围内？还是只读不需要校验？

4. **[Proto-001]** Proto 消息更新范围
   > 如果不确定，我将假设需要更新的消息包括：
   > - **Client 相关**：`ClientAttachment` 或 `Client` 消息新增 `uint32 project_id` 字段，所有 Client 相关 Request 消息新增 `uint32 project_id` 字段
   > - **ClientQuery 相关**：`ClientQueryAttachment` 或 `ClientQuery` 消息新增 `uint32 project_id` 字段，所有 ClientQuery 相关 Request 消息新增 `uint32 project_id` 字段
   > - **ClientEvent 相关**：
   >   - E️⃣ 不更新 Proto（因为逻辑关联，不直接存 project_id）
   >   - F️⃣ 也新增 `uint32 project_id` 字段（冗余存储便于查询）
   >
   > 请确认哪些 Proto 消息需要更新？

5. **[RPC-001]** 是否需要新增 GetClient RPC 接口
   > 参照模板变量的实现（F-003），中间件需要一个 RPC 接口来查询资源详情以校验归属。
   > - G️⃣ 需要新增 `GetClient` RPC 接口供中间件调用（推荐）
   > - H️⃣ 已有现成的 GetClient 接口可以直接复用

6. **[Route-001]** 路由注册策略
   > 参照模板变量的双轨制策略：
   > - I️⃣ 确认采用双轨制（新路由 + 旧路由兼容）（推荐）
   > - J️⃣ 只添加新路由，不保留旧路由（破坏性变更，需确认兼容性影响）

**用户回复**：

1. ✅ 确认 **选项A️⃣**：client_events 通过 client_id 关联到 client 的 project_id，project_id 类型 uint32，默认值 0，migration 已完成
2. ✅ 确认 **选项A️⃣ + C️⃣**：新建统一的 ClientProjectVerified 中间件 + client_events 不单独建中间件，采用业务层校验
3. ✅ 确认 **全部需要**：所有涉及 clients/client_querys/client_events 的读写接口都需要改造
4. ✅ 确认 **选项E️⃣**：ClientEvent 不更新 Proto（保持逻辑关联设计）
5. ✅ 确认 **选项G️⃣**：需要新增 GetClient RPC 接口供中间件调用
6. ✅ 确认 **选项I️⃣**：采用双轨制（新路由 + 旧路由兼容）
