# gsekit 表达式搜索进程逻辑整理

## 基本信息

| 字段 | 值 |
|------|-----|
| 需求 ID | 1020451610135756804（短 ID：135756804） |
| 需求名称 | gsekit 表达式搜索进程逻辑整理 |
| 优先级 | High |
| 父需求 | 1020451610135732990（进程配置管理插件优化） |
| 创建时间 | 2026-07-02 20:42:23 |
| 原始需求文档 | docs/reqs/gsekit进程过滤逻辑梳理.md |

> 本需求为研究/整理型需求，产出物为「gsekit 进程过滤（表达式搜索）逻辑梳理文档」，
> 作为父需求「进程配置管理插件优化」中 bscp 对齐实现的前置输入，不含 bscp 侧的技术实现方案。

## 需求背景

### 业务背景

bscp（蓝鲸基础配置平台）在「进程配置管理」场景下需要对进程列表进行搜索/过滤。当前
bscp 仅支持按字段（集群、模块、服务实例、进程别名、cc 进程 ID、内网 IP 等）做**等值枚举
匹配**，不支持通配符、范围、枚举组合等复杂表达式。

gsekit（进程配置管理，bk-process-config-manager）在相同场景下提供了更强的过滤能力：

- **页面操作路径**：用户在界面上勾选具体的集群/模块/服务实例/进程，前端传入具体 ID 列表，
  后端按等值枚举过滤。
- **插件请求路径**：调用方（插件/API）传入「表达式」，后端解析表达式并匹配进程，支持通配符、
  数字/字母范围、枚举列表、排除、切片等复杂匹配。

两条路径在页面场景下效果一致（都归一到进程 ID 列表后再做 DB 过滤），区别在于：
- gsekit 页面/插件最终定位进程使用的是 **CC 进程 ID（bk_process_id）**；
- bscp 页面定位进程使用的是 **bscp 自增主键 ID**（DB 侧另存有 cc_process_id 字段）。

为让 bscp 支持与 gsekit 等价的表达式搜索能力（尤其是插件请求下的复杂匹配），需要先完整
梳理 gsekit 当前的过滤逻辑，作为后续设计与实现的依据。

### 用户故事

作为 bscp 的研发人员
我想要一份完整、准确的 gsekit 进程过滤逻辑梳理文档
以便于在实现 bscp 表达式搜索能力时，能够对齐 gsekit 的匹配语义与行为。

作为使用进程配置管理插件的调用方
我想要 bscp 后续支持与 gsekit 一致的表达式搜索
以便于用同一套表达式在 bscp 上筛选进程，降低迁移与理解成本。

### 需求来源

- **需求渠道**：技术优化 / 父需求「进程配置管理插件优化」拆分
- **关联需求**：父需求 1020451610135732990
- **参考资料**：gsekit 源码（本仓库 `bk-process-config-manager/` 目录）
  - `apps/gsekit/process/views/process.py`（进程查询/操作接口入口）
  - `apps/gsekit/process/handlers/process.py`（过滤核心逻辑 `ProcessHandler.list`）
  - `apps/gsekit/process/serializers/process.py`（请求参数定义）
  - `apps/gsekit/utils/expression_utils/`（表达式解析与匹配：`parse.py`/`match.py`/`serializers.py`/`range2re.py`）
  - `apps/gsekit/constants.py`（`EXPRESSION_SPLITTER`）

## 梳理结论：gsekit 进程过滤逻辑

> 本章为需求的核心产出，完整描述 gsekit 现有的两条过滤路径、表达式语法与匹配流程。

### 一、总览：两条过滤路径

gsekit 进程列表查询（`process_status`）与进程操作（`operate_process`）共用同一套过滤参数
（`ProcessFilterBaseSerializer`），入口方法为 `ProcessHandler.list(...)`。核心过滤有两条
互斥的范围来源：

| 路径 | 参数 | 典型来源 | 匹配方式 |
|------|------|---------|---------|
| 路径一：可 DB 筛选范围 | `scope` | 页面操作（用户勾选具体节点，传 ID） | DB 等值枚举 `IN` 过滤 |
| 路径二：表达式范围 | `expression_scope` | 插件/API 请求（传表达式） | 表达式解析后匹配进程，再归一为进程 ID |

**优先级规则**：`scope` 与 `expression_scope` 同时传入时，**优先使用 `scope`**；两者至少必须
传其一（`ProcessFilterBaseSerializer.validate` 强制校验）。

**空值语义差异**（关键边界）：
- `scope` 中某字段为空列表 = **全选**（不加该维度过滤条件）。
- `expression_scope` 解析后得到的进程列表为空 = **无可选数据**，直接返回空查询集
  （`Process.objects.none()`）。

