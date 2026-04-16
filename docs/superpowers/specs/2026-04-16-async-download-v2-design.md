# Async Download V2 Design

Date: 2026-04-16

## Background

`feed-server` 当前的异步下载链路在同一个文件被大量 pod 同时拉取时，瓶颈集中在 `AsyncDownload` 请求阶段，而不是 GSE 传输阶段。

现有实现的主要问题是：

- 同一个文件的请求在 `upsertAsyncDownloadJob` 阶段使用文件级 Redis 锁串行化。
- 锁内还包含 `KEYS/GET/SET` 和整份 job JSON 的回写。
- `scheduler` 依赖扫描 job，再从大对象里读取 targets。
- 该模型在 `1k+` 并发同文件拉取时已经容易把 `AsyncDownload` RPC 本身拖到分钟级。

本设计面向 `5w` 级别同文件高并发下载，优先目标是让服务端接入路径稳定、实现简单、后续可持续演进，同时保持客户端接口不变。

## Goals

- 保持现有客户端 gRPC 接口和语义不变：
  - `AsyncDownload`
  - `AsyncDownloadStatus`
- 去掉同文件请求路径上的大锁和整对象回写。
- 支持 Redis 单机和 Redis Cluster。
- 不使用 Lua，不依赖跨多个 key 的强原子事务。
- 在极端并发下允许同一个文件形成多个 batch，以换取接入路径稳定。
- 控制单个 batch 和单次 GSE 分发规模，避免出现超级 batch。
- 让旧逻辑和新逻辑可以通过发布策略平滑切换，而不是长期在代码里保留复杂兼容分支。

## Non-Goals

- 本次不修改客户端协议和轮询机制。
- 本次不引入 CSI、节点级下载或新的部署形态。
- 本次不追求同一个文件在任意并发下严格收敛为唯一 batch。
- 本次不引入独立消息队列或事件流系统。

## Design Summary

新的实现保留客户端视角上的“每个请求一个 task”，但服务端内部不再把所有请求强行收敛到单个 job。

新的模型分为三层：

1. `task`
   - 对客户端可见，仍然按 `taskID` 查询状态。
2. `batch`
   - 服务端内部的聚合单元，负责在短时间窗口内收集同一文件版本的多个 targets。
3. `shard`
   - batch 的执行分片，负责将大量 targets 拆成多个较小的 GSE 子任务。

整体原则是：

- 请求阶段只负责登记 task 和 target，尽量挂到某个开放 batch 上。
- 执行阶段才进行分片和 GSE 分发控制。
- 调度阶段只处理到期 batch，不做全量扫描。

## Aggregation Key

batch 的聚合维度使用文件版本，而不是仅使用文件路径。

定义：

- `fileVersionKey = bizID/appID/filePath/fileName/signature`

要求：

- 同一路径不同内容不能进入同一个 batch。
- 同一文件内容的并发请求可以稳定聚合。

如果现有 `signature` 已经足够唯一，实际实现可以在内部将 `filePath/fileName/signature` 组合后做编码，避免 key 过长。

## Redis Data Model

### 1. Open Batch Pointer

Key:

- `AsyncBatchOpenV2:{fileVersionKey}`

Type:

- `String`

Value:

- `batchID`

Purpose:

- 供请求路径快速发现一个可能可加入的开放 batch。

TTL:

- 建议 `30s`
- 必须大于收集窗口，用于避免短暂抖动导致 batch 频繁切换。

### 2. Batch Metadata

Key:

- `AsyncBatchMetaV2:{batchID}`

Type:

- `Hash`

Fields:

- `batch_id`
- `biz_id`
- `app_id`
- `file_path`
- `file_name`
- `file_sign`
- `tenant_id`
- `state`
- `open_until`
- `created_at`
- `dispatch_started_at`
- `target_count`
- `success_count`
- `failed_count`
- `timeout_count`
- `shard_count`

Allowed states:

- `Collecting`
- `Dispatching`
- `Done`
- `Partial`
- `Failed`

Purpose:

- 记录 batch 生命周期和统计信息。

### 3. Batch Targets

Key:

- `AsyncBatchTargetsV2:{batchID}`

