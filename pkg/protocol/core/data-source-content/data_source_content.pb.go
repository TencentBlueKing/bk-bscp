// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.31.0
// 	protoc        v4.25.1
// source: data_source_content.proto

package pbdsc

import (
	base "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/base"
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2/options"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	structpb "google.golang.org/protobuf/types/known/structpb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// DataSourceContent mapped from table <data_source_contents>
type DataSourceContent struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id         uint32                       `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	Spec       *DataSourceContentSpec       `protobuf:"bytes,2,opt,name=spec,proto3" json:"spec,omitempty"`
	Attachment *DataSourceContentAttachment `protobuf:"bytes,3,opt,name=attachment,proto3" json:"attachment,omitempty"`
	Revision   *base.Revision               `protobuf:"bytes,4,opt,name=revision,proto3" json:"revision,omitempty"`
}

func (x *DataSourceContent) Reset() {
	*x = DataSourceContent{}
	if protoimpl.UnsafeEnabled {
		mi := &file_data_source_content_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DataSourceContent) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DataSourceContent) ProtoMessage() {}

func (x *DataSourceContent) ProtoReflect() protoreflect.Message {
	mi := &file_data_source_content_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DataSourceContent.ProtoReflect.Descriptor instead.
func (*DataSourceContent) Descriptor() ([]byte, []int) {
	return file_data_source_content_proto_rawDescGZIP(), []int{0}
}

func (x *DataSourceContent) GetId() uint32 {
	if x != nil {
		return x.Id
	}
	return 0
}

func (x *DataSourceContent) GetSpec() *DataSourceContentSpec {
	if x != nil {
		return x.Spec
	}
	return nil
}

func (x *DataSourceContent) GetAttachment() *DataSourceContentAttachment {
	if x != nil {
		return x.Attachment
	}
	return nil
}

func (x *DataSourceContent) GetRevision() *base.Revision {
	if x != nil {
		return x.Revision
	}
	return nil
}

// DataSourceContentSpec mapped from table <data_source_contents>
type DataSourceContentSpec struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Content *structpb.Struct `protobuf:"bytes,1,opt,name=content,proto3" json:"content,omitempty"`
	Status  string           `protobuf:"bytes,2,opt,name=status,proto3" json:"status,omitempty"`
}

func (x *DataSourceContentSpec) Reset() {
	*x = DataSourceContentSpec{}
	if protoimpl.UnsafeEnabled {
		mi := &file_data_source_content_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DataSourceContentSpec) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DataSourceContentSpec) ProtoMessage() {}

func (x *DataSourceContentSpec) ProtoReflect() protoreflect.Message {
	mi := &file_data_source_content_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DataSourceContentSpec.ProtoReflect.Descriptor instead.
func (*DataSourceContentSpec) Descriptor() ([]byte, []int) {
	return file_data_source_content_proto_rawDescGZIP(), []int{1}
}

func (x *DataSourceContentSpec) GetContent() *structpb.Struct {
	if x != nil {
		return x.Content
	}
	return nil
}

func (x *DataSourceContentSpec) GetStatus() string {
	if x != nil {
		return x.Status
	}
	return ""
}

// DataSourceContentAttachment mapped from table <data_source_contents>
type DataSourceContentAttachment struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	DataSourceMappingId uint32 `protobuf:"varint,2,opt,name=data_source_mapping_id,json=dataSourceMappingId,proto3" json:"data_source_mapping_id,omitempty"`
}

func (x *DataSourceContentAttachment) Reset() {
	*x = DataSourceContentAttachment{}
	if protoimpl.UnsafeEnabled {
		mi := &file_data_source_content_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DataSourceContentAttachment) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DataSourceContentAttachment) ProtoMessage() {}

func (x *DataSourceContentAttachment) ProtoReflect() protoreflect.Message {
	mi := &file_data_source_content_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DataSourceContentAttachment.ProtoReflect.Descriptor instead.
func (*DataSourceContentAttachment) Descriptor() ([]byte, []int) {
	return file_data_source_content_proto_rawDescGZIP(), []int{2}
}

func (x *DataSourceContentAttachment) GetDataSourceMappingId() uint32 {
	if x != nil {
		return x.DataSourceMappingId
	}
	return 0
}

// Field 表结构
type Field struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name       string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Alias      string `protobuf:"bytes,2,opt,name=alias,proto3" json:"alias,omitempty"`
	ColumnType string `protobuf:"bytes,3,opt,name=column_type,json=columnType,proto3" json:"column_type,omitempty"`
	Primary    bool   `protobuf:"varint,4,opt,name=primary,proto3" json:"primary,omitempty"`
	EnumValue  string `protobuf:"bytes,5,opt,name=enum_value,json=enumValue,proto3" json:"enum_value,omitempty"`
	Selected   bool   `protobuf:"varint,6,opt,name=selected,proto3" json:"selected,omitempty"`
}

func (x *Field) Reset() {
	*x = Field{}
	if protoimpl.UnsafeEnabled {
		mi := &file_data_source_content_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Field) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Field) ProtoMessage() {}

func (x *Field) ProtoReflect() protoreflect.Message {
	mi := &file_data_source_content_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Field.ProtoReflect.Descriptor instead.
func (*Field) Descriptor() ([]byte, []int) {
	return file_data_source_content_proto_rawDescGZIP(), []int{3}
}

func (x *Field) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Field) GetAlias() string {
	if x != nil {
		return x.Alias
	}
	return ""
}

func (x *Field) GetColumnType() string {
	if x != nil {
		return x.ColumnType
	}
	return ""
}

func (x *Field) GetPrimary() bool {
	if x != nil {
		return x.Primary
	}
	return false
}

func (x *Field) GetEnumValue() string {
	if x != nil {
		return x.EnumValue
	}
	return ""
}

func (x *Field) GetSelected() bool {
	if x != nil {
		return x.Selected
	}
	return false
}

var File_data_source_content_proto protoreflect.FileDescriptor

var file_data_source_content_proto_rawDesc = []byte{
	0x0a, 0x19, 0x64, 0x61, 0x74, 0x61, 0x5f, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x5f, 0x63, 0x6f,
	0x6e, 0x74, 0x65, 0x6e, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x05, 0x70, 0x62, 0x64,
	0x73, 0x63, 0x1a, 0x21, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c,
	0x2f, 0x63, 0x6f, 0x72, 0x65, 0x2f, 0x62, 0x61, 0x73, 0x65, 0x2f, 0x62, 0x61, 0x73, 0x65, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1c, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x73, 0x74, 0x72, 0x75, 0x63, 0x74, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x1a, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x2d, 0x67, 0x65, 0x6e, 0x2d,
	0x6f, 0x70, 0x65, 0x6e, 0x61, 0x70, 0x69, 0x76, 0x32, 0x2f, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e,
	0x73, 0x2f, 0x61, 0x6e, 0x6e, 0x6f, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x22, 0xdc, 0x01, 0x0a, 0x11, 0x44, 0x61, 0x74, 0x61, 0x53, 0x6f, 0x75, 0x72,
	0x63, 0x65, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x12, 0x23, 0x0a, 0x02, 0x69, 0x64, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x0d, 0x42, 0x13, 0x92, 0x41, 0x10, 0x32, 0x0e, 0xe8, 0xa1, 0xa8, 0xe6,
	0xa0, 0xbc, 0xe6, 0x95, 0xb0, 0xe6, 0x8d, 0xae, 0x49, 0x44, 0x52, 0x02, 0x69, 0x64, 0x12, 0x30,
	0x0a, 0x04, 0x73, 0x70, 0x65, 0x63, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1c, 0x2e, 0x70,
	0x62, 0x64, 0x73, 0x63, 0x2e, 0x44, 0x61, 0x74, 0x61, 0x53, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x43,
	0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x53, 0x70, 0x65, 0x63, 0x52, 0x04, 0x73, 0x70, 0x65, 0x63,
	0x12, 0x42, 0x0a, 0x0a, 0x61, 0x74, 0x74, 0x61, 0x63, 0x68, 0x6d, 0x65, 0x6e, 0x74, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x22, 0x2e, 0x70, 0x62, 0x64, 0x73, 0x63, 0x2e, 0x44, 0x61, 0x74,
	0x61, 0x53, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x41, 0x74,
	0x74, 0x61, 0x63, 0x68, 0x6d, 0x65, 0x6e, 0x74, 0x52, 0x0a, 0x61, 0x74, 0x74, 0x61, 0x63, 0x68,
	0x6d, 0x65, 0x6e, 0x74, 0x12, 0x2c, 0x0a, 0x08, 0x72, 0x65, 0x76, 0x69, 0x73, 0x69, 0x6f, 0x6e,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x10, 0x2e, 0x70, 0x62, 0x62, 0x61, 0x73, 0x65, 0x2e,
	0x52, 0x65, 0x76, 0x69, 0x73, 0x69, 0x6f, 0x6e, 0x52, 0x08, 0x72, 0x65, 0x76, 0x69, 0x73, 0x69,
	0x6f, 0x6e, 0x22, 0xa1, 0x01, 0x0a, 0x15, 0x44, 0x61, 0x74, 0x61, 0x53, 0x6f, 0x75, 0x72, 0x63,
	0x65, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x53, 0x70, 0x65, 0x63, 0x12, 0x3e, 0x0a, 0x07,
	0x63, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x17, 0x2e,
	0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e,
	0x53, 0x74, 0x72, 0x75, 0x63, 0x74, 0x42, 0x0b, 0x92, 0x41, 0x08, 0x32, 0x06, 0xe5, 0x86, 0x85,
	0xe5, 0xae, 0xb9, 0x52, 0x07, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x12, 0x48, 0x0a, 0x06,
	0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x42, 0x30, 0x92, 0x41,
	0x2d, 0x32, 0x2b, 0xe7, 0x8a, 0xb6, 0xe6, 0x80, 0x81, 0xef, 0xbc, 0x9a, 0x28, 0x41, 0x44, 0x44,
	0xe3, 0x80, 0x81, 0x44, 0x45, 0x4c, 0x45, 0x54, 0x45, 0xe3, 0x80, 0x81, 0x52, 0x45, 0x56, 0x49,
	0x53, 0x45, 0xe3, 0x80, 0x81, 0x55, 0x4e, 0x43, 0x48, 0x41, 0x4e, 0x47, 0x45, 0x29, 0x52, 0x06,
	0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x22, 0x64, 0x0a, 0x1b, 0x44, 0x61, 0x74, 0x61, 0x53, 0x6f,
	0x75, 0x72, 0x63, 0x65, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x41, 0x74, 0x74, 0x61, 0x63,
	0x68, 0x6d, 0x65, 0x6e, 0x74, 0x12, 0x45, 0x0a, 0x16, 0x64, 0x61, 0x74, 0x61, 0x5f, 0x73, 0x6f,
	0x75, 0x72, 0x63, 0x65, 0x5f, 0x6d, 0x61, 0x70, 0x70, 0x69, 0x6e, 0x67, 0x5f, 0x69, 0x64, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x0d, 0x42, 0x10, 0x92, 0x41, 0x0d, 0x32, 0x0b, 0xe8, 0xa1, 0xa8, 0xe7,
	0xbb, 0x93, 0xe6, 0x9e, 0x84, 0x49, 0x44, 0x52, 0x13, 0x64, 0x61, 0x74, 0x61, 0x53, 0x6f, 0x75,
	0x72, 0x63, 0x65, 0x4d, 0x61, 0x70, 0x70, 0x69, 0x6e, 0x67, 0x49, 0x64, 0x22, 0xb8, 0x02, 0x0a,
	0x05, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x12, 0x25, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x42, 0x11, 0x92, 0x41, 0x0e, 0x32, 0x0c, 0xe5, 0xad, 0x97, 0xe6, 0xae,
	0xb5, 0xe5, 0x90, 0x8d, 0xe7, 0xa7, 0xb0, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x27, 0x0a,
	0x05, 0x61, 0x6c, 0x69, 0x61, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x42, 0x11, 0x92, 0x41,
	0x0e, 0x32, 0x0c, 0xe5, 0xad, 0x97, 0xe6, 0xae, 0xb5, 0xe5, 0x88, 0xab, 0xe5, 0x90, 0x8d, 0x52,
	0x05, 0x61, 0x6c, 0x69, 0x61, 0x73, 0x12, 0x32, 0x0a, 0x0b, 0x63, 0x6f, 0x6c, 0x75, 0x6d, 0x6e,
	0x5f, 0x74, 0x79, 0x70, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x42, 0x11, 0x92, 0x41, 0x0e,
	0x32, 0x0c, 0xe5, 0xad, 0x97, 0xe6, 0xae, 0xb5, 0xe7, 0xb1, 0xbb, 0xe5, 0x9e, 0x8b, 0x52, 0x0a,
	0x63, 0x6f, 0x6c, 0x75, 0x6d, 0x6e, 0x54, 0x79, 0x70, 0x65, 0x12, 0x2e, 0x0a, 0x07, 0x70, 0x72,
	0x69, 0x6d, 0x61, 0x72, 0x79, 0x18, 0x04, 0x20, 0x01, 0x28, 0x08, 0x42, 0x14, 0x92, 0x41, 0x11,
	0x32, 0x0f, 0xe6, 0x98, 0xaf, 0xe5, 0x90, 0xa6, 0xe4, 0xb8, 0xba, 0xe4, 0xb8, 0xbb, 0xe9, 0x94,
	0xae, 0x52, 0x07, 0x70, 0x72, 0x69, 0x6d, 0x61, 0x72, 0x79, 0x12, 0x2d, 0x0a, 0x0a, 0x65, 0x6e,
	0x75, 0x6d, 0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x42, 0x0e,
	0x92, 0x41, 0x0b, 0x32, 0x09, 0xe6, 0x9e, 0x9a, 0xe4, 0xb8, 0xbe, 0xe5, 0x80, 0xbc, 0x52, 0x09,
	0x65, 0x6e, 0x75, 0x6d, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x4c, 0x0a, 0x08, 0x73, 0x65, 0x6c,
	0x65, 0x63, 0x74, 0x65, 0x64, 0x18, 0x06, 0x20, 0x01, 0x28, 0x08, 0x42, 0x30, 0x92, 0x41, 0x2d,
	0x32, 0x2b, 0xe5, 0xa4, 0x9a, 0xe9, 0x80, 0x89, 0xef, 0xbc, 0x9a, 0xe6, 0x98, 0xaf, 0x3d, 0x74,
	0x72, 0x75, 0x65, 0xef, 0xbc, 0x8c, 0xe5, 0x90, 0xa6, 0x3d, 0x66, 0x61, 0x6c, 0x73, 0x65, 0xef,
	0xbc, 0x8c, 0xe9, 0xbb, 0x98, 0xe8, 0xae, 0xa4, 0x66, 0x61, 0x6c, 0x73, 0x65, 0x52, 0x08, 0x73,
	0x65, 0x6c, 0x65, 0x63, 0x74, 0x65, 0x64, 0x42, 0x50, 0x5a, 0x4e, 0x67, 0x69, 0x74, 0x68, 0x75,
	0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x54, 0x65, 0x6e, 0x63, 0x65, 0x6e, 0x74, 0x42, 0x6c, 0x75,
	0x65, 0x4b, 0x69, 0x6e, 0x67, 0x2f, 0x62, 0x6b, 0x2d, 0x62, 0x73, 0x63, 0x70, 0x2f, 0x70, 0x6b,
	0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2f, 0x63, 0x6f, 0x72, 0x65, 0x2f,
	0x64, 0x61, 0x74, 0x61, 0x2d, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x2d, 0x63, 0x6f, 0x6e, 0x74,
	0x65, 0x6e, 0x74, 0x3b, 0x70, 0x62, 0x64, 0x73, 0x63, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x33,
}

var (
	file_data_source_content_proto_rawDescOnce sync.Once
	file_data_source_content_proto_rawDescData = file_data_source_content_proto_rawDesc
)

func file_data_source_content_proto_rawDescGZIP() []byte {
	file_data_source_content_proto_rawDescOnce.Do(func() {
		file_data_source_content_proto_rawDescData = protoimpl.X.CompressGZIP(file_data_source_content_proto_rawDescData)
	})
	return file_data_source_content_proto_rawDescData
}

var file_data_source_content_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_data_source_content_proto_goTypes = []interface{}{
	(*DataSourceContent)(nil),           // 0: pbdsc.DataSourceContent
	(*DataSourceContentSpec)(nil),       // 1: pbdsc.DataSourceContentSpec
	(*DataSourceContentAttachment)(nil), // 2: pbdsc.DataSourceContentAttachment
	(*Field)(nil),                       // 3: pbdsc.Field
	(*base.Revision)(nil),               // 4: pbbase.Revision
	(*structpb.Struct)(nil),             // 5: google.protobuf.Struct
}
var file_data_source_content_proto_depIdxs = []int32{
	1, // 0: pbdsc.DataSourceContent.spec:type_name -> pbdsc.DataSourceContentSpec
	2, // 1: pbdsc.DataSourceContent.attachment:type_name -> pbdsc.DataSourceContentAttachment
	4, // 2: pbdsc.DataSourceContent.revision:type_name -> pbbase.Revision
	5, // 3: pbdsc.DataSourceContentSpec.content:type_name -> google.protobuf.Struct
	4, // [4:4] is the sub-list for method output_type
	4, // [4:4] is the sub-list for method input_type
	4, // [4:4] is the sub-list for extension type_name
	4, // [4:4] is the sub-list for extension extendee
	0, // [0:4] is the sub-list for field type_name
}

func init() { file_data_source_content_proto_init() }
func file_data_source_content_proto_init() {
	if File_data_source_content_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_data_source_content_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DataSourceContent); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_data_source_content_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DataSourceContentSpec); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_data_source_content_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DataSourceContentAttachment); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_data_source_content_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Field); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_data_source_content_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_data_source_content_proto_goTypes,
		DependencyIndexes: file_data_source_content_proto_depIdxs,
		MessageInfos:      file_data_source_content_proto_msgTypes,
	}.Build()
	File_data_source_content_proto = out.File
	file_data_source_content_proto_rawDesc = nil
	file_data_source_content_proto_goTypes = nil
	file_data_source_content_proto_depIdxs = nil
}