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

package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/bk-bcs/bcs-common/common/task"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	pushmanager "github.com/TencentBlueKing/bk-bscp/internal/components/push_manager"
	"github.com/TencentBlueKing/bk-bscp/internal/dal/dao"
	gesprocessor "github.com/TencentBlueKing/bk-bscp/internal/processor/gse"
	"github.com/TencentBlueKing/bk-bscp/internal/runtime/lock"
	"github.com/TencentBlueKing/bk-bscp/internal/thirdparty/esb/cmdb"
	"github.com/TencentBlueKing/bk-bscp/pkg/cc"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	"github.com/TencentBlueKing/bk-bscp/pkg/kit"
	"github.com/TencentBlueKing/bk-bscp/pkg/logs"
)

const (
	maxWait  = 10 * time.Second
	interval = 2 * time.Second

	dimBizName  = "biz_name"
	dimTaskID   = "task_id"
	dimOperator = "operator"
	dimTaskType = "task_type"
	dimEnv      = "env"
	dimScope    = "scope"
	dimStart    = "start_time"
	dimEnd      = "end_time"
	dimResult   = "execute_result"
)

const taskNotifyDetailTpl = `
<div style="font-size:14px; line-height:22px; color:#333; font-weight:600;">

  <div style="margin-bottom:8px; font-size:16px;">
    <span>业务：</span>
    <span>%s</span>
  </div>

  <div style="margin-bottom:6px;">
    <span>任务ID：</span>
    <span>#%s</span>
  </div>

  <div style="margin-bottom:6px;">
    <span>执行账户：</span>
    <span>%s</span>
  </div>

  <div style="margin-bottom:6px;">
    <span>任务类型：</span>
    <span>%s</span>
  </div>

  <div style="margin-bottom:6px;">
    <span>环境类型：</span>
    <span>%s</span>
  </div>

  <div style="margin-bottom:6px;">
    <span>操作范围：</span>
    <span>%s</span>
  </div>

  <div style="margin-bottom:6px;">
    <span>开始时间：</span>
    <span>%s</span>
  </div>

  <div style="margin-bottom:6px;">
    <span>结束时间：</span>
    <span>%s</span>
  </div>

  <div style="margin-top:10px;">
    <span>执行结果：</span>
    <span>%s</span>
  </div>

</div>
`

const taskNotifyFooterTpl = `<br/><a href="%s">点击查看任务详情</a>`

const executeResultTpl = `
<span><span style="color:#2ecc71; font-weight:600;">%d</span> 个成功</span>，
<span><span style="color:#e74c3c; font-weight:600;">%d</span> 个失败</span>，
<span><span style="color:#f39c12; font-weight:600;">%d</span> 个已完成</span>
`

// Executor common executor
type Executor struct {
	// GseService GSE 服务客户端
	// 用于进程启停、状态查询等运行态操作
	GseService *gse.Service
	// CMDBService CMDB 服务接口
	// 用于获取业务、主机、模块、进程等配置元数据
	CMDBService bkcmdb.Service
	// pm Push Manager 服务
	// 负责消息推送（如 rtx / mail / msg）
	PM pushmanager.Service
	// Dao 数据访问集合
	// 封装数据库相关的读写操作
	Dao dao.Set
	// GseConf GSE 运行时配置
	// 包含 GSE 服务地址、鉴权信息等静态配置
	GseConf cc.GSE
	RedLock *lock.RedisLock
}

// TaskPayload 公用的配置，作为任务快照，方便进行获取以及对比
type TaskPayload struct {
	// 进程相关
	ProcessPayload *ProcessPayload
	// 配置相关
	ConfigPayload *ConfigPayload
}

// ProcessPayload 进程相关
type ProcessPayload struct {
	SetName       string // 集群名
	ModuleName    string // 模块名
	ServiceName   string // 服务实例
	Environment   string // 环境
	Alias         string // 进程别名
	FuncName      string // 进程二进制文件名
	InnerIP       string // IP
	AgentID       string // agnet ID
	CloudID       int    // cloud ID
	CcProcessID   uint32 // CC 进程ID
	HostInstSeq   uint32 // HostInstSeq：主机级别的自增ID
	ModuleInstSeq uint32 // ModuleInstSeq：模块级别的自增ID
	ConfigData    string // 进程启动相关配置，比如启动脚本，优先级等
}

