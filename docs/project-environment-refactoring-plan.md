# 项目与环境维度改造方案

## 目标与边界

在不破坏现有 `biz_id` 一级业务维度的前提下，引入项目和环境维度：

```text
tenant_id -> biz_id -> project_id -> environment_id
```

`biz_id` 继续作为业务、CMDB、租户反查、权限兼容的主边界。项目用于应用配置工作区隔离，环境用于服务、配置数据、版本和发布生命周期隔离。

进程、进程实例和进程配置管理仍然只有业务维度，不引入项目和环境。`ProcessSpec.Environment` 是 CMDB 环境类型，不是本方案新增的 `environment_id`。

环境分两层理解：

```text
环境类型：prod / staging / test / dev，用于分组展示、颜色、权限策略和风险提示
环境实例：Default / prod1 / test1 / dev1，真实参与数据隔离的是 environment_id
```

默认兼容环境固定为：

```text
key = default
type = prod
name = Default 或 默认环境
protected = true
```

不要再创建 `prod` 作为第二套默认环境，避免旧 SDK、feed-server 和迁移逻辑出现两套默认语义。

## 核心原则

这些原则优先级高于具体表结构、接口和产品入口设计，后续实现和评审都需要按这些原则校验。

1. **兼容优先**：旧 UI、旧 API、旧 SDK、旧 sidecar、旧 feed 协议的行为不改变。旧请求缺少 `project_id/environment_id` 时，统一解析到 default project/default environment。
2. **原客户端无影响**：已经部署的客户端不需要升级、不需要修改配置、不需要重新注册，仍然可以继续拉取原有配置；新项目和非默认环境能力只对显式使用新协议或新配置的客户端生效。
3. **用户无感发布**：改造上线过程不阻塞用户创建、编辑、发布和客户端拉取配置。表结构迁移、默认 scope 初始化、历史数据补齐和新站点/新入口开启都分阶段灰度完成，默认路径继续可用。
4. **业务边界不变**：`biz_id` 仍然是业务、CMDB、权限兼容和旧接口兼容的主边界，不能把所有能力强行迁移到项目或环境下。
5. **作用域清晰**：`applications`、配置项、版本、发布策略、客户端状态等服务运行态资源走 EnvScope；分组定义、模板、密钥及授权规则等可被多个环境复用的资源走 ProjectScope；进程与配置管理继续走 BizScope。
6. **默认值唯一**：默认项目和默认环境的 `key` 都固定为 `default`，不引入 `default/prod` 两套默认值。
7. **渐进迁移**：先新增可空字段和兼容读写，再通过 `migrate up` 做默认 project/env 初始化和存量数据补齐，补索引、替换唯一约束，最后收紧为 `NOT NULL`。服务启动时不做大表 DDL、全量 UPDATE 或长事务。
8. **隔离安全**：旧接口只能访问 default project/default environment；新接口必须显式校验 app、project、environment 的归属，避免跨项目或跨环境串读串写。
9. **可观测可回滚**：新能力通过灰度开关控制，迁移补齐任务可暂停、恢复、重跑；失败时关闭新入口，旧路径继续工作，DDL 不作为回滚前提。

## 产品设计

产品交互采用小改造，不重做整体布局。

顶部导航保持当前结构：左侧产品名和一级菜单，右侧继续保留业务选择器，并在业务选择器旁增加项目选择器。

```text
业务：蓝鲸电商(2)    项目：订单中心
```

项目选择器只在 ProjectScope 和 EnvScope 页面启用，包括服务管理、分组管理、模板与变量、客户端管理。进入进程与配置管理这类 BizScope 页面时，项目选择器不参与查询，建议隐藏或置灰并标识为业务级页面。

环境选择器不放在全局 Header，避免用户误以为所有功能都有环境维度。环境选择器只出现在 EnvScope 页面，例如服务列表、服务配置、版本发布、客户端查询。

服务列表参考如下布局：

```text
[新建服务] [环境：生产 / Default v]                         [只显示我创建的服务] [全部服务] [文件型] [键值型] [搜索] [视图切换]
```

环境下拉面板：

