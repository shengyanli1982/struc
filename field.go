// Package struc implements binary packing and unpacking for Go structs.
package struc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"sync"
)

// Field represents a single field in a struct.
// Field 表示结构体中的单个字段。
type Field struct {
	Name     string           // Field name 字段名称
	Ptr      bool             // Whether the field is a pointer 字段是否为指针
	Index    int              // Field index in struct 字段在结构体中的索引
	Type     Type             // Field type 字段类型
	defType  Type             // Default type 默认类型
	Array    bool             // Whether the field is an array 字段是否为数组
	Slice    bool             // Whether the field is a slice 字段是否为切片
	Len      int              // Length for arrays/fixed slices 数组/固定切片的长度
	Order    binary.ByteOrder // Byte order 字节序
	Sizeof   []int            // Sizeof reference indices sizeof 引用索引
	Sizefrom []int            // Size reference indices 大小引用索引
	Fields   Fields           // Nested fields for struct types 结构体类型的嵌套字段
	kind     reflect.Kind     // Reflect kind 反射类型
}

// fieldBufferPool is used to reduce allocations when packing/unpacking
// fieldBufferPool 用于减少打包/解包时的内存分配
var fieldBufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 1024)) // 预分配 1KB 的初始容量
	},
}

// String returns a string representation of the field.
// String 返回字段的字符串表示。
func (f *Field) String() string {
	if f.Type == Pad {
		return fmt.Sprintf("{type: Pad, len: %d}", f.Len)
	}

	b := fieldBufferPool.Get().(*bytes.Buffer)
	b.Reset()
	defer fieldBufferPool.Put(b)

	b.WriteString("{")
	b.WriteString(fmt.Sprintf("type: %s", f.Type))

	if f.Order != nil {
		b.WriteString(fmt.Sprintf(", order: %v", f.Order))
	}
	if f.Sizefrom != nil {
		b.WriteString(fmt.Sprintf(", sizefrom: %v", f.Sizefrom))
	} else if f.Len > 0 {
		b.WriteString(fmt.Sprintf(", len: %d", f.Len))
	}
	if f.Sizeof != nil {
		b.WriteString(fmt.Sprintf(", sizeof: %v", f.Sizeof))
	}
	b.WriteString("}")

	return b.String()
}

// Size calculates the size of the field in bytes.
// Size 计算字段的字节大小。
func (f *Field) Size(val reflect.Value, options *Options) int {
	typ := f.Type.Resolve(options)
	size := 0

	switch typ {
	case Struct:
		size = f.calculateStructSize(val, options)
	case Pad:
		size = f.Len
	case CustomType:
		size = f.calculateCustomSize(val, options)
	default:
		size = f.calculateBasicSize(val, typ, options)
	}

	return f.alignSize(size, options)
}

// calculateStructSize 计算结构体类型的大小
// calculateStructSize calculates size for struct types
func (f *Field) calculateStructSize(val reflect.Value, options *Options) int {
	if f.Slice {
		length := val.Len()
		size := 0
		for i := 0; i < length; i++ {
			size += f.Fields.Sizeof(val.Index(i), options)
		}
		return size
	}
	return f.Fields.Sizeof(val, options)
}

// calculateCustomSize 计算自定义类型的大小
// calculateCustomSize calculates size for custom types
func (f *Field) calculateCustomSize(val reflect.Value, options *Options) int {
	if c, ok := val.Addr().Interface().(Custom); ok {
		return c.Size(options)
	}
	return 0
}

// calculateBasicSize 计算基本类型的大小
// calculateBasicSize calculates size for basic types
func (f *Field) calculateBasicSize(val reflect.Value, typ Type, options *Options) int {
	elemSize := typ.Size()
	if f.Slice || f.kind == reflect.String {
		length := val.Len()
		if f.Len > 1 {
			length = f.Len // 使用指定的固定长度 / Use specified fixed length
		}
		return length * elemSize
	}
	return elemSize
}

