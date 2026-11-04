# 开发地图（Dev Map）

> 基于 graphify 知识图谱，帮助 Agent 和开发者理解项目结构与概念关联。
> 相比旧式 Markdown 索引，token 消耗降低约 70x。

当前图谱按首次生成时确认的范围覆盖 `cmd/`、`internal/`、`pkg/` 和 `api/`；查询 `ui/` 等未纳入目录时应直接检索源码，或重新全量生成图谱。

## 使用方式

调用 graphify skill 查询图谱（无需指定 `--graph` 路径参数，skill 已配置图谱位于 `docs/dev-map/graph.json`）：

| 调用方式 | 用途 |
|---------|------|
| `/graphify query "<问题>"` | 自然语言查询 |
| `/graphify path "<A>" "<B>"` | 路径查询——找两个概念之间的最短连接路径 |
| `/graphify explain "<概念>"` | 概念解释——展开某节点的定义、关联节点和所在 community |

## 维护

代码提交后运行 harness-gardening 轻量检查可增量更新；当前仓库未安装自动 post-commit hook。
图谱文件不存在时自动全量生成。
手动触发词："更新 dev map" 或 "生成开发地图"。

`graph.json` 与 `graph.html`、`GRAPH_REPORT.md` 等生成物不入库（见 `.gitignore`），仅在本地生成和使用。

## 首次生成

`graph.json` 在首次运行 harness-generating（或触发词"更新 dev map"）后自动生成。
