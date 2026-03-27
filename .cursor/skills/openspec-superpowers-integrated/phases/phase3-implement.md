# Phase 3 详细执行规范

## 前置检查

所有检查必须通过才能开始执行。任何一项未通过则停止并说明原因。

1. **分支检查**：`git branch --show-current` 确认为 `feature/<name>`（非 master/main）
   - 配置 `skip_worktree_check: true` 时跳过
2. **worktree 检查**：`git worktree list` 确认当前路径是 worktree
   - 配置 `skip_worktree_check: true` 时跳过
3. **tasks.md 存在**：确认 `openspec/changes/<name>/tasks.md` 文件存在
4. **读取配置**：会话上下文中的配置值优先，否则使用 SKILL.md 默认值
5. **读取 tasks.md，生成 TODO 列表**：分析所有顶层任务，识别类型（TDD/非 TDD），包含代码审查任务和变更文档定稿
6. **中断恢复检测**：若有 `[x]` 标记的任务，输出恢复提示，从首个未完成任务继续

---

## execution_mode 推荐

前置检查通过后，给出推荐并停止等待用户选择：

- 顶层实现任务 **≤ 5 个** 且逻辑连续 → 推荐 **agent**
- 顶层实现任务 **> 5 个** 或涉及多个独立模块 → 推荐 **subagent**

**必须按以下格式输出后停止**：

```
根据本次任务规模（共 X 个顶层任务），推荐执行模式：

1. agent（推荐）— 主代理直接执行，上下文连续
2. subagent — 每任务派独立子代理+审查，适合任务多或模块独立的场景

本次将执行以下任务：
- [ ] 1.1 <任务描述>（TDD）
- [ ] 1.2 <任务描述>（非 TDD）
- [ ] 1.3 代码审查
- [ ] ...
- [ ] N-1. 变更文档定稿

请选择执行模式（输入 1 或 2）：
```

**禁止在用户选择前继续执行任何任务。**

---

## 任务执行流程（agent 模式）

### 1. 判断任务类型

读取 tasks.md 中的注释标注（`<!-- TDD 任务 -->` / `<!-- 非 TDD 任务 -->`）。
未标注时：新功能/Bug 修复/复杂逻辑 → TDD；配置修改/重命名/文档更新 → 非 TDD。

### 2. TDD 任务执行（5 步）

严格遵循 `superpowers:test-driven-development` 的 Red-Green-Refactor 原则：

| 子步骤 | 动作 | 验证 |
|--------|------|------|
| N.M.1 | 写失败测试 | 测试文件存在 |
| N.M.2 | 验证测试失败 | 运行测试命令，确认失败原因是缺少功能（非语法错误） |
| N.M.3 | 写最小实现 | 实现文件存在 |
| N.M.4 | 验证测试通过 | 运行测试命令，确认所有测试通过，输出干净 |
| N.M.5 | 重构 | 整理代码，保持测试通过 |

**铁律**：N.M.2 必须在 N.M.3 之前完成。没有失败测试，不写实现代码。

### 3. 非 TDD 任务执行（3 步）

| 子步骤 | 动作 | 验证 |
|--------|------|------|
| N.M.1 | 执行变更 | 变更文件存在 |
| N.M.2 | 验证无回归 | 运行测试/构建命令，确认输出干净 |
| N.M.3 | 检查完整性 | 确认变更范围完整，无遗漏 |

### 4. 调试规范

**N.M.2（验证失败/验证无回归）或 N.M.4（验证通过）出现意外结果时**：

必须立即调用 `superpowers:systematic-debugging` 进行根因分析。

**禁止**：跳过调试直接猜测修复。

systematic-debugging 要求：
1. 读错误信息 → 2. 稳定复现 → 3. 检查近期变更 → 4. 追踪数据流 → 5. 假设检验 → 6. 修复

### 5. 更新 tasks.md

**每个子步骤完成后立即标记 `[x]`**：先标记子任务（如 `3.1.1`），再在所有子任务完成后标记顶层任务（如 `3.1`）。

**标记顺序**：子任务 `[x]` → 顶层任务 `[x]`（禁止只标记顶层而跳过子任务）。

更新时 `old_str` 选取任务编号行完整文本（编号保证唯一性），`new_str` 仅将 `[ ]` 替换为 `[x]`，禁止修改任务描述。

---

## 代码审查

每个任务组结尾执行代码审查（`review_mode: per_task`）或延迟至统一审查（`review_mode: final`）。

### 审查执行步骤

1. **前置验证**：调用 `superpowers:verification-before-completion`
   - 运行全量测试，确认所有测试通过且输出干净
   - **没有通过验证不允许开始审查**

