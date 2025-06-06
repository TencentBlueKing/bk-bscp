// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.35.2
// 	protoc        v4.25.1
// source: base.proto

package pbbase

import (
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

// Revision source resource reference: pkg/dal/table/table.go
type Revision struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Creator  string `protobuf:"bytes,1,opt,name=creator,proto3" json:"creator,omitempty"`
	Reviser  string `protobuf:"bytes,2,opt,name=reviser,proto3" json:"reviser,omitempty"`
	CreateAt string `protobuf:"bytes,3,opt,name=create_at,json=createAt,proto3" json:"create_at,omitempty"`
	UpdateAt string `protobuf:"bytes,4,opt,name=update_at,json=updateAt,proto3" json:"update_at,omitempty"`
}

func (x *Revision) Reset() {
	*x = Revision{}
	mi := &file_base_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Revision) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Revision) ProtoMessage() {}

func (x *Revision) ProtoReflect() protoreflect.Message {
	mi := &file_base_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Revision.ProtoReflect.Descriptor instead.
func (*Revision) Descriptor() ([]byte, []int) {
	return file_base_proto_rawDescGZIP(), []int{0}
}

func (x *Revision) GetCreator() string {
	if x != nil {
		return x.Creator
	}
	return ""
}

func (x *Revision) GetReviser() string {
	if x != nil {
		return x.Reviser
	}
	return ""
}

func (x *Revision) GetCreateAt() string {
	if x != nil {
		return x.CreateAt
	}
	return ""
}

func (x *Revision) GetUpdateAt() string {
	if x != nil {
		return x.UpdateAt
	}
	return ""
}

// CreatedRevision source resource reference: pkg/dal/table/table.go
type CreatedRevision struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Creator  string `protobuf:"bytes,1,opt,name=creator,proto3" json:"creator,omitempty"`
	CreateAt string `protobuf:"bytes,2,opt,name=create_at,json=createAt,proto3" json:"create_at,omitempty"`
}

func (x *CreatedRevision) Reset() {
	*x = CreatedRevision{}
	mi := &file_base_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CreatedRevision) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreatedRevision) ProtoMessage() {}

func (x *CreatedRevision) ProtoReflect() protoreflect.Message {
	mi := &file_base_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreatedRevision.ProtoReflect.Descriptor instead.
func (*CreatedRevision) Descriptor() ([]byte, []int) {
	return file_base_proto_rawDescGZIP(), []int{1}
}

func (x *CreatedRevision) GetCreator() string {
	if x != nil {
		return x.Creator
	}
	return ""
}

func (x *CreatedRevision) GetCreateAt() string {
	if x != nil {
		return x.CreateAt
	}
	return ""
}

// BasePage source resource reference: pkg/types/page.go
type BasePage struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Count bool   `protobuf:"varint,1,opt,name=count,proto3" json:"count,omitempty"`
	Start uint32 `protobuf:"varint,2,opt,name=start,proto3" json:"start,omitempty"`
	Limit uint32 `protobuf:"varint,3,opt,name=limit,proto3" json:"limit,omitempty"`
	Sort  string `protobuf:"bytes,4,opt,name=sort,proto3" json:"sort,omitempty"`
	// direction is enum type.
	Order string `protobuf:"bytes,5,opt,name=order,proto3" json:"order,omitempty"`
}

func (x *BasePage) Reset() {
	*x = BasePage{}
	mi := &file_base_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *BasePage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BasePage) ProtoMessage() {}

func (x *BasePage) ProtoReflect() protoreflect.Message {
	mi := &file_base_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BasePage.ProtoReflect.Descriptor instead.
func (*BasePage) Descriptor() ([]byte, []int) {
	return file_base_proto_rawDescGZIP(), []int{2}
}

func (x *BasePage) GetCount() bool {
	if x != nil {
		return x.Count
	}
	return false
}

func (x *BasePage) GetStart() uint32 {
	if x != nil {
		return x.Start
	}
	return 0
}

func (x *BasePage) GetLimit() uint32 {
	if x != nil {
		return x.Limit
	}
	return 0
}

func (x *BasePage) GetSort() string {
	if x != nil {
		return x.Sort
	}
	return ""
}

func (x *BasePage) GetOrder() string {
	if x != nil {
		return x.Order
	}
	return ""
}

type EmptyReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *EmptyReq) Reset() {
	*x = EmptyReq{}
	mi := &file_base_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *EmptyReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EmptyReq) ProtoMessage() {}

func (x *EmptyReq) ProtoReflect() protoreflect.Message {
	mi := &file_base_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EmptyReq.ProtoReflect.Descriptor instead.
func (*EmptyReq) Descriptor() ([]byte, []int) {
	return file_base_proto_rawDescGZIP(), []int{3}
}

type EmptyResp struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *EmptyResp) Reset() {
	*x = EmptyResp{}
	mi := &file_base_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *EmptyResp) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EmptyResp) ProtoMessage() {}

