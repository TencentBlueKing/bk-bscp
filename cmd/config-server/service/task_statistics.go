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

package service

import (
	"context"

	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	pbcs "github.com/TencentBlueKing/bk-bscp/pkg/protocol/config-server"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

// defaultFailureTasksLimit 失败明细分页默认条数
const defaultFailureTasksLimit = 20

// TaskStatistics implements pbcs.ConfigServer.
// 跨业务统计：范围为当前租户全部业务，仅按时间范围与人员过滤。
// 按需求不判断用户业务权限，因此不调用 authorizer 的业务资源校验；
// 租户隔离由 grpcKit 携带并在 data-service 侧生效。
func (s *Service) TaskStatistics(ctx context.Context, req *pbcs.TaskStatisticsReq) (*pbcs.TaskStatisticsResp,
	error) {
	grpcKit := kit.FromGrpcContext(ctx)

	resp, err := s.client.DS.TaskStatistics(grpcKit.RpcCtx(), &pbds.TaskStatisticsReq{
		TimeRangeStart: req.GetTimeRangeStart(),
		TimeRangeEnd:   req.GetTimeRangeEnd(),
		Creators:       req.GetCreators(),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.TaskStatisticsResp{
		Overview:   resp.GetOverview(),
		ErrorCodes: resp.GetErrorCodes(),
		Creators:   resp.GetCreators(),
		Businesses: resp.GetBusinesses(),
	}, nil
}

// TaskFailureTasks implements pbcs.ConfigServer.
// 跨业务失败明细：基础范围沿用统计接口的时间范围与人员，另支持扩展筛选与动态筛选选项。
func (s *Service) TaskFailureTasks(ctx context.Context, req *pbcs.TaskFailureTasksReq) (*pbcs.TaskFailureTasksResp,
	error) {
	grpcKit := kit.FromGrpcContext(ctx)

	limit := req.GetLimit()
	if limit == 0 {
		limit = defaultFailureTasksLimit
	}

	resp, err := s.client.DS.TaskFailureTasks(grpcKit.RpcCtx(), &pbds.TaskFailureTasksReq{
		TimeRangeStart: req.GetTimeRangeStart(),
		TimeRangeEnd:   req.GetTimeRangeEnd(),
		Creators:       req.GetCreators(),
		Start:          req.GetStart(),
		Limit:          limit,
		Search:         convertFailureSearch(req.GetSearch()),
	})
	if err != nil {
		return nil, err
	}

	return &pbcs.TaskFailureTasksResp{
		Count:         resp.GetCount(),
		List:          resp.GetList(),
		FilterOptions: resp.GetFilterOptions(),
	}, nil
}

// convertFailureSearch 转换扩展筛选条件，未传时返回 nil 表示不过滤。
func convertFailureSearch(search *pbcs.TaskFailureSearch) *pbds.TaskFailureSearch {
	if search == nil {
		return nil
	}
	return &pbds.TaskFailureSearch{
		TaskIds:        search.GetTaskIds(),
		Creators:       search.GetCreators(),
		Statuses:       search.GetStatuses(),
		BizNames:       search.GetBizNames(),
		BizIds:         search.GetBizIds(),
		StartTime:      search.GetStartTime(),
		EndTime:        search.GetEndTime(),
		ErrorCodes:     search.GetErrorCodes(),
		FailureMessage: search.GetFailureMessage(),
	}
}
