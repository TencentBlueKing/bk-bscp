# CMDB Resource Watch 优化方案（已实施）

> 目标文件：`cmd/data-service/service/crontab/cmdb_resource_watcher.go`
> 配套代码：
> - 配置结构与校验：`pkg/cc/types.go`（`WatchCmdbResourceConfig`，L1367 起）
> - 配置示例：`cmd/data-service/etc/data_service.yaml`（`watchCmdbResource` 段，L199-221）
> - 长轮询客户端：`internal/components/client.go`（`GetLongPollingClient`）、`internal/components/bkcmdb/bkcmdb.go`（`ResourceWatchLongPolling`）
> - 服务装配：`cmd/data-service/app/app.go`（L603-608）
>
> **文档状态**：本方案已在代码中实施完毕，并随全量同步改造更新（2026-10）。下文第 1-3 节描述优化前的现状与 CC 接口特性（背景），第 4-8 节描述当前代码的实际实现，第 5.7 节描述进程全量同步（`sync_cmdb.go`）的删除语义与恢复机制。

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
- `BkEventObj`：`bk_cursor`、`bk_resource`（`ResourceType`）、`bk_event_type`（`EventType`：create/update/delete）、`bk_detail`（`json.RawMessage`，随资源类型不同而不同，通过 `Decode()` 解析；process 事件明细经 `bk_fields` 裁剪仅含 `bk_biz_id`）。
- 资源类型常量：本任务只使用 `ResourceModule`/`ResourceProcess`（set 已不再支持）。

> **已确认：部署的 CC 支持长轮询**。由此得出的两个关键结论：
>
> 1. **客户端只需一个永不退出的请求循环**：每次响应（无论是否有事件）都立即带着最新游标发起下一次请求。事件发生时 CC 即时推回，端到端延迟 ≈ CC 投递延迟 + 1 个 RTT，客户端侧无需任何轮询间隔。
> 2. **keep-alive 空事件的游标必须推进**：它代表 CC 当前事件流的最新位置。若不推进，重启后会从更早的游标重放事件（浪费但不丢数据）；若误把空事件当数据处理则会干扰业务逻辑。

> **客户端超时约束**：`bkcmdb.ResourceWatch` 原本走蓝鲸 API Gateway HTTP 请求，复用全局共享 resty 客户端（`internal/components.GetClient()`），其 `http.Client.Timeout` 硬编码 30s（`internal/components/client.go:38` 的 `timeout` 常量），且 `http.Client.Timeout` 是硬上限、请求级 ctx 无法突破。20s hold + 网络往返在 30s 内可以完成但余量很小——现已为 watch 新增专用长轮询客户端，见第 8 节。

## 3. 优化目标（现状已达成）

1. **准实时**：CC 侧变更到本地生效的延迟从"秒级 10s+"降到"事件到达后 1 个请求周期内"（目标 P99 < 3s，不含 CC 自身投递延迟）。
2. **并发可控**：watch 请求与事件处理并行化，但对 CC 的总请求压力可通过配置收敛（并发上限 + QPS 限流）。
3. **可配置**：扫描间隔、并发度、限流、回溯偏移、监听资源列表等全部走 `data_service.yaml`，不硬编码（仅常量兜底值保留在代码中作防御）。
4. **正确性不回退**：事件按资源内顺序串行处理，不乱序；全量同步幂等，重放无副作用。

## 4. 总体架构（现行实现）

### 4.1 架构总览

```text
shutdown 信号
   │
Run() ── rootCtx（任务级父 ctx，shutdown 时取消一次）
   │    └─ interval（默认 1m）ticker ──► reconcileChains() 对账
   │
   ├─ (tenant1, module)   runWatchChain ──► fetchWatchPage（信号量 + QPS limiter）
   ├─ (tenant1, process)  runWatchChain        │ ResourceWatchLongPolling（专用客户端，可配超时）
   ├─ (tenant2, module)   ...                  ▼
   └─ ...                            splitEvents 拆页（keep-alive 空事件单独标记）
                                          │
                                          ▼
                                    processPage（链内处理，资源内严格保序）
                                    ├─ module：逐条更新 process 表 module_name
                                    └─ process：整页按 biz 归并 → 投递 bizSyncQueue（不等待）

                                    bizSyncQueue（按租户，watcher 级 ctx，独立于链生命周期）
                                    └─ worker × handlerConcurrency：SyncBizProcesses 全量同步 → 派发状态同步任务
```

核心代码结构（`cmd/data-service/service/crontab/cmdb_resource_watcher.go`）：

