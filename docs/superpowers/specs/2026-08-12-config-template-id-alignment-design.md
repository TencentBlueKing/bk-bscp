# 配置模版 ID 对齐设计

日期：2026-08-12

## 背景

GSEKit 数据迁移到 BSCP 后，`config_templates.id` 由 BSCP 的 `id_generators` 重新分配，与 GSEKit 的
`gsekit_configtemplate.config_template_id` 无关。而标准运维（bk-sops）流程模板里保存的是 GSEKit 的
配置模版 ID，插件执行时把它作为 `config_template_ids` 传给 BSCP，导致查不到模版、流程执行失败。
用户必须重新编辑并保存每一条流程才能恢复，迁移无法做到无感。

本设计解决两件事：把存量已迁移数据的 `config_templates.id` 对齐到 GSEKit 的 ID；改造迁移流程，
让后续批次迁移时直接复用 GSEKit ID，不再产生同类问题。

## 事实依据

### 需要对齐的只有 `config_templates.id`

bk-sops 侧下拉框的数据源用 GSEKit 的 `config_template_id` 作为 option value：

- `bk-sops/pipeline_plugins/components/query/sites/open/gsekit.py`
  `{"text": template["template_name"], "value": template["config_template_id"]}`
- `bk-sops/pipeline_plugins/components/collections/gse_kit/job_exec/v1_0.py`
  `extra_data["config_template_ids"] = gsekit_config_template`

BSCP 侧该 ID 最终落到 `config_templates` 表主键查询：
`GenerateConfig` → `operate_range.config_template_ids` → `getConfigTemplatesByIDs`
→ `dao.ConfigTemplate().GetByID`（`bk-bscp/cmd/data-service/service/config_instance.go`）。

标准运维插件只保存了模版 ID，没有保存版本 ID（插件模式下 BSCP 自行取最新版本），
因此 `templates.id` 与 `template_revisions.id` 无需对齐。

### 匹配用的两条锚点

**版本 ID 回溯（主）。** 迁移工具写 `template_revisions` 时，版本名直接由 GSEKit 版本 ID 拼成：

```go
// bk-bscp/cmd/gsekit-migration/migrator/config_template.go
revisionID, fmt.Sprintf("v%d", version.ConfigVersionID), version.Description, ...
```

回溯链：BSCP `config_templates.id` → `template_id` → `templates.id`
→ `template_revisions.revision_name` 形如 `v<config_version_id>`
→ GSEKit `gsekit_configtemplateversion.config_version_id` → `config_template_id`。

这条链锚定的是版本 ID，对改名、改文件名、改路径、新增版本全部免疫。用户新增版本是 INSERT 新行，
不会改动迁移写入的旧行，锚点不会丢失。盲区是用户通过 `DeleteTemplateRevision` 把迁移过来的老版本
全部删除。

> **`^v\d+$` 格式本身不是判据。** BSCP 页面更新配置模版时填入的默认版本名同样是 `v` 加数字
> （时间戳形式，如 `v20260812143153`），且服务端对 `revision_name` 不做任何格式约束，
> 直接采用请求传入的值。因此版本名格式既不能证明"是迁移产物"，也不能证明"不是"。
>
> 格式匹配只允许用于**筛选候选**，锚定必须以"解析出的数字能在 GSEKit
> `gsekit_configtemplateversion` 反查到、且反查到的模版 `bk_biz_id` 等于 BSCP `biz_id`"
> 为唯一依据（即下文阶段 1 的第 2 步）。不要用数字位数之类的启发式替代反查。
>
> 实测依据见 `2026-08-12-config-template-migration-data-analysis.md` 第六节。

**名字匹配（辅）。** GSEKit `ConfigTemplate.Meta.unique_together = ("bk_biz_id", "template_name")`，
BSCP `config_templates` 有 `uniqueIndex idx_bizID_name (biz_id, name)`，且迁移时
`config_templates.name` 直接取 `gsekit.template_name`。所以 `(biz_id, name)` 是双侧唯一的天然键。
盲区是业务在 BSCP 侧改过模版名。

两条锚点的盲区不重叠，互为补充。

> **不要用"按 ID 顺序对应"替代匹配。** 即使某业务两侧条数相同、ID 各自连号，也不能按位置配对：
> 迁移读取 `gsekit_configtemplate` 的查询没有 `ORDER BY`（见下方"迁移分页缺少排序"），
> 遍历顺序不由 `config_template_id` 保证。一旦错配，两侧都是有效 ID、不会报错，
> 属于静默数据损坏。匹配必须逐条走上述两条锚点。

