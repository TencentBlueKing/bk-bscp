# 项目/环境维度权限校验开关

> 对应配置项 `iam.project_environment_auth.enabled`，用于在项目/环境资源实例同步存在延迟时，跳过项目与环境资源本身的校验。
> 该开关只作用于 IAM V3；IAM V4 固定按项目/环境模型鉴权（服务创建只有环境级），忽略该开关。

## 一、背景与目标

BSCP 的 IAM 授权模型为四层拓扑（开关开启时；关闭时退化为「业务 → 服务」两层）：

```
业务（biz） → 项目（project） → 环境（environment） → 服务（app）
```

新增项目或环境后，IAM 侧的资源实例与授权存在同步延迟，按项目、环境实例校验会出现"已经授权但仍被拒绝"的情况，影响业务操作。因此在 auth-server 增加一个开关：

- 关闭：按旧模型处理——项目、环境资源本身不参与鉴权（不请求权限中心），服务创建按业务维度授权，
  服务等资源的路径只有业务一级；
- 开启：按新模型处理——项目、环境按实例校验，服务创建按目标环境授权，服务路径为三级。

开关决定三件事：是否单独校验项目/环境资源、**服务创建的授权维度**（关闭 → `app_create` 按业务，
开启 → `environment_app_create` 按环境），以及**资源鉴权路径的层级**（关闭 → 旧模型「业务 → 服务」，
开启 → 「业务 → 项目 → 环境 → 服务」）。开关同时就是"IAM 侧是否已升级到新模型"的声明，两者必须一起切换，
原因见下节。无论开关取值，服务的动作都只授权到服务实例，关闭开关不会让只有业务访问权限的用户获得服务权限。

创建者策略的写入同样只看开关：开启（新模型）时创建项目或环境即写入创建者授权；关闭（旧模型）时
权限中心没有项目/环境资源类型与对应创建者角色，写入必然失败，auth-server 直接跳过。

## 二、配置

```yaml
iam:
  # 项目/环境维度权限校验开关，只作用于 V3；同时决定 V3 资源鉴权路径的层级。V4 忽略该开关
  project_environment_auth:
    # false: 旧模型（默认）。项目/环境资源本身不鉴权，服务创建按业务授权，服务路径只有业务一级
    # true : 新模型。项目/环境按实例校验，服务创建按目标环境授权，服务路径为三级
    enabled: false
```

| 项 | 说明 |
| --- | --- |
| 默认值 | `enabled=false`（旧模型） |
| 环境变量 | `BK_BSCP_IAM_PROJECT_ENV_AUTH`，仅在配置文件未显式配置时生效 |
| 生效范围 | 仅 auth-server 读取，config-server / api-server / data-service 无需配置 |
| 生效方式 | 修改配置后重启 auth-server；上游服务无需重启 |
| 启动日志 | `iam project/environment auth: disabled, project/environment resources are not authorized` |

### 路径层级由开关推导

代码里不再有"资源拓扑"这一独立概念，也没有 `topology` 配置项：开关本身就是路径层级的唯一来源，
开启时按「业务 → 项目 → 环境」三级拼接，关闭时只声明业务一级。启动时也不探测权限中心已注册的模型。

之所以不保留独立配置，是因为开关与拓扑描述的是同一件事——权限中心里有没有项目/环境模型：

- 模型已是三级、开关却关闭：服务路径被裁成 `/biz,{bizID}/`，权限中心无法用它匹配任何服务级授权，
  表现为"用户已被授予服务查看，列表注解却全是 false"；
- 模型还停在两级、开关却开启：三级路径与父链不符，权限中心直接拒绝请求。

两种情况都比启动失败更难排查，因此把两者合并成一个取值。代价是切换开关必须与 IAM 模型迁移同时进行：

- 开启前先执行 `migrate init-iam` / `migrate init-iam-v4`，否则项目/环境相关操作会全部被拒绝；
- 关闭时线上模型必须仍是业务维度：V4 的 `app_create` 在新模型里注册在环境上，但按业务维度授予
  （`biz_operator` 等角色声明在 biz 上）同样可用，关闭开关后 V4 以 biz 实例鉴权、服务路径只到业务一级，
  要求服务祖先链仍是 `[biz]`。auth-server 不拦启动（合法的旧模型部署仍需能起来），
  但启动日志里以 **Error** 级别显著提示这条组合的风险与处置方式，可用 `migrate diff-iam-v4` 确认线上状态。

## 三、授权维度对照

