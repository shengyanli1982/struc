package struc

import (
	"encoding/binary"
	"math"
	"reflect"
	"unsafe"
)

//go:linkname memclrNoHeapPointers runtime.memclrNoHeapPointers
func memclrNoHeapPointers(ptr unsafe.Pointer, n uintptr)

// typedmemmove 是一个底层的内存移动函数
//
//go:linkname typedmemmove runtime.typedmemmove
func typedmemmove(dst unsafe.Pointer, src unsafe.Pointer, size uintptr)

// memclr 使用 runtime 的内存清零函数, 比循环清零更高效
func memclr(b []byte) {
	if len(b) == 0 {
		return
	}
	memclrNoHeapPointers(unsafe.Pointer(&b[0]), uintptr(len(b)))
}

// unsafeSliceHeader 是切片的内部表示
type unsafeSliceHeader struct {
	Data uintptr
	Len  int
	Cap  int
}

// unsafeBytes2String 使用 unsafe 将字节切片转换为字符串, 避免内存拷贝
func unsafeBytes2String(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// unsafeSetSlice 使用 unsafe 直接设置切片的底层数据, 避免内存拷贝
func unsafeSetSlice(fieldValue reflect.Value, buffer []byte, length int) {
	sh := (*unsafeSliceHeader)(unsafe.Pointer(fieldValue.UnsafeAddr()))
	sh.Data = uintptr(unsafe.Pointer(&buffer[0]))
	sh.Len = length
	sh.Cap = length
}

// unsafeSetString 使用 unsafe 将字节切片转换为字符串并设置到字段, 避免内存拷贝
func unsafeSetString(fieldValue reflect.Value, buffer []byte, length int) {
	str := unsafeBytes2String(buffer[:length])
	fieldValue.SetString(str)
}

// unsafeGetUint64 使用 unsafe 直接读取 uint64 值, 避免内存拷贝
func unsafeGetUint64(buffer []byte, byteOrder binary.ByteOrder) uint64 {
	if byteOrder == binary.LittleEndian {
		return *(*uint64)(unsafe.Pointer(&buffer[0]))
	}
	// 大端序使用 binary 包，可能被编译器优化
	return binary.BigEndian.Uint64(buffer)
}

// unsafeGetUint32 使用 unsafe 直接读取 uint32 值, 避免内存拷贝
func unsafeGetUint32(buffer []byte, byteOrder binary.ByteOrder) uint32 {
	if byteOrder == binary.LittleEndian {
		return *(*uint32)(unsafe.Pointer(&buffer[0]))
	}
	// 大端序使用 binary 包，可能被编译器优化
	return binary.BigEndian.Uint32(buffer)
}

// unsafeGetUint16 使用 unsafe 直接读取 uint16 值, 避免内存拷贝
func unsafeGetUint16(buffer []byte, byteOrder binary.ByteOrder) uint16 {
	if byteOrder == binary.LittleEndian {
		return *(*uint16)(unsafe.Pointer(&buffer[0]))
	}
	// 大端序使用 binary 包，可能被编译器优化
	return binary.BigEndian.Uint16(buffer)
}

// unsafePutUint64 使用 unsafe 直接写入 uint64 值, 避免内存拷贝
func unsafePutUint64(buffer []byte, value uint64, byteOrder binary.ByteOrder) {
	if byteOrder == binary.LittleEndian {
		*(*uint64)(unsafe.Pointer(&buffer[0])) = value
		return
	}
	// 大端序使用 binary 包，可能被编译器优化
	binary.BigEndian.PutUint64(buffer, value)
}

// unsafePutUint32 使用 unsafe 直接写入 uint32 值, 避免内存拷贝
func unsafePutUint32(buffer []byte, value uint32, byteOrder binary.ByteOrder) {
	if byteOrder == binary.LittleEndian {
		*(*uint32)(unsafe.Pointer(&buffer[0])) = value
		return
	}
	// 大端序使用 binary 包，可能被编译器优化
	binary.BigEndian.PutUint32(buffer, value)
}

// unsafePutUint16 使用 unsafe 直接写入 uint16 值, 避免内存拷贝
func unsafePutUint16(buffer []byte, value uint16, byteOrder binary.ByteOrder) {
	if byteOrder == binary.LittleEndian {
		*(*uint16)(unsafe.Pointer(&buffer[0])) = value
		return
	}
	// 大端序使用 binary 包，可能被编译器优化
	binary.BigEndian.PutUint16(buffer, value)
}

// unsafeGetFloat64 使用 unsafe 直接读取 float64 值
// 通过转换为 uint64 位模式实现
func unsafeGetFloat64(buffer []byte, byteOrder binary.ByteOrder) float64 {
	bits := unsafeGetUint64(buffer, byteOrder)
	return math.Float64frombits(bits)
}

// unsafeGetFloat32 使用 unsafe 直接读取 float32 值
// 通过转换为 uint32 位模式实现
func unsafeGetFloat32(buffer []byte, byteOrder binary.ByteOrder) float32 {
	bits := unsafeGetUint32(buffer, byteOrder)
	return math.Float32frombits(bits)
}

// unsafePutFloat64 使用 unsafe 直接写入 float64 值
// 通过转换为 uint64 位模式实现
func unsafePutFloat64(buffer []byte, value float64, byteOrder binary.ByteOrder) {
	bits := math.Float64bits(value)
	unsafePutUint64(buffer, bits, byteOrder)
}

// unsafePutFloat32 使用 unsafe 直接写入 float32 值
// 通过转换为 uint32 位模式实现
func unsafePutFloat32(buffer []byte, value float32, byteOrder binary.ByteOrder) {
	bits := math.Float32bits(value)
	unsafePutUint32(buffer, bits, byteOrder)
}

// unsafeMoveSlice 使用 typedmemmove 移动切片数据
// 直接操作切片的底层数据，避免内存拷贝
func unsafeMoveSlice(dst, src reflect.Value) {
	dstPtr := unsafe.Pointer(dst.Pointer())
	srcPtr := unsafe.Pointer(src.Pointer())

	// The length of data to copy is the length of the source slice in bytes.
	// Since src is always a []byte slice from reflect.ValueOf(buffer),
	// src.Len() is the number of bytes.
	dataLen := uintptr(src.Len())
	if dataLen == 0 {
		return
	}
	typedmemmove(dstPtr, srcPtr, dataLen)
}
