# CMDB Resource Watch 改造实现说明（改造后）

> 本文描述**已落地**的改造实现，与未提交代码一一对应；改造前的背景与方案设计见 [`docs/cmdb-resource-watcher-optimization.md`](cmdb-resource-watcher-optimization.md)。

## 1. 变更概览

`data-service` 的 CMDB 资源监听任务（set / module / process）从"10s 固定 tick + 租户×资源串行 drain"改造为"**(tenant, resource) 维度常驻长轮询链 + 可控并发 + 配置化**"，依托 CC（bk-cmdb）`resource_watch` 短长链设计实现准实时推送。

变更文件清单：

| 文件 | 改动 |
|------|------|
| `cmd/data-service/service/crontab/cmdb_resource_watcher.go` | 核心重构：常驻 watch 链、长轮询循环、FIFO 页处理、并发与退避控制 |
| `pkg/cc/types.go` | `WatchCmdbResourceConfig` 扩展 10 个配置项，新增 `trySetDefault()` / `validate()` |
| `pkg/cc/types_test.go` | 新增 `TestWatchCmdbResource*` 配置默认值/校验用例 |
| `cmd/data-service/etc/data_service.yaml` | `watchCmdbResource` 配置样例与注释 |
| `cmd/data-service/app/app.go` | 装配时传入 `crontabConfig.WatchCmdbResource` |
| `internal/components/client.go` | 抽取 `newRestyClient()`，新增 `GetLongPollingClient(timeout)` |
| `internal/components/bkcmdb/bkcmdb.go` | 抽取 `doRequestWithClient()`，新增 `ResourceWatchLongPolling()` |
| `internal/components/bkcmdb/service.go` | `Service` 接口新增 `ResourceWatchLongPolling` 方法 |

## 2. 改造后架构

```text
Run()
 └─ interval ticker（默认 1m）→ reconcileChains() 对账
     ├─ slave：停止全部链
     └─ master：按 启用租户 × resources 生成目标链集合
         （超 maxWatchChains 时排序截断并告警；新增链补启、失效链取消）
           │
           ▼  每条链一个 goroutine（runWatchChain）
 for {  // 常驻长轮询循环，响应即续连
     semaphore acquire + limiter.Wait()        ← 对 CC 压力控制
     cmdb.ResourceWatchLongPolling(ctx, req, requestTimeout)
     ├─ 游标过期(1103007)：清游标 → startFromOffset 回溯，连续 5 次告警
     ├─ 请求失败：指数退避 1s→2s→4s→…→1m（不占 CC 额度）
     ├─ 无事件/keep-alive：游标进入 FIFO 队列顺序落库；若 CC 未 hold（耗时 < 1s）退避 idleInterval
     └─ 有事件：按 maxEventsPerPage 切块 → pages channel（背压）
 }
           │ pages chan（容量 2，队满阻塞产生背压）
           ▼  同资源 FIFO 处理器（processPages）
     processPageWithRetry：失败退避重试，不推游标；连续失败 3 次告警并继续重试
           │
           ├─ set/module：逐条 handleEvent（update 改名 / delete 级联删进程，事务）
           └─ process：按原始顺序合并连续同类型事件 → 按业务分组 errgroup 有限并发（handlerConcurrency）
```

## 3. 各文件改动明细

### 3.1 `cmd/data-service/service/crontab/cmdb_resource_watcher.go`（核心重构）

**常量：**

| 常量 | 值 | 含义 |
|------|-----|------|
| `errCursorExpiredSign` / `errCursorMissingSign` | `"1103007"` / `"事件节点不存在"` | 游标过期错误特征（CC 错误码 + 文案双匹配） |
| `pageRetryLimit` | 3 | 连续失败告警阈值，达到后继续重试，不跳页 |
| `pageHandoffQueueLen` | 2 | watch 循环→处理器页队列长度，队满产生背压 |
| `keepAliveThreshold` | 1s | 判定 CC 是否 hold 长轮询的耗时阈值 |
| `cursorExpiredAlarmThreshold` | 5 | 连续游标过期告警阈值 |
| `defaultMaxRequestBackoff` | 1m | 请求失败指数退避上限 |

**结构体：** 新增 `conf cc.WatchCmdbResourceConfig`、`limiter *rate.Limiter`（全局 QPS 限流）、`semaphore chan struct{}`（CC watch 请求并发上限，即 CC hold 会话数上限）、`chains map[watchChain]watchChainState`（运行中链注册表，保存取消函数与退出通知，`mu` 保护）。

**关键函数：**

