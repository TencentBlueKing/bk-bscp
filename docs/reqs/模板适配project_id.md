# 【项目和环境】模板功能适配 project_id

## 基本信息

| 字段 | 值 |
|------|-----|
| 需求 ID | 1120451610135611515（短 ID: 135156115） |
| 需求名称 | 【项目和环境】模板功能适配 project_id |
| 优先级 | High |
| 父需求 | 1020451610134444229（【重要】BSCP 承接游戏业务场景需求） |
| 创建时间 | 2026-07-02 16:39:00 |
| 原始需求文档 | docs/reqs/模板适配project_id.md |

> 脱敏要求：基本信息只保留 TAPD 数字 ID（完整 19 位 ID 与短 ID），不写入处理人/负责人等真实人名，
> 也不写入 TAPD 内网域名链接。

## 需求背景

### 业务背景

BSCP 系统正在引入「项目」和「环境」两个新维度以承接游戏业务场景。此前 Group 已完成 project_id 适配（新增 GroupProjectVerified 中间件 + additional_bindings 路由）。模板（Template）作为与 Group 同级的核心资源实体，需要同步进行 project_id 适配。

**当前状态**：
- `template_spaces` 和 `template_variables` 表已在数据库 migration 中新增了 `project_id` 列（migration 20260529100000）
- 但 Proto 定义中的 TemplateSpaceAttachment 尚未更新 `project_id` 字段
- `template_sets`、`template_revisions`、`templates` 这三个表**不直接存储 project_id**

**核心问题**：
- 模板相关的 4 张关联表缺少 project_id 归属关系的代码层校验
- 无法在 API 层面校验模板资源是否属于指定的 Project
- 需要同时兼容旧路由（无 project_id）和新路由（带 project_id）

### 用户故事

作为 BSCP 平台开发者
我想要为模板资源新增项目归属校验能力（通过 TemplateSpaceProjectVerified 中间件 + additional_bindings 路由）
以便于模板能正确归属于某个 Project，支持多项目场景下的资源隔离和数据安全

### 需求来源

- **需求渠道**：产品规划（父需求：BSCP 承接游戏业务场景需求）
- **关联需求**：1120451610134444229（父需求）、1120451610135156099（Group 适配 project_id）
- **参考资料**：Group/Hook 已完成的 project_id 适配实现模式

## 功能需求

### 核心功能点

| 功能编号 | 功能描述 | 优先级 | 涉及角色 | 备注 |
|---------|---------|--------|---------|------|
| F-001 | TemplateSpaceAttachment Proto 新增 project_id 字段 | P0 | 开发者 | 必须（DB已有列） |
| F-002 | 新增 GetTemplateSpace RPC 接口（供中间件调用） | P0 | 开发者 | 必须 |
| F-003 | 新增 TemplateSpaceProjectVerified 校验中间件 | P0 | 开发者 | 必须 |
| F-004 | TemplateSpace 相关接口新增带 project_id 的 additional_bindings 路由 | P0 | 开发者 | 必须包含旧路由兼容 |
| F-005 | TemplateSet 相关接口新增带 project_id 的 additional_bindings 路由 | P0 | 开发者 | 通过 space_id 链式校验 |
| F-006 | Template 相关接口新增带 project_id 的 additional_bindings 路由 | P0 | 开发者 | 通过 space_id 链式校验 |
| F-007 | TemplateRevision 相关接口新增带 project_id 的 additional_bindings 路由 | P0 | 开发者 | 通过 space_id 链式校验 |
| F-008 | AppTemplateBinding 相关接口新增带 project_id 的 additional_bindings 路由 | P0 | 开发者 | 通过 space_id 链式校验 |
| F-009 | API Server 注册新旧两套模板路由 | P0 | 开发者 | 双轨制 |

### 详细功能描述

#### [F-001] TemplateSpace 数据模型更新（Proto 层）

