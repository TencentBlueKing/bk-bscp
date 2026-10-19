# CMDB Resource Watch 优化方案（已实施）

> 目标文件：`cmd/data-service/service/crontab/cmdb_resource_watcher.go`
> 配套代码：
> - 配置结构与校验：`pkg/cc/types.go`（`WatchCmdbResourceConfig`，L1365 起）
> - 配置示例：`cmd/data-service/etc/data_service.yaml`（`watchCmdbResource` 段，L193-232）
> - 长轮询客户端：`internal/components/client.go`（`GetLongPollingClient`）、`internal/components/bkcmdb/bkcmdb.go`（`ResourceWatchLongPolling`）
> - 服务装配：`cmd/data-service/app/app.go`（L597-601）
>
> **文档状态**：本方案已在代码中实施完毕。下文第 1-3 节描述优化前的现状与 CC 接口特性（背景），第 4-8 节描述当前代码的实际实现，并在涉及与原方案伪代码存在差异的地方显式标注「实现差异」。

## 1. 背景与问题（优化前）

优化前 `cmdbResourceWatcher` 用于监听 CC（bk-cmdb）资源（set / module / process）变更事件，并同步本地 DB 与 GSE。旧实现存在两个核心问题：

### 1.1 更新不及时（延迟 10s 甚至更久）

旧代码链路：

```text
Run() -> 10s ticker -> watchCMDBResourcesByTenant() -> watchResourcesForTenant()
      -> 顺序遍历 [set, module, process] 逐个 watchCMDBResources()
      -> 每个资源内部 drain 循环（消费完积压事件后退出）
```

延迟来源分析：

| # | 来源 | 说明 |
|---|------|------|
| 1 | **固定 10s 轮询间隔**（`defaultResourceWatcherTime` 硬编码） | 即使无积压，任何变更最快也要等下一个 tick 才被发现 |
| 2 | **租户 × 资源串行 watch** | 多租户模式下 `for tenant { for resource { ... } }` 全串行，前面任一资源的 drain 慢（如 process 事件多、同步耗时），会推迟后面所有资源/租户的 watch 开始时间 |
| 3 | **事件处理同步阻塞 watch 循环** | `handleSetEvent`/`handleProcessCreateEventsBatch` 等在 watch 循环内同步执行 DB 写 + GSE 任务分发，处理慢直接拉长整轮周期 |
| 4 | **游标丢失时回溯 1 小时**（`now - 1h`） | 首次启动或游标过期（1103007）重置后，要重放 1 小时的事件，期间新事件只能排队，进一步放大延迟 |
| 5 | **进程同步依赖外部调用** | `SyncBizProcessesByIDsWithMode` 会回调 CC 查询接口，慢查询阻塞后续事件消费 |

### 1.2 并发不可控

- 旧实现完全没有并发：对 CC 的 watch 请求、事件处理、GSE 任务全部串行。
- 并发化后必须对 CC 侧压力可控，否则 watch 循环加速 + 多租户 + 多资源叠加，可能在 CMDB 积压回溯时打出大量请求。

## 2. CC Resource Watch 接口特性

参考 CC v3.8+ 的 `resource_watch` 接口（主机事件监听、主机关系事件监听、业务事件监听、集群事件监听、模块数据监听、进程数据监听、模型实例事件监听等，按资源类型分权限）：

- **游标回溯**：通过 `bk_cursor` 从上次位置继续消费；支持 `bk_start_from`（unix 秒）按时间回溯；**仅支持 3 小时内的事件**（时长可能调整，不可依赖）。
- **事件类型过滤**：`bk_event_types` 可选 create/update/delete，事件中含全量数据。
- **字段裁剪**：`bk_fields` 指定返回字段（host 必填，host_relation 可空），满足轻量级 watch。
- **短长链设计**：带游标 watch 且无事件时，服务端保持会话连接，**20s 内有变更则直接推回**；超时则返回空事件（`bk_watched=false` 或 `bk_event_type` 为空的 keep-alive 事件）。
- **批量返回**：单次最多返回 200 条事件（`WatchData.BkEvents`），提升吞吐。
- **游标过期**：返回错误码 1103007（事件节点不存在），需要重置游标。

对应的协议类型定义在 `internal/components/bkcmdb/type.go`：

