# GSEKit 迁移工具使用说明

## 概述

`bk-bscp-gsekit-migration` 是一个将 GSEKit（进程配置管理）数据迁移至 BSCP（蓝鲸服务配置平台）的命令行工具。工具支持按业务 ID 粒度进行数据迁移与清洗，具备幂等性校验，已迁移的业务需先执行清洗才能重新迁移。

## 前置准备：配置文件

所有命令都需要通过 `-c / --config` 指定一个 YAML 配置文件。配置文件示例位于 `cmd/gsekit-migration/etc/migration.yaml`，完整示例如下：

```yaml
migration:
  multi_tenant: false        # 是否多租户模式，false 时 tenant_id 强制为 "default"
  tenant_id: "xx"       # 目标租户 ID
  creator: "xxx"           # 迁移记录的创建者
  reviser: "xxx"           # 迁移记录的修改者
  batch_size: 500            # 每批处理的记录数，默认 500
  continue_on_error: false   # 遇到错误是否继续迁移
  config_template_id_reserve_base: 20000  # config_templates.id 预留基线，默认 20000
  native_biz_id: 100148                   # 自建业务，对齐时整批搬到预留区，前置校验跳过

source:
  mysql:
    endpoints: ["127.0.0.1:3306"]
    database: "xxx"
    user: "xxx"
    password: "xxx"

target:
  mysql:
    endpoints: ["127.0.0.1:3306"]
    database: "xxx"
    user: "xxx"
    password: "xxx"

repository:
  storage_type: "xx"     # 存储后端类型: "BKREPO" 或 "S3"
  bk_repo:
    endpoint: "xxx"
    project: "xxx"
    username: "xxx"
    password: "xxx"

cmdb:
  endpoint: "xxx"
  app_code: "xxx"
  app_secret: "xxx"
  username: "xxx"

gsekit:                       # 仅 compare-render 命令需要
  endpoint: "xxx"
  app_code: "xxx"
  app_secret: "xxx"
  bk_ticket: "xxx"            # 用户登录态 ticket

gse:                          # 必填，迁移进程时查询 agent 状态
  endpoint: "xxx"
  app_code: "xxx"
  app_secret: "xxx"

log:
  level: "info"              # 日志级别: debug / info / warn / error
```

### 关键配置说明

| 配置项 | 说明 |
|---|---|
| `source.mysql` | GSEKit 源数据库连接信息 |
| `target.mysql` | BSCP 目标数据库连接信息 |
| `repository` | 配置文件内容存储后端（BK-Repo 或 S3/COS），用于上传配置模板内容 |
| `cmdb` | CMDB API 配置，用于查询进程关联的主机信息 |
| `gsekit` | GSEKit API 网关配置，仅 `compare-render` 命令使用。`bk_ticket` 为用户登录态 |
| `gse` | GSE API 网关配置（必填）。迁移进程时调用 `list_agent_state` 查询 agent 状态并写入 `agent_status`，使迁移完成后进程可立即参与 GSE 进程状态同步 |
| `migration.config_template_id_reserve_base` | `config_templates.id` 的预留基线，默认 20000。`[1, base)` 归 GSEKit 对齐专用，迁移时直接复用 GSEKit 的 `config_template_id`；`[base, ∞)` 归 BSCP 自建模板，由 `id_generators` 分配。GSEKit 侧水位涨到基线时，`migrate` 与 `align-template-id` 都会硬失败，此时需调大该值并重跑 `align-template-id` |
| `migration.native_biz_id` | BSCP 自建业务。该业务的模板与 GSEKit 无对应关系，`align-template-id` 整批搬到预留区且不做名字匹配，`precheck-align-template-id` 整批跳过。`0` 或不填表示没有例外业务 |

---

## 1. 数据迁移命令 (`migrate`)

将 GSEKit 源数据库中的数据迁移到 BSCP 目标数据库。

### 命令格式

```bash
bk-bscp-gsekit-migration migrate -c <配置文件路径> --biz-ids <业务ID列表> [选项]
```

### 可用选项

