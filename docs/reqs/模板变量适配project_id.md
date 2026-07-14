# 【项目和环境】模板变量全链路适配 project_id

## 基本信息

| 字段 | 值 |
|------|-----|
| 需求 ID | 1120451610135948333（短 ID: 135948333） |
| 需求名称 | 【项目和环境】模板变量全链路适配 project_id |
| 优先级 | High |
| 父需求 | 1020451610134444229（BSCP 承接游戏业务场景需求） |
| 创建时间 | 2026-07-09 11:06:11 |
| 原始需求文档 | docs/reqs/模板变量适配project_id.md |

> 脱敏要求：基本信息只保留 TAPD 数字 ID（完整 19 位 ID 与短 ID），不写入处理人/负责人等真实人名，
> 也不写入 TAPD 内网域名链接。

## 需求背景

### 业务背景

BSCP 系统正在引入「项目」和「环境」两个新维度以承接游戏业务场景。此前 **Group** 和 **TemplateSpace** 已完成 `project_id` 适配（新增对应的 `ProjectVerified` 中间件 + `additional_bindings` 路由）。**TemplateVariable** 作为独立的核心资源实体，需要同步进行 `project_id` 适配。

**当前状态**：
- `template_variables` 表已在数据库 migration 中新增了 `project_id` 列（migration 20260529100000）
- 但 Proto 定义中的 `TemplateVariableAttachment` 尚未更新 `project_id` 字段（目前只有 `biz_id`）
- 无项目归属校验中间件
- 无带 `project_id` 的新路由

**核心问题**：
- 模板变量缺少项目归属关系的代码层校验
- 无法在 API 层面校验模板变量资源是否属于指定的 Project
- 需要同时兼容旧路由（无 `project_id`）和新路由（带 `project_id`）

### 用户故事

作为 BSCP 平台开发者
我想要为模板变量新增项目归属校验能力（通过 TemplateVariableProjectVerified 中间件 + additional_bindings 路由）
以便于模板变量能正确归属于某个 Project，支持多项目场景下的资源隔离和数据安全

### 需求来源

- **需求渠道**：产品规划（父需求：BSCP 承接游戏业务场景需求）
- **关联需求**：
  - 父需求：1020451610134444229（BSCP 承接游戏业务场景需求）
  - 兄弟需求：1120451610135611515（模板功能适配 project_id）
  - 前置依赖：1120451610135156099（Group 适配 project_id）
- **参考资料**：
  - 已完成的 Group/Hook/TemplateSpace 的 project_id 适配实现模式
  - docs/reqs/模板适配project_id.md（TemplateSpace 完整实现方案）

## 功能需求

### 核心功能点

| 功能编号 | 功能描述 | 优先级 | 涉及角色 | 备注 |
|---------|---------|--------|---------|------|
| F-001 | TemplateVariableAttachment Proto 新增 project_id 字段 | P0 | 开发者 | 必须（DB已有列） |
| F-002 | pkg/dal/table/template_variable.go 结构体新增 ProjectID 字段 | P0 | 开发者 | 必须 |
| F-003 | 新增 GetTemplateVariable RPC 接口（供中间件调用） | P0 | 开发者 | 必须 |
| F-004 | 新增 TemplateVariableProjectVerified 校验中间件 | P0 | 开发者 | 必须 |
| F-005 | TemplateVariable 相关接口新增带 project_id 的 additional_bindings 路由 | P0 | 开发者 | 全部 8 个接口 |
| F-006 | API Server 注册新旧两套模板变量路由 | P0 | 开发者 | 双轨制 |

### 详细功能描述

#### [F-001] TemplateVariable 数据模型更新（Proto 层）

- **输入**：Proto 定义修改
- **处理逻辑**：
  1. 在 `pkg/protocol/core/template-variable/template_variable.proto` 的 `TemplateVariableAttachment` 消息中新增 `uint32 project_id = 2`
  2. 在 `pkg/dal/table/template_variable.go` 的 `TemplateVariableAttachment` struct 中新增 `ProjectID uint32` 字段（对应 gorm tag）
  3. 执行 `make proto` 重新生成 Proto 代码
