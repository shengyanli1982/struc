package struc

import (
	"encoding/binary"
	"reflect"
	"sync"
)

// fieldPool 是 Field 对象的全局池
// fieldPool is a global pool for Field objects
var fieldPool = sync.Pool{
	New: func() interface{} {
		return &Field{
			Length:    1,
			ByteOrder: binary.BigEndian, // 默认使用大端字节序 / Default to big-endian
		}
	},
}

// acquireField 从对象池获取一个 Field 对象
// acquireField gets a Field object from the pool
func acquireField() *Field {
	return fieldPool.Get().(*Field)
}

// releaseField 将 Field 对象放回对象池
// releaseField puts a Field object back to the pool
func releaseField(f *Field) {
	if f == nil {
		return
	}
	// 重置字段状态
	// Reset field state
	f.Name = ""
	f.IsPointer = false
	f.Index = 0
	f.Type = 0
	f.defType = 0
	f.IsArray = false
	f.IsSlice = false
	f.Length = 1
	f.ByteOrder = binary.BigEndian
	f.Sizeof = nil
	f.Sizefrom = nil
	f.NestFields = nil
	f.kind = reflect.Invalid

	fieldPool.Put(f)
}

// releaseFields 将 Fields 切片中的所有 Field 对象放回对象池
// releaseFields puts all Field objects in a Fields slice back to the pool
func releaseFields(fields Fields) {
	if fields == nil {
		return
	}
	for _, f := range fields {
		releaseField(f)
	}
}
