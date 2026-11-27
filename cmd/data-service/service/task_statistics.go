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
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	itypes "github.com/Tencent/bk-bcs/bcs-common/common/task/types"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	"github.com/TencentBlueKing/bk-bscp/pkg/criteria/errf"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/i18n"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
	pbtb "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/task_batch"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
	"github.com/TencentBlueKing/bk-bscp/pkg/taskerr"
	"github.com/TencentBlueKing/bk-bscp/pkg/types"
)

const (
	// statisticsBatchChunk 单次 IN 查询携带的批次 ID 数量，避免 SQL 过长。
	// 取 250：按每批次 200~400 条任务记录估算，单批失败行数在 5 万~10 万之间，
	// 配合 statisticsFailureRowLimit 可以避免截断，同时把单批内存控制在 30~70MB。
	statisticsBatchChunk = 250
	// errorCodeTopN 错误码分布 Top N，其余归"其他"
	errorCodeTopN = 10
	// creatorTopN 人员分布 Top N，与原型一致固定为 3
	creatorTopN = 3
	// bizTopN 业务分布 Top N，与原型一致固定为 5
	bizTopN = 5
	// failedReasonMaxLen 失败原因展示长度上限，避免超长 message 撑爆页面
	failedReasonMaxLen = 500
	// defaultStatisticsRange 未传时间范围时的默认窗口
	defaultStatisticsRange = 7 * 24 * time.Hour
	// maxStatisticsRange 单次统计允许的最大时间跨度
	maxStatisticsRange = 31 * 24 * time.Hour
	// taskIndexTypeBatch 任务记录按批次索引，统计只取这类记录
	taskIndexTypeBatch = "task_batch"
	// maxStatisticsCreators 人员筛选允许的最大人数，超出直接拒绝，避免 IN 列表过长
	maxStatisticsCreators = 500
	// maxStatisticsBatches 单次统计允许的批次上限，超出时要求调用方收窄时间范围
	maxStatisticsBatches = 50000
	// statisticsFailureRowLimit 单批（statisticsBatchChunk 个批次）失败记录的最大扫描行数，
	// 超出时截断并记录日志。按 250 个批次、每批 400 条任务记录估算，单批最多 10 万行，
	// 这里留 20% 余量取 12 万，单批内存峰值约 60~85MB。
	statisticsFailureRowLimit = 120000
	// maxFailureTaskRows 失败明细单次查询允许扫描的基础行数上限。
	// 明细的扩展筛选与 filter_options 都基于基础范围（时间窗 + 创建人）的全量行计算，
	// 无法只取分页那一页，因此与统计保持同一量级；超出时要求收窄范围，
	// 不静默截断，避免总数与筛选选项不完整。
	maxFailureTaskRows = 120000
	// defaultFailureTaskLimit 失败明细默认分页条数
	defaultFailureTaskLimit = 20
	// failureNoErrorCodeOption 错误码筛选中表示「无错误码」的特殊选项
	failureNoErrorCodeOption = "--"
	// failureTimeLayout 时间筛选精确到秒时使用的格式（UTC）
	failureTimeLayout = "2006-01-02 15:04:05"
	// failureMinuteLayout 时间筛选精确到分钟时使用的格式
	failureMinuteLayout = "2006-01-02 15:04"
	// failureHourLayout 时间筛选精确到小时时使用的格式
	failureHourLayout = "2006-01-02 15"
	// failureDateLayout 时间筛选与开始/结束时间选项使用的格式（按天）
	failureDateLayout = "2006-01-02"
	// maxFailureTaskIDOptions 任务 ID 选项数量上限。
	// 时间范围最长 31 天，失败任务可达数万，逐条返回会让响应撑爆 gRPC 默认 4MB 接收上限，
	// 前端也无法渲染，因此只返回最新的 N 个任务 ID。
	maxFailureTaskIDOptions = 1
)

// nonFailedStatuses 不算失败的任务状态：成功、幂等忽略，以及尚未结束的 INIT / RUNNING / NOT_STARTED。
// 统计范围虽已限定终态批次，历史数据仍可能残留未收敛的任务，这些任务既不算成功也不算失败，
// 在概览里通过 running / not_started 单独展示，失败明细也不会出现它们。
// 其余状态（FAILURE / TIMEOUT / REVOKED）一律计入失败。
var nonFailedStatuses = []string{
	itypes.TaskStatusSuccess,
	itypes.TaskStatusIgnored,
	itypes.TaskStatusInit,
	itypes.TaskStatusRunning,
	itypes.TaskStatusNotStarted,
}

// finishedBatchStatuses 参与统计的批次状态：只统计已终态的批次，
// 避免运行中的任务被按「非成功即失败」计入，导致失败数随查询时刻跳变。
var finishedBatchStatuses = []string{
	string(table.TaskBatchStatusSucceed),
	string(table.TaskBatchStatusFailed),
	string(table.TaskBatchStatusPartlyFailed),
}

// statisticsTaskObjects 参与统计的任务对象：进程操作与配置文件下发。
var statisticsTaskObjects = []string{
	string(table.TaskObjectProcess),
	string(table.TaskObjectConfigFile),
}

var (
	// errorCodeRegexp 兼容改造前历史数据：从自由文本中提取 errorCode= 错误码。
	// 新数据优先走 [BSCP-ERR] 结构化段，见 errorCodeOf。
	errorCodeRegexp = regexp.MustCompile(`errorCode=(-?\d+)`)
)

