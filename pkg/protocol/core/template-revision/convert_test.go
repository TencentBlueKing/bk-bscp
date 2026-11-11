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

package pbtr

import (
	"testing"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
)

func TestPbTemplateRevisionSpecIsDraft(t *testing.T) {
	tests := []struct {
		name    string
		isDraft bool
	}{
		{name: "formal revision", isDraft: false},
		{name: "draft revision", isDraft: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &table.TemplateRevisionSpec{
				RevisionName: "v1",
				Name:         "a.conf",
				Path:         "/etc/",
				ContentSpec:  &table.ContentSpec{Signature: "sign"},
				IsDraft:      tt.isDraft,
			}

			pbSpec := PbTemplateRevisionSpec(spec, "tmpl")
			if pbSpec.GetIsDraft() != tt.isDraft {
				t.Fatalf("expect pb is_draft %v, got %v", tt.isDraft, pbSpec.GetIsDraft())
			}

			if got := pbSpec.TemplateRevisionSpec().IsDraft; got != tt.isDraft {
				t.Fatalf("expect table is_draft %v, got %v", tt.isDraft, got)
			}
		})
	}
}
