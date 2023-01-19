// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.19.3
// source: formats.proto

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

type DataFormat int32

const (
	DataFormat_TEXT DataFormat = 0
	DataFormat_FTDC DataFormat = 1
	DataFormat_BSON DataFormat = 2
	DataFormat_JSON DataFormat = 3
	DataFormat_CSV  DataFormat = 4
)

// Enum value maps for DataFormat.
var (
	DataFormat_name = map[int32]string{
		0: "TEXT",
		1: "FTDC",
		2: "BSON",
		3: "JSON",
		4: "CSV",
	}
	DataFormat_value = map[string]int32{
		"TEXT": 0,
		"FTDC": 1,
		"BSON": 2,
		"JSON": 3,
		"CSV":  4,
	}
)

func (x DataFormat) Enum() *DataFormat {
	p := new(DataFormat)
	*p = x
	return p
}

func (x DataFormat) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (DataFormat) Descriptor() protoreflect.EnumDescriptor {
	return file_formats_proto_enumTypes[0].Descriptor()
}

func (DataFormat) Type() protoreflect.EnumType {
	return &file_formats_proto_enumTypes[0]
}

func (x DataFormat) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use DataFormat.Descriptor instead.
func (DataFormat) EnumDescriptor() ([]byte, []int) {
	return file_formats_proto_rawDescGZIP(), []int{0}
}

type CompressionType int32

const (
	CompressionType_NONE  CompressionType = 0
	CompressionType_TARGZ CompressionType = 1
	CompressionType_ZIP   CompressionType = 2
	CompressionType_GZ    CompressionType = 3
	CompressionType_XZ    CompressionType = 4
)

// Enum value maps for CompressionType.
var (
	CompressionType_name = map[int32]string{
		0: "NONE",
		1: "TARGZ",
		2: "ZIP",
		3: "GZ",
		4: "XZ",
	}
	CompressionType_value = map[string]int32{
		"NONE":  0,
		"TARGZ": 1,
		"ZIP":   2,
		"GZ":    3,
		"XZ":    4,
	}
)

func (x CompressionType) Enum() *CompressionType {
	p := new(CompressionType)
	*p = x
	return p
}

