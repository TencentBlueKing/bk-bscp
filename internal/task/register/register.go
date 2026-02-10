/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package register

import (
	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	pushmanager "github.com/TencentBlueKing/bk-bscp/internal/components/push_manager"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/repository"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/lock"
	cmdbGse "github.com/TencentBlueKing/bk-bscp/internal/task/executor/cmdb_gse"
	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/config"
	gseSync "github.com/TencentBlueKing/bk-bscp/internal/task/executor/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/hello"
	"github.com/TencentBlueKing/bk-bscp/internal/task/executor/process"
)

// RegisterExecutor register executor.
// RegisterExecutor 中可以补充参数，比如执行器依赖的配置，执行器依赖的第三方服务等
// nolint: revive
func RegisterExecutor(gseService *gse.Service, bkcmdbService bkcmdb.Service, dao dao.Set, repo repository.Provider,
	redLock *lock.RedisLock, pm pushmanager.Service) {
	// 注册 process 执行器
	processExecutor := process.NewProcessExecutor(gseService, bkcmdbService, pm, dao)
	process.RegisterExecutor(processExecutor)

	updateRegisterExecutor := process.NewUpdateRegisterExecutor(gseService, bkcmdbService, dao, redLock)
	process.RegisterUpdateRegisterExecutor(updateRegisterExecutor)

	// 注册 同步cmdb和gse 执行器
	cmdbGseExecutor := cmdbGse.NewSyncCmdbGseExecutor(gseService, bkcmdbService, dao)
	cmdbGse.RegisterExecutor(cmdbGseExecutor)

	gseSyncExecutor := gseSync.NewProcessStateSyncExecutor(bkcmdbService, gseService, dao)
	gseSync.RegisterExecutor(gseSyncExecutor)

	// 注册 配置生成执行器
	configGenerateExecutor := config.NewGenerateConfigExecutor(dao, bkcmdbService, repo, pm)
	// 设置 CMDB 服务，用于获取 CC 拓扑 XML
	configGenerateExecutor.SetCMDBService(bkcmdbService)
	config.RegisterGenerateConfigExecutor(configGenerateExecutor)

	// 注册 配置下发执行器
	configPushExecutor := config.NewPushConfigExecutor(dao, gseService, bkcmdbService, repo, pm)
	config.RegisterPushConfigExecutor(configPushExecutor)

	// 注册 配置检查执行器
	configCheckExecutor := config.NewCheckConfigExecutor(dao, gseService, bkcmdbService, pm)
	config.RegisterCheckConfigExecutor(configCheckExecutor)
}

// RegisterHello register
// nolint: revive
func RegisterHello() {
	// 注册 hello 执行器，
	e := &hello.HelloExecutor{}
	hello.Register(e)
}