// TaskStatistics implements pbds.DataServer.
// 概览与三个图表一次扫描完成：它们都依赖失败任务的 message 解析结果，
// 拆成多次查询会让同一批数据被重复扫描，并因两次查询之间落库的新任务导致数字对不上。
func (s *Service) TaskStatistics(ctx context.Context, req *pbds.TaskStatisticsReq) (*pbds.TaskStatisticsResp,
	error) {
	kt := kit.FromGrpcContext(ctx)

	start, end, err := buildStatisticsTimeRange(kt, req.TimeRangeStart, req.TimeRangeEnd)
	if err != nil {
		return nil, err
	}

	if len(req.Creators) > maxStatisticsCreators {
		return nil, errf.Errorf(errf.InvalidParameter, "%s",
			i18n.T(kt, "creators cannot exceed %d", maxStatisticsCreators))
	}

	batchRefs, err := s.listStatisticsBatchRefs(kt, req.Creators, start, end)
	if err != nil {
		return nil, err
	}

	indexes := batchIndexes(batchRefs)
	if len(indexes) == 0 {
		// 时间窗内没有符合条件的批次时直接返回空结果，避免退化成全表扫描
		return &pbds.TaskStatisticsResp{Overview: buildStatisticsOverview(nil)}, nil
	}

	// 状态计数与失败聚合是两次独立查询，不是同一快照：并发写入时概览失败数与图表数值
	// 可能存在少量偏差，此处按最终一致处理。
	statusCounts, err := s.countStatisticsStatuses(kt, indexes)
	if err != nil {
		logs.Errorf("count task status failed, err: %v, rid: %s", err, kt.Rid)
		return nil, errf.Errorf(errf.DBOpFailed, "%s", i18n.T(kt, "count task status failed, err: %v", err))
	}

	errorCodes, creators, businessCounts, err := s.aggregateStatisticsFailures(kt, indexes)
	if err != nil {
		logs.Errorf("list failed task for statistics failed, err: %v, rid: %s", err, kt.Rid)
		return nil, errf.Errorf(errf.DBOpFailed, "%s", i18n.T(kt, "list failed task for statistics failed, err: %v", err))
	}

	batchBizMap := batchBizIndex(batchRefs)
	businesses := make(map[uint32]uint32, len(batchBizMap))
	for taskIndex, count := range businessCounts {
		if bizID, ok := batchBizMap[taskIndex]; ok {
			businesses[bizID] += count
		}
	}

	bizNameMap := s.listBizNames(kt, businesses)

	return &pbds.TaskStatisticsResp{
		Overview:   buildStatisticsOverview(statusCounts),
		ErrorCodes: buildChartItems(errorCodes, errorCodeTopN),
		Creators:   buildChartItems(creators, creatorTopN),
		Businesses: buildChartItemsUint32(businesses, bizTopN, bizNameMap),
	}, nil
}

// TaskFailureTasks implements pbds.DataServer.
// 批次表做驱动表并 join 任务记录表：批次一条对应任务多条，先取批次 ID 再 IN 回查会
// 放大成多轮查询，交给数据库一次 join 更简单。
//
// 筛选分两层：
//  1. 基础范围（time_range_start/time_range_end + creators）在 SQL 侧收敛，同时决定 filter_options；
//  2. 扩展筛选（任务 ID、状态、业务、时间、错误码、失败信息）在内存中按行计算——
//     错误码与失败原因需要解析 message，无法下推到 SQL；且 filter_options 必须基于
//     基础范围生成（选择某个维度后其他维度不能收缩），因此只扫描一次基础行集，
//     排序沿用 SQL 的 created_at DESC、id DESC，分页在内存中完成。
func (s *Service) TaskFailureTasks(ctx context.Context, req *pbds.TaskFailureTasksReq) (*pbds.TaskFailureTasksResp, error) {
	kt := kit.FromGrpcContext(ctx)

	start, end, err := buildStatisticsTimeRange(kt, req.GetTimeRangeStart(), req.GetTimeRangeEnd())
	if err != nil {
		return nil, err
	}

	if len(req.GetCreators()) > maxStatisticsCreators {
		return nil, errf.Errorf(errf.InvalidParameter, "%s",
			i18n.T(kt, "creators cannot exceed %d", maxStatisticsCreators))
	}

	search, err := buildFailureSearch(kt, req.GetSearch(), *start, *end, req.GetCreators())
	if err != nil {
		return nil, err
	}

	limit := req.GetLimit()
	if limit == 0 {
		limit = defaultFailureTaskLimit
	}
	opt := &types.BasePage{Start: req.GetStart(), Limit: uint(limit)}
	if err = opt.Validate(types.DefaultPageOption); err != nil {
		return nil, errf.Errorf(errf.InvalidParameter, "%s", i18n.T(kt, "validate page parameters failed, err: %v", err))
	}

	// 多取一条用于判断是否超出扫描上限，避免把截断后的行集当成全量
	rows, err := s.dao.TaskRecord().ListFailureRows(kt, &dao.TaskFailureFilter{
		TenantID:        kt.TenantID,
		TaskIndexType:   taskIndexTypeBatch,
		TaskObjects:     statisticsTaskObjects,
		BatchStatuses:   finishedBatchStatuses,
		BatchStartTime:  start,
		BatchEndTime:    end,
		Creators:        req.GetCreators(),
		ExcludeStatuses: nonFailedStatuses,
	}, maxFailureTaskRows+1)
	if err != nil {
		logs.Errorf("list failed task detail failed, err: %v, rid: %s", err, kt.Rid)
		return nil, errf.Errorf(errf.DBOpFailed, "%s", i18n.T(kt, "list failed task detail failed, err: %v", err))
	}
	if len(rows) > maxFailureTaskRows {
		return nil, errf.Errorf(errf.InvalidParameter, "%s", i18n.T(kt,
			"too many failure tasks in the time range, max %d, please narrow it down", maxFailureTaskRows))
	}

	bizNameMap := s.listFailureBizNames(kt, rows)
	// filter_options 只依赖基础范围，保证选择扩展条件后其他维度选项不收缩
	filterOptions := buildFailureFilterOptions(rows, bizNameMap)

	filtered := make([]*dao.TaskFailureRow, 0, len(rows))
	for _, row := range rows {
		if matchFailureSearch(row, search, bizNameMap) {
			filtered = append(filtered, row)
		}
	}

	page := pageFailureRows(filtered, opt.Start, opt.Limit)
	list, err := s.buildFailureTaskItems(kt, page, bizNameMap)
	if err != nil {
		return nil, err
	}

	return &pbds.TaskFailureTasksResp{
		Count:         uint32(len(filtered)),
		List:          list,
		FilterOptions: filterOptions,
	}, nil
}

