package struc

import (
	"io"
	"reflect"
)

// Custom 定义了自定义类型的序列化和反序列化接口
// Custom defines the interface for serialization and deserialization of custom types
type Custom interface {
	// Pack 将数据打包到字节切片中
	// Pack serializes data into a byte slice
	Pack(p []byte, opt *Options) (int, error)

	// Unpack 从 Reader 中读取并解包数据
	// Unpack deserializes data from a Reader
	Unpack(r io.Reader, length int, opt *Options) error

	// Size 返回序列化后的数据大小
	// Size returns the size of serialized data
	Size(opt *Options) int

	// String 返回类型的字符串表示
	// String returns the string representation of the type
	String() string
}

// customFallback 提供了 Custom 接口的基本实现
// customFallback provides a basic implementation of the Custom interface
type customFallback struct {
	custom Custom
}

// Pack 将自定义类型的值打包到缓冲区中
// Pack packs a custom type value into the buffer
func (c customFallback) Pack(p []byte, val reflect.Value, opt *Options) (int, error) {
	// 调用自定义类型的 Pack 方法
	// Call the custom type's Pack method
	return c.custom.Pack(p, opt)
}

// Unpack 从读取器中解包自定义类型的值
// Unpack unpacks a custom type value from the reader
func (c customFallback) Unpack(r io.Reader, val reflect.Value, opt *Options) error {
	// 调用自定义类型的 Unpack 方法，长度固定为1
	// Call the custom type's Unpack method with fixed length 1
	return c.custom.Unpack(r, 1, opt)
}

// Sizeof 返回自定义类型值的大小
// Sizeof returns the size of a custom type value
func (c customFallback) Sizeof(val reflect.Value, opt *Options) int {
	// 调用自定义类型的 Size 方法
	// Call the custom type's Size method
	return c.custom.Size(opt)
}

// String 返回自定义类型的字符串表示
// String returns the string representation of the custom type
func (c customFallback) String() string {
	// 调用自定义类型的 String 方法
	// Call the custom type's String method
	return c.custom.String()
}
