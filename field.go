// Package struc implements binary packing and unpacking for Go structs.
// struc 包实现了 Go 结构体的二进制打包和解包功能。
package struc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"sync"
)

// fieldBufferPool 用于减少打包/解包时的内存分配
// 通过复用缓冲区来提高性能
//
// fieldBufferPool is used to reduce memory allocations during packing/unpacking
// Improves performance by reusing buffers
var fieldBufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 1024)) // 预分配 1KB 的初始容量 / Pre-allocate 1KB initial capacity
	},
}

// Field 表示结构体中的单个字段
// 包含了字段的所有元数据信息，用于二进制打包和解包
//
// Field represents a single field in a struct
// Contains all metadata about the field for binary packing and unpacking
type Field struct {
	Name       string           // 字段名称 / Field name
	IsPointer  bool             // 字段是否为指针类型 / Whether the field is a pointer type
	Index      int              // 字段在结构体中的索引 / Field index in struct
	Type       Type             // 字段的二进制类型 / Binary type of the field
	defType    Type             // 默认的二进制类型 / Default binary type
	IsArray    bool             // 字段是否为数组 / Whether the field is an array
	IsSlice    bool             // 字段是否为切片 / Whether the field is a slice
	Length     int              // 数组/固定切片的长度 / Length for arrays/fixed slices
	ByteOrder  binary.ByteOrder // 字段的字节序 / Byte order of the field
	Sizeof     []int            // sizeof 引用的字段索引 / Field indices referenced by sizeof
	Sizefrom   []int            // 大小引用的字段索引 / Field indices referenced for size
	NestFields Fields           // 嵌套结构体的字段 / Fields of nested struct
	kind       reflect.Kind     // Go 的反射类型 / Go reflection kind
}

// String 返回字段的字符串表示
// 用于调试和日志记录
//
// String returns a string representation of the field
// Used for debugging and logging
func (f *Field) String() string {
	if f.Type == Pad {
		return fmt.Sprintf("{type: Pad, len: %d}", f.Length)
	}

	buffer := getBuffer()
	defer putBuffer(buffer)

	buffer.WriteString("{")
	buffer.WriteString(fmt.Sprintf("type: %s", f.Type))

	if f.ByteOrder != nil {
		buffer.WriteString(fmt.Sprintf(", order: %v", f.ByteOrder))
	}
	if f.Sizefrom != nil {
		buffer.WriteString(fmt.Sprintf(", sizefrom: %v", f.Sizefrom))
	} else if f.Length > 0 {
		buffer.WriteString(fmt.Sprintf(", len: %d", f.Length))
	}
	if f.Sizeof != nil {
		buffer.WriteString(fmt.Sprintf(", sizeof: %v", f.Sizeof))
	}
	buffer.WriteString("}")

	return buffer.String()
}

// Size 计算字段在二进制格式中占用的字节数
// 考虑了对齐和填充要求
//
// Size calculates the number of bytes the field occupies in binary format
// Takes into account alignment and padding requirements
func (f *Field) Size(fieldValue reflect.Value, options *Options) int {
	resolvedType := f.Type.Resolve(options)
	totalSize := 0

	switch resolvedType {
	case Struct:
		totalSize = f.calculateStructSize(fieldValue, options)
	case Pad:
		totalSize = f.Length
	case CustomType:
		totalSize = f.calculateCustomSize(fieldValue, options)
	default:
		totalSize = f.calculateBasicSize(fieldValue, resolvedType, options)
	}

	return f.alignSize(totalSize, options)
}

// calculateStructSize 计算结构体类型的字节大小
// 处理普通结构体和结构体切片
//
// calculateStructSize calculates size for struct types
// Handles both regular structs and slices of structs
func (f *Field) calculateStructSize(fieldValue reflect.Value, options *Options) int {
	if f.IsSlice {
		sliceLength := fieldValue.Len()
		totalSize := 0
		for i := 0; i < sliceLength; i++ {
			totalSize += f.NestFields.Sizeof(fieldValue.Index(i), options)
		}
		return totalSize
	}
	return f.NestFields.Sizeof(fieldValue, options)
}

// calculateCustomSize 计算自定义类型的字节大小
// 通过调用类型的 Size 方法获取
//
// calculateCustomSize calculates size for custom types
// Gets size by calling the type's Size method
func (f *Field) calculateCustomSize(fieldValue reflect.Value, options *Options) int {
	if customType, ok := fieldValue.Addr().Interface().(Custom); ok {
		return customType.Size(options)
	}
	return 0
}

