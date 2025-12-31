# 异步下载功能说明

## 概述

异步下载功能是 BSCP Feed Server 提供的文件批量下载能力，通过 GSE（Global Service Engine）实现 P2P 文件传输。该功能采用 **Task-Job 两级架构** 和 **时间窗口批次机制**，支持多个客户端实例同时请求下载同一个文件时进行批量优化处理。

---

## 核心概念

### Task（任务）- 请求级别

**定义**：Task 代表单个客户端实例的下载请求，一个请求对应一个 Task。

**特点**：
- 每个客户端请求都会创建一个唯一的 Task
- Task 包含单个目标客户端的信息（AgentID + ContainerID）
- Task 通过 `JobID` 字段关联到对应的 Job
- 客户端可以通过 TaskID 查询单个请求的下载状态

**TaskID 格式**：
```
AsyncDownloadTask:{bizID}:{appID}:{filePath}:{UUID}
```
- 包含 UUID，确保每个请求的唯一性

**Task 数据结构**：
```go
type AsyncDownloadTask struct {
    BizID             uint32    // 业务ID
    AppID             uint32    // 应用ID
    JobID             string    // 所属的 Job ID（关键字段）
    TargetAgentID     string    // 目标 Agent ID
    TargetContainerID string    // 目标 Container ID
    FilePath          string    // 文件路径
    FileName          string    // 文件名
    FileSignature     string    // 文件签名（SHA256）
    Status            string    // 任务状态：Pending/Running/Success/Failed/Timeout
    CreateTime        time.Time // 创建时间
}
```

---

### Job（作业）- 文件级别 + 时间窗口

**定义**：Job 代表一个文件在特定时间窗口内的批量下载作业。相同文件在同一时间窗口内的所有请求共享同一个 Job。

**特点**：
- 相同文件（bizID + appID + filePath）+ 相同时间窗口的请求共享同一个 Job
- Job 包含多个 targets（通过 Redis List 存储）
- Job 负责创建和管理 GSE 传输任务
- Job 统一管理所有 targets 的批量状态
- **时间窗口结束后，新请求会创建新的 Job**

**JobID 格式**：
```
AsyncDownloadJob:{bizID}:{appID}:{filePath}:{timeBucket}
```
- `timeBucket`：时间窗口标识，计算方式：`time.Now().Unix() / CollectWindowSeconds`
- 同一时间窗口内的请求使用相同的 JobID
- 跨时间窗口的请求使用不同的 JobID

**TargetsKey 格式**：
```
AsyncDownloadJob:Targets:{bizID}:{appID}:{filePath}:{timeBucket}
```

**Job 数据结构**：
```go
type AsyncDownloadJob struct {
    BizID              uint32    // 业务ID
    AppID              uint32    // 应用ID
    JobID              string    // Job ID（包含时间窗口）
    FilePath           string    // 文件路径
    FileName           string    // 文件名
    FileSignature      string    // 文件签名（SHA256）
    TargetFileDir      string    // 目标文件目录
    TargetUser         string    // 目标用户
    GSETaskID          string    // GSE 任务ID（一个 Job 对应一个 GSE 任务）
    Status             string    // Job 状态：Pending/Running/Success/Failed/Timeout
    CreateTime         time.Time // 创建时间
    ExecuteTime        time.Time // 执行时间
    
    // 批量状态管理（key: "AgentID:ContainerID"）
    SuccessTargets     map[string]gse.TransferFileResultDataResultContent
    FailedTargets      map[string]gse.TransferFileResultDataResultContent
    DownloadingTargets map[string]gse.TransferFileResultDataResultContent
    TimeoutTargets     map[string]gse.TransferFileResultDataResultContent
}
```

---

### 时间窗口机制

**定义**：将时间按固定间隔（默认 15 秒）划分为多个窗口，同一窗口内的请求合并到同一个 Job 处理。

**配置**：
```go
const (
    // CollectWindowSeconds 收集窗口时间（秒）
    CollectWindowSeconds int64 = 15
)
```

**计算方式**：
```go
// 获取当前时间窗口
func getTimeBucket() int64 {
    return time.Now().Unix() / CollectWindowSeconds
}

// 示例：
// 当前时间: 2024-12-31 16:00:00 (Unix: 1735660800)
// timeBucket = 1735660800 / 15 = 115710720
```

