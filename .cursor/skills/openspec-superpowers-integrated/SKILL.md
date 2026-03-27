---
name: openspec-superpowers
description: 当用户执行 /opsx:propose（或 /opsx-propose）、/opsx:apply（或 /opsx-apply）、/opsx:explore（或 /opsx-explore）、/opsx:archive（或 /opsx-archive）时触发。将 OpenSpec 文档管理与 Superpowers 质量工程能力集成为统一开发流程。
---

# OpenSpec + Superpowers 集成技能

## 核心理念

- **OpenSpec** 管理「做什么」— 规范管理、变更追踪、文档归档
- **Superpowers** 管理「怎么高质量地做」— 设计讨论、TDD、代码审查、系统调试
- **本技能** 是桥梁 — 在 OpenSpec 的每个阶段自动调用对应的 Superpowers 技能

## 技能基础路径

本技能所有文件引用使用相对路径。执行时先确定 `SKILL_BASE`：
1. 当前已加载 SKILL.md 所在目录（首选）
2. 兜底：搜索 `openspec-superpowers/SKILL.md`

确定后，所有 `read_file` 使用 `${SKILL_BASE}/<相对路径>`。

## 入口路由

根据用户消息中的命令关键字路由到对应阶段（同时支持冒号 `:` 和连字符 `-` 两种格式，以兼容 Cursor command 命名）：

| 用户消息包含 | 执行路径 | 阶段 |
|-------------|---------|------|
| `opsx:propose` 或 `opsx-propose` 或 `openspec:proposal` | Phase 0 → 1 → 1.5 → 2 | 需求输入 → 设计 → 环境检查 → 文档生成 |
| `opsx:apply` 或 `opsx-apply` 或 `openspec:apply` | Phase 3 | 实施 |
| `opsx:explore` 或 `opsx-explore` | 原生 OpenSpec explore | 探索（无需集成） |
| `opsx:archive` 或 `opsx-archive` | Phase 4 | Delta Spec 同步 → 归档 |
| `opsx:rollback` 或 `opsx-rollback` | 上下文检测 → Phase 3 或 Phase 4 回滚 | 回滚 |
| 无法判断 | 询问用户 | — |

### propose 路径

**【必须】先 `read_file` 读取 `${SKILL_BASE}/phases/phase1-design.md`，严格按该文件执行 Phase 0 → 1 → 1.5 → 2。**

### apply 路径

**【必须】先 `read_file` 读取 `${SKILL_BASE}/phases/phase3-implement.md`，严格按该文件执行 Phase 3。**

### archive 路径

**【必须】先 `read_file` 读取 `${SKILL_BASE}/phases/phase4-archive.md`，严格按该文件执行 Phase 4。**

### rollback 路径

**上下文检测**：根据当前状态自动判断回滚目标阶段。

检测逻辑：
1. 检查 `openspec/changes/archive/` 下是否有**今天**归档的变更 → 有则提供 Phase 4 回滚选项
2. 检查 `openspec/changes/` 下是否有活跃变更的 `tasks.md` 含 `[x]` 标记 → 有则提供 Phase 3 回滚选项
3. 两者都有 → 用 `AskQuestion` 让用户选择回滚哪个阶段
4. 都没有 → 提示无可回滚内容

确定目标后：
- Phase 3 回滚 → 读取 `${SKILL_BASE}/phases/phase3-implement.md` 的「回滚」节执行
- Phase 4 回滚 → 读取 `${SKILL_BASE}/phases/phase4-archive.md` 的「回滚」节执行

### explore / verify

直接使用 OpenSpec 原生命令处理，不需要额外的 Superpowers 集成。

---

## 完整流程全景