### 审计表不能用来回溯原名

`AuditBuilderV2.PrepareUpdate`（`bk-bscp/internal/dal/dao/audit_builder_v2.go`）的注释写着
"会记录 spec 对比值"，但实现只设置了 `ResourceType` / `ResourceID` / `Action`，没有写 `detail`。
配置模版 update 审计的 `res_instance` 记录的是改名**之后**的新名字。因此审计表查不到原名。

另外迁移工具是用裸 SQL `INSERT` 写入 `config_templates` 的，不产生 create 审计记录。

### 预留基线必须取 AUTO_INCREMENT

GSEKit 的 `ConfigTemplate` 继承 `OperateRecordModel`（`bk-process-config-manager/apps/utils/models.py`），
只有 `created_at` / `updated_at` / `created_by` / `updated_by`，**没有 `is_deleted`**——是硬删除。
主键是 `AutoField`，被删除的 ID 不回收。所以 `MAX(config_template_id)` 可能明显低于真实水位，
预留基线必须画在 `information_schema.TABLES.AUTO_INCREMENT` 之上。

同时标准运维流程里可能还保存着已删除模版的 ID，这类 ID 对齐后依然查不到——属于业务侧本来就已失效
的配置，不在本次范围内。

### 引用 `config_templates.id` 的存量数据

| 位置 | 说明 |
| --- | --- |
| `config_instances.config_template_id` | 历史配置实例，业务可见 |
| `task_batches.task_data` | longtext 存的 JSON，内含 `config_template_ids` 数组 |
| `audits.res_id` | `res_type = 'config_template'` 的记录 |

BSCP 不使用外键约束，改主键不会级联也不会报错，只会静默产生脏数据，三处都必须同步更新。

不受影响的：`template_sets.template_ids` 引用 `templates.id`；
`config_templates.cc_process_ids` / `cc_template_process_ids` 存的是 CC 进程 ID。

## 关键决策

| 决策点 | 结论 | 理由 |
| --- | --- | --- |
| 作用范围 | 存量一次性对齐 + 改造 migrate 流程 | 后续批次迁移不再需要二次处理 |
| 搬迁算法 | 全区腾空再入位 | 见下方"为什么全区腾空" |
| 预留基线 | 20000，做成配置项 | 实测 GSEKit 水位 11249，余量 8751；按实测年消耗约 172 可撑约 50 年，余量充裕。fail-fast 护栏与余量告警保留为兜底 |
| 匹配策略 | 版本 ID 回溯为主，名字匹配为辅 | 两者盲区不重叠；实测存量数据名字匹配可 100% 命中，回溯作交叉校验拦截错配 |
| md5 交叉校验 | 不做 | biz_id + 名字交叉已覆盖误匹配；GSEKit 侧内容若被改过会产生假警报 |
| 执行形态 | `gsekit-migration` 新增子命令 | 源库与目标库是不同 MySQL 实例，纯 SQL 无法跨实例 JOIN |
| 引用同步 | 三处全改 | 不留脏数据 |
| 停机窗口 | 不需要 | 见下方"并发安全" |

### 为什么全区腾空

按需腾空（只移动真正冲突的行）附带损伤更小，但会在 `[1, 20000)` 区间残留 BSCP 自建模版。
由于 migrate 流程改造后每批迁移都会往这个区间落 ID，残留的每一条都是后续某批业务迁入时的冲突源，
届时又要重新搬迁一次并再次更新引用表。

全区腾空把 `[1, 20000)` 一次性清空并永久划归 GSEKit 对齐专用，`[20000, ∞)` 归 BSCP 自建。
代价是所有 BSCP 自建模版的 ID 都会变一次——在当前只有 200 多条时付这个代价，比以后分批被动处理便宜。

### 预留基线的余量与上调路径

2026-08-12 实测：GSEKit `gsekit_configtemplate.AUTO_INCREMENT = 11249`，存量 1561 条，
分布在 121 个业务，`MIN = 77`、`MAX = 11248`。基线取 20000 时余量为 8751。

需要注意这张表的 ID 消耗是跳跃式的，不能按存量条数外推：1561 条存量却已消耗 11249 个 ID，
约 86% 的 ID 空间被删除或跳跃吃掉；且 `2018` 到 `9999` 之间没有任何存量行，
存在一段近 8000 个 ID 的断层。加上 121 个业务中绝大多数尚未迁移、仍在持续新建，
余量耗尽是有可能发生的。