- `WatchResourceRequest`（L841-847）：`bk_resource`、`bk_event_types`、`bk_fields`、`bk_start_from`、`bk_cursor`。
- `WatchData`（L896-900）：`bk_watched` + `bk_events`（单页最大 200）。
- `BkEventObj`（L902-908）：`bk_cursor`、`bk_resource`（`ResourceType`）、`bk_event_type`（`EventType`：create/update/delete）、`bk_detail`（`json.RawMessage`，随资源类型不同而不同，通过 `Decode()` 解析为 `SetInfo`/`ModuleInfo`/`ProcessInfo`）。
- 资源类型常量（L958-983）：本任务只使用 `ResourceSet`/`ResourceModule`/`ResourceProcess`。

> **已确认：部署的 CC 支持长轮询**。由此得出的两个关键结论：
>
> 1. **客户端只需一个永不退出的请求循环**：每次响应（无论是否有事件）都立即带着最新游标发起下一次请求。事件发生时 CC 即时推回，端到端延迟 ≈ CC 投递延迟 + 1 个 RTT，客户端侧无需任何轮询间隔。
> 2. **keep-alive 空事件的游标必须落库**：它代表 CC 当前事件流的最新位置。若不落库，重启后会从更早的游标重放事件（浪费但不丢数据）；若误把空事件当数据处理则会干扰业务逻辑。

> **客户端超时约束**：`bkcmdb.ResourceWatch` 原本走蓝鲸 API Gateway HTTP 请求，复用全局共享 resty 客户端（`internal/components.GetClient()`），其 `http.Client.Timeout` 硬编码 30s（`internal/components/client.go:38` 的 `timeout` 常量），且 `http.Client.Timeout` 是硬上限、请求级 ctx 无法突破。20s hold + 网络往返在 30s 内可以完成但余量很小——现已为 watch 新增专用长轮询客户端，见第 8 节。

## 3. 优化目标（现状已达成）

1. **准实时**：CC 侧变更到本地生效的延迟从"秒级 10s+"降到"事件到达后 1 个请求周期内"（目标 P99 < 3s，不含 CC 自身投递延迟）。
2. **并发可控**：watch 请求与事件处理并行化，但对 CC 的总请求压力可通过配置收敛（并发上限 + QPS 限流）。
3. **可配置**：扫描间隔、并发度、限流、回溯偏移、监听资源列表等全部走 `data_service.yaml`，不硬编码（仅常量兜底值保留在代码中作防御）。
4. **正确性不回退**：游标持久化语义保持"整页处理成功后才推进"；事件按资源内顺序处理，不乱序。

## 4. 总体架构（现行实现）

### 4.1 架构总览

```text
shutdown 信号
   │
Run() ── rootCtx（任务级父 ctx，shutdown 时取消一次）
   │    └─ interval（默认 1m）ticker ──► reconcileChains() 对账
   │
   ├─ (tenant1, set)      runWatchChain ──► fetchWatchPage（信号量 + QPS limiter）
   ├─ (tenant1, module)   runWatchChain        │ ResourceWatchLongPolling（专用客户端，可配超时）
   ├─ (tenant1, process)  runWatchChain        ▼
   ├─ (tenant2, set)      ...          页队列 pages（容量 2，背压）
   └─ ...                              ▼
                                processPages（每链一个 FIFO 处理器 goroutine）
                                       │ processPage（资源内保序；process 按连续同类型分批、按 biz 有限并发）
                                       ▼
                                persistCursor（整页成功后 UpsertConfig 落库）
```

核心代码结构（`cmd/data-service/service/crontab/cmdb_resource_watcher.go`）：

| 成员/函数 | 职责 |
|-----------|------|
| `cmdbResourceWatcher` | 任务主体：持有 dao、serviced（master 判定）、bkcmdb/gse 服务、taskManager、配置、`rate.Limiter`、并发信号量、链表 `chains` |
| `watchChain` / `watchChainState` | (tenantID, resource) 维度的链标识；state 持有 `cancel` 与 `done` channel |
| `Run()` | 注册 shutdown 通知、创建 rootCtx、按 interval ticker 周期做主从判定与链对账 |
| `reconcileChains()` | master 时补启缺失链/停掉失效链；slave 时全部停止 |
| `runWatchChain()` | 单链常驻长轮询循环 + 启动本链 FIFO 事件处理器 |
| `fetchWatchPage()` | 一次 watch 请求：信号量 + limiter 约束后调用 `ResourceWatchLongPolling` |
| `splitEvents()` | 拆分响应：keep-alive 空事件单独标记，提取页游标 |
| `processPages()` / `processPageWithRetry()` / `processPage()` | FIFO 串行处理、失败退避重试、按资源分派 |
| `handleEvent()` / `handleSetEvent()` / `handleModuleEvent()` | set/module 事件 → 本地 DB 更新或事务删除 |
| `handleProcessCreateEventsBatch()` / `handleProcessUpdateEventsBatch()` / `handleProcessDeleteEventsBatch()` | process 事件按 biz 分组批量同步 |
| `runBizGroups()` | errgroup 按 biz 有限并发执行 |
| `dispatchProcessStateSyncTasks()` | 进程状态同步任务派发 TaskManager |
| `loadCursorWithRetry()` / `persistCursor()` / `getResourceCursorKey()` | 游标加载与写穿落库 |