// alignSize 根据 ByteAlign 选项对齐大小
// alignSize aligns the size according to ByteAlign option
func (f *Field) alignSize(size int, options *Options) int {
	if align := options.ByteAlign; align > 0 {
		if remainder := size % align; remainder != 0 {
			size += align - remainder
		}
	}
	return size
}

// packVal 将单个值打包到缓冲区中
// packVal packs a single value into the buffer
func (f *Field) packVal(buf []byte, val reflect.Value, length int, options *Options) (size int, err error) {
	// 获取字节序并处理指针类型
	// Get byte order and handle pointer type
	order := f.getByteOrder(options)
	if f.Ptr {
		val = val.Elem()
	}

	// 解析类型并根据类型选择相应的打包方法
	// Resolve type and choose appropriate packing method
	typ := f.Type.Resolve(options)
	switch typ {
	case Struct:
		return f.Fields.Pack(buf, val, options)
	case Bool, Int8, Int16, Int32, Int64, Uint8, Uint16, Uint32, Uint64:
		return f.packInteger(buf, val, typ, order)
	case Float32, Float64:
		return f.packFloat(buf, val, typ, order)
	case String:
		return f.packString(buf, val)
	case CustomType:
		return f.packCustom(buf, val, options)
	default:
		return 0, fmt.Errorf("unsupported type for packing: %v", typ)
	}
}

// getByteOrder 返回要使用的字节序
// getByteOrder returns the byte order to use
func (f *Field) getByteOrder(options *Options) binary.ByteOrder {
	if options.Order != nil {
		return options.Order
	}
	return f.Order
}

// packInteger 打包整数值
// packInteger packs an integer value
func (f *Field) packInteger(buf []byte, val reflect.Value, typ Type, order binary.ByteOrder) (int, error) {
	n := f.getIntegerValue(val)
	size := typ.Size()
	if err := f.writeInteger(buf, n, typ, order); err != nil {
		return 0, fmt.Errorf("failed to write integer: %w", err)
	}
	return size, nil
}

// packFloat 打包浮点数值
// packFloat packs a float value
func (f *Field) packFloat(buf []byte, val reflect.Value, typ Type, order binary.ByteOrder) (int, error) {
	n := val.Float()
	size := typ.Size()
	if err := f.writeFloat(buf, n, typ, order); err != nil {
		return 0, fmt.Errorf("failed to write float: %w", err)
	}
	return size, nil
}

// packString 打包字符串值
// packString packs a string value
func (f *Field) packString(buf []byte, val reflect.Value) (int, error) {
	var data []byte
	switch f.kind {
	case reflect.String:
		data = []byte(val.String())
	default:
		data = val.Bytes()
	}
	size := len(data)
	copy(buf, data)
	return size, nil
}

// packCustom 打包自定义类型
// packCustom packs a custom type
func (f *Field) packCustom(buf []byte, val reflect.Value, options *Options) (int, error) {
	if c, ok := val.Addr().Interface().(Custom); ok {
		return c.Pack(buf, options)
	}
	return 0, fmt.Errorf("failed to pack custom type: %v", val.Type())
}

// Pack 将字段值打包到缓冲区中
// Pack packs the field value into the buffer
func (f *Field) Pack(buf []byte, val reflect.Value, length int, options *Options) (int, error) {
	// 处理填充类型
	// Handle padding type
	if typ := f.Type.Resolve(options); typ == Pad {
		return f.packPadding(buf, length)
	}

	// 根据字段是否为切片选择打包方法
	// Choose packing method based on whether the field is a slice
	if f.Slice {
		return f.packSlice(buf, val, length, options)
	}
	return f.packVal(buf, val, length, options)
}

// packPadding 打包填充字节
// packPadding packs padding bytes
func (f *Field) packPadding(buf []byte, length int) (int, error) {
	for i := 0; i < length; i++ {
		buf[i] = 0
	}
	return length, nil
}