### 二、路径一：scope（页面操作，DB 等值过滤）

`scope`（`ScopeSerializer`）字段：

| 字段 | 含义 | 类型 |
|------|------|------|
| `bk_set_env` | 环境类型（测试/体验/正式），默认正式 | 枚举 |
| `bk_set_ids` | 集群 ID 列表 | int[] |
| `bk_module_ids` | 模块 ID 列表 | int[] |
| `bk_service_ids` | 服务实例 ID 列表 | int[] |
| `bk_process_names` | 进程别名列表 | str[] |
| `bk_process_ids` | 进程 ID（CC 进程 ID）列表 | int[] |

`ProcessHandler.list` 将上述字段映射为 Django ORM 的 `__in` / 等值条件（`bk_set_id__in`、
`bk_module_id__in`、`service_instance_id__in`、`bk_process_name__in`、`bk_process_id__in`、
`bk_set_env`），仅对**非 None 且非空列表**的字段追加过滤条件，逻辑与（AND）叠加。

此外还有若干直接 DB 过滤维度（两条路径通用）：
- `bk_cloud_ids` → `bk_cloud_id__in`
- `bk_host_innerips` → `bk_host_innerip__in`
- `process_status` / `process_status_list` → 进程状态过滤
- `is_auto` / `is_auto_list` → 托管状态过滤

### 三、searches（附加模糊查询，两条路径通用）

`searches` 是一个字符串列表，用于对**内网 IP 与云区域名称**做模糊匹配，多个 search 之间
**逻辑与（AND）**叠加。每个 search 的匹配条件为：

```
Q(bk_host_innerip__contains=search) | Q(bk_cloud_id__in=<云区域名称包含 search 的云区域 ID>)
```

即：内网 IP 包含该串 **或** 云区域名称包含该串。这是 gsekit 里唯一对 IP/云区域生效的
「包含」式模糊搜索，不涉及集群/模块/进程名。

### 四、路径二：expression_scope（插件请求，表达式匹配）

#### 4.1 进程的 expression 字段

每个进程在同步（`sync_biz_process`）时会生成一个固定格式的 `expression` 字段，由 5 段用
分隔符 `EXPRESSION_SPLITTER`（值为 `<-GSEKIT->`）拼接：

```
{bk_set_name}<-GSEKIT->{bk_module_name}<-GSEKIT->{service_instance_name}<-GSEKIT->{bk_process_name}<-GSEKIT->{bk_process_id}
```

即：集群名 → 模块名 → 服务实例名 → 进程别名 → CC 进程 ID。

#### 4.2 expression_scope 请求结构

`expression_scope`（`ExpresssionScopeSerializer`）与 expression 五段一一对应，每段是一个
**独立表达式字符串**，缺省均为 `*`（匹配任意）：

| 字段 | 含义 | 缺省 |
|------|------|------|
| `bk_set_env` | 环境类型（必填） | - |
| `bk_set_name` | 集群名称表达式 | `*` |
| `bk_module_name` | 模块名称表达式 | `*` |
| `service_instance_name` | 服务实例名称表达式 | `*` |
| `bk_process_name` | 进程别名表达式 | `*` |
| `bk_process_id` | 进程 ID 表达式（支持切片语法） | `*` |

请求示例（来自 mock_data）：

```json
{
  "bk_set_env": "3",
  "bk_set_name": "[管控平台, PaaS平台]",
  "bk_module_name": "*",
  "service_instance_name": "*",
  "bk_process_name": "*",
  "bk_process_id": "4[6, 8, 9]"
}
```

含义：正式环境下，集群名为「管控平台」或「PaaS平台」，任意模块/服务实例/进程别名，且
CC 进程 ID 为 46、48、49 的进程。

#### 4.3 表达式 → 进程 ID 的匹配流程（expression_scope_to_scope）

1. 取业务下、指定 `bk_set_env` 的所有进程的 `expression` 列表，并建立 `expression → bk_process_id` 映射。
2. **切片语法单独处理**：若 `bk_process_id` 段命中切片模式 `[a:b]`（`SLICE_PATTERN`），
   先把切片表达式提取出来，并将该段临时置为 `*`（切片在匹配后再对结果列表生效）。
3. 用 `gen_expression` 将 `expression_scope` 五段按同一分隔符 `<-GSEKIT->` 拼成**一条完整
   表达式**。
