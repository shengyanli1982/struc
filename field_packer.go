package struc

import (
	"encoding/binary"
	"fmt"
	"reflect"
)

// FieldPacker 定义了字段打包的接口
// 这是Field类职责分离后的第三个组件，专门负责打包逻辑
// 包含所有与字段打包相关的方法
type FieldPacker interface {
	// Pack 将字段值序列化到字节缓冲区中
	Pack(buffer []byte, fieldValue reflect.Value, length int, options *Options) (int, error)

	// PackSingleValue 打包单个字段值
	PackSingleValue(buffer []byte, fieldValue reflect.Value, length int, options *Options) (int, error)

	// PackSliceValue 打包切片字段值
	PackSliceValue(buffer []byte, fieldValue reflect.Value, length int, options *Options) (int, error)

	// PackString 打包字符串字段值
	PackString(buffer []byte, fieldValue reflect.Value) (int, error)

	// PackCustom 打包自定义类型字段值
	PackCustom(buffer []byte, fieldValue reflect.Value, options *Options) (int, error)

	// WriteInteger 写入整数值到缓冲区
	WriteInteger(buffer []byte, intValue uint64, resolvedType Type, byteOrder binary.ByteOrder) error

	// WriteFloat 写入浮点数值到缓冲区
	WriteFloat(buffer []byte, floatValue float64, resolvedType Type, byteOrder binary.ByteOrder) error

	// PackPaddingBytes 打包填充字节
	PackPaddingBytes(buffer []byte, length int) (int, error)
}

// DefaultFieldPacker 是FieldPacker的默认实现
// 包含从现有Field类迁移的打包逻辑
type DefaultFieldPacker struct {
	descriptor *FieldDescriptor
}

// NewFieldPacker 创建一个新的字段打包器
func NewFieldPacker(descriptor *FieldDescriptor) FieldPacker {
	return &DefaultFieldPacker{
		descriptor: descriptor,
	}
}

// Pack 将字段值序列化到字节缓冲区中
func (p *DefaultFieldPacker) Pack(buffer []byte, fieldValue reflect.Value, length int, options *Options) (int, error) {
	if p.descriptor.IsSlice {
		return p.PackSliceValue(buffer, fieldValue, length, options)
	}
	return p.PackSingleValue(buffer, fieldValue, length, options)
}

// PackSingleValue 打包单个字段值
func (p *DefaultFieldPacker) PackSingleValue(buffer []byte, fieldValue reflect.Value, length int, options *Options) (int, error) {
	resolvedType := p.descriptor.Type.Resolve(options)
	byteOrder := p.determineByteOrder(options)

	switch resolvedType {
	case Pad:
		return p.PackPaddingBytes(buffer, length)
	case String:
		return p.PackString(buffer, fieldValue)
	case CustomType:
		return p.PackCustom(buffer, fieldValue, options)
	case Struct:
		return p.descriptor.NestFields.Pack(buffer, fieldValue, options)
	}

	// 处理基本类型
	switch resolvedType {
	case Float32, Float64:
		floatValue := fieldValue.Float()
		if err := p.WriteFloat(buffer, floatValue, resolvedType, byteOrder); err != nil {
			return 0, err
		}
	default:
		intValue := p.getIntegerValue(fieldValue)
		if err := p.WriteInteger(buffer, intValue, resolvedType, byteOrder); err != nil {
			return 0, err
		}
	}

	return resolvedType.Size(), nil
}

// PackSliceValue 打包切片字段值
func (p *DefaultFieldPacker) PackSliceValue(buffer []byte, fieldValue reflect.Value, length int, options *Options) (int, error) {
	resolvedType := p.descriptor.Type.Resolve(options)
	byteOrder := p.determineByteOrder(options)

	position := 0
	sliceLength := fieldValue.Len()

	if length >= 0 && sliceLength != length {
		return 0, fmt.Errorf("struc: field %s has length %d but expected %d", p.descriptor.Name, sliceLength, length)
	}

	for i := 0; i < sliceLength; i++ {
		elementValue := fieldValue.Index(i)

		switch resolvedType {
		case Float32, Float64:
			floatValue := elementValue.Float()
			if err := p.WriteFloat(buffer[position:], floatValue, resolvedType, byteOrder); err != nil {
				return position, err
			}
		default:
			intValue := p.getIntegerValue(elementValue)
			if err := p.WriteInteger(buffer[position:], intValue, resolvedType, byteOrder); err != nil {
				return position, err
			}
		}
		position += resolvedType.Size()
	}

	return position, nil
}

// PackString 打包字符串字段值
func (p *DefaultFieldPacker) PackString(buffer []byte, fieldValue reflect.Value) (int, error) {
	str := fieldValue.String()
	strBytes := unsafeString2Bytes(str)
	copy(buffer, strBytes)
	return len(strBytes), nil
}

// PackCustom 打包自定义类型字段值
func (p *DefaultFieldPacker) PackCustom(buffer []byte, fieldValue reflect.Value, options *Options) (int, error) {
	if customType, ok := fieldValue.Addr().Interface().(CustomBinaryer); ok {
		return customType.Pack(buffer, options)
	}
	return 0, fmt.Errorf("failed to pack custom type: %v", fieldValue.Type())
}

// PackPaddingBytes 打包填充字节
func (p *DefaultFieldPacker) PackPaddingBytes(buffer []byte, length int) (int, error) {
	for i := 0; i < length; i++ {
		buffer[i] = 0
	}
	return length, nil
}

// WriteInteger 写入整数值到缓冲区
func (p *DefaultFieldPacker) WriteInteger(buffer []byte, intValue uint64, resolvedType Type, byteOrder binary.ByteOrder) error {
	switch resolvedType {
	case Int8:
		buffer[0] = byte(int8(intValue))
	case Int16:
		unsafePutUint16(buffer, uint16(int16(intValue)), byteOrder)
	case Int32:
		unsafePutUint32(buffer, uint32(int32(intValue)), byteOrder)
	case Int64:
		unsafePutUint64(buffer, uint64(int64(intValue)), byteOrder)
	case Bool, Uint8:
		buffer[0] = byte(intValue)
	case Uint16:
		unsafePutUint16(buffer, uint16(intValue), byteOrder)
	case Uint32:
		unsafePutUint32(buffer, uint32(intValue), byteOrder)
	case Uint64:
		unsafePutUint64(buffer, uint64(intValue), byteOrder)
	default:
		return fmt.Errorf("unsupported integer type: %v", resolvedType)
	}
	return nil
}

// WriteFloat 写入浮点数值到缓冲区
func (p *DefaultFieldPacker) WriteFloat(buffer []byte, floatValue float64, resolvedType Type, byteOrder binary.ByteOrder) error {
	switch resolvedType {
	case Float32:
		unsafePutFloat32(buffer, float32(floatValue), byteOrder)
	case Float64:
		unsafePutFloat64(buffer, floatValue, byteOrder)
	default:
		return fmt.Errorf("unsupported float type: %v", resolvedType)
	}
	return nil
}

// determineByteOrder 返回要使用的字节序
func (p *DefaultFieldPacker) determineByteOrder(options *Options) binary.ByteOrder {
	if options.Order != nil {
		return options.Order
	}
	return p.descriptor.ByteOrder
}

// getIntegerValue 从 reflect.Value 中提取整数值
func (p *DefaultFieldPacker) getIntegerValue(fieldValue reflect.Value) uint64 {
	switch p.descriptor.kind {
	case reflect.Bool:
		if fieldValue.Bool() {
			return 1
		}
		return 0
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return uint64(fieldValue.Int())
	default:
		return fieldValue.Uint()
	}
}