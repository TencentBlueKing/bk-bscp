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

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/TencentBlueKing/bk-bscp/cmd/data-service/service/crontab"
	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/bedis"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

func main() {
	// 初始化服务名称为 data-service
	cc.InitService(cc.DataServiceName)

	// 设置默认配置文件路径
	defaultConfigFile := "/root/config.yaml"

	// 获取配置文件路径
	var configFile string
	if len(os.Args) >= 2 {
		configFile = os.Args[1]
	} else {
		configFile = defaultConfigFile
		log.Printf("使用默认配置文件: %s", configFile)
	}

	sysOpt := &cc.SysOption{
		ConfigFiles: []string{configFile},
	}

	if err := cc.LoadSettings(sysOpt); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化日志
	logs.InitLogger(cc.DataService().Log.Logs())
	defer logs.CloseLogs()

	// 初始化 DAO set
	daoSet, err := dao.NewDaoSet(cc.DataService().Sharding, cc.DataService().Credential, cc.DataService().Gorm)
	if err != nil {
		log.Fatalf("初始化 DAO set 失败: %v", err)
	}

	logs.Infof("DAO set 初始化成功")
	logs.Infof("配置文件: %s", configFile)

	// 创建 CMDB 服务
	cmdbService, err := bkcmdb.New(&cc.CMDBConfig{
		AppCode:    cc.DataService().Esb.AppCode,
		AppSecret:  cc.DataService().Esb.AppSecret,
		Host:       cc.DataService().Esb.Endpoints[0],
		BkUserName: cc.DataService().Esb.User,
	}, nil)
	if err != nil {
		log.Fatalf("初始化 CMDB 服务失败: %v", err)
	}

	// 创建 Redis 客户端 - 从配置文件加载
	redisClient, err := bedis.NewRedisCache(cc.DataService().RedisCluster)
	if err != nil {
		log.Fatalf("初始化 Redis 客户端失败: %v", err)
	}
	logs.Infof("Redis 客户端初始化成功，连接地址: %v", cc.DataService().RedisCluster.Endpoints)

	logs.Infof("===================== 启动定时任务 =====================")

	// 启动业务主机关系同步定时任务
	logs.Infof("启动业务主机关系同步定时任务...")
	syncBizHost := crontab.NewSyncBizHost(daoSet, nil, cmdbService, redisClient, 500, 50.0)
	// 首次启动时立即执行一次全量同步
	logs.Infof("performing initial full sync on service startup")
	kt := kit.New()
	ctx, cancel := context.WithCancel(kt.Ctx)
	kt.Ctx = ctx
	syncBizHost.SyncBizHost(kt)
	cancel()
	syncBizHost.Run()

	// 启动业务主机事件监听定时任务
	logs.Infof("启动业务主机事件监听定时任务...")
	watchBizHost := crontab.NewWatchBizHost(daoSet, nil, cmdbService, redisClient, 80.0)
	watchBizHost.Run()

	// 启动业务主机关系清理定时任务
	logs.Infof("启动业务主机关系清理定时任务...")
	cleanupBizHost := crontab.NewCleanupBizHost(daoSet, nil, cmdbService, 50.0)
	cleanupBizHost.Run()

	// 等待关闭信号，让定时任务持续运行
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logs.Infof("收到关闭信号，正在停止服务...")
	logs.CloseLogs()
}
