package struc

import (
	"io"
	"reflect"
)

// Packer 定义了数据打包和解包的基本接口
// 实现此接口的类型可以将自身序列化为字节流，并从字节流中反序列化
//
// Packer defines the basic interface for data packing and unpacking
// Types implementing this interface can serialize themselves to byte streams and deserialize from byte streams
type Packer interface {
	// Pack 将值序列化到字节缓冲区中
	// Pack serializes a value into a byte buffer
	Pack(buf []byte, val reflect.Value, options *Options) (int, error)

	// Unpack 从 Reader 中读取数据并反序列化到值中
	// Unpack deserializes data from a Reader into a value
	Unpack(r io.Reader, val reflect.Value, options *Options) error

	// Sizeof 返回值序列化后的字节大小
	// Sizeof returns the size in bytes of the serialized value
	Sizeof(val reflect.Value, options *Options) int

	// String 返回类型的字符串表示
	// String returns a string representation of the type
	String() string
}