| 成员/函数 | 职责 |
|-----------|------|
| `cmdbResourceWatcher` | 任务主体：持有 dao、serviced（master 判定）、bkcmdb/gse 服务、taskManager、配置、`rate.Limiter`、并发信号量、链表 `chains` |
| `watchChain` / `watchChainState` | (tenantID, resource) 维度的链标识；state 持有 `cancel` 与 `done` channel |
| `Run()` | 注册 shutdown 通知、创建 rootCtx、按 interval ticker 周期做主从判定与链对账 |
| `reconcileChains()` | master 时补启缺失链/停掉失效链；slave 时全部停止 |
| `resources()` | 返回固定的监听资源列表 `[module, process]`（不开放配置；set 已不支持） |
| `fieldsFor()` | 按资源返回固化在代码内的 bk_fields：module 为 `[bk_biz_id, bk_set_id, bk_module_id, bk_module_name]`，process 为 `[bk_biz_id]`（事件仅用于定位业务） |
| `eventTypesFor()` | module 仅订阅 update；process 订阅 create/update/delete（不区分消费） |
| `runWatchChain()` | 单链常驻长轮询循环：续连 + 拆页 + 同步处理，游标仅保存在内存并随响应推进 |
| `fetchWatchPage()` | 一次 watch 请求：信号量 + limiter 约束后调用 `ResourceWatchLongPolling` |
| `splitEvents()` | 拆分响应：keep-alive 空事件单独标记，提取页游标 |
| `processPage()` | 按资源分派：module 逐条、process 整页归并后投递队列 |
| `handleModuleEvent()` | module update 事件 → 按 set_id + module_id 更新 process 表 module_name |
| `groupProcessEventsByBiz()` | 解析事件明细 `bk_biz_id` 并按业务归并去重（坏事件跳过并累计错误） |
| `handleProcessEventsBatch()` | process 事件统一入口：按 biz 投递 `bizSyncQueue`，不等待同步完成 |
| `bizSyncQueue` / `newBizSyncQueue()` | 单租户同步执行队列：worker 数 = handlerConcurrency，同业务合并排队、执行期间再投递追加一轮 |
| `syncQueueFor()` / `stopQueuesExcept()` / `stopQueues()` | 按活跃 process 链租户懒创建队列；租户下线或被链上限淘汰时回收，进程退出时回收全部队列 |
| `dispatchProcessStateSyncTasks()` | 进程状态同步任务派发 TaskManager |

### 4.2 关键常量（`cmdb_resource_watcher.go`）

| 常量 | 值 | 含义 |
|------|----|------|
| `errCursorExpiredSign` | `"1103007"` | CC 游标过期错误码特征（内存游标正常运行不会过期，仅在链长时间停滞时命中） |
| `errCursorMissingSign` | `"事件节点不存在"` | 同上的中文错误特征（双重匹配，见 `isCursorExpiredError`） |
| `keepAliveThreshold` | `1s` | 判断 CC 是否 hold 长轮询连接的耗时阈值：响应无事件且耗时 < 该值视为 CC 未 hold（兼容短轮询环境），退避 `idleInterval` 后再续连 |
| `defaultMaxRequestBackoff` | `1m` | 请求失败退避上限（游标过期也走同一退避） |
| `cmdbBizIDField` | `"bk_biz_id"` | 默认 bk_fields 的公共字段 |

## 5. 关键流程详解

### 5.1 启动与链对账：`Run()` + `reconcileChains()`

- `Run()`：注册 `shutdown.AddNotifier()`；创建**任务级 rootCtx**（一次创建、shutdown 时取消一次，作为所有链 ctx 的父节点）；解析 `interval`（非法回退 1m）后进入 ticker 循环——**启动后立即对账一次不等第一个 tick**，此后每个 tick 调用 `reconcileChains()`。收到 shutdown 信号时 `stopAllChains()` + `cancel()` 后退出。
- `reconcileChains()`：
  1. **主从判定**：`state.IsMaster()` 为 false 时停掉全部链并返回（slave 节点不消费事件）；master 切换由对账周期自然收敛，新 master 的链从空游标开始，按 `startFromOffset` 回溯消费。
  2. **目标集合计算**：多租户模式（`cc.DataService().FeatureFlags.EnableMultiTenantMode`）下通过 `bkuser.ListEnabledTenants()` 拉取启用租户，目标 = 租户 × 资源笛卡尔积；单租户模式目标仅为资源列表（`tenantID` 为空串）。资源列表由 `resources()` 固定为 module/process。
  3. **链数上限截断**：链总数超过 `maxWatchChains`（默认 100）时，按 (tenantID, resource) 排序后**确定性保留前 N 个**，超限链不运行并打出告警（提示调大配置），避免多租户场景链数量失控（内存 + CC 空闲请求量）。
  4. **增量对账**：先停掉不在目标集合中的链（统一 `cancel()` + 在**共享** `chainStopWaitTimeout=10s` 预算内等待 `done`，总耗时不随链数线性增长），再为缺失的链创建 `chainCtx`（派生自 rootCtx）并启动 `runWatchChain` goroutine。超时未退出的链保留在 `chains` 并标记 `stopping`：确认 `done` 关闭后才回收，同 key 链在回收前不会重建。状态由唯一的管理 goroutine 维护，`sync.Once` 保证管理循环只启动一次。

### 5.2 单链常驻循环：`runWatchChain()`

单条 (tenant, resource) 链的启动准备：

- **ctx 组合了三层语义**（缺一不可）：
  1. 链级取消：`chainCtx` 派生自 rootCtx，shutdown 或对账停止时级联取消；
  2. `KitKey`：多租户模式下 `bkcmdb.doRequestWithClient` 通过 `kit.MustGetKit(ctx)` 读取租户 ID，ctx 缺失 kit 会 panic，因此 `kt.Ctx = kit.WithKit(chainCtx, kt)`；
  3. Incoming metadata（rid/租户）：`kt.InternalRpcCtx()` 注入，供请求日志关联与多租户链路使用。
- **游标**：**只保存在内存中，不落库**。链启动时游标为空，首轮请求用 `bk_start_from = now - startFromOffset`（默认 5m，上限 1h）回溯；进程重启后同样走这条路径。内存游标随每次响应推进，正常运行不会过期，只有链长时间停滞（超过 CC 3 小时事件保留期）才会命中 1103007，此时重置游标并回溯。落库失败只告警，内存游标照常推进。

主循环状态机：