// calculateBasicSize 计算基本类型的字节大小
// 处理固定大小类型和变长类型(如切片和字符串)
//
// calculateBasicSize calculates size for basic types
// Handles both fixed-size types and variable-length types (slices and strings)
func (f *Field) calculateBasicSize(fieldValue reflect.Value, resolvedType Type, options *Options) int {
	elementSize := resolvedType.Size()
	if f.IsSlice || f.kind == reflect.String {
		length := fieldValue.Len()
		if f.Length > 1 {
			length = f.Length // 使用指定的固定长度 / Use specified fixed length
		}
		return length * elementSize
	}
	return elementSize
}

// alignSize 根据 ByteAlign 选项对齐大小
// 确保字段按指定的字节边界对齐
//
// alignSize aligns the size according to ByteAlign option
// Ensures fields are aligned on specified byte boundaries
func (f *Field) alignSize(size int, options *Options) int {
	if alignment := options.ByteAlign; alignment > 0 {
		if remainder := size % alignment; remainder != 0 {
			size += alignment - remainder
		}
	}
	return size
}

// packSingleValue 将单个值打包到缓冲区中
// 根据字段类型选择适当的打包方法
//
// packSingleValue packs a single value into the buffer
// Chooses appropriate packing method based on field type
func (f *Field) packSingleValue(buffer []byte, fieldValue reflect.Value, length int, options *Options) (size int, err error) {
	// 获取字节序并处理指针类型
	// Get byte order and handle pointer type
	byteOrder := f.determineByteOrder(options)
	if f.IsPointer {
		fieldValue = fieldValue.Elem()
	}

	// 解析类型并根据类型选择相应的打包方法
	// Resolve type and choose appropriate packing method
	resolvedType := f.Type.Resolve(options)
	switch resolvedType {
	case Struct:
		return f.NestFields.Pack(buffer, fieldValue, options)
	case Bool, Int8, Int16, Int32, Int64, Uint8, Uint16, Uint32, Uint64:
		return f.packIntegerValue(buffer, fieldValue, resolvedType, byteOrder)
	case Float32, Float64:
		return f.packFloat(buffer, fieldValue, resolvedType, byteOrder)
	case String:
		return f.packString(buffer, fieldValue)
	case CustomType:
		return f.packCustom(buffer, fieldValue, options)
	default:
		return 0, fmt.Errorf("unsupported type for packing: %v", resolvedType)
	}
}

// determineByteOrder 返回要使用的字节序
// 优先使用选项中指定的字节序，否则使用字段自身的字节序
//
// determineByteOrder returns the byte order to use
// Prioritizes byte order from options, falls back to field's byte order
func (f *Field) determineByteOrder(options *Options) binary.ByteOrder {
	if options.Order != nil {
		return options.Order
	}
	return f.ByteOrder
}

// packIntegerValue 打包整数值到缓冲区
// 支持所有整数类型和布尔类型
//
// packIntegerValue packs an integer value into the buffer
// Supports all integer types and boolean
func (f *Field) packIntegerValue(buffer []byte, fieldValue reflect.Value, resolvedType Type, byteOrder binary.ByteOrder) (int, error) {
	intValue := f.getIntegerValue(fieldValue)
	valueSize := resolvedType.Size()
	if err := f.writeInteger(buffer, intValue, resolvedType, byteOrder); err != nil {
		return 0, fmt.Errorf("failed to write integer: %w", err)
	}
	return valueSize, nil
}

// packFloat 打包浮点数值到缓冲区
// 将浮点数转换为二进制格式并写入缓冲区
//
// packFloat packs a float value into the buffer
// Converts float to binary format and writes to buffer
func (f *Field) packFloat(buffer []byte, fieldValue reflect.Value, resolvedType Type, byteOrder binary.ByteOrder) (int, error) {
	floatValue := fieldValue.Float()
	valueSize := resolvedType.Size()
	if err := f.writeFloat(buffer, floatValue, resolvedType, byteOrder); err != nil {
		return 0, fmt.Errorf("failed to write float: %w", err)
	}
	return valueSize, nil
}

