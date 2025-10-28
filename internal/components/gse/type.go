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

import "encoding/json"

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
	OpTypeStart      = 0
	OpTypeStop       = 1
	OpTypeQuery      = 2
	OpTypeRegister   = 3
	OpTypeUnregister = 4
	OpTypeRestart    = 7
	OpTypeReload     = 8
	OpTypeKill       = 9
)

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
	Namespace string            `json:"namespace"`        // 命名空间，用于进程分组管理
	Name      string            `json:"name"`             // 进程名，用户自定义，与 namespace 共同用于进程标识
	Labels    map[string]string `json:"labels,omitempty"` // 进程标签，key-value 键值对
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
	StartCmd   string `json:"start_cmd,omitempty"`   // 启动命令
	StopCmd    string `json:"stop_cmd,omitempty"`    // 停止命令
	RestartCmd string `json:"restart_cmd,omitempty"` // 重启命令
	ReloadCmd  string `json:"reload_cmd,omitempty"`  // reload 命令
	KillCmd    string `json:"kill_cmd,omitempty"`    // kill 命令
	VersionCmd string `json:"version_cmd,omitempty"` // 进程版本查询命令
	HealthCmd  string `json:"health_cmd,omitempty"`  // 健康检查命令
}

// ProcessResource 进程资源信息
type ProcessResource struct {
	CPU float64 `json:"cpu"` // CPU 使用率上限百分比，例如 30.0 表示最多使用 30%
	Mem float64 `json:"mem"` // 内存使用率上限百分比，例如 10.0 表示最多使用 10%
}

// ProcessMonitorPolicy 进程存活状态监控策略
type ProcessMonitorPolicy struct {
	AutoType       int `json:"auto_type"`                  // 托管参数类型：1=常驻进程，2=单次执行进程
	StartCheckSecs int `json:"start_check_secs,omitempty"` // 启动后检查存活的时间（秒），默认 5
	StopCheckSecs  int `json:"stop_check_secs,omitempty"`  // 停止后检查存活的时间（秒）
	OpTimeout      int `json:"op_timeout,omitempty"`       // 命令执行超时时间（秒），默认 60
}

// FileTaskRequest 启动文件分发任务的请求参数
type FileTaskReq struct {
	Tasks          []FileTask `json:"tasks"`                     // 文件任务配置列表
	TimeoutSeconds int        `json:"timeout_seconds,omitempty"` // 任务超时时长，单位秒，>0，默认1000
	AutoMkdir      bool       `json:"auto_mkdir,omitempty"`      // 是否自动创建目录，默认 true
	UploadSpeed    int        `json:"upload_speed,omitempty"`    // 上传速度限制 (MB)，0 表示无限制
	DownloadSpeed  int        `json:"download_speed,omitempty"`  // 下载速度限制 (MB)，0 表示无限制
}

// FileTask 单个文件传输任务
type FileTask struct {
	Source FileSource `json:"source"` // 源文件定义
	Target FileTarget `json:"target"` // 目标文件定义
}

// FileSource 文件源定义
type FileSource struct {
	FileName string    `json:"file_name"`     // 源文件名，例如 xxxx.tar.gz
	StoreDir string    `json:"store_dir"`     // 源文件所在目录，例如 /data/store/
	MD5      string    `json:"md5,omitempty"` // 文件 MD5，可选，传输完成后校验
	Agent    FileAgent `json:"agent"`         // 源端 Agent 信息
}

// FileTarget 文件传输目标定义
type FileTarget struct {
	FileName   string      `json:"file_name,omitempty"`  // 传输后的文件名，默认与源文件一致
	StoreDir   string      `json:"store_dir,omitempty"`  // 传输后的存放目录，默认与源目录一致
	Owner      string      `json:"owner,omitempty"`      // 文件所有者，默认空
	Permission int         `json:"permission,omitempty"` // 文件权限，整型表示，默认 0
	Agents     []FileAgent `json:"agents"`               // 目标 Agent 信息列表
}

// FileAgent Agent 定义
type FileAgent struct {
	BkAgentID string `json:"bk_agent_id"`   // Agent ID，最长不超过64字符
	User      string `json:"user"`          // 目标机器上存在的用户名
	Pwd       string `json:"pwd,omitempty"` // 对应用户名的密码，可选
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
	ExecutingState TaskState = "pending"
	TimeoutState   TaskState = "timeout"
	FailedState    TaskState = "failed"
	SuccessedState TaskState = "TaskState"
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