4. `match.list_match(所有进程 expression 列表, 完整表达式)`：返回匹配成功的 expression 子集。
5. 通过映射把匹配到的 expression 换回 `bk_process_ids`。
6. `execute_slice(bk_process_ids, 切片表达式)`：对结果列表做 Python 列表切片（如 `[0:10]`、
   `[-5:]`）。

最终得到 `{bk_set_env, bk_process_ids}`，回到与路径一相同的 DB 过滤流程。

#### 4.4 表达式匹配内核（match / list_match）

采用**两层解析**（详见第五章）：

- 第 1 层 `parse_exp2unix_shell_style(expression)`：将表达式中的 `[...]` 语法**展开**为若干条
  Unix shell 风格候选串（笛卡尔积展开，范围/排除会产出 fnmatch 字符集）。
- 第 2 层用标准库 `fnmatch` 对候选串做匹配：`match` 判断单个是否命中，`list_match` 返回命中
  子集（并保持相对原列表的顺序）。

### 五、表达式语法规范（两层解析机制）

> **关键认知**：gsekit 的表达式语法**不是一张写死的白名单**，而是分两层协作。理解这两层，
> 才能准确判断"某个写法到底会怎么匹配"，也是 bscp 对齐实现时最容易踩坑的地方。

#### 5.1 第 1 层：`[...]` 方括号预处理（gsekit 自定义，`parse.py`）

`parse_exp2unix_shell_style_main` 从左到右扫描表达式中的 `[...]` 块并展开成**多条候选串**：

1. 每个 `[...]`：**块外文本**（上一块结束到当前块之间）作为前缀，**块内内容**交给
   `parse_enum_expression` 展开为一个值列表；
2. 前缀 × 值列表拼接；多个 `[...]` 块之间做**笛卡尔积**；
3. 补上末尾的块外文本，得到一个**字符串列表**（每条都是一条完整候选模式）。

块内内容由 `get_match_type` 判定为 5 种 `MatchType` 之一（判定优先级：枚举 `[...]` >
排除 `!` > 逗号词列表 > 连字符范围 > 普通词）。

> **重要**：第 1 层展开出来的产物**不一定是纯字面量**——范围/排除会**主动产出 fnmatch 字符集**，
> 故意留给第 2 层再解释。块外的 `*`/`?` 第 1 层完全不碰，原样保留。

`[...]` 展开产物对照：

| `[]` 内容 | 判定类型 | 第 1 层展开产物 | 第 2 层 fnmatch 语义 |
|-----------|---------|----------------|---------------------|
| `[6, 8, 9]` | 逗号词列表 | `6` / `8` / `9`（纯字面量，自动 strip 空格） | 普通字符 |
| `[管控平台, PaaS平台]` | 逗号词列表 | `管控平台` / `PaaS平台` | 普通字符串（整词） |
| `[1-1000]` | 数字范围 | `[1-9]`、`[1-9][0-9]`…（`range2re` 生成，**保留**方括号） | 一组字符集正则片段 |
| `[a-f]` | 单字符字母范围 | `[a-f]`（**保留**方括号） | 字符集，匹配单个 a~f |
| `[!ab]` | 排除 | `[!ab]`（**保留**方括号） | 排除字符集，匹配非 a/b 的单字符 |
| `[ab]` | 普通词（无逗号/范围/`!`） | `ab`（当**字面量**拼进去） | 普通字符串，**不是** fnmatch 字符集 |

范围判定细则：
- 数字范围需 `begin`/`end` 均为十进制且 `int(begin) < int(end)`；由 `range2re` 按位切割成
  可正则化区间，逐位生成 `[x-y]`。
- 字母范围需为单字符、同大小写、`ord(begin) < ord(end)`。
- 不满足范围条件的 `a-b`（如 `foo-bar`）退化为普通词字面量。

#### 5.2 第 2 层：`fnmatch` 兜底匹配（`match.py`）

第 1 层展开+拼接后的每条候选串，逐一交给 Python 标准库 `fnmatch` 匹配：
- `match(name, expression)`：任一候选串命中即算命中。
- `list_match(names, expression)`：返回命中子集，并保持相对原列表的顺序。

因此 `fnmatch` 原生语法（`*`、`?`、`[seq]`、`[!seq]`）在**方括号之外**均可直接使用，也支持
组合写法（如 `pro*c?`）。

#### 5.3 常用语法速查