func (x CompressionType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (CompressionType) Descriptor() protoreflect.EnumDescriptor {
	return file_formats_proto_enumTypes[1].Descriptor()
}

func (CompressionType) Type() protoreflect.EnumType {
	return &file_formats_proto_enumTypes[1]
}

func (x CompressionType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use CompressionType.Descriptor instead.
func (CompressionType) EnumDescriptor() ([]byte, []int) {
	return file_formats_proto_rawDescGZIP(), []int{1}
}

type SchemaType int32

const (
	SchemaType_RAW_EVENTS             SchemaType = 0
	SchemaType_COLLAPSED_EVENTS       SchemaType = 1
	SchemaType_INTERVAL_SUMMARIZATION SchemaType = 2
	SchemaType_HISTOGRAM              SchemaType = 3
)

// Enum value maps for SchemaType.
var (
	SchemaType_name = map[int32]string{
		0: "RAW_EVENTS",
		1: "COLLAPSED_EVENTS",
		2: "INTERVAL_SUMMARIZATION",
		3: "HISTOGRAM",
	}
	SchemaType_value = map[string]int32{
		"RAW_EVENTS":             0,
		"COLLAPSED_EVENTS":       1,
		"INTERVAL_SUMMARIZATION": 2,
		"HISTOGRAM":              3,
	}
)

func (x SchemaType) Enum() *SchemaType {
	p := new(SchemaType)
	*p = x
	return p
}

func (x SchemaType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (SchemaType) Descriptor() protoreflect.EnumDescriptor {
	return file_formats_proto_enumTypes[2].Descriptor()
}

func (SchemaType) Type() protoreflect.EnumType {
	return &file_formats_proto_enumTypes[2]
}

func (x SchemaType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use SchemaType.Descriptor instead.
func (SchemaType) EnumDescriptor() ([]byte, []int) {
	return file_formats_proto_rawDescGZIP(), []int{2}
}

var File_formats_proto protoreflect.FileDescriptor

var file_formats_proto_rawDesc = []byte{
	0x0a, 0x0d, 0x66, 0x6f, 0x72, 0x6d, 0x61, 0x74, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x05, 0x63, 0x65, 0x64, 0x61, 0x72, 0x2a, 0x3d, 0x0a, 0x0a, 0x44, 0x61, 0x74, 0x61, 0x46, 0x6f,
	0x72, 0x6d, 0x61, 0x74, 0x12, 0x08, 0x0a, 0x04, 0x54, 0x45, 0x58, 0x54, 0x10, 0x00, 0x12, 0x08,
	0x0a, 0x04, 0x46, 0x54, 0x44, 0x43, 0x10, 0x01, 0x12, 0x08, 0x0a, 0x04, 0x42, 0x53, 0x4f, 0x4e,
	0x10, 0x02, 0x12, 0x08, 0x0a, 0x04, 0x4a, 0x53, 0x4f, 0x4e, 0x10, 0x03, 0x12, 0x07, 0x0a, 0x03,
	0x43, 0x53, 0x56, 0x10, 0x04, 0x2a, 0x3f, 0x0a, 0x0f, 0x43, 0x6f, 0x6d, 0x70, 0x72, 0x65, 0x73,
	0x73, 0x69, 0x6f, 0x6e, 0x54, 0x79, 0x70, 0x65, 0x12, 0x08, 0x0a, 0x04, 0x4e, 0x4f, 0x4e, 0x45,
	0x10, 0x00, 0x12, 0x09, 0x0a, 0x05, 0x54, 0x41, 0x52, 0x47, 0x5a, 0x10, 0x01, 0x12, 0x07, 0x0a,
	0x03, 0x5a, 0x49, 0x50, 0x10, 0x02, 0x12, 0x06, 0x0a, 0x02, 0x47, 0x5a, 0x10, 0x03, 0x12, 0x06,
	0x0a, 0x02, 0x58, 0x5a, 0x10, 0x04, 0x2a, 0x5d, 0x0a, 0x0a, 0x53, 0x63, 0x68, 0x65, 0x6d, 0x61,
	0x54, 0x79, 0x70, 0x65, 0x12, 0x0e, 0x0a, 0x0a, 0x52, 0x41, 0x57, 0x5f, 0x45, 0x56, 0x45, 0x4e,
	0x54, 0x53, 0x10, 0x00, 0x12, 0x14, 0x0a, 0x10, 0x43, 0x4f, 0x4c, 0x4c, 0x41, 0x50, 0x53, 0x45,
	0x44, 0x5f, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x53, 0x10, 0x01, 0x12, 0x1a, 0x0a, 0x16, 0x49, 0x4e,
	0x54, 0x45, 0x52, 0x56, 0x41, 0x4c, 0x5f, 0x53, 0x55, 0x4d, 0x4d, 0x41, 0x52, 0x49, 0x5a, 0x41,
	0x54, 0x49, 0x4f, 0x4e, 0x10, 0x02, 0x12, 0x0d, 0x0a, 0x09, 0x48, 0x49, 0x53, 0x54, 0x4f, 0x47,
	0x52, 0x41, 0x4d, 0x10, 0x03, 0x42, 0x0e, 0x5a, 0x0c, 0x72, 0x70, 0x63, 0x2f, 0x69, 0x6e, 0x74,
	0x65, 0x72, 0x6e, 0x61, 0x6c, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_formats_proto_rawDescOnce sync.Once
	file_formats_proto_rawDescData = file_formats_proto_rawDesc
)

func file_formats_proto_rawDescGZIP() []byte {
	file_formats_proto_rawDescOnce.Do(func() {
		file_formats_proto_rawDescData = protoimpl.X.CompressGZIP(file_formats_proto_rawDescData)
	})
	return file_formats_proto_rawDescData
}

var file_formats_proto_enumTypes = make([]protoimpl.EnumInfo, 3)
var file_formats_proto_goTypes = []interface{}{
	(DataFormat)(0),      // 0: cedar.DataFormat
	(CompressionType)(0), // 1: cedar.CompressionType
	(SchemaType)(0),      // 2: cedar.SchemaType
}
var file_formats_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_formats_proto_init() }
func file_formats_proto_init() {
	if File_formats_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_formats_proto_rawDesc,
			NumEnums:      3,
			NumMessages:   0,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_formats_proto_goTypes,
		DependencyIndexes: file_formats_proto_depIdxs,
		EnumInfos:         file_formats_proto_enumTypes,
	}.Build()
	File_formats_proto = out.File
	file_formats_proto_rawDesc = nil
	file_formats_proto_goTypes = nil
	file_formats_proto_depIdxs = nil
}
