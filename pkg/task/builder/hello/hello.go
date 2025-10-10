package hello

import (
	"github.com/Tencent/bk-bcs/bcs-common/common/task/types"
	"github.com/TencentBlueKing/bk-bscp/pkg/task/step/hello"
)

type helloTask struct {
	a int
	b int
}

// NewHelloTask 创建一个 hello 任务
func NewHelloTask(a, b int) types.TaskBuilder {
	return &helloTask{a: a, b: b}
}

// FinalizeTask implements types.TaskBuilder.
func (h *helloTask) FinalizeTask(t *types.Task) error {
	// 设置一些通用的回调，比如执行结果回调
	return nil
}

// Steps implements types.TaskBuilder.
func (h *helloTask) Steps() ([]*types.Step, error) {
	// 构建任务的步骤
	return []*types.Step{hello.Add(h.a, h.b)}, nil
}

// TaskInfo implements types.TaskBuilder.
func (h *helloTask) TaskInfo() types.TaskInfo {
	return types.TaskInfo{
		TaskName:      "hello",
		TaskType:      "example",
		TaskIndexType: "key", // 任务一个索引类型，比如key，uuid等，
		TaskIndex:     "1",   // 任务索引，比如具体key，uuid等
		Creator:       "admin",
	}
}
