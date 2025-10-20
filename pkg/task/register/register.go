package register

import (
	"github.com/TencentBlueKing/bk-bscp/pkg/task/executor/cmdb"
	"github.com/TencentBlueKing/bk-bscp/pkg/task/executor/hello"
)

// RegisterExecutor register all executor.
// RegisterExecutor 中可以补充参数，比如执行器依赖的配置，执行器依赖的第三方服务等
func RegisterExecutor() {
	// 注册 hello 执行器，
	e := &hello.HelloExecutor{}
	hello.Register(e)

	c := cmdb.NewSyncBizExecutor()
	cmdb.Register(c)

	// 注册回调 等待补充
}

// RegisterHello register
// nolint: revive
func RegisterHello() {
	// 注册 hello 执行器，
	e := &hello.HelloExecutor{}
	hello.Register(e)
}