- **输出**：数据模型支持 project_id 归属字段
- **边界条件**：project_id 为 uint32 类型，默认值为 0
- **异常处理**：无需额外校验（由上层业务保证）

> **明确说明**：数据库层面的 `template_variables.project_id` 列已通过 migration 20260529100000 添加并回填完成，本次仅更新代码层的 Proto/Table 定义使其对齐。

#### [F-002] Table 结构体更新

- **输入**：Go 结构体修改
- **处理逻辑**：
  1. 在 `pkg/dal/table/template_variable.go` 的 `TemplateVariableAttachment` struct 中新增字段：
     ```go
     type TemplateVariableAttachment struct {
         BizID    uint32 `gorm:"column:biz_id" json:"biz_id"`
         ProjectID uint32 `gorm:"column:project_id" json:"project_id"` // 新增
     }
     ```
  2. 确保 gorm tag 与数据库列名一致
- **输出**：ORM 层支持 project_id 字段的读写

#### [F-003] 新增 GetTemplateVariable RPC

- **输入**：GetTemplateVariableReq（biz_id, template_variable_id, project_id）
- **处理逻辑**：
  1. ConfigServer 新增 `GetTemplateVariable` RPC 方法
  2. 内部调用 DataServer 的 `GetTemplateVariableByID` 接口
  3. 校验返回的 TemplateVariable.Attachment.ProjectID 是否与请求的 project_id 一致
  4. 返回完整的 TemplateVariable 信息
- **输出**：GetTemplateVariableResp（包含 pbtv.TemplateVariable data）

> **设计说明**：该接口主要供 `TemplateVariableProjectVerified` 中间件调用，用于校验模板变量的项目归属关系。

#### [F-004] TemplateVariableProjectVerified 中间件

- **功能**：类似已有的 `TemplateSpaceProjectVerified`，校验 URL 中的 template_variable_id 对应的 TemplateVariable 是否属于 kt.ProjectID 指定的项目
- **前置条件**：kt.ProjectID 已被赋值（通过 VerifyProjectExists 或 checkOrCreateDefaultProjectEnv）
- **适用范围**：用于所有含 `{template_variable_id}` 动态参数的新路由
- **处理逻辑**：
  1. 从 URL 参数提取 template_variable_id
  2. 调用 config-server GetTemplateVariable 接口（传入 biz_id, template_variable_id, project_id）
  3. 若返回错误 → 返回 400 "template_variable does not belong to the specified project"
  4. 若成功 → 放行到下一个 handler
- **错误响应**：400 Bad Request + 明确错误消息

> **设计说明**：TemplateVariable 是独立模块，不归属任何 TemplateSpace，因此需要独立的 `TemplateVariableProjectVerified` 中间件，
> 直接通过自身的 `project_id` 进行归属校验。

#### [F-005] TemplateVariable 接口 additional_bindings 路由

以下 8 个 RPC 接口均需新增带 `{project_id}` 的 additional_bindings 路由：

| RPC | HTTP 方法 | 原有路由 | 新增路由 (additional_bindings) |
|-----|----------|---------|-------------------------------|
| CreateTemplateVariable | POST | /biz/{biz_id}/template_variables | POST /biz/{biz_id}/projects/{project_id}/template_variables |
| DeleteTemplateVariable | DELETE | /biz/{biz_id}/template_variables/{id} | DELETE .../projects/{project_id}/template_variables/{id} |
| UpdateTemplateVariable | PUT | /biz/{biz_id}/template_variables/{id} | PUT .../projects/{project_id}/template_variables/{id} |
| BatchDeleteTemplateVariable | POST | /biz/{biz_id}/template_variables/batch_delete | POST .../projects/{project_id}/.../batch_delete |
| ListTemplateVariables | GET | /biz/{biz_id}/template_variables | GET /biz/{biz_id}/projects/{project_id}/template_variables |
| ImportTemplateVariables | POST | /biz/{biz_id}/template_variables/import | POST .../projects/{project_id}/.../import |
| ImportOtherFormatTemplateVariables | POST | /biz/{biz_id}/template_variables/import_other_format | POST .../projects/{project_id}/.../import_other_format |
| ConfigTemplateVariable | GET/POST | /biz/{biz_id}/config_template_variables | GET/POST .../projects/{project_id}/config_template_variables |

