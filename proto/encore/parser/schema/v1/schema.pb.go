// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.25.0
// 	protoc        v3.14.0
// source: encore/parser/schema/v1/schema.proto

package v1

import (
	proto "github.com/golang/protobuf/proto"
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

// This is a compile-time assertion that a sufficiently up-to-date version
// of the legacy proto package is being used.
const _ = proto.ProtoPackageIsVersion4

type Builtin int32

const (
	Builtin_ANY     Builtin = 0
	Builtin_BOOL    Builtin = 1
	Builtin_INT8    Builtin = 2
	Builtin_INT16   Builtin = 3
	Builtin_INT32   Builtin = 4
	Builtin_INT64   Builtin = 5
	Builtin_UINT8   Builtin = 6
	Builtin_UINT16  Builtin = 7
	Builtin_UINT32  Builtin = 8
	Builtin_UINT64  Builtin = 9
	Builtin_FLOAT32 Builtin = 10
	Builtin_FLOAT64 Builtin = 11
	Builtin_STRING  Builtin = 12
	Builtin_BYTES   Builtin = 13
	Builtin_TIME    Builtin = 14
	Builtin_UUID    Builtin = 15
	Builtin_JSON    Builtin = 16
	Builtin_USER_ID Builtin = 17
	Builtin_INT     Builtin = 18
	Builtin_UINT    Builtin = 19
)

// Enum value maps for Builtin.
var (
	Builtin_name = map[int32]string{
		0:  "ANY",
		1:  "BOOL",
		2:  "INT8",
		3:  "INT16",
		4:  "INT32",
		5:  "INT64",
		6:  "UINT8",
		7:  "UINT16",
		8:  "UINT32",
		9:  "UINT64",
		10: "FLOAT32",
		11: "FLOAT64",
		12: "STRING",
		13: "BYTES",
		14: "TIME",
		15: "UUID",
		16: "JSON",
		17: "USER_ID",
		18: "INT",
		19: "UINT",
	}
	Builtin_value = map[string]int32{
		"ANY":     0,
		"BOOL":    1,
		"INT8":    2,
		"INT16":   3,
		"INT32":   4,
		"INT64":   5,
		"UINT8":   6,
		"UINT16":  7,
		"UINT32":  8,
		"UINT64":  9,
		"FLOAT32": 10,
		"FLOAT64": 11,
		"STRING":  12,
		"BYTES":   13,
		"TIME":    14,
		"UUID":    15,
		"JSON":    16,
		"USER_ID": 17,
		"INT":     18,
		"UINT":    19,
	}
)

func (x Builtin) Enum() *Builtin {
	p := new(Builtin)
	*p = x
	return p
}

func (x Builtin) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Builtin) Descriptor() protoreflect.EnumDescriptor {
	return file_encore_parser_schema_v1_schema_proto_enumTypes[0].Descriptor()
}

func (Builtin) Type() protoreflect.EnumType {
	return &file_encore_parser_schema_v1_schema_proto_enumTypes[0]
}

func (x Builtin) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Builtin.Descriptor instead.
func (Builtin) EnumDescriptor() ([]byte, []int) {
	return file_encore_parser_schema_v1_schema_proto_rawDescGZIP(), []int{0}
}

type Type struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Types that are assignable to Typ:
	//	*Type_Named
	//	*Type_Struct
	//	*Type_Map
	//	*Type_List
	//	*Type_Builtin
	Typ isType_Typ `protobuf_oneof:"typ"`
}

func (x *Type) Reset() {
	*x = Type{}
	if protoimpl.UnsafeEnabled {
		mi := &file_encore_parser_schema_v1_schema_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Type) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Type) ProtoMessage() {}

