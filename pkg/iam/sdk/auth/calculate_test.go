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

package auth

import (
	"context"
	"testing"

	"github.com/TencentBlueKing/bk-bscp/pkg/iam/client"
	"github.com/TencentBlueKing/bk-bscp/pkg/iam/sdk/operator"
)

// TestCalculatePolicyMissingResourceType 策略中的资源类型不在本次鉴权资源里时，
// 该条件视为不匹配，不应让整批鉴权失败。
func TestCalculatePolicyMissingResourceType(t *testing.T) {
	ctx := context.Background()
	authorizer := &Authorize{}

	// 本次鉴权只带业务资源
	resources := []client.Resource{
		{System: "bk_bscp", Type: "business", ID: "2"},
	}

	idPolicy := func(resource, value string) *operator.Policy {
		return &operator.Policy{
			Operator: operator.OpType(operator.Equal),
			Element: &operator.FieldValue{
				Field: operator.Field{Resource: resource, Attribute: client.IamIDKey},
				Value: value,
			},
		}
	}

	// 策略中出现本次鉴权未携带的 environment 资源类型
	envPolicy := idPolicy("environment", "1")
	bizPolicy := idPolicy("business", "2")

	tests := []struct {
		name   string
		policy *operator.Policy
		want   bool
	}{
		{
			name:   "field value policy with missing resource type",
			policy: envPolicy,
			want:   false,
		},
		{
			name: "and content with missing resource type",
			policy: &operator.Policy{
				Operator: operator.And,
				Element:  &operator.Content{Content: []*operator.Policy{envPolicy, bizPolicy}},
			},
			want: false,
		},
		{
			name: "or content with missing resource type",
			policy: &operator.Policy{
				Operator: operator.Or,
				Element:  &operator.Content{Content: []*operator.Policy{envPolicy, bizPolicy}},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authorized, err := authorizer.calculatePolicy(ctx, resources, tt.policy)
			if err != nil {
				t.Fatalf("calculate policy should not fail, err: %v", err)
			}
			if authorized != tt.want {
				t.Fatalf("authorized = %v, want %v", authorized, tt.want)
			}
		})
	}
}

// TestCalculatePolicyWithUnlimitedEnvironment 覆盖权限中心「环境: 无限制」的表达：
// 在项目上勾选后环境全部放开，策略路径里的环境层级是 "*"，
// 该项目下所有环境的服务都必须命中，其他项目仍然拒绝。
func TestCalculatePolicyWithUnlimitedEnvironment(t *testing.T) {
	ctx := context.Background()
	authorizer := &Authorize{}

	pathPolicy := func(value string) *operator.Policy {
		return &operator.Policy{
			Operator: operator.OpType(operator.StartWith),
			Element: &operator.FieldValue{
				Field: operator.Field{Resource: "app", Attribute: client.IamPathKey},
				Value: value,
			},
		}
	}
	idPolicy := func(value string) *operator.Policy {
		return &operator.Policy{
			Operator: operator.OpType(operator.Equal),
			Element: &operator.FieldValue{
				Field: operator.Field{Resource: "app", Attribute: client.IamIDKey},
				Value: value,
			},
		}
	}

	// 与权限中心实际返回一致：具体服务实例 + 「项目/环境无限制」实例
	policy := &operator.Policy{
		Operator: operator.Or,
		Element: &operator.Content{Content: []*operator.Policy{
			{
				Operator: operator.And,
				Element: &operator.Content{Content: []*operator.Policy{
					idPolicy("439"),
					pathPolicy("/biz,2/project,11/environment,16/"),
				}},
			},
			pathPolicy("/biz,2/project,1/environment,*/"),
		}},
	}

	tests := []struct {
		name string
		id   string
		path string
		want bool
	}{
		{
			name: "explicit service instance",
			id:   "439",
			path: "/biz,2/project,11/environment,16/",
			want: true,
		},
		{
			name: "service in unlimited project, exact environment",
			id:   "500",
			path: "/biz,2/project,1/environment,16/",
			want: true,
		},
		{
			name: "service in unlimited project, other environment",
			id:   "501",
			path: "/biz,2/project,1/environment,20/",
			want: true,
		},
		{
			name: "service in other project",
			id:   "502",
			path: "/biz,2/project,3/environment,16/",
			want: false,
		},
		{
			name: "service in other biz",
			id:   "503",
			path: "/biz,9/project,1/environment,16/",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources := []client.Resource{{
				System:    "bk-bscp",
				Type:      "app",
				ID:        tt.id,
				Attribute: map[string]interface{}{client.IamPathKey: []string{tt.path}},
			}}

			authorized, err := authorizer.calculatePolicy(ctx, resources, policy)
			if err != nil {
				t.Fatalf("calculate policy should not fail, err: %v", err)
			}
			if authorized != tt.want {
				t.Fatalf("authorized = %v, want %v", authorized, tt.want)
			}
		})
	}
}

// TestCalculatePolicyWithUnlimitedID 覆盖实例 ID 为 "*" 的表达：
// 在业务上勾选「无限制」时，任意实例都应命中，取反类运算符则一律不命中。
func TestCalculatePolicyWithUnlimitedID(t *testing.T) {
	ctx := context.Background()
	authorizer := &Authorize{}

	tests := []struct {
		name string
		op   operator.OpType
		want bool
	}{
		{name: "equal wildcard", op: operator.OpType(operator.Equal), want: true},
		{name: "in wildcard", op: operator.OpType(operator.In), want: true},
		{name: "not equal wildcard", op: operator.OpType(operator.NEqual), want: false},
		{name: "not in wildcard", op: operator.OpType(operator.Nin), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := interface{}(client.IamWildcard)
			if tt.op == operator.OpType(operator.In) || tt.op == operator.OpType(operator.Nin) {
				value = []interface{}{client.IamWildcard}
			}

			policy := &operator.Policy{
				Operator: tt.op,
				Element: &operator.FieldValue{
					Field: operator.Field{Resource: "app", Attribute: client.IamIDKey},
					Value: value,
				},
			}
			resources := []client.Resource{{System: "bk-bscp", Type: "app", ID: "999"}}

			authorized, err := authorizer.calculatePolicy(ctx, resources, policy)
			if err != nil {
				t.Fatalf("calculate policy should not fail, err: %v", err)
			}
			if authorized != tt.want {
				t.Fatalf("authorized = %v, want %v", authorized, tt.want)
			}
		})
	}
}