- **输入**：Proto 定义修改
- **处理逻辑**：
  1. 在 `pkg/protocol/core/template-space/template_space.proto` 的 `TemplateSpaceAttachment` 消息中新增 `uint32 project_id = 2`
  2. 在 `pkg/dal/table/template_space.go` 的 `TemplateSpaceAttachment` struct 中新增 `ProjectID uint32` 字段（对应 gorm tag）
  3. 执行 `make proto` 重新生成 Proto 代码
- **输出**：数据模型支持 project_id 归属字段
- **边界条件**：project_id 为 uint32 类型，默认值为 0
- **异常处理**：无需额外校验（由上层业务保证）

> **明确说明**：数据库层面的 `template_spaces.project_id` 列已通过 migration 20260529100000 添加并回填完成，本次仅更新代码层的 Proto/Table 定义使其对齐。

#### [F-002] 新增 GetTemplateSpace RPC

- **输入**：GetTemplateSpaceReq（biz_id, template_space_id, project_id）
- **处理逻辑**：
  1. ConfigServer 新增 `GetTemplateSpace` RPC 方法
  2. 内部调用 DataServer 的 `GetTemplateSpaceByID` 接口
  3. 校验返回的 TemplateSpace.Attachment.ProjectID 是否与请求的 project_id 一致
  4. 返回完整的 TemplateSpace 信息
- **输出**：GetTemplateSpaceResp（包含 pbts.TemplateSpace data）

#### [F-003] TemplateSpaceProjectVerified 中间件

- **功能**：类似已有的 `GroupProjectVerified`，校验 URL 中的 template_space_id 对应的 TemplateSpace 是否属于 kt.ProjectID 指定的项目
- **前置条件**：kt.ProjectID 已被赋值（通过 VerifyProjectExists 或 checkOrCreateDefaultProjectEnv）
- **适用范围**：用于所有含 `{template_space_id}` 动态参数的新路由
- **处理逻辑**：
  1. 从 URL 参数提取 template_space_id
  2. 调用 config-server GetTemplateSpace 接口（传入 biz_id, template_space_id, project_id）
  3. 若返回错误 → 返回 400 "template_space does not belong to the specified project"
  4. 若成功 → 放行到下一个 handler
- **错误响应**：400 Bad Request + 明确错误消息

> **设计说明**：由于 `template_sets`、`templates`、`template_revisions` 都通过 `template_space_id` 关联到 `template_spaces`，
> 因此只需要这一个中间件即可完成整个模板链路的归属校验，无需为每张表创建独立的 Verified 中间件。
> 对于不含 `template_space_id` 但含 `template_set_id` / `template_id` / `template_revision_id` 的接口，
> 可在 RPC 内部通过链式查询实现归属校验（如：template_set → template_space → project_id）。

#### [F-004] TemplateSpace 接口 additional_bindings 路由

以下 RPC 接口均需新增带 `{project_id}` 的 additional_bindings 路由：

| RPC | 原有路由 | 新增路由 (additional_bindings) |
|-----|---------|-------------------------------|
| CreateTemplateSpace | POST /biz/{biz_id}/template_spaces | POST /biz/{biz_id}/projects/{project_id}/template_spaces |
| DeleteTemplateSpace | DELETE .../template_spaces/{template_space_id} | DELETE .../projects/{project_id}/template_spaces/{template_space_id} |
| UpdateTemplateSpace | PUT .../template_spaces/{template_space_id} | PUT .../projects/{project_id}/template_spaces/{template_space_id} |
| ListTemplateSpaces | GET /biz/{biz_id}/template_spaces | GET /biz/{biz_id}/projects/{project_id}/template_spaces |

各 Request 消息新增 `uint32 project_id` 字段（标注为 additional_bindings 使用）。

#### [F-005] TemplateSet 接口 additional_bindings 路由

以下 RPC 接口均需新增带 `{project_id}` 的 additional_bindings 路由：