```text
搜索：按环境名、Key、描述搜索                 [创建环境]

全部环境
  查看各环境状态概览

生产环境
  Default  key: default  兼容默认环境
  prod1    key: prod1

预发布环境
  staging  key: staging

测试环境
  test1    key: test1
  test2    key: test2

开发环境
  dev1     key: dev1
  dev2     key: dev2
```

服务列表是“当前项目 + 当前环境”下的服务视图。`applications` 本身属于环境维度，选择某个环境后，表格只展示该环境下的服务：

```text
服务名称 | 类型 | 当前环境配置数 | 当前环境版本 | 发布状态 | 客户端数 | 最近更新 | 更新人 | 操作
```

选择“全部环境”时进入环境概览模式，不和单环境列表混用：

```text
服务名称 | 类型 | 环境状态 | 最近更新 | 更新人 | 操作
```

环境状态用标签展示，例如：

```text
开发 编辑中    测试 已上线    预发 未初始化    生产 已上线
```

新建服务时直接在当前项目和当前环境创建 `applications` 记录。需要在其他环境使用同名服务时，在目标环境新建对应 `applications` 记录；各环境服务使用独立 `app_id`，但可通过相同服务名和项目归属在产品上做跨环境对照。

## 资源作用域

| 作用域 | 维度 | 资源 |
| --- | --- | --- |
| BizScope | `tenant_id + biz_id` | 进程、进程实例、CMDB 同步、业务主机、进程配置管理、`config_templates`、`config_instances`、进程任务执行类资源 |
| ProjectScope | `tenant_id + biz_id + project_id` | group 定义、模板空间、模板、模板套餐、模板版本、模板变量、credential、credential_scope、hook |
| EnvScope | `tenant_id + biz_id + project_id + environment_id` | applications、group 与 app 绑定、config item、kv、commit、release、strategy、released artifact、app template binding、app template variable、feed 拉取、环境级事件订阅、客户端状态、客户端下载任务 |
| MixedScope | `tenant_id + biz_id + nullable project_id/environment_id` | events、audit、operation record 等混合作用域资源，业务级操作为空，项目/环境级操作记录对应上下文 |

“服务放在环境下”直接体现在 `applications` 表：

```text
applications: 环境级服务，新增 project_id + environment_id
app_id: 环境内服务实例 ID，不再作为跨环境稳定身份
```

服务列表、服务配置、版本发布、客户端查询等 EnvScope 页面直接以环境下的 `applications` 为入口。分组定义、密钥授权、模板资产仍是项目级复用资源；涉及 `app_id` 的绑定关系和发布快照进入环境维度。

分组定义和模板资产放在项目维度。模板资产可以被同一项目下的不同环境复用；环境只保存某个 app 在该环境中的模板绑定、变量覆盖和发布结果。

`contents` 按 owner 继承作用域：当前配置项内容属于 EnvScope；模板版本内容仍按模板资产的项目级语义处理，不能把模板资产误加 `environment_id`。

进程配置管理如果底层复用 `template_*` 表，第一阶段固定落到 default project 做兼容存储，但产品语义和接口仍然是 BizScope，不向用户暴露项目或环境选择。

## 数据模型

新增表：

```text
projects(
  id,
  tenant_id,
  biz_id,
  key,
  name,
  memo,
  protected,
  creator,
  reviser,
  created_at,
  updated_at
)

environments(
  id,
  tenant_id,
  biz_id,
  project_id,
  key,
  name,
  type,
  memo,
  display_order,
  protected,
  creator,
  reviser,
  created_at,
  updated_at
)
```

约束：

```text
projects: tenant_id + biz_id + key 唯一
environments: tenant_id + biz_id + project_id + key 唯一
默认 project/env protected = true，不允许删除，不允许修改 key
environment.type 首期固定为 prod/staging/test/dev
```

字段新增规则：

```text
ProjectScope 表新增 project_id
EnvScope 表新增 project_id + environment_id
BizScope 表不新增 project_id/environment_id
MixedScope 表按审计查询需要新增可空 project_id/environment_id
```

第一阶段字段允许为空：

```sql
project_id bigint unsigned null
environment_id bigint unsigned null
```

等历史数据回填完成、兼容读写稳定后，再切换为非空约束。

## 现有表改造清单

第一阶段按下表确认改造范围，避免只改核心表后遗漏 feed、审计、客户端状态。

