package struc

import (
	"encoding/binary"
	"math"
	"reflect"
	"unsafe"
)

// typedmemmove 是一个底层的内存移动函数
//
//go:linkname typedmemmove runtime.typedmemmove
func typedmemmove(dst unsafe.Pointer, src unsafe.Pointer, size uintptr)

// unsafeSliceHeader 是切片的内部表示
type unsafeSliceHeader struct {
	Data uintptr
	Len  int
	Cap  int
}

// unsafeBytes2String 使用 unsafe 将字节切片转换为字符串
// 避免内存拷贝，提高性能
func unsafeBytes2String(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// unsafeSetSlice 使用 unsafe 直接设置切片的底层数据
// 避免内存拷贝，提高性能
func unsafeSetSlice(fieldValue reflect.Value, buffer []byte, length int) {
	sh := (*unsafeSliceHeader)(unsafe.Pointer(fieldValue.UnsafeAddr()))
	sh.Data = uintptr(unsafe.Pointer(&buffer[0]))
	sh.Len = length
	sh.Cap = length
}

// unsafeSetString 使用 unsafe 将字节切片转换为字符串并设置到字段
// 避免内存拷贝，提高性能
func unsafeSetString(fieldValue reflect.Value, buffer []byte, length int) {
	str := unsafeBytes2String(buffer[:length])
	fieldValue.SetString(str)
}

// unsafeGetUint64 使用 unsafe 直接读取 uint64 值
func unsafeGetUint64(buffer []byte, byteOrder binary.ByteOrder) uint64 {
	if byteOrder == binary.LittleEndian {
		return *(*uint64)(unsafe.Pointer(&buffer[0]))
	}
	// 大端序需要字节交换
	return uint64(buffer[7]) | uint64(buffer[6])<<8 | uint64(buffer[5])<<16 | uint64(buffer[4])<<24 |
		uint64(buffer[3])<<32 | uint64(buffer[2])<<40 | uint64(buffer[1])<<48 | uint64(buffer[0])<<56
}

// unsafeGetUint32 使用 unsafe 直接读取 uint32 值
func unsafeGetUint32(buffer []byte, byteOrder binary.ByteOrder) uint32 {
	if byteOrder == binary.LittleEndian {
		return *(*uint32)(unsafe.Pointer(&buffer[0]))
	}
	// 大端序需要字节交换
	return uint32(buffer[3]) | uint32(buffer[2])<<8 | uint32(buffer[1])<<16 | uint32(buffer[0])<<24
}

// unsafeGetUint16 使用 unsafe 直接读取 uint16 值
func unsafeGetUint16(buffer []byte, byteOrder binary.ByteOrder) uint16 {
	if byteOrder == binary.LittleEndian {
		return *(*uint16)(unsafe.Pointer(&buffer[0]))
	}
	// 大端序需要字节交换
	return uint16(buffer[1]) | uint16(buffer[0])<<8
}

// unsafePutUint64 使用 unsafe 直接写入 uint64 值
func unsafePutUint64(buffer []byte, value uint64, byteOrder binary.ByteOrder) {
	if byteOrder == binary.LittleEndian {
		*(*uint64)(unsafe.Pointer(&buffer[0])) = value
		return
	}
	// 大端序需要字节交换
	buffer[0] = byte(value >> 56)
	buffer[1] = byte(value >> 48)
	buffer[2] = byte(value >> 40)
	buffer[3] = byte(value >> 32)
	buffer[4] = byte(value >> 24)
	buffer[5] = byte(value >> 16)
	buffer[6] = byte(value >> 8)
	buffer[7] = byte(value)
}

// unsafePutUint32 使用 unsafe 直接写入 uint32 值
func unsafePutUint32(buffer []byte, value uint32, byteOrder binary.ByteOrder) {
	if byteOrder == binary.LittleEndian {
		*(*uint32)(unsafe.Pointer(&buffer[0])) = value
		return
	}
	// 大端序需要字节交换
	buffer[0] = byte(value >> 24)
	buffer[1] = byte(value >> 16)
	buffer[2] = byte(value >> 8)
	buffer[3] = byte(value)
}

// unsafePutUint16 使用 unsafe 直接写入 uint16 值
func unsafePutUint16(buffer []byte, value uint16, byteOrder binary.ByteOrder) {
	if byteOrder == binary.LittleEndian {
		*(*uint16)(unsafe.Pointer(&buffer[0])) = value
		return
	}
	// 大端序需要字节交换
	buffer[0] = byte(value >> 8)
	buffer[1] = byte(value)
}

// unsafeGetFloat64 使用 unsafe 直接读取 float64 值
func unsafeGetFloat64(buffer []byte, byteOrder binary.ByteOrder) float64 {
	bits := unsafeGetUint64(buffer, byteOrder)
	return math.Float64frombits(bits)
}

// unsafeGetFloat32 使用 unsafe 直接读取 float32 值
func unsafeGetFloat32(buffer []byte, byteOrder binary.ByteOrder) float32 {
	bits := unsafeGetUint32(buffer, byteOrder)
	return math.Float32frombits(bits)
}

// unsafePutFloat64 使用 unsafe 直接写入 float64 值
func unsafePutFloat64(buffer []byte, value float64, byteOrder binary.ByteOrder) {
	bits := math.Float64bits(value)
	unsafePutUint64(buffer, bits, byteOrder)
}

// unsafePutFloat32 使用 unsafe 直接写入 float32 值
func unsafePutFloat32(buffer []byte, value float32, byteOrder binary.ByteOrder) {
	bits := math.Float32bits(value)
	unsafePutUint32(buffer, bits, byteOrder)
}

// unsafeMoveSlice 使用 typedmemmove 移动切片数据
// 直接操作切片的底层数据，避免内存拷贝
func unsafeMoveSlice(dst, src reflect.Value) {
	dstHeader := (*unsafeSliceHeader)(unsafe.Pointer(dst.UnsafeAddr()))
	srcHeader := (*unsafeSliceHeader)(unsafe.Pointer(src.UnsafeAddr()))

	// 直接设置切片的底层指针和长度
	dstHeader.Data = srcHeader.Data
	dstHeader.Len = srcHeader.Len
	dstHeader.Cap = srcHeader.Len // 容量设置为长度，避免越界访问
}
