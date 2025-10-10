// Package task do the task flow control
package task

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	machineryConf "github.com/RichardKnop/machinery/v2/config"
	"github.com/samber/lo"

	"github.com/Tencent/bk-bcs/bcs-common/common/task"
	etcdbackend "github.com/Tencent/bk-bcs/bcs-common/common/task/backends/etcd"
	etcdbroker "github.com/Tencent/bk-bcs/bcs-common/common/task/brokers/etcd"
	etcdlock "github.com/Tencent/bk-bcs/bcs-common/common/task/locks/etcd"
	etcdrevoker "github.com/Tencent/bk-bcs/bcs-common/common/task/revokers/etcd"
	mysqlstore "github.com/Tencent/bk-bcs/bcs-common/common/task/stores/mysql"
	itypes "github.com/Tencent/bk-bcs/bcs-common/common/task/types"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

var (
	// NewByTaskBuilder alias task.NewByTaskBuilder
	NewByTaskBuilder = task.NewByTaskBuilder
	// TaskStatusSlice ...
	TaskStatusSlice = []string{
		itypes.TaskStatusInit,
		itypes.TaskStatusRunning,
		itypes.TaskStatusSuccess,
		itypes.TaskStatusFailure,
		itypes.TaskStatusTimeout,
		itypes.TaskStatusRevoked,
		itypes.TaskStatusNotStarted,
	}

	// defaultWorkerNum 默认worker数量
	defaultWorkerNum = 200
)

// TaskManager task manager
type TaskManager struct {
	*task.TaskManager
}

// NewTaskMgr new task manager
func NewTaskMgr(ctx context.Context, etcdConfig cc.Etcd, dbConfig cc.Database) (*TaskManager, error) {
	name, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	name = fmt.Sprintf("%s-%d", name, os.Getpid())
	// etcd tls config
	tlsConfig, err := parseTLSConfig(&etcdConfig.TLS)
	if err != nil {
		return nil, err
	}

	conf := &machineryConf.Config{
		DefaultQueue:    "bk-bscp",
		Broker:          etcdConfig.Endpoints[0],
		ResultBackend:   etcdConfig.Endpoints[0],
		Lock:            etcdConfig.Endpoints[0],
		ResultsExpireIn: 60 * 60 * 2, // 2个小时过期
		NoUnixSignals:   true,
		TLSConfig:       tlsConfig,
	}
	broker, err := etcdbroker.New(ctx, conf)
	if err != nil {
		return nil, err
	}

	revoker, err := etcdrevoker.New(ctx, conf)
	if err != nil {
		return nil, err
	}

	backend, err := etcdbackend.New(ctx, conf)
	if err != nil {
		return nil, err
	}

	lock, err := etcdlock.New(ctx, conf, 3)
	if err != nil {
		return nil, err
	}
	dsn := lo.Ternary(dbConfig.Database != "", fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbConfig.User, dbConfig.Password, dbConfig.Endpoints[0], dbConfig.Database), "")
	store, err := mysqlstore.New(
		dsn,
		mysqlstore.WithDebug(true),
	)
	if err != nil {
		return nil, err
	}

	// Create server instance
	mgr := task.NewTaskManager()

	cfg := task.ManagerConfig{
		ModuleName:   "bk-bscp",
		Broker:       broker,
		Revoker:      revoker,
		Backend:      backend,
		Lock:         lock,
		Store:        store,
		ServerConfig: conf,
		WorkerName:   name,
		WorkerNum:    defaultWorkerNum, // 200并发
	}
	if err := mgr.Init(&cfg); err != nil {
		return nil, err
	}

	return &TaskManager{mgr}, nil
}

// Dispatch same task.Dispatch but log when error
func (taskMgr *TaskManager) Dispatch(t *itypes.Task) {
	err := taskMgr.TaskManager.Dispatch(t)
	if err != nil {
		logs.Errorf("dispatch task taskID[%s] taskIndex[%s] %v error: %s", t.TaskID, t.TaskIndex, t, err)
	}
}

// EnsureTable auto migration
func (taskMgr *TaskManager) EnsureTable(ctx context.Context) error {
	return task.GetGlobalStorage().EnsureTable(ctx)
}

func parseTLSConfig(tlsConfig *cc.TLSConfig) (*tls.Config, error) {
	if tlsConfig.CertFile == "" || tlsConfig.CAFile == "" || tlsConfig.KeyFile == "" {
		return nil, nil
	}

	cert, err := tls.LoadX509KeyPair(tlsConfig.CertFile, tlsConfig.KeyFile)
	if err != nil {
		return nil, err
	}

	caData, err := os.ReadFile(tlsConfig.CAFile)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caData)

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
		MinVersion:   tls.VersionTLS12,
	}

	return tlsCfg, nil
}
