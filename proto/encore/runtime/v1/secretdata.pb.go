// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.12
// source: encore/runtime/v1/secretdata.proto

package runtimev1

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

type SecretData_Encoding int32

const (
	// Indicates the value is used as-is.
	SecretData_ENCODING_NONE SecretData_Encoding = 0
	// Indicates the value is base64-encoded.
	SecretData_ENCODING_BASE64 SecretData_Encoding = 1
)

// Enum value maps for SecretData_Encoding.
var (
	SecretData_Encoding_name = map[int32]string{
		0: "ENCODING_NONE",
		1: "ENCODING_BASE64",
	}
	SecretData_Encoding_value = map[string]int32{
		"ENCODING_NONE":   0,
		"ENCODING_BASE64": 1,
	}
)

func (x SecretData_Encoding) Enum() *SecretData_Encoding {
	p := new(SecretData_Encoding)
	*p = x
	return p
}

func (x SecretData_Encoding) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (SecretData_Encoding) Descriptor() protoreflect.EnumDescriptor {
	return file_encore_runtime_v1_secretdata_proto_enumTypes[0].Descriptor()
}

func (SecretData_Encoding) Type() protoreflect.EnumType {
	return &file_encore_runtime_v1_secretdata_proto_enumTypes[0]
}

func (x SecretData_Encoding) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use SecretData_Encoding.Descriptor instead.
func (SecretData_Encoding) EnumDescriptor() ([]byte, []int) {
	return file_encore_runtime_v1_secretdata_proto_rawDescGZIP(), []int{0, 0}
}

// Defines how to resolve a secret value.
type SecretData struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// How to resolve the initial secret value.
	// The output of this step is always a byte slice.
	//
	// Types that are assignable to Source:
	//	*SecretData_Embedded
	//	*SecretData_Env
	Source isSecretData_Source `protobuf_oneof:"source"`
	// How the value is encoded.
	Encoding SecretData_Encoding `protobuf:"varint,20,opt,name=encoding,proto3,enum=encore.runtime.v1.SecretData_Encoding" json:"encoding,omitempty"`
	// sub_path is an optional path to a sub-value within the secret data.
	//
	// Types that are assignable to SubPath:
	//	*SecretData_JsonKey
	SubPath isSecretData_SubPath `protobuf_oneof:"sub_path"`
}