| 资源/操作 | IAM 动作 | 授权维度 |
| --- | --- | --- |
| 业务访问 | `find_business_resource` | 业务 |
| 项目创建 / 项目列表 | `project_create` / `find_business_resource` | 业务 |
| 项目查看 / 编辑 / 删除 | `project_view` / `project_edit` / `project_delete` | 项目 |
| 环境创建 | `environment_create` + `project_view` | 项目 |
| 环境查看 / 编辑 / 删除 | `environment_view` / `environment_edit` / `environment_delete` | 环境 |
| 服务创建 | 开启：V3 `environment_app_create` / V4 `app_create` + `project_view` / `environment_view`；关闭：`app_create` | 开启：环境 + 项目/环境查看；关闭：业务 |
| 服务克隆 | 同服务创建 | 同服务创建（克隆即新建服务，请求不带源服务实例，不校验源服务的 `app_edit`） |
| 服务查看 / 编辑 / 删除 / 生成版本 / 上线版本 | `app_view` / `app_edit` / `app_delete` / `release_generate` / `release_publish` + `project_view` / `environment_view` | 服务 + 项目 + 环境 |
| 服务密钥查看 / 管理 | `app_credential_view` / `app_credential_manage` + `project_view` | 业务 + 项目 |
| 操作记录查看 | `audit_view` + `project_view` | 业务 + 项目 |

权限中心 V3 的 `app_create` 带着按业务授予的存量策略，而 V3 不允许修改带策略操作的关联资源（1901409），
因此它保持关联业务，是关闭开关时的服务创建动作（「业务下任意环境都可创建服务」的粗粒度权限）；
按环境的细粒度权限是新操作 `environment_app_create`（服务创建（环境级）），开启开关时使用。

服务创建的授权维度只由开关决定，也不会在两种维度之间回退：
开启时只按目标环境校验 `environment_app_create`，业务维度的 `app_create` 不参与判定——
回退放行会让一条业务级授权直接覆盖按项目/环境授予的细粒度创建权限，环境隔离随之失效
（只授了某业务「服务创建」的用户，在没被授权创建的环境里也会被判为有权限）；
关闭时只按业务校验 `app_create`，没有项目/环境模型，目标环境完全不参与鉴权。
代价是存量业务级 `app_create` 授权在开启开关后不再放行服务创建，需要按环境重新授权；
开启态下无权限申请链接给出的就是 `environment_app_create`，目标环境回查不到归属时判为无权限。

创建与克隆还同时要求目标项目、环境的查看权限（`project_view` / `environment_view`），
与读取已有服务内资源同一条规则：只能在自己看得见的项目/环境里建服务，不会只凭一条环境创建授权
就建到看不见的环境里。配套影响：
V4 的 `app_creator` 角色已包含这两项，已上线的环境升级后需重跑 `migrate init-iam-v4`，
把角色的操作同步到权限中心；V3 的用户组/角色由运维在权限中心配置，若其中只配了
`environment_app_create`，需要一并补上目标项目与环境的查看权限。

密钥与操作记录的动作本身仍按业务授权（保持存量策略可用），项目维度以并列的 `project_view`
校验补齐：两者都依赖具体项目才能访问，因此服务密钥与操作记录的每个入口都同时要求
「业务访问 + 项目查看 + 模块动作」，只具备业务权限或模块动作权限不能读取无权查看项目下的数据。
项目查看权限经拓扑路径继承，授予业务的 `project_view` 覆盖该业务下全部项目。

开启态下服务自身的接口（读取、编辑、删除、服务列表）同为四项：
「业务访问 + 项目查看 + 环境查看 + 服务动作」。服务挂在项目与环境之下，只授服务动作不足以读取
项目/环境范围外的数据，项目/环境两项的实例由调用方给出线索、auth-server 按服务真实归属改写，
服务列表没有服务实例，按请求声明的项目与环境校验。克隆例外：它不操作已有服务实例，
与创建同为「业务访问 + 目标环境上的服务创建」。
关闭态下这两项资源被跳过，退回「业务访问 + 服务动作」。服务创建按开关选择维度——
开启时按目标环境授予 `environment_app_create`（V4 为 `app_create`），关闭时按业务授予 `app_create`，
两种都不额外要求项目查看。

鉴权路径（`_bk_iam_path_`）表示资源的祖先链，IAM 不反查拓扑，路径写到哪一级就只认哪一级的授权。
路径层级由开关推导：