各 Request 消息新增 `uint32 project_id` 字段（标注为 additional_bindings 使用）。

#### [F-006] 路由注册策略

采用与 Group/Hook/TemplateSpace 一致的双轨制路由：

| 路由类型 | 路径模式 | ProjectID 注入方式 | 中间件 |
|---------|---------|-------------------|--------|
| **新路由** | `/api/v1/config/biz/{biz_id}/projects/{project_id}/template_variables/...` | URL 参数 → VerifyProjectExists | TemplateVariableProjectVerified（含 {template_variable_id} 时） |
| **旧路由(兼容)** | `/api/v1/config/biz/{biz_id}/template_variables/...` | checkOrCreateDefaultProjectEnv | TemplateVariableProjectVerified |

路由注册代码示例（routers.go）：

```go
// 模板变量相关
r.Route("/template_variables", func(r chi.Router) {
    r.Mount("/", p.cfgSvrMux)
    r.Route("/{template_variable_id}", func(r chi.Router) {
        r.Use(p.TemplateVariableProjectVerified) // 校验 TemplateVariable 归属于该项目
        r.Mount("/", p.cfgSvrMux)
    })
})
```

关键规则：
- 新路由位于 `/biz/{biz_id}/projects/{project_id}/` 下，先经过 `VerifyProjectExists` 校验
- 旧路由保持原有路径不变（如 `/api/v1/config/biz/{biz_id}/template_variables`），使用 `checkOrCreateDefaultProjectEnv` 自动注入默认 ProjectID
- 含 `{template_variable_id}` 动态参数的路由挂载 `TemplateVariableProjectVerified` 中间件
- 列表类等不含单个资源 ID 的静态路由不挂载该中间件（但 Request 中仍需携带 project_id 用于过滤）

## 非功能需求

### 兼容性

- **向后兼容**：旧路由完全保留，行为不变
- **Proto 兼容性**：新增字段使用递增序号（=2），不影响现有序列化
- **数据库**：✅ 无需额外 migration（template_variables 表的 project_id 列已存在）

### 性能需求

- **响应时间**：新增的 TemplateVariableProjectVerified 中间件增加的单次 RPC 调用耗时 ≤ 10ms（P99）
- **并发能力**：无特殊要求，沿用现有连接池配置

### 安全需求

- **权限控制**：所有模板变量接口维持现有 IAM 鉴权逻辑不变
- **归属校验**：TemplateVariableProjectVerified 是额外的项目归属校验层，在 IAM 鉴权之后执行
- **数据隔离**：确保不同项目间的模板变量资源不可越权访问

## 业务规则

### 数据模型关系

```
template_variables (有 project_id，完全独立模块)

注意：template_variables 与 template_spaces 无直接关联关系，
它通过自身的 project_id 字段确定项目归属。
```

### 权限规则

- 所有 TemplateVariable 接口维持现有 IAM 鉴权逻辑不变
- TemplateVariableProjectVerified 是额外的项目归属校验层，在 IAM 鉴权之后执行
- 校验失败时返回明确的错误信息，便于前端展示

### 数据规则

- TemplateVariableAttachment.project_id 标识 TemplateVariable 归属的项目
- TemplateVariable 是独立模块，不依赖 TemplateSpace 的归属关系
- 通过 TemplateVariable 自身的 project_id 字段直接进行归属校验

## 外部依赖与集成

### 内部组件依赖

| 组件 | 交互方式 | 说明 |
|------|---------|------|
| DataServer | gRPC | GetTemplateVariableByID 获取 TemplateVariable 详情 |
| ConfigServer（自身） | gRPC | GetTemplateVariable 供中间件调用 |

### 接口契约示例

**GetTemplateVariable 请求/响应**：