因此基线做成配置项，并保证上调路径可用：

- 阶段 3 的腾空条件是"所有 `id < reserve_base` 的行搬到高位"，因此把基线从 20000 调到更大值后
  重跑 `align-template-id`，会把此前落在 `[20000, 新基线)` 的 BSCP 自建模版连同已对齐的迁移产物
  一起重新腾空再入位，结果依然正确。
- 代价是引用表要再更新一遍，且这批自建模版的 ID 会再变一次。所以基线应尽早定得宽裕，
  上调是兜底手段而非常规操作。

### 并发安全

`UPDATE id_generators SET max_id = GREATEST(max_id, 20000) WHERE resource = 'config_templates'`
是一个可以独立先提交的原子操作。它一旦生效，之后任何并发新建拿到的 ID 都在预留区，
不可能落进 GSEKit 区间。

剩下的搬迁只和存量的 200 多行竞争，在单个 MySQL 事务里完成，耗时毫秒到秒级。
配合阶段 0 检查"无进行中的配置生成/下发任务"，不需要真正的停机窗口。

## align-template-id 子命令

命令形式：

```
bk-bscp-gsekit-migration align-template-id -c <配置文件> [选项]
```

| 选项 | 说明 |
| --- | --- |
| `-c, --config` | 配置文件路径（必填） |
| `--dry-run` | 只输出报告不改数据。默认行为，不传 `--execute` 即等于 dry-run |
| `--execute` | 真正执行；与 `--dry-run` 互斥，必须显式传入 |
| `--mapping-file` | 人工映射覆盖 CSV，格式 `biz_id,bscp_config_template_id,gsekit_config_template_id` |
| `-o, --output` | 报告输出路径，默认 `align-template-id-report-<YYYYMMDD-HHMMSS>.json` |
| `--strict` | 只有两法一致才自动对齐，其余全部转为待人工确认 |
| `--force` | 跳过"存在进行中任务"的检查 |
| `--rollback` | 按指定报告 JSON 反向执行 |
| `--reserve-base` | 覆盖配置文件中的预留基线，用于基线上调场景 |

新增配置项 `migration.config_template_id_reserve_base`，默认 20000，被子命令与 migrate 流程共用。

### 阶段 0：体检（只读）

1. 读 GSEKit `gsekit_configtemplate` 的 `AUTO_INCREMENT`；≥ 预留基线则失败退出，提示先调大基线。
2. 读 BSCP `config_templates` 全量：`id, biz_id, name, template_id, creator, reviser, created_at, updated_at`。
3. 读 `id_generators` 中 `config_templates` 的 `max_id`。
4. 检查 `task_batches` 是否存在非终态记录；存在则拒绝执行，除非 `--force`。
5. 扫描目标库 `information_schema.COLUMNS`，确认引用 `config_templates.id` 的列没有超出已知三处；
   发现未知列则失败退出，提示补充实现。

### 阶段 1：建映射

对每条 BSCP `config_templates` 行求目标 GSEKit ID，优先级从高到低：

1. 人工 CSV 覆盖。
2. 版本 ID 回溯：取该模版所有 `revision_name` 匹配 `^v(\d+)$` 的 `template_revisions` 行，
   逐个用捕获到的数字去 GSEKit `gsekit_configtemplateversion` 反查 `config_template_id`，
   并校验反查到的 `gsekit_configtemplate.bk_biz_id` 等于 BSCP 的 `biz_id`。命中即锚定。
3. 名字匹配：GSEKit `(bk_biz_id, template_name)` 等于 BSCP `(biz_id, name)`。

按两种自动策略的一致性分类：

| 分类 | 含义 | 默认处理 |
| --- | --- | --- |
| `MATCHED_BOTH` | 两法命中同一 ID | 自动对齐 |
| `MATCHED_REVISION_ONLY` | 只有版本回溯命中，疑似业务改过名 | 自动对齐，报告标注 |
| `MATCHED_NAME_ONLY` | 只有名字命中，疑似迁移版本被删 | 自动对齐，报告标注 |
| `CONFLICT` | 两法命中不同 ID | 整单失败，必须 CSV 裁决 |
| `UNMATCHED_NATIVE` | 都没命中，特征像 BSCP 自建 | 搬到预留区 |
| `UNMATCHED_UNKNOWN` | 都没命中，特征像迁移产物 | 报告，需人工确认后走 CSV |

