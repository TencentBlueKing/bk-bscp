// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.31.0
// 	protoc        v4.25.1
// source: template_variable.proto

package pbtv

import (
	base "github.com/TencentBlueKing/bk-bscp/pkg/protocol/core/base"
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2/options"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// TemplateVariable source resource reference: pkg/dal/table/template_variable.go
type TemplateVariable struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id         uint32                      `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	Spec       *TemplateVariableSpec       `protobuf:"bytes,2,opt,name=spec,proto3" json:"spec,omitempty"`
	Attachment *TemplateVariableAttachment `protobuf:"bytes,3,opt,name=attachment,proto3" json:"attachment,omitempty"`
	Revision   *base.Revision              `protobuf:"bytes,4,opt,name=revision,proto3" json:"revision,omitempty"`
}

func (x *TemplateVariable) Reset() {
	*x = TemplateVariable{}
	if protoimpl.UnsafeEnabled {
		mi := &file_template_variable_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TemplateVariable) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TemplateVariable) ProtoMessage() {}

func (x *TemplateVariable) ProtoReflect() protoreflect.Message {
	mi := &file_template_variable_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TemplateVariable.ProtoReflect.Descriptor instead.
func (*TemplateVariable) Descriptor() ([]byte, []int) {
	return file_template_variable_proto_rawDescGZIP(), []int{0}
}

func (x *TemplateVariable) GetId() uint32 {
	if x != nil {
		return x.Id
	}
	return 0
}

func (x *TemplateVariable) GetSpec() *TemplateVariableSpec {
	if x != nil {
		return x.Spec
	}
	return nil
}

func (x *TemplateVariable) GetAttachment() *TemplateVariableAttachment {
	if x != nil {
		return x.Attachment
	}
	return nil
}

func (x *TemplateVariable) GetRevision() *base.Revision {
	if x != nil {
		return x.Revision
	}
	return nil
}

// TemplateVariableSpec source resource reference: pkg/dal/table/template_variable.go
type TemplateVariableSpec struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name       string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Type       string `protobuf:"bytes,2,opt,name=type,proto3" json:"type,omitempty"`
	DefaultVal string `protobuf:"bytes,3,opt,name=default_val,json=defaultVal,proto3" json:"default_val,omitempty"`
	Memo       string `protobuf:"bytes,4,opt,name=memo,proto3" json:"memo,omitempty"`
}

func (x *TemplateVariableSpec) Reset() {
	*x = TemplateVariableSpec{}
	if protoimpl.UnsafeEnabled {
		mi := &file_template_variable_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TemplateVariableSpec) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TemplateVariableSpec) ProtoMessage() {}

func (x *TemplateVariableSpec) ProtoReflect() protoreflect.Message {
	mi := &file_template_variable_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TemplateVariableSpec.ProtoReflect.Descriptor instead.
func (*TemplateVariableSpec) Descriptor() ([]byte, []int) {
	return file_template_variable_proto_rawDescGZIP(), []int{1}
}

func (x *TemplateVariableSpec) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *TemplateVariableSpec) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

func (x *TemplateVariableSpec) GetDefaultVal() string {
	if x != nil {
		return x.DefaultVal
	}
	return ""
}

func (x *TemplateVariableSpec) GetMemo() string {
	if x != nil {
		return x.Memo
	}
	return ""
}

// TemplateVariableAttachment source resource reference: pkg/dal/table/template_variable.go
type TemplateVariableAttachment struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BizId uint32 `protobuf:"varint,1,opt,name=biz_id,json=bizId,proto3" json:"biz_id,omitempty"`
}

func (x *TemplateVariableAttachment) Reset() {
	*x = TemplateVariableAttachment{}
	if protoimpl.UnsafeEnabled {
		mi := &file_template_variable_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TemplateVariableAttachment) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TemplateVariableAttachment) ProtoMessage() {}

func (x *TemplateVariableAttachment) ProtoReflect() protoreflect.Message {
	mi := &file_template_variable_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TemplateVariableAttachment.ProtoReflect.Descriptor instead.
func (*TemplateVariableAttachment) Descriptor() ([]byte, []int) {
	return file_template_variable_proto_rawDescGZIP(), []int{2}
}

func (x *TemplateVariableAttachment) GetBizId() uint32 {
	if x != nil {
		return x.BizId
	}
	return 0
}

var File_template_variable_proto protoreflect.FileDescriptor

var file_template_variable_proto_rawDesc = []byte{
	0x0a, 0x17, 0x74, 0x65, 0x6d, 0x70, 0x6c, 0x61, 0x74, 0x65, 0x5f, 0x76, 0x61, 0x72, 0x69, 0x61,
	0x62, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x04, 0x70, 0x62, 0x74, 0x76, 0x1a,
	0x21, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2f, 0x63, 0x6f,
	0x72, 0x65, 0x2f, 0x62, 0x61, 0x73, 0x65, 0x2f, 0x62, 0x61, 0x73, 0x65, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x1a, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x2d, 0x67, 0x65, 0x6e, 0x2d, 0x6f,
	0x70, 0x65, 0x6e, 0x61, 0x70, 0x69, 0x76, 0x32, 0x2f, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73,
	0x2f, 0x61, 0x6e, 0x6e, 0x6f, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x22, 0xd1, 0x01, 0x0a, 0x10, 0x54, 0x65, 0x6d, 0x70, 0x6c, 0x61, 0x74, 0x65, 0x56,
	0x61, 0x72, 0x69, 0x61, 0x62, 0x6c, 0x65, 0x12, 0x1d, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0d, 0x42, 0x0d, 0x92, 0x41, 0x0a, 0x32, 0x08, 0xe5, 0x8f, 0x98, 0xe9, 0x87, 0x8f,
	0x49, 0x44, 0x52, 0x02, 0x69, 0x64, 0x12, 0x2e, 0x0a, 0x04, 0x73, 0x70, 0x65, 0x63, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x70, 0x62, 0x74, 0x76, 0x2e, 0x54, 0x65, 0x6d, 0x70,
	0x6c, 0x61, 0x74, 0x65, 0x56, 0x61, 0x72, 0x69, 0x61, 0x62, 0x6c, 0x65, 0x53, 0x70, 0x65, 0x63,
	0x52, 0x04, 0x73, 0x70, 0x65, 0x63, 0x12, 0x40, 0x0a, 0x0a, 0x61, 0x74, 0x74, 0x61, 0x63, 0x68,
	0x6d, 0x65, 0x6e, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x20, 0x2e, 0x70, 0x62, 0x74,
	0x76, 0x2e, 0x54, 0x65, 0x6d, 0x70, 0x6c, 0x61, 0x74, 0x65, 0x56, 0x61, 0x72, 0x69, 0x61, 0x62,
	0x6c, 0x65, 0x41, 0x74, 0x74, 0x61, 0x63, 0x68, 0x6d, 0x65, 0x6e, 0x74, 0x52, 0x0a, 0x61, 0x74,
	0x74, 0x61, 0x63, 0x68, 0x6d, 0x65, 0x6e, 0x74, 0x12, 0x2c, 0x0a, 0x08, 0x72, 0x65, 0x76, 0x69,
	0x73, 0x69, 0x6f, 0x6e, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x10, 0x2e, 0x70, 0x62, 0x62,
	0x61, 0x73, 0x65, 0x2e, 0x52, 0x65, 0x76, 0x69, 0x73, 0x69, 0x6f, 0x6e, 0x52, 0x08, 0x72, 0x65,
	0x76, 0x69, 0x73, 0x69, 0x6f, 0x6e, 0x22, 0xce, 0x01, 0x0a, 0x14, 0x54, 0x65, 0x6d, 0x70, 0x6c,
	0x61, 0x74, 0x65, 0x56, 0x61, 0x72, 0x69, 0x61, 0x62, 0x6c, 0x65, 0x53, 0x70, 0x65, 0x63, 0x12,
	0x25, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x42, 0x11, 0x92,
	0x41, 0x0e, 0x32, 0x0c, 0xe5, 0x8f, 0x98, 0xe9, 0x87, 0x8f, 0xe5, 0x90, 0x8d, 0xe7, 0xa7, 0xb0,
	0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x37, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x09, 0x42, 0x23, 0x92, 0x41, 0x20, 0x32, 0x1e, 0xe5, 0x8f, 0x98, 0xe9, 0x87,
	0x8f, 0xe7, 0xb1, 0xbb, 0xe5, 0x9e, 0x8b, 0xef, 0xbc, 0x9a, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67,
	0xe3, 0x80, 0x81, 0x6e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12,
	0x2f, 0x0a, 0x0b, 0x64, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74, 0x5f, 0x76, 0x61, 0x6c, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x09, 0x42, 0x0e, 0x92, 0x41, 0x0b, 0x32, 0x09, 0xe9, 0xbb, 0x98, 0xe8, 0xae,
	0xa4, 0xe5, 0x80, 0xbc, 0x52, 0x0a, 0x64, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74, 0x56, 0x61, 0x6c,
	0x12, 0x25, 0x0a, 0x04, 0x6d, 0x65, 0x6d, 0x6f, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x42, 0x11,
	0x92, 0x41, 0x0e, 0x32, 0x0c, 0xe5, 0x8f, 0x98, 0xe9, 0x87, 0x8f, 0xe6, 0x8f, 0x8f, 0xe8, 0xbf,
	0xb0, 0x52, 0x04, 0x6d, 0x65, 0x6d, 0x6f, 0x22, 0x42, 0x0a, 0x1a, 0x54, 0x65, 0x6d, 0x70, 0x6c,
	0x61, 0x74, 0x65, 0x56, 0x61, 0x72, 0x69, 0x61, 0x62, 0x6c, 0x65, 0x41, 0x74, 0x74, 0x61, 0x63,
	0x68, 0x6d, 0x65, 0x6e, 0x74, 0x12, 0x24, 0x0a, 0x06, 0x62, 0x69, 0x7a, 0x5f, 0x69, 0x64, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x0d, 0x42, 0x0d, 0x92, 0x41, 0x0a, 0x32, 0x08, 0xe4, 0xb8, 0x9a, 0xe5,
	0x8a, 0xa1, 0x49, 0x44, 0x52, 0x05, 0x62, 0x69, 0x7a, 0x49, 0x64, 0x42, 0x4d, 0x5a, 0x4b, 0x67,
	0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x54, 0x65, 0x6e, 0x63, 0x65, 0x6e,
	0x74, 0x42, 0x6c, 0x75, 0x65, 0x4b, 0x69, 0x6e, 0x67, 0x2f, 0x62, 0x6b, 0x2d, 0x62, 0x73, 0x63,
	0x70, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2f, 0x63,
	0x6f, 0x72, 0x65, 0x2f, 0x74, 0x65, 0x6d, 0x70, 0x6c, 0x61, 0x74, 0x65, 0x2d, 0x76, 0x61, 0x72,
	0x69, 0x61, 0x62, 0x6c, 0x65, 0x3b, 0x70, 0x62, 0x74, 0x76, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
}

var (
	file_template_variable_proto_rawDescOnce sync.Once
	file_template_variable_proto_rawDescData = file_template_variable_proto_rawDesc
)

func file_template_variable_proto_rawDescGZIP() []byte {
	file_template_variable_proto_rawDescOnce.Do(func() {
		file_template_variable_proto_rawDescData = protoimpl.X.CompressGZIP(file_template_variable_proto_rawDescData)
	})
	return file_template_variable_proto_rawDescData
}

var file_template_variable_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_template_variable_proto_goTypes = []interface{}{
	(*TemplateVariable)(nil),           // 0: pbtv.TemplateVariable
	(*TemplateVariableSpec)(nil),       // 1: pbtv.TemplateVariableSpec
	(*TemplateVariableAttachment)(nil), // 2: pbtv.TemplateVariableAttachment
	(*base.Revision)(nil),              // 3: pbbase.Revision
}
var file_template_variable_proto_depIdxs = []int32{
	1, // 0: pbtv.TemplateVariable.spec:type_name -> pbtv.TemplateVariableSpec
	2, // 1: pbtv.TemplateVariable.attachment:type_name -> pbtv.TemplateVariableAttachment
	3, // 2: pbtv.TemplateVariable.revision:type_name -> pbbase.Revision
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_template_variable_proto_init() }
func file_template_variable_proto_init() {
	if File_template_variable_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_template_variable_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TemplateVariable); i {
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
		file_template_variable_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TemplateVariableSpec); i {
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
		file_template_variable_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TemplateVariableAttachment); i {
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
			RawDescriptor: file_template_variable_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_template_variable_proto_goTypes,
		DependencyIndexes: file_template_variable_proto_depIdxs,
		MessageInfos:      file_template_variable_proto_msgTypes,
	}.Build()
	File_template_variable_proto = out.File
	file_template_variable_proto_rawDesc = nil
	file_template_variable_proto_goTypes = nil
	file_template_variable_proto_depIdxs = nil
}
