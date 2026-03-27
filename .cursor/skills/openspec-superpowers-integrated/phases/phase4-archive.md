# Phase 4 归档执行规范

## 概述

Phase 4 负责将已完成的变更归档，包括三个核心同步：
1. 同步功能文档（spec.md + design.md）到 `openspec/specs/`
2. 根据变更内容更新 `docs/services/` 下的服务设计文档
3. 移动变更目录到归档区

**触发方式**：用户执行 `opsx:archive` / `opsx-archive`。

---

## Step 1：选择变更

### 自动检测

```bash
ls openspec/changes/ | grep -v archive | grep -v README
```

| 情况 | 处理 |
|------|------|
| 用户指定了变更名 | 直接使用 |
| 仅一个活跃变更 | 自动选择，输出确认 |
| 多个活跃变更 | 使用 `AskQuestion` 让用户选择 |
| 无活跃变更 | 提示 "无活跃变更可归档"，终止 |

输出格式：
```
将归档变更：<change-name>
位置：openspec/changes/<change-name>/
```

---

## Step 2：完整性检查

### 2.1 读取 tasks.md

```bash
cat openspec/changes/<name>/tasks.md
```

统计 `- [ ]`（未完成）和 `- [x]`（已完成）数量。

| 结果 | 处理 |
|------|------|
| 全部完成 | 继续 |
| 存在未完成 | 显示警告 + 未完成任务列表，使用 `AskQuestion`：`继续归档 / 取消` |
| tasks.md 不存在 | 继续（不阻断） |

### 2.2 检查必要文件

确认以下文件存在（缺失不阻断，仅警告）：
- `openspec/changes/<name>/proposal.md`
- `openspec/changes/<name>/design.md`

---

## Step 3：功能文档同步到 openspec/specs/

**这是归档的核心步骤，将变更的功能文档（spec.md + design.md）同步到主 specs 目录。**

### 3.1 扫描变更文档

```bash
find openspec/changes/<name>/specs/ -name "spec.md" -type f
ls openspec/changes/<name>/design.md
```

| 结果 | 处理 |
|------|------|
| 无 delta specs 且无 design.md | 跳过同步，直接进入 Step 4 |
| 存在 delta specs 或 design.md | 逐个分析，继续 3.2 |

### 3.2 分析每个 delta spec

对每个 `openspec/changes/<name>/specs/<capability>/spec.md`：

1. **读取 delta spec 内容**
2. **检查主 spec 是否存在**：`openspec/specs/<capability>/spec.md`
3. **判断同步类型**：

| delta spec 特征 | 主 spec 存在？ | 同步类型 | 操作 |
|-----------------|---------------|---------|------|
| 任意内容 | 否 | **新建** | 将 delta spec 作为新主 spec |
| 包含 `## ADDED` | 是 | **追加** | 将 ADDED 章节追加到主 spec |
| 包含 `## MODIFIED` 或 `### MODIFIED:` | 是 | **替换** | 替换主 spec 中对应章节 |
| 包含 `### REMOVED:` | 是 | **删除** | 从主 spec 中移除对应章节 |
| 混合多种标记 | 是 | **混合** | 按标记类型逐一处理 |

### 3.3 分析 design.md 同步

对 `openspec/changes/<name>/design.md`：

1. **读取变更级别的 design.md**
2. **确定同步目标**：
   - 如果变更有多个 capability（即 `specs/` 下有多个子目录），将 design.md 复制到每个 capability 目录
   - 如果变更只有一个 capability，直接复制
   - **目标路径**：`openspec/specs/<capability>/design.md`
3. **判断操作类型**：

| design.md 目标 | 主 design 存在？ | 操作 |
|----------------|-----------------|------|
| `openspec/specs/<capability>/design.md` | 否 | **新建** — 直接复制 |
| `openspec/specs/<capability>/design.md` | 是 | **追加** — 将本次设计决策追加到现有 design.md 末尾，用分隔线区分 |

**追加格式**：
```markdown
---

## <change-name> (<YYYY-MM-DD>)

<本次 design.md 内容>
```

### 3.4 生成同步预览

**必须在执行任何变更前，向用户展示完整的同步预览。**

格式：
```
## 功能文档同步预览

### Spec 同步

#### <capability-1>（新建）
- 将在 openspec/specs/<capability-1>/ 创建新的 spec.md
- 内容来源：openspec/changes/<name>/specs/<capability-1>/spec.md

#### <capability-2>（合并）
- 目标：openspec/specs/<capability-2>/spec.md
- MODIFIED: <章节路径>（替换现有内容）
- ADDED: <新章节标题>（追加到 <位置>）

### Design 同步

#### <capability-1>（新建）
- 将在 openspec/specs/<capability-1>/ 创建 design.md
- 内容来源：openspec/changes/<name>/design.md

#### <capability-2>（追加）
- 目标：openspec/specs/<capability-2>/design.md
- 追加本次设计决策（标注日期和变更名）

确认执行同步？
a. 执行同步并继续（推荐）
b. 跳过同步，直接归档
c. 取消
```