### 4.2 关键常量（`cmdb_resource_watcher.go:45-67`）

| 常量 | 值 | 含义 |
|------|----|------|
| `errCursorExpiredSign` | `"1103007"` | CC 游标过期错误码特征 |
| `errCursorMissingSign` | `"事件节点不存在"` | 同上的中文错误特征（双重匹配，见 `isCursorExpiredError`） |
| `pageRetryLimit` | `3` | 连续失败告警阈值；超过后继续退避重试，**不能跳过未处理事件** |
| `pageHandoffQueueLen` | `2` | watch 循环与事件处理器之间的页队列长度；队列满时 watch 循环阻塞产生背压，避免在 CC hold 会话期间无界积压未处理页 |
| `keepAliveThreshold` | `1s` | 判断 CC 是否 hold 长轮询连接的耗时阈值：响应无事件且耗时 < 该值视为 CC 未 hold（兼容短轮询环境），退避 `idleInterval` 后再续连 |
| `cursorExpiredAlarmThreshold` | `5` | 单位时间内连续游标过期的告警阈值，超过说明 CC 侧事件流异常 |
| `defaultMaxRequestBackoff` | `1m` | 请求失败退避上限 |
| `cmdbBizIDField` | `"bk_biz_id"` | 默认 bk_fields 的公共字段 |

## 5. 关键流程详解

### 5.1 启动与链对账：`Run()` + `reconcileChains()`

- `Run()`（L142-175）：注册 `shutdown.AddNotifier()`；创建**任务级 rootCtx**（一次创建、shutdown 时取消一次，作为所有链 ctx 的父节点）；解析 `interval`（非法回退 1m）后进入 ticker 循环——**启动后立即对账一次不等第一个 tick**，此后每个 tick 调用 `reconcileChains()`。收到 shutdown 信号时 `stopAllChains()` + `cancel()` 后退出。
- `reconcileChains()`（L178-262）：
  1. **主从判定**：`state.IsMaster()` 为 false 时停掉全部链并返回（slave 节点不消费事件）；master 切换由对账周期自然收敛，新 master 从 DB 游标续传，最多重放一页。
  2. **目标集合计算**：多租户模式（`cc.DataService().FeatureFlags.EnableMultiTenantMode`）下通过 `bkuser.ListEnabledTenants()` 拉取启用租户，目标 = 租户 × 资源笛卡尔积；单租户模式目标仅为资源列表（`tenantID` 为空串）。资源列表经 `resources()` 过滤非法配置（仅接受 set/module/process）。
  3. **链数上限截断**：链总数超过 `maxWatchChains`（默认 100）时，按 (tenantID, resource) 排序后**确定性保留前 N 个**，超限链不运行并打出告警（提示调大配置或减少 resources），避免多租户场景链数量失控（内存 + CC 空闲请求量）。
  4. **增量对账**：先停掉不在目标集合中的链（`cancel()` + 等待 `done`），再为缺失的链创建 `chainCtx`（派生自 rootCtx）并启动 `runWatchChain` goroutine。

### 5.2 单链常驻循环：`runWatchChain()`

单条 (tenant, resource) 链的启动准备（L315-350）：

- **ctx 组合了三层语义**（缺一不可）：
  1. 链级取消：`chainCtx` 派生自 rootCtx，shutdown 或对账停止时级联取消；
  2. `KitKey`：多租户模式下 `bkcmdb.doRequestWithClient` 通过 `kit.MustGetKit(ctx)` 读取租户 ID，ctx 缺失 kit 会 panic，因此 `kt.Ctx = kit.WithKit(chainCtx, kt)`；
  3. Incoming metadata（rid/租户）：`kt.InternalRpcCtx()` 注入，供请求日志关联与多租户链路使用。
