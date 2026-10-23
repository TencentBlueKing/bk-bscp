# CMDB Resource Watch 实现说明

> 本文描述**当前已落地**的实现要点；设计背景、CC 接口能力与完整方案见
> [`docs/cmdb-resource-watcher-optimization.md`](cmdb-resource-watcher-optimization.md)。

## 1. 变更概览

`data-service` 的 CMDB 资源监听任务从「10s 固定 tick + 租户×资源串行 drain」改造为
「(tenant, resource) 维度常驻长轮询链 + 可控并发 + 配置化」，依托 CC（bk-cmdb）
`resource_watch` 短长链设计实现准实时推送。本期进一步把**进程事件**从「按事件进程 ID
增量同步」改为「任一进程事件触发该业务全量同步」。

变更文件清单：

| 文件 | 改动 |
|------|------|
| `cmd/data-service/service/crontab/cmdb_resource_watcher.go` | 常驻 watch 链、长轮询循环、页处理、按活跃租户回收同步队列、并发与退避控制 |
| `pkg/cc/types.go` | `WatchCmdbResourceConfig` 配置项、`trySetDefault()` / `validate()` |
| `pkg/cc/types_test.go` | `TestWatchCmdbResource*` 配置默认值/校验用例 |
| `cmd/data-service/etc/data_service.yaml` | `watchCmdbResource` 配置样例与注释 |
| `cmd/data-service/app/app.go` | 装配时传入 `crontabConfig.WatchCmdbResource` |
| `internal/components/client.go` | 抽取 `newRestyClient()`，新增 `GetLongPollingClient(timeout)`（`timeout <= 0` 回退 35s） |
| `internal/components/bkcmdb/bkcmdb.go` / `service.go` | 抽取 `doRequestWithClient()`，新增 `ResourceWatchLongPolling()` |
| `internal/processor/cmdb/sync_cmdb.go` | 全量同步删除语义修复、分页推进修复、软删除恢复、重复存活行收敛、无变更短路 |
| `internal/dal/dao/process.go` | 冲突检查（`ListDuplicateHostAliasWithTx` / `RestoreAbnormalWithTx`）支持租户隔离 |

## 2. 架构与关键函数

```text
Run()
 └─ interval ticker（默认 1m）→ reconcileChains()
     ├─ slave：停止全部链（日志降级为 V(3)）
     └─ master：按 启用租户 × 固定资源(module/process) 生成目标链集合
         （超 maxWatchChains 排序截断并告警；链数 > maxConcurrency 时告警提示事件延迟劣化）
           │
           └─ 每链一个 goroutine：runWatchChain()
                ├─ fetchWatchPage()：信号量(maxConcurrency) + QPS limiter → CC 长轮询
                ├─ splitEvents()：拆事件、取页游标、识别 keep-alive 空事件
                ├─ processPage()：链内处理（资源内严格保序）
                │    ├─ module：handleModuleEvent() 逐条更新 module_name
                │    └─ process：handleProcessEventsBatch()
                │         └─ groupProcessEventsByBiz() → bizSyncQueue.enqueue()（只投递，不等待）
                ├─ 游标仅保存在内存并随响应推进；事件处理失败仅告警、游标照常推进
                │
                bizSyncQueue（按活跃 process 链租户创建，watcher 级 ctx 子 context）
                ├─ worker × handlerConcurrency：cmdb.SyncBizProcesses() 全量同步 → dispatchProcessStateSyncTasks()
                └─ skippedBizRetrier：抢锁跳过的 biz 排队，每 30s 补一轮（上限约 20 分钟）
                ```

## 3. 事件处理语义

- **module**：仅订阅 `update`，用于把模块名变更同步到进程表（`set_id + module_id` 双条件）。
  页内逐条独立处理，单条失败不中断本页其余事件，错误聚合后统一告警（游标照常推进，被跳过的事件
  由下一轮事件或一键全量同步兜底）。