| RPC | 原有路由模式 | 新增路由 (additional_bindings) |
|-----|------------|-------------------------------|
| CreateTemplateSet | POST .../template_sets | POST .../projects/{project_id}/template_sets |
| DeleteTemplateSet | DELETE .../template_sets/{template_set_id} | DELETE .../projects/{project_id}/template_sets/{template_set_id} |
| UpdateTemplateSet | PUT .../template_sets/{template_set_id} | PUT .../projects/{project_id}/template_sets/{template_set_id} |
| ListTemplateSets | GET .../template_sets | GET .../projects/{project_id}/template_sets |
| GetLatestTemplateVersionsInSpace | GET .../latest_template_versions | GET .../projects/{project_id}/.../latest_template_versions |
| ListAppTemplateSets | GET .../app_template_sets | GET .../projects/{project_id}/.../app_template_sets |
| ListTemplateSetsByIDs | POST .../template_sets/list_by_ids | POST .../projects/{project_id}/.../list_by_ids |

> **校验方式**：含 `{template_set_id}` 的路由可挂载 TemplateSpaceProjectVerified 中间件（内部通过 template_set → template_space 链式查询校验），
> 或者在 ConfigServer RPC 实现中增加 project_id 归属校验逻辑。

#### [F-006] Template 接口 additional_bindings 路由

以下 RPC 接口均需新增带 `{project_id}` 的 additional_bindings 路由：

| RPC | 原有路由模式 | 新增路由 (additional_bindings) |
|-----|------------|-------------------------------|
| CreateTemplate | POST .../templates | POST .../projects/{project_id}/templates |
| DeleteTemplate | DELETE .../templates/{template_id} | DELETE .../projects/{project_id}/templates/{template_id} |
| UpdateTemplate | PUT .../templates/{template_id} | PUT .../projects/{project_id}/templates/{template_id} |
| ListTemplates | GET .../templates | GET .../projects/{project_id}/templates |
| ListTemplatesByIDs | POST .../templates/list_by_ids | POST .../projects/{project_id}/.../list_by_ids |
| ListTemplatesNotBound | POST .../templates/not_bound | POST .../projects/{project_id}/.../not_bound |
| ListTemplateByTuple | POST .../templates/list_by_tuple | POST .../projects/{project_id}/.../list_by_tuple |
| ListTemplateSetsAndRevisions | GET .../template_sets_and_revisions | GET .../projects/{project_id}/.../template_sets_and_revisions |

> **校验方式**：同上，通过 template → template_space 链式查询或中间件校验。

#### [F-007] TemplateRevision 接口 additional_bindings 路由

以下 RPC 接口均需新增带 `{project_id}` 的 additional_bindings 路由：

| RPC | 原有路由模式 | 新增路由 (additional_bindings) |
|-----|------------|-------------------------------|
| CreateTemplateRevision | POST .../template_revisions | POST .../projects/{project_id}/template_revisions |
| UpdateTemplateRevision | PUT .../template_revisions/{template_revision_id} | PUT .../projects/{project_id}/.../{template_revision_id} |
| ListTemplateRevisions | GET .../template_revisions | GET .../projects/{project_id}/template_revisions |
| GetTemplateRevision | GET .../template_revisions/{template_revision_id} | GET .../projects/{project_id}/.../{template_revision_id} |
| DeleteTemplateRevision | DELETE .../template_revisions/{template_revision_id} | DELETE .../projects/{project_id}/.../{template_revision_id} |
| ListTemplateRevisionsByIDs | POST .../template_revisions/list_by_ids | POST .../projects/{project_id}/.../list_by_ids |

#### [F-008] AppTemplateBinding 接口 additional_bindings 路由

以下 RPC 接口均需新增带 `{project_id}` 的 additional_bindings 路由：