| 选项 | 说明 |
|---|---|
| `-c, --config` | 配置文件路径（必填） |
| `--biz-ids` | 逗号分隔的业务 ID 列表（**必填**） |
| `-y, --yes` | 跳过确认提示，直接执行迁移 |

### 迁移步骤

工具按以下顺序依次执行迁移：

1. **创建模板空间** — 在目标库中为每个业务创建名为 `config_delivery` 的模板空间。该空间是业务级共享的系统内置空间，服务于进程配置管理，产品上不引入项目概念，`project_id` 固定写入哨兵值 `0`（`constant.GlobalProjectID`），与 BSCP 主干中 `config_delivery` 的处理方式一致。不能留空：唯一索引 `(tenant_id, biz_id, project_id, name)` 中 `NULL ≠ NULL`，留空会让业务内唯一性失效且无法自愈
2. **迁移进程数据** — 将 `gsekit_process` 表数据迁移到 BSCP 的 `processes` 表
3. **迁移进程实例** — 将 `gsekit_processinst` 表数据迁移到 BSCP 的 `process_instances` 表
4. **迁移配置模板** — 将配置模板及版本迁移到 BSCP 的 `templates` / `template_revisions` 表，同时上传模板内容到制品库
5. **迁移配置实例** — 将配置实例迁移到 BSCP 的 `config_instances` 表

> 目标库需已执行项目与环境数据隔离的 DB migration（`20260817154117_add_core_table_columns` 及之后）。
> 未执行时第 1 步会因 `template_spaces` 缺少 `project_id` 列而报 `Unknown column` 并中止，此时不会产生任何写入。

### 使用示例

```bash
# 迁移指定业务
bk-bscp-gsekit-migration migrate -c etc/migration.yaml --biz-ids 2,3

# 指定业务 ID 并跳过确认
bk-bscp-gsekit-migration migrate -c etc/migration.yaml --biz-ids 2,3,5 -y

# 仅迁移单个业务
bk-bscp-gsekit-migration migrate -c etc/migration.yaml --biz-ids 100 -y
```

### 幂等性说明

- 执行迁移前，工具会自动检查目标库中是否已存在对应业务的迁移数据（通过 `template_spaces` 表中 `name=config_delivery` 的记录判断）。
- 如果检测到业务已迁移，工具将**拒绝执行**并提示先执行 `cleanup` 命令清除旧数据后再重试。
- 写入 `config_templates` 前会检查本批 GSEKit `config_template_id` 是否已在目标库被占用。主键直接复用 GSEKit ID，若冲突则**整单拒绝迁移**（不受 `continue_on_error` 影响），提示先跑 `align-template-id` 把自建模版腾出 GSEKit 区间，或 `cleanup` 残留数据后再重试。

### 执行结果

迁移完成后会输出一份报告，包含每个步骤的执行状态、耗时和 ID 映射统计：

```
========== Migration Report ==========
Status: SUCCESS
Duration: 12.345s
Biz IDs: [2 3]

Steps:
  [OK] Create template spaces (50ms)
  [OK] Migrate processes (3.2s)
  [OK] Migrate process instances (4.1s)
  [OK] Migrate config templates (3.8s)
  [OK] Migrate config instances (1.2s)

ID Mappings:
  Processes: 150
  Config Templates: 42
  Config Versions: 78
  Templates: 42
=======================================
```

---

## 2. 数据清洗命令 (`cleanup`)

从 BSCP 目标数据库中删除指定业务的全部已迁移数据，用于迁移回滚或重新迁移前的数据清理。

### 命令格式

```bash
bk-bscp-gsekit-migration cleanup -c <配置文件路径> --biz-ids <业务ID列表> [选项]
```

### 可用选项

| 选项 | 说明 |
|---|---|
| `-c, --config` | 配置文件路径（必填） |
| `--biz-ids` | 逗号分隔的业务 ID 列表（**必填**） |
| `-f, --force` | 跳过确认提示，强制执行清洗 |

### 清洗范围

工具按反向依赖顺序删除以下表中的迁移数据：