**时间窗口示例**：
```
时间线：
├─────── 窗口 115710720 ───────┼─────── 窗口 115710721 ───────┤
│      [16:00:00, 16:00:15)     │      [16:00:15, 16:00:30)     │
│                               │                               │
│  请求1,2,3 → Job_115710720    │  请求4,5 → Job_115710721      │
│  targets: [1,2,3]             │  targets: [4,5]               │
│                               │                               │
│  窗口结束 → 开始处理          │  窗口结束 → 开始处理          │
│  → Success                    │  → Success                    │
```

**设计目的**：
- **解决 Job 状态无法结束问题**：每个时间窗口独立的 Job，状态流转清晰
- **避免 Targets 无限增长**：每个 Job 只包含一个窗口内的 targets
- **新请求自动隔离**：新请求不影响正在处理的 Job

---

## Task 和 Job 的关系

### 关系图

```
┌─────────────────────────────────────────────────────────────────┐
│                    Job (文件级别 + 时间窗口)                      │
│  JobID: AsyncDownloadJob:1:1:/test/path/file.txt:115710720      │
│  GSETaskID: gse-task-123                                        │
│  Status: Running                                                │
│  FileSignature: abc123...                                       │
│                                                                 │
│  Targets (存储在 Redis List):                                   │
│    AsyncDownloadJob:Targets:1:1:/test/path/file.txt:115710720   │
│    - {AgentID: "agent-1", ContainerID: "container-1"}           │
│    - {AgentID: "agent-2", ContainerID: "container-2"}           │
│    - {AgentID: "agent-3", ContainerID: "container-3"}           │
│    - ...                                                        │
└─────────────────────────────────────────────────────────────────┘
                    ▲
                    │ JobID 引用
                    │
    ┌───────────────┼───────────────┬───────────────┐
    │               │               │               │
┌───┴───┐      ┌───┴───┐      ┌───┴───┐      ┌───┴───┐
│ Task1 │      │ Task2 │      │ Task3 │      │ TaskN │
│ UUID1 │      │ UUID2 │      │ UUID3 │      │ UUIDN │
│       │      │       │      │       │      │       │
│ JobID │      │ JobID │      │ JobID │      │ JobID │
│ ────→ │      │ ────→ │      │ ────→ │      │ ────→ │
└───────┘      └───────┘      └───────┘      └───────┘
```

### 关系说明

- **1:N 关系**：1 个 Job 可以包含多个 Tasks（同一时间窗口内）
- **时间窗口隔离**：不同时间窗口的请求属于不同的 Job
- **关联方式**：Task 通过 `JobID` 字段关联到 Job
- **数据存储**：
  - Task 存储在 Redis，key 为 `taskID`
  - Job 存储在 Redis，key 为 `jobID`（包含 timeBucket）
  - Targets 存储在 Redis List，key 为 `targetsKey`（包含 timeBucket）

---

## 工作流程

### 1. 创建阶段（请求处理）

**流程**：
```
客户端请求 → CreateAsyncDownloadTask
    ↓
计算当前时间窗口 (timeBucket)
    ↓
创建/获取 Job (upsertAsyncDownloadJob)
    ├─ Job 不存在 → 使用 SetNX 原子创建新 Job + LPUSH 添加 target
    └─ Job 已存在 → LPUSH 添加 target 到 Redis List
    ↓
创建 Task
    ├─ 设置 JobID 指向 Job（包含 timeBucket）
    ├─ 包含单个 target 信息
    └─ 存储到 Redis
    ↓
返回 taskID 给客户端
```

**关键代码**：
```go
// 计算时间窗口
timeBucket := getTimeBucket()
jobKey := GetJobKey(bizID, appID, fullPath, timeBucket)
targetsKey := GetTargetsKey(bizID, appID, fullPath, timeBucket)

// 1. 使用 SetNX 原子创建 Job
ok, err := ad.cs.Redis().SetNX(kt.Ctx, jobKey, string(js), 30*60)

// 2. 使用 LPUSH 原子添加 target
err := ad.cs.Redis().LPush(kt.Ctx, targetsKey, string(targetData))

// 3. 创建 Task
task := &types.AsyncDownloadTask{
    JobID: jobKey,  // 关联到 Job（包含 timeBucket）
    TargetAgentID: targetAgentID,
    TargetContainerID: targetContainerID,
    // ...
}
```

