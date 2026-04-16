# BSCP 多租户迁移报告

**迁移时间：** 2026-03-20  
**源库：** bk_bscp_admin（单租户）  
**目标库：** bk_bscp_admin（多租户）  
**迁移工具版本：** v2（含虚拟 ID 优化）

---

## 一、迁移概览

| 指标 | 数值 |
|------|------|
| 总业务数 | 17 |
| 总应用数 | 92 |
| 源库总记录 | 70,747 |
| 成功迁移 | 51,129 |
| 未迁移（源库脏数据） | 19,618 |
| 全量迁移业务 | 13 个 |
| 有脏数据差异的业务 | 4 个 |

---

## 二、逐业务迁移结果

| 业务 ID | 应用数 | 源记录数 | 迁移数 | 脏数据 | 耗时 |
|---------|--------|---------|--------|--------|------|
| 2 | 28 | 63,106 | 46,020 | 17,086 | 5m25s |
| 20 | 25 | 1,297 | 1,211 | 86 | 8.9s |
| 21 | 1 | 9 | 9 | 0 | 0.2s |
| 25 | 4 | 880 | 880 | 0 | 5.3s |
| 63 | 7 | 263 | 263 | 0 | 2.8s |
| 18791 | 1 | 4,126 | 1,782 | 2,344 | 18.2s |
| 18814 | 1 | 22 | 22 | 0 | 0.3s |
| 18841 | 5 | 254 | 254 | 0 | 3.8s |
| 18927 | 1 | 20 | 20 | 0 | 0.3s |
| 18938 | 1 | 6 | 6 | 0 | 0.2s |
| 18985 | 2 | 70 | 70 | 0 | 0.7s |
| 18986 | 4 | 84 | 84 | 0 | 0.8s |
| 18987 | 1 | 21 | 21 | 0 | 0.2s |
| 18994 | 6 | 172 | 90 | 82 | 1.8s |
| 19020 | 2 | 345 | 345 | 0 | 2.1s |
| 19025 | 1 | 22 | 22 | 0 | 0.3s |
| 19078 | 2 | 30 | 30 | 0 | 0.4s |
| **跨业务孤儿** | - | 20 | - | **20** | - |
| **合计** | **92** | **70,747** | **51,129** | **19,618** | **~6min** |

> 注：源记录数 70,727 为迁移时 17 个业务的源数据之和，另有 20 条跨业务孤儿记录不属于任何业务，合计 70,747。

---

## 三、与上一版迁移对比（虚拟 ID 优化效果）

本次迁移相比上一版（2026-03-19），新增了 **虚拟 ID（OptionalFKs）** 策略，用于处理第二层孤儿数据——`config_item_id` 指向已删除配置项的 `commits`/`contents`/`released_config_items` 记录。

### 3.1 逐业务恢复明细

| 业务 ID | 上次脏数据 | 本次脏数据 | 恢复数 | 说明 |
|---------|-----------|-----------|--------|------|
| 25 | 375 | **0** | +375 | 21 个已删除 config_item 的关联数据全部通过虚拟 ID 迁移 |
| 18814 | 2 | **0** | +2 | 1 个已删除 config_item 的关联数据通过虚拟 ID 迁移 |
| 19020 | 3 | **0** | +3 | 1 个已删除 config_item 的关联数据通过虚拟 ID 迁移 |
| 18791 | 2,531 | 2,344 | +187 | 第二层孤儿全部恢复（+4 commits, +4 contents, +179 rci），剩余为第一层 |
| 2 | 17,340 | 17,086 | +254 | 第二层孤儿恢复，剩余为已删除 app 的第一层孤儿 |
| 20 | 127 | 86 | +41 | 第二层孤儿恢复，剩余为已删除 app 的第一层孤儿 |
| 18994 | 82 | 82 | 0 | 全部为已删除 app 的第一层孤儿（KV 类型），无第二层孤儿 |
| **合计** | **20,480** | **19,618** | **+862** | |

### 3.2 总结