| RPC | 原有路由模式 | 新增路由 (additional_bindings) |
|-----|------------|-------------------------------|
| CreateAppTemplateBinding | POST .../app_template_bindings | POST .../projects/{project_id}/app_template_bindings |
| DeleteAppTemplateBinding | DELETE .../app_template_bindings | DELETE .../projects/{project_id}/app_template_bindings |
| UpdateAppTemplateBinding | PUT .../app_template_bindings | PUT .../projects/{project_id}/app_template_bindings |
| ListAppTemplateBindings | GET .../app_template_bindings | GET .../projects/{project_id}/app_template_bindings |

#### [F-009] 路由注册策略

采用与 Group/Hook 一致的双轨制路由：

| 路由类型 | 路径模式 | ProjectID 注入方式 | 中间件 |
|---------|---------|-------------------|--------|
| **新路由** | `/api/v1/config/biz/{biz_id}/projects/{project_id}/template_spaces/...` | URL 参数 → VerifyProjectExists | TemplateSpaceProjectVerified（含 {template_space_id} 时） |
| **新路由** | `/api/v1/config/biz/{biz_id}/projects/{project_id}/template_sets/...` | URL 参数 → VerifyProjectExists | 链式校验或 TemplateSpaceProjectVerified |
| **新路由** | `/api/v1/config/biz/{biz_id}/projects/{project_id}/templates/...` | URL 参数 → VerifyProjectExists | 链式校验或 TemplateSpaceProjectVerified |
| **新路由** | `/api/v1/config/biz/{biz_id}/projects/{project_id}/template_revisions/...` | URL 参数 → VerifyProjectExists | 链式校验或 TemplateSpaceProjectVerified |
| **旧路由(兼容)** | `/api/v1/config/biz/{biz_id}/template_spaces/...` | checkOrCreateDefaultProjectEnv | TemplateSpaceProjectVerified |

关键规则：
- 新路由位于 `/biz/{biz_id}/projects/{project_id}/` 下，先经过 `VerifyProjectExists` 校验
- 旧路由保持原有路径不变（如 `/api/v1/config/biz/{biz_id}/template_spaces`），使用 `checkOrCreateDefaultProjectEnv` 自动注入默认 ProjectID
- 含 `{template_space_id}` 动态参数的路由挂载 `TemplateSpaceProjectVerified` 中间件
- 含 `{template_set_id}/{template_id}/{template_revision_id}` 的路由可通过链式查询校验或在 RPC 内部校验
- 列表类等不含单个资源 ID 的静态路由不挂载该中间件（但 Request 中仍需携带 project_id 用于过滤）

## 非功能需求

### 兼容性

- **向后兼容**：旧路由完全保留，行为不变
- **Proto 兼容性**：新增字段使用递增序号（=2），不影响现有序列化
- **数据库**：✅ 无需额外 migration（template_spaces 表的 project_id 列已存在）

### 性能需求

- **响应时间**：新增的 TemplateSpaceProjectVerified 中间件增加的单次 RPC 调用耗时 ≤ 10ms（P99）
- **并发能力**：无特殊要求，沿用现有连接池配置

### 安全需求

- **权限控制**：所有模板接口维持现有 IAM 鉴权逻辑不变
- **归属校验**：TemplateSpaceProjectVerified 是额外的项目归属校验层，在 IAM 鉴权之后执行
- **数据隔离**：确保不同项目间的模板资源不可越权访问

## 业务规则

### 数据模型关系

```
template_spaces (有 project_id)
    ├── template_sets (通过 template_space_id 关联，无 project_id)
    │       └── templates (通过 template_space_id 关联，无 project_id)
    └── templates (通过 template_space_id 关联，无 project_id)
            └── template_revisions (通过 template_id → template_space_id 关联，无 project_id)

template_variables (有 project_id，独立模块，不在本次改造范围内)
```

### 权限规则

- 所有 Template/TemplateSet/TemplateRevision/AppTemplateBinding 接口维持现有 IAM 鉴权逻辑不变
- TemplateSpaceProjectVerified 是额外的项目归属校验层，在 IAM 鉴权之后执行
- 校验失败时返回明确的错误信息，便于前端展示

### 数据规则