Type:

- `Set`

Member:

- `targetID`

Target ID format:

- `agentID:containerID`

Purpose:

- 收集 batch 内所有目标。
- `SADD` 天然去重，适合重复请求和重试场景。

### 4. Due Queue

Key:

- `AsyncBatchDueV2`

Type:

- `ZSet`

Member:

- `batchID`

Score:

- `open_until`

Purpose:

- `scheduler` 只消费到期 batch，而不是扫描所有 job。

### 5. Task State

Key:

- `AsyncTaskV2:{taskID}`

Type:

- `Hash`

Fields:

- `task_id`
- `batch_id`
- `target_id`
- `biz_id`
- `app_id`
- `tenant_id`
- `state`
- `created_at`
- `updated_at`
- `err_msg`

Allowed states:

- `Pending`
- `Running`
- `Success`
- `Failed`
- `Timeout`

Purpose:

- 保持现有 `AsyncDownloadStatus` 的查询方式。

## Request Flow

### Step 1. Validate Request

`rpc_sidecar.AsyncDownload` 保留现有这些职责：

- 基本参数校验
- 文件元信息校验
- `agentID/containerID` 识别
- agent 是否属于业务的校验

这些逻辑的性能优化可以后续再做，但本设计不改变它们的职责边界。

### Step 2. Build `fileVersionKey`

在参数和元信息校验通过后，构造本次请求的 `fileVersionKey`。

### Step 3. Generate `taskID`

每个请求都生成自己的 `taskID`，这一点对客户端保持不变。

### Step 4. Try to Join an Open Batch

服务端读取：

- `AsyncBatchOpenV2:{fileVersionKey}`

处理规则：

1. 如果存在 `batchID`
   - 读取 `AsyncBatchMetaV2:{batchID}`
   - 当且仅当下面条件都满足时，将请求加入该 batch：
     - `state = Collecting`
     - `open_until > now`
     - `target_count < max_targets_per_batch`
   - 执行 `SADD AsyncBatchTargetsV2:{batchID} targetID`
   - 仅当 `SADD` 返回新增成功时，执行 `HINCRBY AsyncBatchMetaV2:{batchID} target_count 1`
   - 写入 `AsyncTaskV2:{taskID}`
2. 如果任一条件不满足
   - 忽略该 open batch
   - 创建新 batch

### Step 5. Create a New Batch When Needed

新建 batch 时需要写入：

- `AsyncBatchMetaV2:{batchID}`
- `AsyncBatchOpenV2:{fileVersionKey}`
- `AsyncBatchDueV2`
- `AsyncBatchTargetsV2:{batchID}`
- `AsyncTaskV2:{taskID}`

推荐写入顺序：

1. `HSET AsyncBatchMetaV2:{batchID}`
2. `ZADD AsyncBatchDueV2 open_until batchID`
3. `SETEX AsyncBatchOpenV2:{fileVersionKey} batchID`
4. `SADD AsyncBatchTargetsV2:{batchID} targetID`
5. `HSET AsyncTaskV2:{taskID}`

该顺序的考虑是：

- `Meta` 和 `DueQueue` 是 batch 可执行的基础。
- `OpenV2` 只是请求入口提示，不是真相来源。
- `Targets` 和 `Task` 可以容忍短暂不一致，并由后台修复逻辑兜底。

### Step 6. Write the Task

无论是加入现有 batch，还是新建 batch，都写入：

- `AsyncTaskV2:{taskID}`

初始状态：

- `Pending`

### Accepted Consistency Model

在不使用 Lua 且兼容 Redis Cluster 的前提下，本设计显式接受以下情况：

- 同一文件的并发请求可能在极端场景下创建多个 batch。
- `OpenV2` 可能被后创建的 batch 覆盖。
- 个别请求可能出现 task 已写入但 target 未写入 batch 的短暂不一致。

这些情况都不应阻塞请求主链路，而应通过后续的恢复逻辑收敛。

### `target_count` Truth Source

`AsyncBatchMetaV2.target_count` 是请求路径上的快速判断字段，不是最终真相。

最终真相来源是：

- `SCARD AsyncBatchTargetsV2:{batchID}`

规则：