- 虚拟 ID 方案成功恢复 **862 条** 第二层孤儿数据
- **3 个业务**（25、18814、19020）由"有差异"变为"全量迁移"
- 全量迁移业务从 10 个增至 **13 个**
- 剩余未迁移数据**全部**为已删除 App 的第一层孤儿或跨业务孤儿，属于不可访问、不影响功能的脏数据

---

## 四、脏数据分析

### 4.1 根因

所有未迁移记录（19,618 条）均为**源库本身的数据完整性问题**，分为两类：

#### 第一类：已删除 Application 的孤儿数据（19,598 条）

源库中部分 Application 被删除后，其关联的下游数据未做级联清理，形成孤儿记录。

**影响链路：**

```
Application 被删除（仅删了 applications 表记录）
│
├── config_items     残留（app_id 指向已删除的 app）
├── releases         残留
├── commits          残留（app_id 指向已删除的 app）
├── contents         残留（同上）
├── strategies       残留
├── current_published_strategies  残留
├── kvs              残留
├── released_config_items  残留
├── released_kvs     残留
├── released_groups  残留
├── released_hooks   残留
└── app_template_variables  残留
```

> 注：在虚拟 ID 优化后，所有剩余脏数据均为**第一层孤儿**（`app_id` 指向已删除 App），不再有第二层孤儿。

#### 第二类：跨业务孤儿数据（20 条）

部分记录的 `biz_id` 在 `applications` 表中不存在任何对应业务，属于业务本身已被删除后的残留。

**分布：**

| 表 | 孤儿数 |
|---|---|
| template_spaces | 2 |
| template_sets | 2 |
| releases | 4 |
| config_items | 1 |
| commits | 1 |
| contents | 1 |
| strategies | 3 |
| kvs | 1 |
| released_config_items | 3 |
| released_kvs | 1 |
| app_template_variables | 1 |
| **合计** | **20** |

### 4.2 逐业务脏数据详细分析

#### biz_id=2（脏数据 17,086 条）

**已删除的 App：** 共 56 个，其中 app_id=3 贡献约 68% 的孤儿数据。

全部 17,086 条均为第一层孤儿（`app_id` 指向已删除 app）。

| 表 | 孤儿数 |
|---|---|
| strategies | 2,776 |
| releases | 2,774 |
| contents | 2,294 |
| commits | 2,294 |
| released_config_items | 2,318 |
| current_published_strategies | 2,197 |
| released_kvs | 1,776 |
| kvs | 624 |
| app_template_variables | 31 |
| config_items | 2 |
| **合计** | **17,086** |

#### biz_id=18791（脏数据 2,344 条）

**已删除的 App：** app_id=2、app_id=5（`applications` 表中已不存在）

全部 2,344 条均为第一层孤儿（`app_id` 指向已删除 app）。

| 表 | 孤儿数 |
|---|---|
| released_config_items | 757 |
| commits | 548 |
| contents | 548 |
| strategies | 219 |
| releases | 159 |
| current_published_strategies | 104 |
| config_items | 8 |
| app_template_variables | 1 |
| **合计** | **2,344** |

> 注：上一版迁移中该业务有 187 条第二层孤儿（4 commits + 4 contents + 179 released_config_items，`config_item_id` 指向已删除配置项），本次已通过虚拟 ID 全部恢复迁移。

#### biz_id=20（脏数据 86 条）

**已删除的 App：** app_id=1, 6, 14, 17, 72（共 5 个）

全部 86 条均为第一层孤儿（`app_id` 指向已删除 app）。

| 表 | 孤儿数 |
|---|---|
| released_config_items | 22 |
| contents | 18 |
| commits | 18 |
| config_items | 9 |
| releases | 7 |
| strategies | 5 |
| released_hooks | 4 |
| app_template_variables | 2 |
| released_groups | 1 |
| **合计** | **86** |

#### biz_id=18994（脏数据 82 条）

**已删除的 App：** app_id=80、app_id=83、app_id=140（`applications` 表中已不存在）

全部 82 条均为第一层孤儿（`app_id` 指向已删除 app），无第二层孤儿。