`--strict` 下 `MATCHED_REVISION_ONLY` 和 `MATCHED_NAME_ONLY` 都转为待人工确认。

`UNMATCHED_NATIVE` 与 `UNMATCHED_UNKNOWN` 的最终数据操作相同（都搬到预留区），区别只在于
`UNMATCHED_UNKNOWN` 会阻塞执行、要求人工确认。区分判据是：`creator` 等于迁移配置中的
`migration.creator`（例如 `xiaolnwang`），则归为 `UNMATCHED_UNKNOWN`，
否则归为 `UNMATCHED_NATIVE`。

该判据不精确，因此刻意选择了保守的误判方向——把自建模版误判成 `UNMATCHED_UNKNOWN` 只是
多要求一次人工确认，而把迁移产物误判成 `UNMATCHED_NATIVE` 会导致漏对齐、问题残留。
判据本身不参与"对齐到哪个 ID"的决策，只决定是否需要人工介入。

#### 自建业务（`migration.native_biz_id`）

部分业务在 BSCP 侧的 `config_templates` 全部由人工用迁移账号手工创建，
与 GSEKit 无对应关系。按 `creator` 判据，这批记录会被整批归入
`UNMATCHED_UNKNOWN` 从而阻塞执行，而人工逐条确认的结论必然是"自建"，
这是一轮确定无收益的人工成本。当前环境里这指的是 `100148`。

因此新增 `FORCED_NATIVE` 分类：`biz_id` 等于配置项 `migration.native_biz_id`
的记录，在名字匹配之前就被拦下，无条件归入自建区，终值处理与
`UNMATCHED_NATIVE` 完全一致（已在预留区则不动，否则搬到 `planAlignment`
分配的高位 ID）。该值为 0 时不做任何例外处理。

之所以放在名字匹配之前而不是之后，是因为 GSEKit 侧这个业务也可能有数据
（`100148` 有 8 条，ID `11196`–`11248`）。若先做名字匹配，BSCP 侧偶然同名的
记录会被对齐到 GSEKit ID——但那只是名字巧合，两侧并非同一份模版，对齐反而
制造了错误的关联。整批搬走之后，GSEKit 的这段 ID 区间彻底空出，将来真要
`migrate` 这个业务时可以一次到位写入，不会与已搬到预留区的存量记录相撞。

单独立一个分类而不是复用 `UNMATCHED_NATIVE`，是为了让报告的 `summary` 能把这批
记录单独计数，审计时一眼能看出哪些搬迁源于业务白名单而非匹配结论。

`precheck-align-template-id` 对该业务的处理与此一致：整批跳过，不纳入前置校验。
两处都读 `migration.native_biz_id`。

### 阶段 2：报告

报告 JSON 内容：

- 每条记录：`bscp_id`、`biz_id`、`name`、`classification`、`gsekit_id_by_revision`、
  `gsekit_id_by_name`、`final_new_id`、`decision_source`
- 三张引用表各自的受影响行数
- GSEKit `AUTO_INCREMENT`、预留基线、`id_generators` 当前水位
- 分类统计汇总

`--dry-run`（默认）到此结束。`CONFLICT` 或 `UNMATCHED_UNKNOWN` 非空时，即使传了 `--execute` 也拒绝执行，
除非这些条目已被 `--mapping-file` 覆盖。

### 阶段 3：执行

第一步单独提交，不在后续事务内：

```sql
UPDATE id_generators SET max_id = GREATEST(max_id, <reserve_base>), updated_at = NOW()
WHERE resource = 'config_templates';
```

其余在单个事务内完成：

1. **腾空。** 所有 `id < reserve_base` 的 `config_templates` 行，从 `id_generators` 批量分配
   连续的高位 ID 并逐条 UPDATE。此时 GSEKit ID 区间彻底为空。
2. **入位。** 阶段 1 匹配到目标 ID 的行，从临时高位 ID UPDATE 到目标 GSEKit ID。
3. **更新引用。** 按最终映射 `old_id → final_id`（不是两阶段的中间值）一次性更新：
   - `UPDATE config_instances SET config_template_id = ? WHERE config_template_id = ?`
   - `task_batches.task_data`：Go 侧 unmarshal 成 `TaskExecutionData`，改写
     `ConfigTemplateIDs`，再 marshal 写回。不使用 SQL 的 JSON 函数，避免依赖 MySQL 版本。
   - `UPDATE audits SET res_id = ? WHERE res_type = 'config_template' AND res_id = ?`