```text
for {  // ctx 取消即退出
    resp, err := fetchWatchPage(...)          // 计时 startAt
    if err != nil {
        if ctx.Err() != nil → break
        // 内存游标正常运行不会过期，仅链长时间停滞才会命中
        if 游标过期(1103007/事件节点不存在) → cursor = ""  // 下一轮改用 bk_start_from 回溯
        请求失败 → 指数退避 1s→2s→4s→...→1m，退避期间可被 ctx 取消
        continue
    }
    requestBackoff = 0
    events, pageCursor, hasEmptyEvent := splitEvents(resp.BkEvents)

    // 页处理只做「解析 + 投递」：module 事件直接写库，process 事件投递 bizSyncQueue
    // 处理失败仅告警，游标照常推进（漏掉的变更由队列重试或全量同步兜底）
    processPage(ch, kt, events)               // 链内处理，资源内严格保序
    cursor = pageCursor

    if len(events) == 0 && (!hasEmptyEvent || elapsed < 1s) {
        // 空响应或纯 keep-alive 且 CC 未 hold（短轮询环境）→ idle 退避，防紧密循环
        sleep idleInterval（clamp ≥ 1s）
    }
    // CC 已 hold → 立即续连
}
```

**可靠性取舍（对齐 gsekit）**：事件处理失败只告警、游标照常推进，不做页级重试；游标只存在内存，重启后按 `startFromOffset` 回溯；漏掉的变更由队列重试、定时全量同步或一键全量同步兜底。watch 循环与页处理在同一 goroutine 内串行执行，天然保序、免锁；全量同步本身在 `bizSyncQueue` 的 worker 中执行，链只负责投递，见 5.4 与 5.5。

**keep-alive 判定细节（`splitEvents`）**：遍历响应事件，遇到 `BkEventType == ""` 的事件即停止收集并标记 `hasEmptyEvent`；`pageCursor` 取最后扫描到的事件游标（即空事件游标，代表 CC 当前事件流位置）。CC 长轮询正常时（hold 至少 20s 超时返回），`elapsed >= keepAliveThreshold(1s)` 成立，不 sleep 直接续连；若 CC 未开启长轮询（立即返回空事件），`elapsed < 1s` 触发 `idleInterval` 退避，循环退化为受控短轮询。

### 5.3 单次请求：`fetchWatchPage()` 与三层压力控制

请求构造：

- `bk_resource` = 链资源；`bk_event_types` 来自 `eventTypesFor(resource)`（module 仅 `[update]`；process 为 `[create, update, delete]`，消费时不区分类型）；`bk_fields` 由 `fieldsFor(resource)` 固定：module 为 `[bk_biz_id, bk_set_id, bk_module_id, bk_module_name]`，**process 为 `[bk_biz_id]`（进程事件仅用于定位受影响业务再触发全量同步，事件明细无需进程属性）**。
- 游标为空（链启动、进程重启、游标过期重置）时改用 `bk_start_from = now - startFromOffset`（默认 5m，替代旧实现 1h，显著缩短追赶窗口；上限 1h，仍受 CC 3 小时事件保留限制）；否则带 `bk_cursor`。

压力控制顺序（先信号量、后限流）：

1. **信号量 `maxConcurrency`**（默认 8）：使用 Go 扩展库 `semaphore.Weighted.Acquire(ctx, 1)` 获取额度，`defer Release(1)` 释放；等待支持 context 取消。goroutine 数可以远多于信号量额度，但同时在途的 watch 请求不超过该值；由于每个长轮询请求占用 CC 侧一个 hold 会话约 20s，该值实质上也是 **CC watch session 数上限**，需与 CC 侧连接容量对齐。
2. **QPS limiter `qpsLimit`**（默认 20，`rate.NewLimiter(limit, 1)`）：**取得并发槽之后再 `limiter.Wait`**——防止大量等待槽位的 goroutine 积攒令牌、槽位释放后同时发出突发请求。
3. **`idleInterval` 下限保护**：见 5.2，仅在 CC 未 hold 时生效，配置再小也 clamp 到 ≥ 1s。

最终调用 `c.cmdb.ResourceWatchLongPolling(ctx, req, c.requestTimeout)`，`requestTimeout` 在构造时解析，非法或 ≤ 20s 时回退 35s。

### 5.4 事件处理：module 逐条更新 + process 触发业务级全量同步

#### 分派：`processPage()`

- 每条链内**同步串行**处理本页事件（无独立处理器 goroutine），**同资源事件严格保序**。
- module 事件逐条独立处理（单条失败不中断本页其余事件，错误聚合后统一告警）；process 事件整页交给 `handleProcessEventsBatch`。
- 页内出现非本链资源的事件视为异常：该条只跳过并计错，同页的进程事件照常同步（正常情况下 CC 按订阅资源推送）。

#### module 事件（`handleModuleEvent()`）

- 事件 `bk_detail` 解码为 `ModuleInfo`，`validCMDBID` 校验 `bk_biz_id`、`bk_set_id`、`bk_module_id`——防止缺失、负数或溢出的 ID 被转换为数据库资源 ID。
- watch 链仅订阅 **update** 事件，用于将模块名变更同步到进程表：`Process().UpdateSelectedFields` 按 set_id + module_id 双条件批量更新 `module_name`。
- 非 update 事件（理论不会出现）仅告警跳过，不阻断游标推进。
- 本事件是 `module_name` 的**快路径**（秒级生效），但不是唯一写入点：事件处理失败后游标仍照常推进
  （见 5.2），漏掉的一次改名不会自行修复，因此 `module_name` 同样纳入全量同步的
  `BuildProcessChanges` 拓扑 diff 兜底（与 `set_name` / `service_name` / `environment` 一并覆盖）。

#### process 事件批量同步（`handleProcessEventsBatch()`）

