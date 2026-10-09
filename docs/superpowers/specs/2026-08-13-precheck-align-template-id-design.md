# 配置模版 ID 对齐前置校验设计

日期：2026-08-13

## 背景

`align-template-id` 按 `(biz_id, name)` 把存量 `config_templates.id` 对齐到 GSEKit。
执行前需确认：被识别为迁移产物的记录，当前名字仍能在 GSEKit 同业务下命中。
历史 SQL 手工校验会过期，因此提供独立只读子命令，在对齐前再跑一次。

本工具**不参与** ID 重构流程，不阻塞、不调用 `align-template-id`；是否继续对齐由运维根据报告自行判断。

## 命令

```
bk-bscp-gsekit-migration precheck-align-template-id -c <配置文件> [-o <报告路径>]
```

| 选项 | 说明 |
| --- | --- |
| `-c, --config` | 配置文件（必填），复用双库连接与 `migration.creator` |
| `-o, --output` | 报告 JSON 路径，默认 `precheck-align-template-id-report-<YYYYMMDD-HHMMSS>.json` |

只读。无 `--execute` / `--dry-run`。

## 判定规则

1. 读 BSCP `config_templates` 全量。
2. 候选：`creator == migration.creator` 且 `biz_id != migration.native_biz_id`（为 0 时不过滤业务）。
3. 用 `(biz_id, name)` 查 GSEKit `gsekit_configtemplate`。
4. 命中 → `OK`（记录对应 `gsekit_config_template_id`）；未命中 → `ALERT`。

不做版本回溯、不做业务条数对账、不改任何数据。

## 报告

终端打印 summary 与全部 ALERT 明细。

JSON 含：`generated_at`、`migration_creator`、`excluded_biz_ids`、`summary`、`alerts`、`oks`。

有 ALERT 时进程退出码非 0，便于一眼发现问题；仍不阻止随后手动执行对齐。

## 与 align-template-id 的关系

| | precheck | align-template-id |
| --- | --- | --- |
| 职责 | 只读确认迁移产物能否按名命中 GSEKit | 腾空 / 入位 / 改引用 |
| 耦合 | 无 | 无 |
| 运维顺序 | 建议先跑 | 看报告后再决定是否跑 |

## 测试

- 单元：候选过滤（creator / `native_biz_id` 排除）、名字命中 / 未命中分类。
- 不强制集成测双库；逻辑以纯函数单测为主。