- **游标加载**：`cursorKey = getResourceCursorKey(tenantID, resource)`（格式：`resource:<res>:cursor`，多租户时加 `<tenantID>-` 前缀）；`loadCursorWithRetry` 从 `config` 表加载，DB 错误每 1s 重试直至成功或 ctx 取消（记录不存在视为空游标）。
- **FIFO 处理器**：启动独立 goroutine 执行 `processPages`，通过容量为 `pageHandoffQueueLen(2)` 的 `pages` channel 收页；链退出时等待 `processorDone` 保证处理器排空。

主循环状态机（L357-447）：

```text
for {  // ctx 取消即退出
    resp, err := fetchWatchPage(...)          // 计时 startAt
    if err != nil {
        if ctx.Err() != nil → break
        if 游标过期(1103007/事件节点不存在) {
            consecutiveExpired++              // ≥5 次连续过期升级为 Errorf 告警
            cursor = ""                       // 仅重置请求游标【实现差异①，见下】
            continue                          // 下一轮用 bk_start_from = now - startFromOffset 回溯
        }
        请求失败 → 指数退避 1s→2s→4s→...→1m，退避期间可被 ctx 取消
        continue
    }
    requestBackoff = 0; consecutiveExpired = 0
    events, pageCursor, hasEmptyEvent := splitEvents(resp.BkEvents)

    if len(events) == 0 {                     // 空响应或纯 keep-alive
        if pageCursor 有效且变化 {
            pages <- watchPage{cursor: pageCursor}   // 无事件页，仅由处理器推进落库游标
            cursor = pageCursor
        }
        if !hasEmptyEvent || elapsed < 1s {   // CC 未 hold（短轮询环境）→ idle 退避，防紧密循环
            sleep idleInterval（clamp ≥ 1s）
        }
        continue                              // CC 已 hold → 立即续连
    }

    按 maxEventsPerPage 切块（每块游标取块内最后事件游标，最后一块取 pageCursor）
    逐块推入 pages（队列满则阻塞背压；ctx 取消则退出）
    cursor = pageCursor                       // 内存游标即刻推进，持久化由处理器完成
}
```

**实现差异①（与原方案伪代码）**：原方案中"游标过期重置后直接 continue、keep-alive 游标写穿落库"由 watch 循环自己完成；实际实现中**所有游标落库统一收敛到本链的 FIFO 处理器**——watch 循环只推进内存游标，keep-alive 游标作为"无事件页"投递进队列，游标过期时也只重置请求游标、不落库。这保证落库位置永远不会越过任何未处理（或未持久化成功）的页，彻底消除"跳页丢事件"的可能。

**keep-alive 判定细节（`splitEvents`，L487-499）**：遍历响应事件，遇到 `BkEventType == ""` 的事件即停止收集并标记 `hasEmptyEvent`；`pageCursor` 取最后扫描到的事件游标（即空事件游标，代表 CC 当前事件流位置）。CC 长轮询正常时（hold 至少 20s 超时返回），`elapsed >= keepAliveThreshold(1s)` 成立，不 sleep 直接续连；若 CC 未开启长轮询（立即返回空事件），`elapsed < 1s` 触发 `idleInterval` 退避，循环退化为受控短轮询。

### 5.3 单次请求：`fetchWatchPage()` 与三层压力控制

请求构造（L455-485）：

- `bk_resource` = 链资源；`bk_event_types = [create, update, delete]`；`bk_fields` 来自 `fieldsFor(resource)`——优先取配置 `fields`，未配置的资源回退代码默认值：set 为 `[bk_biz_id, bk_set_id, bk_set_name, bk_set_env, set_template_id]`，module 为 `[bk_biz_id, bk_set_id, bk_module_id, bk_module_name]`，**process 为空列表（等价于返回全部字段，因处理需要完整进程属性）**。
- 游标为空（首次启动或过期重置）时改用 `bk_start_from = now - startFromOffset`（默认 5m，替代旧实现 1h，显著缩短追赶窗口；仍受 CC 3 小时事件保留限制）；否则带 `bk_cursor`。

压力控制顺序（**实现差异②**：先信号量、后限流）：

