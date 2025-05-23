// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.35.2
// 	protoc        v4.25.1
// source: credential-scope.proto

package pbcrs

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

type CredentialScopeAttachment struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BizId        uint32 `protobuf:"varint,1,opt,name=biz_id,json=bizId,proto3" json:"biz_id,omitempty"`
	CredentialId uint32 `protobuf:"varint,2,opt,name=credential_id,json=credentialId,proto3" json:"credential_id,omitempty"`
}

func (x *CredentialScopeAttachment) Reset() {
	*x = CredentialScopeAttachment{}
	mi := &file_credential_scope_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CredentialScopeAttachment) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CredentialScopeAttachment) ProtoMessage() {}

func (x *CredentialScopeAttachment) ProtoReflect() protoreflect.Message {
	mi := &file_credential_scope_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CredentialScopeAttachment.ProtoReflect.Descriptor instead.
func (*CredentialScopeAttachment) Descriptor() ([]byte, []int) {
	return file_credential_scope_proto_rawDescGZIP(), []int{0}
}

func (x *CredentialScopeAttachment) GetBizId() uint32 {
	if x != nil {
		return x.BizId
	}
	return 0
}

func (x *CredentialScopeAttachment) GetCredentialId() uint32 {
	if x != nil {
		return x.CredentialId
	}
	return 0
}

type CredentialScopeList struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id         uint32                     `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	Spec       *CredentialScopeSpec       `protobuf:"bytes,2,opt,name=spec,proto3" json:"spec,omitempty"`
	Attachment *CredentialScopeAttachment `protobuf:"bytes,3,opt,name=attachment,proto3" json:"attachment,omitempty"`
	Revision   *base.Revision             `protobuf:"bytes,4,opt,name=revision,proto3" json:"revision,omitempty"`
}

func (x *CredentialScopeList) Reset() {
	*x = CredentialScopeList{}
	mi := &file_credential_scope_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CredentialScopeList) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CredentialScopeList) ProtoMessage() {}

func (x *CredentialScopeList) ProtoReflect() protoreflect.Message {
	mi := &file_credential_scope_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CredentialScopeList.ProtoReflect.Descriptor instead.
func (*CredentialScopeList) Descriptor() ([]byte, []int) {
	return file_credential_scope_proto_rawDescGZIP(), []int{1}
}

func (x *CredentialScopeList) GetId() uint32 {
	if x != nil {
		return x.Id
	}
	return 0
}

func (x *CredentialScopeList) GetSpec() *CredentialScopeSpec {
	if x != nil {
		return x.Spec
	}
	return nil
}

func (x *CredentialScopeList) GetAttachment() *CredentialScopeAttachment {
	if x != nil {
		return x.Attachment
	}
	return nil
}

func (x *CredentialScopeList) GetRevision() *base.Revision {
	if x != nil {
		return x.Revision
	}
	return nil
}

type CredentialScopeSpec struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	App   string `protobuf:"bytes,1,opt,name=app,proto3" json:"app,omitempty"`
	Scope string `protobuf:"bytes,2,opt,name=scope,proto3" json:"scope,omitempty"`
}

func (x *CredentialScopeSpec) Reset() {
	*x = CredentialScopeSpec{}
	mi := &file_credential_scope_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CredentialScopeSpec) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CredentialScopeSpec) ProtoMessage() {}

func (x *CredentialScopeSpec) ProtoReflect() protoreflect.Message {
	mi := &file_credential_scope_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CredentialScopeSpec.ProtoReflect.Descriptor instead.
func (*CredentialScopeSpec) Descriptor() ([]byte, []int) {
	return file_credential_scope_proto_rawDescGZIP(), []int{2}
}

func (x *CredentialScopeSpec) GetApp() string {
	if x != nil {
		return x.App
	}
	return ""
}

func (x *CredentialScopeSpec) GetScope() string {
	if x != nil {
		return x.Scope
	}
	return ""
}

type UpdateScopeSpec struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id    uint32 `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	App   string `protobuf:"bytes,2,opt,name=app,proto3" json:"app,omitempty"`
	Scope string `protobuf:"bytes,3,opt,name=scope,proto3" json:"scope,omitempty"`
}

func (x *UpdateScopeSpec) Reset() {
	*x = UpdateScopeSpec{}
	mi := &file_credential_scope_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *UpdateScopeSpec) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateScopeSpec) ProtoMessage() {}

