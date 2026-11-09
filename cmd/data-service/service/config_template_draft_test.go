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
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	pbds "github.com/TencentBlueKing/bk-bscp/pkg/protocol/data-service"
)

func TestResolveDraftSaveAction(t *testing.T) {
	tests := []struct {
		name        string
		hasDraft    bool
		saveAsDraft bool
		want        draftSaveAction
	}{
		{
			name:        "不存在草稿版本时直接保存，新增正式版本记录",
			hasDraft:    false,
			saveAsDraft: false,
			want:        draftSaveActionCreateFormal,
		},
		{
			name:        "不存在草稿版本时暂存草稿，新增草稿版本记录",
			hasDraft:    false,
			saveAsDraft: true,
			want:        draftSaveActionCreateDraft,
		},
		{
			name:        "存在草稿版本时保存，草稿版本原地转正",
			hasDraft:    true,
			saveAsDraft: false,
			want:        draftSaveActionPromote,
		},
		{
			name:        "存在草稿版本时暂存草稿，原地更新当前草稿",
			hasDraft:    true,
			saveAsDraft: true,
			want:        draftSaveActionUpdateDraft,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveDraftSaveAction(tt.hasDraft, tt.saveAsDraft); got != tt.want {
				t.Fatalf("got action %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewConfigTemplateRevisionSpec(t *testing.T) {
	base := &table.TemplateRevision{
		Spec: &table.TemplateRevisionSpec{
			RevisionName: "v20260101000000",
			RevisionMemo: "draft memo",
			FileType:     table.Text,
		},
	}
	req := &pbds.UpdateConfigTemplateReq{
		RevisionMemo: "new memo",
		User:         "nobody",
		UserGroup:    "nobody",
		Privilege:    "644",
		Sign:         "sign",
		ByteSize:     10,
		Md5:          "md5",
		Charset:      "utf-8",
		FileMode:     "unix",
	}

	t.Run("草稿版本转正沿用原有版本号", func(t *testing.T) {
		got := newConfigTemplateRevisionSpec(req, base, "/etc", "app.conf", draftSaveActionPromote)
		if got.RevisionName != base.Spec.RevisionName {
			t.Fatalf("got revision name %s, want %s", got.RevisionName, base.Spec.RevisionName)
		}
		if got.IsDraft {
			t.Fatal("expect formal revision after promote")
		}
		if got.Path != "/etc" || got.Name != "app.conf" {
			t.Fatalf("got path %s/%s, want /etc/app.conf", got.Path, got.Name)
		}
		if got.RevisionMemo != "new memo" || got.FileType != table.Text {
			t.Fatalf("got memo %s file type %s, want new memo and %s", got.RevisionMemo, got.FileType, table.Text)
		}
		if got.Permission.User != "nobody" || got.ContentSpec.Signature != "sign" {
			t.Fatalf("got permission %+v content %+v, want request values", got.Permission, got.ContentSpec)
		}
	})

	t.Run("新增正式版本时生成新的版本号", func(t *testing.T) {
		got := newConfigTemplateRevisionSpec(req, base, "/etc", "app.conf", draftSaveActionCreateFormal)
		if got.RevisionName == "" || got.RevisionName == base.Spec.RevisionName {
			t.Fatalf("got revision name %s, want a generated revision name", got.RevisionName)
		}
		if got.IsDraft {
			t.Fatal("expect formal revision")
		}
	})

	t.Run("暂存草稿时生成新的版本号并标记草稿", func(t *testing.T) {
		draftReq := proto.Clone(req).(*pbds.UpdateConfigTemplateReq)
		draftReq.IsDraft = true
		got := newConfigTemplateRevisionSpec(draftReq, base, "/etc", "app.conf", draftSaveActionCreateDraft)
		if got.RevisionName == "" || got.RevisionName == base.Spec.RevisionName {
			t.Fatalf("got revision name %s, want a generated revision name", got.RevisionName)
		}
		if !got.IsDraft {
			t.Fatal("expect draft revision")
		}
	})

	t.Run("已有草稿暂存草稿沿用原有版本号并保持草稿状态", func(t *testing.T) {
		draftReq := proto.Clone(req).(*pbds.UpdateConfigTemplateReq)
		draftReq.IsDraft = true
		got := newConfigTemplateRevisionSpec(draftReq, base, "/etc", "app.conf", draftSaveActionUpdateDraft)
		if got.RevisionName != base.Spec.RevisionName {
			t.Fatalf("got revision name %s, want %s", got.RevisionName, base.Spec.RevisionName)
		}
		if !got.IsDraft {
			t.Fatal("expect draft revision")
		}
	})

	t.Run("显式指定版本号时以请求为准", func(t *testing.T) {
		nameReq := proto.Clone(req).(*pbds.UpdateConfigTemplateReq)
		nameReq.RevisionName = "v20260202000000"
		got := newConfigTemplateRevisionSpec(nameReq, base, "/etc", "app.conf", draftSaveActionPromote)
		if got.RevisionName != "v20260202000000" {
			t.Fatalf("got revision name %s, want v20260202000000", got.RevisionName)
		}
	})
}