| 表 | 目标作用域 | 改造方式 |
| --- | --- | --- |
| `applications` | EnvScope | 新增 `project_id`、`environment_id` |
| `archived_apps` | EnvScope | 新增 `project_id`、`environment_id`，归档的是环境下的 app |
| `groups` | ProjectScope | 新增 `project_id` |
| `group_app_binds` | EnvScope | 新增 `project_id`、`environment_id`，校验 group 属于同一项目、app 属于同一环境 |
| `template_spaces` | ProjectScope | 新增 `project_id` |
| `templates` | ProjectScope | 新增 `project_id` |
| `template_sets` | ProjectScope | 新增 `project_id` |
| `template_revisions` | ProjectScope | 新增 `project_id` |
| `template_variables` | ProjectScope | 新增 `project_id` |
| `credentials` / `credential_scopes` | ProjectScope | 新增 `project_id`，密钥和授权规则范围跟随项目 |
| `hooks` / `hook_revisions` | ProjectScope | 新增 `project_id`，发布快照仍进入环境 |
| `config_items` | EnvScope | 新增 `project_id`、`environment_id` |
| `contents` | EnvScope | 当前配置项内容新增 `project_id`、`environment_id` |
| `commits` | EnvScope | 新增 `project_id`、`environment_id` |
| `kvs` | EnvScope | 新增 `project_id`、`environment_id` |
| `releases` | EnvScope | 新增 `project_id`、`environment_id` |
| `strategy_sets` / `strategies` | EnvScope | 新增 `project_id`、`environment_id` |
| `current_published_strategies` | EnvScope | 新增 `project_id`、`environment_id` |
| `published_strategy_histories` | EnvScope | 新增 `project_id`、`environment_id` |
| `released_config_items` / `released_kvs` | EnvScope | 新增 `project_id`、`environment_id` |
| `released_groups` | EnvScope | 新增 `project_id`、`environment_id`，group 定义仍是 ProjectScope |
| `released_hooks` | EnvScope | 新增 `project_id`、`environment_id` |
| `app_template_bindings` | EnvScope | 新增 `project_id`、`environment_id` |
| `app_template_variables` | EnvScope | 新增 `project_id`、`environment_id` |
| `released_app_templates` | EnvScope | 新增 `project_id`、`environment_id` |
| `released_app_template_variables` | EnvScope | 新增 `project_id`、`environment_id` |
| `current_released_instances` | EnvScope | 新增 `project_id`、`environment_id` |
| `clients` / `client_events` | EnvScope | 新增 `project_id`、`environment_id` |
| `client_querys` | EnvScope | 新增 `project_id`、`environment_id` |
| `events` | MixedScope | 新增可空 `project_id`、`environment_id`，按 `resource` 决定具体作用域 |
| `audits` | MixedScope | 新增可空 `project_id`、`environment_id`，用于筛选和审计回溯 |
| `processes` / `process_instances` | BizScope | 不新增项目和环境 |
| `biz_hosts` | BizScope | 不新增项目和环境 |
| `config_templates` / `config_instances` | BizScope | 不新增项目和环境 |
| `configs` | SystemScope | 不新增项目和环境 |

`credential_scopes` 必须与 `credentials` 一起进入 ProjectScope。授权规则当前按 app name 匹配，允许不同项目或同一项目不同环境存在同名 app 后，匹配、删除 app 后清理规则、缓存刷新和事件消费都必须带 `project_id` 过滤，不能继续只按 `biz_id + app_name` 判断。首期语义是同一项目下同名服务跨环境共享授权；如果未来需要环境级密钥授权，再给 `credential_scopes` 增加可选 `environment_id`。

`events` 不能整体收紧为 EnvScope，需要按 `resource` 分流：

| resource | 作用域 | 规则 |
| --- | --- | --- |
| `Application` | EnvScope | 必须带 `project_id + environment_id` |
| `CredentialEvent` | ProjectScope | 必须带 `project_id`，`environment_id` 为空 |
| `Publish` | EnvScope | 必须带 `project_id + environment_id` |
| `RetryApp` / `RetryInstance` | EnvScope | 必须带 `project_id + environment_id` |
| `CursorReminder` | System/Internal | 不绑定项目和环境 |

## 迁移策略