// packString 打包字符串或字节切片到缓冲区
// 处理字符串和 []byte 类型
//
// packString packs a string or byte slice into the buffer
// Handles both string and []byte types
func (f *Field) packString(buffer []byte, fieldValue reflect.Value) (int, error) {
	var data []byte
	switch f.kind {
	case reflect.String:
		data = []byte(fieldValue.String())
	default:
		data = fieldValue.Bytes()
	}
	dataSize := len(data)
	copy(buffer, data)
	return dataSize, nil
}

// packCustom 打包自定义类型到缓冲区
// 通过调用类型的 Pack 方法实现
//
// packCustom packs a custom type into the buffer
// Implemented by calling the type's Pack method
func (f *Field) packCustom(buffer []byte, fieldValue reflect.Value, options *Options) (int, error) {
	if customType, ok := fieldValue.Addr().Interface().(Custom); ok {
		return customType.Pack(buffer, options)
	}
	return 0, fmt.Errorf("failed to pack custom type: %v", fieldValue.Type())
}

// Pack 将字段值打包到缓冲区中
// 处理所有类型的字段，包括填充、切片和单个值
//
// Pack packs the field value into the buffer
// Handles all field types including padding, slices and single values
func (f *Field) Pack(buffer []byte, fieldValue reflect.Value, length int, options *Options) (int, error) {
	// 处理填充类型
	// Handle padding type
	if resolvedType := f.Type.Resolve(options); resolvedType == Pad {
		return f.packPaddingBytes(buffer, length)
	}

	// 根据字段是否为切片选择打包方法
	// Choose packing method based on whether the field is a slice
	if f.IsSlice {
		return f.packSliceValue(buffer, fieldValue, length, options)
	}
	return f.packSingleValue(buffer, fieldValue, length, options)
}

// packPaddingBytes 打包填充字节到缓冲区
// 用零值填充指定长度的空间
//
// packPaddingBytes packs padding bytes into the buffer
// Fills specified length with zero values
func (f *Field) packPaddingBytes(buffer []byte, length int) (int, error) {
	for i := 0; i < length; i++ {
		buffer[i] = 0
	}
	return length, nil
}

// packSliceValue 打包切片值到缓冲区
// 处理字节切片和其他类型的切片
//
// packSliceValue packs a slice value into the buffer
// Handles both byte slices and slices of other types
func (f *Field) packSliceValue(buffer []byte, fieldValue reflect.Value, length int, options *Options) (int, error) {
	resolvedType := f.Type.Resolve(options)

	// 对字节切片和字符串类型进行优化处理
	// Optimize handling for byte slices and strings
	if !f.IsArray && resolvedType == Uint8 && (f.defType == Uint8 || f.kind == reflect.String) {
		return f.packOptimizedByteSlice(buffer, fieldValue, fieldValue.Len(), length)
	}

	return f.packGenericSlice(buffer, fieldValue, fieldValue.Len(), length, options)
}

// packOptimizedByteSlice 优化字节切片的打包
// 直接复制数据并处理填充
//
// packOptimizedByteSlice optimizes packing for byte slices
// Direct copy of data with padding handling
func (f *Field) packOptimizedByteSlice(buffer []byte, fieldValue reflect.Value, dataLength, targetLength int) (int, error) {
	var data []byte
	if f.kind == reflect.String {
		data = []byte(fieldValue.String())
	} else {
		data = fieldValue.Bytes()
	}
	copy(buffer, data)
	if dataLength < targetLength {
		// 用零值填充剩余空间
		// Zero-fill the remaining space
		for i := dataLength; i < targetLength; i++ {
			buffer[i] = 0
		}
		return targetLength, nil
	}
	return dataLength, nil
}

// packGenericSlice 打包通用切片
// 逐个处理切片元素
//
// packGenericSlice packs a generic slice
// Processes slice elements one by one
func (f *Field) packGenericSlice(buffer []byte, fieldValue reflect.Value, dataLength, targetLength int, options *Options) (int, error) {
	position := 0
	var zeroValue reflect.Value
	if dataLength < targetLength {
		zeroValue = reflect.Zero(fieldValue.Type().Elem())
	}

	for i := 0; i < targetLength; i++ {
		currentValue := zeroValue
		if i < dataLength {
			currentValue = fieldValue.Index(i)
		}
		bytesWritten, err := f.packSingleValue(buffer[position:], currentValue, 1, options)
		if err != nil {
			return position, fmt.Errorf("failed to pack slice element %d: %w", i, err)
		}
		position += bytesWritten
	}

	return position, nil
}