// ConfigPayload 配置相关
type ConfigPayload struct {
	ConfigTemplateID        uint32
	ConfigTemplateVersionID uint32
	ConfigTemplateName      string
	ConfigFileName          string
	ConfigFilePath          string
	ConfigFileOwner         string
	ConfigFileGroup         string
	ConfigFilePermission    string
	ConfigInstanceKey       string // 配置实例标识: {configTemplateID}-{ccProcessID}-{moduleInstSeq}
	ConfigContent           string
	ConfigContentSignature  string        // 配置内容的签名(sha256)
	CompareStatus           CompareStatus // 对比状态
}

// CompareStatus 对比状态
type CompareStatus string

const (
	// CompareResultSame 一致
	CompareResultSame CompareStatus = "SAME" // 一致
	// CompareResultDifferent 不一致
	CompareResultDifferent CompareStatus = "DIFFERENT" // 不一致
	// CompareResultNeverPublished 从未下发
	CompareResultNeverPublished CompareStatus = "NEVER_PUBLISHED" // 从未下发
	// CompareResultUnknown 未知
	CompareResultUnknown CompareStatus = "UNKNOWN" // 未知
)

// NewExecutor new executor
func NewExecutor(gseService *gse.Service, cmdbService bkcmdb.Service, dao dao.Set,
	redLock *lock.RedisLock, pm pushmanager.Service) *Executor {
	return &Executor{
		GseService:  gseService,
		Dao:         dao,
		GseConf:     cc.G().GSE,
		RedLock:     redLock,
		PM:          pm,
		CMDBService: cmdbService,
	}
}

// WaitTaskFinish 等待进程操作任务执行结束
func (e *Executor) WaitProcOperateTaskFinish(
	ctx context.Context,
	gseTaskID string,
	bizID, hostInstSeq uint32,
	alias string,
	agentID string,
) (map[string]gse.ProcResult, error) {
	var (
		result          map[string]gse.ProcResult
		err             error
		gseResp         *gse.GESResponse
		inProgressCount int
	)

	err = task.LoopDoFunc(ctx, func() error {
		// 获取gse侧进程操作结果
		gseResp, err = e.GseService.GetProcOperateResultV2(ctx, &gse.QueryProcResultReq{
			TaskID: gseTaskID,
		})
		if err != nil {
			logs.Warnf("WaitTaskFinish get gse task state error, gseTaskID %s, err=%+v", gseTaskID, err)
			return nil
		}
		if gseResp.Code != 0 {
			logs.Errorf("WaitTaskFinish get gse task result failed, gseTaskID %s, code=%d, message=%s",
				gseTaskID, gseResp.Code, gseResp.Message)
			return fmt.Errorf("get gse task result failed, code=%d, message=%s", gseResp.Code, gseResp.Message)
		}

		err = gseResp.Decode(&result)
		if err != nil {
			return err
		}

		key := gse.BuildResultKey(agentID, bizID, alias, hostInstSeq)
		procResult, ok := result[key]
		if !ok {
			return fmt.Errorf("gse result missing key=%s, taskID=%s", key, gseTaskID)
		}

		// 115：仍在执行中
		if gse.IsInProgress(procResult.ErrorCode) {
			inProgressCount++
			logs.Infof(
				"WaitTaskFinish task %s still in progress (errorCode=115), retry=%d/%d",
				gseTaskID, inProgressCount, gesprocessor.MaxInProgressRetries,
			)

			if inProgressCount >= gesprocessor.MaxInProgressRetries {
				logs.Warnf(
					"WaitTaskFinish task %s exceeded max in-progress retries, treat as finished",
					gseTaskID,
				)
				return task.ErrEndLoop
			}

			return nil // 继续轮询
		}

		// 非 115，认为任务已结束（成功或失败由上层判断）
		return task.ErrEndLoop

	}, task.LoopInterval(gesprocessor.DefaultInterval))

	if err != nil {
		logs.Errorf("WaitTaskFinish error, gseTaskID %s, err=%+v", gseTaskID, err)
		return nil, err
	}
	return result, nil
}