- **不区分 create/update/delete（参考 gsekit：仅存在进程事件即全量同步）**：任一进程事件都触发受影响业务的全量同步，事件明细经 `bk_fields` 裁剪仅含 `bk_biz_id`，不参与数据构建。
- `groupProcessEventsByBiz`：解码事件明细，仅校验并提取 `bk_biz_id`，按业务归并去重。**单条明细非法只跳过该条并累计错误**（`errors.Join` 后上抛）：CC 一页可含多个业务，坏事件不能连带丢弃同页其他业务的变更——游标照常推进，被跳过的事件不会重放，只能等定时/一键全量同步。
- **投递而非执行**：归并出的业务 ID 逐条 `enqueue` 进该租户的 `bizSyncQueue` 后立即返回，页处理不再等待同步完成。这是本方案与「链内串行同步」的关键差别，见下文「同步执行队列」。
- 队列 worker 的每个业务：先检查 `svc.IsProcessConfigViewEnabled(bizID)`，未开启进程配置视图的业务直接跳过；然后 `cmdb.NewSyncCMDBService(...).SyncBizProcesses` 做**全量拉取 + 全量 diff 落库**（详见 5.7），成功后 `dispatchProcessStateSyncTasks` 把进程状态同步任务派发给 TaskManager。
- **GSE 任务派发的量控**（`dispatchProcessStateSyncTasks`）：`taskManager.Dispatch` 异步入队，实际执行由 worker 池消费。单次派发量为该业务全量同步产生的进程变更数；worker 并发数不限制队列积压。

#### 同步执行队列（`bizSyncQueue`）

全量同步不再占用 watch 链的生命周期，而是投递给按租户隔离的执行队列：

- **为什么解耦**：链内串行执行时，一个大业务的全量同步会挡住该租户后续所有进程事件页；停链又只等 `chainStopWaitTimeout`(10s)，超时后链保留在 `stopping` 状态、对账不会为同一个 key 重建，直到那次同步返回才恢复——慢同步或卡住的 DB 调用会让该租户的 process watch 在这段时间里消失。投递模型下链只做「拉页 + 入队」，停链立即生效。
- **ctx 归属与回收**：队列使用 watcher（`syncCtx`）的子 context，而非链 ctx。链短暂重启时沿用队列；租户下线或其 process 链被 `maxWatchChains` 淘汰时，`stopQueuesExcept()` 取消并等待该租户的 retrier/worker 退出；slave 切换时停止全部 watch 链并回收全部队列。进程退出时 `stopQueues()` 回收全部队列。
- **合并与串行**：`pending`（待执行）+ `running`（执行中）两个集合。已在 `pending` 的重复投递直接合并；`running` 中的业务再次投递会进入 `pending` 追加一轮——覆盖「事件晚于当前同步快照」的情况，同一业务不会被两个 worker 并发执行。
- **并发度**：worker 数 = `handlerConcurrency`（默认 4，按租户）；业务之间互不影响，单业务失败不中断其他业务。
- **抢锁跳过**：worker 收到 `ErrSyncSkipped` 时不当作已同步，交给 `skippedBizRetrier` 每 30s 补一轮（上限约 20 分钟），补跑同样经过同步入口。
- **可靠性**：队列只在内存中，进程重启后待执行项丢失，由定时/一键全量同步兜底（全量同步幂等）；投递成功即推进游标，重放同一页不会产生额外代价。
- **内存边界**：队列实例数随实际运行的 process 链租户数变化，并受 `maxWatchChains` 限制；租户下线或被截断时取消并回收 worker、retrier 和队列项。单队列的 `pending` 按业务 ID 去重，仍可能随不同待处理业务数增长；这是积压而非残留租户泄漏，需结合同步速度与事件速率监控。

### 5.5 游标生命周期小结

游标**只保存在内存中，不落库**（每个 `(tenant, resource)` 链各持有一个字符串）。这样做的取舍是：

- **收益**：零 DB 写入；内存游标随每次响应推进，永远新鲜，`1103007` 几乎不可达，不存在「库里有坏游标、重启后反复过期」的状态；`runWatchChain` 只有一条恢复路径（游标为空 → `startFromOffset` 回溯）。
- **代价**：进程重启后游标必然丢失，每次重启都要重放 `startFromOffset` 窗口的事件（每个受影响业务触发一次全量同步）；停机时长超过该窗口时漏掉的事件只能等定时全量同步（默认 30m 一轮、每轮 10 个业务）或一键全量同步补偿，滞后可达小时级。

| 场景 | 处理 |
|------|------|
| 链启动 / 进程重启 | 内存游标为空 → 首轮请求用 `bk_start_from = now - startFromOffset`（默认 5m，上限 1h）回溯 |
| 每页处理完成 | 内存游标推进到页内最后事件游标（module 直接写库，process 仅投递队列；处理失败仅告警、游标照常推进），无任何写库 |
| keep-alive 空事件 | 空事件游标代表 CC 当前事件流位置，推进游标但不作为数据处理；空事件之后的事件继续处理，不丢弃 |
| 空响应（`bk_events` 为空数组） | 不携带游标，**保留旧游标**，避免退化成每轮回溯并重放事件 |
| 游标过期（1103007） | 正常运行不会命中（内存游标随响应推进）。仅在链长时间停滞超过 CC 事件保留期时出现：重置游标 → 下一轮 `bk_start_from` 回溯，并沿用请求失败的指数退避（1s → 2s → … → 1m），避免紧密重试 |
| 重启 / master 切换 | 无续传能力，一律按 `startFromOffset` 回溯；全量同步幂等，重放无副作用 |
| 链下线（租户禁用/资源下线） | 链被停止时游标随之丢弃，重新上线时按 `startFromOffset` 回溯 |
| 优雅关闭 | 链 ctx 取消，watch 循环立即退出（不再等待同步）；对账/关闭等待链退出最多 `chainStopWaitTimeout=10s`，随后 `stopQueues()` 取消队列 ctx 并等待 worker 退出 |
| 主从切换 / 链被停 | 只取消拉取：队列里的待执行项与执行中的同步按 watcher 级 ctx 继续跑完，队列空闲后不退出；链重启后沿用同一队列 |

