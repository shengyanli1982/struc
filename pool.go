package struc

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"sync"
)

// MaxBufferCapSize 定义了缓冲区的最大容量限制
// 超过此限制的缓冲区不会被放入对象池
//
// MaxBufferCapSize defines the maximum capacity limit for buffers
// Buffers exceeding this limit will not be put into the object pool
const MaxBufferCapSize = 1 << 20

// MaxBytesSliceSize 定义了字节切片的最大容量限制
// 超过此限制的字节切片不会被放入对象池
//
// MaxBytesSliceSize defines the maximum capacity limit for byte slices
// Byte slices exceeding this limit will not be put into the object pool
const MaxBytesSliceSize = 4096

// bufferPool 用于减少[]byte的内存分配
// bufferPool is used to reduce allocations when []byte is used
var bufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 1024))
	},
}

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

// sizeofMapPool 是用于复用 sizeofMap 的对象池
// sizeofMapPool is an object pool for reusing sizeofMap
var sizeofMapPool = sync.Pool{
	New: func() interface{} {
		return make(map[string][]int)
	},
}

// acquireSizeofMap 从对象池获取一个 sizeofMap
// acquireSizeofMap gets a sizeofMap from the pool
func acquireSizeofMap() map[string][]int {
	return sizeofMapPool.Get().(map[string][]int)
}

// releaseSizeofMap 将 sizeofMap 放回对象池
// releaseSizeofMap puts a sizeofMap back to the pool
func releaseSizeofMap(m map[string][]int) {
	if m == nil {
		return
	}
	// 清空 map
	// Clear the map
	for k := range m {
		delete(m, k)
	}
	sizeofMapPool.Put(m)
}

// acquireBuffer 从对象池获取缓冲区
// acquireBuffer gets a buffer from the pool
func acquireBuffer() *bytes.Buffer {
	return bufferPool.Get().(*bytes.Buffer)
}

// releaseBuffer 将缓冲区放回对象池
// releaseBuffer returns a buffer to the pool
func releaseBuffer(buf *bytes.Buffer) {
	if buf == nil || buf.Cap() > MaxBufferCapSize {
		return
	}

	buf.Reset()
	bufferPool.Put(buf)
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

// BytesSlicePool 是一个用于管理共享字节切片的结构体
// 它提供了线程安全的切片分配和重用功能
//
// BytesSlicePool is a structure for managing shared byte slices
// It provides thread-safe slice allocation and reuse functionality
type BytesSlicePool struct {
	bytes  []byte     // 底层字节数组 / underlying byte array
	offset int32      // 当前偏移量 / current offset position
	size   int        // 当前块大小 / current block size
	mu     sync.Mutex // 互斥锁用于保护并发访问 / mutex for protecting concurrent access
}

// NewBytesSlicePool 创建一个新的 BytesSlicePool 实例
// 初始化时，会分配一个 4096 字节的字节数组
//
// NewBytesSlicePool creates a new BytesSlicePool instance
// Initializes with a 4096-byte byte array
func NewBytesSlicePool(size int) *BytesSlicePool {
	// 如果 size 小于等于 0 或者大于 MaxBytesSliceSize，则使用 MaxBytesSliceSize
	// If size is less than or equal to 0 or greater than MaxBytesSliceSize, use MaxBytesSliceSize
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
//
// GetSlice returns a byte slice of specified size
// If current block has insufficient space, allocates new block and resets offset
func (b *BytesSlicePool) GetSlice(size int) []byte {
	b.mu.Lock()

	// 如果请求的大小超过了最大限制，直接分配新的切片
	// If the requested size exceeds the maximum limit, allocate a new slice directly
	if size > b.size {
		b.mu.Unlock()
		return make([]byte, size)
	}

	// 检查剩余空间是否足够
	// Check if remaining space is sufficient
	if int(b.offset)+size > b.size {
		// 分配新的固定大小块（size字节）并重置偏移量
		// Allocate new fixed-size block (size bytes) and reset offset
		b.bytes = make([]byte, b.size)
		b.offset = 0
	}

	// 计算结束位置
	// Calculate end position
	tail := b.offset + int32(size)

	// 从当前偏移量位置切割指定大小的切片
	// Slice the requested size from current offset position
	slice := b.bytes[b.offset:tail]

	// 更新偏移量
	// Update offset
	b.offset = tail

	b.mu.Unlock()
	return slice
}