- TemplateSpaceAttachment.project_id 标识 TemplateSpace 归属的项目
- template_sets/templates/template_revisions 不存储 project_id，通过关联的 template_space_id 间接确定归属
- template_variables 表虽然已有 project_id 字段，但它是独立模块，本次不做路由适配（后续如有需要可单独处理）

## 外部依赖与集成

### 内部组件依赖

| 组件 | 交互方式 | 说明 |
|------|---------|------|
| DataServer | gRPC | GetTemplateSpaceByID 获取 TemplateSpace 详情 |
| ConfigServer（自身） | gRPC | GetTemplateSpace 供中间件调用 |

### 接口契约示例

**GetTemplateSpace 请求/响应**：

```protobuf
// Request
message GetTemplateSpaceReq {
  uint32 biz_id = 1;
  uint32 template_space_id = 2;
  uint32 project_id = 3; // additional_bindings 使用
}

// Response
message GetTemplateSpaceResp {
  uint32 code = 1;
  string message = 2;
  pbts.TemplateSpace data = 3;
}
```

## 验收标准

### 功能验收

- [ ] **AC-001**：Given TemplateSpaceAttachment Proto 定义已更新 When 序列化/反序列化 Then project_id 字段正确读写
- [ ] **AC-002**：Given 已有 TemplateSpace 数据 When 调用 GetTemplateSpace RPC 传入 biz_id 和 template_space_id Then 返回正确的 TemplateSpace 详情（含 project_id）
- [ ] **AC-003**：Given 请求新路由 `/biz/{bid}/projects/{pid}/template_spaces/{tsid}` When tsid 属于 project_id 则请求正常放行
- [ ] **AC-004**：Given 请求新路由 `/biz/{bid}/projects/{pid}/template_spaces/{tsid}` When tsid 不属于 project_id 则返回 400 错误 "template_space does not belong to the specified project"
- [ ] **AC-005**：Given 请求旧路由 `/biz/{bid}/template_spaces/{tsid}` When 旧客户端未传 project_id 则通过 checkOrCreateDefaultProjectEnv 正常工作
- [ ] **AC-006**：Given 调用 CreateTemplateSpace 使用新路由（含 project_id）When 请求体包含 project_id 则创建的 TemplateSpace 归属于指定项目
- [ ] **AC-007**：Given 调用 TemplateSet/Template/TemplateRevision/AppTemplateBinding 的任意接口使用新路由 When URL 包含 {project_id} 则请求正常路由到对应处理器
- [ ] **AC-008**：Given 通过 template_set_id/template_id/template_revision_id 访问资源 When 该资源所属的 template_space 不属于指定项目 Then 返回 400 归属错误
- [ ] **AC-009**：Given 所有 29 个模板相关 RPC 接口 When 检查 Proto 定义 Then 每个 Req 消息都包含 `uint32 project_id` 字段
- [ ] **AC-010**：Given 旧客户端访问旧路由 When 不传 project_id Then 行为与改造前完全一致（向后兼容）

### 性能验收

- [ ] **AC-P01**：TemplateSpaceProjectVerified 中间件的 P99 耗时 ≤ 10ms

### 安全验收

- [ ] **AC-S01**：不同项目间的模板资源无法通过新路由跨项目访问
- [ ] **AC-S02**：IAM 鉴权逻辑不受影响，原有权限控制仍然生效

## 边界范围

### 本期包含

- ✅ TemplateSpaceAttachment Proto 定义新增 project_id 字段
- ✅ pkg/dal/table/template_space.go 结构体新增 ProjectID 字段
- ✅ 新增 GetTemplateSpace RPC 接口（ConfigServer + DataServer 实现）
- ✅ 新增 TemplateSpaceProjectVerified 校验中间件
- ✅ 29 个模板相关 RPC 接口的 additional_bindings 路由（见下表）
- ✅ 双轨制路由注册（新路由 + 旧路由兼容）
- ✅ 各 Request 消息的 project_id 字段扩展
- ✅ Proto 代码重新生成（make proto）