func (x *SecretData) Reset() {
	*x = SecretData{}
	if protoimpl.UnsafeEnabled {
		mi := &file_encore_runtime_v1_secretdata_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SecretData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SecretData) ProtoMessage() {}

func (x *SecretData) ProtoReflect() protoreflect.Message {
	mi := &file_encore_runtime_v1_secretdata_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SecretData.ProtoReflect.Descriptor instead.
func (*SecretData) Descriptor() ([]byte, []int) {
	return file_encore_runtime_v1_secretdata_proto_rawDescGZIP(), []int{0}
}

func (m *SecretData) GetSource() isSecretData_Source {
	if m != nil {
		return m.Source
	}
	return nil
}

func (x *SecretData) GetEmbedded() []byte {
	if x, ok := x.GetSource().(*SecretData_Embedded); ok {
		return x.Embedded
	}
	return nil
}

func (x *SecretData) GetEnv() string {
	if x, ok := x.GetSource().(*SecretData_Env); ok {
		return x.Env
	}
	return ""
}

func (x *SecretData) GetEncoding() SecretData_Encoding {
	if x != nil {
		return x.Encoding
	}
	return SecretData_ENCODING_NONE
}

func (m *SecretData) GetSubPath() isSecretData_SubPath {
	if m != nil {
		return m.SubPath
	}
	return nil
}

func (x *SecretData) GetJsonKey() string {
	if x, ok := x.GetSubPath().(*SecretData_JsonKey); ok {
		return x.JsonKey
	}
	return ""
}

type isSecretData_Source interface {
	isSecretData_Source()
}

type SecretData_Embedded struct {
	// The secret data is embedded directly in the configuration.
	// This is insecure unless `encrypted` is true, and should only
	// be used for local development.
	Embedded []byte `protobuf:"bytes,1,opt,name=embedded,proto3,oneof"`
}

type SecretData_Env struct {
	// Look up the secret data in an env variable with the given name.
	// Assumes the
	Env string `protobuf:"bytes,2,opt,name=env,proto3,oneof"`
}

func (*SecretData_Embedded) isSecretData_Source() {}

func (*SecretData_Env) isSecretData_Source() {}

type isSecretData_SubPath interface {
	isSecretData_SubPath()
}

type SecretData_JsonKey struct {
	// json_key indicates the secret data is a JSON map,
	// and the resolved secret value is a key in that map.
	//
	// The value is encoded differently based on its type.
	// Supported types are utf-8 strings and raw bytes:
	// - For strings, the value is the string itself, e.g. "foo".
	// - For raw bytes, the value is a JSON object with a single key "bytes" and the value is the base64-encoded bytes.
	//
	// For example: '{"foo": "string-value", "bar": {"bytes": "aGVsbG8="}}'.
	JsonKey string `protobuf:"bytes,10,opt,name=json_key,json=jsonKey,proto3,oneof"`
}

func (*SecretData_JsonKey) isSecretData_SubPath() {}

var File_encore_runtime_v1_secretdata_proto protoreflect.FileDescriptor

var file_encore_runtime_v1_secretdata_proto_rawDesc = []byte{
	0x0a, 0x22, 0x65, 0x6e, 0x63, 0x6f, 0x72, 0x65, 0x2f, 0x72, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65,
	0x2f, 0x76, 0x31, 0x2f, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74, 0x64, 0x61, 0x74, 0x61, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x12, 0x11, 0x65, 0x6e, 0x63, 0x6f, 0x72, 0x65, 0x2e, 0x72, 0x75, 0x6e,
	0x74, 0x69, 0x6d, 0x65, 0x2e, 0x76, 0x31, 0x22, 0xf5, 0x01, 0x0a, 0x0a, 0x53, 0x65, 0x63, 0x72,
	0x65, 0x74, 0x44, 0x61, 0x74, 0x61, 0x12, 0x1c, 0x0a, 0x08, 0x65, 0x6d, 0x62, 0x65, 0x64, 0x64,
	0x65, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x48, 0x00, 0x52, 0x08, 0x65, 0x6d, 0x62, 0x65,
	0x64, 0x64, 0x65, 0x64, 0x12, 0x12, 0x0a, 0x03, 0x65, 0x6e, 0x76, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x09, 0x48, 0x00, 0x52, 0x03, 0x65, 0x6e, 0x76, 0x12, 0x42, 0x0a, 0x08, 0x65, 0x6e, 0x63, 0x6f,
	0x64, 0x69, 0x6e, 0x67, 0x18, 0x14, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x26, 0x2e, 0x65, 0x6e, 0x63,
	0x6f, 0x72, 0x65, 0x2e, 0x72, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x53,
	0x65, 0x63, 0x72, 0x65, 0x74, 0x44, 0x61, 0x74, 0x61, 0x2e, 0x45, 0x6e, 0x63, 0x6f, 0x64, 0x69,
	0x6e, 0x67, 0x52, 0x08, 0x65, 0x6e, 0x63, 0x6f, 0x64, 0x69, 0x6e, 0x67, 0x12, 0x1b, 0x0a, 0x08,
	0x6a, 0x73, 0x6f, 0x6e, 0x5f, 0x6b, 0x65, 0x79, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x09, 0x48, 0x01,
	0x52, 0x07, 0x6a, 0x73, 0x6f, 0x6e, 0x4b, 0x65, 0x79, 0x22, 0x32, 0x0a, 0x08, 0x45, 0x6e, 0x63,
	0x6f, 0x64, 0x69, 0x6e, 0x67, 0x12, 0x11, 0x0a, 0x0d, 0x45, 0x4e, 0x43, 0x4f, 0x44, 0x49, 0x4e,
	0x47, 0x5f, 0x4e, 0x4f, 0x4e, 0x45, 0x10, 0x00, 0x12, 0x13, 0x0a, 0x0f, 0x45, 0x4e, 0x43, 0x4f,
	0x44, 0x49, 0x4e, 0x47, 0x5f, 0x42, 0x41, 0x53, 0x45, 0x36, 0x34, 0x10, 0x01, 0x42, 0x08, 0x0a,
	0x06, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x42, 0x0a, 0x0a, 0x08, 0x73, 0x75, 0x62, 0x5f, 0x70,
	0x61, 0x74, 0x68, 0x4a, 0x04, 0x08, 0x03, 0x10, 0x0a, 0x4a, 0x04, 0x08, 0x0c, 0x10, 0x14, 0x42,
	0x2c, 0x5a, 0x2a, 0x65, 0x6e, 0x63, 0x72, 0x2e, 0x64, 0x65, 0x76, 0x2f, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x2f, 0x65, 0x6e, 0x63, 0x6f, 0x72, 0x65, 0x2f, 0x72, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65,
	0x2f, 0x76, 0x31, 0x3b, 0x72, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x76, 0x31, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_encore_runtime_v1_secretdata_proto_rawDescOnce sync.Once
	file_encore_runtime_v1_secretdata_proto_rawDescData = file_encore_runtime_v1_secretdata_proto_rawDesc
)

func file_encore_runtime_v1_secretdata_proto_rawDescGZIP() []byte {
	file_encore_runtime_v1_secretdata_proto_rawDescOnce.Do(func() {
		file_encore_runtime_v1_secretdata_proto_rawDescData = protoimpl.X.CompressGZIP(file_encore_runtime_v1_secretdata_proto_rawDescData)
	})
	return file_encore_runtime_v1_secretdata_proto_rawDescData
}

var file_encore_runtime_v1_secretdata_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_encore_runtime_v1_secretdata_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_encore_runtime_v1_secretdata_proto_goTypes = []interface{}{
	(SecretData_Encoding)(0), // 0: encore.runtime.v1.SecretData.Encoding
	(*SecretData)(nil),       // 1: encore.runtime.v1.SecretData
}
var file_encore_runtime_v1_secretdata_proto_depIdxs = []int32{
	0, // 0: encore.runtime.v1.SecretData.encoding:type_name -> encore.runtime.v1.SecretData.Encoding
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_encore_runtime_v1_secretdata_proto_init() }
func file_encore_runtime_v1_secretdata_proto_init() {
	if File_encore_runtime_v1_secretdata_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_encore_runtime_v1_secretdata_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SecretData); i {
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
	file_encore_runtime_v1_secretdata_proto_msgTypes[0].OneofWrappers = []interface{}{
		(*SecretData_Embedded)(nil),
		(*SecretData_Env)(nil),
		(*SecretData_JsonKey)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_encore_runtime_v1_secretdata_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_encore_runtime_v1_secretdata_proto_goTypes,
		DependencyIndexes: file_encore_runtime_v1_secretdata_proto_depIdxs,
		EnumInfos:         file_encore_runtime_v1_secretdata_proto_enumTypes,
		MessageInfos:      file_encore_runtime_v1_secretdata_proto_msgTypes,
	}.Build()
	File_encore_runtime_v1_secretdata_proto = out.File
	file_encore_runtime_v1_secretdata_proto_rawDesc = nil
	file_encore_runtime_v1_secretdata_proto_goTypes = nil
	file_encore_runtime_v1_secretdata_proto_depIdxs = nil
}
