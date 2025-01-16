package struc

import (
	"encoding/binary"
	"fmt"
)

// defaultPackingOptions 是默认的打包选项实例
// 用于避免重复分配内存，提高性能
//
// defaultPackingOptions is the default packing options instance
// Used to avoid repeated memory allocations and improve performance
var defaultPackingOptions = &Options{}

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

func init() {
	_ = defaultPackingOptions.Validate()
}
