# BSCP 使用的 IAM V4 网关接口清单

## 适用范围

本文只列 BSCP 主动调用 `bkiam` 网关的接口，即需要在蓝鲸 API 网关为 BSCP 应用申请权限的那部分。

- 调用方均为 `auth-server`：常驻服务用 4 个接口，`migrate` 子命令用 19 个模型管理接口，共 **23 个**。
- 常驻服务的那 4 个接口仅在配置 `iam.version: v4` 时生效，v3 模式下网关客户端为 nil，不会产生任何调用。
- `migrate` 的四个 `*-iam-v4` 子命令与版本开关无关：它们只校验 `iam.v4` 配置是否齐全，从不读 `iam.version`。因此「仍运行 v3、但已预置 V4 配置以便切换前先注册模型」这一常规切换路径下，这 19 个接口照样会被真实调用，网关权限必须在此之前就申请到位。
- 权限中心回调 BSCP 拉取资源实例是反向调用（IAM → BSCP），不需要申请网关权限，因此不在本文范围内。

调用凭证取 `iam.v4.app_code` / `iam.v4.app_secret`，未单独配置时回退到外层 `iam.appCode` / `iam.appSecret`。除网关权限外，权限中心侧接入系统的 `clients` 白名单必须包含该 `app_code`，否则即使网关放行，IAM 应用层仍会拒绝。

下文的「网关接口名」即 `bkiam` 网关上的资源名，申请权限时按此名称勾选。

## 一、运行时接口（4 个）

`auth-server` 常驻进程调用，任一接口无权限都会导致对应功能不可用。

| 用途 | 网关接口名 | 方法与路径 | 调用位置 |
| --- | --- | --- | --- |
| 批量鉴权 | `direct_auth_by_resources` | `POST /api/v1/open/rbac/authorization/systems/{system_id}/auth-by-resources/` | `pkg/iam/v4/auth/auth.go` |
| 生成权限申请链接 | `generate_perm_apply_url` | `POST /api/v1/open/application/permission-apply-urls/` | `cmd/auth-server/service/auth/auth.go` |
| 授予角色权限（创建者授权） | `add_authorization` | `POST /api/v1/open/rbac/mgmt/systems/{system_id}/authorizations/` | `cmd/auth-server/service/auth/auth.go` |
| 获取系统 Auth Token | `retrieve_system_auth_token` | `GET /api/v1/open/rbac/model/systems/{system_id}/auth-token/` | `cmd/auth-server/service/service.go` |

### 1. 批量鉴权 `direct_auth_by_resources`

BSCP 唯一的鉴权入口，所有权限判定都汇聚到这里。请求前先由 `adaptor.Adapt` 把 BSCP 的资源类型与操作映射为 V4 的 `action_id` 与资源实例，命中本地缓存的不再请求网关，其余按 `action_id` 分组、每批最多 20 个资源、以并发度 5 并行发出。

调用量与缓存参数相关，默认缓存 20000 条、TTL 30 秒（`iam.v4.auth_cache_size` / `auth_cache_ttl_seconds`）。该接口不可用时鉴权直接返回错误，等同于 BSCP 全站不可用。

网关上同时存在 `direct_auth`（单资源鉴权）与 `direct_auth_by_actions`（多操作鉴权），BSCP 代码里虽有封装但无生产调用点，无需申请。

### 2. 生成权限申请链接 `generate_perm_apply_url`

用户无权限时生成跳转权限中心的申请链接，由 `GetPermissionToApply` 触发。注意该接口路径不含 `system_id`，系统 ID 在请求体里给出，因此权限申请的网关资源与其他接口不同。

### 3. 授予角色权限 `add_authorization`

服务创建成功后把 `app_operator` 角色授予创建者，有效期 365 天，`X-Bkiam-Operator` 头填创建者本人。