func (x *UpdateScopeSpec) ProtoReflect() protoreflect.Message {
	mi := &file_credential_scope_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateScopeSpec.ProtoReflect.Descriptor instead.
func (*UpdateScopeSpec) Descriptor() ([]byte, []int) {
	return file_credential_scope_proto_rawDescGZIP(), []int{3}
}

func (x *UpdateScopeSpec) GetId() uint32 {
	if x != nil {
		return x.Id
	}
	return 0
}

func (x *UpdateScopeSpec) GetApp() string {
	if x != nil {
		return x.App
	}
	return ""
}

func (x *UpdateScopeSpec) GetScope() string {
	if x != nil {
		return x.Scope
	}
	return ""
}

var File_credential_scope_proto protoreflect.FileDescriptor

var file_credential_scope_proto_rawDesc = []byte{
	0x0a, 0x16, 0x63, 0x72, 0x65, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x61, 0x6c, 0x2d, 0x73, 0x63, 0x6f,
	0x70, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x05, 0x70, 0x62, 0x63, 0x72, 0x73, 0x1a,
	0x21, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2f, 0x63, 0x6f,
	0x72, 0x65, 0x2f, 0x62, 0x61, 0x73, 0x65, 0x2f, 0x62, 0x61, 0x73, 0x65, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x1a, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x2d, 0x67, 0x65, 0x6e, 0x2d, 0x6f,
	0x70, 0x65, 0x6e, 0x61, 0x70, 0x69, 0x76, 0x32, 0x2f, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73,
	0x2f, 0x61, 0x6e, 0x6e, 0x6f, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x22, 0x7e, 0x0a, 0x19, 0x43, 0x72, 0x65, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x61, 0x6c,
	0x53, 0x63, 0x6f, 0x70, 0x65, 0x41, 0x74, 0x74, 0x61, 0x63, 0x68, 0x6d, 0x65, 0x6e, 0x74, 0x12,
	0x24, 0x0a, 0x06, 0x62, 0x69, 0x7a, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0d, 0x42,
	0x0d, 0x92, 0x41, 0x0a, 0x32, 0x08, 0xe4, 0xb8, 0x9a, 0xe5, 0x8a, 0xa1, 0x49, 0x44, 0x52, 0x05,
	0x62, 0x69, 0x7a, 0x49, 0x64, 0x12, 0x3b, 0x0a, 0x0d, 0x63, 0x72, 0x65, 0x64, 0x65, 0x6e, 0x74,
	0x69, 0x61, 0x6c, 0x5f, 0x69, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x42, 0x16, 0x92, 0x41,
	0x13, 0x32, 0x11, 0xe5, 0xae, 0xa2, 0xe6, 0x88, 0xb7, 0xe7, 0xab, 0xaf, 0xe5, 0xaf, 0x86, 0xe9,
	0x92, 0xa5, 0x49, 0x44, 0x52, 0x0c, 0x63, 0x72, 0x65, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x61, 0x6c,
	0x49, 0x64, 0x22, 0xe9, 0x01, 0x0a, 0x13, 0x43, 0x72, 0x65, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x61,
	0x6c, 0x53, 0x63, 0x6f, 0x70, 0x65, 0x4c, 0x69, 0x73, 0x74, 0x12, 0x32, 0x0a, 0x02, 0x69, 0x64,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x0d, 0x42, 0x22, 0x92, 0x41, 0x1f, 0x32, 0x1d, 0xe5, 0xae, 0xa2,
	0xe6, 0x88, 0xb7, 0xe7, 0xab, 0xaf, 0xe5, 0xaf, 0x86, 0xe9, 0x92, 0xa5, 0xe5, 0x85, 0xb3, 0xe8,
	0x81, 0x94, 0xe6, 0x9c, 0x8d, 0xe5, 0x8a, 0xa1, 0x49, 0x44, 0x52, 0x02, 0x69, 0x64, 0x12, 0x2e,
	0x0a, 0x04, 0x73, 0x70, 0x65, 0x63, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x70,
	0x62, 0x63, 0x72, 0x73, 0x2e, 0x43, 0x72, 0x65, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x61, 0x6c, 0x53,
	0x63, 0x6f, 0x70, 0x65, 0x53, 0x70, 0x65, 0x63, 0x52, 0x04, 0x73, 0x70, 0x65, 0x63, 0x12, 0x40,
	0x0a, 0x0a, 0x61, 0x74, 0x74, 0x61, 0x63, 0x68, 0x6d, 0x65, 0x6e, 0x74, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x20, 0x2e, 0x70, 0x62, 0x63, 0x72, 0x73, 0x2e, 0x43, 0x72, 0x65, 0x64, 0x65,
	0x6e, 0x74, 0x69, 0x61, 0x6c, 0x53, 0x63, 0x6f, 0x70, 0x65, 0x41, 0x74, 0x74, 0x61, 0x63, 0x68,
	0x6d, 0x65, 0x6e, 0x74, 0x52, 0x0a, 0x61, 0x74, 0x74, 0x61, 0x63, 0x68, 0x6d, 0x65, 0x6e, 0x74,
	0x12, 0x2c, 0x0a, 0x08, 0x72, 0x65, 0x76, 0x69, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x04, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x10, 0x2e, 0x70, 0x62, 0x62, 0x61, 0x73, 0x65, 0x2e, 0x52, 0x65, 0x76, 0x69,
	0x73, 0x69, 0x6f, 0x6e, 0x52, 0x08, 0x72, 0x65, 0x76, 0x69, 0x73, 0x69, 0x6f, 0x6e, 0x22, 0x63,
	0x0a, 0x13, 0x43, 0x72, 0x65, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x61, 0x6c, 0x53, 0x63, 0x6f, 0x70,
	0x65, 0x53, 0x70, 0x65, 0x63, 0x12, 0x23, 0x0a, 0x03, 0x61, 0x70, 0x70, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x42, 0x11, 0x92, 0x41, 0x0e, 0x32, 0x0c, 0xe6, 0x9c, 0x8d, 0xe5, 0x8a, 0xa1, 0xe5,
	0x90, 0x8d, 0xe7, 0xa7, 0xb0, 0x52, 0x03, 0x61, 0x70, 0x70, 0x12, 0x27, 0x0a, 0x05, 0x73, 0x63,
	0x6f, 0x70, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x42, 0x11, 0x92, 0x41, 0x0e, 0x32, 0x0c,
	0xe5, 0x85, 0xb3, 0xe8, 0x81, 0x94, 0xe8, 0xa7, 0x84, 0xe5, 0x88, 0x99, 0x52, 0x05, 0x73, 0x63,
	0x6f, 0x70, 0x65, 0x22, 0x93, 0x01, 0x0a, 0x0f, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x53, 0x63,
	0x6f, 0x70, 0x65, 0x53, 0x70, 0x65, 0x63, 0x12, 0x32, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0d, 0x42, 0x22, 0x92, 0x41, 0x1f, 0x32, 0x1d, 0xe5, 0xae, 0xa2, 0xe6, 0x88, 0xb7,
	0xe7, 0xab, 0xaf, 0xe5, 0xaf, 0x86, 0xe9, 0x92, 0xa5, 0xe5, 0x85, 0xb3, 0xe8, 0x81, 0x94, 0xe6,
	0x9c, 0x8d, 0xe5, 0x8a, 0xa1, 0x49, 0x44, 0x52, 0x02, 0x69, 0x64, 0x12, 0x23, 0x0a, 0x03, 0x61,
	0x70, 0x70, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x42, 0x11, 0x92, 0x41, 0x0e, 0x32, 0x0c, 0xe6,
	0x9c, 0x8d, 0xe5, 0x8a, 0xa1, 0xe5, 0x90, 0x8d, 0xe7, 0xa7, 0xb0, 0x52, 0x03, 0x61, 0x70, 0x70,
	0x12, 0x27, 0x0a, 0x05, 0x73, 0x63, 0x6f, 0x70, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x42,
	0x11, 0x92, 0x41, 0x0e, 0x32, 0x0c, 0xe5, 0x85, 0xb3, 0xe8, 0x81, 0x94, 0xe8, 0xa7, 0x84, 0xe5,
	0x88, 0x99, 0x52, 0x05, 0x73, 0x63, 0x6f, 0x70, 0x65, 0x42, 0x4d, 0x5a, 0x4b, 0x67, 0x69, 0x74,
	0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x54, 0x65, 0x6e, 0x63, 0x65, 0x6e, 0x74, 0x42,
	0x6c, 0x75, 0x65, 0x4b, 0x69, 0x6e, 0x67, 0x2f, 0x62, 0x6b, 0x2d, 0x62, 0x73, 0x63, 0x70, 0x2f,
	0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2f, 0x63, 0x6f, 0x72,
	0x65, 0x2f, 0x63, 0x72, 0x65, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x61, 0x6c, 0x2d, 0x73, 0x63, 0x6f,
	0x70, 0x65, 0x3b, 0x70, 0x62, 0x63, 0x72, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_credential_scope_proto_rawDescOnce sync.Once
	file_credential_scope_proto_rawDescData = file_credential_scope_proto_rawDesc
)

func file_credential_scope_proto_rawDescGZIP() []byte {
	file_credential_scope_proto_rawDescOnce.Do(func() {
		file_credential_scope_proto_rawDescData = protoimpl.X.CompressGZIP(file_credential_scope_proto_rawDescData)
	})
	return file_credential_scope_proto_rawDescData
}

var file_credential_scope_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_credential_scope_proto_goTypes = []any{
	(*CredentialScopeAttachment)(nil), // 0: pbcrs.CredentialScopeAttachment
	(*CredentialScopeList)(nil),       // 1: pbcrs.CredentialScopeList
	(*CredentialScopeSpec)(nil),       // 2: pbcrs.CredentialScopeSpec
	(*UpdateScopeSpec)(nil),           // 3: pbcrs.UpdateScopeSpec
	(*base.Revision)(nil),             // 4: pbbase.Revision
}
var file_credential_scope_proto_depIdxs = []int32{
	2, // 0: pbcrs.CredentialScopeList.spec:type_name -> pbcrs.CredentialScopeSpec
	0, // 1: pbcrs.CredentialScopeList.attachment:type_name -> pbcrs.CredentialScopeAttachment
	4, // 2: pbcrs.CredentialScopeList.revision:type_name -> pbbase.Revision
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_credential_scope_proto_init() }
func file_credential_scope_proto_init() {
	if File_credential_scope_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_credential_scope_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_credential_scope_proto_goTypes,
		DependencyIndexes: file_credential_scope_proto_depIdxs,
		MessageInfos:      file_credential_scope_proto_msgTypes,
	}.Build()
	File_credential_scope_proto = out.File
	file_credential_scope_proto_rawDesc = nil
	file_credential_scope_proto_goTypes = nil
	file_credential_scope_proto_depIdxs = nil
}