- **process**：订阅 `create/update/delete`，但**消费时不区分类型**——任一进程事件都触发
  受影响业务的全量同步（参考 gsekit）。事件明细经 `bk_fields` 裁剪，仅含 `bk_biz_id`，
  只用于定位业务，不参与数据构建。
- **set**：不再订阅 set 资源（`resources()` 固定为 module/process）。集群名变更由全量同步的
  `BuildProcessChanges` 拓扑 diff 覆盖（`set_name` / `service_name` / `environment`）；
  set/module 删除后的进程清理依赖 CC 推送的进程删除事件触发的全量对账。
- **可靠性取舍**：事件处理失败只告警、游标照常推进（重放整页会让页内其他业务反复做全量同步），
  漏掉的变更由下一轮事件、定时/一键全量同步兜底。
- **抢锁跳过的补偿**：`ErrSyncSkipped` 表示本轮全量同步未执行。**不能把「持锁方正在进行的同步」
  当作本事件的补偿**——锁覆盖「拉取全量 → diff → 写库」全过程，本事件的变更可能发生在对方快照
  取走之后。因此跳过的业务会登记到队列级 `skippedBizRetrier`，每 30s 重试一次（上限约 20 分钟），
  成功或失败后出队；队列只在内存中，进程重启后丢弃，最终由定时/一键同步兜底。
- **游标推进**：**游标只保存在内存中，不落库**；仅在响应携带非空游标时推进（空响应保留旧游标），
  keep-alive 空事件之后的事件继续处理，不因空事件提前结束。
- **重启语义**：进程重启后内存游标丢失，链启动时游标为空，首轮请求按
  `bk_start_from = now - startFromOffset`（默认 5m，上限 1h）回溯；回溯窗口之外漏掉的变更
  由定时全量同步（默认 30m 一轮）或一键全量同步补偿，全量同步幂等。
- **游标过期**：内存游标随每次响应推进，正常运行不会过期；只有链长时间停滞（超过 CC 事件保留期）
  才可能命中 `1103007`，此时重置游标并按 `startFromOffset` 回溯，同时沿用请求失败的指数退避。
- **业务去重**：每个租户仅有一条 process 链、一个 `bizSyncQueue`；页内按业务去重，队列内同业务合并排队（执行中再投递追加一轮）。其他入口与跨实例互斥由 Redis 锁保证。

## 4. 全量同步的关键不变量（`internal/processor/cmdb/sync_cmdb.go`）

1. **删除语义前提**：`diffProcesses` 会删除「库内存在、CMDB 全量列表中不存在」的进程，
   因此新列表必须是业务全量；已移除按进程 ID 过滤的增量拉取能力，禁止恢复。
2. **空列表语义**：CC 返回成功且列表为空（`count=0`）代表「该业务已无进程」，空列表照常交给
   diff，库内该业务存活进程按删除语义收敛（软删 + 清理 stopped/unmanaged 实例），
   否则最后一个进程被删除后本地行会永久残留。CC 权限、限流、接口异常由错误返回拦截
   （`fetchAllProcessRelatedInfo` → `PageFetcher` 返回 error 时直接放弃本轮）；「拉取到非空
   关联信息却一条进程都构建不出来」视为构建异常并返回错误，不按删除语义处理。
3. **分页推进**：`PageFetcher` 按实际返回条数推进 `start`；总数不可靠时以空页结束；
   翻页上限 `maxPageFetcherPages=200`。**无法确认取满全量（触达上限、总数未拉满却遇空页）时
   返回错误、放弃本轮同步**，禁止把部分列表当作全量交给 diff（否则漏段进程会被误删）。
4. **软删除恢复**：diff 区分存活行与 `cc_sync_status='deleted'` 行；CMDB 进程命中软删除记录时
   走 `restoreDeletedProcess` 原地恢复（保留进程 ID、实例历史、`os_type`、`process_state_synced_at`），
   而不是新增，避免同 `cc_process_id` 出现重复存活行。