1. **信号量 `maxConcurrency`**（默认 8）：`select { case c.semaphore <- struct{}{} }` acquire，`defer` 释放。goroutine 数可以远多于信号量额度，但同时在途的 watch 请求不超过该值；由于每个长轮询请求占用 CC 侧一个 hold 会话约 20s，该值实质上也是 **CC watch session 数上限**，需与 CC 侧连接容量对齐。
2. **QPS limiter `qpsLimit`**（默认 20，`rate.NewLimiter(limit, 1)`）：**取得并发槽之后再 `limiter.Wait`**——防止大量等待槽位的 goroutine 积攒令牌、槽位释放后同时发出突发请求。
3. **`idleInterval` 下限保护**：见 5.2，仅在 CC 未 hold 时生效，配置再小也 clamp 到 ≥ 1s。

最终调用 `c.cmdb.ResourceWatchLongPolling(ctx, req, c.requestTimeout())`，`requestTimeout` 非法或 ≤ 20s 时回退 35s。

### 5.4 事件处理：资源内 FIFO + process 按 biz 有限并发

#### 页处理与游标持久化（`processPages` / `processPageWithRetry`，L501-549）

- 每条链一个 FIFO 处理器 goroutine，从 `pages` 队列逐页取出处理，**同资源事件严格保序**。
- `processPageWithRetry`：`processPage` 失败 → 整页重试；处理成功但 `persistCursor`（`UpsertConfig` 写穿 DB）失败 → **跳过重复处理、仅重试落库**（`processed` 标记）；每次失败退避 1s 起步、翻倍至上限 1m。第 3 次（`pageRetryLimit`）失败时打出 "page stalled, retrying without skipping" 告警——**失败绝不跳页**，宁可阻塞消费也不能越过未处理事件；ctx 取消时直接返回（游标未推进，重启后重放）。

#### set / module 事件（`handleEvent` → `handleSetEvent` / `handleModuleEvent`，L743-868）

- 事件 `bk_detail` 解码为 `SetInfo` / `ModuleInfo`，`validCMDBID` 校验 `bk_biz_id`、`bk_set_id`（及 module 的 `bk_module_id`）——防止缺失、负数或溢出的 ID 被转换为数据库资源 ID。
- **update**：`Process().UpdateSelectedFields` 批量更新 process 表的 `set_name` / `module_name`（module 事件按 set_id + module_id 双条件过滤）。
- **delete**：开启事务，查出该集群/模块下全部进程 ID（`GetBySetIDWithTx` / `GetByModuleIDWithTx`），调用 `cmdb.MarkProcessDeletedAndCleanInstancesTx` 标记进程删除并清理服务实例，事务失败逐级回滚。
- create 事件与未知事件类型：仅打日志、不报错（create 无需本地动作；未知类型不阻断游标推进）。

#### process 事件批量同步（L551-587, 676-741, 870-1010）

- **分批规则（实现差异③）**：`processPage` 只把**连续同类型**的 process 事件合并为一个批次（不同类型交错时拆成多个批次），保证 create/update/delete 语义不因跨类型合并而混乱；set/module 则逐条处理。
- `groupProcessEventsByBiz`：解码 `ProcessInfo` 并校验 `bk_biz_id`、`bk_process_id`、`service_instance_id` 后按业务分组。
- **`runBizGroups`（L679-699）**：`errgroup.Group.SetLimit(HandlerConcurrency)`（默认 4）按 biz 有限并发执行；**刻意不使用 `errgroup.WithContext` 派生 ctx 中断兄弟任务**——单业务失败不影响其他业务执行完毕，整页失败由上游 `processPageWithRetry` 统一重试。`HandlerConcurrency` 的作用域是"单链内 process 批次的 biz 级并发"，跨资源并行来自每链独立 goroutine（**实现差异④**：原方案的"全局 handler worker pool"落地为"每链 FIFO 处理器 + 链内 biz 级并发"，既保序又免锁）。
- create / update：先检查 `svc.IsProcessConfigViewEnabled(bizID)`，未开启的业务直接跳过；然后 `cmdb.NewSyncCMDBService(...).SyncBizProcessesByIDsWithMode`（create，回调 CC 查询接口做全量同步）/ `UpdateProcess`（update），成功后 `dispatchProcessStateSyncTasks` 把进程状态同步任务派发给 TaskManager。
- delete：`collectDeleteProcessIDs` 按事件查询本地进程 ID（记录不存在则跳过），再按 biz 并发执行事务：`MarkProcessDeletedAndCleanInstancesTx` + `CheckAndMarkHostAliasConflicts`（检查主机别名冲突）+ commit。
- **GSE 任务派发的量控**（`dispatchProcessStateSyncTasks`，L721-741）：`taskManager.Dispatch` 仅为异步入队（machinery/etcd broker），实际执行由全局 worker 池（`defaultWorkerNum=200`）消费，入队速率不会直接打爆下游；单次派发量由 `maxEventsPerPage` 间接约束。