// failureSearch 失败明细扩展筛选条件（解析态）。
// 同一维度内为 OR 关系，不同维度之间为 AND 关系，空维度不参与过滤。
type failureSearch struct {
	taskIDs        map[string]struct{}
	creators       map[string]struct{}
	statuses       map[string]struct{}
	bizNames       map[string]struct{}
	bizIDs         map[uint32]struct{}
	startTime      *failureTimeFilter
	endTime        *failureTimeFilter
	errorCodes     map[string]struct{}
	noErrorCode    bool
	failureMessage string
}

// failureTimeFilter 单个时间边界条件，与 TimeRangeStart/TimeRangeEnd 一样是区间语义：
// start_time 是任务开始时间的下界（>=），end_time 是任务结束时间的上界（<=）。
// 入参只给到某一段（如只给日期）时，下界取该段起点、上界取该段末尾。
type failureTimeFilter struct {
	value    time.Time
	dateOnly bool
}

// buildFailureSearch 解析扩展筛选条件，入参为空时返回空条件（不过滤任何行）。
// 扩展的时间与人员必须落在基础筛选范围（time range + creators）内，允许与边界相等，
// 避免出现「扩展条件选了一条基础范围外的数据，结果却为空」的歧义。
func buildFailureSearch(kt *kit.Kit, search *pbds.TaskFailureSearch, start, end time.Time,
	creators []string) (*failureSearch, error) {
	result := &failureSearch{
		taskIDs:        stringSet(search.GetTaskIds()),
		creators:       stringSet(search.GetCreators()),
		statuses:       stringSet(search.GetStatuses()),
		bizNames:       stringSet(search.GetBizNames()),
		bizIDs:         make(map[uint32]struct{}),
		errorCodes:     make(map[string]struct{}),
		failureMessage: strings.ToLower(strings.TrimSpace(search.GetFailureMessage())),
	}

	if err := validateFailureCreators(kt, result.creators, creators); err != nil {
		return nil, err
	}

	for _, code := range search.GetErrorCodes() {
		code = strings.TrimSpace(code)
		switch code {
		case "":
			continue
		case failureNoErrorCodeOption:
			result.noErrorCode = true
		default:
			result.errorCodes[code] = struct{}{}
		}
	}

	for _, id := range search.GetBizIds() {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		parsed, err := strconv.ParseUint(id, 10, 32)
		if err != nil {
			return nil, errf.Errorf(errf.InvalidParameter, "%s", i18n.T(kt, "invalid biz_id: %s", id))
		}
		result.bizIDs[uint32(parsed)] = struct{}{}
	}

	var err error
	if search.GetStartTime() != "" {
		if result.startTime, err = parseFailureTimeFilter(kt, search.GetStartTime(), "start_time", false); err != nil {
			return nil, err
		}
		if !result.startTime.withinBaseRange(start, end) {
			return nil, errf.Errorf(errf.InvalidParameter, "%s",
				i18n.T(kt, "start_time must be within the time range, range end: %s",
					end.UTC().Format(failureTimeLayout)))
		}
	}
	if search.GetEndTime() != "" {
		if result.endTime, err = parseFailureTimeFilter(kt, search.GetEndTime(), "end_time", true); err != nil {
			return nil, err
		}
		if !result.endTime.withinBaseRange(start, end) {
			return nil, errf.Errorf(errf.InvalidParameter, "%s",
				i18n.T(kt, "end_time must be within the time range, range start: %s",
					start.UTC().Format(failureTimeLayout)))
		}
	}
	if result.startTime != nil && result.endTime != nil && result.startTime.value.After(result.endTime.value) {
		return nil, errf.Errorf(errf.InvalidParameter, "%s",
			i18n.T(kt, "end_time must not be earlier than start_time"))
	}

	return result, nil
}

