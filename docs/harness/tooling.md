# 工具能力（Tooling）

> 目标：让 Agent 使用边界清晰、可审计且可验证的工具完成任务。依赖权威源为 `.codebuddy/skills/harness-engineering/references/tool-dependencies.md`。

## 1. 工具清单

### 1.0 Skill 清单与触发

`$SKILL_ROOT` 按接入优先级识别为 `.codebuddy/skills`。下表仅列出该目录顶层已安装且在工具依赖权威清单登记的 Skill；子 Skill 由父 Skill 编排。

| Skill | 典型触发 | 功能概要 |
|-------|---------|---------|
| `harness-engineering` | Harness 规范、文档园艺、开发地图 | 生成并维护 Agent 运行环境规范 |
| `graphify` | 查询代码结构、生成/更新开发地图 | 构建和查询持久化知识图谱 |
| `tapd-story-clarification` | 需求澄清、完善需求描述 | 拉取并规范化 TAPD 需求 |
| `tapd-story-review` | 需求评审、评论闭环 | 驱动澄清或拆分结果通过评审 |
| `tapd-story-evaluation` | 需求拆分、RICE 评分 | 拆分需求并进行价值规模评估 |
| `tapd-story-govern-pipeline` | 需求整理流水线 | 编排澄清、评审和评估 |
| `tapd-story-pipeline` | 实现单个需求、缺陷修复 | 编排规范、计划、TDD、校验和提交 |
| `speckit-specify` | 生成 feature spec | 从需求描述生成或更新规范 |
| `speckit-plan` | 生成实现计划 | 根据规范产出设计与计划 |
| `speckit-tasks` | 生成任务清单 | 生成依赖有序的 `tasks.md` |
| `speckit-analyze` | 检查 spec/plan/tasks | 非破坏性检查产物一致性 |
| `speckit-checklist` | 生成验收清单 | 按需求生成定制检查清单 |
| `speckit-implement` | 执行 tasks.md | 按任务清单实施并更新状态 |
| `speckit-constitution` | 项目原则、constitution | 创建或同步项目开发原则 |
| `code-review` | 代码评审 | 按代码评审规范检查变更 |
| `bk-security-redlines` | 安全评审、红线检查 | 检查输入校验、鉴权和数据加密红线 |

`bkbscp-kv-config`、`bkbscp-file-config` 和 OpenSpec 等已安装 Skill 未在当前权威依赖清单登记，因此不纳入本表；如需纳管，应先更新治理仓权威清单，再重新生成。

### 1.1 MCP

| MCP 逻辑名 | 所需接口 | 必需条件 | 2026-07-29 环境状态 |
|------------|---------|---------|----------------------|
| `tapd` | `stories_*`、`bugs_*`、`iterations_*`、`comments_*` 等 | TAPD 需求/缺陷/迭代流程 | 已就绪；`stories_get` 只读探测成功 |
| `gongfeng` | Issue、MR、提交只读接口 | 工蜂 Issue 前置流程 | 当前场景未启用，未探测 |
| `bkm-bkte` | metrics、logs、tracing、alarms 等 | SRE 可观测性流程 | 当前场景未启用，未探测 |

实际 Server 标识由 IDE/MCP 配置决定，文档只使用逻辑名。MCP 写操作必须使用对应 Skill 的参数约束、确认点和验证闭环。

### 1.2 CLI

本次按场景 A（TAPD 迭代研发）、D（评审与安全）和 G（Harness）探测。`go.mod` 中的 `go-micro.dev/v4` 仅为间接依赖，源码没有 go-micro import，因此场景 C 不启用；Proto 工具状态仅作为项目生成链路事实记录，不计入场景 C 缺口。

| 工具 | 必需 | 检测条件 | 2026-07-29 环境状态 |
|------|------|---------|----------------------|
| `git` | 是 | 始终 | 已就绪 |
| `bash` | 是 | 场景 A | 已就绪 |
| `jq` | 是 | 场景 A 的迭代报告 | **缺失** |
| `go` | 是 | 根目录存在 `go.mod` | 已就绪，Go 1.25.0 |
| `protoc` | 项目生成链路 | 运行 Proto 生成目标 | 已安装 25.1，与 Makefile 的 `PROTOC_VERSION` 一致 |
| `protoc-gen-go` | 项目生成链路 | 同上 | 已就绪 |
| `protoc-gen-grpc-gateway` | 项目生成链路 | 同上 | 已就绪 |
| `protoc-gen-openapiv2` | 项目生成链路 | 同上 | 已就绪 |
| `make` | 项目生成链路 | 构建与生成 | 已就绪 |
| `graphify` | 否 | 生成/更新开发地图 | 由 `graphify` Skill 校验并按固定版本安装 |

