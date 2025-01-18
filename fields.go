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
//
// unpackBasicTypeSlicePool is a globally shared byte block instance
// Used for sharing byte slices within the unpackBasicType method to reduce memory allocations
var unpackBasicTypeSlicePool = NewBytesSlicePool(0)

// Fields 是字段切片类型，用于管理结构体的字段集合
// 它提供了字段的序列化、反序列化和大小计算等功能
//
// Fields is a slice of Field pointers, used to manage a collection of struct fields
// It provides functionality for field serialization, deserialization, and size calculation
type Fields []*Field

// SetByteOrder 为所有字段设置字节序
// 这会影响字段值的二进制表示方式
//
// SetByteOrder sets the byte order for all fields
// This affects how field values are represented in binary
func (f Fields) SetByteOrder(byteOrder binary.ByteOrder) {
	for _, field := range f {
		if field != nil {
			field.ByteOrder = byteOrder
		}
	}
}

// String 返回字段集合的字符串表示
// 主要用于调试和日志记录
//
// String returns a string representation of the fields collection
// Primarily used for debugging and logging
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
//
// Sizeof calculates the total size of fields collection in memory (in bytes)
// Takes into account alignment and padding requirements
func (f Fields) Sizeof(structValue reflect.Value, options *Options) int {
	// 解引用所有指针，获取实际的结构体值
	// Dereference all pointers to get the actual struct value
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
//
// sizefrom determines the length of a slice or array based on a referenced field's value
// Supports both signed and unsigned integer types for length fields
func (f Fields) sizefrom(structValue reflect.Value, fieldIndex []int) int {
	// 获取长度字段的值
	// Get the value of the length field
	lengthField := structValue.FieldByIndex(fieldIndex)

	// 根据字段类型处理不同的整数类型
	// Handle different integer types based on field kind
	switch lengthField.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// 处理有符号整数类型
		// Handle signed integer types
		return int(lengthField.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		// 处理无符号整数类型
		// Handle unsigned integer types
		lengthValue := int(lengthField.Uint())
		// 防止出现异常截断
		// Prevent abnormal truncation
		if lengthValue < 0 {
			return 0
		}
		return lengthValue
	default:
		// 如果字段类型不是整数，抛出异常
		// Throw panic if field type is not integer
		fieldName := structValue.Type().FieldByIndex(fieldIndex).Name
		panic(fmt.Sprintf("sizeof field %T.%s not an integer type", structValue.Interface(), fieldName))
	}
}

// Pack 将字段集合打包到字节缓冲区中
// 支持基本类型、结构体、切片和自定义类型
//
// Pack serializes the fields collection into a byte buffer
// Supports basic types, structs, slices and custom types
func (f Fields) Pack(buffer []byte, structValue reflect.Value, options *Options) (int, error) {
	// 解引用指针，直到获取到非指针类型
	// Dereference pointers until we get a non-pointer type
	for structValue.Kind() == reflect.Ptr {
		structValue = structValue.Elem()
	}

	position := 0 // 当前缓冲区位置 / Current buffer position

	// 遍历所有字段进行打包
	// Iterate through all fields for packing
	for i, field := range f {
		if field == nil {
			continue
		}

		// 获取字段值和长度
		// Get field value and length
		fieldValue := structValue.Field(i)
		fieldLength := field.Length

		// 处理动态长度字段
		// Handle dynamic length fields
		if field.Sizefrom != nil {
			fieldLength = f.sizefrom(structValue, field.Sizefrom)
		}
		if fieldLength <= 0 && field.IsSlice {
			fieldLength = fieldValue.Len()
		}

		// 处理 sizeof 字段
		// Handle sizeof fields
		if field.Sizeof != nil {
			// 获取引用字段的长度
			// Get the length of referenced field
			sizeofLength := structValue.FieldByIndex(field.Sizeof).Len()

			// 根据字段类型设置长度值
			// Set length value based on field type
			switch field.kind {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				fieldValue.SetInt(int64(sizeofLength))
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				fieldValue.SetUint(uint64(sizeofLength))
			default:
				panic(fmt.Sprintf("sizeof field is not int or uint type: %s, %s", field.Name, fieldValue.Type()))
			}
		}

		// 打包字段值并更新位置
		// Pack field value and update position
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
//
// Release releases all Field objects in the Fields slice
// Used for memory management and resource cleanup
func (f Fields) Release() {
	releaseFields(f)
}

// unpackStruct 处理结构体类型的解包
// unpackStruct handles unpacking of struct types
func (f Fields) unpackStruct(reader io.Reader, fieldValue reflect.Value, field *Field, fieldLength int, options *Options) error {
	if field.IsSlice {
		return f.unpackStructSlice(reader, fieldValue, fieldLength, field.IsArray, options)
	}
	return f.unpackSingleStruct(reader, fieldValue, options)
}

// unpackStructSlice 处理结构体切片的解包
// unpackStructSlice handles unpacking of struct slices
func (f Fields) unpackStructSlice(reader io.Reader, fieldValue reflect.Value, fieldLength int, isArray bool, options *Options) error {
	// 创建切片值，如果是数组则使用原值
	// Create slice value, use original value if it's an array
	sliceValue := fieldValue
	if !isArray {
		sliceValue = reflect.MakeSlice(fieldValue.Type(), fieldLength, fieldLength)
	}

	// 遍历处理每个元素
	// Process each element
	for i := 0; i < fieldLength; i++ {
		elementValue := sliceValue.Index(i)
		// 解析元素的字段
		// Parse fields of the element
		fields, err := parseFields(elementValue)
		if err != nil {
			return err
		}
		// 解包元素值
		// Unpack element value
		if err := fields.Unpack(reader, elementValue, options); err != nil {
			return err
		}
	}

	// 如果不是数组，设置切片值
	// If not array, set the slice value
	if !isArray {
		fieldValue.Set(sliceValue)
	}
	return nil
}

// unpackSingleStruct 处理单个结构体的解包
// unpackSingleStruct handles unpacking of a single struct
func (f Fields) unpackSingleStruct(reader io.Reader, fieldValue reflect.Value, options *Options) error {
	fields, err := parseFields(fieldValue)
	if err != nil {
		return err
	}
	return fields.Unpack(reader, fieldValue, options)
}

// unpackBasicType 处理基本类型和自定义类型的解包
// unpackBasicType handles unpacking of basic and custom types
func (f Fields) unpackBasicType(reader io.Reader, fieldValue reflect.Value, field *Field, fieldLength int, options *Options) error {
	// 解析类型
	// Resolve type
	resolvedType := field.Type.Resolve(options)
	if resolvedType == CustomType {
		// 处理自定义类型
		// Handle custom type
		return fieldValue.Addr().Interface().(CustomBinaryer).Unpack(reader, fieldLength, options)
	}

	// 计算数据大小并分配缓冲区
	// Calculate data size and allocate buffer
	dataSize := fieldLength * resolvedType.Size()
	buffer := unpackBasicTypeSlicePool.GetSlice(dataSize)

	// 从 reader 读取数据
	// Read data from reader
	if _, err := io.ReadFull(reader, buffer); err != nil {
		return err
	}

	// 解包数据到字段值
	// Unpack data into field value
	return field.Unpack(buffer[:dataSize], fieldValue, fieldLength, options)
}

// Unpack 从 Reader 中读取数据并解包到字段集合中
// 支持基本类型、结构体、切片和自定义类型
//
// Unpack deserializes data from a Reader into the fields collection
// Supports basic types, structs, slices and custom types
func (f Fields) Unpack(reader io.Reader, structValue reflect.Value, options *Options) error {
	// 解引用指针，直到获取到非指针类型
	// Dereference pointers until we get a non-pointer type
	for structValue.Kind() == reflect.Ptr {
		structValue = structValue.Elem()
	}

	// 遍历所有字段进行解包
	// Iterate through all fields for unpacking
	for i, field := range f {
		if field == nil {
			continue
		}

		// 获取字段值和长度
		// Get field value and length
		fieldValue := structValue.Field(i)
		fieldLength := field.Length
		if field.Sizefrom != nil {
			fieldLength = f.sizefrom(structValue, field.Sizefrom)
		}

		// 处理指针类型
		// Handle pointer types
		if fieldValue.Kind() == reflect.Ptr && !fieldValue.Elem().IsValid() {
			fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
		}

		// 根据字段类型选择相应的解包方法
		// Choose appropriate unpacking method based on field type
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