// validateFailureCreators 校验扩展人员必须包含在基础筛选人员内；基础人员为空表示不限，不做校验。
// 空创建人与 "-" 视为同一取值（与明细展示口径一致）。
func validateFailureCreators(kt *kit.Kit, searchCreators map[string]struct{}, baseCreators []string) error {
	if len(searchCreators) == 0 || len(baseCreators) == 0 {
		return nil
	}

	allowed := make(map[string]struct{}, len(baseCreators))
	for _, creator := range baseCreators {
		allowed[failureCreatorOf(strings.TrimSpace(creator))] = struct{}{}
	}

	invalid := make([]string, 0)
	for creator := range searchCreators {
		if _, ok := allowed[failureCreatorOf(creator)]; !ok {
			invalid = append(invalid, creator)
		}
	}
	if len(invalid) == 0 {
		return nil
	}
	sort.Strings(invalid)
	return errf.Errorf(errf.InvalidParameter, "%s",
		i18n.T(kt, "creators must be within the base filter range: %s", strings.Join(invalid, ",")))
}

// parseFailureTimeFilter 解析单个时间边界条件：
// 支持 yyyy-MM-dd、yyyy-MM-dd HH、yyyy-MM-dd HH:mm、yyyy-MM-dd HH:mm:ss 与 RFC3339。
// upper 为 true（end_time）时取入参精度的末尾，例如只给日期取当天 23:59:59，
// 避免「结束时间 = 2026-11-20」把当天数据全部排除。
func parseFailureTimeFilter(kt *kit.Kit, value, fieldName string, upper bool) (*failureTimeFilter, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{
		failureDateLayout, failureHourLayout, failureMinuteLayout, failureTimeLayout, time.RFC3339,
	} {
		parsed, err := time.Parse(layout, value)
		if err != nil {
			continue
		}
		utc := parsed.UTC()
		if upper {
			utc = endOfTimePeriod(utc, layout)
		}
		return &failureTimeFilter{value: utc, dateOnly: layout == failureDateLayout}, nil
	}
	return nil, errf.Errorf(errf.InvalidParameter, "%s",
		i18n.T(kt, "invalid %s format: %s", fieldName, value))
}

// endOfTimePeriod 返回入参精度对应区间的最后一秒。
func endOfTimePeriod(utc time.Time, layout string) time.Time {
	switch layout {
	case failureDateLayout:
		return utc.AddDate(0, 0, 1).Add(-time.Second)
	case failureHourLayout:
		return utc.Add(time.Hour - time.Second)
	case failureMinuteLayout:
		return utc.Add(time.Minute - time.Second)
	default:
		return utc
	}
}

// matchLower 下界匹配：值必须不早于下界；从未开始的零值时间自然不命中。
func (f *failureTimeFilter) matchLower(value time.Time) bool {
	if f == nil {
		return true
	}
	return !value.Before(f.value)
}

// matchUpper 上界匹配：值必须不晚于上界；结束时间为空的行不命中。
func (f *failureTimeFilter) matchUpper(value time.Time) bool {
	if f == nil {
		return true
	}
	if value.IsZero() {
		return false
	}
	return !value.After(f.value)
}

// withinBaseRange 判断时间条件是否落在基础时间范围内，允许与边界相等。
// 只给日期时与基础范围按日期比较，避免「选的当天有数据、但当天 00:00 早于范围内起点」被误拒。
func (f *failureTimeFilter) withinBaseRange(start, end time.Time) bool {
	if f.dateOnly {
		day := f.value.UTC().Format(failureDateLayout)
		return day >= start.UTC().Format(failureDateLayout) && day <= end.UTC().Format(failureDateLayout)
	}
	return !f.value.Before(start) && !f.value.After(end)
}