func (x *Type) ProtoReflect() protoreflect.Message {
	mi := &file_encore_parser_schema_v1_schema_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Type.ProtoReflect.Descriptor instead.
func (*Type) Descriptor() ([]byte, []int) {
	return file_encore_parser_schema_v1_schema_proto_rawDescGZIP(), []int{0}
}

func (m *Type) GetTyp() isType_Typ {
	if m != nil {
		return m.Typ
	}
	return nil
}

func (x *Type) GetNamed() *Named {
	if x, ok := x.GetTyp().(*Type_Named); ok {
		return x.Named
	}
	return nil
}

func (x *Type) GetStruct() *Struct {
	if x, ok := x.GetTyp().(*Type_Struct); ok {
		return x.Struct
	}
	return nil
}

func (x *Type) GetMap() *Map {
	if x, ok := x.GetTyp().(*Type_Map); ok {
		return x.Map
	}
	return nil
}

func (x *Type) GetList() *List {
	if x, ok := x.GetTyp().(*Type_List); ok {
		return x.List
	}
	return nil
}

func (x *Type) GetBuiltin() Builtin {
	if x, ok := x.GetTyp().(*Type_Builtin); ok {
		return x.Builtin
	}
	return Builtin_ANY
}

type isType_Typ interface {
	isType_Typ()
}

type Type_Named struct {
	Named *Named `protobuf:"bytes,1,opt,name=named,proto3,oneof"`
}

type Type_Struct struct {
	Struct *Struct `protobuf:"bytes,2,opt,name=struct,proto3,oneof"`
}

type Type_Map struct {
	Map *Map `protobuf:"bytes,3,opt,name=map,proto3,oneof"`
}

type Type_List struct {
	List *List `protobuf:"bytes,4,opt,name=list,proto3,oneof"`
}

type Type_Builtin struct {
	Builtin Builtin `protobuf:"varint,5,opt,name=builtin,proto3,enum=encore.parser.schema.v1.Builtin,oneof"`
}

func (*Type_Named) isType_Typ() {}

func (*Type_Struct) isType_Typ() {}

func (*Type_Map) isType_Typ() {}

func (*Type_List) isType_Typ() {}

func (*Type_Builtin) isType_Typ() {}

type Decl struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id   uint32 `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	Name string `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"` // type name
	Type *Type  `protobuf:"bytes,3,opt,name=type,proto3" json:"type,omitempty"`
	Doc  string `protobuf:"bytes,4,opt,name=doc,proto3" json:"doc,omitempty"`
	Loc  *Loc   `protobuf:"bytes,5,opt,name=loc,proto3" json:"loc,omitempty"`
}

func (x *Decl) Reset() {
	*x = Decl{}
	if protoimpl.UnsafeEnabled {
		mi := &file_encore_parser_schema_v1_schema_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Decl) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Decl) ProtoMessage() {}

func (x *Decl) ProtoReflect() protoreflect.Message {
	mi := &file_encore_parser_schema_v1_schema_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Decl.ProtoReflect.Descriptor instead.
func (*Decl) Descriptor() ([]byte, []int) {
	return file_encore_parser_schema_v1_schema_proto_rawDescGZIP(), []int{1}
}

func (x *Decl) GetId() uint32 {
	if x != nil {
		return x.Id
	}
	return 0
}

func (x *Decl) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Decl) GetType() *Type {
	if x != nil {
		return x.Type
	}
	return nil
}

func (x *Decl) GetDoc() string {
	if x != nil {
		return x.Doc
	}
	return ""
}

func (x *Decl) GetLoc() *Loc {
	if x != nil {
		return x.Loc
	}
	return nil
}

type Loc struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PkgPath      string `protobuf:"bytes,1,opt,name=pkg_path,json=pkgPath,proto3" json:"pkg_path,omitempty"`
	PkgName      string `protobuf:"bytes,2,opt,name=pkg_name,json=pkgName,proto3" json:"pkg_name,omitempty"`
	Filename     string `protobuf:"bytes,3,opt,name=filename,proto3" json:"filename,omitempty"`
	StartPos     int32  `protobuf:"varint,4,opt,name=start_pos,json=startPos,proto3" json:"start_pos,omitempty"`
	EndPos       int32  `protobuf:"varint,5,opt,name=end_pos,json=endPos,proto3" json:"end_pos,omitempty"`
	SrcLineStart int32  `protobuf:"varint,6,opt,name=src_line_start,json=srcLineStart,proto3" json:"src_line_start,omitempty"`
	SrcLineEnd   int32  `protobuf:"varint,7,opt,name=src_line_end,json=srcLineEnd,proto3" json:"src_line_end,omitempty"`
	SrcColStart  int32  `protobuf:"varint,8,opt,name=src_col_start,json=srcColStart,proto3" json:"src_col_start,omitempty"`
	SrcColEnd    int32  `protobuf:"varint,9,opt,name=src_col_end,json=srcColEnd,proto3" json:"src_col_end,omitempty"`
}

