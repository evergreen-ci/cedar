// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.19.3
// source: system_metrics.proto

package internal

import (
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

type SystemMetrics struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Info     *SystemMetricsInfo         `protobuf:"bytes,1,opt,name=info,proto3" json:"info,omitempty"`
	Artifact *SystemMetricsArtifactInfo `protobuf:"bytes,2,opt,name=artifact,proto3" json:"artifact,omitempty"`
}

func (x *SystemMetrics) Reset() {
	*x = SystemMetrics{}
	if protoimpl.UnsafeEnabled {
		mi := &file_system_metrics_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SystemMetrics) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SystemMetrics) ProtoMessage() {}

func (x *SystemMetrics) ProtoReflect() protoreflect.Message {
	mi := &file_system_metrics_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SystemMetrics.ProtoReflect.Descriptor instead.
func (*SystemMetrics) Descriptor() ([]byte, []int) {
	return file_system_metrics_proto_rawDescGZIP(), []int{0}
}

func (x *SystemMetrics) GetInfo() *SystemMetricsInfo {
	if x != nil {
		return x.Info
	}
	return nil
}

func (x *SystemMetrics) GetArtifact() *SystemMetricsArtifactInfo {
	if x != nil {
		return x.Artifact
	}
	return nil
}

type SystemMetricsInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Project   string `protobuf:"bytes,1,opt,name=project,proto3" json:"project,omitempty"`
	Version   string `protobuf:"bytes,2,opt,name=version,proto3" json:"version,omitempty"`
	Variant   string `protobuf:"bytes,3,opt,name=variant,proto3" json:"variant,omitempty"`
	TaskName  string `protobuf:"bytes,4,opt,name=task_name,json=taskName,proto3" json:"task_name,omitempty"`
	TaskId    string `protobuf:"bytes,5,opt,name=task_id,json=taskId,proto3" json:"task_id,omitempty"`
	Execution int32  `protobuf:"varint,6,opt,name=execution,proto3" json:"execution,omitempty"`
	Mainline  bool   `protobuf:"varint,7,opt,name=mainline,proto3" json:"mainline,omitempty"`
}

func (x *SystemMetricsInfo) Reset() {
	*x = SystemMetricsInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_system_metrics_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SystemMetricsInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SystemMetricsInfo) ProtoMessage() {}

func (x *SystemMetricsInfo) ProtoReflect() protoreflect.Message {
	mi := &file_system_metrics_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SystemMetricsInfo.ProtoReflect.Descriptor instead.
func (*SystemMetricsInfo) Descriptor() ([]byte, []int) {
	return file_system_metrics_proto_rawDescGZIP(), []int{1}
}

func (x *SystemMetricsInfo) GetProject() string {
	if x != nil {
		return x.Project
	}
	return ""
}

func (x *SystemMetricsInfo) GetVersion() string {
	if x != nil {
		return x.Version
	}
	return ""
}

func (x *SystemMetricsInfo) GetVariant() string {
	if x != nil {
		return x.Variant
	}
	return ""
}

func (x *SystemMetricsInfo) GetTaskName() string {
	if x != nil {
		return x.TaskName
	}
	return ""
}

func (x *SystemMetricsInfo) GetTaskId() string {
	if x != nil {
		return x.TaskId
	}
	return ""
}

func (x *SystemMetricsInfo) GetExecution() int32 {
	if x != nil {
		return x.Execution
	}
	return 0
}

func (x *SystemMetricsInfo) GetMainline() bool {
	if x != nil {
		return x.Mainline
	}
	return false
}

type SystemMetricsArtifactInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Compression CompressionType `protobuf:"varint,1,opt,name=compression,proto3,enum=cedar.CompressionType" json:"compression,omitempty"`
	Schema      SchemaType      `protobuf:"varint,2,opt,name=schema,proto3,enum=cedar.SchemaType" json:"schema,omitempty"`
}