5. **重复存活行收敛**：唯一键为
   `(tenant_id, biz_id, cc_process_id, cc_sync_status, alias)`，同 `cc_process_id` 可能存在多条
   不同别名的 `synced` 行（别名重建残留）。diff 用 `pickProcessMatch` 确定性选取与 CMDB 别名一致的
   基准行，其余仅在「实例全部停止且未托管」时收敛为 deleted，否则保留并告警。
6. **并发控制**：同一 `(tenant, biz)` 的全量同步由 **Redis 租约锁**协调（`lock.TokenLock`，
   资源名 `cmdb:process:sync:<tenant>:<bizID>`，TTL 600s，最多等待 10s 后跳过本轮），
   覆盖多实例部署；未注入 `redLock` 时退化为进程内互斥。无任何变更时 `SyncProcessData`
   直接返回，跳过冲突检查等批量写。
   锁在拉取全量数据**之前**获取，覆盖分页拉取、GSE 查询、事务内 diff 与写库；单次耗时随业务规模
   增长且不可预估，因此**持锁期间按 TTL 的一半续期**，正常续期时不会因超过 600s 被其他实例按
   「过期」接管。续期失败会取消同步 context，并在事务提交前检查取消状态，使当前事务回滚。
   value 为 `<token>|<截止时间>`，续期与释放走 Lua 脚本先校验持有者 token：锁过期被接管后旧持有者
   不能续期，也不会误删新持有者的锁（`lock.RedisLock` 的纯时间戳 value 没有这个能力）。
   这是带续期的租约锁，不提供数据库 fencing token；取消/回滚不能保证已暂停进程恢复后绝无并发写入，
   严格防护需要数据库写路径校验单调递增 fencing token。
7. **多租户**：旧进程、冲突检查、`RestoreAbnormalWithTx` 均按租户隔离，避免同号 `bk_biz_id`
   跨租户误删/误标。`tenant_id` 为空串的 legacy 行在多租户模式下不参与 diff（归属不明，放开会让
   多个租户争抢同一批历史行）；单租户模式下列表不过滤，legacy 行照常参与 diff。
   diff 的新增、重复行收敛、软删除恢复与删除扫描统一走 `sameOrLegacyTenant`：单租户模式下
   `syncProcessesWithTx` 的显式租户过滤不生效（`s.tenantID == ""`），该判定是唯一实际生效的一层，
   租户取 `syncTenantID(kit)`（空 → default，与写入侧 `getTenantID` 归一化一致）。
8. **实例快照预取**：事务内一次性批量预取旧进程实例（`InstancesByProcessID`，500/批），diff 的
   安全判定与实例数量走内存，避免 N+1 查询拉长事务；回填实例 `process_id` 时若同
   `cc_process_id` 命中多条进程则报错回滚，避免实例挂错进程。
9. **链停止与重建**：停止链时先统一 `cancel`，再在共享 `chainStopWaitTimeout=10s` 预算内等待退出
   （总等待时间不随链数线性增长）；超时未退出的链保留在 `chains` 中并标记 `stopping`，
   在其 goroutine 真正退出前不重建同 key 链；后续对账通过 `done` 非阻塞回收。
   链状态由唯一的管理 goroutine 维护，`sync.Once` 保证 `Run` 重复调用不会启动多个管理循环。

## 5. 配置项

见 `cmd/data-service/etc/data_service.yaml` 的 `crontab.watchCmdbResource`，关键项：

| 配置 | 默认 | 说明 |
|------|------|------|
| `enabled` | false | 是否开启 watch 任务 |
| `interval` | 1m | 租户/资源发现与主从对账周期 |
| `idleInterval` | 1s | CC 未 hold 长轮询时的退避间隔（clamp ≥ 1s） |
| `maxConcurrency` | 8 | 并发 watch 请求数上限（≈ CC watch 会话数） |
| `qpsLimit` | 20 | watch 请求 QPS 上限；0 表示使用默认值 |
| `handlerConcurrency` | 4 | 每个租户 process 同步队列的 worker 并发数（同一业务不会并发同步） |
| `startFromOffset` | 5m | 每次重启/游标过期时的回溯偏移，上限 1h（超出启动期报错） |
| `requestTimeout` | 35s | 单次长轮询超时，必须 > 20s |
| 资源类型 | 固定 module/process | **不开放配置**（`resources()` 写死；set 已不支持），存量 yaml 残留字段被忽略 |
| `bk_fields` | 固定 | **不开放配置**（`fieldsFor()` 写死）：module 为拓扑 ID + 模块名，process 为 `[bk_biz_id]` |
| `maxWatchChains` | 100 | 同时运行的链上限，超出确定性截断并告警 |

