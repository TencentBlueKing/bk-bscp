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

package render

// RenderInput represents the input data for template rendering
// nolint:revive
type RenderInput struct {
	// Template is the Mako template content
	Template string `json:"template"`
	// Context contains the variables to be used in template rendering
	Context map[string]interface{} `json:"context"`
}

// RenderOutput represents the output of template rendering
// nolint:revive
type RenderOutput struct {
	// Result is the rendered content
	Result string
	// Error contains error message if rendering failed
	Error string
}

// ProcessContext represents the context data structure for process template rendering
// This matches the structure from get_process_context in the original Python code
type ProcessContext struct {
	Scope           string                 `json:"Scope"`
	FuncID          string                 `json:"FuncID"`
	ModuleInstSeq   int                    `json:"ModuleInstSeq"`
	InstID0         int                    `json:"InstID0"`
	HostInstSeq     int                    `json:"HostInstSeq"`
	LocalInstID0    int                    `json:"LocalInstID0"`
	BkSetName       string                 `json:"bk_set_name"`
	BkModuleName    string                 `json:"bk_module_name"`
	BkHostInnerIP   string                 `json:"bk_host_innerip"`
	BkCloudID       int                    `json:"bk_cloud_id"`
	BkProcessID     int                    `json:"bk_process_id"`
	BkProcessName   string                 `json:"bk_process_name"`
	FuncName        string                 `json:"FuncName"`
	ProcName        string                 `json:"ProcName"`
	WorkPath        string                 `json:"WorkPath"`
	GlobalVariables map[string]interface{} `json:"global_variables"`
}