**优化点**：
- 使用 `SetNX` *单条命令原子性* 创建 Job 记录，避免同一时间窗口内重复创建同一个 Job（注意：与后续操作组合时，并非整个流程强原子）
- 使用 `LPUSH` *单条命令原子性* 添加 target 元素，无需在 Redis 层面加锁
- JobID 包含时间窗口，实现不同时间窗口之间的 Key 隔离，配合上层的重试／幂等逻辑来缓解（而非完全消除）并发下的状态不一致风险

---

### 2. 收集阶段（Pending 状态）

**流程**：
```
Scheduler 每 5 秒扫描所有 Jobs
    ↓
发现 Pending 状态的 Job
    ↓
检查时间窗口是否结束 (IsTimeWindowExpired)
    ├─ 窗口未结束 → 继续收集 targets（等待更多请求）
    └─ 窗口已结束 → 开始处理（进入 Running 状态）
```

**判断逻辑**：
```go
// 根据 JobID 中的 timeBucket 计算窗口结束时间
func IsTimeWindowExpired(jobKey string) bool {
    timeBucket := ParseTimeBucketFromJobKey(jobKey)
    if timeBucket == 0 {
        return true  // 解析失败，保守策略
    }
    // 窗口结束时间 = (timeBucket + 1) * CollectWindowSeconds
    windowEndTime := time.Unix((timeBucket+1)*CollectWindowSeconds, 0)
    return time.Now().After(windowEndTime)
}
```

**设计目的**：
- **批量优化**：时间窗口内的请求自动合并到一个 Job 中处理
- **减少 GSE 任务**：避免为每个请求单独创建 GSE 任务
- **状态清晰**：每个时间窗口独立，Job 状态不会被后续请求影响

---

### 3. 处理阶段（Running 状态）

**流程**：
```
handleDownload(job)
    ↓
1. 更新 Job 状态为 Running
    ↓
2. 从 Redis List 读取所有 targets
    ├─ 使用 GetTargetsKeyFromJobKey(job.JobID) 获取 targetsKey
    ├─ 解析每个 target（AgentID + ContainerID）
    └─ 构建 targetAgents 列表
    ↓
3. 下载文件到本地（只下载一次）
    ├─ 检查文件是否已存在（fileLock 保护）
    ├─ 不存在则从 Repository 下载
    └─ 保存到本地缓存目录
    ↓
4. 创建 GSE 文件传输任务
    ├─ 一个 Job 对应一个 GSE 任务
    ├─ 包含所有 targets
    └─ 返回 GSETaskID
    ↓
5. 更新 Job 状态和 GSETaskID
```

**关键代码**：
```go
// 从 JobID 解析出对应的 targetsKey
targetsKey := GetTargetsKeyFromJobKey(job.JobID)
targetsData, err := a.bds.LRange(a.ctx, targetsKey, 0, -1)

// 创建 GSE 任务（包含所有 targets）
taskID, err := createTaskFunc(a.ctx, a.serverAgentID, a.serverContainerID, 
    sourceDir, agentUser, signature, job.TargetFileDir, targetAgents)
```

---

### 4. 状态同步阶段（Running 状态持续监控）

**流程**：
```
checkJobStatus(job)
    ↓
1. 查询 GSE 任务结果
    ├─ 调用 gse.TransferFileResult(job.GSETaskID)
    └─ 获取所有 targets 的传输状态
    ↓
2. 更新每个 target 的状态
    ├─ Success: ErrorCode == 0
    ├─ Downloading: ErrorCode == 115
    ├─ Failed: 其他 ErrorCode
    └─ Upload Failed: 所有 targets 标记为 Failed
    ↓
3. 同步更新所有相关 Tasks 的状态
    ├─ 通过 Job 的 target 状态映射
    └─ 更新 Task 的 Status 字段
    ↓
4. 判断 Job 最终状态
    ├─ 所有 targets 成功 → Job Status = Success
    ├─ 所有 targets 完成且有失败 → Job Status = Failed
    ├─ 超时（10分钟）→ Job Status = Timeout
    └─ 否则继续监控
```

