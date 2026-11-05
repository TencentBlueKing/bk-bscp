# 项目/环境维度权限校验开关

> 对应配置项 `iam.project_environment_auth.enabled`，用于在项目/环境资源实例同步存在延迟时，跳过项目与环境资源本身的校验。

## 一、背景与目标

BSCP 的 IAM 授权模型为四层拓扑：

```
业务（biz） → 项目（project） → 环境（environment） → 服务（app）
```

新增项目或环境后，IAM 侧的资源实例与授权存在同步延迟，按项目、环境实例校验会出现"已经授权但仍被拒绝"的情况，影响业务操作。因此在 auth-server 增加一个开关：

- 关闭：项目、环境资源本身不参与鉴权（不请求权限中心）；
- 开启：项目、环境按实例校验。

开关只影响"是否单独校验项目/环境资源"，**不改变服务的授权要求，也不改变任何资源的鉴权路径**。服务的动作（`app_view` 等）在 IAM 模型中只能授权到服务实例，关闭开关不会让只有业务访问权限的用户获得服务权限。

## 二、配置

```yaml
iam:
  # 项目/环境维度权限校验开关，V3、V4 共用
  project_environment_auth:
    # false: 项目/环境资源本身不鉴权（默认），服务等服务级资源仍逐实例校验
    # true : 项目/环境按实例校验
    enabled: false
    # IAM 侧已注册的资源拓扑，与 enabled 相互独立
    # auto        : 启动时探测服务资源的父链（默认）
    # legacy      : 业务 → 服务
    # hierarchical: 业务 → 项目 → 环境 → 服务
    topology: auto
```

| 项 | 说明 |
| --- | --- |
| 默认值 | `enabled=false`、`topology=auto` |
| 环境变量 | `BK_BSCP_IAM_PROJECT_ENV_AUTH`、`BK_BSCP_IAM_RESOURCE_TOPOLOGY`，仅在配置文件未显式配置对应项时生效；取值非法会导致启动校验失败 |
| 生效范围 | 仅 auth-server 读取，config-server / api-server / data-service 无需配置 |
| 生效方式 | 修改配置后重启 auth-server；上游服务无需重启 |
| 启动日志 | `iam project/environment auth: disabled, project/environment resources are not authorized, resource topology: hierarchical` |

### 为什么拓扑与开关是两个维度

开关回答的是"要不要校验项目/环境这两类资源"，拓扑回答的是"权限中心里资源挂在哪一层"。
把两者混在一起（关闭开关就把服务路径裁成 `/biz,{bizID}/`）在新拓扑下会让服务级授权全部匹配不到，
表现为"用户已被授予服务查看，列表注解却全是 false"；反之在旧拓扑下使用三级路径会被权限中心拒绝。
因此路径只由 `topology` 决定，`enabled` 只决定是否跳过项目/环境资源。

`topology=auto` 时 auth-server 启动后探测一次：

- V3 读取 `resource_types[app].parents` 是否包含 `environment`；
- V4 读取 `resource_types[app].ancestors` 是否包含 `environment`。

探测会带上与 `migrate init-iam` / `init-iam-v4` 相同的模型租户（多租户环境为 `system`，单租户环境为 `default`），
权限中心按租户隔离模型，缺少租户头的请求会被拒绝。

探测失败会重试，仍失败则**启动失败**，错误信息提示显式配置 `iam.project_environment_auth.topology`。
这里不做 `legacy` 回退：权限中心已是新拓扑时按旧拓扑裁剪服务路径，会让服务级授权全部匹配不到，
这类故障比启动失败更难排查。权限中心尚未升级模型时应显式配置 `topology: legacy`。

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

鉴权路径（`_bk_iam_path_`）表示资源的祖先链，IAM 不反查拓扑，路径写到哪一级就只认哪一级的授权。
路径层级由 `topology` 决定，与 `enabled` 无关：

| 对象 | `topology: hierarchical` | `topology: legacy` |
| --- | --- | --- |
| 项目 | `/biz,{bizID}/` | 不校验（旧模型无项目资源） |
| 环境 | `/biz,{bizID}/project,{projectID}/` | 不校验（旧模型无环境资源） |
| 服务 | `/biz,{bizID}/project,{projectID}/environment,{envID}/` | `/biz,{bizID}/` |
| 服务创建 | 实例是目标环境，路径 `/biz,{bizID}/project,{projectID}/` | 实例是业务，无路径 |

