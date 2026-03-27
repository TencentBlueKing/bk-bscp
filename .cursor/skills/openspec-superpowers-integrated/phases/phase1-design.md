# Phase 0 + 1 + 1.5 + 2 详细执行规范

## Phase 0：需求输入

**本阶段负责确定需求来源并收集需求信息，为 Phase 1 设计讨论提供上下文。**

### 输入检测

根据用户消息自动判断输入模式：

| 输入特征 | 模式 |
|----------|------|
| 包含 `tapd.cn` 的 URL | **TAPD 模式** — 从 URL 解析类型和 ID |
| 长数字字符串（以 `10` 或 `11` 开头） | **TAPD 模式** — 尝试按 ID 查询 |
| 包含需求描述或 kebab-case 名称 | **直接描述模式** |
| 无实质输入 | **询问模式** — 让用户选择 |

**询问模式**：使用 `AskQuestion` 工具：

```
问题: 你希望如何提供需求？
选项:
  - "直接描述需求" (describe)
  - "从 TAPD 获取单据" (tapd)
```

### 直接描述模式

直接进入 Phase 1，将用户描述作为 brainstorming 的输入上下文。

### TAPD 模式

遵循 `tapd-manager` 技能的 MCP 调用流程，核心步骤：

**Step 1 — MCP 探活**

调用 `lookup_tapd_tool`（server: `user-tapd_mcp_http`, toolName: `lookup_tapd_tool`）：
```json
{ "task_description": "查询需求" }
```

| 结果 | 处理 |
|------|------|
| 成功 | 继续 |
| `MCP server does not exist` | 输出安装引导（见 `tapd-manager` 技能），终止流程 |
| 权限/网络错误 | 按 `tapd-manager` 错误处理，终止或重试 |

**Step 2 — 解析用户输入**

TAPD URL 模式：
- `https://www.tapd.cn/<workspace_id>/prong/stories/view/<id>` → type=`stories`
- `https://www.tapd.cn/<workspace_id>/bugtrace/bugs/view?bug_id=<id>` → type=`bugs`
- `https://www.tapd.cn/<workspace_id>/prong/tasks/view/<id>` → type=`tasks`

纯数字 ID：先尝试 `stories_get`，失败再依次 `bugs_get` / `tasks_get`。

若用户未提供 ID，询问："请输入 TAPD 单据链接或 ID"。

**Step 3 — 获取单据详情**

按 `tapd-manager` 核心调用流程（`lookup_tool_param_schema` → `proxy_execute_tool`）：
- 工具：`stories_get` / `bugs_get` / `tasks_get`
- 参数：`workspace_id=<YOUR_WORKSPACE_ID>`，`id=<extracted_id>`

提取关键字段：

| 字段 | 用途 |
|------|------|
| `name` / `title` | 需求标题 → 推导 change name |
| `description` | 需求描述 → Phase 1 brainstorming 上下文 + Phase 2 proposal |
| `priority` / `priority_label` | 优先级 → Phase 2 tasks 排列 |
| `status` | 当前状态 → 上下文参考 |
| `owner` | 处理人 → 上下文参考 |
| `category_name` | 分类 → 上下文参考 |
| `begin` / `due` | 时间约束 → Phase 2 tasks 规划 |

**Step 4 — 展示并确认**

```
从 TAPD 获取到以下需求信息：

- 标题：xxx
- 类型：需求/缺陷/任务
- 优先级：High
- 状态：xxx
- 处理人：xxx
- 描述：
  (展示描述内容摘要)

将基于此信息进入设计讨论。
```

使用 `AskQuestion` 提供选项：`确认继续 | 补充说明 | 取消`
- **确认继续** → 进入 Phase 1
- **补充说明** → 用户追加上下文，合并后进入 Phase 1
- **取消** → 终止流程

**Step 5 — 构造需求上下文**

将获取的信息构造为 `tapd_context`，在后续阶段中引用：

```yaml
tapd_context:
  source: "TAPD"
  type: stories/bugs/tasks
  id: "<tapd_item_id>"
  url: "https://www.tapd.cn/<workspace_id>/prong/stories/view/..."
  title: "xxx"
  description: "..."
  priority: "High"
  owner: "xxx"
  status: "xxx"
  time_constraint: { begin: "YYYY-MM-DD", due: "YYYY-MM-DD" }
  user_supplement: "（用户补充的说明，如有）"
```

**Step 6 — 推导 change name（预推导，Phase 2 使用）**

从 TAPD 标题推导 kebab-case 名称：
- "支持环境变量批量导入" → `env-variable-batch-import`
- "修复成员权限校验失败" → `fix-member-permission-check`