| 顺序 | 目标表 | 说明 |
|---|---|---|
| 1 | `config_instances` | 配置实例 |
| 2 | `config_templates` | 配置模板关联 |
| 3 | `template_revisions` | 模板版本 |
| 4 | `templates` | 模板 |
| 5 | `template_sets` | 模板套餐 |
| 6 | `template_spaces` | 模板空间 |
| 7 | `process_instances` | 进程实例 |
| 8 | `processes` | 进程 |

### 使用示例

```bash
# 交互式确认清洗
bk-bscp-gsekit-migration cleanup -c etc/migration.yaml --biz-ids 2,3

# 指定业务 ID 并跳过确认
bk-bscp-gsekit-migration cleanup -c etc/migration.yaml --biz-ids 2,3 -f

# 清洗后重新迁移
bk-bscp-gsekit-migration cleanup -c etc/migration.yaml --biz-ids 2,3 -f && \
bk-bscp-gsekit-migration migrate -c etc/migration.yaml --biz-ids 2,3 -y
```

### 执行结果

清洗完成后会输出报告，显示每个表的删除记录数：

```
========== Cleanup Report ==========
Status: SUCCESS
Duration: 2.345s

Tables:
  [OK] config_instances: 230 records deleted
  [OK] config_templates: 42 records deleted
  [OK] template_revisions: 78 records deleted
  [OK] templates: 42 records deleted
  [OK] template_sets: 3 records deleted
  [OK] template_spaces: 2 records deleted
  [OK] process_instances: 310 records deleted
  [OK] processes: 150 records deleted
====================================
```

### 注意事项

- 清洗操作**仅删除目标库（BSCP）中的数据**，不会影响源库（GSEKit）数据。
- 清洗操作不可逆，执行前请确认业务 ID 无误。未使用 `-f` 参数时，工具会进行交互式确认。
- 清洗操作仅删除数据库记录，不会清理已上传到制品库（BK-Repo / S3）的配置文件内容。

---

## 3. 连通性校验命令 (`preflight`)

在正式执行迁移前，检查所有外部依赖的连通性和认证状态，包括数据库、CMDB API 和制品库。建议在首次迁移前运行此命令以排查环境问题。

### 校验项目

| 检查项 | 校验方式 | 说明 |
|---|---|---|
| Source MySQL (GSEKit) | TCP 连接 + Ping | 源数据库连通性 |
| Target MySQL (BSCP) | TCP 连接 + Ping | 目标数据库连通性 |
| CMDB API | 发送轻量级 API 请求并验证认证 | CMDB 接口可达性及 app_code/app_secret 有效性 |
| BKRepo / S3 / COS | 发送 HTTP 请求并验证认证 | 制品库可达性及账号密码/密钥有效性 |

### 命令格式

```bash
bk-bscp-gsekit-migration preflight -c <配置文件路径>
```

### 可用选项

| 选项 | 说明 |
|---|---|
| `-c, --config` | 配置文件路径（必填） |

### 使用示例

```bash
# 校验配置文件中所有外部依赖的连通性
bk-bscp-gsekit-migration preflight -c etc/migration.yaml
```

### 执行结果

校验完成后输出报告，显示每个检查项的状态和延迟：

```
========== Preflight Check Report ==========
Status: SUCCESS

Checks:
  [PASS] Source MySQL (GSEKit) (latency: 12ms)
         database=gsekit_db, endpoints=[127.0.0.1:3306]
  [PASS] Target MySQL (BSCP) (latency: 8ms)
         database=bscp_db, endpoints=[127.0.0.1:3306]
  [PASS] CMDB API (latency: 156ms)
         endpoint=http://cmdb.example.com
  [PASS] BKRepo (latency: 89ms)
         endpoint=http://bkrepo.example.com
==============================================
```

当存在校验失败时：

```
========== Preflight Check Report ==========
Status: FAILED

Checks:
  [PASS] Source MySQL (GSEKit) (latency: 12ms)
         database=gsekit_db, endpoints=[127.0.0.1:33060]
  [FAIL] Target MySQL (BSCP) (latency: 15.003s)
         ping failed: context deadline exceeded
  [FAIL] CMDB API (latency: 0s)
         cmdb.endpoint is not configured
  [FAIL] Repository (latency: 0s)
         repository.storage_type is not configured
==============================================
```

