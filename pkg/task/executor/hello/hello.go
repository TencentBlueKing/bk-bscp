package hello

import (
	"fmt"
	"strconv"

	istep "github.com/Tencent/bk-bcs/bcs-common/common/task/steps/iface"
)

const (
	// AddStepName add step name
	AddStepName istep.StepName = "Add"
)

// HelloExecutor hello step executor
type HelloExecutor struct {
}

// Add implements istep.Step.
func (e *HelloExecutor) Add(c *istep.Context) error {
	a, exists := c.GetParam("a")
	if !exists {
		return fmt.Errorf("a not exists")
	}
	b, exists := c.GetParam("b")
	if !exists {
		return fmt.Errorf("b not exists")
	}

	aInt, err := strconv.Atoi(a)
	if err != nil {
		return err
	}
	bInt, err := strconv.Atoi(b)
	if err != nil {
		return err
	}

	fmt.Printf("a(%d) + b(%d) = %d\n", aInt, bInt, aInt+bInt)
	return nil
}

// Register register step
func Register(e *HelloExecutor) {
	istep.Register(AddStepName, istep.StepExecutorFunc(e.Add))
}