// WaitTransferFileTaskFinish 等待文件传输任务执行结束
func (e *Executor) WaitTransferFileTaskFinish(
	ctx context.Context,
	gseTaskID string,
) (*gse.TransferFileResultData, error) {
	var (
		result *gse.TransferFileResultData
		err    error
	)

	err = task.LoopDoFunc(ctx, func() error {
		// 获取gse侧文件传输任务结果
		result, err = e.GseService.GetExtensionsTransferFileResult(ctx, &gse.GetTransferFileResultReq{
			TaskID: gseTaskID,
		})
		if err != nil {
			logs.Warnf("WaitTransferFileTaskFinish get gse task state error, gseTaskID %s, err=%+v", gseTaskID, err)
			return nil
		}

		// 检查所有目标的传输状态
		allFinished := true
		for _, r := range result.Result {
			// ErrorCode 115 表示任务正在执行中
			if gse.IsInProgress(r.ErrorCode) {
				allFinished = false
				logs.Infof("WaitTransferFileTaskFinish task %s is in progress, agentID: %s, errorCode=%d",
					gseTaskID, r.Content.DestAgentID, r.ErrorCode)
				break
			}
		}

		// 如果所有任务都已完成（成功或失败），结束循环
		if allFinished {
			logs.Infof("WaitTransferFileTaskFinish task %s finished", gseTaskID)
			return task.ErrEndLoop
		}

		// 继续等待
		return nil
	}, task.LoopInterval(2*time.Second))

	if err != nil {
		logs.Errorf("WaitTransferFileTaskFinish error, gseTaskID %s, err=%+v", gseTaskID, err)
		return nil, err
	}
	return result, nil
}

// WaitExecuteScriptFinish 等待脚本执行任务完成
func (e *Executor) WaitExecuteScriptFinish(ctx context.Context, gseTaskID, bkAgentID string) (*gse.ExecuteScriptResult, error) {
	var result *gse.ExecuteScriptResult

	err := wait.PollUntilContextTimeout(
		ctx,
		interval,
		maxWait,
		true,
		func(ctx context.Context) (bool, error) {
			resp, err := e.GseService.GetExecuteScriptResult(ctx, &gse.GetExecuteScriptResultReq{
				TaskID: gseTaskID,
				AgentTasks: []gse.AgentTaskQuery{
					{
						BkAgentID: bkAgentID,
						AtomicTasks: []gse.AtomicTaskQuery{
							{Offset: 0},
						},
					},
				},
			})
			if err != nil {
				return false, fmt.Errorf("get execute script result failed, taskID=%s, err=%v", gseTaskID, err)
			}

			result = resp

			// 是否仍存在执行中的任务
			for _, r := range result.Result {
				// 正在执行：status=1
				if r.ErrorCode == 0 && r.Status == 1 {
					logs.Infof("script executing, task=%s, agentID=%s, containerID=%s", gseTaskID, r.BkAgentID, r.BkContainerID)
					return false, nil
				}

				// 兼容 GSE 其他 executing 状态
				if gse.IsInProgress(r.ErrorCode) {
					logs.Infof(
						"script in progress, task=%s, agentID=%s, containerID=%s, errorCode=%d",
						gseTaskID, r.BkAgentID, r.BkContainerID, r.ErrorCode,
					)
					return false, nil
				}
			}

			// 全部非执行中
			return true, nil
		},
	)

	if err != nil {
		if wait.Interrupted(err) || errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("wait execute script timeout, taskID=%s, maxWait=%s", gseTaskID, maxWait)
		}
		return nil, err
	}

	return result, nil
}

