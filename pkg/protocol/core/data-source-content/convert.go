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

// Package xxx
package pbdsc

import (
	"encoding/json"

	"github.com/TencentBlueKing/bk-bscp/pkg/dal/table"
	pbbase "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/base"
	"google.golang.org/protobuf/types/known/structpb"
)

// DataSourceContentSpec convert pb DataSourceContentSpec to table DataSourceContentSpec
func (m *DataSourceContentSpec) DataSourceContentSpec() *table.DataSourceContentSpec {
	if m == nil {
		return nil
	}

	return &table.DataSourceContentSpec{
		Content: m.Content.AsMap(),
		Status:  m.Status,
	}
}

// DataSourceContentAttachment convert pb DataSourceContentAttachment to table DataSourceContentAttachment
func (m *DataSourceContentAttachment) DataSourceContentAttachment() *table.DataSourceContentAttachment {
	if m == nil {
		return nil
	}

	return &table.DataSourceContentAttachment{
		DataSourceMappingID: m.DataSourceMappingId,
	}
}

// PbDataSourceContentSpec convert table DataSourceContentSpec to pb DataSourceContentSpec
func PbDataSourceContentSpec(spec *table.DataSourceContentSpec) *DataSourceContentSpec {
	if spec == nil {
		return nil
	}

	jsonBytes, _ := json.Marshal(spec.Content)
	var strct *structpb.Struct
	_ = json.Unmarshal(jsonBytes, &strct)

	return &DataSourceContentSpec{
		Content: strct,
		Status:  spec.Status,
	}
}

// PbDataSourceContentAttachment convert table DataSourceContentAttachment to pb DataSourceContentAttachment
func PbDataSourceContentAttachment(attachment *table.DataSourceContentAttachment) *DataSourceContentAttachment {
	if attachment == nil {
		return nil
	}

	return &DataSourceContentAttachment{
		DataSourceMappingId: attachment.DataSourceMappingID,
	}
}

// PbDataSourceContent convert table DataSourceContent to pb DataSourceContent
func PbDataSourceContent(c *table.DataSourceContent) *DataSourceContent {
	if c == nil {
		return nil
	}

	return &DataSourceContent{
		Id:         c.ID,
		Spec:       PbDataSourceContentSpec(c.Spec),
		Attachment: PbDataSourceContentAttachment(c.Attachment),
		Revision:   pbbase.PbRevision(c.Revision),
	}
}

// PbDataSourceContents convert table DataSourceContent to pb DataSourceContent
func PbDataSourceContents(c []*table.DataSourceContent) []*DataSourceContent {
	if c == nil {
		return make([]*DataSourceContent, 0)
	}
	result := make([]*DataSourceContent, 0)
	for _, v := range c {
		result = append(result, PbDataSourceContent(v))
	}

	return result
}