func (x *Loc) Reset() {
	*x = Loc{}
	if protoimpl.UnsafeEnabled {
		mi := &file_encore_parser_schema_v1_schema_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Loc) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Loc) ProtoMessage() {}

func (x *Loc) ProtoReflect() protoreflect.Message {
	mi := &file_encore_parser_schema_v1_schema_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Loc.ProtoReflect.Descriptor instead.
func (*Loc) Descriptor() ([]byte, []int) {
	return file_encore_parser_schema_v1_schema_proto_rawDescGZIP(), []int{2}
}

func (x *Loc) GetPkgPath() string {
	if x != nil {
		return x.PkgPath
	}
	return ""
}

func (x *Loc) GetPkgName() string {
	if x != nil {
		return x.PkgName
	}
	return ""
}

func (x *Loc) GetFilename() string {
	if x != nil {
		return x.Filename
	}
	return ""
}

func (x *Loc) GetStartPos() int32 {
	if x != nil {
		return x.StartPos
	}
	return 0
}

func (x *Loc) GetEndPos() int32 {
	if x != nil {
		return x.EndPos
	}
	return 0
}

func (x *Loc) GetSrcLineStart() int32 {
	if x != nil {
		return x.SrcLineStart
	}
	return 0
}

func (x *Loc) GetSrcLineEnd() int32 {
	if x != nil {
		return x.SrcLineEnd
	}
	return 0
}

func (x *Loc) GetSrcColStart() int32 {
	if x != nil {
		return x.SrcColStart
	}
	return 0
}

func (x *Loc) GetSrcColEnd() int32 {
	if x != nil {
		return x.SrcColEnd
	}
	return 0
}

type Named struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id uint32 `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
}

func (x *Named) Reset() {
	*x = Named{}
	if protoimpl.UnsafeEnabled {
		mi := &file_encore_parser_schema_v1_schema_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Named) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Named) ProtoMessage() {}

func (x *Named) ProtoReflect() protoreflect.Message {
	mi := &file_encore_parser_schema_v1_schema_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Named.ProtoReflect.Descriptor instead.
func (*Named) Descriptor() ([]byte, []int) {
	return file_encore_parser_schema_v1_schema_proto_rawDescGZIP(), []int{3}
}

func (x *Named) GetId() uint32 {
	if x != nil {
		return x.Id
	}
	return 0
}

type Struct struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Fields []*Field `protobuf:"bytes,1,rep,name=fields,proto3" json:"fields,omitempty"`
}

func (x *Struct) Reset() {
	*x = Struct{}
	if protoimpl.UnsafeEnabled {
		mi := &file_encore_parser_schema_v1_schema_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Struct) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Struct) ProtoMessage() {}

func (x *Struct) ProtoReflect() protoreflect.Message {
	mi := &file_encore_parser_schema_v1_schema_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Struct.ProtoReflect.Descriptor instead.
func (*Struct) Descriptor() ([]byte, []int) {
	return file_encore_parser_schema_v1_schema_proto_rawDescGZIP(), []int{4}
}

func (x *Struct) GetFields() []*Field {
	if x != nil {
		return x.Fields
	}
	return nil
}

type Field struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Typ  *Type  `protobuf:"bytes,1,opt,name=typ,proto3" json:"typ,omitempty"`
	Name string `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Doc  string `protobuf:"bytes,3,opt,name=doc,proto3" json:"doc,omitempty"`
	// The optional json name if it's different from the field name.
	JsonName string `protobuf:"bytes,4,opt,name=json_name,json=jsonName,proto3" json:"json_name,omitempty"`
	// Whether the field is optional.
	Optional bool `protobuf:"varint,5,opt,name=optional,proto3" json:"optional,omitempty"`
}

func (x *Field) Reset() {
	*x = Field{}
	if protoimpl.UnsafeEnabled {
		mi := &file_encore_parser_schema_v1_schema_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Field) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Field) ProtoMessage() {}

func (x *Field) ProtoReflect() protoreflect.Message {
	mi := &file_encore_parser_schema_v1_schema_proto_msgTypes[5]
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
	return file_encore_parser_schema_v1_schema_proto_rawDescGZIP(), []int{5}
}

func (x *Field) GetTyp() *Type {
	if x != nil {
		return x.Typ
	}
	return nil
}

func (x *Field) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Field) GetDoc() string {
	if x != nil {
		return x.Doc
	}
	return ""
}