### Phase 0 退出检查

进入 Phase 1 前确认：
- [x] 已获取明确的需求信息（描述或 TAPD 单据）
- [x] 用户已确认需求信息（TAPD 模式下）
- [x] 本阶段未创建任何文件（⛔ 继承 Phase 1 的文件禁令）

---

## Phase 1：设计讨论

**本阶段可在任意目录执行**，无需 worktree。

### 第一步（强制）：调用 brainstorming

必须使用 Skill 工具 / Read 工具加载 `superpowers:brainstorming` 技能并按其指引执行。
禁止在未调用该技能的情况下直接输出设计方案。

**brainstorming 不可用时**：输出提示要求用户检查 superpowers 插件安装状态，禁止跳过设计讨论。

### brainstorming 覆盖规则

brainstorming 技能的 **"After the Design"** 部分在本集成技能中 **全部被禁止**，包括：
- 写入 `docs/plans/`
- 调用 `writing-plans`
- 调用 `using-git-worktrees`（由 Phase 1.5 统一处理）

设计讨论完成后，直接进入下方「设计确认」流程。

**双重防护**：brainstorming 完成设计呈现后，如果输出包含 `docs/plans/`、`writing-plans`、`Ready to set up for implementation?` 等关键词，必须忽略并输出：
> "brainstorming 建议的后续步骤已被 OpenSpec 集成技能覆盖。现在进入设计确认流程。"

### 执行 brainstorming 的过程

按 brainstorming 技能指引完成：

1. **探索项目上下文** — 读取现有文件、检查 `openspec/specs/` 和 `openspec/changes/`
   - **若 Phase 0 产出了 `tapd_context`**：将 TAPD 标题和描述作为初始上下文呈现给 brainstorming，减少重复提问。brainstorming 的"探索项目上下文"步骤应同时包含 TAPD 需求信息。
2. **逐一提问** — 了解目的、约束、成功标准（每次只问一个问题）
   - **TAPD 模式下**：TAPD 描述中已包含的信息无需重复询问，聚焦于 TAPD 未覆盖的技术设计问题
3. **提出 2-3 个方案** — 附权衡分析和推荐理由
4. **分节呈现设计** — 每节后获取用户确认

### 设计确认

**Phase 1 完成的唯一标准**：用户对完整设计方案给出明确的整体确认。

明确确认词：`确认`、`ok`、`yes`、`同意`、`好的`、`开始生成`、`可以`、`没问题`
明确拒绝词：`不`、`不行`、`重新设计`、`修改`、`再想想`

模糊回复（如「嗯」「差不多了」「先这样吧」）→ 追问：
> "请确认是否可以开始生成 OpenSpec 文档？（请回复'确认'或'需要修改'）"

**确认询问的强制格式**：只能提供「生成文档」选项，禁止在询问中出现「直接实施代码」等选项。

### Phase 1 退出检查

进入 Phase 1.5 前确认以下全部为真：
- [x] 已调用 brainstorming 技能
- [x] 用户已对完整设计方案给出明确确认
- [x] 本阶段未创建任何文件
- [x] 未调用 writing-plans

---

## Phase 1.5：环境检查

Phase 1 完成后自动执行，不需要用户手动触发。

**配置 `skip_worktree_check: true` 时**：跳过，直接进入 Phase 2。

### 检测逻辑

```bash
git rev-parse --git-dir
```

**情况 A：输出包含 `.git/worktrees/`**
→ 当前已在 worktree 中，直接进入 Phase 2。

**情况 B：输出为 `.git`（在主仓库）**
→ 输出以下选项后停止等待：

```
当前在主仓库目录，建议在功能分支的 worktree 中生成 OpenSpec 文档。

请选择：
1. 创建新 worktree（推荐）— 调用 superpowers:using-git-worktrees 创建功能分支
2. 在当前目录继续（不推荐）— 直接生成文档，需自行管理分支
3. 取消操作
```

| 用户选择 | 行为 |
|---------|------|
| 1 | 调用 `superpowers:using-git-worktrees` 创建 worktree，就绪后进入 Phase 2 |
| 2 | 在当前目录继续，进入 Phase 2 |
| 3 | 终止流程 |

**情况 C：不是 Git 仓库**
→ 提示用户先 `git init`，终止流程。

---

## Phase 2：文档生成

**必须在 Phase 1.5 完成后才能执行。**

### 强制前置步骤：加载模板

**生成任何文件前，必须**：
1. `read_file` 读取 `${SKILL_BASE}/templates/tasks-template.md`
2. 内部确认模板要素已就绪后才开始生成
3. **禁止凭记忆生成 tasks.md**