### 5.5 游标生命周期小结

| 场景 | 处理 |
|------|------|
| 链启动 | `config` 表加载（`resource:<res>:cursor`，多租户加租户前缀），DB 错误每 1s 重试 |
| 每页处理成功 | FIFO 处理器 `UpsertConfig` 写穿落库；失败仅重试落库，内存/DB 游标不越过该页 |
| keep-alive 空事件 | 游标作为"无事件页"经 FIFO 队列落库（推进消费位置，防重启重放） |
| 游标过期（1103007） | 仅重置请求游标为空 → 下一轮 `bk_start_from = now - startFromOffset`（默认 5m）回溯；连续过期 ≥ 5 次升级为错误告警（CC 事件流可能异常） |
| DB 写失败 | 告警 + 重试，不影响内存推进；重启后最多重放一页 |
| 优雅关闭 | 链 ctx 取消，watch 循环退出前 `<-processorDone` 等待处理器排空已入队页面 |

### 5.6 配置默认值与防御性兜底

配置加载链路（`pkg/cc/types.go`）：

- `CrontabConfig.validate()`（L1495）会调用 `WatchCmdbResource.validate()`（L1575-1633）：
  - 四个 duration（`interval` / `idleInterval` / `startFromOffset` / `requestTimeout`）非空时必须能解析且 > 0；
  - `requestTimeout` 非空时必须 **> 20s**（CC 长轮询 hold 窗口）；
  - `maxConcurrency` / `handlerConcurrency` / `maxWatchChains` / `maxEventsPerPage` / `qpsLimit` 必须 ≥ 0，且 `qpsLimit` 拒绝 NaN / ±Inf；
  - `resources` 枚举校验（合法集合 `watchCmdbResourceValidResources` 在 pkg/cc 本地定义，因 pkg/cc 不能反向 import `internal/components/bkcmdb`，避免 import cycle）。
- `trySetDefault()`（L1636-1703）：`interval=1m`、`idleInterval=1s`、`maxConcurrency=8`、`qpsLimit=20`、`handlerConcurrency=4`、`maxWatchChains=100`、`maxEventsPerPage=200`、`startFromOffset=5m`、`requestTimeout=35s`、`resources=[set,module,process]`、`fields` 默认 set/module 裁剪列表。
- **运行时兜底（`NewCmdbResourceWatcher`，L69-102）**：防止调用方绕过 `trySetDefault` 传入零值——`qpsLimit <= 0 / NaN / Inf` 回退 20；`maxConcurrency < 1` 回退 8（避免 limiter burst=0 请求被永久拒绝、信号量为 0 链静默卡死）；`handlerConcurrency < 1` 回退 4。此外 `interval`（回退 1m）、`idleInterval`（clamp ≥ 1s）、`startFromOffset`（回退 5m）、`requestTimeout`（回退 35s）、`maxWatchChains`（回退 100）、`maxEventsPerPage`（回退 200）在使用点也有独立的解析回退。

## 6. 配置示例（`cmd/data-service/etc/data_service.yaml`，实际生效内容）