**状态同步机制**：
```go
// GetAsyncDownloadTaskStatus 方法中
// Task 的状态从 Job 的 target 状态中获取
if _, ok := job.SuccessTargets[fmt.Sprintf("%s:%s", 
    task.TargetAgentID, task.TargetContainerID)]; ok {
    task.Status = types.AsyncDownloadJobStatusSuccess
}
```

---

## 状态流转

### Job 状态流转

```
Pending (收集期 - 等待时间窗口结束)
    ↓ (窗口结束后)
Running (处理中)
    ↓
    ├─→ Success (所有 targets 成功)
    ├─→ Failed (有 targets 失败)
    └─→ Timeout (超时 10 分钟)
```

### Task 状态流转

```
Pending (初始状态)
    ↓ (Job 进入 Running)
Running (Job 处理中)
    ↓
    ├─→ Success (对应 target 成功)
    ├─→ Failed (对应 target 失败)
    └─→ Timeout (Job 超时)
```

---

## 数据存储

### Redis 存储结构

**1. Task 存储**：
- **Key**: `AsyncDownloadTask:{bizID}:{appID}:{filePath}:{UUID}`
- **Value**: Task JSON 数据
- **TTL**: 30 分钟

**2. Job 存储**：
- **Key**: `AsyncDownloadJob:{bizID}:{appID}:{filePath}:{timeBucket}`
- **Value**: Job JSON 数据
- **TTL**: 30 分钟

**3. Targets 存储（Redis List）**：
- **Key**: `AsyncDownloadJob:Targets:{bizID}:{appID}:{filePath}:{timeBucket}`
- **Value**: Target JSON 数组（每个元素是一个 target）
- **TTL**: 30 分钟
- **操作**：
  - `LPUSH`: 添加 target（原子操作）
  - `LRange`: 读取所有 targets
  - `LLen`: 获取 targets 数量

---

## 辅助函数

### Key 生成函数

```go
// GetJobKey 获取带时间窗口的 Job Key
func GetJobKey(bizID, appID uint32, fullPath string, timeBucket int64) string {
    return fmt.Sprintf("AsyncDownloadJob:%d:%d:%s:%d", bizID, appID, fullPath, timeBucket)
}

// GetTargetsKey 获取带时间窗口的 Targets Key
func GetTargetsKey(bizID, appID uint32, fullPath string, timeBucket int64) string {
    return fmt.Sprintf("AsyncDownloadJob:Targets:%d:%d:%s:%d", bizID, appID, fullPath, timeBucket)
}

// GetTargetsKeyFromJobKey 从 JobKey 获取对应的 TargetsKey
func GetTargetsKeyFromJobKey(jobKey string) string {
    return strings.Replace(jobKey, "AsyncDownloadJob:", "AsyncDownloadJob:Targets:", 1)
}
```

### 时间窗口函数

```go
// getTimeBucket 获取当前时间窗口
func getTimeBucket() int64 {
    return time.Now().Unix() / CollectWindowSeconds
}

// ParseTimeBucketFromJobKey 从 JobKey 中解析时间窗口
func ParseTimeBucketFromJobKey(jobKey string) int64

// IsTimeWindowExpired 判断时间窗口是否已结束
func IsTimeWindowExpired(jobKey string) bool
```

---

## 设计优势

### 1. 批量优化
- **文件只下载一次**：相同文件的所有请求共享一次下载
- **GSE 任务合并**：同一时间窗口内的 targets 合并到一个 GSE 任务中
- **资源节约**：减少网络带宽和存储空间

### 2. 并发处理
- **无锁设计**：使用 Redis 单条命令原子操作（SetNX、LPUSH）避免锁竞争
- **高并发支持**：支持大量客户端同时请求
- **性能优化**：15 秒时间窗口，批量处理提高效率
- **注意**：虽然单条 Redis 命令是原子的，但 SetNX + LPush 的组合操作不是强原子，存在中间状态。通过重试机制和状态检查来缓解竞态条件

### 3. 状态管理
- **统一管理**：Job 统一管理所有 targets 的状态
- **灵活查询**：客户端可通过 TaskID 查询单个请求状态
- **实时同步**：Task 状态实时从 Job 同步

