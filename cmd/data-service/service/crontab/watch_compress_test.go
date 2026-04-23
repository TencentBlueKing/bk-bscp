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

package crontab

import (
	"reflect"
	"testing"

	"github.com/TencentBlueKing/bk-bscp/internal/components/bkcmdb"
)

func intPtr(v int) *int          { return &v }
func strPtr(v string) *string    { return &v }

func relEvent(eventType string, bizID, hostID *int) bkcmdb.HostRelationEvent {
	var detail *bkcmdb.HostRelationDetail
	if bizID != nil || hostID != nil {
		detail = &bkcmdb.HostRelationDetail{BkBizID: bizID, BkHostID: hostID}
	}
	return bkcmdb.HostRelationEvent{
		BkEventType: eventType,
		BkDetail:    detail,
	}
}

func TestCompressEvents(t *testing.T) {
	cases := []struct {
		name   string
		events []bkcmdb.HostRelationEvent
		want   []compressedIntent
	}{
		{
			name:   "empty input",
			events: nil,
			want:   []compressedIntent{},
		},
		{
			name: "single create",
			events: []bkcmdb.HostRelationEvent{
				relEvent(bizHostRelationCreateEvent, intPtr(1), intPtr(10)),
			},
			want: []compressedIntent{
				{bizID: 1, hostID: 10, finalOp: bizHostRelationCreateEvent},
			},
		},
		{
			name: "single delete",
			events: []bkcmdb.HostRelationEvent{
				relEvent(bizHostRelationDeleteEvent, intPtr(1), intPtr(10)),
			},
			want: []compressedIntent{
				{bizID: 1, hostID: 10, finalOp: bizHostRelationDeleteEvent},
			},
		},
		{
			name: "create then delete collapses to delete",
			events: []bkcmdb.HostRelationEvent{
				relEvent(bizHostRelationCreateEvent, intPtr(1), intPtr(10)),
				relEvent(bizHostRelationDeleteEvent, intPtr(1), intPtr(10)),
			},
			want: []compressedIntent{
				{bizID: 1, hostID: 10, finalOp: bizHostRelationDeleteEvent},
			},
		},
		{
			name: "delete then create collapses to create",
			events: []bkcmdb.HostRelationEvent{
				relEvent(bizHostRelationDeleteEvent, intPtr(1), intPtr(10)),
				relEvent(bizHostRelationCreateEvent, intPtr(1), intPtr(10)),
			},
			want: []compressedIntent{
				{bizID: 1, hostID: 10, finalOp: bizHostRelationCreateEvent},
			},
		},
		{
			name: "multiple bizs and hosts preserve first-seen order",
			events: []bkcmdb.HostRelationEvent{
				relEvent(bizHostRelationCreateEvent, intPtr(1), intPtr(10)),
				relEvent(bizHostRelationCreateEvent, intPtr(2), intPtr(20)),
				relEvent(bizHostRelationDeleteEvent, intPtr(1), intPtr(10)),
				relEvent(bizHostRelationCreateEvent, intPtr(1), intPtr(11)),
			},
			want: []compressedIntent{
				{bizID: 1, hostID: 10, finalOp: bizHostRelationDeleteEvent},
				{bizID: 2, hostID: 20, finalOp: bizHostRelationCreateEvent},
				{bizID: 1, hostID: 11, finalOp: bizHostRelationCreateEvent},
			},
		},
		{
			name: "nil detail is dropped",
			events: []bkcmdb.HostRelationEvent{
				{BkEventType: bizHostRelationCreateEvent, BkDetail: nil},
				relEvent(bizHostRelationCreateEvent, intPtr(1), intPtr(10)),
			},
			want: []compressedIntent{
				{bizID: 1, hostID: 10, finalOp: bizHostRelationCreateEvent},
			},
		},
		{
			name: "missing bizID or hostID is dropped",
			events: []bkcmdb.HostRelationEvent{
				relEvent(bizHostRelationCreateEvent, nil, intPtr(10)),
				relEvent(bizHostRelationCreateEvent, intPtr(1), nil),
				relEvent(bizHostRelationDeleteEvent, intPtr(1), intPtr(10)),
			},
			want: []compressedIntent{
				{bizID: 1, hostID: 10, finalOp: bizHostRelationDeleteEvent},
			},
		},
		{
			name: "unknown event type is dropped",
			events: []bkcmdb.HostRelationEvent{
				relEvent("update", intPtr(1), intPtr(10)),
				relEvent(bizHostRelationCreateEvent, intPtr(1), intPtr(11)),
			},
			want: []compressedIntent{
				{bizID: 1, hostID: 11, finalOp: bizHostRelationCreateEvent},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := compressEvents(tc.events)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("compressEvents mismatch\n got: %+v\nwant: %+v", got, tc.want)
			}
		})
	}
}

func hostEvent(eventType string, hostID *int, agentID *string) bkcmdb.HostEvent {
	var detail *bkcmdb.HostDetail
	if hostID != nil || agentID != nil {
		detail = &bkcmdb.HostDetail{BkHostID: hostID, BkAgentID: agentID}
	}
	return bkcmdb.HostEvent{
		BkEventType: eventType,
		BkDetail:    detail,
	}
}

func TestCompressHostUpdateEvents(t *testing.T) {
	cases := []struct {
		name   string
		events []bkcmdb.HostEvent
		want   map[int]string
	}{
		{
			name:   "empty input",
			events: nil,
			want:   map[int]string{},
		},
		{
			name: "single update",
			events: []bkcmdb.HostEvent{
				hostEvent(hostUpdateEvent, intPtr(10), strPtr("a1")),
			},
			want: map[int]string{10: "a1"},
		},
		{
			name: "multiple updates for same host keep last agent",
			events: []bkcmdb.HostEvent{
				hostEvent(hostUpdateEvent, intPtr(10), strPtr("a1")),
				hostEvent(hostUpdateEvent, intPtr(10), strPtr("a2")),
				hostEvent(hostUpdateEvent, intPtr(10), strPtr("")),
			},
			want: map[int]string{10: ""},
		},
		{
			name: "nil agent id becomes empty string",
			events: []bkcmdb.HostEvent{
				hostEvent(hostUpdateEvent, intPtr(10), nil),
			},
			want: map[int]string{10: ""},
		},
		{
			name: "nil detail and unknown type dropped",
			events: []bkcmdb.HostEvent{
				{BkEventType: hostUpdateEvent, BkDetail: nil},
				hostEvent("create", intPtr(10), strPtr("a1")),
				hostEvent(hostUpdateEvent, intPtr(11), strPtr("b1")),
			},
			want: map[int]string{11: "b1"},
		},
		{
			name: "multiple hosts independent",
			events: []bkcmdb.HostEvent{
				hostEvent(hostUpdateEvent, intPtr(10), strPtr("a1")),
				hostEvent(hostUpdateEvent, intPtr(11), strPtr("b1")),
				hostEvent(hostUpdateEvent, intPtr(10), strPtr("a2")),
			},
			want: map[int]string{10: "a2", 11: "b1"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := compressHostUpdateEvents(tc.events)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("compressHostUpdateEvents mismatch\n got: %+v\nwant: %+v", got, tc.want)
			}
		})
	}
}
