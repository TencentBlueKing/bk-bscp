# GSEKit 迁移工具使用说明

## 概述

`gsekit-migration` 是一个将 GSEKit（进程配置管理）数据迁移至 BSCP（蓝鲸服务配置平台）的命令行工具。工具支持按业务 ID 粒度进行数据迁移与清洗，具备幂等性校验，已迁移的业务需先执行清洗才能重新迁移。

## 前置准备：配置文件

所有命令都需要通过 `-c / --config` 指定一个 YAML 配置文件。配置文件示例位于 `cmd/gsekit-migration/etc/migration.yaml`，完整示例如下：

```yaml
migration:
  multi_tenant: false        # 是否多租户模式，false 时 tenant_id 强制为 "default"
  tenant_id: "default"       # 目标租户 ID
  creator: "admin"           # 迁移记录的创建者
  reviser: "admin"           # 迁移记录的修改者
  biz_ids: [2, 3]            # 需要迁移的业务 ID 列表（必填）
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

log:
  level: "info"              # 日志级别: debug / info / warn / error
```

### 关键配置说明

| 配置项 | 说明 |
|---|---|
| `migration.biz_ids` | **必填**。指定需要迁移/清洗的业务 ID 列表 |
| `source.mysql` | GSEKit 源数据库连接信息 |
| `target.mysql` | BSCP 目标数据库连接信息 |
| `repository` | 配置文件内容存储后端（BK-Repo 或 S3/COS），用于上传配置模板内容 |
| `cmdb` | CMDB API 配置，用于查询进程关联的主机信息 |

---

## 1. 数据迁移命令 (`migrate`)

将 GSEKit 源数据库中的数据迁移到 BSCP 目标数据库。

### 命令格式

```bash
gsekit-migration migrate -c <配置文件路径> [选项]
```

### 可用选项

| 选项 | 说明 |
|---|---|
| `-c, --config` | 配置文件路径（必填） |
| `--biz-ids` | 逗号分隔的业务 ID 列表，覆盖配置文件中的 `biz_ids` |
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
# 使用配置文件迁移
gsekit-migration migrate -c etc/migration.yaml

# 指定业务 ID 并跳过确认
gsekit-migration migrate -c etc/migration.yaml --biz-ids 2,3,5 -y

# 仅迁移单个业务
gsekit-migration migrate -c etc/migration.yaml --biz-ids 100 -y
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
gsekit-migration cleanup -c <配置文件路径> [选项]
```

### 可用选项

| 选项 | 说明 |
|---|---|
| `-c, --config` | 配置文件路径（必填） |
| `--biz-ids` | 逗号分隔的业务 ID 列表，覆盖配置文件中的 `biz_ids` |
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
gsekit-migration cleanup -c etc/migration.yaml

# 指定业务 ID 并跳过确认
gsekit-migration cleanup -c etc/migration.yaml --biz-ids 2,3 -f

# 清洗后重新迁移
gsekit-migration cleanup -c etc/migration.yaml -f && \
gsekit-migration migrate -c etc/migration.yaml -y
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