| 对象 | `enabled: true`（新模型） | `enabled: false`（旧模型） |
| --- | --- | --- |
| 项目 | `/biz,{bizID}/` | 不校验（旧模型无项目资源） |
| 环境 | `/biz,{bizID}/project,{projectID}/` | 不校验（旧模型无环境资源） |
| 服务 | `/biz,{bizID}/project,{projectID}/environment,{envID}/` | `/biz,{bizID}/` |
| 服务创建 | 实例是目标环境，路径 `/biz,{bizID}/project,{projectID}/` | 实例是业务，无路径 |

`enabled: false` 时项目/环境这两类资源本身不进入权限中心，不做实例校验；服务创建同时回到业务维度。

开启态下路径里的祖先一律以回查到的真实归属为准：服务按服务实例回查业务/项目/环境，
服务创建按目标环境回查业务与项目，项目按项目实例回查业务。只跳过校验、不回查会让 `project_id` 直接采信请求值，
拥有项目 P1 权限的调用方配上 P2 下的环境就能拼出 P1 的路径。

服务内资源的「项目查看 / 环境查看」还要对齐服务自身的归属：调用方传入的 `project_id` / `environment_id`
只当作线索，同批次存在服务资源时由 auth-server 改写成该服务的真实项目与环境。
否则拥有项目 P1 查看权限的调用方配上项目 P2 的服务 ID，就能用 P1 的 `project_view` 放行对 P2 服务的配置操作。

服务的路径必须与权限中心实际注册的父链一致：新模型下是三级，旧模型下只有业务一级。
把两者搞混（新模型用一级路径、旧模型用三级路径）会让服务级授权全部匹配不到，
表现为服务列表权限注解全部为 `false`，这也是开关必须与 IAM 模型同步切换的原因。

## 四、开关行为

| 资源/操作 | `enabled: false` | `enabled: true` |
| --- | --- | --- |
| 项目、环境相关操作 | 跳过，不请求权限中心 | 按项目/环境实例校验 |
| 服务创建 | 按业务授权（`app_create`），目标环境不参与 | 按目标环境授权（`environment_app_create`），同时要求目标项目/环境的查看权限，不回退业务级 `app_create` |
| 服务查看/编辑/删除/发布 | 按服务实例校验，路径只有业务一级；项目/环境资源被跳过，退回业务访问 + 服务动作 | 按服务实例校验，路径三级，同时要求项目/环境查看 |
| 服务列表 | 业务访问 + 项目查看（声明环境时再加环境查看）；项目/环境资源被跳过时退回业务访问 | 业务访问 + 项目查看（声明环境时再加环境查看） |
| 业务访问 | 业务维度校验，不变 | 业务维度校验，不变 |
| 项目/环境列表权限注解 | 全部返回有权限 | 按实例返回 |
| 项目/环境创建者授权 | 跳过：旧模型没有项目/环境资源类型 | 按实例授予创建者 |
| 服务密钥、操作记录 | 业务访问 + 模块动作，与项目/环境维度引入前一致 | 追加项目查看校验，缺项目查看权限即拒绝 |
| 无权限申请链接 | 旧模型祖先链（服务只到业务一层，服务创建按业务） | 新模型祖先链（三级，服务创建按环境） |

服务内资源的四项校验要求角色自带对应维度：`app_viewer`、`app_publisher`、`environment_viewer`、
`environment_operator` 含 `project_view` 与 `environment_view`，`credential_manager`、`audit_viewer` 含
`project_view`。新增角色若需触达这些接口必须同步补齐，`pkg/iam/v4/model` 的用例会校验这条约束。

### 创建者角色与创建后自动授权（V4）

项目、环境、服务的创建都遵循同一模式：申请只放行创建动作的「创建者」角色，创建成功后由 BSCP
自动授予对应的「运维」角色管理该实例。V4 的授权单位是角色，`add_authorization` 必须引用角色，
因此自动授权复用已有的运维角色，不再单独注册"创建后自动授予"的角色。

| 资源 | 申请的创建者角色 | 创建后自动授予 |
| --- | --- | --- |
| 项目 | `project_creator`：业务维度 `find_business_resource`、`project_create` | `project_operator` |
| 环境 | `environment_creator`：业务维度 `find_business_resource`；项目维度 `project_view`、`environment_create` | `environment_operator` |
| 服务 | `app_creator`：业务维度 `find_business_resource`；项目维度 `project_view`；环境维度 `environment_view`、`app_create` | `app_operator` |