## 6. 已知限制与后续

- **锁的滚动升级**：全量同步锁已换为 `lock.TokenLock`（value 为 `<token>|<截止时间>`，续期与释放
  校验持有者 token）。新旧实例共用 `Lock:<res>` 命名空间：新实例可读旧格式（纯 RFC3339）并按过期接管，
  旧实例无法解析新格式，在等待窗口内保持跳过。旧实例不会续期，因此升级窗口内由新实例持有长锁时，旧实例会持续跳过该业务的同步，
  直到本次同步结束。
- **锁依赖 Redis 脚本能力**：`lock.TokenLock` 的续期、释放与过期接管走单 key 的 `EVAL`
  （标准 Redis 均支持）；若部署的代理禁用了脚本，抢锁会持续失败并告警，全量同步将一直跳过，
  需放开 EVAL。
- **锁续期与请求生命周期**：续期 RPC 使用同步任务 context；续期失败会取消该 context，事务在提交前
  检查取消并回滚。取消不具备 fencing 语义，无法约束已暂停的旧持有者恢复后对数据库的写入。
- **锁时间基准**：锁截止时间目前由应用节点的墙上时钟生成和比较；多实例部署应保证节点时钟同步。
  大幅时钟偏差可能导致提前接管或延迟接管。严格一致性场景应改用 Redis 服务端时间/数据库 fencing token。
- **队列和内存生命周期**：仅为实际运行 process watch 链的租户保留同步队列。主实例对账发现租户下线或链被
  `maxWatchChains` 淘汰时，会取消并等待对应队列退出；slave 状态停止全部 watch 链并回收全部队列。
  队列数受活跃 process 链数限制；队列内待处理业务按 ID 去重，但 distinct business 积压仍会占用内存。
  未注入 Redis 锁时的本地 keyed mutex 在最后一个持有者/等待者退出后删除条目。
- **分页容量**：`PageFetcher` 当前硬上限为 200 页。达到上限仍无法确认取全时会放弃同步，避免不完整快照误删；
  超大业务会持续失败，需要根据 CC 实际页大小调整上限并监控触顶错误。
- **回溯窗口**：游标不落库，**每次重启**都按 `startFromOffset`（默认 5m，上限 1h）回溯并重放该窗口的事件，
  窗口之外漏掉的变更依赖定时/一键全量同步补偿；不再有游标落库与跨重启续传能力。
- **重复存活行**：存在运行中实例的重复行只告警不删除，需按业务口径人工收敛。

## 7. 实现简化与依赖选择

- 并发控制使用 Go 扩展库 [`golang.org/x/sync/semaphore`](https://pkg.go.dev/golang.org/x/sync/semaphore) 的 `Weighted`，支持 context 取消；仓库已有此模块依赖。
- 业务并发使用固定 worker pool 和合并队列，QPS 控制使用 [`rate.Limiter`](https://pkg.go.dev/golang.org/x/time/rate)，不新增模块或升级版本。
- 不使用 `singleflight` 替代链生命周期管理：重复请求共享结果与取消旧链、等待退出、防止同 key 重建是不同语义。
- duration 配置在构造时解析一次；请求耗时只统计实际 CMDB 调用，排队等待额度和限速的时间不参与 keep-alive 判断。
- 测试使用标准库 `testing/synctest` 验证共享超时、排队取消和请求耗时，避免真实等待影响稳定性。