### 注意事项

- 任一检查项失败时，命令退出码为 1，可用于脚本中判断环境是否就绪。
- 配置文件中 `cmdb.endpoint` 和 `repository.storage_type` 为必需配置，未配置时对应检查项将直接标记为 FAIL。
- 此命令**不会修改任何数据**，可放心反复执行。

---

## 4. 渲染对比命令 (`compare-render`)

对比 BSCP 渲染引擎与 GSEKit 预览 API 的渲染结果，验证迁移后模板渲染一致性。

### 工作原理

对于每个配置模板，工具执行以下流程：

1. 查询模板的最新已发布版本（非草稿）
2. 通过绑定关系表查找模板关联的进程（优先 INSTANCE 直接绑定，其次 TEMPLATE 绑定）
3. 获取该进程的第一个实例（按主键 ID 升序，与 GSEKit `ProcessInst.get_single_inst()` 一致）
4. 分别调用 **GSEKit 预览 API** 和 **BSCP Mako 渲染引擎** 渲染模板
5. 对比两者输出，记录差异

### 前置要求

- 配置文件中需要填写 `gsekit` 段的 API 网关配置
- `bk_ticket` 为用户登录态 ticket，可从浏览器 Cookie 中获取
- 需要配置 `cmdb` 段用于获取进程上下文（集群名、模块名、主机 IP 等）
- 需要配置 `source.mysql` 用于读取 GSEKit 源数据

### 命令格式

```bash
bk-bscp-gsekit-migration compare-render -c <配置文件路径> --biz-ids <业务ID列表> [选项]
```

### 可用选项

| 选项 | 说明 |
|---|---|
| `-c, --config` | 配置文件路径（必填） |
| `--biz-ids` | 逗号分隔的业务 ID 列表（**必填**） |
| `-o, --output` | JSON 报告输出文件路径，默认 `compare-render-report-<YYYYMMDD-HHMMSS>.json`（带时间戳，不会覆盖历史报告） |
| `--show-diff` | 显示不一致模板的 unified diff，默认开启 |
| `--diff-context-lines` | diff 上下文行数，默认 3 |
| `--render-timeout` | 单次渲染超时时间，默认 `30s` |

### 使用示例

```bash
# 对比指定业务的渲染结果（默认开启 diff 显示并输出 JSON 报告）
bk-bscp-gsekit-migration compare-render -c etc/migration.yaml --biz-ids xxx

# 关闭 diff 显示
bk-bscp-gsekit-migration compare-render -c etc/migration.yaml --biz-ids xxxx --show-diff=false

# 指定报告输出路径
bk-bscp-gsekit-migration compare-render -c etc/migration.yaml --biz-ids xxxx -o my-report.json

# 调整渲染超时时间
bk-bscp-gsekit-migration compare-render -c etc/migration.yaml --biz-ids xxx --render-timeout 60s
```

### 执行结果

对比完成后输出报告：

```
========== Compare Render Report ==========
Status: SUCCESS

Biz 100148:
  Total:         42
  Matched:       40
  Mismatched:    1
  Render Failed: 0
  Skipped:       1

  Differences (1):
    - Template 123/nginx.conf (version=456, process=22445554): content_mismatch
=============================================
```

### 结果字段说明

| 字段 | 说明 |
|---|---|
| `Total` | 参与对比的模板总数 |
| `Matched` | 渲染结果一致的模板数 |
| `Mismatched` | 渲染结果不一致的模板数 |
| `Render Failed` | 渲染失败的模板数（GSEKit 或 BSCP 渲染出错） |
| `Skipped` | 跳过的模板数（无绑定进程或无进程实例） |

### 差异原因分类

| Reason | 说明 |
|---|---|
| `content_mismatch` | GSEKit 和 BSCP 渲染结果不一致 |
| `render_error` | BSCP 渲染引擎执行失败 |
| `gsekit_render_error` | GSEKit 预览 API 返回错误 |
| `ginclude_expand_error` | BSCP 侧 Ginclude 指令展开失败 |