// matchFailureSearch 判断单行是否命中扩展筛选条件。
func matchFailureSearch(row *dao.TaskFailureRow, search *failureSearch, bizNames map[uint32]string) bool {
	if len(search.taskIDs) > 0 {
		if _, ok := search.taskIDs[row.TaskID]; !ok {
			return false
		}
	}
	if len(search.creators) > 0 {
		if _, ok := search.creators[failureCreatorOf(row.Creator)]; !ok {
			return false
		}
	}
	if len(search.statuses) > 0 {
		if _, ok := search.statuses[row.Status]; !ok {
			return false
		}
	}
	if len(search.bizIDs) > 0 {
		if _, ok := search.bizIDs[row.BizID]; !ok {
			return false
		}
	}
	if len(search.bizNames) > 0 {
		matched := false
		for _, name := range failureBizNameCandidates(row.BizID, bizNames) {
			if _, ok := search.bizNames[name]; ok {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if !search.startTime.matchLower(row.Start) {
		return false
	}
	if !search.endTime.matchUpper(row.End) {
		return false
	}
	if len(search.errorCodes) > 0 || search.noErrorCode {
		code := errorCodeKeyOf(row.Message)
		if code == "" {
			if !search.noErrorCode {
				return false
			}
		} else if _, ok := search.errorCodes[code]; !ok {
			return false
		}
	}
	if search.failureMessage != "" {
		if !strings.Contains(strings.ToLower(failedReasonOf(row.Message)), search.failureMessage) {
			return false
		}
	}
	return true
}

// pageFailureRows 在内存中对已排序的失败行分页，越界时返回空页。
func pageFailureRows(rows []*dao.TaskFailureRow, start uint32, limit uint) []*dao.TaskFailureRow {
	if int(start) >= len(rows) {
		return nil
	}
	end := int(start) + int(limit)
	if end > len(rows) || end < 0 {
		end = len(rows)
	}
	return rows[start:end]
}

// listFailureBizNames 查询基础结果集涉及的业务名称映射，映射失败时降级为空。
func (s *Service) listFailureBizNames(kt *kit.Kit, rows []*dao.TaskFailureRow) map[uint32]string {
	bizIDs := make(map[uint32]uint32, len(rows))
	for _, row := range rows {
		if row.BizID > 0 {
			bizIDs[row.BizID] = row.BizID
		}
	}
	return s.listBizNames(kt, bizIDs)
}

// buildFailureTaskItems 组装当前页明细。common_payload 只对本页任务回填，避免全量扫描大字段。
func (s *Service) buildFailureTaskItems(kt *kit.Kit, rows []*dao.TaskFailureRow,
	bizNames map[uint32]string) ([]*pbtb.FailureTaskItem, error) {
	taskIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		taskIDs = append(taskIDs, row.TaskID)
	}

	payloads, err := s.dao.TaskRecord().ListFailurePayloads(kt, taskIDs)
	if err != nil {
		logs.Errorf("list failed task payload failed, err: %v, rid: %s", err, kt.Rid)
		return nil, errf.Errorf(errf.DBOpFailed, "%s", i18n.T(kt, "list failed task payload failed, err: %v", err))
	}

	list := make([]*pbtb.FailureTaskItem, 0, len(rows))
	for _, row := range rows {
		item := &pbtb.FailureTaskItem{
			TaskId:       row.TaskID,
			Creator:      row.Creator,
			CcProcessId:  ccProcessIDOf(payloads[row.TaskID]),
			Status:       row.Status,
			ErrorCode:    errorCodeOf(row.Message),
			FailedReason: failedReasonOf(row.Message),
			BizId:        row.BizID,
			BizName:      bizNames[row.BizID],
			BatchId:      batchIDOf(row.TaskIndex),
		}
		if !row.Start.IsZero() {
			item.Start = timestamppb.New(row.Start.UTC())
		}
		if !row.End.IsZero() {
			item.End = timestamppb.New(row.End.UTC())
		}
		list = append(list, item)
	}
	return list, nil
}

// buildFailureFilterOptions 基于基础范围（未应用扩展筛选）的行生成各维度可选项。
// 开始/结束时间按天生成（时间范围最长 31 天，选项最多 31 个）；任务 ID 只取最新的
// maxFailureTaskIDOptions 个（行已按 created_at 倒序），避免响应超过 gRPC 接收上限。
func buildFailureFilterOptions(rows []*dao.TaskFailureRow,
	bizNames map[uint32]string) *pbtb.TaskFailureFilterOptions {
	taskIDs := make([]string, 0, maxFailureTaskIDOptions)
	taskIDSeen := make(map[string]struct{}, maxFailureTaskIDOptions)
	creators := make(map[string]struct{}, len(rows))
	statuses := make(map[string]struct{}, len(rows))
	bizNameOptions := make(map[string]struct{}, len(rows))
	bizIDOptions := make(map[uint32]struct{}, len(rows))
	startTimes := make(map[string]struct{}, len(rows))
	endTimes := make(map[string]struct{}, len(rows))
	errorCodes := make(map[string]struct{}, len(rows))

	for _, row := range rows {
		if row.TaskID != "" {
			if _, ok := taskIDSeen[row.TaskID]; !ok && len(taskIDs) < maxFailureTaskIDOptions {
				taskIDSeen[row.TaskID] = struct{}{}
				taskIDs = append(taskIDs, row.TaskID)
			}
		}
		creators[failureCreatorOf(row.Creator)] = struct{}{}
		if row.Status != "" {
			statuses[row.Status] = struct{}{}
		}
		if row.BizID > 0 {
			bizIDOptions[row.BizID] = struct{}{}
		}
		// 业务名称缺失时回落业务 ID，保证 CMDB 映射失败也能产出可选项
		if name := failureBizName(row.BizID, bizNames); name != "" {
			bizNameOptions[name] = struct{}{}
		}
		if !row.Start.IsZero() {
			startTimes[row.Start.UTC().Format(failureDateLayout)] = struct{}{}
		}
		if !row.End.IsZero() {
			endTimes[row.End.UTC().Format(failureDateLayout)] = struct{}{}
		}
		errorCodes[errorCodeKeyOf(row.Message)] = struct{}{}
	}
	sort.Strings(taskIDs)

	return &pbtb.TaskFailureFilterOptions{
		TaskIds:    choicesOf(taskIDs),
		Creators:   sortedChoices(creators),
		Statuses:   sortedChoices(statuses),
		BizNames:   sortedChoices(bizNameOptions),
		BizIds:     sortedBizIDChoices(bizIDOptions, bizNames),
		StartTimes: sortedChoices(startTimes),
		EndTimes:   sortedChoices(endTimes),
		ErrorCodes: errorCodeChoices(errorCodes),
	}
}

// failureCreatorOf 创建人为空时统一归 "-"，与人员分布口径一致。
func failureCreatorOf(creator string) string {
	if creator == "" {
		return "-"
	}
	return creator
}

// failureBizName 返回一行业务的展示名：业务名缺失时回落业务 ID 字符串，
// 保证 CMDB 映射失败时业务名称选项仍可用。
func failureBizName(bizID uint32, bizNames map[uint32]string) string {
	if bizID == 0 {
		return ""
	}
	if name := bizNames[bizID]; name != "" {
		return name
	}
	return strconv.FormatUint(uint64(bizID), 10)
}

// failureBizNameCandidates 返回一行业务在「业务名称」维度下可被选中的值：
// 业务名与业务 ID 字符串都接受，避免前端传 ID 或名称时匹配不到。
func failureBizNameCandidates(bizID uint32, bizNames map[uint32]string) []string {
	name := failureBizName(bizID, bizNames)
	if name == "" {
		return nil
	}
	id := strconv.FormatUint(uint64(bizID), 10)
	if name == id {
		return []string{id}
	}
	return []string{name, id}
}

// sortedChoices 将字符串集合转为按字典序升序的选项列表，id 与 name 相同。
func sortedChoices(values map[string]struct{}) []*pbtb.Choice {
	keys := make([]string, 0, len(values))
	for value := range values {
		if value == "" {
			continue
		}
		keys = append(keys, value)
	}
	sort.Strings(keys)
	return choicesOf(keys)
}

// choicesOf 将已排序的字符串列表转为选项列表，id 与 name 相同。
func choicesOf(values []string) []*pbtb.Choice {
	choices := make([]*pbtb.Choice, 0, len(values))
	for _, value := range values {
		choices = append(choices, &pbtb.Choice{Id: value, Name: value})
	}
	return choices
}

// sortedBizIDChoices 生成业务 ID 选项：id 为业务 ID，name 为业务名称（缺失时回落业务 ID）。
func sortedBizIDChoices(values map[uint32]struct{}, bizNames map[uint32]string) []*pbtb.Choice {
	ids := make([]uint32, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	choices := make([]*pbtb.Choice, 0, len(ids))
	for _, id := range ids {
		key := strconv.FormatUint(uint64(id), 10)
		name := bizNames[id]
		if name == "" {
			name = key
		}
		choices = append(choices, &pbtb.Choice{Id: key, Name: name})
	}
	return choices
}

// errorCodeChoices 生成错误码选项：无错误码固定为第一个选项（id 为 "--"），其余按数值升序。
func errorCodeChoices(values map[string]struct{}) []*pbtb.Choice {
	codes := make([]int64, 0, len(values))
	for value := range values {
		if value == "" {
			continue
		}
		code, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			continue
		}
		codes = append(codes, code)
	}
	sort.Slice(codes, func(i, j int) bool { return codes[i] < codes[j] })

	choices := make([]*pbtb.Choice, 0, len(codes)+1)
	choices = append(choices, &pbtb.Choice{Id: failureNoErrorCodeOption, Name: failureNoErrorCodeOption})
	for _, code := range codes {
		key := strconv.FormatInt(code, 10)
		choices = append(choices, &pbtb.Choice{Id: key, Name: key})
	}
	return choices
}

// stringSet 将字符串切片转为集合，忽略空值。
func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result[value] = struct{}{}
		}
	}
	return result
}

