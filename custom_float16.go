package struc

import (
	"encoding/binary"
	"io"
	"math"
	"strconv"
	"sync"
)

// Float16 表示一个16位浮点数。
// 内部使用 float64 存储以获得更好的计算精度，
// 但在序列化和反序列化时使用16位格式。
//
// Float16 represents a 16-bit floating-point number.
// It is stored internally as a float64 for better precision during calculations,
// but serializes to/from a 16-bit format.
//
// 格式 (IEEE 754-2008 binary16):
// 1 位: 符号位
// 5 位: 指数位
// 10 位: 小数位
//
// Format (IEEE 754-2008 binary16):
// 1 bit  : Sign bit
// 5 bits : Exponent
// 10 bits: Fraction
type Float16 float64

// float16BufferPool 为 Float16 操作提供线程安全的缓冲池
// float16BufferPool provides a thread-safe buffer pool for Float16 operations
var float16BufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 2)
	},
}

// Pack 将 Float16 值序列化为16位二进制格式。
// 二进制格式遵循 IEEE 754-2008 binary16 规范。
//
// Pack serializes the Float16 value into a 16-bit binary format.
// The binary format follows the IEEE 754-2008 binary16 specification.
func (f *Float16) Pack(p []byte, opt *Options) (int, error) {
	// 检查缓冲区大小是否足够
	// Check if buffer size is sufficient
	if len(p) < 2 {
		return 0, io.ErrShortBuffer
	}

	// 获取字节序，如果未指定则使用大端序
	// Get byte order, use big-endian if not specified
	order := opt.Order
	if order == nil {
		order = binary.BigEndian
	}

	// 获取符号位：负数为1，正数为0
	// Get sign bit: 1 for negative, 0 for positive
	sign := uint16(0)
	if *f < 0 {
		sign = 1
	}

	var frac, exp uint16
	val := float64(*f)

	// 处理特殊值：无穷大、NaN、负无穷大和零
	// Handle special values: infinity, NaN, negative infinity, and zero
	switch {
	case math.IsInf(val, 0):
		// 正无穷大：指数全1，小数为0
		// Positive infinity: all ones in exponent, zero in fraction
		exp = 0x1f
		frac = 0
	case math.IsNaN(val):
		// NaN：指数全1，小数非0
		// NaN: all ones in exponent, non-zero in fraction
		exp = 0x1f
		frac = 1
	case math.IsInf(val, -1):
		// 负无穷大：指数全1，小数为0，符号为1
		// Negative infinity: all ones in exponent, zero in fraction, sign bit 1
		exp = 0x1f
		frac = 0
		sign = 1
	case val == 0:
		// 处理正零和负零
		// Handle both positive and negative zero
		if math.Signbit(val) {
			sign = 1
		}
	default:
		// 将 float64 转换为 float16 格式
		// Convert from float64 to float16 format
		bits := math.Float64bits(val)
		// 提取指数位
		// Extract exponent bits
		exp64 := (bits >> 52) & 0x7ff
		if exp64 != 0 {
			// 调整指数偏移：从float64的1023调整到float16的15
			// Adjust exponent bias from float64's 1023 to float16's 15
			exp = uint16((exp64 - 1023 + 15) & 0x1f)
		}
		// 提取小数位并舍入到10位
		// Extract fraction bits and round to 10 bits
		frac = uint16((bits >> 42) & 0x3ff)
	}

	// 组合符号位、指数位和小数位
	// Combine sign bit, exponent bits and fraction bits
	out := (sign << 15) | (exp << 10) | (frac & 0x3ff)
	order.PutUint16(p, out)
	return 2, nil
}

// Unpack 将16位二进制格式反序列化为 Float16 值。
// 二进制格式遵循 IEEE 754-2008 binary16 规范。
//
// Unpack deserializes a 16-bit binary format into a Float16 value.
// The binary format follows the IEEE 754-2008 binary16 specification.
func (f *Float16) Unpack(r io.Reader, length int, opt *Options) error {
	// 从对象池获取缓冲区
	// Get buffer from pool
	tmp := float16BufferPool.Get().([]byte)
	defer float16BufferPool.Put(tmp)

	// 获取字节序，如果未指定则使用大端序
	// Get byte order, use big-endian if not specified
	order := opt.Order
	if order == nil {
		order = binary.BigEndian
	}

	// 读取2字节数据
	// Read 2 bytes of data
	if _, err := io.ReadFull(r, tmp); err != nil {
		return err
	}

	// 解析16位值
	// Parse 16-bit value
	val := order.Uint16(tmp)
	// 提取符号位、指数位和小数位
	// Extract sign bit, exponent bits and fraction bits
	sign := (val >> 15) & 1
	exp := int16((val >> 10) & 0x1f)
	frac := val & 0x3ff

	// 处理特殊值和常规值
	// Handle special values and regular values
	switch {
	case exp == 0x1f && frac != 0:
		// NaN：指数全1，小数非0
		// NaN: all ones in exponent, non-zero fraction
		*f = Float16(math.NaN())
	case exp == 0x1f:
		// 无穷大：指数全1，小数为0
		// Infinity: all ones in exponent, zero fraction
		*f = Float16(math.Inf(int(sign)*-2 + 1))
	case exp == 0 && frac == 0:
		// 处理带符号的零
		// Handle signed zero
		if sign == 1 {
			*f = Float16(math.Copysign(0, -1))
		} else {
			*f = 0
		}
	default:
		// 转换为 float64 格式
		// Convert to float64 format
		var bits uint64
		// 设置符号位
		// Set sign bit
		bits |= uint64(sign) << 63
		// 设置小数位
		// Set fraction bits
		bits |= uint64(frac) << 42
		if exp > 0 {
			// 调整指数偏移：从float16的15调整到float64的1023
			// Adjust exponent bias from float16's 15 to float64's 1023
			bits |= uint64(exp-15+1023) << 52
		}
		*f = Float16(math.Float64frombits(bits))
	}
	return nil
}

// Size 返回 Float16 的字节大小。
// Size returns the size of Float16 in bytes.
func (f *Float16) Size(opt *Options) int {
	return 2
}

// String 返回 Float16 值的字符串表示。
// String returns a string representation of the Float16 value.
func (f *Float16) String() string {
	return strconv.FormatFloat(float64(*f), 'g', -1, 32)
}