2. **执行审查**：调用 `superpowers:requesting-code-review`
   - 占位符映射：
     - `{PLAN_OR_REQUIREMENTS}` → `openspec/changes/<name>/specs/*.md` + `tasks.md`
     - `{WHAT_WAS_IMPLEMENTED}` → 本任务组所有变更文件
     - `{BASE_SHA}` → 任务组开始前的 commit SHA
     - `{HEAD_SHA}` → 当前 HEAD

3. **处理审查结果**：
   - 仅 Minor 或无问题 → 自动继续下一任务组
   - 存在 Critical / Important → 输出结果 + 选项，停等用户

### 审查选项（Critical/Important 时）

```
请选择操作：
a. 处理指定条目 — 输入 `处理 1,2` 修复指定编号
b. 全部处理 — 修复所有 Critical 和 Important
c. 跳过 — 不修改代码，继续下一任务
d. 跳过并备注 — 输入 `跳过，备注：<原因>` 记录原因
```

用户选择「处理」后 → 调用 `superpowers:receiving-code-review` 对每条意见做技术验证后再实施。

### review_mode: final 差异

任务组结尾的审查标记为 `[DEFERRED]`，所有实现完成后统一执行一次代码审查（scope = 全部变更）。

---

## subagent 模式

主代理为每个顶层任务派发独立子代理。派发时 prompt 必须包含：

```
任务：{任务完整文本，来自 tasks.md}
任务类型：{TDD / 非 TDD}
需求规格：{openspec/changes/<name>/specs/*.md 相关内容}
架构决策：{openspec/changes/<name>/design.md 的 Decisions 节}
工作目录：{worktree 绝对路径}

执行规范：
- TDD 任务严格按 N.M.1→N.M.5 执行
- 非 TDD 任务按 N.M.1→N.M.3 执行
- 意外结果时调用 superpowers:systematic-debugging
- 禁止跳过任何子任务步骤
- 完成后输出：变更文件列表、测试结果、是否有偏差
```

**子代理内置审查与 tasks.md 审查任务的关系**：
- 子代理内部审查 = 内部质量门控，结果不输出给用户
- tasks.md 审查任务 = 面向用户的可见审查，由主代理执行
- 两者不重复，subagent 模式下主代理仍须执行 tasks.md 中的审查任务

---

## 变更文档定稿（最后一组任务）

**作用域**：仅更新 `openspec/changes/<change-id>/` 下的文档，确保变更级别的文档与实际实现一致。
**不涉及**：主 specs（`openspec/specs/`）的同步——由 Phase 4（`/opsx:archive`）负责。

