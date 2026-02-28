package struc

import (
	"reflect"
)

// FieldFacade 是 Field 类的外观模式实现
// 通过组合各个职责分离的组件，提供统一的 Field 接口
// 保持与现有 Field 类相同的接口，确保向后兼容
type FieldFacade struct {
	descriptor     *FieldDescriptor
	sizeCalculator FieldSizeCalculator
	packer         FieldPacker
	unpacker       FieldUnpacker
}

// NewFieldFacade 创建一个新的 Field 外观类
func NewFieldFacade(descriptor *FieldDescriptor) *FieldFacade {
	return &FieldFacade{
		descriptor:     descriptor,
		sizeCalculator: NewFieldSizeCalculator(descriptor),
		packer:         NewFieldPacker(descriptor),
		unpacker:       NewFieldUnpacker(descriptor),
	}
}

// Size 计算字段在二进制格式中占用的字节数
// 考虑了对齐和填充要求
func (f *FieldFacade) Size(fieldValue reflect.Value, options *Options) int {
	return f.sizeCalculator.Size(fieldValue, options)
}

// Pack 将字段值序列化到字节缓冲区中
func (f *FieldFacade) Pack(buffer []byte, fieldValue reflect.Value, length int, options *Options) (int, error) {
	return f.packer.Pack(buffer, fieldValue, length, options)
}

// Unpack 从字节缓冲区中解包字段值
func (f *FieldFacade) Unpack(buffer []byte, fieldValue reflect.Value, length int, options *Options) error {
	return f.unpacker.Unpack(buffer, fieldValue, length, options)
}

// GetDescriptor 返回字段描述符
func (f *FieldFacade) GetDescriptor() *FieldDescriptor {
	return f.descriptor
}

// GetSizeCalculator 返回大小计算器
func (f *FieldFacade) GetSizeCalculator() FieldSizeCalculator {
	return f.sizeCalculator
}

// GetPacker 返回打包器
func (f *FieldFacade) GetPacker() FieldPacker {
	return f.packer
}

// GetUnpacker 返回解包器
func (f *FieldFacade) GetUnpacker() FieldUnpacker {
	return f.unpacker
}