创建者角色都不含被创建资源的查看权限（如项目创建者不含 `project_view`）：业务或项目维度的查看会放开
该范围内他人的全部资源，自己建的资源由自动授权覆盖。自动授权按角色内的每个维度各写一条：
项目运维写项目、业务两条，环境运维写环境、业务、所属项目三条，服务运维写服务、业务、项目、环境四条。
因为每次都会写一条业务维度的记录，被自动授予的角色在业务维度只能有 `find_business_resource`
（`pkg/iam/v4/model` 的用例校验），否则每个建过资源的人都会在整个业务下获得额外权限；
同理创建者角色不能被用于自动授权。

### 运维角色是各自范围的管理员（V4）

| 角色 | 授权维度 | 范围内的操作 | 不含 |
| --- | --- | --- | --- |
| `biz_operator` 业务运维 | 业务 | 全部操作 | — |
| `project_operator` 项目运维 | 项目 | 项目查看/编辑/删除；环境创建/查看/编辑/删除；服务创建/查看/编辑/删除/生成版本/上线 | 项目创建 |
| `environment_operator` 环境运维 | 环境（另带所属项目的 `project_view`） | 环境查看/编辑/删除；服务创建/查看/编辑/删除/生成版本/上线 | 环境创建 |
| `app_operator` 服务运维 | 服务（另带所属项目/环境的查看） | 服务查看/编辑/删除/生成版本/上线 | 服务创建 |

运维角色管理自己范围内的一切，包括创建下级资源（项目运维建环境、环境运维建服务）与管理范围内他人建的服务；
不含同级资源的创建，它属于上一级范围，由对应的创建者角色负责。范围内新建的服务经路径继承自动覆盖。
职责分离（如生产配置发布）依赖服务自身的上线审批，审批流程不受角色影响。

授权范围因此比 V3 的 `ResourceCreatorActions` 宽：创建项目/环境的人自动成为该范围的运维，
可管理之后他人在该范围内建的服务。按业务注册的操作（服务密钥、操作记录、进程配置）无法按项目授予，
项目运维不含这些操作。

### 项目内共享资源的写操作要求项目编辑

模板空间、模板、模板版本、模板套餐、模板变量、分组、脚本及脚本版本属于项目而非某个服务：
被多个服务引用，脚本还会在客户端执行。它们的新增、编辑、删除、发布（共 35 个接口，见 config-server 的
`template*.go`、`group.go`、`hook*.go`）校验 `meta.ProjectEditResource`（`project_edit`），
只有项目运维与业务运维持有；读取仍校验 `project_view`。服务内资源（配置项、KV、服务绑定的模板版本与变量）
的写操作不变，仍由服务维度的动作把关。无权限时前端统一走 403 无权限页与申请链接。

V3 的影响：开关关闭（V3 默认）时项目资源被跳过，这些接口与改动前一致，只校验业务访问；
开关开启时 V3 同样要求 `project_edit`，只配了 `project_view` 的用户组（如「业务只读」常用操作）将无法再写这些资源，
需在权限中心为维护模板/分组/脚本的用户组补上对应项目的「项目编辑」。同理，V3 开启开关时环境创建也会叠加 `project_view` 校验。

V4 的授权有效期上限为 365 天（网关拒绝更长的 `expired_at`），创建者授权目前固定为 365 天且没有续期或回收逻辑，
到期后创建人需在权限中心续期。

环境创建与服务创建同一条规则：接口同时要求所属项目的 `project_view`，看不见的项目里不能建环境。
项目运维、环境创建者与业务运维都已包含该项；服务创建同理，项目运维、环境运维、服务创建者与业务运维都已包含
所需的项目与环境查看。

权限查询只声明项目、未声明环境时（例如当前项目下没有可见环境时点击「新建服务」），
auth-server 按该项目的默认环境补齐服务创建与环境查看的实例，申请链接随之指向该项目的默认环境；
项目没有默认环境时判为无权限。不会退回业务默认环境：业务默认环境属于另一个项目，拼出的
「项目 A + 项目 B 的环境」会让在 B 有权限的用户在 A 上被判为有权限。

**从旧角色 ID 升级**：早期版本中 `project_creator` / `environment_creator` 是自动授予的角色。
升级后同一 ID 变为可申请的创建者，若直接执行 `init-iam-v4`，存量的项目创建人会经业务维度那条授权获得
`project_create`（在整个业务下建项目）。因此已部署过旧版本的环境需先清理这两个角色的存量授权，
或执行 `delete-iam-v4` 后重新 `init-iam-v4`（会清空 BSCP 在权限中心的全部授权，需提前导出重放）。

