package struc

import (
	"encoding/binary"
	"io"
	"reflect"
	"sync"
)

// binaryWriterPool 用于复用 binaryWriter 对象，减少内存分配
// binaryWriterPool is used to reuse binaryWriter objects to reduce memory allocations
var binaryWriterPool = sync.Pool{
	New: func() interface{} {
		return &binaryWriter{}
	},
}

// binaryWriter 实现了 io.Writer 接口，用于高效的字节写入
// binaryWriter implements io.Writer interface for efficient byte writing
type binaryWriter struct {
	buf []byte
	pos int
}

// Write 实现 io.Writer 接口
// Write implements io.Writer interface
func (b *binaryWriter) Write(p []byte) (int, error) {
	// 计算剩余可写容量
	// Calculate remaining writable capacity
	capacity := len(b.buf) - b.pos
	if capacity < len(p) {
		// 如果容量不足，截断写入数据
		// If capacity is insufficient, truncate write data
		p = p[:capacity]
	}
	if len(p) > 0 {
		// 复制数据并更新位置
		// Copy data and update position
		copy(b.buf[b.pos:], p)
		b.pos += len(p)
	}
	return len(p), nil
}

// reset 重置 binaryWriter 的状态以便复用
// reset resets the binaryWriter state for reuse
func (b *binaryWriter) reset(buf []byte) {
	// 重置缓冲区和位置指针
	// Reset buffer and position pointer
	b.buf = buf
	b.pos = 0
}

// getBinaryWriter 从对象池获取 binaryWriter
// getBinaryWriter gets a binaryWriter from the pool
func getBinaryWriter(buf []byte) *binaryWriter {
	w := binaryWriterPool.Get().(*binaryWriter)
	w.reset(buf)
	return w
}

// putBinaryWriter 将 binaryWriter 放回对象池
// putBinaryWriter puts the binaryWriter back to the pool
func putBinaryWriter(w *binaryWriter) {
	w.reset(nil)
	binaryWriterPool.Put(w)
}

type binaryFallback reflect.Value

// String 返回二进制回退处理器的类型字符串
// String returns the type string of binary fallback handler
func (b binaryFallback) String() string {
	return reflect.Value(b).Type().String()
}

// Sizeof 返回值的二进制大小
// Sizeof returns the binary size of the value
func (b binaryFallback) Sizeof(val reflect.Value, options *Options) int {
	return binary.Size(val.Interface())
}

// Pack 将值打包到缓冲区中
// Pack packs the value into the buffer
func (b binaryFallback) Pack(buf []byte, val reflect.Value, options *Options) (int, error) {
	// 从对象池获取临时写入器
	// Get temporary writer from object pool
	tmp := getBinaryWriter(buf)
	defer putBinaryWriter(tmp)

	// 获取字节序，默认使用大端序
	// Get byte order, use big-endian by default
	order := options.Order
	if order == nil {
		order = binary.BigEndian
	}
	err := binary.Write(tmp, order, val.Interface())
	return tmp.pos, err
}

// Unpack 从读取器中解包值
// Unpack unpacks value from reader
func (b binaryFallback) Unpack(r io.Reader, val reflect.Value, options *Options) error {
	// 获取字节序，默认使用大端序
	// Get byte order, use big-endian by default
	order := options.Order
	if order == nil {
		order = binary.BigEndian
	}
	return binary.Read(r, order, val.Interface())
}