一次创建会发送两条授权记录，不能合并：一次 `add_authorization` 只授予 `related_resource_type_id` 指定的单一授权维度，若只授 `app` 维度，角色内按 `biz` 维度授权的 `find_business_resource` 不会生效；BSCP 每个业务接口都有业务访问前置校验，该操作缺失时创建者对自己新建的服务也会被全部拒绝。

与之配对的 `revoke_authorization`（撤销授权）和 `list_authorization_subject`（查询已授权用户）目前没有调用，暂不需要申请；这意味着创建者授权只能靠 365 天过期自然失效，没有主动回收路径。

### 4. 获取系统 Auth Token `retrieve_system_auth_token`

这个接口不用于调用 IAM，而是反向服务于回调认证：权限中心回调 BSCP 时携带 `Authorization: Basic base64(bk_iam:{token})`，`IAMVerify` 用取回的 token 做比对，结果缓存 1 分钟。无权限时权限中心拉取不到业务与服务实例，授权页面上无法选择资源。

## 二、模型同步接口（19 个）

仅由 `auth-server migrate init-iam-v4` / `diff-iam-v4` / `prune-iam-v4` / `delete-iam-v4` 调用，常驻服务不涉及。同步内容为 4 个资源类型（`biz` → `project` → `environment` → `app`）、25 个操作、13 个角色，定义在 `pkg/iam/v4/model/model.go`。资源类型的祖先链与操作的关联资源类型不可更新，需要变更时由 `delete-iam-v4` 清空后重建。

### 系统

| 用途 | 网关接口名 | 方法与路径 |
| --- | --- | --- |
| 查询系统 | `retrieve_system` | `GET /api/v1/open/rbac/model/systems/{system_id}/` |
| 注册系统 | `create_system` | `POST /api/v1/open/rbac/model/systems/` |
| 更新系统 | `update_system` | `PUT /api/v1/open/rbac/model/systems/{system_id}/` |

更新系统会写入回调地址与 `clients` 白名单。白名单按集合补齐而非整体覆盖，避免冲掉权限中心侧另行添加的调用方。

### 资源类型、操作、角色

以下路径均以 `/api/v1/open/rbac/model/systems/{system_id}` 为前缀。

| 用途 | 网关接口名 | 方法与路径 |
| --- | --- | --- |
| 资源类型：查询 | `list_resource_type` | `GET /resource-types/` |
| 资源类型：批量创建 | `batch_create_resource_type` | `POST /resource-types/` |
| 资源类型：更新 | `update_resource_type` | `PUT /resource-types/{resource_type_id}/` |
| 资源类型：删除 | `delete_resource_type` | `DELETE /resource-types/{resource_type_id}/` |
| 操作：查询 | `list_action` | `GET /actions/` |
| 操作：批量创建 | `batch_create_action` | `POST /actions/` |
| 操作：更新 | `update_action` | `PUT /actions/{action_id}/` |
| 操作：删除 | `delete_action` | `DELETE /actions/{action_id}/` |
| 角色：查询 | `list_role` | `GET /roles/` |
| 角色：批量创建 | `batch_create_role` | `POST /roles/` |
| 角色：更新 | `update_role` | `PUT /roles/{role_id}/` |
| 角色：删除 | `delete_role` | `DELETE /roles/{role_id}/` |
| 角色内操作：批量添加 | `batch_create_role_action` | `POST /roles/{role_id}/actions/` |
| 角色内操作：批量移除 | `batch_delete_role_action` | `DELETE /roles/{role_id}/actions/?ids=a,b` |

`delete_resource_type`、`delete_action`、`delete_role` 只有两个子命令会调用：`prune-iam-v4` 只删线上存在、本地定义已移除的元素；`delete-iam-v4` 不比较差异，把线上全部角色、操作、资源类型一并删掉。两者都必须人工确认后才执行（`prune-iam-v4` 必须交互输入 `yes`，`delete-iam-v4` 可加 `--yes` 跳过交互）。`init-iam-v4` 只做新增与更新。删除角色会使该角色下全部授权失效，删除操作会让引用它的角色失去该权限，均不可逆。