创建项目时会自动生成一条默认环境。V3 的项目创建者权限只落在项目实例上、不会传递到环境实例，
因此 config-server 在项目创建成功后会按环境实例再补授一次创建者权限（V3 走 `resource_creator_action`，
V4 走 `add_authorization`），创建者由此获得默认环境的查看/编辑/删除。V4 的 `project_operator`
已在项目维度覆盖其下全部环境，这次补授对 V4 冗余但无害。
`enabled: true` 时照常写入；`enabled: false`（旧模型）下权限中心没有项目/环境资源类型，
auth-server 直接跳过，避免无效授权请求刷错误日志。
授权失败只记录日志，不影响项目创建。

关闭期间创建的项目与环境不会写入创建者策略：开关打开后，这批创建者对自己的项目与默认环境没有权限，
需要运维按实例补授。这是"先关后开"的迁移代价，建议把开关与 `migrate init-iam` / `init-iam-v4` 一次开到位。

`enabled: true` 前必须先完成模型初始化（项目/环境资源类型与动作、服务三级祖先链、按环境的服务创建操作），
否则项目/环境相关操作会全部被拒绝。auth-server 已不再探测权限中心模型，无法在启动时发现这种不一致。

**V4 不读取该开关**：V4 固定按「业务 → 项目 → 环境 → 服务」模型鉴权，服务创建只有环境级（`app_create` 关联环境），
不存在业务级的服务创建；项目/环境资源始终按实例校验，服务路径始终为三级，创建者授权与申请链接同理。
配置为 `enabled: false` 时 auth-server 启动日志给出 Warn 提示该配置对 V4 无效。本文其余关于开关关闭（旧模型）的描述只适用于 V3。

### 关闭态下的服务创建

关闭开关时全部按业务维度处理，服务创建使用业务维度的 `app_create`：动作注册在 CMDB 业务实例上，
不需要环境实例，也不受"按环境的操作只能关联环境实例"的约束。

目标环境在关闭态不参与鉴权：不解析 `environment_id`、不回查环境父级、也不做默认环境补齐，
`project_id` / `environment_id` 即使传了也不会进入鉴权。业务就是请求声明的业务
（上游已有业务访问校验作为约束），语义是「允许在该业务下创建服务」，
申请链接给出业务实例的 `app_create`；业务为 0 时拼不出实例，判为无权限且不生成申请项。

### 开启态下的服务创建

开启开关时按环境的服务创建操作（V3 为 `environment_app_create`，V4 为 `app_create`）在 IAM 中只关联环境实例
（没有业务实例定义），用业务实例发起鉴权或申请会被拒绝：

```
1902412  action(environment_app_create) not support to grant instance
```

因此开启态的服务创建始终用目标环境作为资源实例（`ResourceID` 即环境 ID），路径声明「业务 → 项目」两级。
这两级祖先由鉴权侧按目标环境回查：请求里的 `project_id` 只当线索，回查不到时清空归属（鉴权退化为无权限），
不会拿请求值冒充祖先。未指定目标环境时按业务的默认项目/环境补齐，鉴权与申请链接都指向默认环境；
业务也没有可用的默认归属时归属为 0，鉴权判为无权限且不生成申请项。

### 旧模型下的实例视图

`enabled: false` 时权限中心里只有「业务 → 服务」两层，要求的节点链也随之变短：

- 服务：`业务 → 服务`
- 服务创建：业务（无路径）

此时若提交三级路径或把服务创建指向环境实例，会被权限中心拒绝，报错形如：

```
1902412 Parameter verification failed: resource([biz,2, app,1]) not satisfy instance selection
```

鉴权侧同理：路径与父链不一致时，权限中心无法用该路径匹配到服务级授权，结果就是"用户已被授予服务查看，
注解仍全是 false"。因此路径层级只能跟着开关走；归属（业务/项目/环境）缺失、拼不出完整链时，
申请链接放弃生成该项，避免提交 ID 为 0 的非法节点。

## 五、IAM 模型要求

| 场景 | 是否需要初始化 |
| --- | --- |
| `enabled: false`（旧模型，仅 V3） | 不需要。权限中心保持「业务 → 服务」即可，服务路径只用业务一层。V4 忽略开关，始终需要新模型 |
| `enabled: true`（新模型） | 需要 IAM 模型已是新拓扑：项目/环境资源类型与动作、服务三级祖先链、按环境的服务创建操作（V3 `environment_app_create`，V4 `app_create`）关联环境 |

