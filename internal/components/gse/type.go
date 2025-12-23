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

// Package gse provides gse api client.
package gse

import (
	"encoding/json"
	"fmt"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

// GESResponse 通用响应结构
type GESResponse struct {
	Code    int             `json:"code"`    // 错误编码
	Message string          `json:"message"` // 错误信息
	Data    json.RawMessage `json:"data"`
}

// Decode 把 Data 部分解码到目标结构里
func (r *GESResponse) Decode(v any) error {
	if len(r.Data) == 0 {
		return nil
	}
	return json.Unmarshal(r.Data, v)
}

// Encode 把目标结构编码回 Data 部分
func (r *GESResponse) Encode(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	r.Data = b
	return nil
}

// MultiProcOperateReq 批量进程操作请求
type MultiProcOperateReq struct {
	ProcOperateReq []ProcessOperate `json:"proc_operate_req"` // 进程操作请求数组
}

// OpType 操作类型
type OpType int

const (
	/*
		0:启动进程（start）,调用spec.control中的start_cmd启动进程，启动成功会注册托管；
		1:停止进程（stop）, 调用spec.control中的stop_cmd启动进程，停止成功会取消托管；
		2:进程状态查询；
		3:注册托管进程，令gse_agent对该进程进行托管（托管：当托管进程异常退出时，agent会自动拉起托管进程；当托管进程资源超限时，agent会杀死托管进程）；
		4:取消托管进程，令gse_agent对该进程不再托管；
		7:重启进程（restart）,调用spec.control中的restart_cmd启动进程；
		8:重新加载进程（reload）,调用spec.control中的reload_cmd启动进程；
		9:杀死进程（kill）,调用spec.control中的kill_cmd启动进程，杀死成功会取消托管
	*/
	OpTypeStart      OpType = 0
	OpTypeStop       OpType = 1
	OpTypeQuery      OpType = 2
	OpTypeRegister   OpType = 3
	OpTypeUnregister OpType = 4
	OpTypeRestart    OpType = 7
	OpTypeReload     OpType = 8
	OpTypeKill       OpType = 9
)

const (
	// 常驻进程
	AutoTypePersistent = 1
	// 单次执行进程
	AutoTypeOneTime = 2
)

// GSE 命名空间和格式常量
const (
	// NamespacePrefix GSE 命名空间前缀，用于进程分组管理
	NamespacePrefix = "GSEKIT_BIZ_"

	// ResultKeyWithInstanceFormat GSE 进程操作结果的 key 格式（带实例ID）
	// 格式：{agentID}:{namespace}:{processName}_{processInstanceID}
	// 示例：020000000242010a00002f17521298676503:GSEKIT_BIZ_3:http-server-test1_1
	ResultKeyWithInstanceFormat = "%s:%s%d:%s_%d"

	// ResultKeyWithoutInstanceFormat GSE 进程操作结果的 key 格式（不带实例ID）
	// 格式：{agentID}:{namespace}:{processName}
	// 示例：020000000242010a00002f17521298676503:GSEKIT_BIZ_3:http-server-test1
	ResultKeyWithoutInstanceFormat = "%s:%s%d:%s"
)

// GSE 错误码常量
const (
	// ErrCodeSuccess 操作成功
	ErrCodeSuccess = 0

	// ErrCodeInProgress 任务正在执行中（GSE 侧任务尚未完成）
	ErrCodeInProgress = 115

	// ErrCodeStopping 停止启动
	ErrCodeStopping = 1015012
)

// BuildNamespace 构建 GSE 命名空间
// 格式：GSEKIT_BIZ_{bizID}
func BuildNamespace(bizID uint32) string {
	return fmt.Sprintf("%s%d", NamespacePrefix, bizID)
}

// BuildProcessName 构建下发的进程别名
func BuildProcessName(alias string, hostInstSeq uint32) string {
	return fmt.Sprintf("%s_%d", alias, hostInstSeq)
}

// BuildResultKey 构建 GSE 进程操作结果的查询 key
func BuildResultKey(agentID string, bizID uint32, alias string, hostInstSeq uint32) string {
	namespace := BuildNamespace(bizID)
	processName := BuildProcessName(alias, hostInstSeq)
	return fmt.Sprintf("%s:%s:%s", agentID, namespace, processName)
}

// ConvertProcessOperateTypeToOpType 将 ProcessOperateType 转换为 GSE OpType
// GSE 操作类型定义：
// 0: 启动进程（start）- 调用 spec.control 中的 start_cmd，启动成功会注册托管
// 1: 停止进程（stop）- 调用 spec.control 中的 stop_cmd，停止成功会取消托管
// 2: 进程状态查询
// 3: 注册托管进程 - 令 gse_agent 对该进程进行托管
// 4: 取消托管进程 - 令 gse_agent 对该进程不再托管
// 7: 重启进程（restart）- 调用 spec.control 中的 restart_cmd
// 8: 重新加载进程（reload）- 调用 spec.control 中的 reload_cmd
// 9: 杀死进程（kill）- 调用 spec.control 中的 kill_cmd，杀死成功会取消托管
func ConvertProcessOperateTypeToOpType(operateType table.ProcessOperateType) (OpType, error) {
	switch operateType {
	case table.StartProcessOperate:
		return OpTypeStart, nil
	case table.StopProcessOperate:
		return OpTypeStop, nil
	case table.QueryStatusProcessOperate:
		return OpTypeQuery, nil
	case table.RegisterProcessOperate:
		return OpTypeRegister, nil
	case table.UnregisterProcessOperate:
		return OpTypeUnregister, nil
	case table.RestartProcessOperate:
		return OpTypeRestart, nil
	case table.ReloadProcessOperate:
		return OpTypeReload, nil
	case table.KillProcessOperate:
		return OpTypeKill, nil
	default:
		return OpType(0), fmt.Errorf("unsupported operation type: %s", operateType)
	}
}

// IsSuccess 判断错误码是否表示成功
func IsSuccess(errorCode int) bool {
	return errorCode == ErrCodeSuccess
}

// IsInProgress 判断错误码是否表示任务正在执行中
func IsInProgress(errorCode int) bool {
	return errorCode == ErrCodeInProgress
}

// nolint
// ProcessOperate 单个进程操作对象
type ProcessOperate struct {
	Meta        ProcessMeta `json:"meta"`            // 进程管理元数据
	AgentIDList []string    `json:"agent_id_list"`   // 目标节点 Agent ID 列表
	Hosts       []HostInfo  `json:"hosts,omitempty"` // 主机对象数组（可选，若设置了 AgentIDList 则忽略） ,只用agentID 进行下发
	OpType      OpType      `json:"op_type"`         // 操作类型: 0=start,1=stop,2=query,3=register,4=unregister,7=restart,8=reload,9=kill
	Spec        ProcessSpec `json:"spec"`            // 进程详细信息
}

// ProcessMeta 进程管理元数据
type ProcessMeta struct {
	Namespace string `json:"namespace"` // 命名空间，用于进程分组管理
	Name      string `json:"name"`      // 进程名，用户自定义，与 namespace 共同用于进程标识
}

// HostInfo 主机信息
type HostInfo struct {
	IP        string `json:"ip"`          // 主机 IP 地址
	BkCloudID int    `json:"bk_cloud_id"` // 云区域 ID
}

// ProcessSpec 进程详细信息
type ProcessSpec struct {
	Identity      ProcessIdentity      `json:"identity"`       // 进程身份信息
	Control       ProcessControl       `json:"control"`        // 进程控制信息
	Resource      ProcessResource      `json:"resource"`       // 进程资源信息
	MonitorPolicy ProcessMonitorPolicy `json:"monitor_policy"` // 存活状态监控策略
}

// ProcessIdentity 进程身份信息
type ProcessIdentity struct {
	ProcName   string `json:"proc_name"`             // 进程二进制文件名
	SetupPath  string `json:"setup_path"`            // 工作路径（绝对路径）
	PidPath    string `json:"pid_path"`              // PID 文件路径（绝对路径）
	ConfigPath string `json:"config_path,omitempty"` // 配置文件路径（绝对路径，可选）
	LogPath    string `json:"log_path,omitempty"`    // 日志路径（绝对路径，可选）
	User       string `json:"user"`                  // 进程所属系统账户（如 root）
}

// ProcessControl 进程控制信息
type ProcessControl struct {
	StartCmd   string `json:"start_cmd,omitempty"`   // 启动命令（可选）
	StopCmd    string `json:"stop_cmd,omitempty"`    // 停止命令（可选）
	RestartCmd string `json:"restart_cmd,omitempty"` // 重启命令（可选）
	ReloadCmd  string `json:"reload_cmd,omitempty"`  // reload 命令（可选）
	KillCmd    string `json:"kill_cmd,omitempty"`    // kill 命令（可选）
	VersionCmd string `json:"version_cmd,omitempty"` // 进程版本查询命令（可选）
	HealthCmd  string `json:"health_cmd,omitempty"`  // 健康检查命令（可选）
}

// ProcessResource 进程资源信息
type ProcessResource struct {
	CPU float64 `json:"cpu"` // CPU 使用率上限百分比，例如 30.0 表示最多使用 30%（必填）
	Mem float64 `json:"mem"` // 内存使用率上限百分比，例如 10.0 表示最多使用 10%（必填）
}

// ProcessMonitorPolicy 进程存活状态监控策略
type ProcessMonitorPolicy struct {
	AutoType       int `json:"auto_type"`                  // 托管参数类型：1=常驻进程，2=单次执行进程（必填）
	StartCheckSecs int `json:"start_check_secs,omitempty"` // 启动后检查存活的时间（秒），默认 5（可选）
	StopCheckSecs  int `json:"stop_check_secs,omitempty"`  // 停止后检查存活的时间（秒）(可选)
	OpTimeout      int `json:"op_timeout,omitempty"`       // 命令执行超时时间（秒），默认 60（可选）
}

// TaskResp xxx
type TaskResp struct {
	Result struct {
		TaskID string `json:"task_id"` // 任务 ID
	} `json:"result"`
}

// UpdateProcInfoReq xxx
type UpdateProcInfoReq struct {
	Meta        ProcessMeta `json:"meta"`            // 进程管理元数据
	AgentIDList []string    `json:"agent_id_list"`   // 目标节点 Agent ID 列表
	Hosts       []HostInfo  `json:"hosts,omitempty"` // 主机对象数组（可选，若设置了 AgentIDList 则忽略）
	Spec        ProcessSpec `json:"spec"`            // 进程详细信息
}

// ProcResult 表示单个进程操作结果的详细信息
// 每个 key 对应一个进程的执行结果
type ProcResult struct {
	ErrorCode int    `json:"error_code"`
	ErrorMsg  string `json:"error_msg"`
	Content   string `json:"content"`
}

// TaskReq 任务的请求参数
type TaskReq struct {
	AgentIDList []string `json:"agent_id_list"` // 目标节点 Agent ID 列表
	TaskID      string   `json:"task_id"`       // 需要终止的任务 ID
}

// TaskState 任务状态
type TaskState string

const (
	PendingState   TaskState = "pending"
	ExecutingState TaskState = "executing"
	TimeoutState   TaskState = "timeout"
	FailedState    TaskState = "failed"
	SuccessedState TaskState = "successed"
)

// TaskOperateResult 表示任务操作的具体结果
type TaskOperateResult struct {
	Result struct {
		State           TaskState `json:"state"`            // pending，executing，timeout，failed，successed
		SuccessedAgents []string  `json:"successed_agents"` // 执行成功的 agent 列表
		TimeoutAgents   []string  `json:"timeout_agents"`   // 执行超时的 agent 列表
		FailedAgents    []string  `json:"failed_agents"`    // 执行失败的 agent 列表
		PendingAgents   []string  `json:"pending_agents"`   // 状态暂时不确定的 agent 列表
		OfflineAgents   []string  `json:"offline_agents"`   // 离线的 agent 列表
		RestartedAgents []string  `json:"restarted_agents"` // 在任务执行期间发生重启的 agent 列表
	} `json:"result"`
}

// QueryProcResultReq  用于根据 task_id 查询进程操作任务的执行结果。
type QueryProcResultReq struct {
	// TaskID 为进程操作接口返回的任务 ID。
	// 该字段为必选参数。
	TaskID string `json:"task_id" binding:"required"`
}

// QueryProcStatusReq 定义“查询进程状态信息”接口的请求参数结构
// 该接口用于查询指定进程在多个 Agent 节点上的运行状态
type QueryProcStatusReq struct {
	// Meta 进程管理元数据，用于唯一标识一个进程。
	// 该字段为必选参数。
	Meta ProcMeta `json:"meta" binding:"required"`

	// AgentIDList 目标节点 Agent ID 列表。
	// 每个 ID 最大长度不超过 64 个字符。
	// 若设置此参数，则 hosts 参数会被忽略。
	AgentIDList []string `json:"agent_id_list" binding:"required"`

	// Hosts 主机对象数组，为兼容参数。
	// 当设置了 agent_id_list 时，此参数会被忽略。
	Hosts []Host `json:"hosts,omitempty"`
}

// ProcMeta 定义进程的元数据信息，用于进程分组与标识。
type ProcMeta struct {
	// Namespace 命名空间，用于进程分组管理。
	Namespace string `json:"namespace" binding:"required"`

	// Name 进程名，由用户自定义。
	// 与 Namespace 共同组成进程的唯一标识。
	Name string `json:"name" binding:"required"`

	// Labels 进程标签，用于按标签管理进程。
	// key 和 value 均为用户自定义，value 为字符串。
	Labels map[string]string `json:"labels,omitempty"`
}

// Host 表示主机信息对象。
// 用于指定进程所在主机的 IP 和云区域 ID。
type Host struct {
	// IP 主机 IP 地址。
	IP string `json:"ip" binding:"required"`

	// BkCloudID 云区域 ID。
	BkCloudID int `json:"bk_cloud_id" binding:"required"`
}

// ProcStatusData 表示查询结果中的数据部分。
// 包含进程状态信息数组。
type ProcStatusData struct {
	// ProcInfos 进程状态信息列表。
	ProcInfos []ProcInfo `json:"proc_infos"`
}

// ProcInfo 表示单个进程在某个 Agent 节点上的状态信息。
type ProcInfo struct {
	// Meta 进程管理元数据。
	Meta ProcMeta `json:"meta"`

	// BkAgentID Agent ID，最大长度不超过 64 个字符。
	BkAgentID string `json:"bk_agent_id"`

	// Status 动态运行状态：
	// 0 表示未注册；
	// 1 表示运行中；
	// 2 表示停止。
	Status int `json:"status"`

	// IsAuto 表示该进程是否被托管。
	// true 表示已托管；false 表示未托管。
	IsAuto bool `json:"isauto"`

	// PID 进程 ID。
	PID int `json:"pid"`

	// Version 进程版本号。
	Version string `json:"version"`

	// ReportTime 信息上报时间（时间戳）。
	ReportTime int64 `json:"report_time"`

	// CPUUsage 进程 CPU 使用率。
	CPUUsage float64 `json:"cpu_usage"`

	// MemUsage 进程内存使用率。
	MemUsage float64 `json:"mem_usage"`
}

// SyncQueryProcStatusReq 定义“同步查询进程状态信息”接口的请求结构。
// 该接口用于分页同步查询指定命名空间下所有进程的运行状态。
type SyncQueryProcStatusReq struct {
	// Meta 进程管理元数据。
	// 用于指定查询的命名空间。
	Meta SyncProcMeta `json:"meta" binding:"required"`

	// Page 分页查询条件。
	// 指定记录起始位置与每页数量。
	Page Page `json:"page" binding:"required"`
}

// SyncProcMeta 定义同步查询中用于过滤的进程元数据。
// 仅包含命名空间字段。
type SyncProcMeta struct {
	// Namespace 命名空间，用于进程分组管理。
	Namespace string `json:"namespace" binding:"required"`
}

// Page 定义分页查询条件。
type Page struct {
	// Start 记录开始位置（从 0 开始）。
	Start int `json:"start" binding:"required"`

	// Limit 每页限制条数，最大 1000。
	Limit int `json:"limit" binding:"required"`
}

// SyncProcStatusData 表示同步查询结果的数据部分。
// 包含记录总数及进程状态列表。
type SyncProcStatusData struct {
	// Count 查询结果的总记录条数。
	Count int `json:"count"`

	// ProcInfos 进程状态信息列表。
	ProcInfos []SyncProcInfo `json:"proc_infos"`
}

// SyncProcInfo 表示单个进程的状态信息。
// 用于展示进程的运行状态及资源使用情况。
type SyncProcInfo struct {
	// Meta 进程管理元数据。
	Meta ProcMeta `json:"meta"`

	// BkAgentID Agent ID，最大长度不超过 64 个字符。
	BkAgentID string `json:"bk_agent_id"`

	// Status 动态运行状态：
	// 0 表示未注册；
	// 1 表示运行中；
	// 2 表示停止。
	Status int `json:"status"`

	// PID 进程 ID。
	PID int `json:"pid"`

	// Version 进程版本号。
	Version string `json:"version"`

	// ReportTime 信息上报时间（时间戳）。
	ReportTime int64 `json:"report_time"`

	// CPUUsage 进程 CPU 使用率。
	CPUUsage float64 `json:"cpu_usage"`

	// MemUsage 进程内存使用率。
	MemUsage float64 `json:"mem_usage"`
}

// ProcOperationReq 定义“进程操作”接口的请求结构。
// 用于在指定节点上执行进程的启动、停止、托管、取消托管、重启等操作。
type ProcOperationReq struct {
	// Meta 进程管理元数据，用于唯一标识进程。
	Meta ProcMeta `json:"meta" binding:"required"`

	// AgentIDList 目标节点 Agent ID 列表。
	// 每个 ID 最大长度不超过 64 个字符。
	// 若设置此参数，则 hosts 参数会被忽略。
	AgentIDList []string `json:"agent_id_list" binding:"required"`

	// Hosts 主机对象数组，为兼容参数。
	// 当设置了 agent_id_list 时，此参数会被忽略。
	Hosts []Host `json:"hosts,omitempty"`

	// OpType 进程操作类型：
	// 0: 启动进程 (start)
	// 1: 停止进程 (stop)
	// 2: 查询状态 (status)
	// 3: 注册托管 (register)
	// 4: 取消托管 (unregister)
	// 7: 重启进程 (restart)
	// 8: 重新加载 (reload)
	// 9: 杀死进程 (kill)
	OpType int `json:"op_type" binding:"required"`

	// Spec 进程详细信息定义，包含身份、控制、资源和监控策略。
	Spec ProcSpec `json:"spec" binding:"required"`
}

// ProcSpec 定义进程的详细信息。
// 包含进程身份、控制命令、资源限制及监控策略。
type ProcSpec struct {
	// Identity 进程身份信息。
	Identity ProcIdentity `json:"identity" binding:"required"`

	// Control 进程控制命令。
	Control ProcControl `json:"control" binding:"required"`

	// Resource 进程资源限制。
	Resource ProcResource `json:"resource" binding:"required"`

	// MonitorPolicy 进程存活监控策略。
	MonitorPolicy ProcMonitorPolicy `json:"monitor_policy" binding:"required"`
}

// ProcIdentity 定义进程的身份信息。
type ProcIdentity struct {
	// ProcName 进程二进制文件名。
	ProcName string `json:"proc_name" binding:"required"`

	// SetupPath 工作路径（绝对路径）。
	SetupPath string `json:"setup_path" binding:"required"`

	// PidPath PID 文件路径（绝对路径）。
	PidPath string `json:"pid_path" binding:"required"`

	// ConfigPath 配置文件路径（绝对路径）。
	ConfigPath string `json:"config_path,omitempty"`

	// LogPath 日志文件路径（绝对路径）。
	LogPath string `json:"log_path,omitempty"`

	// User 进程所属系统账户，如 root 或 Administrator。
	User string `json:"user" binding:"required"`
}

// ProcControl 定义进程的控制命令集合。
// 所有命令均为可选字段。
type ProcControl struct {
	StartCmd   string `json:"start_cmd,omitempty"`   // 启动命令
	StopCmd    string `json:"stop_cmd,omitempty"`    // 停止命令
	RestartCmd string `json:"restart_cmd,omitempty"` // 重启命令
	ReloadCmd  string `json:"reload_cmd,omitempty"`  // reload 命令
	KillCmd    string `json:"kill_cmd,omitempty"`    // kill 命令
	VersionCmd string `json:"version_cmd,omitempty"` // 版本查询命令
	HealthCmd  string `json:"health_cmd,omitempty"`  // 健康检查命令
}

// ProcResource 定义进程资源限制信息。
type ProcResource struct {
	// CPU CPU 使用率上限百分比（总占比，非单核占比）。
	// 例如 30.0 表示 CPU 总使用率上限为 30%。
	CPU float64 `json:"cpu" binding:"required"`

	// Mem 内存使用率上限百分比。
	// 例如 10.0 表示内存使用率上限为 10%。
	Mem float64 `json:"mem" binding:"required"`
}

// ProcMonitorPolicy 定义进程的存活监控策略。
// 兼容字段名为 alive_monitor_policy。
type ProcMonitorPolicy struct {
	// AutoType 托管参数类型：
	// 1 表示常驻进程；
	// 2 表示单次执行进程。
	AutoType int `json:"auto_type" binding:"required"`

	// StartCheckSecs 启动命令执行后开始检查进程存活的时间（秒）。
	// 默认值为 5。
	StartCheckSecs int `json:"start_check_secs,omitempty"`

	// StopCheckSecs 停止命令执行后开始检查进程存活的时间（秒）。
	StopCheckSecs int `json:"stop_check_secs,omitempty"`

	// OpTimeout 命令执行超时时间（秒）。
	// 默认值为 60。
	OpTimeout int `json:"op_timeout,omitempty"`
}

// ProcOperationData 定义进程操作返回的结果数据。
type ProcOperationData struct {
	// TaskID 进程操作实例 ID。
	TaskID string `json:"task_id"`
}

// ProcessStatusContent GSE 进程查询接口返回的进程状态 content 内容结构
type ProcessStatusContent struct {
	IP        string          `json:"ip"`
	BkAgentID string          `json:"bk_agent_id"`
	UTCTime   string          `json:"utctime"`
	UTCTime2  string          `json:"utctime2"`
	Timezone  int             `json:"timezone"`
	Process   []ProcessDetail `json:"process"`
}

// ProcessDetail GSE 进程查询接口返回的进程详情
type ProcessDetail struct {
	ProcName string            `json:"procname"`
	Instance []ProcessInstance `json:"instance"`
}

// ProcessInstance GSE 进程查询接口返回的进程实例详情
type ProcessInstance struct {
	Cmdline       string  `json:"cmdline"`
	ProcessName   string  `json:"processName"`
	Version       string  `json:"version"`
	Health        string  `json:"health"`
	IsAuto        bool    `json:"isAuto"`          // 是否托管
	CPUUsage      float64 `json:"cpuUsage"`        // CPU 使用率
	CPUUsageAve   float64 `json:"cpuUsageAve"`     // CPU 平均使用率
	PhyMemUsage   float64 `json:"phyMemUsage"`     // 物理内存使用率
	UsePhyMem     int64   `json:"usePhyMem"`       // 使用的物理内存
	DiskSize      int64   `json:"diskSize"`        // 磁盘大小
	PID           int     `json:"pid"`             // 进程ID，小于0表示进程未运行
	StartTime     string  `json:"startTime"`       // 启动时间
	Stat          string  `json:"stat"`            // 状态
	UTime         string  `json:"utime"`           // 用户态时间
	STime         string  `json:"stime"`           // 内核态时间
	ThreadCount   int     `json:"threadCount"`     // 线程数
	ElapsedTime   int64   `json:"elapsedTime"`     // 运行时长
	RegisterTime  int64   `json:"register_time"`   // 注册时间
	LastStartTime int64   `json:"last_start_time"` // 最后启动时间
	ReportTime    int64   `json:"report_time"`     // 上报时间
}

// StoppingContent 停止状态返回的内容
type StoppingContent struct {
	Value []StoppingItem `json:"value"`
}

type StoppingItem struct {
	ProcName   string `json:"procName"`
	SetupPath  string `json:"setupPath"`
	FuncID     string `json:"funcID"`
	InstanceID string `json:"instanceID"`
	Result     string `json:"result"`
	IsAuto     bool   `json:"isAuto"`
}

// ExecuteScriptReq 表示一次作业执行请求的整体结构
type ExecuteScriptReq struct {
	// Agents 执行脚本的目标机器列表
	Agents []Agent `json:"agents"`
	// Scripts 需要下发并执行的脚本列表
	Scripts []Script `json:"scripts"`
	// AtomicTasks 原子任务定义，每个任务通常对应一次命令执行
	AtomicTasks []AtomicTask `json:"atomic_tasks"`
	// AtomicTasksRelations 原子任务之间的依赖关系定义
	AtomicTasksRelations []AtomicTaskRelation `json:"atomic_tasks_relations"`
}

// Agent 表示一台执行任务的 Agent 机器
type Agent struct {
	// BkAgentID Agent 在系统中的唯一标识
	BkAgentID string `json:"bk_agent_id"`
	// User 执行命令时使用的系统用户
	User string `json:"user"`
	// Pwd 对应用户的密码（可为空，表示免密或其他认证方式）
	Pwd string `json:"pwd"`
	// BkContainerID 目标容器 ID, 空则为主机
	BkContainerID string `json:"bk_container_id"`
}

// Script 表示需要下发到目标机器的脚本信息
type Script struct {
	// ScriptName 脚本文件名
	ScriptName string `json:"script_name"`
	// ScriptStoreDir 脚本在目标机器上的存放目录
	ScriptStoreDir string `json:"script_store_dir"`
	// ScriptContent 脚本的具体内容
	ScriptContent string `json:"script_content"`
}

// AtomicTask 表示一个最小执行单元（原子任务）
type AtomicTask struct {
	// AtomicTaskID 原子任务唯一 ID，用于依赖关系引用
	AtomicTaskID int `json:"atomic_task_id"`
	// Command 实际执行的命令或脚本路径
	Command string `json:"command"`
	// TimeoutSeconds 执行超时时间（单位：秒，0 表示不限制）
	TimeoutSeconds int `json:"timeout_seconds"`
}

// AtomicTaskRelation 定义原子任务之间的依赖关系
type AtomicTaskRelation struct {
	// AtomicTaskID 当前原子任务 ID
	AtomicTaskID int `json:"atomic_task_id"`
	// AtomicTaskIDIdx 当前任务依赖的其他原子任务 ID 列表
	// 只有依赖任务全部完成后，当前任务才可执行
	AtomicTaskIDIdx []int `json:"atomic_task_id_idx"`
}

// GetExecuteScriptResultReq 定义“获取脚本执行结果”接口的请求参数结构
type GetExecuteScriptResultReq struct {
	// TaskID 任务 ID（由上游任务创建返回）
	TaskID string `json:"task_id"`
	// AgentTasks 按 Agent 维度查询的任务列表
	AgentTasks []AgentTaskQuery `json:"agent_tasks"`
}

// AgentTaskQuery 单个 Agent 的查询条件
type AgentTaskQuery struct {
	// BkAgentID Agent 唯一标识
	BkAgentID string `json:"bk_agent_id"`
	// BkContainerID 容器 ID（非容器场景一般为全 0）
	BkContainerID string `json:"bk_container_id"`
	// AtomicTasks 需要查询的原子任务列表
	AtomicTasks []AtomicTaskQuery `json:"atomic_tasks"`
}

// AtomicTaskQuery 原子任务查询条件
type AtomicTaskQuery struct {
	// AtomicTaskID 原子任务 ID
	AtomicTaskID int `json:"atomic_task_id"`
	// Offset 返回日志/屏幕输出的起始偏移
	Offset int `json:"offset"`
	// Limit 返回日志/屏幕输出的最大长度
	Limit int `json:"limit"`
}

// GetExecuteScriptResultResp 执行结果响应
type GetExecuteScriptResultResp struct {
	Code    int                  `json:"code"`
	Message string               `json:"message"`
	Data    *ExecuteScriptResult `json:"data"`
}

// ExecuteScriptResult 响应数据主体
type ExecuteScriptResult struct {
	// Result 每个 Agent 原子任务的执行结果
	Result []AgentAtomicTaskResult `json:"result"`
}

// AgentAtomicTaskResult Agent 上某个原子任务的执行结果
type AgentAtomicTaskResult struct {
	// BkAgentID Agent 唯一标识
	BkAgentID string `json:"bk_agent_id"`
	// BkContainerID 容器 ID
	BkContainerID string `json:"bk_container_id"`
	// Status 任务状态（如：0=成功，其他值表示运行中/失败，具体以 GSE 定义为准）
	Status int `json:"status"`
	// ErrorCode 错误码
	ErrorCode int `json:"error_code"`
	// ErrorMsg 错误信息
	ErrorMsg string `json:"error_msg"`
	// StartTime 任务开始时间（Unix 时间戳，单位以接口约定为准，通常为秒或毫秒）
	StartTime int64 `json:"start_time"`
	// EndTime 任务结束时间（Unix 时间戳）
	EndTime int64 `json:"end_time"`
	// ScriptExitCode 脚本退出码（0 表示成功）
	ScriptExitCode int `json:"script_exit_code"`
	// Tag 任务标签（可用于幂等或追踪）
	Tag string `json:"tag"`
	// Screen 脚本输出内容（受 offset/limit 影响）
	Screen string `json:"screen"`
	// AtomicTaskID 原子任务 ID
	AtomicTaskID int `json:"atomic_task_id"`
}