func (x *SystemMetricsArtifactInfo) Reset() {
	*x = SystemMetricsArtifactInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_system_metrics_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SystemMetricsArtifactInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SystemMetricsArtifactInfo) ProtoMessage() {}

func (x *SystemMetricsArtifactInfo) ProtoReflect() protoreflect.Message {
	mi := &file_system_metrics_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SystemMetricsArtifactInfo.ProtoReflect.Descriptor instead.
func (*SystemMetricsArtifactInfo) Descriptor() ([]byte, []int) {
	return file_system_metrics_proto_rawDescGZIP(), []int{2}
}

func (x *SystemMetricsArtifactInfo) GetCompression() CompressionType {
	if x != nil {
		return x.Compression
	}
	return CompressionType_NONE
}

func (x *SystemMetricsArtifactInfo) GetSchema() SchemaType {
	if x != nil {
		return x.Schema
	}
	return SchemaType_RAW_EVENTS
}

type SystemMetricsData struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id     string     `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Type   string     `protobuf:"bytes,2,opt,name=type,proto3" json:"type,omitempty"`
	Format DataFormat `protobuf:"varint,3,opt,name=format,proto3,enum=cedar.DataFormat" json:"format,omitempty"`
	Data   []byte     `protobuf:"bytes,4,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *SystemMetricsData) Reset() {
	*x = SystemMetricsData{}
	if protoimpl.UnsafeEnabled {
		mi := &file_system_metrics_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SystemMetricsData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SystemMetricsData) ProtoMessage() {}

func (x *SystemMetricsData) ProtoReflect() protoreflect.Message {
	mi := &file_system_metrics_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SystemMetricsData.ProtoReflect.Descriptor instead.
func (*SystemMetricsData) Descriptor() ([]byte, []int) {
	return file_system_metrics_proto_rawDescGZIP(), []int{3}
}

func (x *SystemMetricsData) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *SystemMetricsData) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

func (x *SystemMetricsData) GetFormat() DataFormat {
	if x != nil {
		return x.Format
	}
	return DataFormat_TEXT
}

func (x *SystemMetricsData) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

type SystemMetricsSeriesEnd struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id      string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Success bool   `protobuf:"varint,2,opt,name=success,proto3" json:"success,omitempty"`
}

func (x *SystemMetricsSeriesEnd) Reset() {
	*x = SystemMetricsSeriesEnd{}
	if protoimpl.UnsafeEnabled {
		mi := &file_system_metrics_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SystemMetricsSeriesEnd) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SystemMetricsSeriesEnd) ProtoMessage() {}

func (x *SystemMetricsSeriesEnd) ProtoReflect() protoreflect.Message {
	mi := &file_system_metrics_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SystemMetricsSeriesEnd.ProtoReflect.Descriptor instead.
func (*SystemMetricsSeriesEnd) Descriptor() ([]byte, []int) {
	return file_system_metrics_proto_rawDescGZIP(), []int{4}
}

func (x *SystemMetricsSeriesEnd) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *SystemMetricsSeriesEnd) GetSuccess() bool {
	if x != nil {
		return x.Success
	}
	return false
}

type SystemMetricsResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
}

func (x *SystemMetricsResponse) Reset() {
	*x = SystemMetricsResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_system_metrics_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SystemMetricsResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SystemMetricsResponse) ProtoMessage() {}

