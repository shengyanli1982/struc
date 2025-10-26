package struc

import (
	"encoding/binary"
	"reflect"
)

// FieldDescriptor 表示结构体字段的元数据信息
// 包含字段的所有描述信息，用于二进制打包和解包
// 这是Field类职责分离后的第一个组件，专门负责字段描述
// 保持与现有Field类相同的字段定义，确保向后兼容
type FieldDescriptor struct {
	// 字段基本信息
	Name       string           // 字段名称
	IsPointer  bool             // 字段是否为指针类型
	Index      int              // 字段在结构体中的索引
	Type       Type             // 字段的二进制类型
	defType    Type             // 默认的二进制类型
	IsArray    bool             // 字段是否为数组
	IsSlice    bool             // 字段是否为切片
	Length     int              // 数组/固定切片的长度
	ByteOrder  binary.ByteOrder // 字段的字节序
	Sizeof     []int            // sizeof 引用的字段索引
	Sizefrom   []int            // 大小引用的字段索引
	NestFields Fields           // 嵌套结构体的字段
	kind       reflect.Kind     // Go 的反射类型
}

// NewFieldDescriptor 创建一个新的字段描述符
// 使用与现有Field类相同的字段初始化逻辑
func NewFieldDescriptor() *FieldDescriptor {
	return &FieldDescriptor{
		Length:    1,
		ByteOrder: binary.BigEndian, // 默认使用大端字节序
	}
}

// CopyFrom 从现有Field对象复制字段描述信息
// 用于在重构过程中保持字段定义的一致性
func (fd *FieldDescriptor) CopyFrom(field *Field) {
	fd.Name = field.Name
	fd.IsPointer = field.IsPointer
	fd.Index = field.Index
	fd.Type = field.Type
	fd.defType = field.defType
	fd.IsArray = field.IsArray
	fd.IsSlice = field.IsSlice
	fd.Length = field.Length
	fd.ByteOrder = field.ByteOrder
	fd.Sizeof = field.Sizeof
	fd.Sizefrom = field.Sizefrom
	fd.NestFields = field.NestFields
	fd.kind = field.kind
}

// GetKind 返回字段的反射类型
func (fd *FieldDescriptor) GetKind() reflect.Kind {
	return fd.kind
}

// GetType 返回字段的二进制类型
func (fd *FieldDescriptor) GetType() Type {
	return fd.Type
}

// IsCustomType 检查字段是否为自定义类型
func (fd *FieldDescriptor) IsCustomType() bool {
	return fd.Type == CustomType
}

// IsStructType 检查字段是否为结构体类型
func (fd *FieldDescriptor) IsStructType() bool {
	return fd.Type == Struct
}

// GetLength 返回字段的长度
func (fd *FieldDescriptor) GetLength() int {
	return fd.Length
}