// Unpack 从缓冲区中解包字段值
// 处理所有类型的字段值的解包
//
// Unpack unpacks the field value from the buffer
// Handles unpacking of all field value types
func (f *Field) Unpack(buffer []byte, fieldValue reflect.Value, length int, options *Options) error {
	resolvedType := f.Type.Resolve(options)

	// 处理填充和字符串类型
	// Handle padding and string types
	if resolvedType == Pad || f.kind == reflect.String {
		return f.unpackPaddingOrStringValue(buffer, fieldValue, resolvedType)
	}

	// 根据字段是否为切片选择解包方法
	// Choose unpacking method based on whether the field is a slice
	if f.IsSlice {
		return f.unpackSliceValue(buffer, fieldValue, length, options)
	}

	return f.unpackSingleValue(buffer, fieldValue, length, options)
}

// unpackPaddingOrStringValue 处理填充或字符串类型的解包
// 忽略填充类型，将字节数据转换为字符串
//
// unpackPaddingOrStringValue handles unpacking of padding or string types
// Ignores padding type, converts byte data to string
func (f *Field) unpackPaddingOrStringValue(buffer []byte, fieldValue reflect.Value, resolvedType Type) error {
	if resolvedType == Pad {
		return nil
	}
	fieldValue.SetString(string(buffer))
	return nil
}

// unpackSliceValue 处理切片类型的解包
// 调整切片容量并填充数据
//
// unpackSliceValue handles unpacking of slice types
// Adjusts slice capacity and fills data
func (f *Field) unpackSliceValue(buffer []byte, fieldValue reflect.Value, length int, options *Options) error {
	// 确保切片有足够的容量
	// Ensure slice has sufficient capacity
	if fieldValue.Cap() < length {
		fieldValue.Set(reflect.MakeSlice(fieldValue.Type(), length, length))
	} else if fieldValue.Len() < length {
		fieldValue.Set(fieldValue.Slice(0, length))
	}

	resolvedType := f.Type.Resolve(options)
	// 优化字节切片的处理
	// Optimize byte slice handling
	if !f.IsArray && resolvedType == Uint8 && f.defType == Uint8 {
		copy(fieldValue.Bytes(), buffer[:length])
		return nil
	}

	// 处理其他类型的切片
	// Handle other slice types
	elementSize := resolvedType.Size()
	for i := 0; i < length; i++ {
		position := i * elementSize
		if err := f.unpackSingleValue(buffer[position:position+elementSize], fieldValue.Index(i), 1, options); err != nil {
			return fmt.Errorf("failed to unpack slice element %d: %w", i, err)
		}
	}
	return nil
}

// unpackSingleValue 从缓冲区中解包单个值
// 根据类型选择适当的解包方法
//
// unpackSingleValue unpacks a single value from the buffer
// Chooses appropriate unpacking method based on type
func (f *Field) unpackSingleValue(buffer []byte, fieldValue reflect.Value, length int, options *Options) error {
	// 获取字节序并处理指针类型
	// Get byte order and handle pointer type
	byteOrder := f.determineByteOrder(options)
	if f.IsPointer {
		fieldValue = fieldValue.Elem()
	}

	// 根据类型选择相应的解包方法
	// Choose appropriate unpacking method based on type
	resolvedType := f.Type.Resolve(options)
	switch resolvedType {
	case Float32, Float64:
		return f.unpackFloat(buffer, fieldValue, resolvedType, byteOrder)
	case Bool, Int8, Int16, Int32, Int64, Uint8, Uint16, Uint32, Uint64:
		return f.unpackIntegerValue(buffer, fieldValue, resolvedType, byteOrder)
	default:
		return fmt.Errorf("no unpack handler for type: %s", resolvedType)
	}
}

