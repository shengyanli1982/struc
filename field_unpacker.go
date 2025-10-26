package struc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"reflect"
)

// FieldUnpacker 定义了字段解包的接口
// 这是Field类职责分离后的第四个组件，专门负责解包逻辑
// 包含所有与字段解包相关的方法
type FieldUnpacker interface {
	// Unpack 从字节缓冲区中解包字段值
	Unpack(buffer []byte, fieldValue reflect.Value, length int, options *Options) error

	// UnpackSingleValue 解包单个字段值
	UnpackSingleValue(buffer []byte, fieldValue reflect.Value, length int, options *Options) error

	// UnpackSliceValue 解包切片字段值
	UnpackSliceValue(buffer []byte, fieldValue reflect.Value, length int, options *Options) error

	// UnpackPaddingOrStringValue 解包填充或字符串字段值
	UnpackPaddingOrStringValue(buffer []byte, fieldValue reflect.Value, resolvedType Type) error

	// ReadInteger 从缓冲区读取整数值
	ReadInteger(buffer []byte, resolvedType Type, byteOrder binary.ByteOrder) uint64
}

// DefaultFieldUnpacker 是FieldUnpacker的默认实现
// 包含从现有Field类迁移的解包逻辑
type DefaultFieldUnpacker struct {
	descriptor *FieldDescriptor
}

// NewFieldUnpacker 创建一个新的字段解包器
func NewFieldUnpacker(descriptor *FieldDescriptor) FieldUnpacker {
	return &DefaultFieldUnpacker{
		descriptor: descriptor,
	}
}

// Unpack 从字节缓冲区中解包字段值
func (u *DefaultFieldUnpacker) Unpack(buffer []byte, fieldValue reflect.Value, length int, options *Options) error {
	if u.descriptor.IsSlice {
		return u.UnpackSliceValue(buffer, fieldValue, length, options)
	}
	return u.UnpackSingleValue(buffer, fieldValue, length, options)
}

// UnpackSingleValue 解包单个字段值
func (u *DefaultFieldUnpacker) UnpackSingleValue(buffer []byte, fieldValue reflect.Value, length int, options *Options) error {
	resolvedType := u.descriptor.Type.Resolve(options)
	byteOrder := u.determineByteOrder(options)

	if resolvedType == Pad || resolvedType == String {
		return u.UnpackPaddingOrStringValue(buffer, fieldValue, resolvedType)
	}

	switch resolvedType {
	case Float32, Float64:
		floatValue := u.readFloat(buffer, resolvedType, byteOrder)
		if u.descriptor.kind == reflect.Float32 || u.descriptor.kind == reflect.Float64 {
			fieldValue.SetFloat(floatValue)
			return nil
		}
		return fmt.Errorf("struc: refusing to unpack float into field %s of type %s", u.descriptor.Name, u.descriptor.kind.String())

	case Bool, Int8, Int16, Int32, Int64, Uint8, Uint16, Uint32, Uint64:
		// 处理整数和布尔类型
		intValue := u.ReadInteger(buffer, resolvedType, byteOrder)
		switch u.descriptor.kind {
		case reflect.Bool:
			fieldValue.SetBool(intValue != 0)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			fieldValue.SetInt(int64(intValue))
		default:
			fieldValue.SetUint(intValue)
		}
		return nil

	case Struct:
		return u.descriptor.NestFields.Unpack(bytes.NewReader(buffer), fieldValue, options)

	case String:
		if u.descriptor.kind != reflect.String {
			return fmt.Errorf("cannot unpack string into field %s of type %s", u.descriptor.Name, u.descriptor.kind)
		}
		str := unsafeBytes2String(buffer[:length])
		fieldValue.SetString(str)
		return nil

	case CustomType:
		if customType, ok := fieldValue.Addr().Interface().(CustomBinaryer); ok {
			return customType.Unpack(bytes.NewReader(buffer), length, options)
		}
		return fmt.Errorf("failed to unpack custom type: %v", fieldValue.Type())

	default:
		return fmt.Errorf("unsupported type for unpacking: %v", resolvedType)
	}
}

// UnpackSliceValue 解包切片字段值
func (u *DefaultFieldUnpacker) UnpackSliceValue(buffer []byte, fieldValue reflect.Value, length int, options *Options) error {
	resolvedType := u.descriptor.Type.Resolve(options)

	// 如果是数组则使用原值, 否则创建切片
	sliceValue := fieldValue
	if !u.descriptor.IsArray {
		sliceValue = reflect.MakeSlice(fieldValue.Type(), length, length)
	}

	elementSize := resolvedType.Size()
	for i := 0; i < length; i++ {
		start := i * elementSize
		end := start + elementSize
		if end > len(buffer) {
			return fmt.Errorf("buffer too small for slice unpacking")
		}

		elementValue := sliceValue.Index(i)
		if err := u.UnpackSingleValue(buffer[start:end], elementValue, 1, options); err != nil {
			return err
		}
	}

	if !u.descriptor.IsArray {
		fieldValue.Set(sliceValue)
	}
	return nil
}

// UnpackPaddingOrStringValue 解包填充或字符串字段值
func (u *DefaultFieldUnpacker) UnpackPaddingOrStringValue(buffer []byte, fieldValue reflect.Value, resolvedType Type) error {
	if resolvedType == Pad {
		// 填充字段不需要设置值
		return nil
	}
	// 字符串类型已经在UnpackSingleValue中处理
	return fmt.Errorf("unexpected type for padding/string unpacking: %v", resolvedType)
}

// ReadInteger 从缓冲区读取整数值
func (u *DefaultFieldUnpacker) ReadInteger(buffer []byte, resolvedType Type, byteOrder binary.ByteOrder) uint64 {
	switch resolvedType {
	case Int8:
		return uint64(int64(int8(buffer[0])))
	case Int16:
		return uint64(int64(int16(unsafeGetUint16(buffer, byteOrder))))
	case Int32:
		return uint64(int64(int32(unsafeGetUint32(buffer, byteOrder))))
	case Int64:
		return uint64(int64(unsafeGetUint64(buffer, byteOrder)))
	case Bool, Uint8:
		return uint64(buffer[0])
	case Uint16:
		return uint64(unsafeGetUint16(buffer, byteOrder))
	case Uint32:
		return uint64(unsafeGetUint32(buffer, byteOrder))
	case Uint64:
		return unsafeGetUint64(buffer, byteOrder)
	default:
		return 0
	}
}

// determineByteOrder 返回要使用的字节序
func (u *DefaultFieldUnpacker) determineByteOrder(options *Options) binary.ByteOrder {
	if options.Order != nil {
		return options.Order
	}
	return u.descriptor.ByteOrder
}

// readFloat 从缓冲区读取浮点数值
func (u *DefaultFieldUnpacker) readFloat(buffer []byte, resolvedType Type, byteOrder binary.ByteOrder) float64 {
	switch resolvedType {
	case Float32:
		return float64(unsafeGetFloat32(buffer, byteOrder))
	case Float64:
		return unsafeGetFloat64(buffer, byteOrder)
	default:
		return 0
	}
}