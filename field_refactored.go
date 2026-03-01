package struc

import (
	"encoding/binary"
	"reflect"
)

// FieldRefactored 是重构后的 Field 类
// 使用职责分离的组件来实现功能，保持与原有接口的兼容性
type FieldRefactored struct {
	facade *FieldFacade
}

// NewFieldRefactored 创建一个新的重构后的 Field 类
func NewFieldRefactored() *FieldRefactored {
	descriptor := NewFieldDescriptor()
	facade := NewFieldFacade(descriptor)
	return &FieldRefactored{
		facade: facade,
	}
}

// CopyFrom 从现有 Field 对象复制字段描述信息
func (f *FieldRefactored) CopyFrom(field *Field) {
	f.facade.GetDescriptor().CopyFrom(field)
}

// Size 计算字段在二进制格式中占用的字节数
// 考虑了对齐和填充要求
func (f *FieldRefactored) Size(fieldValue reflect.Value, options *Options) int {
	return f.facade.Size(fieldValue, options)
}

// Pack 将字段值序列化到字节缓冲区中
func (f *FieldRefactored) Pack(buffer []byte, fieldValue reflect.Value, length int, options *Options) (int, error) {
	return f.facade.Pack(buffer, fieldValue, length, options)
}

// Unpack 从字节缓冲区中解包字段值
func (f *FieldRefactored) Unpack(buffer []byte, fieldValue reflect.Value, length int, options *Options) error {
	return f.facade.Unpack(buffer, fieldValue, length, options)
}

// GetDescriptor 返回字段描述符
func (f *FieldRefactored) GetDescriptor() *FieldDescriptor {
	return f.facade.GetDescriptor()
}

// ==================== 保持向后兼容的字段访问方法 ====================

// Name 返回字段名称
func (f *FieldRefactored) Name() string {
	return f.facade.GetDescriptor().Name
}

// IsPointer 返回字段是否为指针类型
func (f *FieldRefactored) IsPointer() bool {
	return f.facade.GetDescriptor().IsPointer
}

// Index 返回字段在结构体中的索引
func (f *FieldRefactored) Index() int {
	return f.facade.GetDescriptor().Index
}

// Type 返回字段的二进制类型
func (f *FieldRefactored) Type() Type {
	return f.facade.GetDescriptor().Type
}

// IsArray 返回字段是否为数组
func (f *FieldRefactored) IsArray() bool {
	return f.facade.GetDescriptor().IsArray
}

// IsSlice 返回字段是否为切片
func (f *FieldRefactored) IsSlice() bool {
	return f.facade.GetDescriptor().IsSlice
}

// Length 返回数组/固定切片的长度
func (f *FieldRefactored) Length() int {
	return f.facade.GetDescriptor().Length
}

// ByteOrder 返回字段的字节序
func (f *FieldRefactored) ByteOrder() binary.ByteOrder {
	return f.facade.GetDescriptor().ByteOrder
}

// Sizeof 返回 sizeof 引用的字段索引
func (f *FieldRefactored) Sizeof() []int {
	return f.facade.GetDescriptor().Sizeof
}

// Sizefrom 返回大小引用的字段索引
func (f *FieldRefactored) Sizefrom() []int {
	return f.facade.GetDescriptor().Sizefrom
}

// NestFields 返回嵌套结构体的字段
func (f *FieldRefactored) NestFields() Fields {
	return f.facade.GetDescriptor().NestFields
}

// Kind 返回 Go 的反射类型
func (f *FieldRefactored) Kind() reflect.Kind {
	return f.facade.GetDescriptor().GetKind()
}
