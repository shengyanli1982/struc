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
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
)

// Options 定义了打包和解包的配置选项
// 包含字节对齐、指针大小和字节序等设置
//
// Options defines the configuration options for packing and unpacking
// Contains settings for byte alignment, pointer size, and byte order
type Options struct {
	// ByteAlign 指定打包字段的字节对齐方式
	// 值为 0 表示不进行对齐，其他值表示按该字节数对齐
	//
	// ByteAlign specifies the byte alignment for packed fields
	// 0 means no alignment, other values specify alignment boundary
	ByteAlign int

	// PtrSize 指定指针的大小（以位为单位）
	// 可选值：8、16、32 或 64
	// 默认值：32
	//
	// PtrSize specifies the size of pointers in bits
	// Valid values: 8, 16, 32, or 64
	// Default: 32
	PtrSize int

	// Order 指定字节序（大端或小端）
	// 如果为 nil，则使用大端序
	//
	// Order specifies the byte order (big or little endian)
	// If nil, big-endian is used
	Order binary.ByteOrder
}

// Validate 验证选项的有效性
// 检查指针大小是否合法，并设置默认值
//
// Validate checks if the options are valid
// Verifies pointer size and sets default values
func (o *Options) Validate() error {
	if o.PtrSize == 0 {
		o.PtrSize = 32 // 设置默认指针大小 / Set default pointer size
	} else {
		switch o.PtrSize {
		case 8, 16, 32, 64:
			// 有效的指针大小 / Valid pointer sizes
		default:
			return fmt.Errorf("invalid Options.PtrSize: %d (must be 8, 16, 32, or 64)", o.PtrSize)
		}
	}
	return nil
}

// defaultPackingOptions 是默认的打包选项实例
// 用于避免重复分配内存，提高性能
//
// defaultPackingOptions is the default packing options instance
// Used to avoid repeated memory allocations and improve performance
var defaultPackingOptions = &Options{}

// init 初始化默认打包选项
// 确保在包加载时填充默认值，避免数据竞争
//
// init initializes default packing options
// Ensures default values are filled during package loading to avoid data races
func init() {
	_ = defaultPackingOptions.Validate()
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
			packer = fields
		}
	default:
		if !value.IsValid() {
			return reflect.Value{}, nil, fmt.Errorf("invalid reflect.Value for %+v", data)
		}
		// 处理自定义类型和基本类型
		// Handle custom types and basic types
		if customPacker, ok := data.(Custom); ok {
			packer = customFallback{customPacker}
		} else {
			packer = binaryFallback(value)
		}
	}

	return value, packer, err
}

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
// Sizeof returns the size of the packed data using default options
// This is a convenience method that calls SizeofWithOptions internally
func Sizeof(data interface{}) (int, error) {
	return SizeofWithOptions(data, nil)
}

// SizeofWithOptions 使用指定的选项返回打包数据的大小
// 支持自定义选项，如字节对齐和字节序
//
// SizeofWithOptions returns the size of the packed data using specified options
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

	// 准备数据并计算大小
	// Prepare data and calculate size
	value, packer, err := prepareValueForPacking(data)
	if err != nil {
		return 0, fmt.Errorf("preparation failed: %w", err)
	}

	return packer.Sizeof(value, options), nil
}
