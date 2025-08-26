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

// Package itsmv4 xxx
package v4

import "github.com/TencentBlueKing/bk-bscp/internal/components/itsm/api"

type ListWorkflowReq struct {
	WorkflowKeys string `json:"workflow_keys"`
}

type ListWorkflowResp struct {
	api.CommonResp
	Data *ListWorkflowData `json:"data"`
}

type ListWorkflowData struct {
	Items []*Workflow `json:"items"`
}
type Workflow struct {
	WorkflowKey string `json:"workflow_key"`
	Key         string `json:"key"`
	Desc        any    `json:"desc"`
	Activities  map[string]struct {
		Key  string `json:"key"`
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"activities,omitempty"`
}