---

## 5. 配置模板 ID 对齐前置校验 (`precheck-align-template-id`)

在跑 `align-template-id` **之前**的只读确认：迁移产物当前名字是否仍能在 GSEKit
同业务下命中。与对齐命令无耦合，不改库、不阻塞对齐；是否继续由运维看报告自行决定。

### 命令格式

```bash
./bk-bscp-gsekit-migration precheck-align-template-id -c <配置文件> [-o <报告路径>]
```

| 选项 | 说明 |
|---|---|
| `-c, --config` | 配置文件路径（必填），使用其中的双库连接、`migration.creator` 与 `migration.native_biz_id` |
| `-o, --output` | 报告 JSON 路径，默认 `precheck-align-template-id-report-<YYYYMMDD-HHMMSS>.json` |

### 判定规则

1. 读 BSCP `config_templates`，筛 `creator == migration.creator` 且 `biz_id != migration.native_biz_id`（`native_biz_id` 为 0 时不过滤业务）
2. 用 `(biz_id, name)` 查 GSEKit `gsekit_configtemplate`
3. 命中 → OK；未命中 → ALERT

有 ALERT 时进程退出码非 0，报告仍会写出。建议先处理 ALERT，再执行对齐。

### 建议顺序

```bash
./bk-bscp-gsekit-migration precheck-align-template-id -c etc/migration.yaml
./bk-bscp-gsekit-migration align-template-id -c etc/migration.yaml          # dry-run
./bk-bscp-gsekit-migration align-template-id -c etc/migration.yaml --execute
```

## 6. 配置模板 ID 对齐命令 (`align-template-id`)

把已迁移的 `config_templates.id` 对齐到 GSEKit 的 `config_template_id`。

### 为什么需要对齐

标准运维（bk-sops）的流程模板里保存的是 GSEKit 的配置模板 ID，插件执行时把它作为
`config_template_ids` 传给 BSCP。而首批迁移时 `config_templates.id` 由 BSCP 的
`id_generators` 重新分配，与 GSEKit 无关，导致插件查不到模板、流程执行失败，
用户必须重新编辑并保存每一条流程才能恢复。

只有 `config_templates.id` 需要对齐。插件没有保存版本 ID（插件模式下 BSCP 自行取最新版本），
`templates.id` 与 `template_revisions.id` 无需处理。

本命令是一次性的存量修复。`migrate` 流程已同步改造为写入时直接复用 GSEKit ID，
后续批次迁移不会再产生同类问题，也不需要再跑本命令。

### 命令格式

```bash
./bk-bscp-gsekit-migration align-template-id -c <配置文件> [选项]
```

### 可用选项

| 选项 | 说明 |
|---|---|
| `-c, --config` | 配置文件路径（必填） |
| `--dry-run` | 只输出报告不改数据。默认行为，不传 `--execute` 即等于试跑 |
| `--execute` | 真正执行搬迁。与 `--dry-run` 互斥，必须显式传入 |
| `-o, --output` | 报告输出路径，默认 `align-template-id-report-<YYYYMMDD-HHMMSS>.json` |

这是全库操作，不接受 `--biz-ids`：`[1, 预留基线)` 区间要一次性清空，按业务分批做不出正确结果。

### 执行阶段

| 阶段 | 内容 |
|---|---|
| 0 体检 | 读 GSEKit `gsekit_configtemplate` 的 `AUTO_INCREMENT`，达到预留基线则失败退出；读 BSCP `config_templates` 全量与 `id_generators` 水位；存在 `status = 'running'` 的 `task_batches` 则拒绝执行 |
| 1 建映射 | 按 `(biz_id, name)` 在 GSEKit 中匹配，得出每条记录的目标 ID 与分类 |
| 2 报告 | 报告 JSON 落盘。`--dry-run` 到此结束 |
| 3 执行 | 先独立提交水位抬高，再在单个事务内腾空、入位、改写引用、收尾水位 |
| 4 校验 | 三项只读断言 |

预留基线取 `AUTO_INCREMENT` 而非 `MAX(config_template_id)`：GSEKit 的模板是硬删除且主键不回收，
`MAX` 会明显低于真实水位。