迁移分为结构迁移、默认 scope 初始化和存量数据补齐三类，统一走现有 data-service `migrate up` 机制执行。不要在 data-service、feed-server 正常启动路径里做 DDL 或全量回填。

默认兼容数据：

```text
每个 biz_id 创建一个默认项目：key = default
每个默认项目创建一个默认环境：key = default, type = prod
所有存量 ProjectScope 数据回填到默认项目
所有存量 EnvScope 数据回填到默认项目和默认环境
MixedScope 数据按资源类型回填，项目级事件只回填默认项目，环境级事件回填默认项目和默认环境
旧站点和旧接口后续新增的数据也默认写入 default project/default environment
```

如果 `migrate up` 执行后、兼容后台服务切换前仍有旧服务写入数据，新增行可能暂时为空 scope。字段第一阶段必须允许为空，新后台读取时保留 `IS NULL` 兼容；兼容后台稳定写入 default scope 后，再执行一次增量补齐，最后才收紧 `NOT NULL`。

`migrate up` 可以直接完成默认数据补齐，但需要满足以下约束：

```text
1. 先创建 projects/environments 表和默认 project/env
2. 再给各业务数据表新增 project_id/environment_id 可空字段
3. 按表、按主键范围小批量回填，不使用单个长事务覆盖全表
4. 回填 SQL 幂等，可重复执行；失败后重跑不产生重复默认项目或默认环境
5. 数据量较大时记录迁移进度，允许暂停、恢复、重跑
```

建议新增迁移状态表：

```text
scope_migration_tasks(
  table_name,
  scope_type,
  last_id,
  status,
  updated_at
)
```

存量数据补齐按主键小批量执行：

```sql
UPDATE config_items
SET project_id = ?,
    environment_id = ?
WHERE id > ?
  AND project_id IS NULL
ORDER BY id
LIMIT 1000;
```

每批提交，记录 `last_id`，失败后可重试。迁移任务必须幂等，多实例部署时使用迁移锁或唯一索引避免重复执行。

## 启动时处理

服务启动时只做轻量检查：

```text
1. 确认表结构版本满足当前服务要求
2. 确认 default project/default environment 存在
3. 旧接口缺少 project/env 时解析到默认 scope
4. 不阻塞 data-service、feed-server 正常启动
```

启动时不做：

```text
1. 大表 ALTER
2. 大表 CREATE INDEX
3. 全量 UPDATE
4. 全量扫描所有 biz 创建默认数据
5. 长事务回填
```

如果检查发现默认 project/env 缺失，说明 `migrate up` 未完整执行，应暴露启动检查告警；兼容路径可临时按需创建默认 scope，但不能依赖启动逻辑完成正式迁移。

## 服务层改造

`Kit` 增加：

```text
ProjectID
EnvironmentID
```

新增作用域解析：

```text
BizScopeResolver
ProjectScopeResolver
EnvScopeResolver
```

旧请求不传 `project_id/environment_id` 时，应用配置类请求解析到默认项目和默认环境。BizScope 请求不解析项目和环境。

中间件按需挂载：

```text
BizVerified
ProjectVerified
EnvironmentVerified
AppVerified
```

业务级能力只挂 `BizVerified`，不能强制要求项目或环境。

旧接口中带 `app_id` 的请求必须额外校验 app 属于 default project/default environment，不能只用 `biz_id + app_id` 查询，避免旧接口访问到非默认项目或非默认环境的数据。

新接口中带 `app_id` 的请求必须校验：

```text
app.project_id == request.project_id
app.environment_id == request.environment_id
environment.project_id == request.project_id
```

## 接口兼容

旧接口保留：

```text
/api/v1/config/biz/{biz_id}/apps/{app_id}/...
```

旧接口内部自动补齐：

```text
project_id = default_project_id
environment_id = default_environment_id
```

新增接口：

```text
/api/v1/config/biz/{biz_id}/projects
/api/v1/config/biz/{biz_id}/projects/{project_id}/envs
/api/v1/config/biz/{biz_id}/projects/{project_id}/envs/{env_id}/apps
/api/v1/config/biz/{biz_id}/projects/{project_id}/envs/{env_id}/apps/{app_id}/config_items
/api/v1/config/biz/{biz_id}/projects/{project_id}/envs/{env_id}/apps/{app_id}/releases
```

