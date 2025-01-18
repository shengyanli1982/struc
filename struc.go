// Package struc implements binary packing and unpacking for Go structs.
// struc 包实现了 Go 结构体的二进制打包和解包功能。
// 它提供了高效的序列化和反序列化方法，支持自定义类型、字节对齐和不同的字节序。
//
// Features:
// - 支持基本类型和复杂结构体的序列化
// - 支持自定义类型的序列化接口
// - 支持字节对齐和字节序控制
// - 提供高性能的内存管理
// - 支持指针和嵌套结构
//
// Features:
// - Serialization of basic types and complex structs
// - Custom type serialization interface
// - Byte alignment and byte order control
// - High-performance memory management
// - Support for pointers and nested structures
package struc

import (
	"fmt"
	"io"
	"reflect"
)

// Pack 使用默认选项将数据打包到写入器中
// 这是一个便捷方法，内部调用 PackWithOptions
//
// Pack packs the data into the writer using default options
// This is a convenience method that calls PackWithOptions internally
func Pack(writer io.Writer, data interface{}) error {
	return PackWithOptions(writer, data, nil)
}

// PackWithOptions 使用指定的选项将数据打包到写入器中
// 支持自定义选项，如字节对齐和字节序
//
// PackWithOptions packs the data into the writer using specified options
// Supports custom options like byte alignment and byte order
func PackWithOptions(writer io.Writer, data interface{}, options *Options) error {
	// 使用默认选项或验证自定义选项
	// Use default options or validate custom options
	if options == nil {
		options = defaultPackingOptions
	}
	if err := options.Validate(); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}

	// 准备数据进行打包
	// Prepare data for packing
	value, packer, err := prepareValueForPacking(data)
	if err != nil {
		return fmt.Errorf("preparation failed: %w", err)
	}

	// 将字符串转换为字节切片以统一处理
	// Convert string to byte slice for uniform handling
	if value.Type().Kind() == reflect.String {
		value = value.Convert(reflect.TypeOf([]byte{}))
	}

	// 预分配精确大小的缓冲区
	// Pre-allocate buffer with exact size
	bufferSize := packer.Sizeof(value, options)
	buffer := make([]byte, bufferSize)

	// 打包数据到缓冲区
	// Pack data into buffer
	if _, err := packer.Pack(buffer, value, options); err != nil {
		return fmt.Errorf("packing failed: %w", err)
	}

	// 写入数据到输出流
	// Write data to output stream
	if _, err = writer.Write(buffer); err != nil {
		return fmt.Errorf("writing failed: %w", err)
	}

	return nil
}

// Unpack 使用默认选项从读取器中解包数据
// 这是一个便捷方法，内部调用 UnpackWithOptions
//
// Unpack unpacks the data from the reader using default options
// This is a convenience method that calls UnpackWithOptions internally
func Unpack(reader io.Reader, data interface{}) error {
	return UnpackWithOptions(reader, data, nil)
}

// UnpackWithOptions 使用指定的选项从读取器中解包数据
// 支持自定义选项，如字节对齐和字节序
//
// UnpackWithOptions unpacks the data from the reader using specified options
// Supports custom options like byte alignment and byte order
func UnpackWithOptions(reader io.Reader, data interface{}, options *Options) error {
	// 使用默认选项或验证自定义选项
	// Use default options or validate custom options
	if options == nil {
		options = defaultPackingOptions
	}
	if err := options.Validate(); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}

	// 准备数据进行解包
	// Prepare data for unpacking
	value, packer, err := prepareValueForPacking(data)
	if err != nil {
		return fmt.Errorf("preparation failed: %w", err)
	}

	return packer.Unpack(reader, value, options)
}

// Sizeof 使用默认选项返回打包数据的大小
// 这是一个便捷方法，内部调用 SizeofWithOptions
//
// Sizeof returns the size of packed data using default options
// This is a convenience method that calls SizeofWithOptions internally
func Sizeof(data interface{}) (int, error) {
	return SizeofWithOptions(data, nil)
}

// SizeofWithOptions 使用指定的选项返回打包数据的大小
// 支持自定义选项，如字节对齐和字节序
//
// SizeofWithOptions returns the size of packed data using specified options
// Supports custom options like byte alignment and byte order
func SizeofWithOptions(data interface{}, options *Options) (int, error) {
	// 使用默认选项或验证自定义选项
	// Use default options or validate custom options
	if options == nil {
		options = defaultPackingOptions
	}
	if err := options.Validate(); err != nil {
		return 0, fmt.Errorf("invalid options: %w", err)
	}

	// 准备数据进行大小计算
	// Prepare data for size calculation
	value, packer, err := prepareValueForPacking(data)
	if err != nil {
		return 0, fmt.Errorf("preparation failed: %w", err)
	}

	return packer.Sizeof(value, options), nil
}

// prepareValueForPacking 准备一个值用于打包或解包
// 处理指针解引用、类型检查和打包器选择
//
// prepareValueForPacking prepares a value for packing or unpacking
// Handles pointer dereferencing, type checking, and packer selection
func prepareValueForPacking(data interface{}) (reflect.Value, Packer, error) {
	if data == nil {
		return reflect.Value{}, nil, fmt.Errorf("cannot pack/unpack nil data")
	}

	// 获取数据的反射值
	// Get the reflection value of the data
	value := reflect.ValueOf(data)

	// 解引用指针直到获取非指针类型
	// Dereference pointers until we get a non-pointer type
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

	// 根据值类型选择合适的打包器
	// Choose appropriate packer based on value type
	switch value.Kind() {
	case reflect.Struct:
		// 解析结构体字段并创建字段打包器
		// Parse struct fields and create field packer
		if fields, err := parseFields(value); err != nil {
			return reflect.Value{}, nil, fmt.Errorf("failed to parse fields: %w", err)
		} else {
			// 缓存解析的字段以供将来使用
			// Cache parsed fields for future use
			packer = fields
		}
	default:
		if !value.IsValid() {
			return reflect.Value{}, nil, fmt.Errorf("invalid reflect.Value for %+v", data)
		}
		// 处理自定义类型和基本类型
		// Handle custom types and basic types
		if customPacker, ok := data.(Custom); ok {
			// 使用自定义类型的打包器
			// Use custom type packer
			packer = customFallback{customPacker}
		} else {
			// 使用默认的二进制打包器
			// Use default binary packer
			packer = binaryFallback(value)
		}
	}

	return value, packer, err
}