### 5.6 配置默认值与防御性兜底

配置加载链路（`pkg/cc/types.go`）：

- `CrontabConfig.validate()` 会调用 `WatchCmdbResource.validate()`：
  - 四个 duration（`interval` / `idleInterval` / `startFromOffset` / `requestTimeout`）非空时必须能解析且 > 0；
  - `requestTimeout` 非空时必须 **> 20s**（CC 长轮询 hold 窗口）；该校验**仅在 `enabled=true` 时生效**，避免未启用的存量配置因运行期不生效的字段阻塞服务启动；
  - `startFromOffset` 必须 **≤ 1h**（`maxWatchStartFromOffset`）：游标不落库，这是每次重启的重放窗口，过大会反复触发大批业务的全量同步，且不能顶到 CC 约 3 小时的保留边界；同样仅在 `enabled=true` 时校验；
  - `maxConcurrency` / `handlerConcurrency` / `maxWatchChains` / `qpsLimit` 必须 ≥ 0，且 `qpsLimit` 拒绝 NaN / ±Inf。
- **watch 资源类型与 `bk_fields` 写死在代码内**（`resources()` / `fieldsFor()`），不开放配置：事件处理逻辑与资源类型、字段强耦合（module 事件需要拓扑 ID 与模块名，process 事件只需业务 ID），错误配置只会让链静默不工作或整页解析失败。存量 yaml 中残留的 `resources` / `fields` 会被 yaml 解析忽略，不影响升级。
- `trySetDefault()`：`interval=1m`、`idleInterval=1s`、`maxConcurrency=8`、`qpsLimit=20`、`handlerConcurrency=4`、`maxWatchChains=100`、`startFromOffset=5m`、`requestTimeout=35s`。
- **运行时兜底（`NewCmdbResourceWatcher`）**：防止调用方绕过 `trySetDefault` 传入零值——`qpsLimit <= 0 / NaN / Inf` 回退 20；`maxConcurrency < 1` 回退 8（避免 limiter burst=0 请求被永久拒绝、信号量为 0 链静默卡死）；`handlerConcurrency < 1` 回退 4。`interval`（回退 1m）、`idleInterval`（clamp ≥ 1s）、`startFromOffset`（回退 5m）、`requestTimeout`（回退 35s）、`maxWatchChains`（回退 100）均在构造时完成解析和兜底，运行期间直接读取。

### 5.7 进程全量同步的关键语义（`internal/processor/cmdb/sync_cmdb.go`）

process 事件触发的 `SyncBizProcesses` 是**全量对账**语义，正确性依赖以下不变量与防护：

1. **删除语义的前提**：`SyncProcessData → diffProcesses` 会删除「库内存活进程中、CMDB 全量列表中不存在」的进程。该语义只在**新列表 = 业务全量**时成立——若新列表是子集（如按进程 ID 过滤的增量拉取），子集外存量进程会被全部误删。因此 `syncProcesses` 已移除按进程 ID 过滤的增量能力，并在注释中显式禁止。
2. **空列表语义**：CC 返回成功且列表为空（`count=0`）即「该业务在 CMDB 已无进程」，空列表照常交给 diff，库内该业务存活进程按删除语义收敛，否则最后一个进程被删除后本地行会永久残留 `synced`。CC 权限、限流、接口异常由错误返回拦截（`PageFetcher` 返回 error 即放弃本轮，不会被当成空列表）；「拉取到非空关联信息却一条进程都构建不出来」视为构建异常并返回错误，同样不触发删除。已 `deleted` 的行不参与重复标记，避免全量清理后每轮同步产生写放大。
3. **分页推进与「拿不全即失败」**：`PageFetcher` 以**实际返回条数**推进 `start`（而非请求的 limit）。CC 部分接口对页大小有上限（实际返回 < limit），按 limit 推进会跳页漏段，漏段进程同样会被误删；总数不可靠（`count <= 0`）时以空页作为结束条件；当 `count` 恰好等于首页条数且首页已满页时视为总数语义不确定，继续翻页确认。**只要无法确认已取满全量（触达 `maxPageFetcherPages=200`、总数未拉满却遇到空页），一律返回错误并放弃本轮同步**，绝不把部分列表交给 diff。该函数同时被集群（`cc_topo.go`）、服务模板（`config_template.go`）拉取复用。
4. **软删除恢复**：`diffProcesses` 把库内行分为存活索引与软删除索引（`cc_sync_status='deleted'`）。CMDB 进程在存活索引中不存在、但命中软删除记录时，走 `restoreDeletedProcess` **原地恢复**（保留原进程 ID、实例历史、`os_type`、`process_state_synced_at`，以 CMDB 数据刷新规格与拓扑附件，按 `proc_num` 扩缩容实例），而不是新增——避免同 `cc_process_id` 出现重复存活行。此前因误操作被软删除的正常进程，靠该机制在下一次全量同步时自动恢复。
5. **重复存活行收敛**：唯一键为 `(tenant_id, biz_id, cc_process_id, cc_sync_status, alias)`，同一 `cc_process_id` 可能存在多条不同别名的 `synced` 行（别名重建残留）。diff 用 `pickProcessMatch` 确定性选取与 CMDB 别名一致的基准行；其余行仅在「实例全部停止且未托管」时收敛为 deleted，否则保留并告警（存在运行中实例，需人工确认）。
6. **无变更短路**：`SyncProcessData` 在 diff 无任何变更时直接返回（`SyncProcessResult.Changed=false`），调用方跳过冲突检查等全表级批量写——事件驱动的全量同步绝大多数是无变更场景。
7. **同业务串行**：`SyncBizProcesses` 用 `(tenant, biz)` 维度的 **Redis 租约锁**（`lock.TokenLock`，资源名 `cmdb:process:sync:<tenant>:<bizID>`，TTL 600s）协调多实例；抢锁最多等待 10s（期间响应 ctx 取消），超时说明其他入口正在同步同一业务，本轮跳过即可（游标照常推进，跳过的业务由链级重试队列每 30s 补一轮，上限约 20 分钟，之后由定时全量同步或一键同步覆盖）。未注入 `redLock` 时退化为进程内互斥。锁覆盖分页拉取、GSE 查询、事务内 diff 与写库，持锁期间按 TTL 的一半续期；value 带持有者 token，续期与释放经 Lua 校验 token，过期接管后旧持有者不能续期或误删新锁。续期调用使用可取消 context；续期失败会取消同步 context，并在事务提交前检查、回滚。
   - 此锁是可续期租约，不是数据库 fencing token；暂停/网络分区后的旧持有者在检测到续期失败前仍可能运行。严格避免两个写入者需在数据库写路径校验单调递增 token。
   - 截止时间由应用节点墙上时钟生成/比较，多实例应保持时钟同步；大幅时钟偏差会影响过期接管判断。
   - **超时不是成功**：抢锁超时返回哨兵错误 `ErrSyncSkipped`（`IsSyncSkipped` 判定），表示「本轮未执行」——此前的实现返回空结果，会被调用方当成「已同步且无变更」。
   - **watcher 路径**：`bizSyncQueue` 的 worker 遇到 `ErrSyncSkipped` 只告警并把业务交给 `skippedBizRetrier` 补一轮，不按失败上报。
   - **一键同步入口**：任务步骤遇到 `ErrSyncSkipped` 返回包装后的错误（任务不再显示成功），交由任务框架重试，重试耗尽时任务状态如实反映「未执行」。