```
用户：我要做一个新功能 / 提供 TAPD 链接

Phase 0：需求输入
  ├── 自动检测输入（TAPD URL/ID vs 描述 vs 空）
  ├── 无输入 → AskQuestion 让用户选择「直接描述」或「从 TAPD 获取」
  ├── TAPD 模式：通过 tapd-manager MCP 获取单据详情
  ├── 展示并确认需求信息
  └── 携带需求上下文（tapd_context 或 user_description）进入 Phase 1

Phase 1：设计讨论
  ├── 调用 superpowers:brainstorming 进行设计探索（携带 Phase 0 上下文）
  ├── 逐一提问 → 提出 2-3 个方案 → 分节呈现设计
  ├── 用户确认完整设计
  └── ⛔ 此阶段禁止：创建任何文件、调用 writing-plans

Phase 1.5：环境检查
  ├── 检测是否在 Git worktree 中
  ├── 不在 → 选项：创建 worktree / 当前目录继续 / 取消
  └── 就绪 → 进入 Phase 2

Phase 2：文档生成
  ├── 重新读取 tasks-template.md 模板（强制，防止遗忘）
  ├── 生成 openspec/changes/<name>/ 目录：
  │   ├── proposal.md    （Why + What Changes + Impact）
  │   ├── specs/*.md      （功能需求规格 + Delta Specs → 归档后同步到 openspec/specs/）
  │   ├── design.md       （Context + Goals + Decisions + Risks → 归档后同步到 openspec/specs/）
  │   └── tasks.md        （TDD/非TDD 子任务 + 代码审查 + 变更文档定稿）
  └── ⛔ 禁止：调用 writing-plans、写入 docs/plans/

用户：/opsx:apply

Phase 3：实施
  ├── 前置检查（分支、worktree、tasks.md 存在性）
  ├── 读取 tasks.md → 生成 TODO 列表 → 选择执行模式
  ├── 逐任务执行：
  │   ├── TDD 任务：写失败测试 → 验证失败 → 最小实现 → 验证通过 → 重构
  │   │   └── 意外失败 → 调用 superpowers:systematic-debugging
  │   ├── 非 TDD 任务：执行变更 → 验证无回归 → 检查完整性
  │   └── 任务组代码审查：
  │       ├── 先调用 superpowers:verification-before-completion 确认测试通过
  │       ├── 再调用 superpowers:requesting-code-review 执行审查
  │       └── 用户选择处理 → 调用 superpowers:receiving-code-review 验证后实施
  ├── 变更文档定稿（校验变更级别文档与实际实现一致）
  └── 调用 superpowers:finishing-a-development-branch 引导分支决策

用户：/opsx:archive

Phase 4：归档
  ├── 选择变更（自动检测或询问）
  ├── 完整性检查（任务完成度、必要文件）
  ├── 功能文档同步（openspec/specs/）：
  │   ├── 扫描 openspec/changes/<name>/specs/ 下的 delta specs
  │   ├── 分析类型（新建 / 追加 / 替换 / 删除）
  │   ├── 同步 design.md 到 openspec/specs/<capability>/design.md
  │   ├── 生成同步预览 → 用户确认
  │   └── 执行同步 spec.md + design.md 到 openspec/specs/
  ├── 服务设计文档更新（docs/services/）：
  │   ├── 识别受影响的服务（从代码变更和设计文档推导）
  │   ├── 确定更新内容（追加/修改章节）
  │   ├── 生成更新预览 → 用户确认
  │   └── 执行更新到 docs/services/<service>.md
  ├── 归档：mv openspec/changes/<name> → archive/YYYY-MM-DD-<name>
  └── 输出归档摘要
```

---

## Superpowers 技能调用矩阵

| 阶段 | Superpowers 技能 | 调用时机 |
|------|-----------------|---------|
| Phase 0 | `tapd-manager` | TAPD 模式时获取单据（条件触发） |
| Phase 1 | `brainstorming` | 设计讨论开始时（强制） |
| Phase 1.5 | `using-git-worktrees` | 需要创建 worktree 时（可选） |
| Phase 3 | `test-driven-development` | 执行 TDD 任务时（隐式遵循） |
| Phase 3 | `systematic-debugging` | 测试结果与预期不符时（强制） |
| Phase 3 | `verification-before-completion` | 代码审查前（强制） |
| Phase 3 | `requesting-code-review` | 每个任务组完成后（强制） |
| Phase 3 | `receiving-code-review` | 用户选择处理审查意见后（强制） |
| Phase 3 | `finishing-a-development-branch` | 所有任务完成后（强制） |
| Phase 4 | — | 不依赖 Superpowers 技能，独立执行 delta spec 同步与归档 |

---

## HARD STOPS

违反以下任何规则必须立即停止并向用户报错：