### 匹配策略

只用 `(业务, 模板名)` 匹配。GSEKit 侧 `unique_together (bk_biz_id, template_name)`，
BSCP 侧 `uniqueIndex (biz_id, name)`，且迁移时 `config_templates.name` 直接取
`gsekit.template_name`，所以这是双侧唯一的天然键。

盲区是业务在 BSCP 侧改过模板名。执行前已核对存量数据：15 条被改动过的记录里，
4 条属于迁移产物，全部未改名，名字匹配 100% 命中，**不存在真实改过名的迁移产物**。
因此不再保留版本 ID 回溯之类的兜底策略。

历史数据里 GSEKit 版本 ID 唯一的残留痕迹是 `template_revisions.revision_name`
的 `v<config_version_id>` 形式，但 BSCP 页面更新模板时填入的默认版本名同样是 `v` 加数字
（时间戳形式），格式本身不能作为判据，一旦真的需要回溯必须逐个反查 GSEKit 才能锚定。

### 分类与处理

| 分类 | 含义 | 处理 |
|---|---|---|
| `FORCED_NATIVE` | `biz_id` 等于 `migration.native_biz_id` | 搬到预留区，不做名字匹配 |
| `MATCHED_NAME` | 名字在 GSEKit 同业务下命中 | 自动对齐到该 ID。已经相等的记录完全不动 |
| `UNMATCHED_NATIVE` | 未命中，特征像 BSCP 自建 | 搬到预留区 |
| `UNMATCHED_UNKNOWN` | 未命中，特征却像迁移产物 | 阻塞执行，在报告中列出交人工处理 |

后两者的区分判据是 `creator` 等于配置里的 `migration.creator`（例如 `xiaolnwang`）。
判据不精确，因此误判方向刻意保守：把自建误判成 `UNMATCHED_UNKNOWN` 只是多一次人工确认，
反之会导致漏对齐、问题残留。判据不参与"对齐到哪个 ID"的决策，只决定是否需要人工介入。

`migration.native_biz_id` 指定的业务是例外，它在判据之前先被拦下。这个业务在 BSCP
侧的模板是人工自建的，与 GSEKit 无对应关系，按 `creator` 判据会被整批误判成
`UNMATCHED_UNKNOWN` 从而阻塞执行。因此整批归入自建区，即便某条记录的名字
恰好与 GSEKit 撞上也不对齐——那只是巧合，不是同一份模板。`precheck-align-template-id`
对这个业务的处理与此一致，同样整批跳过。示例配置里写的是 `100148`。

已经落在预留区的自建模板不会被搬动，省掉一次无谓的引用改写。

### 搬迁算法

`[1, 预留基线)` 整个区间一次性清空并永久划归 GSEKit 对齐专用，代价是所有 BSCP
自建模板的 ID 都会变一次。相比只移动真正冲突的行，全区腾空避免了残留记录成为
后续每批迁移的冲突源。

每一条要移动的记录都先搬到高位临时 ID，再落到终值，不做"目标必然空闲"的一步到位优化：
搬迁会让一行的新 ID 恰好是另一行的旧 ID，少走一次中转就可能撞主键。

三处引用同步改写——`config_instances.config_template_id`、`task_batches.task_data`
里的 `config_template_ids` 数组、`audits` 中 `res_type = 'config_template'` 的 `res_id`。
前两处用单条 `CASE WHEN` 语句在同一语句内基于原值求值，逐条 UPDATE 会让先改出来的值
被后一条再次改掉。BSCP 不使用外键约束，改主键既不级联也不报错，漏改一处只会留下静默的脏数据。

### 报告样例

