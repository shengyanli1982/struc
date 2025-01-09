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

// reset 重置 binaryWriter 的状态以便复用
// reset resets the binaryWriter state for reuse
func (b *binaryWriter) reset(buf []byte) {
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
	binaryWriterPool.Put(w)
}

type binaryFallback reflect.Value

func (b binaryFallback) String() string {
	return reflect.Value(b).Type().String()
}

func (b binaryFallback) Sizeof(val reflect.Value, options *Options) int {
	return binary.Size(val.Interface())
}

func (b binaryFallback) Pack(buf []byte, val reflect.Value, options *Options) (int, error) {
	tmp := getBinaryWriter(buf)
	defer putBinaryWriter(tmp)

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