**禁止在用户确认前执行任何同步操作。**

### 3.5 执行同步

用户选择「执行同步」后，按以下规则处理：

#### Spec 同步

##### 新建（主 spec 不存在）

```
openspec/specs/<capability>/spec.md
```

生成格式：
```markdown
# <capability> Specification

## Purpose
<从 delta spec 首段描述或 proposal.md 的 Why 部分提取>

## Requirements
<delta spec 中 ADDED Requirements 部分的内容，去掉 "ADDED" 前缀>
```

**规则**：
- 标题统一为 `# <capability> Specification`
- `## Purpose` 从 delta spec 开头描述或 proposal 推导，不得为 "TBD"
- `## Requirements` 下放置所有需求内容
- 如果 delta spec 已有合适的标题和结构，可直接采用（不强制改写）
- 创建目录：`mkdir -p openspec/specs/<capability>/`

##### 追加（ADDED 章节）

读取主 spec → 在末尾或指定位置插入新章节。

**规则**：
- 去掉 `ADDED` 前缀，如 `## ADDED Requirements` → `## Requirements`
- 如果主 spec 已有 `## Requirements` 节，在该节末尾追加新的 `### Requirement:` 子章节
- 如果 delta spec 中有 `<!-- 插入位置：在 <某章节> 之后 -->` 注释，按指定位置插入

##### 替换（MODIFIED 章节）

读取主 spec → 定位目标章节 → 用 delta 内容替换。

**规则**：
- `### MODIFIED: <章节路径>` 中的 `<章节路径>` 用于定位主 spec 中的章节
- 匹配策略：按章节标题的文本内容匹配（忽略标题级别差异）
- 找到匹配 → 用 delta 内容替换整个章节（到下一个同级标题之前）
- 未找到匹配 → 警告用户，让用户决定是追加还是跳过
- 替换内容去掉 `MODIFIED:` 前缀

##### 删除（REMOVED 章节）

读取主 spec → 定位并移除目标章节。

**规则**：
- `### REMOVED: <章节标题>` 指定要移除的章节
- 移除整个章节（到下一个同级标题之前）
- 未找到匹配 → 警告用户，继续处理其他操作

#### Design 同步

##### 新建（主 design 不存在）

```bash
cp openspec/changes/<name>/design.md openspec/specs/<capability>/design.md
```

##### 追加（主 design 已存在）

在现有 `openspec/specs/<capability>/design.md` 末尾追加，用分隔线和标题区分：

```markdown
---

## <change-name> (<YYYY-MM-DD>)

<变更级别 design.md 的完整内容>
```

### 3.6 同步后验证

每个 capability 同步完成后：
1. 读取更新后的主 spec，确认内容完整、格式正确
2. 确认没有残留的 `ADDED`/`MODIFIED`/`REMOVED` 标记
3. 确认 `design.md` 已正确同步
4. 输出同步结果：`✓ <capability>/spec.md 已同步` / `✓ <capability>/design.md 已同步`

---

## Step 4：同步更新 docs/services/ 设计文档

**归档时必须根据本次功能变更和方案设计，同步更新 `docs/services/` 下对应的服务设计文档。**

### 4.1 识别受影响的服务

从变更文档中提取受影响的服务：

1. **读取 `openspec/changes/<name>/design.md`**：从 Context、Decisions 节识别涉及的服务
2. **读取 `openspec/changes/<name>/proposal.md`**：从 Impact 节确认影响范围
3. **扫描代码变更**：`git diff --stat` 或从 tasks.md 提取变更文件路径，映射到服务

**服务映射表**：

| 代码路径特征 | 服务 | 设计文档 |
|-------------|------|----------|
| `internal/space/` 或 `cmd/space/` | 空间服务 | `docs/services/01-space-service.md` |
| `internal/member/` 或 `cmd/member/` | 成员服务 | `docs/services/02-member-service.md` |
| `internal/environment/` 或 `cmd/environment/` | 环境服务 | `docs/services/03-environment-service.md` |
| `internal/component/` 或 `cmd/component/` | 组件服务 | `docs/services/04-component-service.md` |
| `internal/dependency/` 或 `cmd/dependency/` | 依赖服务 | `docs/services/05-dependency-service.md` |
| `internal/task/` 或 `cmd/task/` | 任务服务 | `docs/services/06-task-service.md` |
| `internal/audit/` 或 `cmd/audit/` | 审计服务 | `docs/services/07-audit-service.md` |
| `internal/variable/` 或 `cmd/variable/` | 变量服务 | `docs/services/08-variable-service.md` |
| `internal/credential/` 或 `cmd/credential/` | 凭证服务 | `docs/services/09-credential-service.md` |
| `internal/pluginmanager/` 或 `cmd/pluginmanager/` | 插件管理器 | `docs/services/10-plugin-manager.md` |
| `internal/platformplugin/` 或 `cmd/platform-plugin/` | 平台插件 | `docs/services/11-platform-plugin.md` |

