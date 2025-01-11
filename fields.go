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
	lengthField := structValue.FieldByIndex(fieldIndex)
	switch lengthField.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return int(lengthField.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		lengthValue := int(lengthField.Uint())
		// 所有内置数组长度类型都是原生 int
		// 这里防止出现异常截断
		// all the builtin array length types are native int
		// this guards against weird truncation
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

		// 获取字段值
		// Get field value
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
			sizeofLength := structValue.FieldByIndex(field.Sizeof).Len()
			switch field.kind {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				// 创建新的整数值以避免修改原结构体
				// Create new integer value to avoid modifying original struct
				fieldValue = reflect.New(fieldValue.Type()).Elem()
				fieldValue.SetInt(int64(sizeofLength))
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				fieldValue = reflect.New(fieldValue.Type()).Elem()
				fieldValue.SetUint(uint64(sizeofLength))
			default:
				panic(fmt.Sprintf("sizeof field is not int or uint type: %s, %s", field.Name, fieldValue.Type()))
			}
		}

		// 打包字段值
		// Pack field value
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

// Clone 返回 Fields 切片的浅拷贝
// 由于 Field 对象是不可变的（解析后不会修改），所以可以安全地共享
//
// Clone returns a shallow copy of Fields slice
// Since Field objects are immutable (won't be modified after parsing), they can be safely shared
//
// 总结：
// 1. Field 对象在创建后是不可变的
// 2. 所有操作都是只读的
// 3. 多个 Fields 可以安全地共享 Field 对象
// 4. 浅复制可以提高性能并减少内存使用
// 5. 不可变性保证了并发安全
//
// Summary:
// 1. Field objects are immutable after creation
// 2. All operations are read-only
// 3. Multiple Fields can safely share Field objects
// 4. Shallow copying improves performance and reduces memory usage
// 5. Immutability ensures thread safety
func (f Fields) Clone() Fields {
	if f == nil {
		return nil
	}
	// 直接复制切片，共享底层的 Field 对象
	// Copy the slice directly, sharing the underlying Field objects
	newFields := make(Fields, len(f))
	copy(newFields, f)
	return newFields
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

	// 创建临时缓冲区
	// Create temporary buffer
	var tempBuffer [8]byte
	var buffer []byte

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

		// 处理结构体类型
		// Handle struct types
		if field.Type == Struct {
			if field.IsSlice {
				// 处理结构体切片
				// Handle struct slices
				sliceValue := fieldValue
				if !field.IsArray {
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
				if !field.IsArray {
					fieldValue.Set(sliceValue)
				}
			} else {
				// 处理单个结构体
				// Handle single struct
				fields, err := parseFields(fieldValue)
				if err != nil {
					return err
				}
				if err := fields.Unpack(reader, fieldValue, options); err != nil {
					return err
				}
			}
			continue
		} else {
			// 处理基本类型和自定义类型
			// Handle basic types and custom types
			resolvedType := field.Type.Resolve(options)
			if resolvedType == CustomType {
				if err := fieldValue.Addr().Interface().(Custom).Unpack(reader, fieldLength, options); err != nil {
					return err
				}
			} else {
				// 读取数据到缓冲区
				// Read data into buffer
				dataSize := fieldLength * field.Type.Resolve(options).Size()
				if dataSize < 8 {
					buffer = tempBuffer[:dataSize]
				} else {
					buffer = make([]byte, dataSize)
				}

				// 从 reader 读取数据
				// Read data from reader
				if _, err := io.ReadFull(reader, buffer); err != nil {
					return err
				}

				// 解包数据到字段值
				// Unpack data into field value
				if err := field.Unpack(buffer[:dataSize], fieldValue, fieldLength, options); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
