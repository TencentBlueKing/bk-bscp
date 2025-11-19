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

package gse

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TencentBlueKing/bk-bscp/internal/components/gse"
	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

// setupTestRendererEnv 设置测试环境变量，用于指定 Python 脚本路径
// 如果环境变量未设置，使用项目根目录下的 render/python 路径
// 兼容从项目根目录或子目录执行测试
func setupTestRendererEnv(t *testing.T) {
	// 如果环境变量已设置，直接使用
	if os.Getenv("BSCP_PYTHON_RENDER_PATH") != "" {
		t.Logf("Using BSCP_PYTHON_RENDER_PATH from environment: %s", os.Getenv("BSCP_PYTHON_RENDER_PATH"))
		return
	}

	// 方法: 从当前工作目录向上查找项目根目录（通过 go.mod 文件定位）
	wd, err := os.Getwd()
	if err != nil {
		t.Logf("Warning: Could not get working directory: %v", err)
		return
	}

	// 从当前工作目录向上查找包含 go.mod 的目录（项目根目录）
	dir := wd
	// 最多向上查找 4 级目录，方便测试
	for i := 0; i < 4; i++ {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			// 找到 go.mod，说明这是项目根目录
			pythonPath := filepath.Join(dir, "render", "python")
			mainPy := filepath.Join(pythonPath, "main.py")
			if _, err := os.Stat(mainPy); err == nil {
				// 找到脚本，设置环境变量
				os.Setenv("BSCP_PYTHON_RENDER_PATH", pythonPath)
				return
			}
			// 项目根目录存在但没有 render/python/main.py
			break
		}
		// 向上查找
		parent := filepath.Dir(dir)
		if parent == dir {
			// 已经到达根目录
			break
		}
		dir = parent
	}

	t.Logf("Warning: Could not find render/python/main.py")
	t.Logf("Please set BSCP_PYTHON_RENDER_PATH environment variable")
	t.Logf("Example: export BSCP_PYTHON_RENDER_PATH=/path/to/render/python")
}