func (x *SystemMetricsResponse) ProtoReflect() protoreflect.Message {
	mi := &file_system_metrics_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SystemMetricsResponse.ProtoReflect.Descriptor instead.
func (*SystemMetricsResponse) Descriptor() ([]byte, []int) {
	return file_system_metrics_proto_rawDescGZIP(), []int{5}
}

func (x *SystemMetricsResponse) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

var File_system_metrics_proto protoreflect.FileDescriptor

var file_system_metrics_proto_rawDesc = []byte{
	0x0a, 0x14, 0x73, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x5f, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x05, 0x63, 0x65, 0x64, 0x61, 0x72, 0x1a, 0x0d, 0x66,
	0x6f, 0x72, 0x6d, 0x61, 0x74, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x7b, 0x0a, 0x0d,
	0x53, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x12, 0x2c, 0x0a,
	0x04, 0x69, 0x6e, 0x66, 0x6f, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x18, 0x2e, 0x63, 0x65,
	0x64, 0x61, 0x72, 0x2e, 0x53, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63,
	0x73, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x04, 0x69, 0x6e, 0x66, 0x6f, 0x12, 0x3c, 0x0a, 0x08, 0x61,
	0x72, 0x74, 0x69, 0x66, 0x61, 0x63, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x20, 0x2e,
	0x63, 0x65, 0x64, 0x61, 0x72, 0x2e, 0x53, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x4d, 0x65, 0x74, 0x72,
	0x69, 0x63, 0x73, 0x41, 0x72, 0x74, 0x69, 0x66, 0x61, 0x63, 0x74, 0x49, 0x6e, 0x66, 0x6f, 0x52,
	0x08, 0x61, 0x72, 0x74, 0x69, 0x66, 0x61, 0x63, 0x74, 0x22, 0xd1, 0x01, 0x0a, 0x11, 0x53, 0x79,
	0x73, 0x74, 0x65, 0x6d, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x49, 0x6e, 0x66, 0x6f, 0x12,
	0x18, 0x0a, 0x07, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x07, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x12, 0x18, 0x0a, 0x07, 0x76, 0x65, 0x72,
	0x73, 0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x76, 0x65, 0x72, 0x73,
	0x69, 0x6f, 0x6e, 0x12, 0x18, 0x0a, 0x07, 0x76, 0x61, 0x72, 0x69, 0x61, 0x6e, 0x74, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x76, 0x61, 0x72, 0x69, 0x61, 0x6e, 0x74, 0x12, 0x1b, 0x0a,
	0x09, 0x74, 0x61, 0x73, 0x6b, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x08, 0x74, 0x61, 0x73, 0x6b, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x17, 0x0a, 0x07, 0x74, 0x61,
	0x73, 0x6b, 0x5f, 0x69, 0x64, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x74, 0x61, 0x73,
	0x6b, 0x49, 0x64, 0x12, 0x1c, 0x0a, 0x09, 0x65, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e,
	0x18, 0x06, 0x20, 0x01, 0x28, 0x05, 0x52, 0x09, 0x65, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f,
	0x6e, 0x12, 0x1a, 0x0a, 0x08, 0x6d, 0x61, 0x69, 0x6e, 0x6c, 0x69, 0x6e, 0x65, 0x18, 0x07, 0x20,
	0x01, 0x28, 0x08, 0x52, 0x08, 0x6d, 0x61, 0x69, 0x6e, 0x6c, 0x69, 0x6e, 0x65, 0x22, 0x80, 0x01,
	0x0a, 0x19, 0x53, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x41,
	0x72, 0x74, 0x69, 0x66, 0x61, 0x63, 0x74, 0x49, 0x6e, 0x66, 0x6f, 0x12, 0x38, 0x0a, 0x0b, 0x63,
	0x6f, 0x6d, 0x70, 0x72, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e,
	0x32, 0x16, 0x2e, 0x63, 0x65, 0x64, 0x61, 0x72, 0x2e, 0x43, 0x6f, 0x6d, 0x70, 0x72, 0x65, 0x73,
	0x73, 0x69, 0x6f, 0x6e, 0x54, 0x79, 0x70, 0x65, 0x52, 0x0b, 0x63, 0x6f, 0x6d, 0x70, 0x72, 0x65,
	0x73, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x29, 0x0a, 0x06, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x11, 0x2e, 0x63, 0x65, 0x64, 0x61, 0x72, 0x2e, 0x53, 0x63,
	0x68, 0x65, 0x6d, 0x61, 0x54, 0x79, 0x70, 0x65, 0x52, 0x06, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61,
	0x22, 0x76, 0x0a, 0x11, 0x53, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63,
	0x73, 0x44, 0x61, 0x74, 0x61, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x29, 0x0a, 0x06, 0x66, 0x6f, 0x72,
	0x6d, 0x61, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x11, 0x2e, 0x63, 0x65, 0x64, 0x61,
	0x72, 0x2e, 0x44, 0x61, 0x74, 0x61, 0x46, 0x6f, 0x72, 0x6d, 0x61, 0x74, 0x52, 0x06, 0x66, 0x6f,
	0x72, 0x6d, 0x61, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x04, 0x20, 0x01,
	0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x22, 0x42, 0x0a, 0x16, 0x53, 0x79, 0x73, 0x74,
	0x65, 0x6d, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x53, 0x65, 0x72, 0x69, 0x65, 0x73, 0x45,
	0x6e, 0x64, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02,
	0x69, 0x64, 0x12, 0x18, 0x0a, 0x07, 0x73, 0x75, 0x63, 0x63, 0x65, 0x73, 0x73, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x08, 0x52, 0x07, 0x73, 0x75, 0x63, 0x63, 0x65, 0x73, 0x73, 0x22, 0x27, 0x0a, 0x15,
	0x53, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x02, 0x69, 0x64, 0x32, 0xcf, 0x02, 0x0a, 0x12, 0x43, 0x65, 0x64, 0x61, 0x72, 0x53,
	0x79, 0x73, 0x74, 0x65, 0x6d, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x12, 0x4f, 0x0a, 0x19,
	0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x53, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x4d, 0x65, 0x74, 0x72,
	0x69, 0x63, 0x73, 0x52, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x12, 0x14, 0x2e, 0x63, 0x65, 0x64, 0x61,
	0x72, 0x2e, 0x53, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x1a,
	0x1c, 0x2e, 0x63, 0x65, 0x64, 0x61, 0x72, 0x2e, 0x53, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x4d, 0x65,
	0x74, 0x72, 0x69, 0x63, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x4a, 0x0a,
	0x10, 0x41, 0x64, 0x64, 0x53, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63,
	0x73, 0x12, 0x18, 0x2e, 0x63, 0x65, 0x64, 0x61, 0x72, 0x2e, 0x53, 0x79, 0x73, 0x74, 0x65, 0x6d,
	0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x44, 0x61, 0x74, 0x61, 0x1a, 0x1c, 0x2e, 0x63, 0x65,
	0x64, 0x61, 0x72, 0x2e, 0x53, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63,
	0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x4f, 0x0a, 0x13, 0x53, 0x74, 0x72,
	0x65, 0x61, 0x6d, 0x53, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73,
	0x12, 0x18, 0x2e, 0x63, 0x65, 0x64, 0x61, 0x72, 0x2e, 0x53, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x4d,
	0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x44, 0x61, 0x74, 0x61, 0x1a, 0x1c, 0x2e, 0x63, 0x65, 0x64,
	0x61, 0x72, 0x2e, 0x53, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x28, 0x01, 0x12, 0x4b, 0x0a, 0x0c, 0x43, 0x6c,
	0x6f, 0x73, 0x65, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x12, 0x1d, 0x2e, 0x63, 0x65, 0x64,
	0x61, 0x72, 0x2e, 0x53, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73,
	0x53, 0x65, 0x72, 0x69, 0x65, 0x73, 0x45, 0x6e, 0x64, 0x1a, 0x1c, 0x2e, 0x63, 0x65, 0x64, 0x61,
	0x72, 0x2e, 0x53, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x42, 0x0e, 0x5a, 0x0c, 0x72, 0x70, 0x63, 0x2f, 0x69,
	0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_system_metrics_proto_rawDescOnce sync.Once
	file_system_metrics_proto_rawDescData = file_system_metrics_proto_rawDesc
)

func file_system_metrics_proto_rawDescGZIP() []byte {
	file_system_metrics_proto_rawDescOnce.Do(func() {
		file_system_metrics_proto_rawDescData = protoimpl.X.CompressGZIP(file_system_metrics_proto_rawDescData)
	})
	return file_system_metrics_proto_rawDescData
}

var file_system_metrics_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_system_metrics_proto_goTypes = []interface{}{
	(*SystemMetrics)(nil),             // 0: cedar.SystemMetrics
	(*SystemMetricsInfo)(nil),         // 1: cedar.SystemMetricsInfo
	(*SystemMetricsArtifactInfo)(nil), // 2: cedar.SystemMetricsArtifactInfo
	(*SystemMetricsData)(nil),         // 3: cedar.SystemMetricsData
	(*SystemMetricsSeriesEnd)(nil),    // 4: cedar.SystemMetricsSeriesEnd
	(*SystemMetricsResponse)(nil),     // 5: cedar.SystemMetricsResponse
	(CompressionType)(0),              // 6: cedar.CompressionType
	(SchemaType)(0),                   // 7: cedar.SchemaType
	(DataFormat)(0),                   // 8: cedar.DataFormat
}
var file_system_metrics_proto_depIdxs = []int32{
	1, // 0: cedar.SystemMetrics.info:type_name -> cedar.SystemMetricsInfo
	2, // 1: cedar.SystemMetrics.artifact:type_name -> cedar.SystemMetricsArtifactInfo
	6, // 2: cedar.SystemMetricsArtifactInfo.compression:type_name -> cedar.CompressionType
	7, // 3: cedar.SystemMetricsArtifactInfo.schema:type_name -> cedar.SchemaType
	8, // 4: cedar.SystemMetricsData.format:type_name -> cedar.DataFormat
	0, // 5: cedar.CedarSystemMetrics.CreateSystemMetricsRecord:input_type -> cedar.SystemMetrics
	3, // 6: cedar.CedarSystemMetrics.AddSystemMetrics:input_type -> cedar.SystemMetricsData
	3, // 7: cedar.CedarSystemMetrics.StreamSystemMetrics:input_type -> cedar.SystemMetricsData
	4, // 8: cedar.CedarSystemMetrics.CloseMetrics:input_type -> cedar.SystemMetricsSeriesEnd
	5, // 9: cedar.CedarSystemMetrics.CreateSystemMetricsRecord:output_type -> cedar.SystemMetricsResponse
	5, // 10: cedar.CedarSystemMetrics.AddSystemMetrics:output_type -> cedar.SystemMetricsResponse
	5, // 11: cedar.CedarSystemMetrics.StreamSystemMetrics:output_type -> cedar.SystemMetricsResponse
	5, // 12: cedar.CedarSystemMetrics.CloseMetrics:output_type -> cedar.SystemMetricsResponse
	9, // [9:13] is the sub-list for method output_type
	5, // [5:9] is the sub-list for method input_type
	5, // [5:5] is the sub-list for extension type_name
	5, // [5:5] is the sub-list for extension extendee
	0, // [0:5] is the sub-list for field type_name
}

func init() { file_system_metrics_proto_init() }
func file_system_metrics_proto_init() {
	if File_system_metrics_proto != nil {
		return
	}
	file_formats_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_system_metrics_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SystemMetrics); i {
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
		file_system_metrics_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SystemMetricsInfo); i {
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
		file_system_metrics_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SystemMetricsArtifactInfo); i {
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
		file_system_metrics_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SystemMetricsData); i {
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
		file_system_metrics_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SystemMetricsSeriesEnd); i {
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
		file_system_metrics_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SystemMetricsResponse); i {
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
			RawDescriptor: file_system_metrics_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_system_metrics_proto_goTypes,
		DependencyIndexes: file_system_metrics_proto_depIdxs,
		MessageInfos:      file_system_metrics_proto_msgTypes,
	}.Build()
	File_system_metrics_proto = out.File
	file_system_metrics_proto_rawDesc = nil
	file_system_metrics_proto_goTypes = nil
	file_system_metrics_proto_depIdxs = nil
}