// listStatisticsBatchRefs 查询参与统计的批次引用。
// 统计范围是当前租户的全部业务：只按时间、人员、任务对象与终态过滤，
// 不按业务维度收窄，也因此无法复用需要 biz_id 的 TaskBatch().List 或 GetByID。
// 这里不设批次上限，后续按 statisticsBatchChunk 分批查询任务记录并在内存聚合。
func (s *Service) listStatisticsBatchRefs(kt *kit.Kit, creators []string, start, end *time.Time) (
	[]*table.TaskBatch, error) {
	filter := &dao.TaskBatchListFilter{
		TaskObjects:    statisticsTaskObjects,
		Executors:      creators,
		TimeRangeStart: start,
		TimeRangeEnd:   end,
		Statuses:       finishedBatchStatuses,
	}

	// 多取一条用于判断是否超限，避免把截断后的批次当成全量
	refs, err := s.dao.TaskBatch().ListRefs(kt, filter, nil, maxStatisticsBatches+1)
	if err != nil {
		logs.Errorf("list task batch refs failed, err: %v, rid: %s", err, kt.Rid)
		return nil, errf.Errorf(errf.DBOpFailed, "%s", i18n.T(kt, "list task batch refs failed, err: %v", err))
	}
	if len(refs) > maxStatisticsBatches {
		return nil, errf.Errorf(errf.InvalidParameter, "%s",
			i18n.T(kt, "too many task batches in the time range, max %d, please narrow it down", maxStatisticsBatches))
	}

	return refs, nil
}

// countStatisticsStatuses 分片统计任务状态计数，避免单条 SQL 的 IN 列表过长。
func (s *Service) countStatisticsStatuses(kt *kit.Kit, indexes []string) (map[string]int64, error) {
	result := make(map[string]int64)
	for _, chunk := range chunkStrings(indexes, statisticsBatchChunk) {
		counts, err := s.dao.TaskRecord().StatusCount(kt, &dao.TaskRecordFilter{
			TaskIndexes:   chunk,
			TaskIndexType: taskIndexTypeBatch,
		})
		if err != nil {
			return nil, err
		}
		for status, count := range counts {
			result[status] += count
		}
	}
	return result, nil
}

