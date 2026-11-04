# 项目/环境维度权限校验开关

> 对应配置项 `iam.project_environment_auth.enabled`，用于在 IAM 资源同步/刷新存在延迟时，临时关闭项目与环境维度的权限校验，退化回业务维度。

## 一、背景与目标

BSCP 的 IAM 授权模型为四层拓扑：

```
业务（biz） → 项目（project） → 环境（environment） → 服务（app）
```

新增项目或环境后，IAM 侧的资源实例、动作与授权存在同步延迟，可能出现"已经授权但仍被拒绝"的情况，影响业务操作。因此在 auth-server 增加一个开关，按需控制是否校验项目与环境维度：

- 关闭：只校验业务维度，项目/环境维度不参与鉴权；
- 开启：在业务维度基础上追加项目与环境维度校验。

## 二、配置

```yaml
iam:
  # 项目/环境维度权限校验开关，V3、V4 共用
  project_environment_auth:
    # false: 只校验业务维度（默认）
    # true : 追加项目/环境维度校验
    enabled: false
```

| 项 | 说明 |
| --- | --- |
| 默认值 | `false` |
| 环境变量 | `BK_BSCP_IAM_PROJECT_ENV_AUTH`，仅在配置文件未显式配置 `enabled` 时生效；取值非法会导致启动校验失败 |
| 生效范围 | 仅 auth-server 读取，config-server / api-server / data-service 无需配置 |
| 生效方式 | 修改配置后重启 auth-server；上游服务无需重启 |
| 启动日志 | `iam project/environment auth: enabled` 或 `disabled, only business dimension is authorized` |

## 三、授权维度对照

| 资源/操作 | IAM 动作 | 授权维度 |
| --- | --- | --- |
| 业务访问 | `find_business_resource` | 业务 |
| 项目创建 / 项目列表 | `project_create` / `find_business_resource` | 业务 |
| 项目查看 / 编辑 / 删除 | `project_view` / `project_edit` / `project_delete` | 项目 |
| 环境创建 | `environment_create` | 项目 |
| 环境查看 / 编辑 / 删除 | `environment_view` / `environment_edit` / `environment_delete` | 环境 |
| 服务创建 | `app_create` | 环境 |
| 服务克隆 | `app_create` + `app_edit` | 环境 + 业务（请求不带源服务实例，`app_edit` 按业务维度校验） |
| 服务查看 / 编辑 / 删除 / 生成版本 / 上线版本 | `app_view` / `app_edit` / `app_delete` / `release_generate` / `release_publish` | 服务 |

鉴权路径（`_bk_iam_path_`）表示资源的祖先链，IAM 不反查拓扑，路径写到哪一级就只认哪一级的授权：

| 对象 | 开启态路径 | 关闭态路径 |
| --- | --- | --- |
| 项目 | `/biz,{bizID}/` | 不校验 |
| 环境 | `/biz,{bizID}/project,{projectID}/` | 不校验 |
| 服务 | `/biz,{bizID}/project,{projectID}/environment,{envID}/` | `/biz,{bizID}/` |

## 四、开关行为

| 资源/操作 | `enabled: false` | `enabled: true` |
| --- | --- | --- |
| 项目、环境相关操作 | 跳过，不请求权限中心 | 按项目/环境实例校验 |
| 服务创建 | 按业务维度校验：实例仍用目标环境，路径仅 `/biz,{bizID}/` | 按目标环境校验（`ResourceID` 即环境 ID） |
| 服务查看/编辑/删除/发布 | 按服务实例校验，路径仅声明业务 | 按服务实例校验，完整三级路径 |
| 服务列表 / 业务访问 | 业务维度校验，不变 | 业务维度校验，不变 |
| 项目/环境列表权限注解 | 全部返回有权限 | 按实例返回 |
| 无权限申请链接 | 不含项目/环境祖先；`app_create` 只回显业务一层 | 含完整祖先链 |

### 关闭态下服务创建为什么仍用环境实例

`app_create` 在 IAM 中只关联环境实例（没有业务实例定义），用业务实例发起鉴权或申请会被拒绝：

```
1902412  action(app_create) not support to grant instance
```

因此关闭态采用"实例不变、路径退化"的做法：

- 资源实例仍是目标环境（`ResourceID` 即环境 ID），满足模型约束；
- `_bk_iam_path_` 只声明 `/biz,{bizID}/`，项目与环境维度的授权不再参与判断；
- 有业务维度 `app_create` 授权的用户可以建服务，只有业务访问权限的用户依然会被拒绝。

调用方未指定目标环境（`ResourceID` 为 0）时，鉴权仍按业务维度发起；申请链接不会生成 `app_create` 项，避免向权限中心提交不存在的实例。

## 五、IAM 模型要求

| 场景 | 是否需要初始化 |
| --- | --- |
| 关闭开关 | 不需要。开关只影响运行时发送哪些资源，不改模型定义 |
| 打开开关 | 需要 IAM 模型已是新拓扑：项目/环境资源类型与动作、服务三级祖先链、`app_create` 关联环境 |

初始化命令：

```bash
# V3
bk-bscp-authservice migrate init-iam

# V4：先看差异，再原地更新
bk-bscp-authservice migrate diff-iam-v4
bk-bscp-authservice migrate init-iam-v4
```

V4 若 `init-iam-v4` 报"权限中心未接受原地更新"，说明该版本不支持修改 `resource_type_id`，需按提示 `delete-iam-v4` 后重建（会清空 BSCP 在 IAM 的授权，需提前导出重放）。

## 六、实现位置

| 文件 | 职责 |
| --- | --- |
| `pkg/cc/types.go` | 配置结构、默认值、环境变量回退与校验 |
| `pkg/config/frontend.go` | 环境变量名常量 |
| `cmd/auth-server/service/service.go` | 读取开关、传递给鉴权模块、启动日志 |
| `cmd/auth-server/service/auth/auth.go` | `AuthorizeBatch` / `GetPermissionToApply` 按开关决定是否补齐归属，V4 鉴权器注入开关 |
| `cmd/auth-server/service/auth/adaptor.go`、`gen_id.go` | V3 资源映射与申请链接裁剪 |
| `cmd/auth-server/service/auth/app_scope.go` | 关闭态只回查服务归属（归一化业务），跳过项目/环境父级 |
| `pkg/iam/v4/adaptor/adaptor.go`、`pkg/iam/v4/auth/auth.go` | V4 映射与鉴权器开关 |

## 七、验证

1. 启动日志确认开关取值；
2. 关闭态调用 `/api/v1/auth/iam/permission/check`，传入 `{"biz_id":2,"basic":{"type":"app","action":"create"}}`，有业务维度 `app_create` 授权时应返回 `is_allowed=true`，权限中心收到的鉴权请求路径为 `/biz,2/`；只有业务访问权限时应返回 `is_allowed=false`；
3. 关闭态调用项目/环境接口，权限中心无记录；
4. 开启态用带 `project_id` 与 `basic.resource_id`（环境 ID）的请求验证环境维度判定；
5. 开关切换后重启 auth-server，行为立即变化，无缓存残留。

## 八、已知取舍

- 关闭态下服务创建的实例仍是目标环境，若用户恰好被授予该环境维度的 `app_create`，仍会通过（环境维度授权本身就比业务维度更严格，且属于预期内的细化授权，不会因为关闭开关而失效）；
- 调用方未指定目标环境时，关闭态只能按业务维度判定；此时无权限的用户不会拿到 `app_create` 的申请链接；
- 开关是全局的，暂不支持按业务/租户灰度。