| 语法 | 示例 | 归属层 | 说明 |
|------|------|--------|------|
| 通配符 `*` | `proc*` | 第 2 层 | 匹配任意长度任意字符 |
| 通配符 `?` | `proc?` | 第 2 层 | 匹配任意单个字符 |
| 枚举列表 `[w1, w2]` | `[管控平台, PaaS平台]` | 第 1 层 | 匹配列表中任一整词（逗号分隔） |
| 数字范围 `[a-b]` | `[1-1000]` | 第 1 层→第 2 层 | 范围内任意整数 |
| 字母范围 `[a-z]` | `[a-f]` | 第 1 层→第 2 层 | 单字符字母范围 |
| 排除 `[!seq]` | `[!ab]` | 第 1 层→第 2 层 | 匹配非 seq 的单字符 |
| 前缀 + 枚举组合 | `4[6, 8, 9]` | 第 1 层 | 展开为 `46`/`48`/`49`（笛卡尔积拼接） |
| 切片 `[a:b]` | `[0:10]`、`[-5:]` | 独立（非上述两层） | **仅 `bk_process_id` 段**，匹配前提取、匹配后对结果列表切片 |

> **易踩坑**：想"匹配单个字符 a 或 b"不能写 `[ab]`（会被第 1 层当字面量 `ab`），需写枚举
> `[a, b]`（整词匹配 `a`/`b`）或单字符范围 `[a-b]`（fnmatch 字符集）。切片 `[a:b]` 与
> 枚举/字符集共用方括号，但走的是独立的 `SLICE_PATTERN` 分支，且只对进程 ID 段生效。

#### 5.4 异常

表达式解析异常会抛出 `ExpressionSyntaxException` / `ExpressionParseException` /
`ExpressionSliceException`（详见 `expression_utils/exceptions.py`）。

### 六、scope ⇄ expression_scope 相互转换

gsekit 提供两个方向的转换（`ProcessHandler` 内）：

- `scope_to_expression_scope(scope)`：把「可 DB 筛选范围」（ID 列表）转为「表达式范围」。
  借助 `parse_list2expr` 把 ID/名称列表压缩为表达式：单值直接返回该值；多值压缩为
  `[v1,v2,...]`，且连续数字会压缩为区间 `a-b`（`compressed_list`）。空列表转为 `*`。
- `expression_scope_to_scope(expression_scope)`：即 4.3 描述的表达式 → 进程 ID 反解。

这套双向转换是「页面路径」与「插件路径」效果一致的实现基础：页面勾选的具体 ID 可转为等价
表达式，插件传入的表达式也可反解为进程 ID 列表。

## bscp 现状对照（背景参考）

> 仅为帮助理解差异，非本次整理的产出重点；后续设计需求另行细化。

bscp 进程列表查询入口 `ListProcess` → `dao.Process().List` → `handleSearch`，过滤条件封装在
`ProcessSearchCondition`：

| 维度 | gsekit | bscp | 匹配方式差异 |
|------|--------|------|-------------|
| 集群 | `bk_set_ids` / `bk_set_name` 表达式 | `sets`（集群名称） | bscp 仅 `IN` 等值 |
| 模块 | `bk_module_ids` / `bk_module_name` 表达式 | `modules` | bscp 仅 `IN` 等值 |
| 服务实例 | `bk_service_ids` / `service_instance_name` 表达式 | `service_instances` | bscp 仅 `IN` 等值 |
| 进程别名 | `bk_process_names` / `bk_process_name` 表达式 | `process_aliases` | bscp 仅 `IN` 等值 |
| 进程 ID | `bk_process_ids`（CC 进程 ID）/ 表达式 + 切片 | `cc_process_ids`（CC 进程 ID） | bscp 仅 `IN` 等值，无切片 |
| 内网 IP | `searches` 模糊 + `bk_host_innerips` | `inner_ips` | bscp 仅 `IN` 等值 |
| 环境 | `bk_set_env` | `environment` | 一致（等值） |
| 进程/托管状态 | `process_status(_list)` / `is_auto(_list)` | `process_statuses` / `managed_statuses` | 一致（等值枚举） |

核心差异：
1. **匹配能力**：bscp 全部字段为等值枚举 `IN`；gsekit 在插件路径支持通配符、范围、枚举、
   排除、切片等表达式匹配。
2. **进程标识**：gsekit 页面/插件统一以 CC 进程 ID（`bk_process_id`）定位进程；bscp 页面
   以 bscp 自增主键定位，DB 另存 `cc_process_id`。
3. **表达式载体**：gsekit 用「五段拼接 + 分隔符」的 expression 字段承载多维匹配；bscp 目前
   无等价的表达式字段/解析器。

## 边界范围

### 本期包含

- 完整梳理 gsekit 进程过滤的两条路径（scope 等值 / expression_scope 表达式）。
- 梳理 gsekit 表达式语法规范（通配符、枚举、范围、排除、切片、组合）及匹配流程。
- 梳理 searches 附加模糊、scope⇄expression_scope 转换、优先级与空值语义等关键规则。
- 输出 bscp 现状与 gsekit 的差异对照（背景参考）。