### 本期不包含

- ❌ template_variables 的 project_id 适配（独立模块，后续按需处理）
- ❌ template_sets/templates/template_revisions 表新增 project_id 列（采用链式查询方案）
- ❌ 前端界面改造
- ❌ 数据库 migration 脚本（template_spaces.project_id 列已存在）
- ❌ ConfigTemplate 相关接口（另一套模板系统，不在本次范围）

### 涉及接口清单（共 29 个）

| 类别 | RPC 接口名称 | 数量 |
|------|-------------|------|
| TemplateSpace | CreateTemplateSpace, DeleteTemplateSpace, UpdateTemplateSpace, ListTemplateSpaces | 4 |
| TemplateSet | CreateTemplateSet, DeleteTemplateSet, UpdateTemplateSet, ListTemplateSets, GetLatestTemplateVersionsInSpace, ListAppTemplateSets, ListTemplateSetsByIDs | 7 |
| Template | CreateTemplate, DeleteTemplate, UpdateTemplate, ListTemplates, ListTemplatesByIDs, ListTemplatesNotBound, ListTemplateByTuple, ListTemplateSetsAndRevisions | 8 |
| TemplateRevision | CreateTemplateRevision, UpdateTemplateRevision, ListTemplateRevisions, GetTemplateRevision, DeleteTemplateRevision, ListTemplateRevisionsByIDs | 6 |
| AppTemplateBinding | CreateAppTemplateBinding, DeleteAppTemplateBinding, UpdateAppTemplateBinding, ListAppTemplateBindings | 4 |
| **合计** | | **29** |

## 约束条件

- **技术限制**：必须复用 Group/Hook 已有的 project_id 适配模式（中间件 + additional_bindings），保持架构一致性
- **兼容限制**：旧路由不可破坏，必须保持 checkOrCreateDefaultProjectEnv 行为不变
- **依赖限制**：依赖 migration 20260529100000 已执行完毕（template_spaces 表已有 project_id 列）

## 未解决问题

| 问题 ID | 问题描述 | 截止日期 | 状态 |
|--------|---------|---------|------|
| 无 | - | - | - |

---

## 原需求描述

> 需求澄清tapd单据135156115，按照现有的脚本、分组方式来处理模板相关的功能。模板涉及的表有
> template_spaces、template_sets、template_revisions、templates、template_variables，目前只有template_spaces和template_variables
> 表有新增项目ID字段，template_variables其实是一个单独的模块。template_spaces、template_sets、template_revisions、templates这
> 几个是有关联的。template_sets、template_revisions、templates通过代码来判断是否属于该空间下的，该空间是否来自该业务和该项目，
> 需要有个类似GroupProjectVerified的方法，校验是否属于指定的项目。proto中旧路由不变，新路由放在additional_bindings中。

## 澄清记录

### 第 1 轮澄清

**提问时间**：2026-07-02 16:39

**Agent 提问**：
1. 🔴 **[Data-001]** template_sets/templates/template_revisions 是否需要在DB/Proto层面新增project_id字段？（方案A：不新增，通过链式查询；方案B：新增）
2. 🟡 **[Middleware-001]** TemplateSpaceProjectVerified 的实现方式？是否每个表都需要独立的Verified中间件？
3. 🟡 **[Route-001]** 模板相关接口的范围？是否全部都需要新增additional_bindings路由？
4. 🟡 **[Proto-001]** TemplateSpaceAttachment的Proto定义是否需要更新？

**用户回复**：
1. ✅ 确认**方案A**：template_sets/templates/template_revisions不新增project_id字段，通过代码链式查询校验
2. ✅ 确认**选项A**：只需要TemplateSpaceProjectVerified一个中间件
3. ✅ 确认**全部需要**：所有29个模板相关RPC接口都需要新增additional_bindings路由
4. ✅ 确认**是的**：需要同步更新Proto定义（TemplateSpaceAttachment新增project_id字段 + make proto）
