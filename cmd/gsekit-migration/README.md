# GSEKit 迁移工具使用说明

## 概述

`bk-bscp-gsekit-migration` 是一个将 GSEKit（进程配置管理）数据迁移至 BSCP（蓝鲸服务配置平台）的命令行工具。工具支持按业务 ID 粒度进行数据迁移与清洗，具备幂等性校验，已迁移的业务需先执行清洗才能重新迁移。

## 前置准备：配置文件

所有命令都需要通过 `-c / --config` 指定一个 YAML 配置文件。配置文件示例位于 `cmd/gsekit-migration/etc/migration.yaml`，完整示例如下：

```yaml
migration:
  multi_tenant: false        # 是否多租户模式，false 时 tenant_id 强制为 "default"
  tenant_id: "default"       # 目标租户 ID
  creator: "admin"           # 迁移记录的创建者
  reviser: "admin"           # 迁移记录的修改者
  batch_size: 500            # 每批处理的记录数，默认 500
  continue_on_error: false   # 遇到错误是否继续迁移

source:
  mysql:
    endpoints: ["127.0.0.1:33060"]
    database: "gsekit_db"
    user: "root"
    password: "xxx"

target:
  mysql:
    endpoints: ["127.0.0.1:3306"]
    database: "bscp_db"
    user: "root"
    password: "xxx"

repository:
  storage_type: "BKREPO"     # 存储后端类型: "BKREPO" 或 "S3"
  bk_repo:
    endpoint: "http://bkrepo.example.com"
    project: "bscp"
    username: "admin"
    password: "xxx"

cmdb:
  endpoint: "http://cmdb.example.com"
  app_code: "bk_bscp"
  app_secret: "xxx"
  username: "admin"

gsekit:                       # 仅 compare-render 命令需要
  endpoint: "https://bk-gsekit.apigw.o.woa.com/prod"
  app_code: "bk-bscp"
  app_secret: "xxx"
  bk_ticket: "xxx"            # 用户登录态 ticket

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

1. **创建模板空间** — 在目标库中为每个业务创建名为 `config_delivery` 的模板空间
2. **迁移进程数据** — 将 `gsekit_process` 表数据迁移到 BSCP 的 `processes` 表
3. **迁移进程实例** — 将 `gsekit_processinst` 表数据迁移到 BSCP 的 `process_instances` 表
4. **迁移配置模板** — 将配置模板及版本迁移到 BSCP 的 `templates` / `template_revisions` 表，同时上传模板内容到制品库
5. **迁移配置实例** — 将配置实例迁移到 BSCP 的 `config_instances` 表

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
         database=gsekit_db, endpoints=[127.0.0.1:33060]
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
bk-bscp-gsekit-migration compare-render -c etc/migration.yaml --biz-ids 100148

# 关闭 diff 显示
bk-bscp-gsekit-migration compare-render -c etc/migration.yaml --biz-ids 100148 --show-diff=false

# 指定报告输出路径
bk-bscp-gsekit-migration compare-render -c etc/migration.yaml --biz-ids 100148 -o my-report.json

# 调整渲染超时时间
bk-bscp-gsekit-migration compare-render -c etc/migration.yaml --biz-ids 100148 --render-timeout 60s
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