// aggregateStatisticsFailures 分片扫描失败任务并聚合图表数据。
// 返回错误码分布、人员分布与按批次维度累加的失败数；每批记录聚合后即丢弃，只保留计数。
func (s *Service) aggregateStatisticsFailures(kt *kit.Kit, indexes []string) (
	map[string]uint32, map[string]uint32, map[string]uint32, error) {
	errorCodes := make(map[string]uint32)
	creators := make(map[string]uint32)
	businessCounts := make(map[string]uint32)

	for _, chunk := range chunkStrings(indexes, statisticsBatchChunk) {
		records, err := s.dao.TaskRecord().ListForStatistics(kt, &dao.TaskRecordFilter{
			TaskIndexes:     chunk,
			TaskIndexType:   taskIndexTypeBatch,
			ExcludeStatuses: nonFailedStatuses,
		}, statisticsFailureRowLimit)
		if err != nil {
			return nil, nil, nil, err
		}
		if len(records) >= statisticsFailureRowLimit {
			// 单批失败记录已到上限，后续批次仍会继续统计，但该批的错误码/人员分布可能偏少
			logs.Errorf("failure aggregation truncated at %d rows for one batch chunk, rid: %s",
				statisticsFailureRowLimit, kt.Rid)
		}

		for _, record := range records {
			// 没有错误码（含错误码为 0）的失败不进入错误码分布
			if key := errorCodeKeyOf(record.Message); key != "" {
				errorCodes[key]++
			}

			creator := record.Creator
			if creator == "" {
				creator = "-"
			}
			creators[creator]++
			businessCounts[record.TaskIndex]++
		}
	}
	return errorCodes, creators, businessCounts, nil
}

// chunkStrings 按固定大小切分切片，避免单条 SQL 的 IN 列表过长。
// 空输入返回 nil，避免调用方拼出空的 IN () 条件。
func chunkStrings(values []string, size int) [][]string {
	if len(values) == 0 {
		return nil
	}
	if size <= 0 || len(values) <= size {
		return [][]string{values}
	}
	chunks := make([][]string, 0, (len(values)+size-1)/size)
	for start := 0; start < len(values); start += size {
		end := start + size
		if end > len(values) {
			end = len(values)
		}
		chunks = append(chunks, values[start:end])
	}
	return chunks
}

// batchIndexes 提取批次 ID 字符串列表，用于任务记录过滤。
func batchIndexes(refs []*table.TaskBatch) []string {
	indexes := make([]string, 0, len(refs))
	for _, ref := range refs {
		indexes = append(indexes, strconv.FormatUint(uint64(ref.ID), 10))
	}
	return indexes
}

// batchBizIndex 构造批次 ID → 业务 ID 的映射，用于业务维度聚合。
func batchBizIndex(refs []*table.TaskBatch) map[string]uint32 {
	result := make(map[string]uint32, len(refs))
	for _, ref := range refs {
		if ref.Attachment == nil {
			continue
		}
		result[strconv.FormatUint(uint64(ref.ID), 10)] = ref.Attachment.BizID
	}
	return result
}

// listBizNames 查询业务名称映射。映射失败时降级为 biz_id，不影响主体数据。
func (s *Service) listBizNames(kt *kit.Kit, bizIDs map[uint32]uint32) map[uint32]string {
	result := make(map[uint32]string, len(bizIDs))
	if len(bizIDs) == 0 || s.cmdb == nil {
		return result
	}

	bizList, err := s.cmdb.ListAllBusiness(kt.Ctx)
	if err != nil {
		logs.Errorf("list all business for statistics failed, err: %v, rid: %s", err, kt.Rid)
		return result
	}

	for _, biz := range bizList.Info {
		if _, ok := bizIDs[uint32(biz.BizID)]; ok {
			result[uint32(biz.BizID)] = biz.BizName
		}
	}
	return result
}

// buildStatisticsTimeRange 构造时间范围，未传时默认最近 7 天。
func buildStatisticsTimeRange(kt *kit.Kit, startStr, endStr string) (*time.Time, *time.Time, error) {
	start, err := parseTimeIfNotEmpty(kt, startStr, "time_range_start")
	if err != nil {
		return nil, nil, err
	}
	end, err := parseTimeIfNotEmpty(kt, endStr, "time_range_end")
	if err != nil {
		return nil, nil, err
	}

	now := time.Now()
	if start == nil && end == nil {
		defaultStart := now.Add(-defaultStatisticsRange)
		return &defaultStart, &now, nil
	}

	// 只传一端时补齐另一端，既保证查询生效，也把跨度限制在一个月内
	if end == nil {
		filledEnd := start.Add(maxStatisticsRange)
		if filledEnd.After(now) {
			filledEnd = now
		}
		end = &filledEnd
	}
	if start == nil {
		filledStart := end.Add(-maxStatisticsRange)
		start = &filledStart
	}

	if end.Before(*start) {
		return nil, nil, errf.Errorf(errf.InvalidParameter, "%s",
			i18n.T(kt, "time_range_end must not be earlier than time_range_start"))
	}
	if end.Sub(*start) > maxStatisticsRange {
		return nil, nil, errf.Errorf(errf.InvalidParameter, "%s",
			i18n.T(kt, "time range cannot exceed 31 days"))
	}
	return start, end, nil
}

// buildStatisticsOverview 由状态计数构造概览。
// 平台问题导致失败依赖结构化错误码（[BSCP-ERR]），写入侧改造完成前固定为 0。
func buildStatisticsOverview(counts map[string]int64) *pbtb.TaskStatisticsOverview {
	total := int64(0)
	for _, count := range counts {
		total += count
	}

	success := counts[itypes.TaskStatusSuccess]
	ignored := counts[itypes.TaskStatusIgnored]
	notStarted := counts[itypes.TaskStatusNotStarted]
	running := counts[itypes.TaskStatusRunning] + counts[itypes.TaskStatusInit]
	// 失败口径：除成功、幂等忽略与尚未结束（运行中 / 未开始）之外的状态都算失败（含超时、撤销）。
	// 运行中与未开始不算失败，但仍在 total 中，保证
	// total = success + ignored + failed + running + not_started。
	failed := total - success - ignored - running - notStarted

	failureRate := float32(0)
	if total > 0 {
		failureRate = float32(failed) / float32(total)
	}

	return &pbtb.TaskStatisticsOverview{
		PlatformBugFailed: 0,
		Failed:            uint32(failed),
		Total:             uint32(total),
		Success:           uint32(success),
		Ignored:           uint32(ignored),
		NotStarted:        uint32(notStarted),
		Running:           uint32(running),
		FailureRate:       failureRate,
	}
}

