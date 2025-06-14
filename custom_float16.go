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
var float16SlicePool = NewBytesSlicePool(0)

// Pack 将 Float16 值序列化为16位二进制格式
// 二进制格式遵循 IEEE 754-2008 binary16 规范
// 支持特殊值：±0, ±∞, NaN
//
// Pack serializes the Float16 value into a 16-bit binary format
// The binary format follows the IEEE 754-2008 binary16 specification
// Supports special values: ±0, ±∞, NaN
func (f *Float16) Pack(buffer []byte, options *Options) (int, error) {
	if len(buffer) < 2 {
		return 0, io.ErrShortBuffer
	}

	byteOrder := options.Order
	if byteOrder == nil {
		byteOrder = binary.BigEndian
	}

	// This conversion is based on github.com/stdlib-js/math-float64-to-float16
	// It correctly handles special values and flushes subnormals to zero.
	bits := math.Float64bits(float64(*f))

	sign := uint16((bits >> 48) & 0x8000)
	exp := int((bits >> 52) & 0x07FF) // Use int for exponent to handle negative results
	mant := bits & 0x000FFFFFFFFFFFFF

	var res uint16
	if exp == 0x07FF { // NaN or Inf
		res = sign | 0x7C00
		if mant != 0 { // NaN
			res |= 0x0200 // Ensure mantissa is non-zero
		}
	} else {
		// Re-bias exponent
		exp = exp - 1023 + 15
		if exp >= 0x1F { // Overflow
			res = sign | 0x7C00
		} else if exp <= 0 { // Underflow, flush to zero
			res = sign
		} else { // Normal number
			mant >>= 42
			res = sign | uint16(exp<<10) | uint16(mant)
		}
	}

	byteOrder.PutUint16(buffer, res)
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

	value := byteOrder.Uint16(buffer)

	sign := uint64(value>>15) & 1
	exp16 := (value >> 10) & 0x1f
	mant16 := value & 0x3ff

	var bits64 uint64
	if exp16 == 0x1f { // Inf or NaN
		bits64 = sign<<63 | uint64(0x7ff)<<52
		if mant16 != 0 {
			bits64 |= 1 << 51 // Make it a quiet NaN
		}
	} else if exp16 == 0 { // Zero or subnormal (flushed to zero)
		bits64 = sign << 63
	} else { // Normal
		exp64 := uint64(exp16) + 1023 - 15
		mant64 := uint64(mant16)
		bits64 = sign<<63 | exp64<<52 | mant64<<42
	}

	*f = Float16(math.Float64frombits(bits64))
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