| 表 | 孤儿数 |
|---|---|
| released_kvs | 35 |
| strategies | 22 |
| releases | 18 |
| kvs | 7 |
| **合计** | **82** |

该业务为 KV 类型应用，脏数据集中在 `kvs`/`released_kvs`/`releases`/`strategies` 四张表。

#### 已通过虚拟 ID 恢复的业务

以下 3 个业务在上一版迁移中有脏数据差异，本次通过虚拟 ID 优化已**全量迁移**：

| 业务 ID | 上次脏数据 | 恢复原因 |
|---------|-----------|---------|
| **25** | 375 | 21 个已删除 config_item（id: 8254~8391）的 305 released_config_items + 35 commits + 35 contents |
| **19020** | 3 | 1 个已删除 config_item（id=8355）的 1 released_config_item + 1 commit + 1 content |
| **18814** | 2 | 1 个已删除 config_item（id=8364）的 1 commit + 1 content |

#### 汇总

| 业务 ID | 脏数据总数 | 类型 |
|---------|-----------|------|
| 2 | 17,086 | 第一层（app 删除） |
| 18791 | 2,344 | 第一层（app 删除） |
| 20 | 86 | 第一层（app 删除） |
| 18994 | 82 | 第一层（app 删除） |
| 跨业务 | 20 | biz_id 不存在 |
| **合计** | **19,618** | |

### 4.3 脏数据产生的代码根因

经审查 `DeleteApp` 逻辑（`cmd/data-service/service/app.go`），删除 Application 时只级联清理了**部分**关联表：

**已级联清理的表（删除 App 时会一并删除）：**

| 表 | 清理方式 |
|---|---|
| app_template_bindings | DeleteByAppIDWithTx |
| group_app_binds | BatchDeleteByAppIDWithTx |
| released_groups | BatchDeleteByAppIDWithTx |
| released_app_templates | BatchDeleteByAppIDWithTx |
| released_app_template_variables | BatchDeleteByAppIDWithTx |
| released_hooks | DeleteByAppIDWithTx |
| credential_scopes | BatchDeleteWithTx（匹配 app name） |

**未级联清理的表（删除 App 后产生孤儿数据）：**

| 表 | 残留原因 |
|---|---|
| config_items | app_id 指向已删除的 app |
| releases | 同上 |
| commits | 同上 |
| contents | 同上 |
| strategies | 同上 |
| current_published_strategies | 同上 |
| released_config_items | 同上 |
| kvs | app_id 指向已删除的 app |
| released_kvs | 同上 |
| app_template_variables | 同上 |

这是 BSCP 应用层的已有缺陷，`DeleteApp` 未做完整的级联清理，导致上述 10 张表在删除 App 后残留孤儿记录。迁移工具检测到这些记录的外键引用无效后正确地跳过了它们。

### 4.4 BSCP 配置项删除与恢复机制

#### 未命名版本（工作草稿）的状态判断

BSCP 通过对比 `config_items`（当前工作草稿）和 `released_config_items`（最近一次发布快照）来确定每个配置项的状态：

```
config_items（当前）  vs  released_config_items（最新发布）
    │
    ├── 两边都有，commit 未变 → UNCHANGE（未修改）
    ├── 两边都有，commit 已变 → REVISE（已修改）
    ├── 只在 config_items 中  → ADD（新增文件）
    └── 只在 released 中      → DELETE（已删除，可恢复）
                                    ↑ 数据来源是 released_config_items 的快照
```

"最新发布"的判断方式：取 `released_config_items` 中 `release_id` 最大的一组记录（`ORDER BY release_id DESC LIMIT 1`），即最近一次发布的版本。

#### 配置项删除机制

用户删除配置项时：
- `config_items` 记录被**硬删除**
- `released_config_items`/`commits`/`contents` **按设计保留**
- 页面上该配置项显示为 "DELETE" 状态（红色标记），数据来源是 `released_config_items` 中冗余存储的配置项快照（名称、路径、文件类型等）

#### 配置项恢复（UnDelete）机制

恢复前提：该配置项**在最新发布版本中存在**（即删除发生在最近一次发布之后）。