两个删除命令的适用场景不同：本地只是删掉了几项定义时用 `prune-iam-v4`；要改资源类型的祖先链、操作关联的资源类型这类不支持更新的字段时，`prune-iam-v4` 处理不了（`init-iam-v4` / `diff-iam-v4` 只会报冲突），必须先 `delete-iam-v4` 清空再 `init-iam-v4` 重建。`delete-iam-v4` 保留系统本身，回调地址与 `clients` 白名单不受影响，但清空到重建之间线上没有任何操作与角色，鉴权会全部失败，因此必须在同一维护窗口内完成重建。

`batch_delete_role_action` 走 query 参数 `ids` 传逗号连接的操作 ID，而非请求体，与添加接口的形态不同。

### 角色展示层级

| 用途 | 网关接口名 | 方法与路径 |
| --- | --- | --- |
| 查询展示层级 | `list_role_display_resource_types` | `GET /api/v1/open/mgmt/systems/{system_id}/roles/{role_id}/display-resource-types/` |
| 覆盖展示层级 | `update_role_display_resource_types` | `PUT /api/v1/open/mgmt/systems/{system_id}/roles/{role_id}/display-resource-types/` |

这两个接口的路径前缀是 `/api/v1/open/mgmt/`，**不含 `rbac`**，申请网关权限时容易漏掉。`PUT` 是全量覆盖，未传入的关联资源类型会回退为默认值（拓扑树第一层资源类型）后整体校验。

## 三、申请清单（23 个接口名）

运行时必备：

```
direct_auth_by_resources
generate_perm_apply_url
add_authorization
retrieve_system_auth_token
```

模型同步（`migrate` 命令）：

```
retrieve_system
create_system
update_system
list_resource_type
batch_create_resource_type
update_resource_type
delete_resource_type
list_action
batch_create_action
update_action
delete_action
list_role
batch_create_role
update_role
delete_role
batch_create_role_action
batch_delete_role_action
list_role_display_resource_types
update_role_display_resource_types
```

若只申请运行时权限、暂不做模型同步，前 4 个为最小集合；但系统与模型未注册时鉴权无从生效，首次接入必须先具备模型同步权限。

以下接口在 `bkiam` 网关上存在、BSCP 也已封装，但没有生产调用点，本次无需申请：`direct_auth`、`direct_auth_by_actions`、`revoke_authorization`、`list_authorization_subject`。

## 四、请求头约定

| 头 | 取值 | 适用范围 |
| --- | --- | --- |
| `X-Bkapi-Authorization` | `{"bk_app_code":"...","bk_app_secret":"..."}` | 全部接口 |
| `X-Bk-Tenant-Id` | 当前租户 ID，`kit.Kit.TenantID` 非空时携带 | 全部接口 |
| `X-Bkiam-Operator` | 操作人，创建者授权时填创建者本人 | 授权类接口 |

多租户环境下 `X-Bk-Tenant-Id` 不是可选的：应用为 `tenant_mode=global` 时，网关要求该头必须存在且非空，否则返回 `1640302 Cross tenant forbidden`。运行时的租户 ID 来自请求上下文，而 `migrate` 命令没有请求上下文：未传 `--tenant-id` 时按租户模式选择，开启多租户（`featureFlags.enableTenantMode`）用运营租户 `system`，否则用 `default`。权限模型属于平台而非业务租户，这条规则与 IAM 版本无关，V3 的 `init-iam` 同样适用。多租户 IAM 不认 `default`，误用会返回 `Invalid tenant ID default`。

响应处理上有两点需要留意：成功状态码不止 200，创建类返回 201、全量更新与删除类返回 204 且无响应体；错误分网关层与应用层两套体系，前者 `code` 是整数（形如 1640001）、后者是字符串枚举，排障时以响应中的 `request_id` 为准。