```protobuf
// Request
message GetTemplateVariableReq {
  uint32 biz_id = 1;
  uint32 template_variable_id = 2;
  uint32 project_id = 3; // additional_bindings 使用
}

// Response
message GetTemplateVariableResp {
  uint32 code = 1;
  string message = 2;
  pbtv.TemplateVariable data = 3;
}
```

## 验收标准

### 功能验收

- [ ] **AC-001**：Given TemplateVariableAttachment Proto 定义已更新 When 序列化/反序列化 Then project_id 字段正确读写
- [ ] **AC-002**：Given 已有 TemplateVariable 数据 When 调用 GetTemplateVariable RPC 传入 biz_id 和 template_variable_id Then 返回正确的 TemplateVariable 详情（含 project_id）
- [ ] **AC-003**：Given 请求新路由 `/biz/{bid}/projects/{pid}/template_variables/{tvid}` When tvid 属于 project_id 则请求正常放行
- [ ] **AC-004**：Given 请求新路由 `/biz/{bid}/projects/{pid}/template_variables/{tvid}` When tvid 不属于 project_id 则返回 400 错误 "template_variable does not belong to the specified project"
- [ ] **AC-005**：Given 请求旧路由 `/biz/{bid}/template_variables/{tvid}` When 旧客户端未传 project_id 则通过 checkOrCreateDefaultProjectEnv 正常工作
- [ ] **AC-006**：Given 调用 CreateTemplateVariable 使用新路由（含 project_id）When 请求体包含 project_id 则创建的 TemplateVariable 归属于指定项目
- [ ] **AC-007**：Given 调用任意 TemplateVariable 接口使用新路由 When URL 包含 {project_id} 则请求正常路由到对应处理器
- [ ] **AC-008**：Given 全部 8 个 TemplateVariable RPC 接口 When 检查 Proto 定义 Then 每个 Req 消息都包含 `uint32 project_id` 字段
- [ ] **AC-009**：Given 旧客户端访问旧路由 When 不传 project_id Then 行为与改造前完全一致（向后兼容）
- [ ] **AC-010**：Given 调用 ImportTemplateVariables / ImportOtherFormatTemplateVariables 使用新路由 When URL 包含 {project_id} Then 导入的模板变量归属于指定项目

### 性能验收

- [ ] **AC-P01**：TemplateVariableProjectVerified 中间件的 P99 耗时 ≤ 10ms

### 安全验收

- [ ] **AC-S01**：不同项目间的模板变量无法通过新路由跨项目访问
- [ ] **AC-S02**：IAM 鉴权逻辑不受影响，原有权限控制仍然生效

## 边界范围

### 本期包含

- ✅ TemplateVariableAttachment Proto 定义新增 project_id 字段
- ✅ pkg/dal/table/template_variable.go 结构体新增 ProjectID 字段
- ✅ 新增 GetTemplateVariable RPC 接口（ConfigServer + DataServer 实现）
- ✅ 新增 TemplateVariableProjectVerified 校验中间件
- ✅ 全部 8 个 TemplateVariable 相关 RPC 接口的 additional_bindings 路由
- ✅ 双轨制路由注册（新路由 + 旧路由兼容）
- ✅ 各 Request 消息的 project_id 字段扩展
- ✅ Proto 代码重新生成（make proto）

### 本期不包含

- ❌ TemplateVariable 与 TemplateSpace 的关联关系设计（本次按独立模块处理）
- ❌ 前端界面改造
- ❌ 数据库 migration 脚本（template_variables.project_id 列已存在）
- ❌ 其他模板资源的二次改造（已在兄弟需求中完成或不在范围）
- ❌ AppTemplateVariable 引用的 project_id 适配（如有需要可单独处理）

### 涉及接口清单（共 8 个）

| 类别 | RPC 接口名称 | 数量 |
|------|-------------|------|
| TemplateVariable | CreateTemplateVariable, DeleteTemplateVariable, UpdateTemplateVariable, BatchDeleteTemplateVariable, ListTemplateVariables, ImportTemplateVariables, ImportOtherFormatTemplateVariables, ConfigTemplateVariable | 8 |
| **合计** | | **8** |

