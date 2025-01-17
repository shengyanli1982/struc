package struc

import (
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