`enabled: false` 只影响项目/环境这两类资源本身：它们不进入权限中心，不做实例校验。

路径里的祖先一律以回查到的真实归属为准，与 `enabled` 无关：服务按服务实例回查业务/项目/环境，
服务创建按目标环境回查业务/项目。只跳过校验、不回查会让 `project_id` 直接采信请求值，
拥有项目 P1 权限的调用方配上 P2 下的环境就能拼出 P1 的路径。

服务的路径不能裁剪。资源类型的父链是注册在权限中心里的（服务挂环境、环境挂项目、项目挂业务），
权限中心不接受比父链更短的路径；把服务路径缩成 `/biz,{bizID}/` 会让所有服务级授权都匹配不到，
表现为服务列表权限注解全部为 `false`。

## 四、开关行为

| 资源/操作 | `enabled: false` | `enabled: true` |
| --- | --- | --- |
| 项目、环境相关操作 | 跳过，不请求权限中心 | 按项目/环境实例校验（要求 `hierarchical`） |
| 服务创建 | `hierarchical`：按目标环境；`legacy`：按业务 | 同左 |
| 服务查看/编辑/删除/发布 | 按服务实例校验，路径由拓扑决定 | 同左 |
| 服务列表 / 业务访问 | 业务维度校验，不变 | 业务维度校验，不变 |
| 项目/环境列表权限注解 | 全部返回有权限 | 按实例返回 |
| 无权限申请链接 | 与拓扑一致的祖先链 | 与拓扑一致的祖先链 |

`enabled: true` 且探测/配置为 `legacy` 时 auth-server 启动失败，提示先执行 `migrate init-iam` / `init-iam-v4`：
此时项目、环境资源类型并不存在，继续启动只会让相关操作全部被拒绝。

### 关闭态下服务创建为什么仍用环境实例

`app_create` 在 IAM 中只关联环境实例（没有业务实例定义），用业务实例发起鉴权或申请会被拒绝：

```
1902412  action(app_create) not support to grant instance
```

因此服务创建始终用目标环境作为资源实例（`ResourceID` 即环境 ID），路径声明「业务 → 项目」两级。
新拓扑下这两级祖先由鉴权侧按目标环境回查，与开关无关：
请求里的 `project_id` 只当线索，回查不到时清空归属（鉴权退化为无权限），不会拿请求值冒充祖先。
`project_id` 因此也不会因为开关关闭而缺失，申请链接能正常生成 `app_create` 项。
调用方未指定目标环境（`ResourceID` 为 0）时，鉴权仍会发起，但申请链接不会生成 `app_create` 项，
避免向权限中心提交不存在的实例。

### 关闭态下为什么不裁剪服务路径

权限中心会按动作注册的实例视图校验节点链：

- 服务：`业务 → 项目 → 环境 → 服务`
- 服务创建（资源类型是环境）：`业务 → 项目 → 环境`

缺任何中间层级都会被拒绝，报错形如：

```
1902412 Parameter verification failed: resource([biz,2, app,1]) not satisfy instance selection
```

鉴权侧同理：路径与父链不一致时，权限中心无法用该路径匹配到服务级授权，结果就是"用户已被授予服务查看，
注解仍全是 false"。所以关闭态只跳过项目/环境这两类资源，不缩短服务与环境资源的路径；
归属（业务/项目/环境）缺失、拼不出完整链时，申请链接放弃生成该项，避免提交 ID 为 0 的非法节点。

## 五、IAM 模型要求

| 场景 | 是否需要初始化 |
| --- | --- |
| `topology: legacy` | 不需要。权限中心保持「业务 → 服务」即可，服务路径只用业务一层 |
| `topology: hierarchical` | 需要 IAM 模型已是新拓扑：项目/环境资源类型与动作、服务三级祖先链、`app_create` 关联环境 |
| `enabled: true` | 同时要求 `hierarchical`，否则启动失败 |

旧拓扑下关闭开关与项目/环境维度引入前的版本行为完全一致：服务路径 `/biz,{bizID}/`、
服务创建按业务维度、申请链接只到业务层。

初始化命令：

```bash
# V3
bk-bscp-authservice migrate init-iam

# V4：先看差异，再原地更新
bk-bscp-authservice migrate diff-iam-v4
bk-bscp-authservice migrate init-iam-v4
```