- `NewCmdbResourceWatcher(...)`：签名新增 `conf` 参数，并对 `QpsLimit/MaxConcurrency/HandlerConcurrency` 做防御性兜底（零值会导致 limiter burst=0 请求永久拒绝、信号量为 0 链静默卡死）。
- `Run()`：注册 `shutdown.AddNotifier()`；创建一次 `rootCtx` 作为所有链 ctx 父节点；启动后立即对账一次，随后按 `interval` ticker 周期对账；shutdown 信号到达时 `stopAllChains()` + `cancel()`。
- `reconcileChains()`：master/slave 判定；多租户模式用 `bkuser.ListEnabledTenants()` 发现租户，单租户模式只按资源建链；链数超 `maxWatchChains` 时按 (tenant, resource) 排序确定性截断并告警；差集补启新链、取消失效链。
- `runWatchChain()`：单条链主循环。ctx 三层组合（链级取消 + `KitKey` 注入避免多租户下 `kit.MustGetKit` panic + rid/租户 metadata）；启动时 `loadCursor` 加载 DB 游标，启动同资源 FIFO 处理器 goroutine；循环内处理游标过期、请求失败退避、keep-alive、切块 hand-off；退出前等待 `<-processorDone`。
- `fetchWatchPage()`：`semaphore` acquire（请求结束释放）→ `limiter.Wait()` → `ResourceWatchLongPolling`；无游标时用 `bk_start_from = now - startFromOffset` 回溯。
- `splitEvents()`：把 keep-alive 空事件（`bk_event_type == ""`）与业务事件分离；空事件游标代表 CC 当前事件流位置，需推进落库但不能当数据处理。
- `processPages()` / `processPageWithRetry()`：同资源 FIFO 串行消费页；事件处理或游标保存失败时指数退避重试（上限 1m），达到 `pageRetryLimit` 告警，成功前阻塞后续页。游标保存失败仅重试保存，不重复处理已成功的事件。
- `processPage()`：set/module 逐条 `handleEvent`；process 事件先按 create/update/delete 分三桶，再分别批量处理。
- `runBizGroups()`：process 事件按业务分组后 `errgroup.SetLimit(handlerConcurrency)` 有限并发。**刻意不用 `errgroup.WithContext`**：单业务失败不中断兄弟业务，整页失败由上游统一重试。
- `dispatchProcessStateSyncTasks()`：GSE 任务派发，`Dispatch` 仅为异步入队（全局 worker 池 200 消费），单次派发量由 `maxEventsPerPage` 间接约束。
- 游标读写：`loadCursor()`（`config` 表，`gorm.ErrRecordNotFound` 视为空，其他错误重试）、`persistCursor()`（写穿落库，失败重试成功前不处理后续页；重启可能重放尚未保存游标的事件）。

### 3.2 `pkg/cc/types.go` — `WatchCmdbResourceConfig`

| 字段 | yaml | 默认值 | 说明 |
|------|------|--------|------|
| `Enabled` | `enabled` | `false` | 是否启用 |
| `Interval` | `interval` | `1m` | 租户/资源扫描（链对账）周期 |
| `IdleInterval` | `idleInterval` | `1s` | CC 未 hold 时的降级探测间隔，clamp ≥ 1s |
| `MaxConcurrency` | `maxConcurrency` | `8` | 并发 watch 请求上限，同时是 CC watch session 数上限 |
| `QpsLimit` | `qpsLimit` | `20` | watch 请求 QPS 限流，主要收敛追积压突发 |
| `HandlerConcurrency` | `handlerConcurrency` | `4` | process 事件按业务处理的并发度；同资源事件始终 FIFO |
| `StartFromOffset` | `startFromOffset` | `5m` | 游标丢失/过期时的回溯偏移（原实现 1h） |
| `RequestTimeout` | `requestTimeout` | `35s` | 单次长轮询超时，必须 > 20s（CC hold 窗口），validate 强制 |
| `Resources` | `resources` | `["set","module","process"]` | 监听资源列表，validate 校验枚举 |
| `Fields` | `fields` | set/module 内置默认字段裁剪 | 各资源 `bk_fields`，未配置的资源回退代码默认值 |
| `MaxWatchChains` | `maxWatchChains` | `100` | 运行中链数上限（租户×资源），超限截断告警 |
| `MaxEventsPerPage` | `maxEventsPerPage` | `200` | 单批处理最大事件数，超出切块（对齐 CC 协议单页上限） |

- `trySetDefault()` 已挂入 `CrontabConfig.trySetDefault()`；`validate()` 拒绝非法 duration、负数、`requestTimeout ≤ 20s`、未知 resource 枚举（配置加载阶段报错）。
- 运行期还有二次防御：`runWatchChain` 内对 `interval/idleInterval/startFromOffset/requestTimeout` 解析失败时打错误日志并回退默认值；`maxWatchChains/maxEventsPerPage` 零值回退。

### 3.3 `internal/components/client.go` — 长轮询专用客户端