func BuildConfigTaskPayload(
	process *table.Process,
	processInstance *table.ProcessInstance,
	templateRevision *table.TemplateRevision,
	configTemplateID uint32,
	configTemplateName string,
) *TaskPayload {

	key := fmt.Sprintf(
		"%d-%d-%d",
		configTemplateID,
		process.Attachment.CcProcessID,
		processInstance.Spec.ModuleInstSeq,
	)

	return &TaskPayload{
		ProcessPayload: &ProcessPayload{
			SetName:       process.Spec.SetName,
			ModuleName:    process.Spec.ModuleName,
			ServiceName:   process.Spec.ServiceName,
			Environment:   process.Spec.Environment,
			Alias:         process.Spec.Alias,
			FuncName:      process.Spec.FuncName,
			InnerIP:       process.Spec.InnerIP,
			AgentID:       process.Attachment.AgentID,
			CcProcessID:   process.Attachment.CcProcessID,
			HostInstSeq:   processInstance.Spec.HostInstSeq,
			ModuleInstSeq: processInstance.Spec.ModuleInstSeq,
			ConfigData:    process.Spec.SourceData,
			CloudID:       int(process.Attachment.CloudID),
		},
		ConfigPayload: &ConfigPayload{
			ConfigTemplateID:        configTemplateID,
			ConfigTemplateVersionID: templateRevision.ID,
			ConfigTemplateName:      configTemplateName,
			ConfigFileName:          templateRevision.Spec.Name,
			ConfigFilePath:          templateRevision.Spec.Path,
			ConfigFileOwner:         templateRevision.Spec.Permission.User,
			ConfigFileGroup:         templateRevision.Spec.Permission.UserGroup,
			ConfigFilePermission:    templateRevision.Spec.Permission.Privilege,
			ConfigInstanceKey:       key,
			ConfigContent:           "",
		},
	}
}

// 推送内容
type pushContent struct {
	receivers string
	title     string
	content   string
}

type pushFieldFiller func(*pushmanager.PushEventFields, pushContent)

var pushTypeFillers = map[pushmanager.PushType]pushFieldFiller{
	pushmanager.PushTypeRTX: func(f *pushmanager.PushEventFields, c pushContent) {
		f.RTXReceivers = c.receivers
		f.RTXTitle = c.title
		f.RTXContent = c.content
	},
	pushmanager.PushTypeMail: func(f *pushmanager.PushEventFields, c pushContent) {
		f.MailReceivers = c.receivers
		f.MailTitle = c.title
		f.MailContent = c.content
	},
	pushmanager.PushTypeMsg: func(f *pushmanager.PushEventFields, c pushContent) {
		f.MsgReceivers = c.receivers
		f.MsgContent = c.content
	},
}

var taskActionTextMap = map[taskActionKey]string{
	// 配置类
	{table.TaskObjectConfigFile, table.TaskActionConfigPublish}:  "配置文件下发",
	{table.TaskObjectConfigFile, table.TaskActionConfigGenerate}: "配置生成",
	{table.TaskObjectConfigFile, table.TaskActionConfigCheck}:    "配置检查",

	// 进程类
	{table.TaskObjectProcess, table.TaskActionStart}:      "进程启动",
	{table.TaskObjectProcess, table.TaskActionStop}:       "进程停止",
	{table.TaskObjectProcess, table.TaskActionKill}:       "进程强制停止",
	{table.TaskObjectProcess, table.TaskActionRestart}:    "进程重启",
	{table.TaskObjectProcess, table.TaskActionReload}:     "进程重载",
	{table.TaskObjectProcess, table.TaskActionRegister}:   "进程托管",
	{table.TaskObjectProcess, table.TaskActionUnregister}: "进程取消托管",
}

type taskActionKey struct {
	object table.TaskObject
	action table.TaskAction
}

func resolveTaskTypeText(object table.TaskObject, action table.TaskAction) (string, error) {
	if text, ok := taskActionTextMap[taskActionKey{object, action}]; ok {
		return text, nil
	}
	return "", fmt.Errorf("unsupported task action: object=%s action=%s",
		object, action)
}

func fillPushEventFields(fields *pushmanager.PushEventFields, pushTypes string, content pushContent) error {
	for _, t := range strings.Split(pushTypes, ",") {
		pt := pushmanager.PushType(strings.TrimSpace(t))
		filler, ok := pushTypeFillers[pt]
		if !ok {
			return fmt.Errorf("unsupported push type: %s", pt)
		}
		filler(fields, content)
	}

	fields.Types = pushTypes
	return nil
}

