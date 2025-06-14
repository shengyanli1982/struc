package struc

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
)

// MaxBufferCapSize 定义了缓冲区的最大容量限制
// 超过此限制的缓冲区不会被放入对象池
const MaxBufferCapSize = 1 << 20

// MaxBytesSliceSize 定义了字节切片的最大容量限制
// 超过此限制的字节切片不会被放入对象池
const MaxBytesSliceSize = 4096

// bufferPool 用于减少[]byte的内存分配
var bufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 1024))
	},
}

// fieldPool 是 Field 对象的全局池
var fieldPool = sync.Pool{
	New: func() interface{} {
		return &Field{
			Length:    1,
			ByteOrder: binary.BigEndian, // 默认使用大端字节序
		}
	},
}

// sizeofMapPool 是用于复用 sizeofMap 的对象池
var sizeofMapPool = sync.Pool{
	New: func() interface{} {
		return make(map[string][]int)
	},
}

// acquireSizeofMap 从对象池获取一个 sizeofMap
func acquireSizeofMap() map[string][]int {
	return sizeofMapPool.Get().(map[string][]int)
}

// releaseSizeofMap 将 sizeofMap 放回对象池
func releaseSizeofMap(m map[string][]int) {
	if m == nil {
		return
	}
	// 清空 map
	for k := range m {
		delete(m, k)
	}
	sizeofMapPool.Put(m)
}

// acquireBuffer 从对象池获取缓冲区
func acquireBuffer() *bytes.Buffer {
	return bufferPool.Get().(*bytes.Buffer)
}

// releaseBuffer 将缓冲区放回对象池
func releaseBuffer(buf *bytes.Buffer) {
	if buf == nil || buf.Cap() > MaxBufferCapSize {
		return
	}

	buf.Reset()
	bufferPool.Put(buf)
}

// acquireField 从对象池获取一个 Field 对象
func acquireField() *Field {
	return fieldPool.Get().(*Field)
}

// releaseField 将 Field 对象放回对象池
func releaseField(f *Field) {
	if f == nil {
		return
	}
	// 重置字段状态
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
func releaseFields(fields Fields) {
	if fields == nil {
		return
	}
	for _, f := range fields {
		releaseField(f)
	}
}

// BytesSlicePool 是一个用于管理共享字节切片的结构体
// 它提供了线程安全的切片分配和重用功能
type BytesSlicePool struct {
	bytes  []byte     // 底层字节数组
	offset int32      // 当前偏移量
	size   int        // 当前块大小
	mu     sync.Mutex // 互斥锁用于保护并发访问
}

// NewBytesSlicePool 创建一个新的 BytesSlicePool 实例
// 初始化时，会分配一个 4096 字节的字节数组
func NewBytesSlicePool(size int) *BytesSlicePool {
	// 如果 size 小于等于 0 或者大于 MaxBytesSliceSize，则使用 MaxBytesSliceSize
	if size > MaxBytesSliceSize || size <= 0 {
		size = MaxBytesSliceSize
	}

	return &BytesSlicePool{
		bytes:  make([]byte, size),
		offset: 0,
		size:   size,
		mu:     sync.Mutex{},
	}
}

// GetSlice 返回指定大小的字节切片
// 如果当前块空间不足，会分配新的块并重置偏移量
func (b *BytesSlicePool) GetSlice(size int) []byte {
	// 如果请求的大小超过了最大限制，直接分配新的切片
	if size > b.size {
		return make([]byte, size)
	}

	// 快速路径：尝试有限次数的原子操作，使用退避策略减少 CPU 压力
	for i := 0; i < 4; i++ { // 最多尝试 4 次 / Maximum 4 attempts
		currentOffset := atomic.LoadInt32(&b.offset)

		if int(currentOffset)+size > b.size {
			break
		}

		newOffset := currentOffset + int32(size)
		if atomic.CompareAndSwapInt32(&b.offset, currentOffset, newOffset) {
			return b.bytes[currentOffset:newOffset]
		}

		// 简单的退避策略，防止 CPU 过热
		if i > 0 {
			for j := 0; j < (1 << i); j++ {
				// 让出 CPU，允许其他 goroutine 执行
				runtime.Gosched()
			}
		}
	}

	// 慢路径：多次尝试失败或空间不足时使用
	return b.getSliceSlow(size)
}

// getSliceSlow 是 GetSlice 的慢路径实现
// 使用互斥锁保护重置操作，减少竞争
func (b *BytesSlicePool) getSliceSlow(size int) []byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	if int(b.offset)+size > b.size {
		b.bytes = make([]byte, b.size)
		b.offset = 0
	}
	start := b.offset
	b.offset += int32(size)

	return b.bytes[start:b.offset]
}