- 原全局客户端构建逻辑抽为 `newRestyClient(requestTimeout)`（超时参数化，其余中间件/TLS/UA 不变），`GetClient()` 复用之，行为不变（30s）。
- 新增 `GetLongPollingClient(requestTimeout)`：按超时缓存复用客户端，与全局客户端共享 pre-request hook 鉴权、响应 hook、错误 hook。**动机**：全局客户端 `http.Client.Timeout` 为硬上限、请求级 ctx 无法突破，30s 对 20s hold 的长轮询余量过小。

### 3.4 `internal/components/bkcmdb` — 长轮询 watch 接口

- `doRequest` 拆出 `doRequestWithClient(ctx, cli, ...)`，支持按请求选择客户端。
- `Service` 接口与实现新增 `ResourceWatchLongPolling(ctx, req, requestTimeout)`：使用 `GetLongPollingClient` 发起 `resource_watch` 请求。原 `ResourceWatch` 保留不动。
- 已有 `ResourceType` / `BkEventObj` / `WatchResourceRequest` / `WatchData` 协议类型直接复用。

### 3.5 装配与配置样例

- `app.go`：`NewCmdbResourceWatcher(..., crontabConfig.WatchCmdbResource)`。
- `data_service.yaml`：`watchCmdbResource` 节新增全部配置项及中文注释（见 3.2 表格），并注明"采用常驻长轮询循环，CC 变更准实时推送"。

## 4. 行为对比（改造前 → 改造后）

| 维度 | 改造前 | 改造后 |
|------|--------|--------|
| 触发方式 | 10s 固定 ticker | 常驻长轮询，响应即续连；事件准实时推回 |
| 并发模型 | 租户×资源全串行 | 每链一个 goroutine；对 CC 并发受 `maxConcurrency` 信号量约束 |
| 对 CC 保护 | 无 | 信号量 + QPS limiter + `idleInterval` 下限（≥1s）兜底 |
| 事件处理 | watch 循环内同步阻塞 | 同资源 FIFO 页队列异步处理；process 按业务 `handlerConcurrency` 并发 |
| 游标回溯 | 丢失后固定 1h | `startFromOffset`（默认 5m），可配置 |
| 失败处理 | 处理失败仅记日志继续 | 持续退避重试，连续失败 3 次告警；请求失败指数退避至 1m |
| 游标推进 | drain 完成后推进 | 整页 hand-off + 处理成功后推进；keep-alive 空事件游标也落库 |
| 超时 | 全局客户端 30s 硬编码 | watch 专用客户端，`requestTimeout` 可配置（默认 35s） |
| 租户 | 单轮遍历 | 多租户模式动态发现启用租户并增减链；单租户模式按资源建链 |
| 主从 | `IsMaster()` 判定 | 保留；slave 对账时停止全部链，master 切换后从 DB 游标续传 |
| 配置 | 仅 `enabled` | 12 项全配置化，带默认值与启动期校验 |

## 5. 测试与验证

- `pkg/cc` 新增用例（`pkg` 为独立 Go module，测试需在 `pkg/` 目录下运行）：
  - `TestWatchCmdbResourceDefaultDisabled` / `TestWatchCmdbResourceCanBeEnabled`
  - `TestWatchCmdbResourceDefaults`（全部默认值）
  - `TestWatchCmdbResourceKeepsConfiguredValues`（用户配置不被覆盖）
  - `TestWatchCmdbResourceValidateRejectsInvalidValues`（requestTimeout 边界、非法 duration、负值、非法资源枚举等子用例）
- 本地验证结果：上述用例全部通过；`go build ./cmd/data-service/... ./internal/components/...` 通过。
- 注：`pkg/cc` 既有的 `TestCC` 依赖本地 `api_server.yaml` 与仓库 basic auth 配置，与本改动无关。
- 建议后续补充（本次未覆盖）：游标过期重置、退避节奏、关闭 flush 的 crontab 层集成用例；灰度环境按设计文档第 7 节验收标准验证延迟与限流。

## 6. 运维注意事项

1. **`maxConcurrency` 即 CC watch session（hold 连接）数上限**：上线前与 CC 侧确认连接容量；空闲时每链约 1 次请求 / 20s（keep-alive），流量极低。
2. **链路中间层超时**：若 API Gateway/LB 对长响应强切且 < 20s，hold 期间会被断开；客户端识别提前断开后按退避重试（游标未推进，无数据丢失），必要时调大 `requestTimeout`。
3. **租户规模**：链数 = 启用租户数 × 资源数，超 `maxWatchChains`（默认 100）的链不运行并告警；需调大配置或减少 `resources`。
4. **阻塞告警**：出现 "page stalled" 日志说明当前页尚未完成，链会持续退避重试；需排查数据库错误或异常事件。
5. **多实例**：仅 master 运行；master 切换后新实例从 DB 游标续传，最多重放一页。
