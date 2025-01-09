package struc

import (
	"encoding/binary"
	"io"
	"reflect"
	"sync"
)

// byteWriterPool 用于复用 byteWriter 对象，减少内存分配
// byteWriterPool is used to reuse byteWriter objects to reduce memory allocations
var byteWriterPool = sync.Pool{
	New: func() interface{} {
		return &byteWriter{}
	},
}

// byteWriter 实现了 io.Writer 接口，用于高效的字节写入
// byteWriter implements io.Writer interface for efficient byte writing
type byteWriter struct {
	buf []byte
	pos int
}

// Write 实现 io.Writer 接口
// Write implements io.Writer interface
func (b *byteWriter) Write(p []byte) (int, error) {
	capacity := len(b.buf) - b.pos
	if capacity < len(p) {
		p = p[:capacity]
	}
	if len(p) > 0 {
		copy(b.buf[b.pos:], p)
		b.pos += len(p)
	}
	return len(p), nil
}

// reset 重置 byteWriter 的状态以便复用
// reset resets the byteWriter state for reuse
func (b *byteWriter) reset(buf []byte) {
	b.buf = buf
	b.pos = 0
}

// getByteWriter 从对象池获取 byteWriter
// getByteWriter gets a byteWriter from the pool
func getByteWriter(buf []byte) *byteWriter {
	w := byteWriterPool.Get().(*byteWriter)
	w.reset(buf)
	return w
}

// putByteWriter 将 byteWriter 放回对象池
// putByteWriter puts the byteWriter back to the pool
func putByteWriter(w *byteWriter) {
	byteWriterPool.Put(w)
}

type binaryFallback reflect.Value

func (b binaryFallback) String() string {
	return reflect.Value(b).Type().String()
}

func (b binaryFallback) Sizeof(val reflect.Value, options *Options) int {
	return binary.Size(val.Interface())
}

func (b binaryFallback) Pack(buf []byte, val reflect.Value, options *Options) (int, error) {
	tmp := getByteWriter(buf)
	defer putByteWriter(tmp)

	order := options.Order
	if order == nil {
		order = binary.BigEndian
	}
	err := binary.Write(tmp, order, val.Interface())
	return tmp.pos, err
}

func (b binaryFallback) Unpack(r io.Reader, val reflect.Value, options *Options) error {
	order := options.Order
	if order == nil {
		order = binary.BigEndian
	}
	return binary.Read(r, order, val.Interface())
}