**映射不到已有服务时**（如纯前端变更或基础库变更）：跳过此步骤，输出提示。

### 4.2 确定更新内容

对每个受影响的服务设计文档：

1. **读取当前服务设计文档**（如 `docs/services/11-platform-plugin.md`）
2. **读取变更的 design.md 和 spec.md**，提取本次变更对该服务的影响
3. **确定更新策略**：

| 变更类型 | 更新方式 | 示例 |
|---------|---------|------|
| 新增组件/插件 | 在服务文档的相关章节追加新内容 | 新增 UDNS 插件 → 在 platform-plugin 设计文档中添加 UDNS 相关章节 |
| 修改已有功能 | 定位并更新服务文档中对应的章节 | 修改 Get 日志 → 更新 helmrelease 插件的日志说明 |
| 新增 API | 在 API 定义章节追加接口说明 | 新增 REST API → 追加到 API 章节 |
| 新增数据模型 | 在数据模型章节追加表结构 | 新增 DB 表 → 追加到表结构章节 |
| 配置变更 | 更新配置说明章节 | 新增配置项 → 追加到配置章节 |

### 4.3 生成服务文档更新预览

**必须在执行更新前向用户展示预览。**

格式：
```
## 服务设计文档更新预览

### docs/services/11-platform-plugin.md
- 追加章节：「UDNS 插件」（在「Helm Release 插件」章节之后）
  - 包含：架构设计、API 说明、配置项、错误处理
- 修改章节：「插件注册」（更新注册列表，新增 UDNS）

### docs/services/04-component-service.md
- 追加章节：「UDNS 组件类型」
  - 包含：组件模板定义、Action 映射

确认更新？
a. 执行更新（推荐）
b. 跳过更新
c. 取消归档
```

**禁止在用户确认前修改 `docs/services/` 下的任何文件。**

### 4.4 执行更新

用户确认后，逐个更新服务设计文档：

1. **定位目标章节**：根据 4.2 分析的更新策略，在服务文档中找到插入/修改位置
2. **执行写入**：
   - **追加**：在目标位置插入新章节，保持与现有文档风格一致
   - **修改**：替换目标章节内容
3. **保持一致性**：
   - 标题级别与现有文档对齐
   - 术语和命名风格保持统一
   - 不修改变更范围之外的内容

### 4.5 更新后验证

每个服务文档更新后：
1. 读取更新后的文档，确认内容完整、格式正确
2. 确认新增内容与变更的 design.md 和 spec.md 一致
3. 输出更新结果：`✓ docs/services/11-platform-plugin.md 已更新`

---

## Step 5：执行归档

### 5.1 创建归档目录

```bash
mkdir -p openspec/changes/archive
```

### 5.2 生成归档目标名

格式：`YYYY-MM-DD-<change-name>`

```bash
date +%Y-%m-%d
```

**冲突检查**：
```bash
ls openspec/changes/archive/ | grep "YYYY-MM-DD-<change-name>"
```

| 结果 | 处理 |
|------|------|
| 不存在 | 继续 |
| 已存在 | 报错，建议用户重命名现有归档或追加后缀 |

### 5.3 移动变更目录

```bash
mv openspec/changes/<name> openspec/changes/archive/YYYY-MM-DD-<name>
```

---

## Step 6：输出归档摘要

```
## 归档完成

**变更名称：** <change-name>
**归档位置：** openspec/changes/archive/YYYY-MM-DD-<name>/

### 功能文档同步结果（openspec/specs/）
- ✓ <capability-1>/spec.md — 新建
- ✓ <capability-1>/design.md — 新建
- ✓ <capability-2>/spec.md — 合并（MODIFIED: 2 个章节）
- ✓ <capability-2>/design.md — 追加
- ○ 无 delta specs（或：跳过同步）

### 服务设计文档更新结果（docs/services/）
- ✓ docs/services/11-platform-plugin.md — 追加 2 个章节
- ✓ docs/services/04-component-service.md — 修改 1 个章节
- ○ 无需更新（或：跳过更新）

### 文件清单
- proposal.md ✓
- design.md ✓
- tasks.md ✓（N/M 任务完成）
- specs/ ✓（K 个 delta specs）
```

---

## 禁止行为