| 子任务 | 动作 |
|--------|------|
| N.1 | 定稿 design.md：记录技术决策、偏差和实现细节 |
| N.2 | 定稿 tasks.md：逐一扫描**所有层级**任务（顶层 + 子任务），将已完成但仍为 `[ ]` 的标记为 `[x]` |
| N.3 | 定稿 proposal.md：更新范围/影响（若有变化） |
| N.4 | 定稿 specs/*.md：更新功能需求（若有变化）+ 验证 delta specs 与实际实现一致 |
| N.5 | 最终校验：确保所有变更文档反映实际实现 |

**N.2 必须全量标记**：逐一扫描每个任务和子任务，不得遗漏。

---

## 变更文档更新规则

实施过程中，代码变更后立即更新以下变更级别文档（无需用户提醒）：

| 文档 | 更新时机 | 作用域 |
|------|----------|--------|
| tasks.md | 任务完成时标记 `[x]` | `openspec/changes/<change-id>/` |
| design.md | 每次代码变更后记录技术决策和实现细节 | `openspec/changes/<change-id>/` |
| proposal.md | 变更范围或影响发生变化时 | `openspec/changes/<change-id>/` |
| specs/*.md | 需求发生变化时（含 delta specs 与实现的一致性校验） | `openspec/changes/<change-id>/` |

**只更新 `openspec/changes/<change-id>/`，不更新 `openspec/specs/`（主文档）或 `archive/`。**
主文档同步由 Phase 4（`/opsx:archive`）在归档时执行。

---

## 实施完成

所有 checkbox 为 `[x]` 后，**必须**调用 `superpowers:finishing-a-development-branch`：

1. 验证测试全部通过
2. 确定基础分支
3. 展示 4 个选项（合并 / 创建 PR / 保留 / 丢弃）
4. 执行用户选择
5. 清理 worktree

**禁止**：直接提示「请手动 merge」，必须通过该技能引导。

---

## 禁止行为

- TDD 任务中，无失败测试时禁止写实现代码
- review_action: confirm 时，未收到用户指令禁止修改代码
- 禁止修改 tasks.md 已确认的任务结构（仅可更新 checkbox）
- 禁止将审查结果写入 tasks.md 或任何 OpenSpec 文档
- 禁止生成 `docs/plans/` 或调用 writing-plans
- 禁止在 `docs/` 根目录下直接创建文件

---

## 回滚

用户触发 `/opsx:rollback`（由 SKILL.md 路由到此）或在 Phase 3 执行中说"回滚"时执行。

### Step 1：状态诊断

并行执行以下命令，收集当前状态：

```bash
git diff --stat                              # 未暂存变更
git diff --cached --stat                     # 已暂存变更
git log --oneline -10                        # 近 10 条提交
git log --oneline HEAD...$(git merge-base HEAD master)  # 当前分支的所有新提交
```

读取 tasks.md，统计：
- 已完成任务（`[x]`）数量和列表
- 未完成任务（`[ ]`）数量
- 当前正在执行的任务（最后一个 `[x]` 的下一个 `[ ]`）

### Step 2：展示状态并选择回滚粒度

**必须展示完整状态后才提供选项，禁止跳过展示直接回滚。**

```
## 当前实施状态

**分支：** feature/<name>
**未提交变更：** N 个文件
**已暂存变更：** M 个文件
**分支新提交：** K 个 commit

**任务进度：** X/Y 完成
- [x] 1.1 <描述>
- [x] 1.2 <描述>
- [ ] 1.3 <描述>  ← 当前任务
- [ ] 2.1 <描述>
...

请选择回滚范围：
a. 回滚当前任务 — 撤销当前正在执行的任务的变更
b. 回滚最近任务组 — 撤销最近一个任务组的所有变更
c. 回滚到指定任务 — 输入任务编号，回滚该任务及之后的所有变更
d. 回滚全部实施 — 撤销分支上所有实施变更，回到 Phase 2 结束状态
e. 取消
```

### Step 3：执行回滚

#### a. 回滚当前任务

针对当前正在执行但未完成的任务：

1. **识别变更范围**：通过 `git diff --stat` 找出当前任务修改的文件
2. **未提交变更**：
   ```bash
   git checkout -- <文件列表>
   ```
3. **已暂存变更**：
   ```bash
   git reset HEAD <文件列表>
   git checkout -- <文件列表>
   ```
4. **更新 tasks.md**：将当前任务的已完成子步骤重置为 `[ ]`

#### b. 回滚最近任务组

1. **识别任务组范围**：找到最近一个含 `[x]` 任务的任务组
2. **检查提交历史**：该任务组的变更是否已提交
   - **未提交** → 同上，`git checkout` 恢复
   - **已提交** → 使用 `git revert` 或 `git reset --soft`（见下方策略）
3. **更新 tasks.md**：将该任务组所有任务及子任务重置为 `[ ]`

#### c. 回滚到指定任务

1. 用户输入目标任务编号（如 `1.3`）
2. 找出该任务及之后所有已完成任务涉及的提交
3. 按 b 的策略批量回滚
4. 更新 tasks.md：将目标任务及之后所有任务重置为 `[ ]`

#### d. 回滚全部实施

1. 找到分支与 base 分支的分叉点：
   ```bash
   git merge-base HEAD master
   ```
2. **仅保留 Phase 2 产出的 OpenSpec 文档**：
   ```bash
   git diff --name-only <merge-base>..HEAD -- ':!openspec/changes/'
   ```
3. 展示将要回滚的文件列表（排除 openspec/changes/ 下的文件）
4. 用户二次确认后执行：
   ```bash
   git checkout <merge-base> -- <实施文件列表>
   ```
5. 更新 tasks.md：将所有实现任务和代码审查任务重置为 `[ ]`（保留变更文档定稿的状态）

### 已提交变更的回滚策略

| 场景 | 策略 | 命令 |
|------|------|------|
| 变更未推送到远程 | `git reset --soft` 保留暂存 + 手动撤销 | `git reset --soft <target>` |
| 变更已推送到远程 | `git revert` 生成反向提交 | `git revert <commit-range> --no-edit` |
| 无法确定 | 询问用户 | — |

**判断是否已推送**：
```bash
git log --oneline origin/$(git branch --show-current)..HEAD 2>/dev/null
```
- 有输出 → 有未推送提交 → 可用 `reset`
- 无输出或报错 → 已推送或无远程 → 用 `revert`

### Step 4：回滚后验证

1. **运行测试**（如有）：确认回滚后项目状态干净
2. **检查 tasks.md**：确认 checkbox 状态与代码状态一致
3. **输出摘要**：
   ```
   ## 回滚完成

   **回滚范围：** <选择的粒度>
   **撤销文件：** N 个
   **撤销提交：** K 个（或"无，仅撤销未提交变更"）
   **tasks.md 已重置：** M 个任务

   可以继续执行 /opsx:apply 重新实施。
   ```