func (x *EmptyResp) ProtoReflect() protoreflect.Message {
	mi := &file_base_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EmptyResp.ProtoReflect.Descriptor instead.
func (*EmptyResp) Descriptor() ([]byte, []int) {
	return file_base_proto_rawDescGZIP(), []int{4}
}

type BaseResp struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Code    int32  `protobuf:"varint,1,opt,name=code,proto3" json:"code,omitempty"`
	Message string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
}

func (x *BaseResp) Reset() {
	*x = BaseResp{}
	mi := &file_base_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *BaseResp) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BaseResp) ProtoMessage() {}

func (x *BaseResp) ProtoReflect() protoreflect.Message {
	mi := &file_base_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BaseResp.ProtoReflect.Descriptor instead.
func (*BaseResp) Descriptor() ([]byte, []int) {
	return file_base_proto_rawDescGZIP(), []int{5}
}

func (x *BaseResp) GetCode() int32 {
	if x != nil {
		return x.Code
	}
	return 0
}

func (x *BaseResp) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

// Versioning defines the version control related info.
type Versioning struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Major version when you make incompatible API changes.
	Major uint32 `protobuf:"varint,1,opt,name=Major,proto3" json:"Major,omitempty"`
	// Minor version when you add functionality in a backwards compatible manner.
	Minor uint32 `protobuf:"varint,2,opt,name=Minor,proto3" json:"Minor,omitempty"`
	// Patch version when you make backwards compatible bug fixes.
	Patch uint32 `protobuf:"varint,3,opt,name=Patch,proto3" json:"Patch,omitempty"`
}

func (x *Versioning) Reset() {
	*x = Versioning{}
	mi := &file_base_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Versioning) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Versioning) ProtoMessage() {}

func (x *Versioning) ProtoReflect() protoreflect.Message {
	mi := &file_base_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Versioning.ProtoReflect.Descriptor instead.
func (*Versioning) Descriptor() ([]byte, []int) {
	return file_base_proto_rawDescGZIP(), []int{6}
}

func (x *Versioning) GetMajor() uint32 {
	if x != nil {
		return x.Major
	}
	return 0
}

func (x *Versioning) GetMinor() uint32 {
	if x != nil {
		return x.Minor
	}
	return 0
}

func (x *Versioning) GetPatch() uint32 {
	if x != nil {
		return x.Patch
	}
	return 0
}

// 参数错误
type InvalidArgument struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Field   string `protobuf:"bytes,1,opt,name=field,proto3" json:"field,omitempty"`
	Message string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
}

func (x *InvalidArgument) Reset() {
	*x = InvalidArgument{}
	mi := &file_base_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *InvalidArgument) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*InvalidArgument) ProtoMessage() {}

func (x *InvalidArgument) ProtoReflect() protoreflect.Message {
	mi := &file_base_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use InvalidArgument.ProtoReflect.Descriptor instead.
func (*InvalidArgument) Descriptor() ([]byte, []int) {
	return file_base_proto_rawDescGZIP(), []int{7}
}

func (x *InvalidArgument) GetField() string {
	if x != nil {
		return x.Field
	}
	return ""
}

func (x *InvalidArgument) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

type ErrDetails struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PrimaryError   uint32 `protobuf:"varint,1,opt,name=PrimaryError,proto3" json:"PrimaryError,omitempty"`
	SecondaryError uint32 `protobuf:"varint,2,opt,name=SecondaryError,proto3" json:"SecondaryError,omitempty"`
}

func (x *ErrDetails) Reset() {
	*x = ErrDetails{}
	mi := &file_base_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ErrDetails) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ErrDetails) ProtoMessage() {}

