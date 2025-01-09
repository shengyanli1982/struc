package struc

import (
	"encoding/binary"
	"io"
	"math"
	"strconv"
	"sync"
)

// Float16 represents a 16-bit floating-point number.
// It is stored internally as a float64 for better precision during calculations,
// but serializes to/from a 16-bit format.
//
// Format (IEEE 754-2008 binary16):
// 1 bit  : Sign bit
// 5 bits : Exponent
// 10 bits: Fraction
type Float16 float64

// float16BufferPool provides a thread-safe buffer pool for Float16 operations
var float16BufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 2)
	},
}

// Pack serializes the Float16 value into a 16-bit binary format.
// The binary format follows the IEEE 754-2008 binary16 specification.
func (f *Float16) Pack(p []byte, opt *Options) (int, error) {
	if len(p) < 2 {
		return 0, io.ErrShortBuffer
	}

	order := opt.Order
	if order == nil {
		order = binary.BigEndian
	}

	// Get sign bit
	sign := uint16(0)
	if *f < 0 {
		sign = 1
	}

	var frac, exp uint16
	val := float64(*f)

	switch {
	case math.IsInf(val, 0):
		exp = 0x1f
		frac = 0
	case math.IsNaN(val):
		exp = 0x1f
		frac = 1
	case math.IsInf(val, -1):
		exp = 0x1f
		frac = 0
		sign = 1
	case val == 0:
		// Handle both positive and negative zero
		if math.Signbit(val) {
			sign = 1
		}
	default:
		// Convert from float64 to float16 format
		bits := math.Float64bits(val)
		exp64 := (bits >> 52) & 0x7ff
		if exp64 != 0 {
			// Adjust exponent bias from float64 to float16
			exp = uint16((exp64 - 1023 + 15) & 0x1f)
		}
		// Extract fraction bits and round to 10 bits
		frac = uint16((bits >> 42) & 0x3ff)
	}

	// Combine sign, exponent and fraction
	out := (sign << 15) | (exp << 10) | (frac & 0x3ff)
	order.PutUint16(p, out)
	return 2, nil
}

// Unpack deserializes a 16-bit binary format into a Float16 value.
// The binary format follows the IEEE 754-2008 binary16 specification.
func (f *Float16) Unpack(r io.Reader, length int, opt *Options) error {
	// Get buffer from pool
	tmp := float16BufferPool.Get().([]byte)
	defer float16BufferPool.Put(tmp)

	order := opt.Order
	if order == nil {
		order = binary.BigEndian
	}

	if _, err := io.ReadFull(r, tmp); err != nil {
		return err
	}

	val := order.Uint16(tmp)
	sign := (val >> 15) & 1
	exp := int16((val >> 10) & 0x1f)
	frac := val & 0x3ff

	switch {
	case exp == 0x1f && frac != 0:
		*f = Float16(math.NaN())
	case exp == 0x1f:
		*f = Float16(math.Inf(int(sign)*-2 + 1))
	case exp == 0 && frac == 0:
		// Handle signed zero
		if sign == 1 {
			*f = Float16(math.Copysign(0, -1))
		} else {
			*f = 0
		}
	default:
		// Convert to float64 format
		var bits uint64
		bits |= uint64(sign) << 63
		bits |= uint64(frac) << 42
		if exp > 0 {
			bits |= uint64(exp-15+1023) << 52
		}
		*f = Float16(math.Float64frombits(bits))
	}
	return nil
}

// Size returns the size of Float16 in bytes.
func (f *Float16) Size(opt *Options) int {
	return 2
}

// String returns a string representation of the Float16 value.
func (f *Float16) String() string {
	return strconv.FormatFloat(float64(*f), 'g', -1, 32)
}