开关关闭时与项目/环境维度引入前的版本行为完全一致：服务路径 `/biz,{bizID}/`、
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
| `pkg/cc/types.go` | 配置结构、默认值、环境变量回退与校验（`enabled`） |
| `pkg/config/frontend.go` | 环境变量名常量 |
| `pkg/iam/meta/resources.go` | 服务内资源的鉴权资源构造函数（业务访问 + 项目查看 + 环境查看 + 服务动作），config-server 与 api-server 共用 |
| `cmd/auth-server/service/service.go` | 读取开关、V4 关闭态拒绝启动、启动日志 |
| `cmd/auth-server/service/auth/auth.go` | `AuthorizeBatch` / `GetPermissionToApply` 统一补齐归属，V4 鉴权器注入开关 |
| `cmd/auth-server/service/auth/adaptor.go`、`gen_id.go` | V3 资源映射：关闭态跳过项目/环境资源，路径层级由开关推导 |
| `cmd/auth-server/service/auth/app_scope.go` | 归属回查：服务按实例回查；服务创建按目标环境回查，均在开启态生效，关闭态全部按业务维度处理 |
| `pkg/iam/v4/adaptor/adaptor.go`、`pkg/iam/v4/auth/auth.go` | V4 映射与鉴权器的开关 |
| `cmd/config-server/service/credential.go`、`credential_scope.go`、`audit.go` | 密钥与操作记录入口统一叠加 `meta.ProjectViewResource` 校验 |
| `cmd/config-server/service/app.go`、`app_template_binding.go`、`app_template_variable.go`、`client.go`、`strategy.go`、`group.go`、`template_set.go` | 服务自身与服务内资源入口统一四项校验 |
| `cmd/api-server/service/repo.go`、`config_export.go`、`config_import.go`、`released_kv.go` | 下载、导出等 api-server 服务维入口的四项校验 |
| `cmd/config-server/service/project.go` | 项目创建后为自动生成的默认环境补授创建者权限 |

## 七、验证

1. 启动日志确认开关取值，例如
   `iam project/environment auth: disabled, project/environment resources are not authorized`；
2. 调用 `/api/v1/auth/iam/permission/check`，请求体是 `resources` 数组，业务 ID 与归属都放在每一项内：

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

   鉴权侧会用服务自身归属覆盖项目/环境：开启态权限中心收到的路径为 `/biz,2/project,3/environment,4/`，
   关闭态为 `/biz,2/`；服务被授予了 `app_view` 时返回 `is_allowed=true`，只授业务访问时返回 `is_allowed=false`。
   缺少 `basic` 会直接返回参数错误，不会落到空指针；
3. 关闭态调用项目/环境接口，权限中心无记录；
4. 关闭态创建服务时目标环境完全不参与：`resource_id` / `environment_id` 指向任意环境都不影响判定，
   只校验请求声明的业务（有该业务的 `app_create` 即放行，否则返回无权限），申请链接也给出业务实例；
   开启态才按目标环境回查业务与项目；
5. 开启态用带 `project_id` 与 `basic.resource_id`（环境 ID）的请求验证环境维度判定；
6. 开关切换后先完成模型迁移再重启 auth-server，行为立即变化，无缓存残留。

## 八、已知取舍

- 关闭开关不会放大服务权限：服务仍要求 `app_view` 等服务级授权，只有业务访问权限的用户看不到服务；
- 开启态下服务未绑定项目/环境时归属为 0，路径退化为 `/biz,{bizID}/project,0/environment,0/`，
  这类历史服务的鉴权会被判为无权限，需先由数据迁移
  `20260817161906_backfill_core_tables` 回填 `applications` 的 `project_id` / `environment_id`
  （不做业务路径兜底，避免绕过服务级隔离）；
- 关闭开关（旧模型）下不校验项目/环境资源，服务路径只看业务一层；
- 开启开关后业务级 `app_create` 不再放行服务创建：存量按业务授予的「服务创建」需要按环境重新授权，
  否则只剩申请链接可用（申请链接指向 `environment_app_create`）；不再回退是为了避免业务级授权覆盖按环境授予的细粒度权限；
- 开启开关但调用方未指定目标环境时，服务创建按业务的默认项目/环境补齐，鉴权与申请链接都落在默认环境上；
  业务没有默认归属时判为无权限，不生成申请项；
- 关闭开关时项目/环境的写操作只受业务访问权限约束（保持改造前取舍），生产环境建议开启；
- 开关是全局的，暂不支持按业务/租户灰度；切换开关等价于切换 IAM 模型，必须与迁移一起进行。