- 请求路径使用 `target_count` 做快速上限判断。
- `scheduler` 在开始分片前必须先执行 `SCARD`，并回写最新的 `target_count`。
- 若 `Meta.target_count` 与 `SCARD` 不一致，以 `SCARD` 为准。

## Batch Control

单个 batch 不能无上限增长。

关闭 batch 的条件只保留两个：

- 到达收集时间窗口：`open_until <= now`
- 达到 target 数量上限：`target_count >= max_targets_per_batch`

推荐初始参数：

- `collect_window = 10s`
- `max_targets_per_batch = 5000`

理由：

- 时间窗口提供聚合机会。
- 数量上限限制单个 batch 的规模。

在 `5w` 级并发下，这意味着同一文件可以自然形成多个 batch，而不是强行压成一个超级 batch。

## Scheduler Flow

### Step 1. Consume Due Batches

`scheduler` 周期性读取：

- `ZRANGEBYSCORE AsyncBatchDueV2 -inf now`

拿到所有已到期 batch。

### Step 2. Dispatch Lock

每个 batch 在执行前单独抢一次处理权：

- `SETNX AsyncBatchDispatchLockV2:{batchID}`

带 TTL，例如：

- `10m`

Purpose:

- 防止多个 scheduler 实例重复处理同一个 batch。

这里的锁只在执行阶段使用，不参与请求接入。

### Step 3. Transition to Dispatching

抢到锁后：

- 校验 `AsyncBatchMetaV2:{batchID}` 的 `state`
- 只有 `Collecting` 状态允许进入执行
- 将状态更新为 `Dispatching`
- 写入 `dispatch_started_at`

### Step 4. Load Targets and Split Shards

读取：

- `SMEMBERS AsyncBatchTargetsV2:{batchID}`

然后按固定大小切 shard。

推荐初始参数：

- `shard_size = 500`

示例：

- `5000` targets -> `10` 个 shard
- `5w` targets 如果按 batch 上限拆分后，大约会变成多个 batch，再各自切 shard

### Step 5. Execute Shards

每个 shard 创建一个 GSE 子任务。

batch 不直接映射到单个 GSE 任务，而是映射到多个 shard 子任务。

对每个 shard：

- 记录其开始时间
- 记录 GSE task 标识
- 轮询结果
- 将每个 target 的结果回写到对应 task

### Step 6. Aggregate Result

当全部 shard 完成后：

- `success_count + failed_count + timeout_count` 必须等于 batch 的 `target_count`

最终 batch 状态规则：

- 全部成功 -> `Done`
- 部分成功、部分失败或超时 -> `Partial`
- 全部失败 -> `Failed`

处理完成后：

- 从 `AsyncBatchDueV2` 中移除该 `batchID`

## Status Query

客户端继续通过 `taskID` 查询状态。

映射规则：

- `Pending`
  - task 已登记到 batch，但 batch 还未进入执行
- `Running`
  - 所属 batch 已进入 `Dispatching`，且该 target 正由某个 shard 处理
- `Success`
  - 该 target 已成功完成传输
- `Failed`
  - 该 target 对应 shard 返回失败
- `Timeout`
  - 该 target 在 batch/shard 超时后仍未完成

`AsyncDownloadStatus` 的对外返回保持现有语义：

- `SUCCESS`
- `DOWNLOADING`
- `FAILED`

其中：

- `Pending` 和 `Running` 都映射为 `DOWNLOADING`
- `Failed` 和 `Timeout` 都映射为 `FAILED`

## Recovery and Repair

本设计将恢复能力放在后台，而不是请求主链路中。

### Repair Case 1. Task Exists but Target Missing

可能原因：

- 写 `AsyncTaskV2` 成功，但 `SADD AsyncBatchTargetsV2` 失败

修复方式：

- 后台修复器根据 `task.batch_id + task.target_id` 检查 target 是否在 batch 中
- 若 batch 仍处于 `Collecting`，则补写 `SADD`
- 若 batch 已结束且 target 仍未进入任何 shard，则将 task 置为 `Failed`

### Repair Case 2. Batch Exists but Open Pointer Is Stale

可能原因：

- `OpenV2` 已过期或被覆盖