### 生成目录结构

```
openspec/changes/<kebab-case-name>/
├── proposal.md                    ← 变更提案（Why + What + Impact）
├── specs/
│   ├── <capability>/spec.md       ← 功能需求规格（归档后同步到 openspec/specs/<capability>/spec.md）
│   └── <service>/spec.md          ← Delta Spec（修改现有服务 spec 时）
├── design.md                      ← 方案设计文档（归档后同步到 openspec/specs/<capability>/design.md）
└── tasks.md                       ← 实施任务清单
```

**归档目标说明**：
- `specs/<capability>/spec.md` → 归档时同步到 `openspec/specs/<capability>/spec.md`（功能需求的单一真相源）
- `design.md` → 归档时复制到 `openspec/specs/<capability>/design.md`（方案设计的单一真相源）
- 归档时还会根据变更内容同步更新 `docs/services/` 下对应的服务设计文档

### proposal.md 格式

```markdown
## Why
<!-- 要解决的问题 -->
<!-- TAPD 模式：引用 TAPD 单据链接作为需求来源 -->

## What Changes
<!-- 新增的能力和修改 -->

## Impact
<!-- 受影响的代码、API、依赖 -->

## Source
<!-- 可选：仅 TAPD 模式填写 -->
<!-- TAPD #ID: <链接> -->
```

**TAPD 模式下**：`Why` 部分应引用 TAPD 需求描述，`Source` 部分填写 TAPD 链接。

### specs/<feature>.md 格式（功能需求规格）

```markdown
## ADDED Requirements

### Requirement: <requirement-name>
<!-- 需求描述 -->

#### Scenario: <scenario-name>
- **WHEN** <条件>
- **THEN** <期望结果>
```

### specs/<service>/spec.md 格式（Delta Spec）

当变更涉及**已有服务**的 spec 文档（即 `openspec/specs/<service>/spec.md` 已存在）时，
**必须**生成对应的 delta spec 文件，路径为 `specs/<service>/spec.md`。

**生成规则**：
1. 读取当前主 spec（`openspec/specs/<service>/spec.md`），识别需要变更的具体章节
2. 根据 Phase 1 确认的设计方案，生成 delta spec，**只包含变更部分**
3. 一个变更可能涉及多个服务，每个服务单独一个 delta spec

**格式**：

```markdown
# Delta: <service-name> Service Spec

## 变更摘要
<!-- 一句话描述本次对该服务 spec 的变更 -->

## 变更章节

### MODIFIED: <章节路径（如 "2.1 表结构"）>
<!-- 给出变更后的完整章节内容，用于替换主 spec 中的对应章节 -->

### ADDED: <新章节标题>
<!-- 需要新增到主 spec 的章节 -->
<!-- 插入位置：在 <某章节> 之后 -->

### REMOVED: <章节标题>
<!-- 需要从主 spec 中删除的章节（少见） -->
```

> **注意**：`MODIFIED`/`ADDED`/`REMOVED` 为结构标记，Phase 4 同步时依赖它们判断操作类型，必须保留英文。

**判断规则**：
- 变更涉及新增/修改数据库字段 → MODIFIED 表结构章节、索引章节
- 变更涉及新增/修改 API → MODIFIED Proto 定义章节、REST API 示例章节
- 变更涉及新增/修改 DTO → MODIFIED Go DTO 章节
- 变更涉及全新能力 → ADDED 对应章节

**禁止**：将整个主 spec 复制进 delta spec，只写变更部分。

### design.md 格式

```markdown
## Context
<!-- 背景和当前状态 -->

## Goals / Non-Goals
**Goals:**
<!-- 本次设计要达成的目标 -->

**Non-Goals:**
<!-- 明确不在范围内的事项 -->

## Decisions
<!-- 关键设计决策和理由 -->

## Risks / Trade-offs
<!-- 已知风险 -->
```

### tasks.md 格式

**必须按 `${SKILL_BASE}/templates/tasks-template.md` 模板生成。**

详细模板规则见该文件，此处不重复。

### 完成输出

```
OpenSpec 文档已在分支 feature/<branch-name> 生成。
请审阅 openspec/changes/<name>/ 下的文件，然后运行 /opsx:apply 开始实施。
实施完成后合并分支到 master 并运行 /opsx:archive 归档。

归档时将自动执行：
1. 同步 spec.md + design.md 到 openspec/specs/<capability>/
2. 根据变更内容更新 docs/services/ 下对应的服务设计文档
```

**⛔ 禁止**：调用 writing-plans、写入 docs/plans/、调用任何其他实施技能。