// buildTitle 构建标题
func buildTitle() string {
	return fmt.Sprintf("%s-任务执行结果通知", cc.G().PushProvider.Config.Domain)
}

var taskResultTextMap = map[table.TaskBatchStatus]string{
	table.TaskBatchStatusSucceed:      "执行成功",
	table.TaskBatchStatusFailed:       "执行失败",
	table.TaskBatchStatusPartlyFailed: "部分失败",
}

func buildTaskNotifyDimensions(bizName, taskID, operator, taskType, env, scope string, startTime, endTime *time.Time,
	successCnt, failedCnt, completedCnt uint32) map[string]string {
	return map[string]string{
		dimBizName:  bizName,
		dimTaskID:   taskID,
		dimOperator: operator,
		dimTaskType: taskType,
		dimEnv:      envToCN(env),
		dimScope:    scope,
		dimStart:    formatTime(startTime),
		dimEnd:      formatTime(endTime),
		dimResult:   buildExecuteResultText(successCnt, failedCnt, completedCnt),
	}
}

// 构建内容
func buildTaskNotifyContent(bizID uint32, taskID string, object table.TaskObject, action table.TaskAction,
	status table.TaskBatchStatus, dim map[string]string) (string, error) {

	// 1. 标题
	actionText, err := resolveTaskTypeText(object, action)
	if err != nil {
		return "", err
	}

	resultText, ok := taskResultTextMap[status]
	if !ok {
		return "", fmt.Errorf("unsupported task status: %s", status)
	}

	// 2. summary 行（正文标题）
	summary := fmt.Sprintf(
		"<h3>%s %s #%s %s</h3>",
		cc.G().PushProvider.Config.Domain,
		actionText,
		taskID,
		resultText,
	)

	// 3. detail（模板）
	detail := fmt.Sprintf(
		taskNotifyDetailTpl,
		dim[dimBizName],
		taskID,
		dim[dimOperator],
		dim[dimTaskType],
		dim[dimEnv],
		buildScopeText(dim[dimScope]),
		dim[dimStart],
		dim[dimEnd],
		dim[dimResult],
	)

	// 4. footer
	footer := fmt.Sprintf(taskNotifyFooterTpl, buildTaskDetailURL(bizID, taskID))

	return summary + "<br/><br/>" + detail + footer, nil
}

// 构建url
func buildTaskDetailURL(bizID uint32, batchID string) string {
	return fmt.Sprintf("%s/bscp/ui/space/%d/task/detail/%s",
		cc.G().PushProvider.Config.BscpHost, bizID, batchID)
}

func buildScopeText(scopeJSON string) string {
	if scopeJSON == "" {
		return "-"
	}

	var or table.OperateRange
	if err := json.Unmarshal([]byte(scopeJSON), &or); err != nil {
		// 兜底：解析失败直接返回原文本，避免通知空白
		return scopeJSON
	}

	if len(or.CCProcessID) == 0 {
		return "*.*.*.*"
	}

	ids := make([]string, 0, len(or.CCProcessID))
	for _, id := range or.CCProcessID {
		ids = append(ids, strconv.FormatUint(uint64(id), 10))
	}

	return "*.*.*.*." + strings.Join(ids, ",")
}