8. **多租户**：`syncProcessesWithTx` 按 tenantID 过滤库内旧进程后再 diff；冲突检查（`ListDuplicateHostAliasWithTx`）与 `RestoreAbnormalWithTx` 也按租户限定，避免同号 `bk_biz_id` 跨租户误删/误标。
9. **业务去重**：每个活跃 process 链租户只有一个 `bizSyncQueue`，页内业务 ID 去重、队列内同业务合并排队（执行中再投递追加一轮）；主实例对账时回收已无 process 链租户的队列。其他入口与跨实例互斥由 Redis 租约锁协调。
10. **实例快照预取**：`SyncProcessData` 在事务内一次性批量预取旧进程实例（`InstancesByProcessID`，500/批），diff 阶段的「是否可安全变更」「实例数量」判定走内存，避免万级进程业务产生上万次事务内查询。
11. **进程 ID 归属唯一**：回填新增实例的 `process_id` 时，若同一 `(tenant, biz, cc_process_id)` 命中多条进程（别名重建残留），直接返回错误回滚，而不是把实例挂到任意一条上。
12. **agent 状态查询去重分批**：`ListAgentState` 前对 `agent_id` 去重并按 500/批请求，避免同一主机的重复 ID 撑爆 GSE 入参；某批失败仅按 Abnormal 处理，不阻断主流程。

## 6. 配置示例（`cmd/data-service/etc/data_service.yaml`，实际生效内容）

```yaml
crontab:
  watchCmdbResource:
    # 是否开启 CMDB 资源 watch 任务（默认 false）
    enabled: false
    # 租户/资源扫描周期，用于发现新租户或新资源、对账增减 watch 链（默认 1m）
    interval: 1m
    # 回退探测间隔，仅在 CC 未持有长轮询连接（响应耗时 < 1s）时生效，下限钳制为 1s（默认 1s）
    idleInterval: 1s
    # 对 CC 的并发 watch 请求数上限，即 CC watch 会话数上限（每个长轮询请求占用一个 CC 会话，最长 20s）（默认 8）
    maxConcurrency: 8
    # watch 请求 QPS 限制，主要用于收敛积压追平时的突发请求，须 >= 0（默认 20）
    qpsLimit: 20
    # 每个租户的 process 全量同步 worker 并发数（同步执行队列的 worker 数，与 watch 链解耦）；
    # 同一资源的事件仍按 FIFO 顺序处理，同一业务不会被并发同步（默认 4）
    handlerConcurrency: 4
    # 游标只保存在内存，进程重启后必然丢失，因此这是每次重启的回溯偏移量（默认 5m）
    # 上限 1h：窗口越大重启重放越多（每个受影响业务触发一次全量同步），超出会在启动期报错
    startFromOffset: 5m
    # 单次长轮询 watch 请求超时，必须 > 20s（CC 持有窗口），校验不通过会在启动期报错（默认 35s）
    requestTimeout: 35s
    # 同时运行的 watch 链（租户×资源）上限；超限的链不会运行并告警，多租户场景需调大（默认 100）
    # 注意：watch 的资源类型固定为 module/process、bk_fields 由代码固定，均不开放配置
    maxWatchChains: 100
```

配置项语义补充：