```yaml
crontab:
  watchCmdbResource:
    # whether the watch cmdb resource task is enabled (default: false)
    enabled: false
    # tenant/resource scan period for discovering new tenants or resources (default: 1m)
    interval: 1m
    # fallback probe interval, only effective when CC does NOT hold the long-polling
    # connection; clamped to >= 1s (default: 1s)
    idleInterval: 1s
    # max number of concurrent watch requests to CC, also the CC watch session
    # limit (each long-polling request holds one CC session up to 20s; default: 8)
    maxConcurrency: 8
    # QPS limit for watch requests, mainly converging backlog catch-up bursts (default: 20)
    qpsLimit: 20
    # event handling worker concurrency across resources; events of the same
    # resource are always processed in FIFO order (default: 4)
    handlerConcurrency: 4
    # lookback offset when the cursor is lost or expired (default: 5m)
    startFromOffset: 5m
    # timeout of a single long-polling watch request, must > 20s (CC hold window; default: 35s)
    requestTimeout: 35s
    # CMDB resource types to watch (default: set/module/process)
    resources:
      - set
      - module
      - process
    # per-resource bk_fields trimming, optional; missing resource falls back to
    # the code default, empty list means all fields
    fields:
      set: ["bk_biz_id", "bk_set_id", "bk_set_name", "bk_set_env", "set_template_id"]
      module: ["bk_biz_id", "bk_set_id", "bk_module_id", "bk_module_name"]
    # max number of concurrently running watch chains (tenant x resource); chains
    # beyond the limit will NOT run, increase this or reduce resources when there
    # are many tenants (default: 100)
    maxWatchChains: 100
    # max events processed per batch; a page larger than this is split into
    # chunks (CC protocol caps one page at 200; default: 200)
    maxEventsPerPage: 200
```

配置项语义补充：

- `interval` 只控制**租户/资源发现与主从对账周期**，不参与事件消费节奏（长轮询模式下节奏由 CC hold 窗口决定）。
- `idleInterval` 仅在 CC 未 hold（响应耗时 < 1s 且无事件）时生效；CC 长轮询正常工作时此配置不参与节奏控制。
- `fields.process` 未配置时为空列表，即 process 事件返回全部字段（处理逻辑需要完整进程属性，一般不需要配置）。
- `maxWatchChains` 截断是确定性的（排序后保留前 N），被截掉的链不会运行且每次对账都会告警。

## 7. 装配与启停

- 装配（`cmd/data-service/app/app.go:597-601`）：`crontabConfig.WatchCmdbResource.Enabled` 为 true 时，`crontab.NewCmdbResourceWatcher(ds.daoSet, ds.sd, ds.cmdb, ds.gseSvc, ds.service, ds.taskManager, crontabConfig.WatchCmdbResource)` 并 `Run()`。
- 优雅关闭：`Run()` 内 `shutdown.AddNotifier()`，收到信号后停掉全部链（逐个 cancel 并等待 `done`）并取消 rootCtx；每条链退出前等待其 FIFO 处理器排空已入队页面。
- 多实例：仅 master 运行链；slave 停止全部链。master 切换后新 master 从 DB 游标续传，最多重放一页。

## 8. 长轮询专用客户端与超时

- **全局客户端**（`internal/components/client.go`）：`timeout = 30s` 硬编码（L38），`GetClient()` 单例初始化；`http.Client.Timeout` 是硬上限，请求级 ctx 无法突破，因此不能直接承载 > 20s 的长轮询。
- **长轮询客户端**（`GetLongPollingClient(requestTimeout)`，L222-237）：与全局客户端共享 `newRestyClient` 构建的中间件与鉴权逻辑，**仅超时可配置**；相同超时的客户端按需缓存复用（`longPollingClients` map + 互斥锁），并发初始化安全（`client_test.go` 的 `TestSharedClientsConcurrentInitialization` 覆盖：并发获取为同一实例、不污染全局客户端、全局超时不变）。
- **入口**（`internal/components/bkcmdb/bkcmdb.go:516-530`）：`ResourceWatchLongPolling(ctx, req, requestTimeout)` 走 `doRequestWithClient` 指定专用客户端，URL 与 `ResourceWatch` 一致（`watchResource` 路由）。契约：`requestTimeout` 必须 > 20s。
- 其余 CMDB 接口仍走全局 30s 客户端，行为不变。

## 9. 与其他 watcher 的关系

`pkg/cc/types.go:1428` 保留 TODO：`WatchBizHostRelation` 和 `WatchHostUpdates` 后续合并为 `WatchCmdbResource`。本期实现的链管理（reconcile/链表/cancel-done）、游标缓存（config 表 + FIFO 处理器）、三层压力控制（信号量 + QPS limiter + idle clamp）、退避与兜底逻辑均集中在 crontab 包内，host/host_relation 合并时可直接复用该框架，本期不实施合并。

## 10. 测试覆盖