进程、进程实例、进程配置管理接口继续保持业务维度，不新增项目或环境路由。

## 读写兼容

写入规则：

```text
旧应用配置接口写入 -> default project/env
新接口写入 -> 指定 project/env
BizScope 接口写入 -> 只使用 biz_id
```

读取规则：

```text
旧应用配置接口读取 -> default project/env
新接口读取 -> 指定 project/env
BizScope 接口读取 -> 只使用 biz_id
```

迁移过渡期可以保留空值兼容：

```sql
AND (project_id = ? OR project_id IS NULL)
AND (environment_id = ? OR environment_id IS NULL)
```

回填完成并完成灰度验证后，移除 `IS NULL` 兼容条件。

## feed-server 与客户端兼容

老客户端协议不能破坏。feed proto 只能新增可选字段，不能改变现有字段含义。已经部署的客户端不需要升级或调整配置，继续按 default project/default environment 拉取原有配置。

老 sidecar 当前以 `SideWatchPayload.BizID + SideAppMeta` 识别应用，`SideAppMeta` 中包含 `AppID`、`App`、`Namespace`、`Uid` 等字段。兼容规则为：

```text
老客户端不传 project/env -> feed-server 解析到 default project/default environment
老客户端传 app_id -> 校验 app_id 属于 default project/default environment
老客户端只传 app name -> 只在 default project/default environment 内按 app name 查找
非默认项目或非默认环境 -> 必须使用新客户端或新配置显式传 project/env
```

新客户端可以显式传：

```text
project_id/project_key
environment_id/environment_key
```

这些字段可以追加到 `SideAppMeta` / `AppMeta`，也可以在没有 `AppMeta` 的下载类请求中追加为可选字段；不能复用或改变已有字段含义。

缓存 key、事件 key、下载任务 key、客户端状态 key 需要扩展：

```text
旧逻辑: biz_id + app_id
新逻辑: biz_id + project_id + environment_id + app_id
```

老客户端统一落到默认项目和默认环境，所以原有拉取结果保持不变。

如果未来允许不同项目或不同环境下存在同名 app，老客户端无法区分项目和环境。兼容边界应明确为：

```text
老客户端只访问 default project/default environment
非默认项目和环境必须使用新客户端或新配置
```

如必须让老客户端访问非默认项目，需要额外维护显式映射表：

```text
legacy_app_routes(
  tenant_id,
  biz_id,
  app_name,
  project_id,
  environment_id
)
```

默认不建议引入该映射，避免同名服务歧义。

## 发布链路

发布链路必须进入环境维度：

```text
同一项目下同名服务:
  dev  app_id=101 -> release A
  test app_id=201 -> release B
  prod app_id=301 -> release C
```

以下资源需要带 `environment_id`：

```text
releases
strategy_sets
strategies
released_groups
released_hooks
released_config_items
released_kvs
released_app_templates
released_app_template_variables
current_published_strategies
published_strategy_histories
current_released_instances
```

旧发布接口默认发布到默认环境。

生产环境发布权限可以单独授权；`environment.type = prod` 的环境在 UI 上展示风险提示，但发布隔离仍以 `environment_id` 为准。

## 模板与变量

模板资产是项目级：

```text
template_space
template
template_set
template_revision
template_variables
```

模板资产不加 `environment_id`，同一项目下的 dev/test/prod 可以复用同一套模板。

模板使用关系和变量覆盖是环境级：

```text
app_template_binding
app_template_variables
template variable override
released_app_template*
```

推荐变量模型：

```text
项目级默认变量值
环境级覆盖变量值
```

渲染时优先使用环境级覆盖值，缺失时回退项目级默认值。

## 索引与约束

索引分阶段处理：

```text
阶段 1：新增字段，不改唯一索引
阶段 2：migrate up 补齐默认 scope 数据
阶段 3：新增 project/env 复合普通索引
阶段 4：灰度验证查询路径
阶段 5：替换唯一索引
阶段 6：字段改为 NOT NULL
```

唯一索引示例：

```text
projects:
tenant_id + biz_id + key

environments:
tenant_id + biz_id + project_id + key

applications:
tenant_id + biz_id + project_id + environment_id + name

config_items:
tenant_id + biz_id + project_id + environment_id + app_id + path + name

kvs:
tenant_id + biz_id + project_id + environment_id + app_id + key + kv_state

releases:
tenant_id + biz_id + project_id + environment_id + app_id + name

clients:
tenant_id + biz_id + project_id + environment_id + app_id + uid
```