修复方式：

- 无需主动修复
- `OpenV2` 仅作为请求提示，不作为执行真相

### Repair Case 3. Dispatch Lock Lost

可能原因：

- scheduler 实例异常退出

修复方式：

- lock TTL 到期后，其他实例可重新抢占
- 重新处理前必须再次检查 batch `state`

### Repair Case 4. Partial Shard Failure

可能原因：

- 某些 GSE 子任务失败或超时

修复方式：

- 只更新失败 shard 涉及的 tasks
- 不回滚已成功 shard
- batch 最终状态允许为 `Partial`

## Migration and Rollout

为了避免长期保留新旧格式兼容代码，本设计采用版本隔离。

### Key Prefix Strategy

新逻辑只读写 `V2` key：

- `AsyncBatchOpenV2:*`
- `AsyncBatchMetaV2:*`
- `AsyncBatchTargetsV2:*`
- `AsyncBatchDueV2`
- `AsyncTaskV2:*`

旧逻辑继续处理旧 key：

- `AsyncDownloadJob:*`
- `AsyncDownloadTask:*`

### Rollout Strategy

发布流程：

1. 部署新版本 feed-server，写入并消费 V2 key。
2. 保留少量旧版本实例作为 drain worker，不接收新流量，只处理旧 key。
3. 待旧 `Pending/Running` job 清空后，下线旧实例。

这样可以避免在主代码里做运行时新旧格式分支判断。

## Metrics and Logging

V2 方案需要新增或调整以下指标：

- `async_batch_create_total`
- `async_batch_join_total`
- `async_batch_due_total`
- `async_batch_dispatch_total`
- `async_batch_target_count`
- `async_shard_dispatch_total`
- `async_shard_duration_seconds`
- `async_task_repair_total`

关键日志点：

- 创建新 batch
- 加入现有 batch
- batch 因时间窗口关闭
- batch 因 target 上限关闭
- batch 进入 `Dispatching`
- shard 创建和完成
- task 修复和超时

所有关键日志都必须带：

- `biz_id`
- `app_id`
- `file_version_key` 或其摘要
- `batch_id`
- `task_id`（如适用）

## Testing Strategy

测试范围聚焦在状态机闭环和高并发行为，不追求模拟所有 Redis 故障。

### Unit Tests

- 构造 `fileVersionKey`
- batch 创建和加入判断
- batch 关闭条件判断
- task 状态映射
- shard 切分逻辑

### Concurrency Tests

- 同一文件高并发请求下：
  - 请求路径不出现文件级串行等待
  - 最终形成有限个 batch
  - 没有 task 丢失

### Scheduler Tests

- due queue 消费
- dispatch lock 防重
- batch -> shards -> task 状态回写
- partial success 和 timeout 聚合

### Integration Tests

- 保留现有 `AsyncDownload` / `AsyncDownloadStatus` 协议不变
- 验证客户端在无感知的情况下完成一次完整异步下载流程

## Configuration

新增配置项建议：

- `async_download_v2_enabled`
- `collect_window_seconds`
- `max_targets_per_batch`
- `shard_size`
- `dispatch_lock_ttl_seconds`
- `task_ttl_seconds`
- `batch_ttl_seconds`

建议默认值：

- `async_download_v2_enabled = false`
- `collect_window_seconds = 10`
- `max_targets_per_batch = 5000`
- `shard_size = 500`
- `dispatch_lock_ttl_seconds = 600`
- `task_ttl_seconds = 86400`
- `batch_ttl_seconds = 86400`

通过 feature flag 控制上线节奏。

## Why This Design

本设计最终选择了“允许多个 batch、执行阶段再分片”的路线，而不是“请求阶段强行合并成唯一 job”，原因是：

- 它去掉了请求路径上的大锁。
- 它不依赖 Lua 和多 key 强事务，更适合 Redis Cluster。
- 它通过 `due queue + shard` 避免了超级 batch。
- 它保留现有客户端协议，改造面集中在服务端内部。
- 它通过 `V2 key + drain rollout` 避免长期兼容代码。

这让它在“简单、可扩展、可持续”三者之间取得了更稳妥的平衡。