func TestBuildProcessOperate(t *testing.T) {
	// 设置测试环境变量
	setupTestRendererEnv(t)

	tests := []struct {
		name     string
		params   BuildProcessOperateParams
		wantErr  bool
		validate func(t *testing.T, result *gse.ProcessOperate)
	}{
		{
			name: "success with template rendering",
			params: BuildProcessOperateParams{
				BizID:         100,
				Alias:         "test-process",
				HostInstSeq:   1,
				ModuleInstSeq: 5,
				SetName:       "test-set",
				ModuleName:    "test-module",
				AgentID:       []string{"agent-001"},
				GseOpType:     gse.OpTypeStart,
				ProcessInfo: table.ProcessInfo{
					WorkPath:    "/opt/app/${bk_set_name}/${host_inst_seq}",
					PidFile:     "/var/run/${bk_process_name}_${host_inst_seq}.pid",
					User:        "appuser",
					StartCmd:    "start.sh ${module_inst_seq}",
					StopCmd:     "stop.sh ${module_inst_seq}",
					RestartCmd:  "restart.sh ${module_inst_seq}",
					ReloadCmd:   "reload.sh ${module_inst_seq}",
					FaceStopCmd: "kill.sh ${module_inst_seq}",
					Timeout:     30,
				},
			},
			wantErr: false,
			validate: func(t *testing.T, result *gse.ProcessOperate) {
				if result == nil {
					t.Fatal("result should not be nil")
				}
				if result.Meta.Namespace != gse.BuildNamespace(100) {
					t.Errorf("Namespace = %v, want %v", result.Meta.Namespace, gse.BuildNamespace(100))
				}
				if result.Meta.Name != gse.BuildProcessName("test-process", 1) {
					t.Errorf("Name = %v, want %v", result.Meta.Name, gse.BuildProcessName("test-process", 1))
				}
				if len(result.AgentIDList) != 1 || result.AgentIDList[0] != "agent-001" {
					t.Errorf("AgentIDList = %v, want [agent-001]", result.AgentIDList)
				}
				if result.OpType != gse.OpTypeStart {
					t.Errorf("OpType = %v, want %v", result.OpType, gse.OpTypeStart)
				}
				if result.Spec.Identity.ProcName != "test-process" {
					t.Errorf("ProcName = %v, want test-process", result.Spec.Identity.ProcName)
				}
				if result.Spec.Identity.User != "appuser" {
					t.Errorf("User = %v, want appuser", result.Spec.Identity.User)
				}
				// 验证模板渲染后的值（应该包含渲染后的内容）
				if result.Spec.Identity.SetupPath == "" {
					t.Error("SetupPath should be rendered and not empty")
				}
				if result.Spec.Identity.PidPath == "" {
					t.Error("PidPath should be rendered and not empty")
				}
				if result.Spec.Control.StartCmd == "" {
					t.Error("StartCmd should be rendered and not empty")
				}
				if result.Spec.MonitorPolicy.OpTimeout != 30 {
					t.Errorf("OpTimeout = %v, want 30", result.Spec.MonitorPolicy.OpTimeout)
				}
			},
		},
		{
			name: "success with simple values (no template)",
			params: BuildProcessOperateParams{
				BizID:         200,
				Alias:         "simple-process",
				HostInstSeq:   20,
				ModuleInstSeq: 6,
				SetName:       "prod-set",
				ModuleName:    "prod-module",
				AgentID:       []string{"agent-002"},
				GseOpType:     gse.OpTypeStop,
				ProcessInfo: table.ProcessInfo{
					WorkPath:    "/opt/app",
					PidFile:     "/var/run/app.pid",
					User:        "root",
					StartCmd:    "start.sh",
					StopCmd:     "stop.sh",
					RestartCmd:  "restart.sh",
					ReloadCmd:   "reload.sh",
					FaceStopCmd: "kill.sh",
					Timeout:     60,
				},
			},
			wantErr: false,
			validate: func(t *testing.T, result *gse.ProcessOperate) {
				if result == nil {
					t.Fatal("result should not be nil")
				}
				if result.Spec.Identity.SetupPath != "/opt/app" {
					t.Errorf("SetupPath = %v, want /opt/app", result.Spec.Identity.SetupPath)
				}
				if result.Spec.Identity.PidPath != "/var/run/app.pid" {
					t.Errorf("PidPath = %v, want /var/run/app.pid", result.Spec.Identity.PidPath)
				}
				if result.Spec.Control.StartCmd != "start.sh" {
					t.Errorf("StartCmd = %v, want start.sh", result.Spec.Control.StartCmd)
				}
			},
		},
		{
			name: "error with empty template field",
			params: BuildProcessOperateParams{
				BizID:         300,
				Alias:         "error-process",
				HostInstSeq:   30,
				ModuleInstSeq: 7,
				SetName:       "test-set",
				ModuleName:    "test-module",
				AgentID:       []string{"agent-003"},
				GseOpType:     gse.OpTypeStart,
				ProcessInfo: table.ProcessInfo{
					WorkPath:    "",
					PidFile:     "/var/run/app.pid",
					User:        "appuser",
					StartCmd:    "start.sh",
					StopCmd:     "stop.sh",
					RestartCmd:  "restart.sh",
					ReloadCmd:   "reload.sh",
					FaceStopCmd: "kill.sh",
					Timeout:     30,
				},
			},
			wantErr:  false,
			validate: nil,
		},
		{
			name: "success with all context variables",
			params: BuildProcessOperateParams{
				BizID:         400,
				Alias:         "context-process",
				HostInstSeq:   40,
				ModuleInstSeq: 8,
				SetName:       "my-set",
				ModuleName:    "my-module",
				AgentID:       []string{"agent-004"},
				GseOpType:     gse.OpTypeRestart,
				ProcessInfo: table.ProcessInfo{
					WorkPath:    "/opt/${bk_set_name}/${bk_module_name}",
					PidFile:     "/var/run/${bk_process_name}_${inst_id}.pid",
					User:        "appuser",
					StartCmd:    "start.sh --inst-id=${inst_id} --local-inst-id=${local_inst_id}",
					StopCmd:     "stop.sh ${InstID}",
					RestartCmd:  "restart.sh ${LocalInstID}",
					ReloadCmd:   "reload.sh ${SetName}",
					FaceStopCmd: "kill.sh ${ModuleName}",
					Timeout:     45,
				},
			},
			wantErr: false,
			validate: func(t *testing.T, result *gse.ProcessOperate) {
				if result == nil {
					t.Fatal("result should not be nil")
				}
				// 验证新版本字段渲染
				if result.Spec.Identity.SetupPath == "" {
					t.Error("SetupPath should be rendered")
				}
				// 验证老版本字段兼容性
				if result.Spec.Control.StopCmd == "" {
					t.Error("StopCmd should be rendered")
				}
			},
		},
		{
			name: "success with query operation",
			params: BuildProcessOperateParams{
				BizID:         500,
				Alias:         "query-process",
				HostInstSeq:   50,
				ModuleInstSeq: 9,
				SetName:       "query-set",
				ModuleName:    "query-module",
				AgentID:       []string{"agent-005"},
				GseOpType:     gse.OpTypeQuery,
				ProcessInfo: table.ProcessInfo{
					WorkPath:    "/opt/app",
					PidFile:     "/var/run/app.pid",
					User:        "appuser",
					StartCmd:    "start.sh",
					StopCmd:     "stop.sh",
					RestartCmd:  "restart.sh",
					ReloadCmd:   "reload.sh",
					FaceStopCmd: "kill.sh",
					Timeout:     10,
				},
			},
			wantErr: false,
			validate: func(t *testing.T, result *gse.ProcessOperate) {
				if result == nil {
					t.Fatal("result should not be nil")
				}
				if result.OpType != gse.OpTypeQuery {
					t.Errorf("OpType = %v, want %v", result.OpType, gse.OpTypeQuery)
				}
				if result.Spec.Resource.CPU != DefaultCPULimit {
					t.Errorf("CPU = %v, want %v", result.Spec.Resource.CPU, DefaultCPULimit)
				}
				if result.Spec.Resource.Mem != DefaultMemLimit {
					t.Errorf("Mem = %v, want %v", result.Spec.Resource.Mem, DefaultMemLimit)
				}
				if result.Spec.MonitorPolicy.AutoType != gse.AutoTypePersistent {
					t.Errorf("AutoType = %v, want %v", result.Spec.MonitorPolicy.AutoType, gse.AutoTypePersistent)
				}
				if result.Spec.MonitorPolicy.StartCheckSecs != DefaultStartCheckSecs {
					t.Errorf("StartCheckSecs = %v, want %v", result.Spec.MonitorPolicy.StartCheckSecs, DefaultStartCheckSecs)
				}
			},
		},
		{
			name: "error with invalid moduleInstSeq (zero)",
			params: BuildProcessOperateParams{
				BizID:         600,
				Alias:         "invalid-process",
				HostInstSeq:   60,
				ModuleInstSeq: 0, // 无效的 moduleInstSeq
				SetName:       "test-set",
				ModuleName:    "test-module",
				AgentID:       []string{"agent-006"},
				GseOpType:     gse.OpTypeStart,
				ProcessInfo: table.ProcessInfo{
					WorkPath:    "/opt/app",
					PidFile:     "/var/run/app.pid",
					User:        "appuser",
					StartCmd:    "start.sh",
					StopCmd:     "stop.sh",
					RestartCmd:  "restart.sh",
					ReloadCmd:   "reload.sh",
					FaceStopCmd: "kill.sh",
					Timeout:     30,
				},
			},
			wantErr:  true,
			validate: nil,
		},
		{
			name: "error with invalid hostInstSeq (zero)",
			params: BuildProcessOperateParams{
				BizID:         700,
				Alias:         "invalid-process",
				HostInstSeq:   0, // 无效的 hostInstSeq
				ModuleInstSeq: 10,
				SetName:       "test-set",
				ModuleName:    "test-module",
				AgentID:       []string{"agent-007"},
				GseOpType:     gse.OpTypeStart,
				ProcessInfo: table.ProcessInfo{
					WorkPath:    "/opt/app",
					PidFile:     "/var/run/app.pid",
					User:        "appuser",
					StartCmd:    "start.sh",
					StopCmd:     "stop.sh",
					RestartCmd:  "restart.sh",
					ReloadCmd:   "reload.sh",
					FaceStopCmd: "kill.sh",
					Timeout:     30,
				},
			},
			wantErr:  true,
			validate: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := BuildProcessOperate(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildProcessOperate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				if result != nil {
					t.Errorf("BuildProcessOperate() result = %v, want nil when error", result)
				}
				return
			}
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestBuildRenderContext(t *testing.T) {
	tests := []struct {
		name   string
		params BuildProcessOperateParams
		want   map[string]interface{}
	}{
		{
			name: "full context",
			params: BuildProcessOperateParams{
				Alias:         "test-process",
				HostInstSeq:   10,
				ModuleInstSeq: 5,
				SetName:       "test-set",
				ModuleName:    "test-module",
			},
			want: map[string]interface{}{
				"module_inst_seq": uint32(5),
				"inst_id_0":       uint32(4),
				"host_inst_seq":   uint32(10),
				"local_inst_id0":  uint32(9),
				"bk_set_name":     "test-set",
				"bk_module_name":  "test-module",
				"bk_process_name": "test-process",
				"ModuleInstSeq":   uint32(5),
				"InstID0":         uint32(4),
				"HostInstSeq":     uint32(10),
				"LocalInstID0":    uint32(9),
				"SetName":         "test-set",
				"ModuleName":      "test-module",
				"FuncID":          "test-process",
			},
		},
		{
			name: "empty set and module name",
			params: BuildProcessOperateParams{
				Alias:         "simple-process",
				HostInstSeq:   20,
				ModuleInstSeq: 6,
				SetName:       "",
				ModuleName:    "",
			},
			want: map[string]interface{}{
				"module_inst_seq": uint32(6),
				"inst_id_0":       uint32(5),
				"host_inst_seq":   uint32(20),
				"local_inst_id0":  uint32(19),
				"bk_set_name":     "",
				"bk_module_name":  "",
				"bk_process_name": "simple-process",
				"ModuleInstSeq":   uint32(6),
				"InstID0":         uint32(5),
				"HostInstSeq":     uint32(20),
				"LocalInstID0":    uint32(19),
				"SetName":         "",
				"ModuleName":      "",
				"FuncID":          "simple-process",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildRenderContext(tt.params)
			for key, wantValue := range tt.want {
				gotValue, ok := got[key]
				if !ok {
					t.Errorf("buildRenderContext() missing key: %s", key)
					continue
				}
				if gotValue != wantValue {
					t.Errorf("buildRenderContext()[%s] = %v, want %v", key, gotValue, wantValue)
				}
			}
			// 验证没有额外的键
			for key := range got {
				if _, ok := tt.want[key]; !ok {
					t.Errorf("buildRenderContext() has unexpected key: %s", key)
				}
			}
		})
	}
}

func TestValidateBuildProcessOperateParams(t *testing.T) {
	tests := []struct {
		name    string
		params  BuildProcessOperateParams
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid params",
			params: BuildProcessOperateParams{
				BizID:         100,
				Alias:         "test-process",
				HostInstSeq:   10,
				ModuleInstSeq: 5,
			},
			wantErr: false,
		},
		{
			name: "error with zero bizID",
			params: BuildProcessOperateParams{
				BizID:         0,
				Alias:         "test-process",
				HostInstSeq:   10,
				ModuleInstSeq: 5,
			},
			wantErr: true,
			errMsg:  "bizID is required",
		},
		{
			name: "error with empty alias",
			params: BuildProcessOperateParams{
				BizID:         100,
				Alias:         "",
				HostInstSeq:   10,
				ModuleInstSeq: 5,
			},
			wantErr: true,
			errMsg:  "alias is required",
		},
		{
			name: "error with zero processInstanceID",
			params: BuildProcessOperateParams{
				BizID:         100,
				Alias:         "test-process",
				HostInstSeq:   10,
				ModuleInstSeq: 5,
			},
			wantErr: true,
			errMsg:  "processInstanceID is required",
		},
		{
			name: "error with zero hostInstSeq",
			params: BuildProcessOperateParams{
				BizID:         100,
				Alias:         "test-process",
				HostInstSeq:   0,
				ModuleInstSeq: 5,
			},
			wantErr: true,
			errMsg:  "hostInstSeq is required",
		},
		{
			name: "error with zero moduleInstSeq",
			params: BuildProcessOperateParams{
				BizID:         100,
				Alias:         "test-process",
				HostInstSeq:   10,
				ModuleInstSeq: 0,
			},
			wantErr: true,
			errMsg:  "moduleInstSeq is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBuildProcessOperateParams(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateBuildProcessOperateParams() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateBuildProcessOperateParams() expected error but got nil")
					return
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("validateBuildProcessOperateParams() error = %v, want %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}