// packSlice 将切片值打包到缓冲区中
// packSlice packs a slice value into the buffer
func (f *Field) packSlice(buf []byte, val reflect.Value, length int, options *Options) (int, error) {
	end := val.Len()
	typ := f.Type.Resolve(options)

	// 对字节切片和字符串类型进行优化处理
	// Optimize handling for byte slices and strings
	if !f.Array && typ == Uint8 && (f.defType == Uint8 || f.kind == reflect.String) {
		return f.packByteSlice(buf, val, end, length)
	}

	return f.packGenericSlice(buf, val, end, length, options)
}

// packByteSlice 优化字节切片的打包
// packByteSlice optimizes packing for byte slices
func (f *Field) packByteSlice(buf []byte, val reflect.Value, end, length int) (int, error) {
	var data []byte
	if f.kind == reflect.String {
		data = []byte(val.String())
	} else {
		data = val.Bytes()
	}
	copy(buf, data)
	if end < length {
		// 用零值填充剩余空间
		// Zero-fill the remaining space
		for i := end; i < length; i++ {
			buf[i] = 0
		}
		return length, nil
	}
	return end, nil
}

// packGenericSlice 打包通用切片
// packGenericSlice packs a generic slice
func (f *Field) packGenericSlice(buf []byte, val reflect.Value, end, length int, options *Options) (int, error) {
	pos := 0
	var zero reflect.Value
	if end < length {
		zero = reflect.Zero(val.Type().Elem())
	}

	for i := 0; i < length; i++ {
		cur := zero
		if i < end {
			cur = val.Index(i)
		}
		n, err := f.packVal(buf[pos:], cur, 1, options)
		if err != nil {
			return pos, fmt.Errorf("failed to pack slice element %d: %w", i, err)
		}
		pos += n
	}

	return pos, nil
}

// Unpack 从缓冲区中解包字段值
// Unpack unpacks the field value from the buffer
func (f *Field) Unpack(buf []byte, val reflect.Value, length int, options *Options) error {
	typ := f.Type.Resolve(options)

	// 处理填充和字符串类型
	// Handle padding and string types
	if typ == Pad || f.kind == reflect.String {
		return f.unpackPadOrString(buf, val, typ)
	}

	// 根据字段是否为切片选择解包方法
	// Choose unpacking method based on whether the field is a slice
	if f.Slice {
		return f.unpackSlice(buf, val, length, options)
	}

	return f.unpackVal(buf, val, length, options)
}

// unpackPadOrString 处理填充或字符串类型的解包
// unpackPadOrString handles unpacking of padding or string types
func (f *Field) unpackPadOrString(buf []byte, val reflect.Value, typ Type) error {
	if typ == Pad {
		return nil
	}
	val.SetString(string(buf))
	return nil
}

// unpackSlice 处理切片类型的解包
// unpackSlice handles unpacking of slice types
func (f *Field) unpackSlice(buf []byte, val reflect.Value, length int, options *Options) error {
	if val.Cap() < length {
		val.Set(reflect.MakeSlice(val.Type(), length, length))
	} else if val.Len() < length {
		val.Set(val.Slice(0, length))
	}

	typ := f.Type.Resolve(options)
	if !f.Array && typ == Uint8 && f.defType == Uint8 {
		copy(val.Bytes(), buf[:length])
		return nil
	}

	size := typ.Size()
	for i := 0; i < length; i++ {
		pos := i * size
		if err := f.unpackVal(buf[pos:pos+size], val.Index(i), 1, options); err != nil {
			return fmt.Errorf("failed to unpack slice element %d: %w", i, err)
		}
	}
	return nil
}