4. **收尾。** `id_generators.max_id` 设为 `max(reserve_base, config_templates 当前最大 id)`。

### 阶段 4：校验（只读）

1. `config_templates` 无重复 ID。
2. 没有 ID 落在 `(gsekit_auto_increment, reserve_base)` 这段本应为空的区间。
3. `config_instances.config_template_id` 全部能在 `config_templates` 中找到。
4. 实际对齐条数与阶段 2 报告一致。

任一项失败则整体报错，提示用 `--rollback` 回滚。

### 幂等与回滚

重复执行时阶段 1 会发现绝大多数行已满足 `id == gsekit_id`，这些行的映射为空操作，
阶段 3 直接跳过。

回滚用 `--rollback <report.json>`，从报告读取完整的 `old_id → final_id` 映射，
反向执行同一套两阶段搬迁与引用更新。`id_generators.max_id` 不回退——水位只增不减是安全的。

## migrate 流程改造

`migrateConfigTemplates`（`bk-bscp/cmd/gsekit-migration/migrator/config_template.go`）中，
分配 `config_templates` ID 的逻辑从 `m.idGen.NextID("config_templates")` 改为直接使用
`tmpl.ConfigTemplateID`。

`templates` 与 `template_revisions` 的 ID 分配方式不变，仍走 `id_generators`。

改造后，后续批次在写入那一刻 ID 即等于 GSEKit ID，**不经过任何匹配环节**，
因此业务事后改名对其毫无影响。匹配策略与人工确认路径只服务于存量对齐这一次性任务。

### 迁移分页缺少排序（既有缺陷，需一并修复）

读取待迁移模版的分页查询没有 `ORDER BY`：

```155:158:bk-bscp/cmd/gsekit-migration/migrator/config_template.go
			var templates []GSEKitConfigTemplate
			if err := m.sourceDB.Where("bk_biz_id = ?", bizID).
				Offset(offset).Limit(batchSize).
				Find(&templates).Error; err != nil {
```

MySQL 不保证无序分页在跨批次时的结果稳定，可能重复返回部分行、遗漏另一部分行。
重复会撞 `(biz_id, name)` 唯一索引，在 `continue_on_error` 为真时被静默跳过；
遗漏则直接漏迁且无任何提示。

触发条件是单业务模版数超过 `batch_size`（默认 500）。已迁移业务中最多的仅 64 条，
存量迁移大概率未触发，但后续 104 个业务需先行确认。

修复方式：补上 `Order("config_template_id ASC")`。同文件中查询版本的逻辑已正确使用
`Order("config_version_id ASC")`，仅模版这一处遗漏。

护栏分两级：

- **硬失败**：每批迁移开始前，查询本批业务的 GSEKit `config_template_id` 最大值，
  若 ≥ 预留基线则拒绝迁移并报错，提示调大基线。这样即使余量耗尽也是显式失败，
  不会静默覆盖 BSCP 自建模版的 ID。
- **余量告警**：读 GSEKit `AUTO_INCREMENT`，若已超过预留基线的 80%，输出警告提示尽早调大基线。
  `align-template-id` 的阶段 0 做同样的告警。

迁移完成后同步抬高 `id_generators` 中 `config_templates` 的水位到
`max(reserve_base, 本批最大 gsekit_id)`，保持水位单调。

## 实测数据（2026-08-12）

GSEKit 源库 `bkapp-gsekit-6kw`：

| 指标 | 值 |
| --- | --- |
| `AUTO_INCREMENT` | 11249 |
| 存量条数 | 1561 |
| `MIN` / `MAX` | 77 / 11248 |
| 业务数 | 121 |
| ID 断层 | `2018` ~ `9999` 无存量行 |

BSCP 目标库 `bk_bscp_admin_v2`：

| 指标 | 值 |
| --- | --- |
| `id_generators.config_templates.max_id` | 203 |
| 被改动过的记录数（`reviser <> creator` 或 `updated_at > created_at + 2s`） | 13（采集于 14:20，现已增至 15） |
| ID 断层成因 | 2021-03 一次性人为跳跃，非常态消耗 |
| GSEKit 年均 ID 消耗 | 约 172（近 12 个月实测） |

> 该 13 条快照在采集后即失效：`100605` 的 BSCP ID 60、61 于 14:31–15:06 被改动，
> 清单实际为 15 条。这印证了阶段 0 必须在执行同一时点重新采集，不接受历史快照。
> 新增的 2 条当前名称与 GSEKit 一致，名字匹配可命中，无需人工介入。