V4 的 `update_action` 不支持修改 `resource_type_id`，`app_create` 改为关联环境时由 `init-iam-v4` 自动重建该操作（先从引用它的角色中移除，重建后按本地定义加回），不影响既有授权。若 `init-iam-v4` 报"权限中心未接受原地更新"，说明资源类型的祖先链未能更新，需按提示 `delete-iam-v4` 后重建（会清空 BSCP 在 IAM 的授权，需提前导出重放）。

## 六、实现位置

| 文件 | 职责 |
| --- | --- |
| `pkg/cc/types.go` | 配置结构、默认值、环境变量回退与校验（`enabled`、`topology`） |
| `pkg/config/frontend.go` | 环境变量名常量 |
| `pkg/iam/meta/meta.go` | `ResourceTopology` 枚举（`legacy` / `hierarchical`） |
| `cmd/auth-server/service/auth/topology.go` | V3/V4 拓扑探测与最终取值解析（探测失败即启动失败、开启态拒绝 legacy） |
| `pkg/iam/sys/sys.go` | V3 只读查询已注册资源类型 |
| `cmd/auth-server/service/service.go` | 读取开关与拓扑、启动探测、传递配置、启动日志 |
| `cmd/auth-server/service/auth/auth.go` | `AuthorizeBatch` / `GetPermissionToApply` 统一补齐归属，V4 鉴权器注入开关与拓扑 |
| `cmd/auth-server/service/auth/adaptor.go`、`gen_id.go` | V3 资源映射：关闭态只跳过项目/环境资源，路径层级由拓扑决定 |
| `cmd/auth-server/service/auth/app_scope.go` | 归属回查：服务按实例回查，服务创建按目标环境回查（新拓扑下与开关无关），关闭态跳过项目/环境资源本身 |
| `pkg/iam/v4/adaptor/adaptor.go`、`pkg/iam/v4/auth/auth.go` | V4 映射与鉴权器的开关、拓扑 |

## 七、验证

1. 启动日志确认开关与拓扑取值，例如
   `iam project/environment auth: disabled, project/environment resources are not authorized, resource topology: hierarchical`；
2. 关闭态调用 `/api/v1/auth/iam/permission/check`，请求体是 `resources` 数组，业务 ID 与归属都放在每一项内：

   ```json
   {
     "resources": [
       {
         "basic": {"type": "app", "action": "view", "resource_id": 3001},
         "biz_id": 2,
         "project_id": 3,
         "environment_id": 4
       }
     ]
   }
   ```

   鉴权侧会用服务自身归属覆盖项目/环境，权限中心收到的路径为 `/biz,2/project,3/environment,4/`；
   服务被授予了 `app_view` 时返回 `is_allowed=true`，只授业务访问时返回 `is_allowed=false`。
   缺少 `basic` 会直接返回参数错误，不会落到空指针；
3. 关闭态调用项目/环境接口，权限中心无记录；
4. 关闭态创建服务时故意把 `project_id` 填成自己有权限的项目、`resource_id`（环境 ID）填到别人项目下的环境：
   鉴权路径里的项目应被回查成该环境的真实项目，拼不出匹配授权时返回无权限；`project_id` 缺省时申请链接仍能生成 `app_create` 项；
5. 开启态用带 `project_id` 与 `basic.resource_id`（环境 ID）的请求验证环境维度判定；
6. 开关切换后重启 auth-server，行为立即变化，无缓存残留。

## 八、已知取舍

- 关闭开关不会放大服务权限：服务仍要求 `app_view` 等服务级授权，只有业务访问权限的用户看不到服务；
- `hierarchical` 拓扑下服务未绑定项目/环境时归属为 0，路径退化为 `/biz,{bizID}/project,0/environment,0/`，
  这类历史服务的鉴权会被判为无权限，需先由数据迁移
  `20260817161906_backfill_core_tables` 回填 `applications` 的 `project_id` / `environment_id`
  （不做业务路径兜底，避免绕过服务级隔离）；
- `legacy` 拓扑下不校验项目/环境资源，服务路径只看业务一层；
- 调用方未指定目标环境时（`hierarchical`），服务创建仍会发起鉴权，但不会生成 `app_create` 的申请链接；
- 关闭开关时项目/环境的写操作只受业务访问权限约束（保持改造前取舍），生产环境建议开启；
- 开关与拓扑都是全局的，暂不支持按业务/租户灰度。