`docker`、`node`、`python3`、`gh` 等可选工具按任务需要安装或探测；不将未主动探测等同于缺失。

### 1.3 Agent 与配置

| 项目 | 要求 | 状态 |
|------|------|------|
| `project.json` | `workspace_id`、`owner` | 文件存在；`workspace_id` 有效，`owner` 仍为占位值 `xxx` |
| `tapd-story-agent` | `.codebuddy/agents/tapd-story-agent.md` | 已就绪 |
| `speckit-execution-agent` | 权威清单中的历史名称 | 实际已部署 `.codebuddy/agents/speckit-executor-agent.md`；治理仓需统一命名 |
| `workflow-agent` | `.codebuddy/agents/workflow-agent.md` | 已就绪 |
| `tapd-iteration-plan` | 工作流第 3 步 | **缺失；工作流执行到迭代规划时阻塞** |
| `tapd-iteration-runner` | 工作流第 4 步 | **缺失；工作流执行到迭代开发时阻塞** |
| Spec Kit 命令 | `.agents/commands/speckit.*.md` | **缺失；Speckit 命令入口不可用** |
| Spec Kit 项目结构 | `.specify/` | **缺失；Speckit 计划与执行阶段不可用** |

## 2. 工具接口规范

### 2.1 调用契约

- 输入使用结构化参数，区分必填、可选、默认值和业务约束。
- 输出至少明确成功状态、业务数据和可行动错误；不得仅返回无法定位原因的自由文本。
- 读取外部数据时记录逻辑环境、对象标识、分页条件和采集时间。
- 写操作应具备幂等键或先查后写策略，并在写后读取验证。
- 参数 schema 不足以表达业务约束时，必须加载对应 Skill，不凭经验补字段。

### 2.2 错误分类

| 类型 | 处理 |
|------|------|
| 临时网络/限流 | 最多 3 次指数退避，保持幂等 |
| 参数或前置条件错误 | 不重试；修正参数或补齐前置数据 |
| 鉴权/授权失败 | 确认一次后停止并报告，不绕过 |
| 资源不存在 | 核对环境和标识，禁止自动创建替代资源 |
| 部分成功 | 明确成功/失败子项，按协议决定补偿或人工处理 |
| 工具不可用 | 使用文档声明的降级路径；无安全替代时停止 |

## 3. 稳定性与安全

### 3.1 执行边界

| 操作 | 约束 |
|------|------|
| 文件编辑 | 限于工作区和用户授权范围，保护未提交改动 |
| 删除/批量替换 | 必须确认目标集合与影响；优先可恢复方式 |
| Git | 禁止强推、硬重置、绕过 hooks/GPG；提交和推送需用户明确授权 |
| 生产配置 | 只读查询可按 Skill 执行；新增、更新、删除、发布须完成确认与发布后验证 |
| 数据库迁移 | 需设计评审、备份/恢复或前向修复方案及验证计划 |
| 凭证与敏感数据 | 不打印、不写入仓库、不通过命令行拼接泄露 |
| 外部消息/单据 | 修改状态、评论或创建资源前确认流程授权 |

### 3.2 默认可靠性策略

- 普通命令默认 30 秒超时；构建、测试、图谱生成等长任务按预期时长设置。
- 只对可识别的临时错误重试；最大 3 次并采用指数退避。
- 创建和发布类操作优先使用服务端幂等机制；没有幂等保证时先读取当前状态。
- 工具返回值必须经过目标状态验证，退出码为 0 不等于业务结果正确。

## 4. 新工具接入

1. 明确单一职责、输入输出 schema、权限和失败语义。
2. 提供只读探测接口、最小示例和幂等/重试说明。
3. 在治理仓 `tool-dependencies.md` 登记依赖方、接口、必需性和场景。
4. 增加 Skill 或 Agent 文档及测试/评估资产。
5. 重新生成 Harness 并验证 Skill、MCP、CLI、配置四类依赖。

## 检查清单

- [ ] 使用的 Skill 已在权威依赖清单登记
- [ ] MCP 参数来自实时 schema，业务约束来自对应 Skill
- [ ] 必需 CLI 和配置已在执行前探测
- [ ] 外部写操作具备授权、幂等和写后验证
- [ ] 未执行破坏性 Git、生产或凭证操作
- [ ] 工具失败按错误类型处理，未盲目重试