这 15 条的 `creator` 分别是 `xiaolnwang`、`regyhuang`、`dathpan`。需要更正一处早期误判：
`xiaolnwang` **就是**迁移账号（`migration.creator`），因此其中 4 条属于"迁移产物被改动"，
并非全部是业务自建。

这 4 条的名字匹配结论已全部验证：

| BSCP ID | biz_id | 改动者 | 名字匹配结果 |
| --- | --- | --- | --- |
| 55 | 5000079 | `regyhuang` | 命中 GSEKit 11120，未改名 |
| 56 | 5000079 | `regyhuang` | 命中 GSEKit 11122，未改名 |
| 60 | 100605 | `xiaolnwang`（测试改名后回滚） | 命中 GSEKit 11012 |
| 61 | 100605 | `xiaolnwang`（测试） | 命中 GSEKit 10589 |

**存量数据中不存在真实改过名的迁移产物，人工确认清单为 0，对齐可 100% 自动完成。**

这不改变设计：双策略与 CSV 覆盖机制全部保留。上述结论只说明本次存量执行不会触发人工路径，
而后续 104 个业务、1395 条模版的迁移仍可能产生改名场景，届时人工路径必须可用。

## 执行前待确认项

以下两项属于校验性质，不影响方案成立，但建议在实施前完成——它们会进一步收窄
`UNMATCHED_NATIVE` 判据的不确定性。

```sql
-- 1. 按 creator 分组，确认 203 条里迁移产物与自建各占多少
SELECT creator, COUNT(*) AS cnt, MIN(id) AS min_id, MAX(id) AS max_id,
       MIN(created_at) AS first_created, MAX(created_at) AS last_created
FROM config_templates
GROUP BY creator
ORDER BY cnt DESC;

-- 2. biz 5016793 在 GSEKit 有 5 条（11124-11128），BSCP 有 5 条 dathpan 创建的记录，
--    数量吻合。取 revision_name 里的数字去 GSEKit 反查（格式匹配本身不能作为判据），
--    能反查到且业务一致才说明是迁移产物，同时验证版本 ID 回溯链在真实数据上可用。
SELECT ct.id, ct.biz_id, ct.name, ct.creator, ct.created_at,
       tr.id AS revision_id, tr.revision_name, tr.created_at AS rev_created_at
FROM config_templates ct
JOIN template_revisions tr ON tr.template_id = ct.template_id
WHERE ct.biz_id = 5016793
ORDER BY ct.id, tr.id;
```

复核用的原始查询：

```sql
-- GSEKit 源库
SELECT TABLE_NAME, AUTO_INCREMENT
FROM information_schema.TABLES
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'gsekit_configtemplate';

SELECT COUNT(*) AS total, MIN(config_template_id) AS min_id, MAX(config_template_id) AS max_id
FROM gsekit_configtemplate;

SELECT bk_biz_id, COUNT(*) AS cnt,
       MIN(config_template_id) AS min_id, MAX(config_template_id) AS max_id
FROM gsekit_configtemplate
GROUP BY bk_biz_id
ORDER BY max_id DESC;

-- BSCP 目标库
SELECT COUNT(*) AS total, MIN(id) AS min_id, MAX(id) AS max_id FROM config_templates;
SELECT resource, max_id FROM id_generators WHERE resource = 'config_templates';

SELECT id, biz_id, name, creator, reviser, created_at, updated_at
FROM config_templates
WHERE reviser <> creator OR updated_at > created_at + INTERVAL 2 SECOND
ORDER BY biz_id, id;
```

## 测试

- **单元测试**：映射分类逻辑（六种分类各自的判定）、版本名 `^v(\d+)$` 解析、
  `task_data` JSON 改写、两阶段搬迁的 ID 分配。
- **集成测试**：用 mock 双库数据构造覆盖各分类的场景，跑 dry-run 校验报告，
  跑 execute 后校验阶段 4 的四项断言，再跑 rollback 校验数据回到初始状态。
- **幂等测试**：连续执行两次 `--execute`，第二次应为空操作且阶段 4 全部通过。
- **回归测试**：改造后的 migrate 流程对同一份 mock 数据迁移，断言
  `config_templates.id == gsekit.config_template_id`，且 `templates` / `template_revisions`
  的 ID 仍由 `id_generators` 分配。