func buildExecuteResultText(success, failed, completed uint32) string {
	return fmt.Sprintf(
		executeResultTpl,
		success,
		failed,
		completed,
	)
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

// envToCN 将环境类型字符串转换为中文描述
// 1: 测试
// 2: 体验
// 3: 正式
func envToCN(env string) string {
	switch env {
	case "1":
		return "测试"
	case "2":
		return "体验"
	case "3":
		return "正式"
	default:
		return "未知环境"
	}
}

func (e *Executor) sendCallbackPushEvent(ctx context.Context, content pushContent, dimension map[string]string) error {
	var fields pushmanager.PushEventFields

	if err := fillPushEventFields(&fields, cc.G().PushProvider.Config.PushType, content); err != nil {
		return err
	}
	domain := cc.G().PushProvider.Config.Domain
	req := &pushmanager.CreatePushEventRequest{
		Event: pushmanager.PushEvent{
			Domain: domain,
			EventDetail: pushmanager.PushEventDetail{
				Fields: fields,
			},
			Dimension: &pushmanager.Dimension{
				Fields: dimension,
			},
			BkBizName: domain,
		}}

	resp, err := e.PM.CreatePushEvent(ctx, req)
	if err != nil {
		return err
	}

	if resp.Code != 0 {
		return fmt.Errorf("push failed: %s", resp.Message)
	}

	return nil
}

// CallbackNotify 回调通知
type CallbackNotify struct {
	BizID    uint32
	BatchID  uint32
	Operator string
	CbErr    error
}

// AfterCallbackNotify 任务回调后通知
func (e *Executor) AfterCallbackNotify(ctx context.Context, notify CallbackNotify) {
	// 注意：这里不返回 error，避免影响 Callback 主流程
	task, err := e.Dao.TaskBatch().GetByID(kit.New(), notify.BatchID)
	if err != nil {
		logs.Errorf("[AfterCallbackNotify] get task batch failed, batchID=%d, err=%v",
			notify.BatchID, err)
		return
	}

	taskTypeText, err := resolveTaskTypeText(
		task.Spec.TaskObject,
		task.Spec.TaskAction,
	)
	if err != nil {
		logs.Errorf("[AfterCallbackNotify] resolve task type text failed, err=%v", err)
		return
	}

	data, err := task.Spec.GetTaskExecutionData()
	if err != nil {
		logs.Errorf("[AfterCallbackNotify] get execution data failed, err=%v", err)
		return
	}

	scope, err := json.Marshal(data.OperateRange)
	if err != nil {
		logs.Errorf("[AfterCallbackNotify] marshal scope failed, err=%v", err)
		return
	}

	searchBizParams := &cmdb.SearchBizParams{
		Fields: []string{"bk_biz_id", "bk_biz_name"},
		Page:   cmdb.BasePage{Limit: 1},
		BizPropertyFilter: &cmdb.QueryFilter{
			Rule: cmdb.CombinedRule{
				Condition: cmdb.ConditionAnd,
				Rules: []cmdb.Rule{
					cmdb.AtomRule{
						Field:    cmdb.BizIDField,
						Operator: cmdb.OperatorEqual,
						Value:    notify.BizID,
					}},
			}},
	}

	bizResp, err := e.CMDBService.SearchBusiness(ctx, searchBizParams)
	if err != nil {
		logs.Errorf("[AfterCallbackNotify] search business failed, err=%v", err)
		return
	}

	var name string
	for _, v := range bizResp.Info {
		name = fmt.Sprintf("[%d] %s", v.BizID, v.BizName)
	}

	dim := buildTaskNotifyDimensions(
		name,
		strconv.Itoa(int(notify.BatchID)),
		task.Revision.Creator,
		taskTypeText,
		data.Environment,
		string(scope),
		task.Spec.StartAt,
		task.Spec.EndAt,
		task.Spec.SuccessCount,
		task.Spec.FailedCount,
		task.Spec.CompletedCount,
	)

	content, err := buildTaskNotifyContent(
		notify.BizID,
		strconv.Itoa(int(notify.BatchID)),
		task.Spec.TaskObject,
		task.Spec.TaskAction,
		task.Spec.Status,
		dim,
	)
	if err != nil {
		logs.Errorf("[AfterCallbackNotify] build notify content failed, err=%v", err)
		return
	}

	if err := e.sendCallbackPushEvent(ctx,
		pushContent{
			receivers: fmt.Sprintf("%s%s", task.Revision.Creator, cc.G().PushProvider.Config.MailSuffix),
			title:     buildTitle(),
			content:   content,
		},
		map[string]string{
			"biz": strconv.Itoa(int(notify.BizID)),
		},
	); err != nil {
		logs.Errorf("[AfterCallbackNotify] push notify failed, err=%v", err)
	}
}
