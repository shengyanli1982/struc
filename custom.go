package struc

import (
	"io"
	"reflect"
)

// Custom 定义了自定义类型的序列化和反序列化接口
// 实现此接口的类型可以控制自己的二进制格式
//
// Custom defines the interface for serialization and deserialization of custom types
// Types implementing this interface can control their own binary format
type Custom interface {
	// Pack 将数据打包到字节切片中
	// 参数：
	//   - p: 目标字节切片
	//   - opt: 序列化选项
	// 返回：
	//   - int: 写入的字节数
	//   - error: 错误信息
	//
	// Pack serializes data into a byte slice
	// Parameters:
	//   - p: target byte slice
	//   - opt: serialization options
	// Returns:
	//   - int: number of bytes written
	//   - error: error information
	Pack(p []byte, opt *Options) (int, error)

	// Unpack 从 Reader 中读取并解包数据
	// 参数：
	//   - r: 数据源读取器
	//   - length: 要读取的数据长度
	//   - opt: 反序列化选项
	// 返回：
	//   - error: 错误信息
	//
	// Unpack deserializes data from a Reader
	// Parameters:
	//   - r: source data reader
	//   - length: length of data to read
	//   - opt: deserialization options
	// Returns:
	//   - error: error information
	Unpack(r io.Reader, length int, opt *Options) error

	// Size 返回序列化后的数据大小
	// 参数：
	//   - opt: 序列化选项
	// 返回：
	//   - int: 序列化后的字节数
	//
	// Size returns the size of serialized data
	// Parameters:
	//   - opt: serialization options
	// Returns:
	//   - int: number of bytes after serialization
	Size(opt *Options) int

	// String 返回类型的字符串表示
	// 用于调试和错误信息展示
	//
	// String returns the string representation of the type
	// Used for debugging and error message display
	String() string
}

// customFallback 提供了 Custom 接口的基本实现
// 作为自定义类型序列化的回退处理器
//
// customFallback provides a basic implementation of the Custom interface
// Serves as a fallback handler for custom type serialization
type customFallback struct {
	custom Custom // 实际的自定义类型实例 / Actual custom type instance
}

// Pack 将自定义类型的值打包到缓冲区中
// 直接调用底层自定义类型的 Pack 方法
//
// Pack packs a custom type value into the buffer
// Directly calls the underlying custom type's Pack method
func (c customFallback) Pack(buf []byte, val reflect.Value, options *Options) (int, error) {
	// 调用自定义类型的 Pack 方法
	// Call the custom type's Pack method
	return c.custom.Pack(buf, options)
}

// Unpack 从读取器中解包自定义类型的值
// 调用底层自定义类型的 Unpack 方法，长度固定为1
//
// Unpack unpacks a custom type value from the reader
// Calls the underlying custom type's Unpack method with fixed length 1
func (c customFallback) Unpack(reader io.Reader, val reflect.Value, options *Options) error {
	// 调用自定义类型的 Unpack 方法，长度固定为1
	// Call the custom type's Unpack method with fixed length 1
	return c.custom.Unpack(reader, 1, options)
}

// Sizeof 返回自定义类型值的大小
// 直接调用底层自定义类型的 Size 方法
//
// Sizeof returns the size of a custom type value
// Directly calls the underlying custom type's Size method
func (c customFallback) Sizeof(val reflect.Value, options *Options) int {
	// 调用自定义类型的 Size 方法
	// Call the custom type's Size method
	return c.custom.Size(options)
}

// String 返回自定义类型的字符串表示
// 直接调用底层自定义类型的 String 方法
//
// String returns the string representation of the custom type
// Directly calls the underlying custom type's String method
func (c customFallback) String() string {
	// 调用自定义类型的 String 方法
	// Call the custom type's String method
	return c.custom.String()
}
