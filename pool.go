package struc

import (
	"bytes"
	"encoding/binary"
	"math/bits"
	"reflect"
	"sync"
	"sync/atomic"
)

// MaxBufferCapSize 定义了缓冲区的最大容量限制
// 超过此限制的缓冲区不会被放入对象池
const MaxBufferCapSize = 1 << 20

// MaxBytesSliceSize 定义了字节切片的最大容量限制
// 超过此限制的字节切片不会被放入对象池
const MaxBytesSliceSize = 4096

// MaxTempBytesSize 定义可归还临时 []byte 的最大容量限制。
// 该池仅用于 Pack 侧与 Unpack 的 scratch buffer（不会被零拷贝借用）。
const MaxTempBytesSize = 1 << 16 // 64KiB

// minTempBytesSize 定义可归还临时 []byte 的最小 size class。
const minTempBytesSize = 64

// tempBytesPools 是按 size class 划分的临时缓冲池。
// 注意：sync.Pool 存放非指针的 []byte 会触发 boxing 分配，因此这里存放 *[]byte。
var tempBytesPools = [...]sync.Pool{
	{New: func() interface{} { b := make([]byte, 1<<6); return &b }},  // 64
	{New: func() interface{} { b := make([]byte, 1<<7); return &b }},  // 128
	{New: func() interface{} { b := make([]byte, 1<<8); return &b }},  // 256
	{New: func() interface{} { b := make([]byte, 1<<9); return &b }},  // 512
	{New: func() interface{} { b := make([]byte, 1<<10); return &b }}, // 1KiB
	{New: func() interface{} { b := make([]byte, 1<<11); return &b }}, // 2KiB
	{New: func() interface{} { b := make([]byte, 1<<12); return &b }}, // 4KiB
	{New: func() interface{} { b := make([]byte, 1<<13); return &b }}, // 8KiB
	{New: func() interface{} { b := make([]byte, 1<<14); return &b }}, // 16KiB
	{New: func() interface{} { b := make([]byte, 1<<15); return &b }}, // 32KiB
	{New: func() interface{} { b := make([]byte, 1<<16); return &b }}, // 64KiB
}

func tempBytesSizeClass(size int) (poolIndex int, classSize int, ok bool) {
	if size <= 0 {
		return 0, 0, false
	}
	if size > MaxTempBytesSize {
		return 0, 0, false
	}
	if size <= minTempBytesSize {
		return 0, minTempBytesSize, true
	}
	// Next power of two.
	classSize = 1 << bits.Len(uint(size-1))
	if classSize < minTempBytesSize || classSize > MaxTempBytesSize {
		return 0, 0, false
	}
	// 64 -> 0, 128 -> 1, ..., 64KiB -> 10
	poolIndex = bits.Len(uint(classSize)) - 7
	if poolIndex < 0 || poolIndex >= len(tempBytesPools) {
		return 0, 0, false
	}
	return poolIndex, classSize, true
}

type tempBytes struct {
	buf       []byte
	holder    *[]byte
	poolIndex int
	classSize int
}

func (t tempBytes) Bytes() []byte {
	return t.buf
}

func (t tempBytes) Release() {
	if t.holder == nil {
		return
	}
	b := *t.holder
	if cap(b) < t.classSize {
		b = make([]byte, t.classSize)
	}
	b = b[:t.classSize]
	*t.holder = b
	tempBytesPools[t.poolIndex].Put(t.holder)
}

// acquireTempBytes 获取一个长度为 size 的临时 []byte（容量被限制为 size，禁止扩容 reslice）。
// 注意：仅用于不会被零拷贝借用的场景（例如 Pack 侧临时 buffer / Unpack scratch）。
func acquireTempBytes(size int) tempBytes {
	if poolIndex, classSize, ok := tempBytesSizeClass(size); ok {
		holder := tempBytesPools[poolIndex].Get().(*[]byte)
		b := *holder
		if cap(b) < classSize {
			b = make([]byte, classSize)
			*holder = b
		}
		if len(b) < classSize {
			b = b[:classSize]
			*holder = b
		}
		return tempBytes{
			buf:       b[:size:size],
			holder:    holder,
			poolIndex: poolIndex,
			classSize: classSize,
		}
	}
	b := make([]byte, size)
	return tempBytes{buf: b}
}

// bufferPool 用于减少[]byte的内存分配
var bufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 1024))
	},
}

// scratchArena 用于 Unpack 路径中“不会零拷贝借用 buffer”的字段读取。
// 该 arena 会在单次 Unpack 调用内被复用，避免为每个字段都向 sync.Pool 取/还 buffer。
type scratchArena struct {
	bytes  []byte
	offset int
}

const (
	defaultScratchArenaSize = 4096
	maxScratchArenaSize     = 1 << 20
)

var scratchArenaPool = sync.Pool{
	New: func() interface{} {
		return &scratchArena{bytes: make([]byte, defaultScratchArenaSize)}
	},
}

func acquireScratchArena() *scratchArena {
	a := scratchArenaPool.Get().(*scratchArena)
	a.offset = 0
	return a
}

func releaseScratchArena(a *scratchArena) {
	if a == nil {
		return
	}
	a.offset = 0
	if cap(a.bytes) > maxScratchArenaSize {
		a.bytes = make([]byte, defaultScratchArenaSize)
	} else {
		a.bytes = a.bytes[:cap(a.bytes)]
	}
	scratchArenaPool.Put(a)
}

func (a *scratchArena) Get(size int) []byte {
	if size <= 0 {
		return nil
	}
	if size > cap(a.bytes) {
		a.bytes = make([]byte, size)
		a.offset = 0
		return a.bytes[:size]
	}
	if a.offset+size > cap(a.bytes) {
		a.offset = 0
	}
	a.bytes = a.bytes[:cap(a.bytes)]
	start := a.offset
	a.offset += size
	return a.bytes[start:a.offset]
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

	// 快速路径：尝试一次原子操作
	currentOffset := atomic.LoadInt32(&b.offset)
	if int(currentOffset)+size <= b.size {
		newOffset := currentOffset + int32(size)
		if atomic.CompareAndSwapInt32(&b.offset, currentOffset, newOffset) {
			return b.bytes[currentOffset:newOffset]
		}
	}

	// 慢路径：使用互斥锁保护重置操作
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