func (x *ErrDetails) ProtoReflect() protoreflect.Message {
	mi := &file_base_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ErrDetails.ProtoReflect.Descriptor instead.
func (*ErrDetails) Descriptor() ([]byte, []int) {
	return file_base_proto_rawDescGZIP(), []int{8}
}

func (x *ErrDetails) GetPrimaryError() uint32 {
	if x != nil {
		return x.PrimaryError
	}
	return 0
}

func (x *ErrDetails) GetSecondaryError() uint32 {
	if x != nil {
		return x.SecondaryError
	}
	return 0
}

var File_base_proto protoreflect.FileDescriptor

var file_base_proto_rawDesc = []byte{
	0x0a, 0x0a, 0x62, 0x61, 0x73, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x06, 0x70, 0x62,
	0x62, 0x61, 0x73, 0x65, 0x1a, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x2d, 0x67, 0x65, 0x6e,
	0x2d, 0x6f, 0x70, 0x65, 0x6e, 0x61, 0x70, 0x69, 0x76, 0x32, 0x2f, 0x6f, 0x70, 0x74, 0x69, 0x6f,
	0x6e, 0x73, 0x2f, 0x61, 0x6e, 0x6e, 0x6f, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x22, 0xbe, 0x01, 0x0a, 0x08, 0x52, 0x65, 0x76, 0x69, 0x73, 0x69, 0x6f,
	0x6e, 0x12, 0x28, 0x0a, 0x07, 0x63, 0x72, 0x65, 0x61, 0x74, 0x6f, 0x72, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x42, 0x0e, 0x92, 0x41, 0x0b, 0x32, 0x09, 0xe5, 0x88, 0x9b, 0xe5, 0xbb, 0xba, 0xe4,
	0xba, 0xba, 0x52, 0x07, 0x63, 0x72, 0x65, 0x61, 0x74, 0x6f, 0x72, 0x12, 0x28, 0x0a, 0x07, 0x72,
	0x65, 0x76, 0x69, 0x73, 0x65, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x42, 0x0e, 0x92, 0x41,
	0x0b, 0x32, 0x09, 0xe6, 0x9b, 0xb4, 0xe6, 0x96, 0xb0, 0xe4, 0xba, 0xba, 0x52, 0x07, 0x72, 0x65,
	0x76, 0x69, 0x73, 0x65, 0x72, 0x12, 0x2e, 0x0a, 0x09, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x5f,
	0x61, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x42, 0x11, 0x92, 0x41, 0x0e, 0x32, 0x0c, 0xe5,
	0x88, 0x9b, 0xe5, 0xbb, 0xba, 0xe6, 0x97, 0xb6, 0xe9, 0x97, 0xb4, 0x52, 0x08, 0x63, 0x72, 0x65,
	0x61, 0x74, 0x65, 0x41, 0x74, 0x12, 0x2e, 0x0a, 0x09, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x5f,
	0x61, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x42, 0x11, 0x92, 0x41, 0x0e, 0x32, 0x0c, 0xe6,
	0x9b, 0xb4, 0xe6, 0x96, 0xb0, 0xe6, 0x97, 0xb6, 0xe9, 0x97, 0xb4, 0x52, 0x08, 0x75, 0x70, 0x64,
	0x61, 0x74, 0x65, 0x41, 0x74, 0x22, 0x6b, 0x0a, 0x0f, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64,
	0x52, 0x65, 0x76, 0x69, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x28, 0x0a, 0x07, 0x63, 0x72, 0x65, 0x61,
	0x74, 0x6f, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x42, 0x0e, 0x92, 0x41, 0x0b, 0x32, 0x09,
	0xe5, 0x88, 0x9b, 0xe5, 0xbb, 0xba, 0xe4, 0xba, 0xba, 0x52, 0x07, 0x63, 0x72, 0x65, 0x61, 0x74,
	0x6f, 0x72, 0x12, 0x2e, 0x0a, 0x09, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x5f, 0x61, 0x74, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x42, 0x11, 0x92, 0x41, 0x0e, 0x32, 0x0c, 0xe5, 0x88, 0x9b, 0xe5,
	0xbb, 0xba, 0xe6, 0x97, 0xb6, 0xe9, 0x97, 0xb4, 0x52, 0x08, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65,
	0x41, 0x74, 0x22, 0xa0, 0x01, 0x0a, 0x08, 0x42, 0x61, 0x73, 0x65, 0x50, 0x61, 0x67, 0x65, 0x12,
	0x21, 0x0a, 0x05, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x42, 0x0b,
	0x92, 0x41, 0x08, 0x32, 0x06, 0xe6, 0x80, 0xbb, 0xe6, 0x95, 0xb0, 0x52, 0x05, 0x63, 0x6f, 0x75,
	0x6e, 0x74, 0x12, 0x21, 0x0a, 0x05, 0x73, 0x74, 0x61, 0x72, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x0d, 0x42, 0x0b, 0x92, 0x41, 0x08, 0x32, 0x06, 0xe9, 0xa1, 0xb5, 0xe7, 0xa0, 0x81, 0x52, 0x05,
	0x73, 0x74, 0x61, 0x72, 0x74, 0x12, 0x24, 0x0a, 0x05, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x0d, 0x42, 0x0e, 0x92, 0x41, 0x0b, 0x32, 0x09, 0xe5, 0x88, 0x9b, 0xe5, 0xbb,
	0xba, 0xe4, 0xba, 0xba, 0x52, 0x05, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x73,
	0x6f, 0x72, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x73, 0x6f, 0x72, 0x74, 0x12,
	0x14, 0x0a, 0x05, 0x6f, 0x72, 0x64, 0x65, 0x72, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05,
	0x6f, 0x72, 0x64, 0x65, 0x72, 0x22, 0x0a, 0x0a, 0x08, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x52, 0x65,
	0x71, 0x22, 0x0b, 0x0a, 0x09, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x52, 0x65, 0x73, 0x70, 0x22, 0x38,
	0x0a, 0x08, 0x42, 0x61, 0x73, 0x65, 0x52, 0x65, 0x73, 0x70, 0x12, 0x12, 0x0a, 0x04, 0x63, 0x6f,
	0x64, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x12, 0x18,
	0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x22, 0x4e, 0x0a, 0x0a, 0x56, 0x65, 0x72, 0x73,
	0x69, 0x6f, 0x6e, 0x69, 0x6e, 0x67, 0x12, 0x14, 0x0a, 0x05, 0x4d, 0x61, 0x6a, 0x6f, 0x72, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x05, 0x4d, 0x61, 0x6a, 0x6f, 0x72, 0x12, 0x14, 0x0a, 0x05,
	0x4d, 0x69, 0x6e, 0x6f, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x05, 0x4d, 0x69, 0x6e,
	0x6f, 0x72, 0x12, 0x14, 0x0a, 0x05, 0x50, 0x61, 0x74, 0x63, 0x68, 0x18, 0x03, 0x20, 0x01, 0x28,
	0x0d, 0x52, 0x05, 0x50, 0x61, 0x74, 0x63, 0x68, 0x22, 0x41, 0x0a, 0x0f, 0x49, 0x6e, 0x76, 0x61,
	0x6c, 0x69, 0x64, 0x41, 0x72, 0x67, 0x75, 0x6d, 0x65, 0x6e, 0x74, 0x12, 0x14, 0x0a, 0x05, 0x66,
	0x69, 0x65, 0x6c, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x66, 0x69, 0x65, 0x6c,
	0x64, 0x12, 0x18, 0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x22, 0x58, 0x0a, 0x0a, 0x45,
	0x72, 0x72, 0x44, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x73, 0x12, 0x22, 0x0a, 0x0c, 0x50, 0x72, 0x69,
	0x6d, 0x61, 0x72, 0x79, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0d, 0x52,
	0x0c, 0x50, 0x72, 0x69, 0x6d, 0x61, 0x72, 0x79, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x12, 0x26, 0x0a,
	0x0e, 0x53, 0x65, 0x63, 0x6f, 0x6e, 0x64, 0x61, 0x72, 0x79, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x0e, 0x53, 0x65, 0x63, 0x6f, 0x6e, 0x64, 0x61, 0x72, 0x79,
	0x45, 0x72, 0x72, 0x6f, 0x72, 0x42, 0x42, 0x5a, 0x40, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e,
	0x63, 0x6f, 0x6d, 0x2f, 0x54, 0x65, 0x6e, 0x63, 0x65, 0x6e, 0x74, 0x42, 0x6c, 0x75, 0x65, 0x4b,
	0x69, 0x6e, 0x67, 0x2f, 0x62, 0x6b, 0x2d, 0x62, 0x73, 0x63, 0x70, 0x2f, 0x70, 0x6b, 0x67, 0x2f,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2f, 0x63, 0x6f, 0x72, 0x65, 0x2f, 0x62, 0x61,
	0x73, 0x65, 0x3b, 0x70, 0x62, 0x62, 0x61, 0x73, 0x65, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x33,
}

var (
	file_base_proto_rawDescOnce sync.Once
	file_base_proto_rawDescData = file_base_proto_rawDesc
)

func file_base_proto_rawDescGZIP() []byte {
	file_base_proto_rawDescOnce.Do(func() {
		file_base_proto_rawDescData = protoimpl.X.CompressGZIP(file_base_proto_rawDescData)
	})
	return file_base_proto_rawDescData
}

var file_base_proto_msgTypes = make([]protoimpl.MessageInfo, 9)
var file_base_proto_goTypes = []any{
	(*Revision)(nil),        // 0: pbbase.Revision
	(*CreatedRevision)(nil), // 1: pbbase.CreatedRevision
	(*BasePage)(nil),        // 2: pbbase.BasePage
	(*EmptyReq)(nil),        // 3: pbbase.EmptyReq
	(*EmptyResp)(nil),       // 4: pbbase.EmptyResp
	(*BaseResp)(nil),        // 5: pbbase.BaseResp
	(*Versioning)(nil),      // 6: pbbase.Versioning
	(*InvalidArgument)(nil), // 7: pbbase.InvalidArgument
	(*ErrDetails)(nil),      // 8: pbbase.ErrDetails
}
var file_base_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_base_proto_init() }
func file_base_proto_init() {
	if File_base_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_base_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   9,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_base_proto_goTypes,
		DependencyIndexes: file_base_proto_depIdxs,
		MessageInfos:      file_base_proto_msgTypes,
	}.Build()
	File_base_proto = out.File
	file_base_proto_rawDesc = nil
	file_base_proto_goTypes = nil
	file_base_proto_depIdxs = nil
}