func (x *Field) GetJsonName() string {
	if x != nil {
		return x.JsonName
	}
	return ""
}

func (x *Field) GetOptional() bool {
	if x != nil {
		return x.Optional
	}
	return false
}

type Map struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Key   *Type `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	Value *Type `protobuf:"bytes,2,opt,name=value,proto3" json:"value,omitempty"`
}

func (x *Map) Reset() {
	*x = Map{}
	if protoimpl.UnsafeEnabled {
		mi := &file_encore_parser_schema_v1_schema_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Map) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Map) ProtoMessage() {}

func (x *Map) ProtoReflect() protoreflect.Message {
	mi := &file_encore_parser_schema_v1_schema_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Map.ProtoReflect.Descriptor instead.
func (*Map) Descriptor() ([]byte, []int) {
	return file_encore_parser_schema_v1_schema_proto_rawDescGZIP(), []int{6}
}

func (x *Map) GetKey() *Type {
	if x != nil {
		return x.Key
	}
	return nil
}

func (x *Map) GetValue() *Type {
	if x != nil {
		return x.Value
	}
	return nil
}

type List struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Elem *Type `protobuf:"bytes,1,opt,name=elem,proto3" json:"elem,omitempty"`
}

func (x *List) Reset() {
	*x = List{}
	if protoimpl.UnsafeEnabled {
		mi := &file_encore_parser_schema_v1_schema_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *List) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*List) ProtoMessage() {}

func (x *List) ProtoReflect() protoreflect.Message {
	mi := &file_encore_parser_schema_v1_schema_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use List.ProtoReflect.Descriptor instead.
func (*List) Descriptor() ([]byte, []int) {
	return file_encore_parser_schema_v1_schema_proto_rawDescGZIP(), []int{7}
}

func (x *List) GetElem() *Type {
	if x != nil {
		return x.Elem
	}
	return nil
}

var File_encore_parser_schema_v1_schema_proto protoreflect.FileDescriptor

var file_encore_parser_schema_v1_schema_proto_rawDesc = []byte{
	0x0a, 0x24, 0x65, 0x6e, 0x63, 0x6f, 0x72, 0x65, 0x2f, 0x70, 0x61, 0x72, 0x73, 0x65, 0x72, 0x2f,
	0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2f, 0x76, 0x31, 0x2f, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x17, 0x65, 0x6e, 0x63, 0x6f, 0x72, 0x65, 0x2e, 0x70,
	0x61, 0x72, 0x73, 0x65, 0x72, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x76, 0x31, 0x22,
	0xa5, 0x02, 0x0a, 0x04, 0x54, 0x79, 0x70, 0x65, 0x12, 0x36, 0x0a, 0x05, 0x6e, 0x61, 0x6d, 0x65,
	0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x65, 0x6e, 0x63, 0x6f, 0x72, 0x65,
	0x2e, 0x70, 0x61, 0x72, 0x73, 0x65, 0x72, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x76,
	0x31, 0x2e, 0x4e, 0x61, 0x6d, 0x65, 0x64, 0x48, 0x00, 0x52, 0x05, 0x6e, 0x61, 0x6d, 0x65, 0x64,
	0x12, 0x39, 0x0a, 0x06, 0x73, 0x74, 0x72, 0x75, 0x63, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x1f, 0x2e, 0x65, 0x6e, 0x63, 0x6f, 0x72, 0x65, 0x2e, 0x70, 0x61, 0x72, 0x73, 0x65, 0x72,
	0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x76, 0x31, 0x2e, 0x53, 0x74, 0x72, 0x75, 0x63,
	0x74, 0x48, 0x00, 0x52, 0x06, 0x73, 0x74, 0x72, 0x75, 0x63, 0x74, 0x12, 0x30, 0x0a, 0x03, 0x6d,
	0x61, 0x70, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1c, 0x2e, 0x65, 0x6e, 0x63, 0x6f, 0x72,
	0x65, 0x2e, 0x70, 0x61, 0x72, 0x73, 0x65, 0x72, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e,
	0x76, 0x31, 0x2e, 0x4d, 0x61, 0x70, 0x48, 0x00, 0x52, 0x03, 0x6d, 0x61, 0x70, 0x12, 0x33, 0x0a,
	0x04, 0x6c, 0x69, 0x73, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1d, 0x2e, 0x65, 0x6e,
	0x63, 0x6f, 0x72, 0x65, 0x2e, 0x70, 0x61, 0x72, 0x73, 0x65, 0x72, 0x2e, 0x73, 0x63, 0x68, 0x65,
	0x6d, 0x61, 0x2e, 0x76, 0x31, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x48, 0x00, 0x52, 0x04, 0x6c, 0x69,
	0x73, 0x74, 0x12, 0x3c, 0x0a, 0x07, 0x62, 0x75, 0x69, 0x6c, 0x74, 0x69, 0x6e, 0x18, 0x05, 0x20,
	0x01, 0x28, 0x0e, 0x32, 0x20, 0x2e, 0x65, 0x6e, 0x63, 0x6f, 0x72, 0x65, 0x2e, 0x70, 0x61, 0x72,
	0x73, 0x65, 0x72, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x76, 0x31, 0x2e, 0x42, 0x75,
	0x69, 0x6c, 0x74, 0x69, 0x6e, 0x48, 0x00, 0x52, 0x07, 0x62, 0x75, 0x69, 0x6c, 0x74, 0x69, 0x6e,
	0x42, 0x05, 0x0a, 0x03, 0x74, 0x79, 0x70, 0x22, 0x9f, 0x01, 0x0a, 0x04, 0x44, 0x65, 0x63, 0x6c,
	0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x02, 0x69, 0x64,
	0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04,
	0x6e, 0x61, 0x6d, 0x65, 0x12, 0x31, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x1d, 0x2e, 0x65, 0x6e, 0x63, 0x6f, 0x72, 0x65, 0x2e, 0x70, 0x61, 0x72, 0x73,
	0x65, 0x72, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x76, 0x31, 0x2e, 0x54, 0x79, 0x70,
	0x65, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x10, 0x0a, 0x03, 0x64, 0x6f, 0x63, 0x18, 0x04,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x64, 0x6f, 0x63, 0x12, 0x2e, 0x0a, 0x03, 0x6c, 0x6f, 0x63,
	0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1c, 0x2e, 0x65, 0x6e, 0x63, 0x6f, 0x72, 0x65, 0x2e,
	0x70, 0x61, 0x72, 0x73, 0x65, 0x72, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x76, 0x31,
	0x2e, 0x4c, 0x6f, 0x63, 0x52, 0x03, 0x6c, 0x6f, 0x63, 0x22, 0x99, 0x02, 0x0a, 0x03, 0x4c, 0x6f,
	0x63, 0x12, 0x19, 0x0a, 0x08, 0x70, 0x6b, 0x67, 0x5f, 0x70, 0x61, 0x74, 0x68, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x07, 0x70, 0x6b, 0x67, 0x50, 0x61, 0x74, 0x68, 0x12, 0x19, 0x0a, 0x08,
	0x70, 0x6b, 0x67, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07,
	0x70, 0x6b, 0x67, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x1a, 0x0a, 0x08, 0x66, 0x69, 0x6c, 0x65, 0x6e,
	0x61, 0x6d, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x66, 0x69, 0x6c, 0x65, 0x6e,
	0x61, 0x6d, 0x65, 0x12, 0x1b, 0x0a, 0x09, 0x73, 0x74, 0x61, 0x72, 0x74, 0x5f, 0x70, 0x6f, 0x73,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x05, 0x52, 0x08, 0x73, 0x74, 0x61, 0x72, 0x74, 0x50, 0x6f, 0x73,
	0x12, 0x17, 0x0a, 0x07, 0x65, 0x6e, 0x64, 0x5f, 0x70, 0x6f, 0x73, 0x18, 0x05, 0x20, 0x01, 0x28,
	0x05, 0x52, 0x06, 0x65, 0x6e, 0x64, 0x50, 0x6f, 0x73, 0x12, 0x24, 0x0a, 0x0e, 0x73, 0x72, 0x63,
	0x5f, 0x6c, 0x69, 0x6e, 0x65, 0x5f, 0x73, 0x74, 0x61, 0x72, 0x74, 0x18, 0x06, 0x20, 0x01, 0x28,
	0x05, 0x52, 0x0c, 0x73, 0x72, 0x63, 0x4c, 0x69, 0x6e, 0x65, 0x53, 0x74, 0x61, 0x72, 0x74, 0x12,
	0x20, 0x0a, 0x0c, 0x73, 0x72, 0x63, 0x5f, 0x6c, 0x69, 0x6e, 0x65, 0x5f, 0x65, 0x6e, 0x64, 0x18,
	0x07, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0a, 0x73, 0x72, 0x63, 0x4c, 0x69, 0x6e, 0x65, 0x45, 0x6e,
	0x64, 0x12, 0x22, 0x0a, 0x0d, 0x73, 0x72, 0x63, 0x5f, 0x63, 0x6f, 0x6c, 0x5f, 0x73, 0x74, 0x61,
	0x72, 0x74, 0x18, 0x08, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0b, 0x73, 0x72, 0x63, 0x43, 0x6f, 0x6c,
	0x53, 0x74, 0x61, 0x72, 0x74, 0x12, 0x1e, 0x0a, 0x0b, 0x73, 0x72, 0x63, 0x5f, 0x63, 0x6f, 0x6c,
	0x5f, 0x65, 0x6e, 0x64, 0x18, 0x09, 0x20, 0x01, 0x28, 0x05, 0x52, 0x09, 0x73, 0x72, 0x63, 0x43,
	0x6f, 0x6c, 0x45, 0x6e, 0x64, 0x22, 0x17, 0x0a, 0x05, 0x4e, 0x61, 0x6d, 0x65, 0x64, 0x12, 0x0e,
	0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x02, 0x69, 0x64, 0x22, 0x40,
	0x0a, 0x06, 0x53, 0x74, 0x72, 0x75, 0x63, 0x74, 0x12, 0x36, 0x0a, 0x06, 0x66, 0x69, 0x65, 0x6c,
	0x64, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x65, 0x6e, 0x63, 0x6f, 0x72,
	0x65, 0x2e, 0x70, 0x61, 0x72, 0x73, 0x65, 0x72, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e,
	0x76, 0x31, 0x2e, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x52, 0x06, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x73,
	0x22, 0x97, 0x01, 0x0a, 0x05, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x12, 0x2f, 0x0a, 0x03, 0x74, 0x79,
	0x70, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1d, 0x2e, 0x65, 0x6e, 0x63, 0x6f, 0x72, 0x65,
	0x2e, 0x70, 0x61, 0x72, 0x73, 0x65, 0x72, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x76,
	0x31, 0x2e, 0x54, 0x79, 0x70, 0x65, 0x52, 0x03, 0x74, 0x79, 0x70, 0x12, 0x12, 0x0a, 0x04, 0x6e,
	0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12,
	0x10, 0x0a, 0x03, 0x64, 0x6f, 0x63, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x64, 0x6f,
	0x63, 0x12, 0x1b, 0x0a, 0x09, 0x6a, 0x73, 0x6f, 0x6e, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x04,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x6a, 0x73, 0x6f, 0x6e, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x1a,
	0x0a, 0x08, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x6c, 0x18, 0x05, 0x20, 0x01, 0x28, 0x08,
	0x52, 0x08, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x6c, 0x22, 0x6b, 0x0a, 0x03, 0x4d, 0x61,
	0x70, 0x12, 0x2f, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1d,
	0x2e, 0x65, 0x6e, 0x63, 0x6f, 0x72, 0x65, 0x2e, 0x70, 0x61, 0x72, 0x73, 0x65, 0x72, 0x2e, 0x73,
	0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x76, 0x31, 0x2e, 0x54, 0x79, 0x70, 0x65, 0x52, 0x03, 0x6b,
	0x65, 0x79, 0x12, 0x33, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x1d, 0x2e, 0x65, 0x6e, 0x63, 0x6f, 0x72, 0x65, 0x2e, 0x70, 0x61, 0x72, 0x73, 0x65,
	0x72, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x76, 0x31, 0x2e, 0x54, 0x79, 0x70, 0x65,
	0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x22, 0x39, 0x0a, 0x04, 0x4c, 0x69, 0x73, 0x74, 0x12,
	0x31, 0x0a, 0x04, 0x65, 0x6c, 0x65, 0x6d, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1d, 0x2e,
	0x65, 0x6e, 0x63, 0x6f, 0x72, 0x65, 0x2e, 0x70, 0x61, 0x72, 0x73, 0x65, 0x72, 0x2e, 0x73, 0x63,
	0x68, 0x65, 0x6d, 0x61, 0x2e, 0x76, 0x31, 0x2e, 0x54, 0x79, 0x70, 0x65, 0x52, 0x04, 0x65, 0x6c,
	0x65, 0x6d, 0x2a, 0xe5, 0x01, 0x0a, 0x07, 0x42, 0x75, 0x69, 0x6c, 0x74, 0x69, 0x6e, 0x12, 0x07,
	0x0a, 0x03, 0x41, 0x4e, 0x59, 0x10, 0x00, 0x12, 0x08, 0x0a, 0x04, 0x42, 0x4f, 0x4f, 0x4c, 0x10,
	0x01, 0x12, 0x08, 0x0a, 0x04, 0x49, 0x4e, 0x54, 0x38, 0x10, 0x02, 0x12, 0x09, 0x0a, 0x05, 0x49,
	0x4e, 0x54, 0x31, 0x36, 0x10, 0x03, 0x12, 0x09, 0x0a, 0x05, 0x49, 0x4e, 0x54, 0x33, 0x32, 0x10,
	0x04, 0x12, 0x09, 0x0a, 0x05, 0x49, 0x4e, 0x54, 0x36, 0x34, 0x10, 0x05, 0x12, 0x09, 0x0a, 0x05,
	0x55, 0x49, 0x4e, 0x54, 0x38, 0x10, 0x06, 0x12, 0x0a, 0x0a, 0x06, 0x55, 0x49, 0x4e, 0x54, 0x31,
	0x36, 0x10, 0x07, 0x12, 0x0a, 0x0a, 0x06, 0x55, 0x49, 0x4e, 0x54, 0x33, 0x32, 0x10, 0x08, 0x12,
	0x0a, 0x0a, 0x06, 0x55, 0x49, 0x4e, 0x54, 0x36, 0x34, 0x10, 0x09, 0x12, 0x0b, 0x0a, 0x07, 0x46,
	0x4c, 0x4f, 0x41, 0x54, 0x33, 0x32, 0x10, 0x0a, 0x12, 0x0b, 0x0a, 0x07, 0x46, 0x4c, 0x4f, 0x41,
	0x54, 0x36, 0x34, 0x10, 0x0b, 0x12, 0x0a, 0x0a, 0x06, 0x53, 0x54, 0x52, 0x49, 0x4e, 0x47, 0x10,
	0x0c, 0x12, 0x09, 0x0a, 0x05, 0x42, 0x59, 0x54, 0x45, 0x53, 0x10, 0x0d, 0x12, 0x08, 0x0a, 0x04,
	0x54, 0x49, 0x4d, 0x45, 0x10, 0x0e, 0x12, 0x08, 0x0a, 0x04, 0x55, 0x55, 0x49, 0x44, 0x10, 0x0f,
	0x12, 0x08, 0x0a, 0x04, 0x4a, 0x53, 0x4f, 0x4e, 0x10, 0x10, 0x12, 0x0b, 0x0a, 0x07, 0x55, 0x53,
	0x45, 0x52, 0x5f, 0x49, 0x44, 0x10, 0x11, 0x12, 0x07, 0x0a, 0x03, 0x49, 0x4e, 0x54, 0x10, 0x12,
	0x12, 0x08, 0x0a, 0x04, 0x55, 0x49, 0x4e, 0x54, 0x10, 0x13, 0x42, 0x28, 0x5a, 0x26, 0x65, 0x6e,
	0x63, 0x72, 0x2e, 0x64, 0x65, 0x76, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x65, 0x6e, 0x63,
	0x6f, 0x72, 0x65, 0x2f, 0x70, 0x61, 0x72, 0x73, 0x65, 0x72, 0x2f, 0x73, 0x63, 0x68, 0x65, 0x6d,
	0x61, 0x2f, 0x76, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_encore_parser_schema_v1_schema_proto_rawDescOnce sync.Once
	file_encore_parser_schema_v1_schema_proto_rawDescData = file_encore_parser_schema_v1_schema_proto_rawDesc
)

func file_encore_parser_schema_v1_schema_proto_rawDescGZIP() []byte {
	file_encore_parser_schema_v1_schema_proto_rawDescOnce.Do(func() {
		file_encore_parser_schema_v1_schema_proto_rawDescData = protoimpl.X.CompressGZIP(file_encore_parser_schema_v1_schema_proto_rawDescData)
	})
	return file_encore_parser_schema_v1_schema_proto_rawDescData
}

var file_encore_parser_schema_v1_schema_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_encore_parser_schema_v1_schema_proto_msgTypes = make([]protoimpl.MessageInfo, 8)
var file_encore_parser_schema_v1_schema_proto_goTypes = []interface{}{
	(Builtin)(0),   // 0: encore.parser.schema.v1.Builtin
	(*Type)(nil),   // 1: encore.parser.schema.v1.Type
	(*Decl)(nil),   // 2: encore.parser.schema.v1.Decl
	(*Loc)(nil),    // 3: encore.parser.schema.v1.Loc
	(*Named)(nil),  // 4: encore.parser.schema.v1.Named
	(*Struct)(nil), // 5: encore.parser.schema.v1.Struct
	(*Field)(nil),  // 6: encore.parser.schema.v1.Field
	(*Map)(nil),    // 7: encore.parser.schema.v1.Map
	(*List)(nil),   // 8: encore.parser.schema.v1.List
}
var file_encore_parser_schema_v1_schema_proto_depIdxs = []int32{
	4,  // 0: encore.parser.schema.v1.Type.named:type_name -> encore.parser.schema.v1.Named
	5,  // 1: encore.parser.schema.v1.Type.struct:type_name -> encore.parser.schema.v1.Struct
	7,  // 2: encore.parser.schema.v1.Type.map:type_name -> encore.parser.schema.v1.Map
	8,  // 3: encore.parser.schema.v1.Type.list:type_name -> encore.parser.schema.v1.List
	0,  // 4: encore.parser.schema.v1.Type.builtin:type_name -> encore.parser.schema.v1.Builtin
	1,  // 5: encore.parser.schema.v1.Decl.type:type_name -> encore.parser.schema.v1.Type
	3,  // 6: encore.parser.schema.v1.Decl.loc:type_name -> encore.parser.schema.v1.Loc
	6,  // 7: encore.parser.schema.v1.Struct.fields:type_name -> encore.parser.schema.v1.Field
	1,  // 8: encore.parser.schema.v1.Field.typ:type_name -> encore.parser.schema.v1.Type
	1,  // 9: encore.parser.schema.v1.Map.key:type_name -> encore.parser.schema.v1.Type
	1,  // 10: encore.parser.schema.v1.Map.value:type_name -> encore.parser.schema.v1.Type
	1,  // 11: encore.parser.schema.v1.List.elem:type_name -> encore.parser.schema.v1.Type
	12, // [12:12] is the sub-list for method output_type
	12, // [12:12] is the sub-list for method input_type
	12, // [12:12] is the sub-list for extension type_name
	12, // [12:12] is the sub-list for extension extendee
	0,  // [0:12] is the sub-list for field type_name
}

func init() { file_encore_parser_schema_v1_schema_proto_init() }
func file_encore_parser_schema_v1_schema_proto_init() {
	if File_encore_parser_schema_v1_schema_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_encore_parser_schema_v1_schema_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Type); i {
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
		file_encore_parser_schema_v1_schema_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Decl); i {
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
		file_encore_parser_schema_v1_schema_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Loc); i {
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
		file_encore_parser_schema_v1_schema_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Named); i {
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
		file_encore_parser_schema_v1_schema_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Struct); i {
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
		file_encore_parser_schema_v1_schema_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
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
		file_encore_parser_schema_v1_schema_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Map); i {
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
		file_encore_parser_schema_v1_schema_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*List); i {
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
	file_encore_parser_schema_v1_schema_proto_msgTypes[0].OneofWrappers = []interface{}{
		(*Type_Named)(nil),
		(*Type_Struct)(nil),
		(*Type_Map)(nil),
		(*Type_List)(nil),
		(*Type_Builtin)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_encore_parser_schema_v1_schema_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   8,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_encore_parser_schema_v1_schema_proto_goTypes,
		DependencyIndexes: file_encore_parser_schema_v1_schema_proto_depIdxs,
		EnumInfos:         file_encore_parser_schema_v1_schema_proto_enumTypes,
		MessageInfos:      file_encore_parser_schema_v1_schema_proto_msgTypes,
	}.Build()
	File_encore_parser_schema_v1_schema_proto = out.File
	file_encore_parser_schema_v1_schema_proto_rawDesc = nil
	file_encore_parser_schema_v1_schema_proto_goTypes = nil
	file_encore_parser_schema_v1_schema_proto_depIdxs = nil
}
