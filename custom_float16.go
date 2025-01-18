package struc

import (
	"encoding/binary"
	"io"
	"math"
	"strconv"
)

// Float16 表示一个16位浮点数
// 内部使用 float64 存储以获得更好的计算精度，
// 但在序列化和反序列化时使用16位格式
//
// 格式 (IEEE 754-2008 binary16):
// 1 位: 符号位 (0=正数, 1=负数)
// 5 位: 指数位 (偏移值为15)
// 10 位: 小数位 (隐含前导1)
//
// Float16 represents a 16-bit floating-point number
// It is stored internally as a float64 for better precision during calculations,
// but serializes to/from a 16-bit format
//
// Format (IEEE 754-2008 binary16):
// 1 bit  : Sign bit (0=positive, 1=negative)
// 5 bits : Exponent (bias of 15)
// 10 bits: Fraction (implied leading 1)
type Float16 float64

// float16SlicePool 为 Float16 操作提供线程安全的缓冲池
// 使用 BytesSlicePool 管理共享字节切片，减少内存分配
//
// float16SlicePool provides a thread-safe buffer pool for Float16 operations
// Uses BytesSlicePool to manage shared byte slices, reducing memory allocations
var float16SlicePool = &BytesSlicePool{
	bytes:  make([]byte, 4096),
	offset: 0,
}

// Pack 将 Float16 值序列化为16位二进制格式
// 二进制格式遵循 IEEE 754-2008 binary16 规范
// 支持特殊值：±0, ±∞, NaN
//
// Pack serializes the Float16 value into a 16-bit binary format
// The binary format follows the IEEE 754-2008 binary16 specification
// Supports special values: ±0, ±∞, NaN
func (f *Float16) Pack(buffer []byte, options *Options) (int, error) {
	// 检查缓冲区大小是否足够
	// Check if buffer size is sufficient
	if len(buffer) < 2 {
		return 0, io.ErrShortBuffer
	}

	// 获取字节序，如果未指定则使用大端序
	// Get byte order, use big-endian if not specified
	byteOrder := options.Order
	if byteOrder == nil {
		byteOrder = binary.BigEndian
	}

	// 获取符号位：负数为1，正数为0
	// Get sign bit: 1 for negative, 0 for positive
	signBit := uint16(0)
	if *f < 0 {
		signBit = 1
	}

	var fractionBits, exponentBits uint16
	value := float64(*f)

	// 处理特殊值：无穷大、NaN、负无穷大和零
	// Handle special values: infinity, NaN, negative infinity, and zero
	switch {
	case math.IsInf(value, 0):
		// 正无穷大：指数全1，小数为0
		// Positive infinity: all ones in exponent, zero in fraction
		exponentBits = 0x1f
		fractionBits = 0
	case math.IsNaN(value):
		// NaN：指数全1，小数非0
		// NaN: all ones in exponent, non-zero in fraction
		exponentBits = 0x1f
		fractionBits = 1
	case math.IsInf(value, -1):
		// 负无穷大：指数全1，小数为0，符号为1
		// Negative infinity: all ones in exponent, zero in fraction, sign bit 1
		exponentBits = 0x1f
		fractionBits = 0
		signBit = 1
	case value == 0:
		// 处理正零和负零
		// Handle both positive and negative zero
		if math.Signbit(value) {
			signBit = 1
		}
	default:
		// 将 float64 转换为 float16 格式
		// Convert from float64 to float16 format
		bits64 := math.Float64bits(value)
		// 提取指数位
		// Extract exponent bits
		exponent64 := (bits64 >> 52) & 0x7ff
		if exponent64 != 0 {
			// 调整指数偏移：从float64的1023调整到float16的15
			// Adjust exponent bias from float64's 1023 to float16's 15
			exponentBits = uint16((exponent64 - 1023 + 15) & 0x1f)
		}
		// 提取小数位并舍入到10位
		// Extract fraction bits and round to 10 bits
		fractionBits = uint16((bits64 >> 42) & 0x3ff)
	}

	// 组合符号位、指数位和小数位
	// Combine sign bit, exponent bits and fraction bits
	result := (signBit << 15) | (exponentBits << 10) | (fractionBits & 0x3ff)
	byteOrder.PutUint16(buffer, result)
	return 2, nil
}

// Unpack 将16位二进制格式反序列化为 Float16 值
// 二进制格式遵循 IEEE 754-2008 binary16 规范
// 支持特殊值：±0, ±∞, NaN
//
// Unpack deserializes a 16-bit binary format into a Float16 value
// The binary format follows the IEEE 754-2008 binary16 specification
// Supports special values: ±0, ±∞, NaN
func (f *Float16) Unpack(reader io.Reader, length int, options *Options) error {
	// 从对象池获取缓冲区
	// Get buffer from pool
	buffer := float16SlicePool.GetSlice(2)

	// 获取字节序，如果未指定则使用大端序
	// Get byte order, use big-endian if not specified
	byteOrder := options.Order
	if byteOrder == nil {
		byteOrder = binary.BigEndian
	}

	// 读取2字节数据
	// Read 2 bytes of data
	if _, err := io.ReadFull(reader, buffer); err != nil {
		return err
	}

	// 解析16位值
	// Parse 16-bit value
	value := byteOrder.Uint16(buffer)
	// 提取符号位、指数位和小数位
	// Extract sign bit, exponent bits and fraction bits
	signBit := (value >> 15) & 1
	exponentBits := int16((value >> 10) & 0x1f)
	fractionBits := value & 0x3ff

	// 处理特殊值和常规值
	// Handle special values and regular values
	switch {
	case exponentBits == 0x1f && fractionBits != 0:
		// NaN：指数全1，小数非0
		// NaN: all ones in exponent, non-zero fraction
		*f = Float16(math.NaN())
	case exponentBits == 0x1f:
		// 无穷大：指数全1，小数为0
		// Infinity: all ones in exponent, zero fraction
		*f = Float16(math.Inf(int(signBit)*-2 + 1))
	case exponentBits == 0 && fractionBits == 0:
		// 处理带符号的零
		// Handle signed zero
		if signBit == 1 {
			*f = Float16(math.Copysign(0, -1))
		} else {
			*f = 0
		}
	default:
		// 转换为 float64 格式
		// Convert to float64 format
		var bits64 uint64
		// 设置符号位
		// Set sign bit
		bits64 |= uint64(signBit) << 63
		// 设置小数位
		// Set fraction bits
		bits64 |= uint64(fractionBits) << 42
		if exponentBits > 0 {
			// 调整指数偏移：从float16的15调整到float64的1023
			// Adjust exponent bias from float16's 15 to float64's 1023
			bits64 |= uint64(exponentBits-15+1023) << 52
		}
		*f = Float16(math.Float64frombits(bits64))
	}
	return nil
}

// Size 返回 Float16 的字节大小
// 固定返回2，因为 Float16 总是占用2字节
//
// Size returns the size of Float16 in bytes
// Always returns 2, as Float16 always occupies 2 bytes
func (f *Float16) Size(options *Options) int {
	return 2
}

// String 返回 Float16 值的字符串表示
// 使用 'g' 格式和32位精度进行格式化
//
// String returns a string representation of the Float16 value
// Uses 'g' format and 32-bit precision for formatting
func (f *Float16) String() string {
	return strconv.FormatFloat(float64(*f), 'g', -1, 32)
}