替换唯一索引前必须完成重复数据检查。发现同名冲突时，不自动改名，生成冲突报告交由管理员处理。

## 权限

IAM 资源树建议扩展为：

```text
biz
  -> project
      -> environment
          -> app
```

项目管理、模板维护、分组维护走项目级权限。

服务管理、配置编辑、生成版本、发布版本走环境级权限。

生产环境可以单独授权发布权限。

权限兼容规则：

```text
旧 app 权限 -> 映射到 default project/default environment 下的 app 权限
旧发布权限 -> 映射到 default environment 的发布权限
biz 管理员 -> 默认拥有该业务下 default project/default environment 管理权限
新建非默认项目或环境 -> 需要显式授权或继承项目管理员权限
```

## 上线步骤

上线目标是用户基本无感：旧站点、旧接口、旧客户端持续可用，新项目和新环境能力优先通过新站点或新入口灰度开放。后台服务需要同时支持旧协议默认 scope 和新协议显式 scope。

```text
1. 发布包含表结构扩展、默认 project/env 初始化、存量数据补齐的 migrate up
2. 部署兼容后的后台服务和 feed-server
3. 开启旧接口默认 scope 解析
4. 旧站点继续访问旧接口，新增和写入默认落到 default project/default environment
5. feed-server 支持默认 project/env 解析
6. 完成历史数据校验
7. 新增 project/env 复合索引并验证查询路径
8. 替换唯一索引
9. 新站点或新入口支持显式 project/env
10. 灰度开启新项目和新环境能力
11. 字段收紧为 NOT NULL
12. 保留旧接口兼容，后续再评估废弃计划
```

灰度开关建议拆分：

```text
enable_project_scope
enable_environment_scope
enable_new_feed_scope
enable_project_env_ui
```

回滚策略：

```text
DDL 不回滚
关闭新站点或新项目/环境入口
旧接口继续走 default project/default environment
`migrate up` 回填任务可暂停、恢复、重跑；已写入的默认 scope 数据保留
```

## 观测与验证

需要增加以下观测：

```text
默认 project/env 创建成功率
`migrate up` 默认 scope 补齐进度和失败表
旧接口默认 scope 命中次数
feed-server 默认 scope 解析次数
feed-server 按 project/env cache miss/hit
跨环境缓存串读检测
客户端状态按环境分布
发布链路按环境的成功率和耗时
```

验证重点：

```text
旧 UI 创建、编辑、发布配置不变
发布过程用户基本无感，不要求停机或客户端升级
旧 sidecar 拉取结果不变
旧 SDK 拉取结果不变
默认项目/默认环境数据完整
旧接口不能访问非 default project/default environment 的 app 数据
进程配置管理接口不要求 project/env
同一项目下多个环境复用同一套模板资产
新项目之间 app/group/template 名称互不冲突
同一项目下同名服务在不同环境可发布不同 release
feed-server 缓存不会跨环境串数据
客户端状态不会跨环境串数据
`migrate up` 补齐任务可中断、可恢复、可重复执行
```

## 待确认选项

以下点建议产品或架构评审时确认：

1. 环境类型首期是否只允许 `prod/staging/test/dev` 四类。推荐首期固定四类，后续再增加 `custom`。
2. BizScope 页面是否隐藏项目选择器，还是置灰展示当前项目。推荐隐藏或置灰并标识“业务级能力”，避免用户误解进程数据被项目隔离。
3. 是否允许老客户端通过映射访问非默认项目。推荐不支持，老客户端固定访问 default project/default environment。

## 结论

合理方案不是把所有资源强行挂到环境下，而是保留业务级能力，新增项目级应用配置工作区，并让 `applications`、应用配置数据、版本、发布和客户端状态进入环境维度。

结构迁移、默认项目/默认环境创建和存量数据补齐走现有 `migrate up`，服务启动时只做轻量检查，不做大表 DDL、不做全量回填。旧站点、旧接口、旧客户端、旧 feed 协议全部默认落到默认项目和默认环境，从而保证现有业务连续可用。
