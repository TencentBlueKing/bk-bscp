# GSEKit 迁移工具命令行参考

所有命令均需通过 `-c / --config` 指定 YAML 配置文件。

容器方式运行时，先拉取镜像：

```bash
docker pull mirrors.tencent.com/bk-bcs-xiaolnwang/gsekit-migrate:latest
```

---

## preflight — 校验外部依赖连通性

```bash
bk-bscp-gsekit-migration preflight -c <配置文件>
```

| 参数 | 说明 |
|---|---|
| `-c, --config` | 配置文件路径（必填） |

容器运行：

```bash
docker run --rm \
  -v /root/xiaolnwang/migration.yaml:/app/migration.yaml \
  mirrors.tencent.com/bk-bcs-xiaolnwang/gsekit-migrate:latest \
  preflight -c /app/migration.yaml
```

---

## compare-render — 对比 GSEKit 与 BSCP 的模板渲染结果

迁移前执行，验证 BSCP 渲染引擎与 GSEKit 预览 API 输出是否一致。

```bash
bk-bscp-gsekit-migration compare-render -c <配置文件> --biz-ids <业务ID>
```

| 参数 | 说明 |
|---|---|
| `-c, --config` | 配置文件路径（必填） |
| `--biz-ids` | 逗号分隔的业务 ID 列表（必填） |
| `-o, --output` | JSON 报告输出路径（默认带时间戳） |
| `--show-diff` | 显示 unified diff（默认开启） |
| `--diff-context-lines` | diff 上下文行数（默认 3） |
| `--render-timeout` | 单次渲染超时（默认 30s） |

容器运行（挂载报告输出目录）：

```bash
docker run --rm \
  -v /root/xiaolnwang/migration.yaml:/app/migration.yaml \
  -v /root/xiaolnwang/reports:/app/reports \
  mirrors.tencent.com/bk-bcs-xiaolnwang/gsekit-migrate:latest \
  compare-render -c /app/migration.yaml --biz-ids 100148 \
  -o /app/reports/compare-render-report.json
```

> 容器使用 `--rm` 退出即销毁，需通过 `-o` 将报告输出到挂载目录，否则文件会随容器删除而丢失。

---

## migrate — 将 GSEKit 数据迁移至 BSCP

```bash
bk-bscp-gsekit-migration migrate -c <配置文件> --biz-ids <业务ID>
```

| 参数 | 说明 |
|---|---|
| `-c, --config` | 配置文件路径（必填） |
| `--biz-ids` | 逗号分隔的业务 ID 列表（必填） |
| `-y, --yes` | 跳过确认提示 |

容器运行：

```bash
    docker run --rm \
    -v /root/xiaolnwang/migration.yaml:/app/migration.yaml \
    mirrors.tencent.com/bk-bcs-xiaolnwang/gsekit-migrate:latest \
    migrate -c /app/migration.yaml --biz-ids 100148 -y
```

---

## cleanup — 清除目标库中已迁移的数据

用于迁移回滚或重新迁移前的数据清理，仅影响 BSCP 目标库。

```bash
bk-bscp-gsekit-migration cleanup -c <配置文件> --biz-ids <业务ID>
```

| 参数 | 说明 |
|---|---|
| `-c, --config` | 配置文件路径（必填） |
| `--biz-ids` | 逗号分隔的业务 ID 列表（必填） |
| `-f, --force` | 跳过确认提示 |

容器运行：

```bash
docker run --rm \
  -v /root/xiaolnwang/migration.yaml:/app/migration.yaml \
  mirrors.tencent.com/bk-bcs-xiaolnwang/gsekit-migrate:latest \
  cleanup -c /app/migration.yaml --biz-ids 100148 -f
```