- `interval` 只控制**租户/资源发现与主从对账周期**，不参与事件消费节奏（长轮询模式下节奏由 CC hold 窗口决定）。
- `idleInterval` 仅在 CC 未 hold（响应耗时 < 1s 且无事件）时生效；CC 长轮询正常工作时此配置不参与节奏控制。
- 资源类型固定为 module/process（`set` 已不支持），`bk_fields` 固定为 module `[bk_biz_id, bk_set_id, bk_module_id, bk_module_name]`、process `[bk_biz_id]`；两者都不开放配置，也不受存量 yaml 残留字段影响。
- `maxWatchChains` 截断是确定性的（排序后保留前 N），被截掉的链不会运行且每次对账都会告警。

## 7. 装配与启停

- 装配（`cmd/data-service/app/app.go:603-608`）：`crontabConfig.WatchCmdbResource.Enabled` 为 true 时，
  `crontab.NewCmdbResourceWatcher(ds.daoSet, ds.sd, ds.cmdb, ds.gseSvc, ds.service, ds.taskManager, ds.cmdbSyncLock, crontabConfig.WatchCmdbResource)`
  并 `Run()`。`ds.cmdbSyncLock` 是同一业务全量同步的分布式锁（`*lock.TokenLock`），未注入时退化为进程内互斥。
- 优雅关闭：`Run()` 内 `shutdown.AddNotifier()`，收到信号后停掉全部链（逐个 cancel 并等待 `done`，共享 `chainStopWaitTimeout=10s` 预算）、
  取消 rootCtx，再 `stopQueues()` 取消同步队列 ctx 并等待 worker 退出。
- 多实例：仅 master 运行链；slave 停止全部链。master 切换后新 master 的链从空游标开始，按 `startFromOffset` 回溯消费（全量同步幂等，重放无副作用）。

## 8. 长轮询专用客户端与超时

- **全局客户端**（`internal/components/client.go`）：`timeout = 30s` 硬编码（L38），`GetClient()` 单例初始化；`http.Client.Timeout` 是硬上限，请求级 ctx 无法突破，因此不能直接承载 > 20s 的长轮询。
- **长轮询客户端**（`GetLongPollingClient(requestTimeout)`，L222-237）：与全局客户端共享 `newRestyClient` 构建的中间件与鉴权逻辑，**仅超时可配置**；相同超时的客户端按需缓存复用（`longPollingClients` map + 互斥锁），并发初始化安全（`client_test.go` 的 `TestSharedClientsConcurrentInitialization` 覆盖：并发获取为同一实例、不污染全局客户端、全局超时不变）。
- **入口**（`internal/components/bkcmdb/bkcmdb.go:516-530`）：`ResourceWatchLongPolling(ctx, req, requestTimeout)` 走 `doRequestWithClient` 指定专用客户端，URL 与 `ResourceWatch` 一致（`watchResource` 路由）。契约：`requestTimeout` 必须 > 20s。
- 其余 CMDB 接口仍走全局 30s 客户端，行为不变。

## 9. 与其他 watcher 的关系

`pkg/cc/types.go:1428` 保留 TODO：`WatchBizHostRelation` 和 `WatchHostUpdates` 后续合并为 `WatchCmdbResource`。本期实现的链管理（reconcile/链表/cancel-done）、游标管理（仅内存 + startFromOffset 回溯）、三层压力控制（信号量 + QPS limiter + idle clamp）、退避与兜底逻辑均集中在 crontab 包内，host/host_relation 合并时可直接复用该框架，本期不实施合并。

## 10. 测试覆盖

| 测试 | 文件 | 覆盖点 |
|------|------|--------|
| `TestWatchCmdbResourceDefaultDisabled` / `CanBeEnabled` / `Defaults` / `KeepsConfiguredValues` / `ValidateRejectsInvalidValues`（含 requestTimeout 过短/等于 20s 边界、非法 duration、负数、NaN/Inf qpsLimit、非法 resource 枚举等） | `pkg/cc/types_test.go` | 配置默认值、透传与校验 |
| `TestWatcherAdvancesCursorPastFailedPage` | `cmd/data-service/service/crontab/cmdb_resource_watcher_test.go` | 事件页处理失败仅告警，游标照常推进（漏掉变更由全量同步兜底） |
| `TestWatcherProcessEventsBatchSkipsBadEvent` | 同上 | 坏事件只跳过该条：聚合错误上抛，同页合法业务仍入队并被同步 |
| `TestWatcherProcessEventsBatchAllInvalid` | 同上 | 整页事件全非法时不投递任何业务，但仍上抛聚合错误 |
| `TestWatcherProcessEventsBatchReturnsBeforeSync` | 同上 | 同步阻塞时投递仍立即返回：拉取周期与同步执行解耦 |
| `TestWatcherProcessPageKeepsProcessEventsOnMixedResource` | 同上 | 非本链资源的事件只跳过计错，同页进程事件仍入队 |
| `TestBizSyncQueueCoalescesAndSerializes` | 同上 | 同业务合并排队、执行期间再投递追加一轮、同一业务不会被并发执行 |
| `TestBizSyncQueueContinuesAfterFailure` / `TestBizSyncQueueRetriesSkipped` | 同上 | 单业务失败不影响其他业务；抢锁跳过由 retrier 补一轮 |
| `TestWatcherRejectsInvalidProcessIdentity` | 同上 | 非法/负数/溢出的 `bk_biz_id` 事件明细被拒绝且不产出业务 |
| `TestWatcherStartsFromOffsetOnChainStart` | 同上 | 链启动无游标时用 `bk_start_from = now - startFromOffset` 回溯 |
| `TestWatcherResetsCursorOnExpiredError` | 同上 | 游标过期（1103007）后重置内存游标并回退 `startFromOffset`，不再带无效游标重试 |
| `TestWatcherKeepsCursorOnEmptyResponse` | 同上 | 空响应不携带游标时保留旧游标，避免反复回溯重放 |
| `TestWatchCmdbResourceStartFromOffsetCap` | `pkg/cc/types_test.go` | `startFromOffset` 上限 1h，未启用时不校验 |
| `TestSharedClientsConcurrentInitialization` | `internal/components/client_test.go` | 长轮询客户端并发初始化与全局客户端隔离 |
| `TestPageFetcherAdvancesByReturnedCount` / `HandlesUnreliableCount` / `StopsAfterSinglePage` / `StopsAtMaxPages` | `internal/processor/cmdb/sync_cmdb_pagefetcher_test.go` | 分页按实际返回条数推进、count 不可靠时不漏段、翻页上限 |
| `TestValidateProcessRelatedInfo*` | `internal/processor/cmdb/sync_cmdb_agentid_test.go` | 进程关联信息校验；非模板进程允许缺失 `process_template` |
| `SyncProcessData` / diff / 扩缩容等既有用例 | `internal/processor/cmdb/` | 全量同步 diff、proc_num 归一化、agent_id 同步等 |