## 约束条件

- **技术限制**：必须复用 Group/Hook/TemplateSpace 已有的 project_id 适配模式（中间件 + additional_bindings），保持架构一致性
- **兼容限制**：旧路由不可破坏，必须保持 checkOrCreateDefaultProjectEnv 行为不变
- **依赖限制**：依赖 migration 20260529100000 已执行完毕（template_variables 表已有 project_id 列）
- **独立性约束**：TemplateVariable 作为独立模块，不应引入对 TemplateSpace 的归属依赖

## 未解决问题

| 问题 ID | 问题描述 | 截止日期 | 状态 |
|--------|---------|---------|------|
| 无 | - | - | - |

---

## 原需求描述

> （无描述内容）

## 澄清记录

### 第 1 轮澄清

**提问时间**：2026-07-09 11:16

**Agent 提问**：

#### 🔴 阻塞性问题（需要先确认才能继续）

1. **[Name-001]** 中间件命名确认
   > 您提到使用 TemplateSpaceProjectVerified，但这个中间件是用于 TemplateSpace 的。
   > 对于 TemplateVariable，建议新建独立的中间件 **TemplateVariableProjectVerified**。
   >
   > **请确认**：
   > - A️⃣ 新建 TemplateVariableProjectVerified 中间件（推荐）
   > - B️⃣ 复用现有的 TemplateSpaceProjectVerified（需说明如何关联）

2. **[Relation-001]** TemplateVariable 与 TemplateSpace 的关联关系
   > 从 Proto 定义看，TemplateVariableAttachment 只有 biz_id，没有 template_space_id 字段。
   >
   > **请确认**：
   > - A️⃣ TemplateVariable 完全独立，不归属任何 TemplateSpace（直接通过自身的 project_id 校验）
   > - B️⃣ TemplateVariable 从属于 TemplateSpace（需要在 Attachment 中新增 template_space_id 字段）

#### 🟡 建议确认的问题（可以先用默认假设）

3. **[Scope-001]** 接口适配范围
   > 如果不确定，将假设：全部 8 个 TemplateVariable 接口都需要新增 additional_bindings 路由
   >
   > 特别确认：
   > - ConfigTemplateVariable 接口是否在本次范围内？
   > - ImportTemplateVariables / ImportOtherFormatTemplateVariables 这两个导入接口？

4. **[DataModel-001]** Proto 更新细节
   > 如果不确定，将假设：
   > - 在 TemplateVariableAttachment 中新增 uint32 project_id = 2
   > - 同步更新 pkg/dal/table/template_variable.go 的 struct
   > - 执行 make proto 重新生成代码

5. **[Middleware-001]** 中间件实现逻辑
   > 参照现有 TemplateSpaceProjectVerified 的实现，计划如下逻辑...
   >
   > **问题**：是否需要新增 GetTemplateVariable RPC 接口供中间件调用？

6. **[Route-001]** 路由注册位置
   > 从现有代码看，TemplateVariable 的路由应该在 routers.go 的 project 路由组下注册...

**用户回复**：

1. ✅ 确认 **选项A**：新建 TemplateVariableProjectVerified 中间件（独立中间件）
2. ✅ 确认 **选项A**：TemplateVariable 完全独立，不归属任何 TemplateSpace（直接通过自身的 project_id 校验）
3. ✅ 确认 **全部需要**：全部 8 个 TemplateVariable 接口都需要新增 additional_bindings 路由（包括 ConfigTemplateVariable、Import、ImportOtherFormat）
4. ✅ 确认 **正确**：Proto 更新方案正确（TemplateVariableAttachment 新增 project_id + Table 结构体 + make proto）
5. ✅ 确认 **参照 TemplateSpaceProjectVerified 实现**：如果没有合适的获取模板变量接口可以新增一个 GetTemplateVariable RPC
6. ✅ 确认 **正确**：路由结构正确（在 routers.go 的 project 路由组下注册，含 {template_variable_id} 的子路由挂载中间件）