恢复流程（`UnDeleteConfigItem`）：
1. 查找该 App 的最新发布版本（`releases` 表中 `release_id` 最大的记录）
2. 通过 `release_id + config_item_id` 从 `released_config_items` 中找到快照数据
3. 用快照数据重建 `config_items`、`commits`、`contents` 记录

**不可恢复的场景：** 如果用户删除配置项后又发布了新版本，新版本不包含该配置项，`released_config_items` 中最新发布不再有它的记录，恢复功能将不可用。

### 4.5 虚拟 ID 迁移策略（OptionalFKs）

#### 问题

第二层孤儿是指 `app_id` 有效但 `config_item_id` 指向已被用户删除的配置项的记录，涉及 `released_config_items`、`commits`、`contents` 三张表。如果直接跳过这些记录，会导致：
- 历史发布版本查看时配置项缺失
- 如果当前活跃发布仍引用这些配置项，客户端拉取配置时会缺少文件
- 处于 "DELETE" 状态的配置项在未命名版本页面上消失，恢复功能不可用

#### 解决方案

迁移工具采用**虚拟 ID** 策略（`OptionalFKs`）：
- 对 `commits`、`contents`、`released_config_items` 三张表的 `config_item_id` 标记为可选外键
- 当 `config_item_id` 在源库中已被删除（映射不到目标 ID）时，从 `id_generators` 表原子分配一个虚拟 ID
- 同一个源 `config_item_id` 在所有表中映射到同一个虚拟 ID（保证一致性）
- **不创建**实际的 `config_items` 记录
- 虚拟 ID 通过 `id_generators` 原子分配，不会与后续正常创建的记录 ID 冲突

#### 效果验证

- `config_items` 表中无该虚拟 ID → 页面正确显示 "DELETE" 状态 ✓
- `released_config_items` 保留完整快照 → 历史版本查看不缺数据 ✓
- `UnDeleteConfigItem` 能通过虚拟 ID 找到快照 → 恢复功能可用 ✓
- `commits`/`contents` 保留 → `released_config_items` 的 `commit_id`/`content_id` 映射不断裂 ✓
- `ConfigItemID.Neq(0)` 过滤条件不会误排除虚拟 ID（虚拟 ID 非零）✓
- 本次迁移成功恢复 **862 条** 记录，3 个业务变为全量迁移 ✓

---

## 五、迁移后数据完整性验证

### 5.1 外键关联验证

对目标库执行全量外键关联检查（覆盖所有 29 张表的 56 组外键关系），所有检查均通过：

| 检查项 | 结果 |
|--------|------|
| commits.content_id → contents | 0 悬空 ✓ |
| released_config_items.content_id → contents | 0 悬空 ✓ |
| released_app_templates.template_space_id → template_spaces | 0 悬空 ✓ |
| released_app_templates.template_set_id → template_sets | 0 悬空 ✓ |
| released_app_templates.template_id → templates | 0 悬空 ✓ |
| released_app_templates.template_revision_id → template_revisions | 0 悬空 ✓ |
| template_sets.bound_apps (JSON) → applications | 0 悬空 ✓ |

### 5.2 数据量对比验证（全局扫描）

扫描时间：2026-03-20T09:45:02+08:00

| 指标 | 数值 |
|------|------|
| 源库总记录 | 70,747 |
| 目标库总记录 | 51,129 |
| 差异 | 19,618 |
| 完全匹配的表 | 15 张 |
| 有差异的表 | 14 张 |

所有差异均来自已删除 App 的第一层孤儿或跨业务孤儿，目标库无数据丢失。

### 5.3 与上一版迁移数据量对比

| 指标 | 上一版（03-19） | 本次（03-20） | 变化 |
|------|----------------|--------------|------|
| 源库总记录 | 70,747 | 70,747 | 0 |
| 目标库总记录 | 50,267 | 51,129 | **+862** |
| 差异 | 20,480 | 19,618 | **-862** |
| 全量迁移业务 | 10 | 13 | **+3** |

目标库增加的 862 条记录即为虚拟 ID 策略恢复的第二层孤儿数据，源库无变化。
