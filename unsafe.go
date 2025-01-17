package struc

import (
	"unsafe"
)

// typedmemmove 是一个底层的内存移动函数
//
//go:linkname typedmemmove runtime.typedmemmove
func typedmemmove(dst unsafe.Pointer, src unsafe.Pointer, size uintptr)
