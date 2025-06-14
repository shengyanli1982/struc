// Package struc 实现了 Go 结构体的二进制打包和解包功能。
// 它提供了高效的序列化和反序列化方法，支持自定义类型、字节对齐和不同的字节序。
//
// Features:
// - 支持基本类型和复杂结构体的序列化
// - 支持自定义类型的序列化接口
// - 支持字节对齐和字节序控制
// - 提供高性能的内存管理
// - 支持指针和嵌套结构
package struc

import (
	"fmt"
	"io"
	"reflect"
)

// packingSlicePool 是用于打包和解包的切片池
// 用于存储和重用字节切片, 提高性能
var packingSlicePool = NewBytesSlicePool(0)

// Pack 使用默认选项将数据打包到写入器中
// 这是一个便捷方法，内部调用 PackWithOptions
func Pack(writer io.Writer, data interface{}) error {
	return PackWithOptions(writer, data, nil)
}

// PackWithOptions 使用指定的选项将数据打包到写入器中
// 支持自定义选项，如字节对齐和字节序
func PackWithOptions(writer io.Writer, data interface{}, options *Options) error {
	if options == nil {
		options = defaultPackingOptions
	}
	if err := options.Validate(); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}

	value, packer, err := prepareValueForPacking(data)
	if err != nil {
		return fmt.Errorf("preparation failed: %w", err)
	}

	if value.Type().Kind() == reflect.String {
		value = value.Convert(reflect.TypeOf([]byte{}))
	}

	bufferSize := packer.Sizeof(value, options)
	buffer := packingSlicePool.GetSlice(bufferSize)

	if _, err := packer.Pack(buffer, value, options); err != nil {
		return fmt.Errorf("packing failed: %w", err)
	}

	if _, err = writer.Write(buffer); err != nil {
		return fmt.Errorf("writing failed: %w", err)
	}

	return nil
}

// Unpack 使用默认选项从读取器中解包数据
// 这是一个便捷方法，内部调用 UnpackWithOptions
func Unpack(reader io.Reader, data interface{}) error {
	return UnpackWithOptions(reader, data, nil)
}

// UnpackWithOptions 使用指定的选项从读取器中解包数据
// 支持自定义选项，如字节对齐和字节序
func UnpackWithOptions(reader io.Reader, data interface{}, options *Options) error {
	if options == nil {
		options = defaultPackingOptions
	}
	if err := options.Validate(); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}

	value, packer, err := prepareValueForPacking(data)
	if err != nil {
		return fmt.Errorf("preparation failed: %w", err)
	}

	return packer.Unpack(reader, value, options)
}

// Sizeof 使用默认选项返回打包数据的大小
// 这是一个便捷方法，内部调用 SizeofWithOptions
func Sizeof(data interface{}) (int, error) {
	return SizeofWithOptions(data, nil)
}

// SizeofWithOptions 使用指定的选项返回打包数据的大小
// 支持自定义选项，如字节对齐和字节序
func SizeofWithOptions(data interface{}, options *Options) (int, error) {
	if options == nil {
		options = defaultPackingOptions
	}
	if err := options.Validate(); err != nil {
		return 0, fmt.Errorf("invalid options: %w", err)
	}

	value, packer, err := prepareValueForPacking(data)
	if err != nil {
		return 0, fmt.Errorf("preparation failed: %w", err)
	}

	return packer.Sizeof(value, options), nil
}

// prepareValueForPacking 准备一个值用于打包或解包
// 处理指针解引用、类型检查和打包器选择
func prepareValueForPacking(data interface{}) (reflect.Value, Packer, error) {
	if data == nil {
		return reflect.Value{}, nil, fmt.Errorf("cannot pack/unpack nil data")
	}

	value := reflect.ValueOf(data)

	// 解引用指针直到获取非指针类型
	for value.Kind() == reflect.Ptr {
		next := value.Elem().Kind()
		if next == reflect.Struct || next == reflect.Ptr {
			value = value.Elem()
		} else {
			break
		}
	}

	var packer Packer
	var err error

	switch value.Kind() {
	case reflect.Struct:
		if fields, err := parseFields(value); err != nil {
			return reflect.Value{}, nil, fmt.Errorf("failed to parse fields: %w", err)
		} else {
			packer = fields
		}
	default:
		if !value.IsValid() {
			return reflect.Value{}, nil, fmt.Errorf("invalid reflect.Value for %+v", data)
		}
		if customPacker, ok := data.(CustomBinaryer); ok {
			packer = customBinaryerFallback{customPacker}
		} else {
			packer = binaryFallback(value)
		}
	}

	return value, packer, err
}