## 11. 风险与对策（现行实现视角）

| 风险 | 现状对策 |
|------|----------|
| 长轮询请求占用 CC watch session 连接 | `maxConcurrency` 即 CC 侧 hold 连接数上限（默认 8），上线前需与 CC 侧确认连接容量；空闲流量极低（每 20s 一次/维度） |
| 链路中间层（API Gateway / LB）对长响应超时 < 20s，hold 期间被切断 | 上线前实测链路超时配置；被切断表现为请求失败，走指数退避重试（游标未推进，无数据丢失） |
| 全局共享 resty 客户端 30s 硬超时截断长轮询 | 已解决：watch 使用 `GetLongPollingClient` 专用客户端（超时可配置，默认 35s），不影响全局客户端 |
| watch 加速后 CC 压力上升 | 空闲流量本身极低；追积压突发由 `maxConcurrency` + `qpsLimit`（先信号量后限流）双重限制；`idleInterval` clamp 兜底短轮询退化场景 |
| 处理失败丢事件 | 失败仅告警、游标照常推进；漏掉的变更由下一轮同类事件或一键全量同步兜底（对齐 gsekit 取舍） |
| 重启/master 切换重复消费 | 游标只在内存，重启后必然按 `startFromOffset`（默认 5m）回溯重放；全量同步幂等、重放无副作用，代价是每次重启都要重放该窗口 |
| 游标落库带来的 DB 压力与失效状态 | 已消除：游标不落库，`runWatchChain` 对 DB 零写入，也不存在「库里有坏游标、重启后反复过期」的跨重启脏状态 |
| 重启重放触发大量全量同步 | `startFromOffset` 默认 5m 且上限 1h，窗口越长重放越多（每个受影响业务一次全量同步）；高频重启场景下这是不落库的主要成本 |
| **全量同步误删存量进程** | 四重防护见 5.7：仅允许全量语义（禁止按进程 ID 增量拉取）、失败与构建异常一律不删（成功空列表才按删除处理）、分页按实际返回推进防漏段、**无法确认取满时直接失败不上库**；软删除记录可被 `restoreDeletedProcess` 原地恢复 |
| 空响应清空游标导致反复回溯重放 | 游标仅在响应携带非空游标时推进，空响应保留旧游标 |
| 同页同业务重复事件 | 页内业务 ID 去重、页间串行处理；其他入口与跨实例由 Redis 锁串行 |
| 回溯窗口 5m 可能漏事件 | 窗口内事件按 `startFromOffset` 重放；超出窗口（停机过久）漏掉的变更由定时全量同步（30m 一轮）或一键全量同步补偿，滞后可达小时级 |
| 租户很多链数量失控 | `maxWatchChains`（默认 100）确定性截断 + 告警提示 |
| 配置零值/非法值导致链卡死或请求被永久拒绝 | `validate()` 启动期拒绝（负数、≤ 20s 超时、NaN/Inf、非法枚举）；`trySetDefault()` 补默认值；`NewCmdbResourceWatcher` 及各使用点再做运行时兜底回退 |

## 12. 验收标准（对照实现）

1. CC 上修改模块名称 → 本地 DB 中 `module_name` 更新延迟 P99 < 3s（CC 长轮询推回及时的前提下）。
2. 空闲状态下观察 (tenant, resource) 维度的 watch 请求频率约为 1 次 / 20s（keep-alive 空事件）。
3. 模拟 1 万条积压事件，watch 追平期间对 CC 的并发请求数 ≤ `maxConcurrency`，QPS ≤ `qpsLimit`。
4. 重启服务后链从空游标开始，首轮请求带 `bk_start_from = now - startFromOffset` 回溯；回溯窗口内事件重放不产生脏数据（全量同步幂等），`configs` 表不再出现 `cmdb_resource_cursor-*` 键。
5. 配置非法值（`maxConcurrency: -1`、`idleInterval: 100ms`、`requestTimeout: 10s` 等）启动报错（`validate`）或运行时 clamp/回退，有明确日志。
6. 单测覆盖见第 10 节；`pkg/cc` 配置用例与 crontab 游标推进/事件校验用例均在 CI 中执行。

实现简化及官方扩展库选择见 [重构说明](cmdb-resource-watcher-refactor.md#7-实现简化与依赖选择)。keep-alive 耗时仅统计 CMDB 调用，不包含并发槽和限速等待。