### 4. 时间窗口隔离
- **状态清晰**：每个时间窗口独立的 Job，状态流转清晰
- **避免无限增长**：Targets 不会无限增长
- **自动隔离**：新请求自动进入新窗口，不影响正在处理的 Job

### 5. 容错机制
- **超时处理**：10 分钟超时自动终止
- **失败处理**：单个 target 失败不影响其他 targets
- **状态持久化**：所有状态存储在 Redis，支持服务重启

---

## 使用示例

### 客户端请求下载

```go
// 1. 创建下载任务
taskID, err := service.CreateAsyncDownloadTask(
    kt, bizID, appID, filePath, fileName,
    targetAgentID, targetContainerID, targetUser, targetDir, signature,
)

// 2. 查询任务状态
status, err := service.GetAsyncDownloadTaskStatus(kt, bizID, taskID)
// 返回: "Pending" / "Running" / "Success" / "Failed" / "Timeout"
```

### 内部处理流程

```go
// Scheduler 自动处理
// 1. 扫描所有 Jobs
// 2. 检查时间窗口是否结束
// 3. 窗口结束后创建 GSE 任务
// 4. 监控 GSE 任务状态
// 5. 更新 Job 和 Task 状态
```

---

## 关键配置

### 时间窗口配置
- **窗口时间**：15 秒（`CollectWindowSeconds = 15`）
- **目的**：平衡批量优化和响应速度

### 超时配置
- **Job 超时**：10 分钟（`JobTimeoutSeconds = 10 * 60`）
- **目的**：防止任务无限期运行

### TTL 配置
- **Task TTL**：30 分钟
- **Job TTL**：30 分钟
- **Targets List TTL**：30 分钟

---

## 注意事项

### 1. 并发安全
- ✅ 使用 Redis *单条命令原子操作*（SetNX、LPUSH）保证单条命令的原子性
- ✅ 使用分布式锁（RedisLock）保护 Job 处理
- ✅ 使用文件锁（FileLock）保护文件下载
- ⚠️ **边界情况**：
  - SetNX 成功但 LPush 失败时会回滚已创建的 Job，避免产生孤立任务
  - Job 状态从 Pending 转为 Running 时，会检查状态并拒绝添加新的 target，避免 targets 被添加但不会被处理
  - 虽然单条 Redis 命令是原子的，但组合操作（如 SetNX + LPush）不是强原子，存在中间状态窗口
  - 通过重试机制、状态检查和回滚逻辑来缓解（而非完全消除）并发下的状态不一致风险

### 2. 数据一致性
- ⚠️ Task 状态从 Job 同步，可能存在短暂延迟
- ⚠️ Job 状态更新需要持有锁，避免并发更新

### 3. 时间窗口边界
- ⚠️ 请求在时间窗口边界附近可能进入不同的 Job
- ⚠️ 这是预期行为，不影响功能正确性

### 4. 性能考虑
- ✅ 15 秒时间窗口平衡了批量优化和响应速度
- ✅ Redis List 存储 targets，支持高并发写入
- ✅ 文件只下载一次，减少存储和带宽消耗

---

## 相关文件

- **业务逻辑**：`cmd/feed-server/bll/asyncdownload/asyncdownload.go`
- **调度器**：`cmd/feed-server/bll/asyncdownload/scheduler.go`
- **类型定义**：`cmd/feed-server/bll/types/types.go`
- **GSE 客户端**：`internal/components/gse/file.go`

---

## 总结

异步下载功能通过 **Task-Job 两级架构** 和 **时间窗口批次机制** 实现了高效的批量文件传输：

- **Task**：代表单个客户端请求，提供细粒度的状态查询
- **Job**：代表文件级别 + 时间窗口的批量作业，实现批量优化和资源节约
- **时间窗口**：自动隔离不同时间段的请求，避免 Job 状态混乱
- **关系**：1 个 Job 包含多个 Tasks（同一时间窗口内），通过 JobID 关联
- **优势**：批量处理、并发安全、状态清晰、自动隔离

这种设计在保证功能完整性的同时，最大化了系统性能和资源利用率，并解决了 Job 状态无法结束和 Targets 无限增长的问题。
