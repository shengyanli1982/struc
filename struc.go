// Package struc implements binary packing and unpacking for Go structs.
// 包 struc 实现了 Go 结构体的二进制打包和解包功能。
package struc

import (
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
	"sync"
)

// Options defines the configuration options for packing and unpacking.
// Options 定义了打包和解包的配置选项。
type Options struct {
	// ByteAlign specifies the byte alignment for packed fields.
	// ByteAlign 指定打包字段的字节对齐方式。
	ByteAlign int

	// PtrSize specifies the size of pointers in bits (8, 16, 32, or 64).
	// PtrSize 指定指针的大小（以位为单位，可以是 8、16、32 或 64）。
	PtrSize int

	// Order specifies the byte order (little or big endian).
	// Order 指定字节序（小端或大端）。
	Order binary.ByteOrder
}

// cache for parsed fields to improve performance
// 缓存已解析的字段以提高性能
var (
	fieldsCache sync.Map // map[reflect.Type]Packer
)

// Validate checks if the options are valid.
// Validate 检查选项是否有效。
func (o *Options) Validate() error {
	if o.PtrSize == 0 {
		o.PtrSize = 32
	} else {
		switch o.PtrSize {
		case 8, 16, 32, 64:
		default:
			return fmt.Errorf("invalid Options.PtrSize: %d (must be 8, 16, 32, or 64)", o.PtrSize)
		}
	}
	return nil
}

// Default options instance to avoid repeated allocations
// 默认选项实例，避免重复分配
var emptyOptions = &Options{}

func init() {
	// Fill default values to avoid data race
	// 填充默认值以避免数据竞争
	_ = emptyOptions.Validate()
}

// prep prepares a value for packing or unpacking.
// prep 准备一个值用于打包或解包。
func prep(data interface{}) (reflect.Value, Packer, error) {
	if data == nil {
		return reflect.Value{}, nil, fmt.Errorf("cannot pack/unpack nil data")
	}

	value := reflect.ValueOf(data)

	// Dereference pointers until we get to a non-pointer type
	// 解引用指针直到我们得到一个非指针类型
	for value.Kind() == reflect.Ptr {
		next := value.Elem().Kind()
		if next == reflect.Struct || next == reflect.Ptr {
			value = value.Elem()
		} else {
			break
		}
	}

	// Check if we have a cached packer for this type
	// 检查是否有此类型的缓存打包器
	if packer, ok := fieldsCache.Load(value.Type()); ok {
		return value, packer.(Packer), nil
	}

	var packer Packer
	var err error

	switch value.Kind() {
	case reflect.Struct:
		if fields, err := parseFields(value); err != nil {
			return reflect.Value{}, nil, fmt.Errorf("failed to parse fields: %w", err)
		} else {
			packer = fields
			// Cache the parsed fields for future use
			// 缓存解析的字段以供将来使用
			fieldsCache.Store(value.Type(), fields)
		}
	default:
		if !value.IsValid() {
			return reflect.Value{}, nil, fmt.Errorf("invalid reflect.Value for %+v", data)
		}
		if c, ok := data.(Custom); ok {
			packer = customFallback{c}
		} else {
			packer = binaryFallback(value)
		}
	}

	return value, packer, err
}

// Pack packs the data into the writer using default options.
// Pack 使用默认选项将数据打包到写入器中。
func Pack(w io.Writer, data interface{}) error {
	return PackWithOptions(w, data, nil)
}

// PackWithOptions packs the data into the writer using the specified options.
// PackWithOptions 使用指定的选项将数据打包到写入器中。
func PackWithOptions(w io.Writer, data interface{}, options *Options) error {
	if options == nil {
		options = emptyOptions
	}
	if err := options.Validate(); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}

	val, packer, err := prep(data)
	if err != nil {
		return fmt.Errorf("preparation failed: %w", err)
	}

	// Convert string to []byte for consistent handling
	// 将字符串转换为 []byte 以便统一处理
	if val.Type().Kind() == reflect.String {
		val = val.Convert(reflect.TypeOf([]byte{}))
	}

	// Pre-allocate buffer with exact size
	// 预分配精确大小的缓冲区
	size := packer.Sizeof(val, options)
	buf := make([]byte, size)

	if _, err := packer.Pack(buf, val, options); err != nil {
		return fmt.Errorf("packing failed: %w", err)
	}

	if _, err = w.Write(buf); err != nil {
		return fmt.Errorf("writing failed: %w", err)
	}

	return nil
}

// Unpack unpacks the data from the reader using default options.
// Unpack 使用默认选项从读取器中解包数据。
func Unpack(r io.Reader, data interface{}) error {
	return UnpackWithOptions(r, data, nil)
}

// UnpackWithOptions unpacks the data from the reader using the specified options.
// UnpackWithOptions 使用指定的选项从读取器中解包数据。
func UnpackWithOptions(r io.Reader, data interface{}, options *Options) error {
	if options == nil {
		options = emptyOptions
	}
	if err := options.Validate(); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}

	val, packer, err := prep(data)
	if err != nil {
		return fmt.Errorf("preparation failed: %w", err)
	}

	return packer.Unpack(r, val, options)
}

// Sizeof returns the size of the packed data using default options.
// Sizeof 使用默认选项返回打包数据的大小。
func Sizeof(data interface{}) (int, error) {
	return SizeofWithOptions(data, nil)
}

// SizeofWithOptions returns the size of the packed data using the specified options.
// SizeofWithOptions 使用指定的选项返回打包数据的大小。
func SizeofWithOptions(data interface{}, options *Options) (int, error) {
	if options == nil {
		options = emptyOptions
	}
	if err := options.Validate(); err != nil {
		return 0, fmt.Errorf("invalid options: %w", err)
	}

	val, packer, err := prep(data)
	if err != nil {
		return 0, fmt.Errorf("preparation failed: %w", err)
	}

	return packer.Sizeof(val, options), nil
}
