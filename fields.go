package struc

import (
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
	"strings"
)

// Fields 是字段切片类型，用于管理结构体的字段集合
// Fields is a slice of Field pointers, used to manage a collection of struct fields
type Fields []*Field

// SetByteOrder 为所有字段设置字节序
// SetByteOrder sets the byte order for all fields
func (f Fields) SetByteOrder(order binary.ByteOrder) {
	for _, field := range f {
		if field != nil {
			field.Order = order
		}
	}
}

// String 返回字段集合的字符串表示
// String returns a string representation of the fields collection
func (f Fields) String() string {
	fields := make([]string, len(f))
	for i, field := range f {
		if field != nil {
			fields[i] = field.String()
		}
	}
	return "{" + strings.Join(fields, ", ") + "}"
}

// Sizeof 计算字段集合在内存中的总大小（字节数）
// Sizeof calculates the total size of fields collection in memory (in bytes)
func (f Fields) Sizeof(val reflect.Value, options *Options) int {
	for val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	size := 0
	for i, field := range f {
		if field != nil {
			size += field.Size(val.Field(i), options)
		}
	}
	return size
}

// sizefrom 根据引用字段的值确定切片或数组的长度
// sizefrom determines the length of a slice or array based on a referenced field's value
func (f Fields) sizefrom(val reflect.Value, index []int) int {
	field := val.FieldByIndex(index)
	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return int(field.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n := int(field.Uint())
		// 所有内置数组长度类型都是原生 int
		// 这里防止出现异常截断
		// all the builtin array length types are native int
		// so this guards against weird truncation
		if n < 0 {
			return 0
		}
		return n
	default:
		name := val.Type().FieldByIndex(index).Name
		panic(fmt.Sprintf("sizeof field %T.%s not an integer type", val.Interface(), name))
	}
}

// Pack 将字段集合打包到字节缓冲区中
// Pack serializes the fields collection into a byte buffer
func (f Fields) Pack(buf []byte, val reflect.Value, options *Options) (int, error) {
	// 解引用指针，直到获取到非指针类型
	// Dereference pointers until we get a non-pointer type
	for val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	pos := 0 // 当前缓冲区位置 / Current buffer position

	// 遍历所有字段进行打包
	// Iterate through all fields for packing
	for i, field := range f {
		if field == nil {
			continue
		}

		// 获取字段值
		// Get field value
		v := val.Field(i)
		length := field.Len

		// 处理动态长度字段
		// Handle dynamic length fields
		if field.Sizefrom != nil {
			length = f.sizefrom(val, field.Sizefrom)
		}
		if length <= 0 && field.Slice {
			length = v.Len()
		}

		// 处理 sizeof 字段
		// Handle sizeof fields
		if field.Sizeof != nil {
			length := val.FieldByIndex(field.Sizeof).Len()
			switch field.kind {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				// 创建新的整数值以避免修改原结构体
				// Create new integer value to avoid modifying original struct
				v = reflect.New(v.Type()).Elem()
				v.SetInt(int64(length))
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				v = reflect.New(v.Type()).Elem()
				v.SetUint(uint64(length))
			default:
				panic(fmt.Sprintf("sizeof field is not int or uint type: %s, %s", field.Name, v.Type()))
			}
		}

		// 打包字段值
		// Pack field value
		if n, err := field.Pack(buf[pos:], v, length, options); err != nil {
			return n, err
		} else {
			pos += n
		}
	}
	return pos, nil
}

// Unpack 从 Reader 中读取数据并解包到字段集合中
// Unpack deserializes data from a Reader into the fields collection
func (f Fields) Unpack(r io.Reader, val reflect.Value, options *Options) error {
	// 解引用指针，直到获取到非指针类型
	// Dereference pointers until we get a non-pointer type
	for val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// 创建临时缓冲区
	// Create temporary buffer
	var tmp [8]byte
	var buf []byte

	// 遍历所有字段进行解包
	// Iterate through all fields for unpacking
	for i, field := range f {
		if field == nil {
			continue
		}

		// 获取字段值和长度
		// Get field value and length
		v := val.Field(i)
		length := field.Len
		if field.Sizefrom != nil {
			length = f.sizefrom(val, field.Sizefrom)
		}

		// 处理指针类型
		// Handle pointer types
		if v.Kind() == reflect.Ptr && !v.Elem().IsValid() {
			v.Set(reflect.New(v.Type().Elem()))
		}

		// 处理结构体类型
		// Handle struct types
		if field.Type == Struct {
			if field.Slice {
				// 处理结构体切片
				// Handle struct slices
				vals := v
				if !field.Array {
					vals = reflect.MakeSlice(v.Type(), length, length)
				}
				for i := 0; i < length; i++ {
					v := vals.Index(i)
					fields, err := parseFields(v)
					if err != nil {
						return err
					}
					if err := fields.Unpack(r, v, options); err != nil {
						return err
					}
				}
				if !field.Array {
					v.Set(vals)
				}
			} else {
				// 处理单个结构体
				// Handle single struct
				fields, err := parseFields(v)
				if err != nil {
					return err
				}
				if err := fields.Unpack(r, v, options); err != nil {
					return err
				}
			}
			continue
		} else {
			// 处理基本类型和自定义类型
			// Handle basic types and custom types
			typ := field.Type.Resolve(options)
			if typ == CustomType {
				if err := v.Addr().Interface().(Custom).Unpack(r, length, options); err != nil {
					return err
				}
			} else {
				// 读取数据到缓冲区
				// Read data into buffer
				size := length * field.Type.Resolve(options).Size()
				if size < 8 {
					buf = tmp[:size]
				} else {
					buf = make([]byte, size)
				}
				if _, err := io.ReadFull(r, buf); err != nil {
					return err
				}
				err := field.Unpack(buf[:size], v, length, options)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