// unpackFloat 解包浮点数值
// 支持 32 位和 64 位浮点数的解包
//
// unpackFloat unpacks a float value
// Supports unpacking of both 32-bit and 64-bit floating point numbers
func (f *Field) unpackFloat(buffer []byte, fieldValue reflect.Value, resolvedType Type, byteOrder binary.ByteOrder) error {
	var floatValue float64
	switch resolvedType {
	case Float32:
		floatValue = float64(math.Float32frombits(byteOrder.Uint32(buffer)))
	case Float64:
		floatValue = math.Float64frombits(byteOrder.Uint64(buffer))
	}

	switch f.kind {
	case reflect.Float32, reflect.Float64:
		fieldValue.SetFloat(floatValue)
		return nil
	default:
		return fmt.Errorf("struc: refusing to unpack float into field %s of type %s", f.Name, f.kind.String())
	}
}

// unpackIntegerValue 解包整数值
// 支持所有整数类型和布尔类型的解包
//
// unpackIntegerValue unpacks an integer value
// Supports unpacking of all integer types and boolean
func (f *Field) unpackIntegerValue(buffer []byte, fieldValue reflect.Value, resolvedType Type, byteOrder binary.ByteOrder) error {
	intValue := f.readInteger(buffer, resolvedType, byteOrder)

	switch f.kind {
	case reflect.Bool:
		fieldValue.SetBool(intValue != 0)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fieldValue.SetInt(int64(intValue))
	default:
		fieldValue.SetUint(intValue)
	}
	return nil
}

// readInteger 从缓冲区读取整数值
// 支持所有整数类型的读取
//
// readInteger reads an integer value from the buffer
// Supports reading of all integer types
func (f *Field) readInteger(buffer []byte, resolvedType Type, byteOrder binary.ByteOrder) uint64 {
	switch resolvedType {
	case Int8:
		return uint64(int64(int8(buffer[0])))
	case Int16:
		return uint64(int64(int16(byteOrder.Uint16(buffer))))
	case Int32:
		return uint64(int64(int32(byteOrder.Uint32(buffer))))
	case Int64:
		return uint64(int64(byteOrder.Uint64(buffer)))
	case Bool, Uint8:
		return uint64(buffer[0])
	case Uint16:
		return uint64(byteOrder.Uint16(buffer))
	case Uint32:
		return uint64(byteOrder.Uint32(buffer))
	case Uint64:
		return uint64(byteOrder.Uint64(buffer))
	default:
		return 0
	}
}

// getIntegerValue 从 reflect.Value 中提取整数值
// 处理布尔值、有符号和无符号整数
//
// getIntegerValue extracts an integer value from a reflect.Value
// Handles boolean values, signed and unsigned integers
func (f *Field) getIntegerValue(fieldValue reflect.Value) uint64 {
	switch f.kind {
	case reflect.Bool:
		if fieldValue.Bool() {
			return 1
		}
		return 0
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return uint64(fieldValue.Int())
	default:
		return fieldValue.Uint()
	}
}

// writeInteger 将整数值写入缓冲区
// 支持所有整数类型和布尔类型的写入
//
// writeInteger writes an integer value to the buffer
// Supports writing of all integer types and boolean
func (f *Field) writeInteger(buffer []byte, intValue uint64, resolvedType Type, byteOrder binary.ByteOrder) error {
	switch resolvedType {
	case Bool:
		if intValue != 0 {
			buffer[0] = 1
		} else {
			buffer[0] = 0
		}
	case Int8, Uint8:
		buffer[0] = byte(intValue)
	case Int16, Uint16:
		byteOrder.PutUint16(buffer, uint16(intValue))
	case Int32, Uint32:
		byteOrder.PutUint32(buffer, uint32(intValue))
	case Int64, Uint64:
		byteOrder.PutUint64(buffer, intValue)
	default:
		return fmt.Errorf("unsupported integer type: %v", resolvedType)
	}
	return nil
}

// writeFloat 将浮点数值写入缓冲区
// 根据类型（Float32/Float64）将浮点数转换为对应的二进制格式
// 使用指定的字节序写入缓冲区
//
// writeFloat writes a float value to the buffer
// Converts float to binary format based on type (Float32/Float64)
// Writes to buffer using specified byte order
func (f *Field) writeFloat(buffer []byte, floatValue float64, resolvedType Type, byteOrder binary.ByteOrder) error {
	switch resolvedType {
	case Float32:
		byteOrder.PutUint32(buffer, math.Float32bits(float32(floatValue)))
	case Float64:
		byteOrder.PutUint64(buffer, math.Float64bits(floatValue))
	default:
		return fmt.Errorf("unsupported float type: %v", resolvedType)
	}
	return nil
}
