# Task 任务框架

Task 模块是基于 [bk-bcs machinery](https://github.com/Tencent/bk-bcs/tree/master/bcs-common/common/task) 实现的分布式任务调度框架，支持任务的分发、执行、监控和管理。

## 架构概述

Task 框架采用分层架构设计：

```
├── task.go          # 任务管理器，负责任务调度和管理
├── builder/         # 任务构建器，定义任务的结构和元信息
├── executor/        # 任务执行器，实现具体的任务执行逻辑
├── step/           # 任务步骤，定义任务的具体执行步骤
└── register/       # 注册器，注册所有的任务执行器
```

### 核心组件

- **TaskManager**: 任务管理器，负责任务的分发、调度和状态管理
- **TaskBuilder**: 任务构建器，定义任务的基本信息和执行步骤
- **TaskExecutor**: 任务执行器，实现具体的任务执行逻辑
- **Step**: 任务步骤，定义单个执行步骤的参数和配置

## 快速开始

### 1. 初始化任务管理器

```go
import (
    "context"
    "github.com/TencentBlueKing/bk-bscp/internal/task"
    "github.com/TencentBlueKing/bk-bscp/internal/task/register"
    "github.com/TencentBlueKing/bk-bscp/pkg/cc"
)

func initTaskManager() (*task.TaskManager, error) {
    // 1. 首先注册所有执行器（必须在创建TaskManager之前）
    register.RegisterExecutor()
    
    // 2. 创建任务管理器
    taskManager, err := task.NewTaskMgr(
        context.Background(),
        cc.DataService().Service.Etcd,        // etcd配置
        cc.DataService().Sharding.AdminDatabase, // 数据库配置
    )
    if err != nil {
        return nil, err
    }
    
    // 3. 启动任务管理器（通常在goroutine中运行）
    go func() {
        if err := taskManager.Run(); err != nil {
            taskManager.Stop()
            // 处理错误
        }
    }()
    
    return taskManager, nil
}
```

### 2. 实现一个新的任务

创建一个新任务需要实现以下四个部分：

#### 2.1 定义执行器 (Executor)

在 `executor/` 目录下创建任务执行器：

```go
// internal/task/executor/mytask/mytask.go
package mytask

import (
    "fmt"
    istep "github.com/Tencent/bk-bcs/bcs-common/common/task/steps/iface"
)

const (
    // ProcessStepName 处理步骤名称
    ProcessStepName istep.StepName = "Process"
)

// MyTaskExecutor 自定义任务执行器
type MyTaskExecutor struct {
    // 可以添加执行器需要的依赖，如数据库连接、配置等
}

// Process 实现具体的处理逻辑
func (e *MyTaskExecutor) Process(c *istep.Context) error {
    // 获取步骤参数
    param1, exists := c.GetParam("param1")
    if !exists {
        return fmt.Errorf("param1 not exists")
    }
    
    // 获取payload（复杂对象参数）
    payload := c.GetPayload()
    if payload != nil {
        // 处理payload
    }
    
    // 执行具体的业务逻辑
    fmt.Printf("Processing with param1: %s\n", param1)
    
    return nil
}

// Register 注册步骤
func Register(e *MyTaskExecutor) {
    istep.Register(ProcessStepName, istep.StepExecutorFunc(e.Process))
}
```

#### 2.2 定义步骤 (Step)

在 `step/` 目录下创建步骤定义：

```go
// internal/task/step/mytask/mytask.go
package mytask

import (
    "time"
    "github.com/Tencent/bk-bcs/bcs-common/common/task/types"
    "github.com/TencentBlueKing/bk-bscp/internal/task/executor/mytask"
)

// Process 创建处理步骤
func Process(param1 string, complexParam interface{}) *types.Step {
    step := types.NewStep("my-process-task", mytask.ProcessStepName.String()).
        SetAlias("process").                    // 设置别名
        AddParam("param1", param1).            // 添加字符串参数
        SetMaxExecution(30 * time.Second).     // 设置最大执行时间
        SetMaxTries(3)                         // 设置最大重试次数
    
    // 如果有复杂对象参数，使用 SetPayload
    if complexParam != nil {
        step.SetPayload(complexParam)
    }
    
    return step
}
```

#### 2.3 创建任务构建器 (Builder)

在 `builder/` 目录下创建任务构建器：

```go
// internal/task/builder/mytask/mytask.go
package mytask

import (
    "github.com/Tencent/bk-bcs/bcs-common/common/task/types"
    "github.com/TencentBlueKing/bk-bscp/internal/task/step/mytask"
)

type myTask struct {
    param1      string
    complexData interface{}
    taskIndex   string
}

// NewMyTask 创建自定义任务
func NewMyTask(param1 string, complexData interface{}, taskIndex string) types.TaskBuilder {
    return &myTask{
        param1:      param1,
        complexData: complexData,
        taskIndex:   taskIndex,
    }
}

// TaskInfo 实现 TaskBuilder 接口 - 定义任务基本信息
func (t *myTask) TaskInfo() types.TaskInfo {
    return types.TaskInfo{
        TaskName:      "my-custom-task",           // 任务名称
        TaskType:      "custom",                   // 任务类型
        TaskIndexType: "business-key",             // 索引类型
        TaskIndex:     t.taskIndex,                // 任务索引（用于去重和查找）
        Creator:       "system",                   // 创建者
    }
}

// Steps 实现 TaskBuilder 接口 - 定义任务执行步骤
func (t *myTask) Steps() ([]*types.Step, error) {
    // 可以包含多个步骤，按顺序执行
    steps := []*types.Step{
        mytask.Process(t.param1, t.complexData),
        // 可以添加更多步骤
    }
    return steps, nil
}

// FinalizeTask 实现 TaskBuilder 接口 - 任务完成后的处理
func (t *myTask) FinalizeTask(task *types.Task) error {
    // 可以设置任务完成后的回调处理
    // 比如状态更新、通知等
    return nil
}
```

#### 2.4 注册执行器

在 `register/register.go` 中注册新的执行器：

```go
package register

import (
    "github.com/TencentBlueKing/bk-bscp/internal/task/executor/hello"
    "github.com/TencentBlueKing/bk-bscp/internal/task/executor/mytask" // 新增
)

// RegisterExecutor 注册所有执行器
func RegisterExecutor() {
    // 注册 hello 执行器
    e := &hello.HelloExecutor{}
    hello.Register(e)

    // 注册自定义执行器
    myExecutor := &mytask.MyTaskExecutor{}
    mytask.Register(myExecutor)
    
    // 可以在这里传入执行器需要的依赖，如配置、数据库连接等
}
```

### 3. 发送和执行任务

```go
import (
    "github.com/TencentBlueKing/bk-bscp/internal/task"
    "github.com/TencentBlueKing/bk-bscp/internal/task/builder/mytask"
)

func sendTask(taskManager *task.TaskManager) error {
    // 1. 创建任务
    taskBuilder := mytask.NewMyTask("test-param", complexData, "unique-task-id")
    
    // 2. 构建任务
    task, err := task.NewByTaskBuilder(taskBuilder)
    if err != nil {
        return fmt.Errorf("create task failed: %v", err)
    }
    
    // 3. 分发任务
    taskManager.Dispatch(task)
    
    return nil
}
```

## 任务状态管理

任务支持以下状态：

- `TaskStatusInit`: 初始化状态
- `TaskStatusRunning`: 运行中
- `TaskStatusSuccess`: 执行成功
- `TaskStatusFailure`: 执行失败
- `TaskStatusTimeout`: 执行超时
- `TaskStatusRevoked`: 已撤销
- `TaskStatusNotStarted`: 未开始

可以通过任务管理器查询任务状态和执行结果。

## 最佳实践

### 1. 错误处理

```go
func (e *MyTaskExecutor) Process(c *istep.Context) error {
    // 验证参数
    param, exists := c.GetParam("required_param")
    if !exists {
        return fmt.Errorf("required parameter missing: required_param")
    }
    
    // 执行业务逻辑，适当处理错误
    if err := doSomething(param); err != nil {
        return fmt.Errorf("business logic failed: %v", err)
    }
    
    return nil
}
```

### 2. 超时和重试配置

```go
func Process(param string) *types.Step {
    return types.NewStep("long-running-task", mytask.ProcessStepName.String()).
        AddParam("param", param).
        SetMaxExecution(5 * time.Minute).  // 设置合理的超时时间
        SetMaxTries(3)                     // 设置重试次数
}
```

### 3. 资源清理

```go
func (t *myTask) FinalizeTask(task *types.Task) error {
    // 在任务完成后进行资源清理
    defer cleanupResources()
    
    // 发送完成通知
    if task.State == types.TaskStatusSuccess {
        notifySuccess(task.TaskID)
    } else {
        notifyFailure(task.TaskID, task.Message)
    }
    
    return nil
}
```

## 命令行工具

框架提供了命令行工具用于任务的管理和测试：

```bash
# 启动任务worker
./bk-bscp-dataservice task run

# 发送测试任务
./bk-bscp-dataservice task send --a 10 --b 20
```

## 配置说明

任务框架依赖以下配置：

```yaml
# etcd配置 - 用于任务分发和协调
etcd:
  endpoints: ["127.0.0.1:2379"]
  
# 数据库配置 - 用于任务状态存储
database:
  endpoints: ["127.0.0.1:3306"]
  user: "bscp"
  password: "password"
  database: "bscp"
```

## 监控和日志

- 任务执行日志会自动记录到系统日志中
- 任务状态变更会持久化到数据库
- 支持通过TaskID查询任务执行历史和状态

## 注意事项

1. **执行器注册顺序**: 必须在创建 `TaskManager` 之前调用 `register.RegisterExecutor()`
2. **任务幂等性**: 任务可能会被重试执行，确保任务逻辑具有幂等性
3. **资源管理**: 长时间运行的任务需要合理设置超时时间和资源清理逻辑
4. **错误处理**: 执行器中的错误会导致任务重试，确保适当的错误处理和日志记录
5. **任务索引**: TaskIndex 用于任务去重和查找，确保其唯一性，一般存储操作对应的一个唯一ID，比如进程ID

## 参考示例

完整的示例可以参考 `hello` 任务的实现：

- `executor/hello/hello.go` - 执行器实现
- `step/hello/hello.go` - 步骤定义  
- `builder/hello/hello.go` - 任务构建器
- `cmd/data-service/cmd/task.go` - 命令行使用示例