### 本期不包含

- bscp 侧表达式搜索的技术实现方案、接口设计、数据模型改造。
- 前端交互设计。
- gsekit 进程状态同步、进程实例生成、进程操作（start/stop 等）等与「过滤/搜索」无关的逻辑。

## 验收标准

### 功能验收

- [ ] **AC-001**：Given 梳理文档已产出，When 研发查阅两条过滤路径章节，Then 能明确 scope
  与 expression_scope 的入参字段、匹配方式、优先级与空值语义。
- [ ] **AC-002**：Given 梳理文档已产出，When 研发查阅表达式语法章节，Then 能说明两层解析
  机制（`[...]` 预处理 + `fnmatch` 兜底），并对照示例（如 `4[6, 8, 9]`、`[1-1000]`、`[!ab]`、
  `[ab]` 字面量陷阱、`[0:10]` 切片）说明每种写法的展开与匹配结果。
- [ ] **AC-003**：Given 梳理文档已产出，When 研发查阅匹配流程章节，Then 能复述
  expression_scope → 进程 ID 的完整 6 步流程（含切片单独处理）。
- [ ] **AC-004**：Given 梳理文档已产出，When 研发查阅 bscp 现状对照，Then 能列出 bscp 与
  gsekit 在匹配能力、进程标识、表达式载体三方面的差异。

## 约束条件

- 梳理对象为本仓库 `bk-process-config-manager/` 目录内的 gsekit 源码，以源码实现为准。
- 表达式语法术语（`EXPRESSION_SPLITTER`、`fnmatch`、`bk_process_id` 等）保持原样，不翻译改名。

## 未解决问题

| 问题 ID | 问题描述 | 状态 |
|--------|---------|------|
| Q-001 | bscp 后续采用哪种表达式载体（沿用五段拼接 expression，还是分字段独立表达式）由后续设计需求决定 | 待后续设计 |
| Q-002 | bscp 是否需要完整支持切片语法（`[a:b]`）及排除语法（`!`），或仅支持通配符/枚举/范围子集 | 待后续设计 |

---

## 原需求描述

> TAPD 原始需求描述为空（无描述内容）。需求名称：gsekit 表达式搜索进程逻辑整理。
> 补充口头背景（来自需求提出人）：
> - bscp 需要实现类似 gsekit 相同的表达式搜索逻辑；
> - bscp 当前仅支持按名称搜索且是等值匹配；
> - 页面操作时两者逻辑虽有区别但效果一致：bscp 传入 bscp 自增 ID，gsekit 传入 CC 进程 ID；
> - 若请求来自插件，gsekit 支持更复杂的匹配逻辑，因此需要先梳理 gsekit 当前的过滤逻辑。

## 澄清记录

### 第 1 轮澄清

**提问时间**：2026-07-02 21:00

**Agent 提问**：
1. gsekit 过滤逻辑的权威来源从哪获取？
2. 本次「整理」的范围边界覆盖哪些？
3. 本需求的产出物形态是什么？
4. 「表达式搜索」在 gsekit 语境下具体指什么？

**用户回复**：
1. 由用户提供 gsekit 源码仓库（放置于本仓库 `bk-process-config-manager/` 目录），以源码为准。
2. 同时梳理「页面操作」等值匹配 +「插件请求」表达式匹配两条路径。
3. 一份 gsekit 过滤逻辑梳理文档（含匹配规则、优先级、示例）。
4. 具体形态待梳理厘清 —— 经梳理确认为：Unix shell 风格通配符 + 枚举/范围/排除/切片组合表达式，
   承载于五段拼接的 expression 字段（详见「梳理结论」章节）。

### 第 2 轮澄清

**提问时间**：2026-07-02 21:48

**用户提问**：表达式是否固定只支持语法表里那几种？`[]` 内是否是 gsekit 自己的解析逻辑，
展开拼接后再交给 fnmatch 匹配？

**Agent 结论**：不是固定白名单，而是两层协作机制——第 1 层 `[...]` 预处理（gsekit 自定义，
支持逗号列表/数字范围/字母范围/排除/字面量，其中范围与排除会主动产出 fnmatch 字符集），
第 2 层 `fnmatch` 兜底（`*`/`?`/`[seq]`/`[!seq]`）。据此重写第五章为「两层解析机制」并修正
原 `[seq]` 字符集透传的错误描述（`[ab]` 实为字面量 `ab`，非字符集）。