| 测试 | 文件 | 覆盖点 |
|------|------|--------|
| `TestWatchCmdbResourceDefaultDisabled` / `CanBeEnabled` / `Defaults` / `KeepsConfiguredValues` / `ValidateRejectsInvalidValues`（含 requestTimeout 过短/等于 20s 边界、非法 duration、负数、NaN/Inf qpsLimit、非法 resource 枚举等） | `pkg/cc/types_test.go` | 配置默认值、透传与校验 |
| `TestWatcherHeartbeatDoesNotCommitPastFailedPage`、`TestWatcherDoesNotSkipFailedPageAfterRetryThreshold` | `cmd/data-service/service/crontab/cmdb_resource_watcher_test.go` | 失败页/keep-alive 游标不会越过未处理页推进 |
| `TestWatcherRetriesCursorBeforeNextPage` | 同上 | 游标持久化失败先重试再消费下一页 |
| `TestWatcherKeepsProcessEventOrder` | 同上 | 同资源 process 事件 FIFO 保序 |
| `TestWatcherLoadCursorPreservesDatabaseErrors` | 同上 | 游标加载区分 DB 错误与记录不存在 |
| `TestWatcherRejectsInvalidProcessIdentity` | 同上 | 非法/负数/溢出进程事件身份被拒绝 |
| `TestWatcherUpdateProcessPreservesDatabaseErrors` | 同上 | DB 错误向上传播触发整页重试 |
| `TestSharedClientsConcurrentInitialization` | `internal/components/client_test.go` | 长轮询客户端并发初始化与全局客户端隔离 |

## 11. 风险与对策（现行实现视角）

| 风险 | 现状对策 |
|------|----------|
| 长轮询请求占用 CC watch session 连接 | `maxConcurrency` 即 CC 侧 hold 连接数上限（默认 8），上线前需与 CC 侧确认连接容量；空闲流量极低（每 20s 一次/维度） |
| 链路中间层（API Gateway / LB）对长响应超时 < 20s，hold 期间被切断 | 上线前实测链路超时配置；被切断表现为请求失败，走指数退避重试（游标未推进，无数据丢失） |
| 全局共享 resty 客户端 30s 硬超时截断长轮询 | 已解决：watch 使用 `GetLongPollingClient` 专用客户端（超时可配置，默认 35s），不影响全局客户端 |
| watch 加速后 CC 压力上升 | 空闲流量本身极低；追积压突发由 `maxConcurrency` + `qpsLimit`（先信号量后限流）双重限制；`idleInterval` clamp 兜底短轮询退化场景 |
| 异步处理导致事件乱序 | 每链单 FIFO 处理器串行消费，页队列容量 2 形成背压；process 仅合并连续同类型事件；游标推进/落库严格在链内 |
| 处理失败丢事件 | 失败不推进持久化游标，整页退避重试（1s→2s→…→1m），第 3 次失败告警 "page stalled" 但不跳页 |
| 多实例重复消费 | 仅 master 运行链（对账周期收敛）；master 切换后从 DB 游标续传，最多重放一页 |
| 回溯窗口 5m 可能漏事件 | 5m 仅用于游标丢失/过期场景（罕见），可配置调大，仍受 CC 3 小时事件保留限制 |
| 租户很多链数量失控 | `maxWatchChains`（默认 100）确定性截断 + 告警提示 |
| 配置零值/非法值导致链卡死或请求被永久拒绝 | `validate()` 启动期拒绝（负数、≤ 20s 超时、NaN/Inf、非法枚举）；`trySetDefault()` 补默认值；`NewCmdbResourceWatcher` 及各使用点再做运行时兜底回退 |

## 12. 验收标准（对照实现）

1. CC 上修改集群/模块名称 → 本地 DB 中 `set_name`/`module_name` 更新延迟 P99 < 3s（CC 长轮询推回及时的前提下）。
2. 空闲状态下观察 (tenant, resource) 维度的 watch 请求频率约为 1 次 / 20s（keep-alive 空事件），空事件游标经 FIFO 处理器正常落库，重启后不重放已消费事件。
3. 模拟 1 万条积压事件，watch 追平期间对 CC 的并发请求数 ≤ `maxConcurrency`，QPS ≤ `qpsLimit`。
4. 删除游标记录重启服务，回溯窗口为 `startFromOffset` 且无事件丢失。
5. 配置非法值（`maxConcurrency: -1`、`idleInterval: 100ms`、`requestTimeout: 10s` 等）启动报错（`validate`）或运行时 clamp/回退，有明确日志。
6. 单测覆盖见第 10 节；`pkg/cc` 配置用例与 crontab 游标/保序/兜底用例均在 CI 中执行。
