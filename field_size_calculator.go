package struc

import (
	"reflect"
)

// FieldSizeCalculator 定义了字段大小计算的接口
// 这是Field类职责分离后的第二个组件，专门负责大小计算逻辑
// 包含所有与字段大小计算相关的方法
type FieldSizeCalculator interface {
	// Size 计算字段在二进制格式中占用的字节数
	// 考虑了对齐和填充要求
	Size(fieldValue reflect.Value, options *Options) int

	// CalculateStructSize 计算结构体类型的字节大小
	// 处理普通结构体和结构体切片
	CalculateStructSize(fieldValue reflect.Value, options *Options) int

	// CalculateCustomSize 计算自定义类型的字节大小
	// 通过调用类型的 Size 方法获取
	CalculateCustomSize(fieldValue reflect.Value, options *Options) int

	// CalculateBasicSize 计算基本类型的字节大小
	// 处理基本类型和平台相关类型
	CalculateBasicSize(fieldValue reflect.Value, resolvedType Type, options *Options) int

	// AlignSize 根据字节对齐要求调整大小
	// 考虑字节对齐和填充
	AlignSize(size int, options *Options) int
}

// DefaultFieldSizeCalculator 是FieldSizeCalculator的默认实现
// 包含从现有Field类迁移的大小计算逻辑
type DefaultFieldSizeCalculator struct {
	descriptor *FieldDescriptor
}

// NewFieldSizeCalculator 创建一个新的字段大小计算器
func NewFieldSizeCalculator(descriptor *FieldDescriptor) FieldSizeCalculator {
	return &DefaultFieldSizeCalculator{
		descriptor: descriptor,
	}
}

// Size 计算字段在二进制格式中占用的字节数
// 考虑了对齐和填充要求
func (c *DefaultFieldSizeCalculator) Size(fieldValue reflect.Value, options *Options) int {
	resolvedType := c.descriptor.Type.Resolve(options)
	totalSize := 0

	switch resolvedType {
	case Struct:
		totalSize = c.CalculateStructSize(fieldValue, options)
	case Pad:
		totalSize = c.descriptor.Length
	case CustomType:
		totalSize = c.CalculateCustomSize(fieldValue, options)
	default:
		totalSize = c.CalculateBasicSize(fieldValue, resolvedType, options)
	}

	return c.AlignSize(totalSize, options)
}

// CalculateStructSize 计算结构体类型的字节大小
// 处理普通结构体和结构体切片
func (c *DefaultFieldSizeCalculator) CalculateStructSize(fieldValue reflect.Value, options *Options) int {
	if c.descriptor.IsSlice {
		sliceLength := fieldValue.Len()
		totalSize := 0
		for i := 0; i < sliceLength; i++ {
			totalSize += c.descriptor.NestFields.Sizeof(fieldValue.Index(i), options)
		}
		return totalSize
	}
	return c.descriptor.NestFields.Sizeof(fieldValue, options)
}

// CalculateCustomSize 计算自定义类型的字节大小
// 通过调用类型的 Size 方法获取
func (c *DefaultFieldSizeCalculator) CalculateCustomSize(fieldValue reflect.Value, options *Options) int {
	if customType, ok := fieldValue.Addr().Interface().(CustomBinaryer); ok {
		return customType.Size(options)
	}
	return 0
}

// CalculateBasicSize 计算基本类型的字节大小
// 处理基本类型和平台相关类型
func (c *DefaultFieldSizeCalculator) CalculateBasicSize(fieldValue reflect.Value, resolvedType Type, options *Options) int {
	if c.descriptor.IsSlice {
		return fieldValue.Len() * resolvedType.Size()
	}
	return resolvedType.Size()
}

// AlignSize 根据字节对齐要求调整大小
// 考虑字节对齐和填充
func (c *DefaultFieldSizeCalculator) AlignSize(size int, options *Options) int {
	if options.ByteAlign > 0 && size > 0 {
		remainder := size % options.ByteAlign
		if remainder > 0 {
			size += options.ByteAlign - remainder
		}
	}
	return size
}