// unpackVal 从缓冲区中解包单个值
// unpackVal unpacks a single value from the buffer
func (f *Field) unpackVal(buf []byte, val reflect.Value, length int, options *Options) error {
	// 获取字节序并处理指针类型
	// Get byte order and handle pointer type
	order := f.getByteOrder(options)
	if f.Ptr {
		val = val.Elem()
	}

	// 根据类型选择相应的解包方法
	// Choose appropriate unpacking method based on type
	typ := f.Type.Resolve(options)
	switch typ {
	case Float32, Float64:
		return f.unpackFloat(buf, val, typ, order)
	case Bool, Int8, Int16, Int32, Int64, Uint8, Uint16, Uint32, Uint64:
		return f.unpackInteger(buf, val, typ, order)
	default:
		return fmt.Errorf("no unpack handler for type: %s", typ)
	}
}

// unpackFloat 解包浮点数值
// unpackFloat unpacks a float value
func (f *Field) unpackFloat(buf []byte, val reflect.Value, typ Type, order binary.ByteOrder) error {
	var n float64
	switch typ {
	case Float32:
		n = float64(math.Float32frombits(order.Uint32(buf)))
	case Float64:
		n = math.Float64frombits(order.Uint64(buf))
	}

	switch f.kind {
	case reflect.Float32, reflect.Float64:
		val.SetFloat(n)
		return nil
	default:
		return fmt.Errorf("struc: refusing to unpack float into field %s of type %s", f.Name, f.kind.String())
	}
}

// unpackInteger 解包整数值
// unpackInteger unpacks an integer value
func (f *Field) unpackInteger(buf []byte, val reflect.Value, typ Type, order binary.ByteOrder) error {
	n := f.readInteger(buf, typ, order)

	switch f.kind {
	case reflect.Bool:
		val.SetBool(n != 0)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		val.SetInt(int64(n))
	default:
		val.SetUint(n)
	}
	return nil
}

// readInteger 从缓冲区读取整数值
// readInteger reads an integer value from the buffer
func (f *Field) readInteger(buf []byte, typ Type, order binary.ByteOrder) uint64 {
	switch typ {
	case Int8:
		return uint64(int64(int8(buf[0])))
	case Int16:
		return uint64(int64(int16(order.Uint16(buf))))
	case Int32:
		return uint64(int64(int32(order.Uint32(buf))))
	case Int64:
		return uint64(int64(order.Uint64(buf)))
	case Bool, Uint8:
		return uint64(buf[0])
	case Uint16:
		return uint64(order.Uint16(buf))
	case Uint32:
		return uint64(order.Uint32(buf))
	case Uint64:
		return uint64(order.Uint64(buf))
	default:
		return 0
	}
}

// getIntegerValue extracts an integer value from a reflect.Value.
// getIntegerValue 从 reflect.Value 中提取整数值。
func (f *Field) getIntegerValue(val reflect.Value) uint64 {
	switch f.kind {
	case reflect.Bool:
		if val.Bool() {
			return 1
		}
		return 0
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return uint64(val.Int())
	default:
		return val.Uint()
	}
}

// writeInteger 将整数值写入缓冲区
// writeInteger writes an integer value to the buffer
func (f *Field) writeInteger(buf []byte, n uint64, typ Type, order binary.ByteOrder) error {
	switch typ {
	case Bool:
		if n != 0 {
			buf[0] = 1
		} else {
			buf[0] = 0
		}
	case Int8, Uint8:
		buf[0] = byte(n)
	case Int16, Uint16:
		order.PutUint16(buf, uint16(n))
	case Int32, Uint32:
		order.PutUint32(buf, uint32(n))
	case Int64, Uint64:
		order.PutUint64(buf, n)
	default:
		return fmt.Errorf("unsupported integer type: %v", typ)
	}
	return nil
}

// writeFloat 将浮点数值写入缓冲区
// writeFloat writes a float value to the buffer
func (f *Field) writeFloat(buf []byte, n float64, typ Type, order binary.ByteOrder) error {
	switch typ {
	case Float32:
		order.PutUint32(buf, math.Float32bits(float32(n)))
	case Float64:
		order.PutUint64(buf, math.Float64bits(n))
	default:
		return fmt.Errorf("unsupported float type: %v", typ)
	}
	return nil
}