```json
{
  "generated_at": "2026-08-13T10:24:05+08:00",
  "dry_run": true,
  "executed": false,
  "reserve_base": 20000,
  "gsekit_auto_increment": 11249,
  "id_generator_max_id": 203,
  "summary": {
    "MATCHED_NAME": 166,
    "UNMATCHED_NATIVE": 37
  },
  "records": [
    {
      "bscp_id": 55,
      "biz_id": 5000079,
      "name": "server.conf",
      "classification": "MATCHED_NAME",
      "gsekit_id_by_name": 11120,
      "final_new_id": 11120,
      "decision_source": "name"
    }
  ],
  "moves": [
    { "old_id": 55, "temp_id": 20001, "final_id": 11120 }
  ],
  "reference_impact": {
    "config_instances": 1842,
    "task_batches": 26,
    "audits": 97
  }
}
```

`moves` 段是执行失败时人工回滚的唯一依据，务必保留报告文件。

### 使用示例

```bash
# 1. 先试跑，检查报告
./bk-bscp-gsekit-migration align-template-id -c migration.yaml

# 2. 确认报告里 UNMATCHED_UNKNOWN 为 0 后执行
./bk-bscp-gsekit-migration align-template-id -c migration.yaml --execute

# 3. 指定报告路径
./bk-bscp-gsekit-migration align-template-id -c migration.yaml --execute -o align-report.json
```

`UNMATCHED_UNKNOWN` 非空时即使传了 `--execute` 也会被拒绝，此时应先人工核对报告里
列出的记录，确认它们到底是自建模板还是漏匹配的迁移产物。

### 并发与停机

水位抬高是一个可以独立先提交的原子操作，它一旦生效，之后任何并发新建拿到的 ID
都落在预留区，不可能撞进 GSEKit 区间。剩下的搬迁只和存量的两百多行竞争，
在单个事务内完成，耗时毫秒到秒级。配合阶段 0 的"无进行中任务"检查，不需要停机窗口。

### 手工验证步骤

单元测试只覆盖纯函数（分类判定、搬迁规划、`CASE WHEN` 生成、`task_data` 改写）。
集成、幂等、回滚需要在真实库上手工验证：

1. **试跑**：`--dry-run` 看报告，确认 `UNMATCHED_UNKNOWN` 为 0，
   `moves` 条数与 `MATCHED_NAME` 里 ID 不相等的条数加上 `UNMATCHED_NATIVE` 条数吻合。
2. **执行**：`--execute` 后确认阶段 4 三项断言全部 PASS。
3. **抽查**：取报告里几条 `moves`，在库里核对 `config_templates.id` 已是 GSEKit ID，
   且对应的 `config_instances.config_template_id` 一并改过。
4. **验幂等**：再跑一次 `--execute`，`moves` 应为空、断言全部 PASS。
5. **端到端**：在 bk-sops 里执行一条此前失败的流程，确认插件能查到模板。
6. **回滚（如需要）**：按报告 `moves` 段反向执行——先把 `final_id` 搬到 `temp_id`，
   再搬回 `old_id`，同时用 `final_id → old_id` 的映射反向改写三处引用。
   `id_generators.max_id` 不必回退，水位只增不减是安全的。

### 与 migrate 的关系

`migrate` 流程已一并改造：

- `config_templates.id` 直接复用 `tmpl.ConfigTemplateID`，不再走 `id_generators`。
  `templates` 与 `template_revisions` 的 ID 分配方式不变。
- 每批迁移开始前校验本批业务的 GSEKit `config_template_id` 最大值，
  达到预留基线则拒绝迁移，避免静默覆盖 BSCP 自建模板的 ID。
- 同一时机还会查出这些 GSEKit ID 是否已存在于 BSCP `config_templates`。
  有冲突则拒绝迁移并列出占用行（id / biz_id / name / creator），避免 INSERT 撞主键。
- 迁移结束后把 `id_generators` 中 `config_templates` 的水位抬到预留基线之上，
  否则后续自建模板会分配到已被迁移数据占用的低位 ID。
- 顺带修复了读取待迁移模板的分页查询缺少 `ORDER BY` 的既有缺陷。MySQL 不保证无序分页
  跨批次的结果稳定，可能重复返回部分行（撞唯一索引后在 `continue_on_error` 为真时被静默跳过）
  并遗漏另一部分行。触发条件是单业务模板数超过 `batch_size`，已迁移业务中最多的仅 64 条，
  存量迁移大概率未触发，但后续业务需要这个修复。