| ID | Phase | 规则 |
|----|-------|------|
| H1 | 1 | 禁止在 Phase 1 创建任何文件 |
| H2 | 1 | 禁止跳过 brainstorming 技能调用 |
| H3 | 1 | 禁止在用户明确确认完整设计前进入 Phase 1.5 |
| H4 | 2 | 禁止调用 writing-plans 技能 |
| H5 | 2 | 禁止在 docs/plans/ 下创建文件 |
| H6 | 2 | 禁止凭记忆生成 tasks.md，必须先重新读取模板 |
| H7 | 3 | 禁止跳过 execution_mode 询问 |
| H8 | 3 | 禁止跳过任务执行子步骤 |
| H9 | 3 | 禁止跳过代码审查 |
| H10 | 3 | 禁止在测试结果异常时跳过 systematic-debugging |
| H11 | 3 | 禁止在完成后跳过 finishing-a-development-branch |
| H12 | 4 | 禁止在用户未确认同步预览前修改主 specs |
| H13 | 4 | 禁止在主 spec 中留下 TBD/TODO 占位符 |
| H14 | 4 | 禁止在主 spec 中残留 ADDED/MODIFIED/REMOVED 标记 |
| H17 | 4 | 禁止在用户未确认更新预览前修改 docs/services/ 下的文件 |
| H18 | 4 | 禁止在 docs/services/ 更新时修改变更范围之外的内容 |
| H15 | 回滚 | 禁止在未展示状态诊断和用户选择前执行任何回滚操作 |
| H16 | 回滚 | 禁止对已推送到远程的提交使用 `git reset`（必须用 `git revert`） |

---

## 配置

```yaml
execution_mode: agent      # agent | subagent（见下方选择指南）
review_mode: per_task      # per_task（每任务组审查）| final（统一审查）
review_action: confirm     # confirm（等用户指令）| auto（自动修复 Critical）
skip_worktree_check: false # true 时跳过 worktree 检查
tdd_strict: true           # false 时允许非 TDD 任务跳过验证
```

### execution_mode 选择指南

| 模式 | 适用场景 | 特点 |
|------|---------|------|
| `agent` | 任务 ≤5 个或逻辑简单、任务间有强依赖 | 主代理顺序执行，上下文连续 |
| `subagent` | 任务多（>5）或功能复杂、任务组内可并行 | 每任务派独立子代理，并行执行同组独立任务 |

Phase 3 启动时会根据 tasks.md 中的任务数量，向用户询问执行模式。

### 跳过已完成任务

Phase 3 执行前会检查每个任务的前置条件。如果目标文件已满足要求（如 proto 文件已有完整注释），应直接标记为已完成并跳过，避免重复工作。

项目可通过 alwaysApply 规则文件覆盖以上默认值。
执行时，会话上下文中的配置值优先于默认值。

---

## 阶段间数据契约

| 数据 | 产生阶段 | 消费阶段 | 传递方式 |
|------|---------|---------|---------|
| 需求上下文（tapd_context / user_description） | Phase 0 | Phase 1, 2 | 会话上下文传递；TAPD 模式下包含 source、type、id、url、title、description、priority、owner、time_constraint |
| 设计方案 | Phase 1 | Phase 2 | 写入 design.md 的 Context 和 Decisions 节 |
| worktree 路径 | Phase 1.5 | Phase 2, 3 | `pwd` 动态获取 |
| change name | Phase 0/2 | Phase 2, 3, 4 | Phase 0 从用户描述或 TAPD 标题推导；Phase 2 用于创建目录；Phase 4 用于定位变更 |
| tasks.md | Phase 2 | Phase 3, 4 | `read_file` 读取 |
| delta specs | Phase 2 | Phase 4 (Step 3) | `openspec/changes/<name>/specs/<capability>/spec.md` 文件 |
| design.md | Phase 2 | Phase 4 (Step 3, 4) | `openspec/changes/<name>/design.md` → 同步到 `openspec/specs/` 和 `docs/services/` |
| proposal.md | Phase 2 | Phase 4 (Step 3) | 新建主 spec 时提取 Purpose 描述 |

**跨会话恢复**：Phase 3 通过 `/opsx:apply` 独立触发时，前置检查覆盖必要的恢复步骤。Phase 4 通过 `/opsx:archive` 独立触发时，从文件系统重新读取所有上下文。AI 不应假设前置阶段的上下文仍在内存中。

---

## 上下文刷新检查点

长对话中 AI 可能遗忘早期指令。以下检查点处必须重新读取对应内容：

| 检查点 | 重新加载内容 |
|--------|------------|
| 每 5 个顶层任务完成后 | 本文件 HARD STOPS 表格 |
| 每次代码审查前 | phase3-implement.md 的「代码审查」节 |
| 每次派发子代理前 | phase3-implement.md 的「subagent 模式」节 |
| 变更文档定稿前 | phase3-implement.md 的「禁止行为」节 |
| 功能文档同步前 | phase4-archive.md 的「3.5 执行同步」节和「禁止行为」节 |
| 服务文档更新前 | phase4-archive.md 的「Step 4：同步更新 docs/services/」节和「禁止行为」节 |
