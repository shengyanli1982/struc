// Package struc implements binary packing and unpacking for Go structs.
// struc 包实现了 Go 结构体的二进制打包和解包功能。
package struc

import (
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
	"strings"
)

// unpackBasicTypeSlicePool 是全局共享的字节块实例
// 用于在 unpackBasicType 方法内共享字节切片，减少内存分配
var unpackBasicTypeSlicePool = NewBytesSlicePool(0)

// Fields 是字段切片类型，用于管理结构体的字段集合
// 它提供了字段的序列化、反序列化和大小计算等功能
type Fields []*Field

// SetByteOrder 为所有字段设置字节序
// 这会影响字段值的二进制表示方式
func (f Fields) SetByteOrder(byteOrder binary.ByteOrder) {
	for _, field := range f {
		if field != nil {
			field.ByteOrder = byteOrder
		}
	}
}

// String 返回字段集合的字符串表示
// 主要用于调试和日志记录
func (f Fields) String() string {
	fieldStrings := make([]string, len(f))
	for i, field := range f {
		if field != nil {
			fieldStrings[i] = field.String()
		}
	}
	return "{" + strings.Join(fieldStrings, ", ") + "}"
}

// Sizeof 计算字段集合在内存中的总大小（字节数）
// 考虑了对齐和填充要求
func (f Fields) Sizeof(structValue reflect.Value, options *Options) int {
	// 解引用所有指针，获取实际的结构体值
	for structValue.Kind() == reflect.Ptr {
		structValue = structValue.Elem()
	}

	totalSize := 0
	for i, field := range f {
		if field != nil {
			totalSize += field.Size(structValue.Field(i), options)
		}
	}
	return totalSize
}

// sizefrom 根据引用字段的值确定切片或数组的长度
// 支持有符号和无符号整数类型的长度字段
func (f Fields) sizefrom(structValue reflect.Value, fieldIndex []int) int {
	lengthField := structValue.FieldByIndex(fieldIndex)

	switch lengthField.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return int(lengthField.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		lengthValue := int(lengthField.Uint())
		if lengthValue < 0 {
			return 0
		}
		return lengthValue
	default:
		fieldName := structValue.Type().FieldByIndex(fieldIndex).Name
		panic(fmt.Sprintf("sizeof field %T.%s not an integer type", structValue.Interface(), fieldName))
	}
}

// Pack 将字段集合打包到字节缓冲区中
// 支持基本类型、结构体、切片和自定义类型
func (f Fields) Pack(buffer []byte, structValue reflect.Value, options *Options) (int, error) {
	// 解引用指针，直到获取到非指针类型
	for structValue.Kind() == reflect.Ptr {
		structValue = structValue.Elem()
	}

	position := 0 // 当前缓冲区位置

	for i, field := range f {
		if field == nil {
			continue
		}

		fieldValue := structValue.Field(i)
		fieldLength := field.Length

		if field.Sizefrom != nil {
			fieldLength = f.sizefrom(structValue, field.Sizefrom)
		}
		if fieldLength <= 0 && field.IsSlice {
			fieldLength = fieldValue.Len()
		}

		if field.Sizeof != nil {
			sizeofLength := structValue.FieldByIndex(field.Sizeof).Len()

			switch field.kind {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				fieldValue.SetInt(int64(sizeofLength))
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				fieldValue.SetUint(uint64(sizeofLength))
			default:
				panic(fmt.Sprintf("sizeof field is not int or uint type: %s, %s", field.Name, fieldValue.Type()))
			}
		}

		bytesWritten, err := field.Pack(buffer[position:], fieldValue, fieldLength, options)
		if err != nil {
			return bytesWritten, err
		}
		position += bytesWritten
	}
	return position, nil
}

// Release 释放 Fields 切片中的所有 Field 对象
// 用于内存管理和资源回收
func (f Fields) Release() {
	releaseFields(f)
}

// unpackStruct 处理结构体类型的解包
func (f Fields) unpackStruct(reader io.Reader, fieldValue reflect.Value, field *Field, fieldLength int, options *Options) error {
	if field.IsSlice {
		return f.unpackStructSlice(reader, fieldValue, fieldLength, field.IsArray, options)
	}
	return f.unpackSingleStruct(reader, fieldValue, options)
}

// unpackStructSlice 处理结构体切片的解包
func (f Fields) unpackStructSlice(reader io.Reader, fieldValue reflect.Value, fieldLength int, isArray bool, options *Options) error {
	// 如果是数组则使用原值, 否则创建切片
	sliceValue := fieldValue
	if !isArray {
		sliceValue = reflect.MakeSlice(fieldValue.Type(), fieldLength, fieldLength)
	}

	for i := 0; i < fieldLength; i++ {
		elementValue := sliceValue.Index(i)
		fields, err := parseFields(elementValue)
		if err != nil {
			return err
		}
		if err := fields.Unpack(reader, elementValue, options); err != nil {
			return err
		}
	}

	if !isArray {
		fieldValue.Set(sliceValue)
	}
	return nil
}

// unpackSingleStruct 处理单个结构体的解包
func (f Fields) unpackSingleStruct(reader io.Reader, fieldValue reflect.Value, options *Options) error {
	fields, err := parseFields(fieldValue)
	if err != nil {
		return err
	}
	return fields.Unpack(reader, fieldValue, options)
}

// unpackBasicType 处理基本类型和自定义类型的解包
func (f Fields) unpackBasicType(reader io.Reader, fieldValue reflect.Value, field *Field, fieldLength int, options *Options) error {
	resolvedType := field.Type.Resolve(options)
	if resolvedType == CustomType {
		return fieldValue.Addr().Interface().(CustomBinaryer).Unpack(reader, fieldLength, options)
	}

	dataSize := fieldLength * resolvedType.Size()
	buffer := unpackBasicTypeSlicePool.GetSlice(dataSize)

	if _, err := io.ReadFull(reader, buffer); err != nil {
		return err
	}

	return field.Unpack(buffer[:dataSize], fieldValue, fieldLength, options)
}

// Unpack 从 Reader 中读取数据并解包到字段集合中
// 支持基本类型、结构体、切片和自定义类型
func (f Fields) Unpack(reader io.Reader, structValue reflect.Value, options *Options) error {
	// 解引用指针，直到获取到非指针类型
	for structValue.Kind() == reflect.Ptr {
		structValue = structValue.Elem()
	}

	for i, field := range f {
		if field == nil {
			continue
		}

		fieldValue := structValue.Field(i)
		fieldLength := field.Length
		if field.Sizefrom != nil {
			fieldLength = f.sizefrom(structValue, field.Sizefrom)
		}

		if fieldValue.Kind() == reflect.Ptr && !fieldValue.Elem().IsValid() {
			fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
		}

		if field.Type == Struct {
			if err := f.unpackStruct(reader, fieldValue, field, fieldLength, options); err != nil {
				return err
			}
		} else {
			if err := f.unpackBasicType(reader, fieldValue, field, fieldLength, options); err != nil {
				return err
			}
		}
	}
	return nil
}