// buildChartItems 将 "名称→数量" 映射按数量倒序转为图表项，超出 TopN 归入"其他"。
func buildChartItems(data map[string]uint32, topN int) []*pbtb.ChartItem {
	type entry struct {
		key   string
		value uint32
	}
	entries := make([]entry, 0, len(data))
	for key, value := range data {
		entries = append(entries, entry{key: key, value: value})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].value == entries[j].value {
			return entries[i].key < entries[j].key
		}
		return entries[i].value > entries[j].value
	})

	items := make([]*pbtb.ChartItem, 0, topN+1)
	others := uint32(0)
	for i, item := range entries {
		if i >= topN {
			others += item.value
			continue
		}
		items = append(items, &pbtb.ChartItem{Key: item.key, Name: item.key, Value: item.value})
	}
	if others > 0 {
		items = append(items, &pbtb.ChartItem{Key: "other", Name: "其他", Value: others})
	}
	return items
}

// buildChartItemsUint32 业务维度图表，key 为业务 ID，展示名为业务名称。
func buildChartItemsUint32(data map[uint32]uint32, topN int, bizNames map[uint32]string) []*pbtb.ChartItem {
	type entry struct {
		key   uint32
		value uint32
	}
	entries := make([]entry, 0, len(data))
	for key, value := range data {
		entries = append(entries, entry{key: key, value: value})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].value == entries[j].value {
			return entries[i].key < entries[j].key
		}
		return entries[i].value > entries[j].value
	})

	items := make([]*pbtb.ChartItem, 0, topN+1)
	others := uint32(0)
	for i, item := range entries {
		if i >= topN {
			others += item.value
			continue
		}
		key := strconv.FormatUint(uint64(item.key), 10)
		name := bizNames[item.key]
		if name == "" {
			name = key
		}
		items = append(items, &pbtb.ChartItem{Key: key, Name: name, Value: item.value})
	}
	if others > 0 {
		items = append(items, &pbtb.ChartItem{Key: "other", Name: "其他", Value: others})
	}
	return items
}

// errorCodeOf 从 message 中提取错误码，解析不到返回 0（0 表示没有错误码，不是真实错误码）。
// 新数据优先取 [BSCP-ERR] 结构化段，历史数据回退 errorCode= 文本。
func errorCodeOf(message string) int32 {
	if info, ok := taskerr.Decode(message); ok {
		return info.Code
	}
	matches := errorCodeRegexp.FindStringSubmatch(message)
	if len(matches) < 2 {
		return 0
	}
	code, err := strconv.ParseInt(matches[1], 10, 32)
	if err != nil {
		return 0
	}
	return int32(code)
}

// errorCodeKeyOf 构造错误码分布的分组 key：直接使用错误码数值。
// 返回空串表示该失败没有错误码（含错误码为 0），不参与错误码分布。
func errorCodeKeyOf(message string) string {
	code := errorCodeOf(message)
	if code == 0 {
		return ""
	}
	return strconv.FormatInt(int64(code), 10)
}

// failedReasonOf 提取失败原因文本：剥离结构化错误段与框架模板，截断超长内容。
// 若剥离后为空（步骤返回的 error 只有结构段），回退到结构段的 msg，
// 避免页面展示无意义的 "step xxx running failed, err="。
func failedReasonOf(message string) string {
	reason := taskerr.Readable(message)
	// 按字符而非字节截断，避免把中文等多字节字符切断成非法 UTF-8
	if runes := []rune(reason); len(runes) > failedReasonMaxLen {
		return string(runes[:failedReasonMaxLen])
	}
	return reason
}

// taskRecordPayload 只声明统计需要的 common_payload 字段。
// 结构与 executor/common.TaskPayload 对应（该结构体无 json tag，序列化后 key 为 PascalCase）；
// 这里不声明 ConfigPayload 等大字段，避免把配置快照解码进内存。
type taskRecordPayload struct {
	ProcessPayload *taskRecordProcessPayload `json:"ProcessPayload"`
}

// taskRecordProcessPayload common_payload 中进程相关的字段
type taskRecordProcessPayload struct {
	CcProcessID uint32 `json:"CcProcessID"`
}

// ccProcessIDOf 从任务记录的 common_payload 解析 CC 进程 ID，解析不到返回 0。
// 历史任务可能没有该字段，统一按 0 返回。
func ccProcessIDOf(commonPayload string) uint32 {
	if commonPayload == "" {
		return 0
	}
	var payload taskRecordPayload
	if err := json.Unmarshal([]byte(commonPayload), &payload); err != nil {
		return 0
	}
	if payload.ProcessPayload == nil {
		return 0
	}
	return payload.ProcessPayload.CcProcessID
}

// batchIDOf 将任务记录的任务索引转为批次 ID。
// batchIDOf 从 task_index 还原批次 ID。批次 ID 为自增 uint32，解析失败或越界时返回 0。
func batchIDOf(taskIndex string) uint32 {
	id, err := strconv.ParseUint(taskIndex, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(id)
}