- 禁止在用户未确认同步预览前执行任何主 spec 修改
- 禁止在用户未确认更新预览前修改 `docs/services/` 下的任何文件
- 禁止在主 spec 中留下 "TBD"、"TODO"、占位符内容
- 禁止在主 spec 中残留 `ADDED`/`MODIFIED`/`REMOVED` 标记
- 禁止修改 `openspec/changes/archive/` 下的已归档变更
- 禁止删除变更目录（只能 mv 到归档）
- 禁止修改原生 `openspec-archive-change` 技能
- 禁止在 `docs/services/` 更新时修改变更范围之外的内容

---

## 回滚

用户触发 `/opsx:rollback`（由 SKILL.md 路由到此）或在 Phase 4 执行中说"回滚"时执行。

### Step 1：状态诊断

检测当前归档状态，确定可回滚的操作：

```bash
# 检查主 specs 是否有未提交变更（功能文档同步产生的）
git diff --stat openspec/specs/
git diff --cached --stat openspec/specs/

# 检查 docs/services/ 是否有未提交变更（服务文档更新产生的）
git diff --stat docs/services/
git diff --cached --stat docs/services/

# 检查今天归档的变更
ls openspec/changes/archive/ | grep "$(date +%Y-%m-%d)"
```

同时检查 `openspec/changes/` 下是否还有对应的活跃变更（判断归档 mv 是否已执行）。

### Step 2：展示状态并选择回滚粒度

```
## 当前归档状态

**主 Specs 变更：** N 个文件（未提交 / 已提交 / 无变更）
**服务文档变更：** M 个文件（未提交 / 已提交 / 无变更）
**今日归档：** <change-name-1>, <change-name-2>（或"无"）

请选择回滚范围：
a. 回滚功能文档同步 — 撤销 openspec/specs/ 的变更
b. 回滚服务文档更新 — 撤销 docs/services/ 的变更
c. 回滚归档移动 — 将变更从 archive/ 移回 changes/
d. 全部回滚 — 撤销所有同步 + 恢复归档移动
e. 取消
```

**多个归档时**：用 `AskQuestion` 让用户选择要回滚哪个变更。

### Step 3：执行回滚

#### a. 回滚功能文档同步

1. **未提交变更**：
   ```bash
   git checkout -- openspec/specs/
   ```

2. **已暂存变更**：
   ```bash
   git reset HEAD openspec/specs/
   git checkout -- openspec/specs/
   ```

3. **已提交变更**：
   - 找到同步产生的提交：
     ```bash
     git log --oneline --all -- openspec/specs/ | head -5
     ```
   - 未推送 → `git reset --soft <同步前commit>`
   - 已推送 → `git revert <同步commit> --no-edit`

4. **验证**：
   ```bash
   git diff --stat openspec/specs/  # 应为空
   ```

#### b. 回滚服务文档更新

1. **未提交变更**：
   ```bash
   git checkout -- docs/services/
   ```

2. **已暂存变更**：
   ```bash
   git reset HEAD docs/services/
   git checkout -- docs/services/
   ```

3. **已提交变更**：
   - 找到更新产生的提交：
     ```bash
     git log --oneline --all -- docs/services/ | head -5
     ```
   - 未推送 → `git reset --soft <更新前commit>`
   - 已推送 → `git revert <更新commit> --no-edit`

4. **验证**：
   ```bash
   git diff --stat docs/services/  # 应为空
   ```

#### c. 回滚归档移动

1. **定位归档目录**：
   ```bash
   ls -d openspec/changes/archive/*-<change-name>
   ```

2. **检查目标不冲突**：
   ```bash
   ls openspec/changes/<change-name> 2>/dev/null
   ```
   - 目标已存在 → 报错，让用户先处理冲突
   - 目标不存在 → 继续

3. **执行恢复**：
   ```bash
   mv openspec/changes/archive/YYYY-MM-DD-<change-name> openspec/changes/<change-name>
   ```

4. **验证**：
   ```bash
   ls openspec/changes/<change-name>/  # 确认文件完整
   ```

#### d. 全部回滚

按顺序执行：先 a（回滚功能文档同步），再 b（回滚服务文档更新），最后 c（回滚归档移动）。

### Step 4：回滚后验证与摘要

```
## 归档回滚完成

**变更名称：** <change-name>
**回滚范围：** <选择的粒度>
**功能文档同步：** ✓ 已撤销（N 个文件恢复）/ ○ 未操作
**服务文档更新：** ✓ 已撤销（M 个文件恢复）/ ○ 未操作
**归档移动：** ✓ 已恢复到 openspec/changes/<name>/ / ○ 未操作

当前状态：变更 <change-name> 已恢复为活跃状态。
可以重新执行 /opsx:archive 重新归档。
```
