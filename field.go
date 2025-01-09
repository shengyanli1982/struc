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

// bufferPool is used to reduce allocations when packing/unpacking
// bufferPool 用于减少打包/解包时的内存分配
var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// String returns a string representation of the field.
// String 返回字段的字符串表示。
func (f *Field) String() string {
	if f.Type == Pad {
		return fmt.Sprintf("{type: Pad, len: %d}", f.Len)
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("type: %s", f.Type))
	if f.Order != nil {
		parts = append(parts, fmt.Sprintf("order: %v", f.Order))
	}
	if f.Sizefrom != nil {
		parts = append(parts, fmt.Sprintf("sizefrom: %v", f.Sizefrom))
	} else if f.Len > 0 {
		parts = append(parts, fmt.Sprintf("len: %d", f.Len))
	}
	if f.Sizeof != nil {
		parts = append(parts, fmt.Sprintf("sizeof: %v", f.Sizeof))
	}

	return "{" + joinStrings(parts, ", ") + "}"
}

// Size calculates the size of the field in bytes.
// Size 计算字段的字节大小。
func (f *Field) Size(val reflect.Value, options *Options) int {
	typ := f.Type.Resolve(options)
	size := 0

	switch typ {
	case Struct:
		if f.Slice {
			length := val.Len()
			for i := 0; i < length; i++ {
				size += f.Fields.Sizeof(val.Index(i), options)
			}
		} else {
			size = f.Fields.Sizeof(val, options)
		}
	case Pad:
		size = f.Len
	case CustomType:
		if c, ok := val.Addr().Interface().(Custom); ok {
			size = c.Size(options)
		}
	default:
		elemSize := typ.Size()
		if f.Slice || f.kind == reflect.String {
			length := val.Len()
			if f.Len > 1 {
				length = f.Len
			}
			size = length * elemSize
		} else {
			size = elemSize
		}
	}

	// Apply byte alignment if specified
	if align := options.ByteAlign; align > 0 {
		if remainder := size % align; remainder != 0 {
			size += align - remainder
		}
	}

	return size
}

// packVal packs a single value into the buffer.
// packVal 将单个值打包到缓冲区中。
func (f *Field) packVal(buf []byte, val reflect.Value, length int, options *Options) (size int, err error) {
	order := f.Order
	if options.Order != nil {
		order = options.Order
	}
	if f.Ptr {
		val = val.Elem()
	}

	typ := f.Type.Resolve(options)
	switch typ {
	case Struct:
		return f.Fields.Pack(buf, val, options)
	case Bool, Int8, Int16, Int32, Int64, Uint8, Uint16, Uint32, Uint64:
		size = typ.Size()
		n := f.getIntegerValue(val)
		if err := f.writeInteger(buf, n, typ, order); err != nil {
			return 0, fmt.Errorf("failed to write integer: %w", err)
		}
	case Float32, Float64:
		size = typ.Size()
		n := val.Float()
		if err := f.writeFloat(buf, n, typ, order); err != nil {
			return 0, fmt.Errorf("failed to write float: %w", err)
		}
	case String:
		var data []byte
		switch f.kind {
		case reflect.String:
			data = []byte(val.String())
		default:
			data = val.Bytes()
		}
		size = len(data)
		copy(buf, data)
	case CustomType:
		if c, ok := val.Addr().Interface().(Custom); ok {
			return c.Pack(buf, options)
		}
		return 0, fmt.Errorf("failed to pack custom type: %v", val.Type())
	default:
		return 0, fmt.Errorf("unsupported type for packing: %v", typ)
	}
	return size, nil
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

// writeInteger writes an integer value to the buffer.
// writeInteger 将整数值写入缓冲区。
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

// writeFloat writes a float value to the buffer.
// writeFloat 将浮点值写入缓冲区。
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

// Pack packs the field value into the buffer.
// Pack 将字段值打包到缓冲区中。
func (f *Field) Pack(buf []byte, val reflect.Value, length int, options *Options) (int, error) {
	if typ := f.Type.Resolve(options); typ == Pad {
		for i := 0; i < length; i++ {
			buf[i] = 0
		}
		return length, nil
	}

	if f.Slice {
		return f.packSlice(buf, val, length, options)
	}
	return f.packVal(buf, val, length, options)
}

// packSlice packs a slice value into the buffer.
// packSlice 将切片值打包到缓冲区中。
func (f *Field) packSlice(buf []byte, val reflect.Value, length int, options *Options) (int, error) {
	end := val.Len()
	typ := f.Type.Resolve(options)

	// Optimize for byte slices and strings
	if !f.Array && typ == Uint8 && (f.defType == Uint8 || f.kind == reflect.String) {
		var data []byte
		if f.kind == reflect.String {
			data = []byte(val.String())
		} else {
			data = val.Bytes()
		}
		copy(buf, data)
		if end < length {
			// Zero-fill the remaining space
			for i := end; i < length; i++ {
				buf[i] = 0
			}
			return length, nil
		}
		return end, nil
	}

	// Pack slice elements
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

// joinStrings joins strings with a separator.
// joinStrings 使用分隔符连接字符串。
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	if len(strs) == 1 {
		return strs[0]
	}

	n := len(sep) * (len(strs) - 1)
	for i := 0; i < len(strs); i++ {
		n += len(strs[i])
	}

	var b bytes.Buffer
	b.Grow(n)
	b.WriteString(strs[0])
	for _, s := range strs[1:] {
		b.WriteString(sep)
		b.WriteString(s)
	}
	return b.String()
}

func (f *Field) unpackVal(buf []byte, val reflect.Value, length int, options *Options) error {
	order := f.Order
	if options.Order != nil {
		order = options.Order
	}
	if f.Ptr {
		val = val.Elem()
	}
	typ := f.Type.Resolve(options)
	switch typ {
	case Float32, Float64:
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
		default:
			return fmt.Errorf("struc: refusing to unpack float into field %s of type %s", f.Name, f.kind.String())
		}
	case Bool, Int8, Int16, Int32, Int64, Uint8, Uint16, Uint32, Uint64:
		var n uint64
		switch typ {
		case Int8:
			n = uint64(int64(int8(buf[0])))
		case Int16:
			n = uint64(int64(int16(order.Uint16(buf))))
		case Int32:
			n = uint64(int64(int32(order.Uint32(buf))))
		case Int64:
			n = uint64(int64(order.Uint64(buf)))
		case Bool, Uint8:
			n = uint64(buf[0])
		case Uint16:
			n = uint64(order.Uint16(buf))
		case Uint32:
			n = uint64(order.Uint32(buf))
		case Uint64:
			n = uint64(order.Uint64(buf))
		}
		switch f.kind {
		case reflect.Bool:
			val.SetBool(n != 0)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			val.SetInt(int64(n))
		default:
			val.SetUint(n)
		}
	default:
		panic(fmt.Sprintf("no unpack handler for type: %s", typ))
	}
	return nil
}

func (f *Field) Unpack(buf []byte, val reflect.Value, length int, options *Options) error {
	typ := f.Type.Resolve(options)
	if typ == Pad || f.kind == reflect.String {
		if typ == Pad {
			return nil
		} else {
			val.SetString(string(buf))
			return nil
		}
	} else if f.Slice {
		if val.Cap() < length {
			val.Set(reflect.MakeSlice(val.Type(), length, length))
		} else if val.Len() < length {
			val.Set(val.Slice(0, length))
		}
		// special case byte slices for performance
		if !f.Array && typ == Uint8 && f.defType == Uint8 {
			copy(val.Bytes(), buf[:length])
			return nil
		}
		pos := 0
		size := typ.Size()
		for i := 0; i < length; i++ {
			if err := f.unpackVal(buf[pos:pos+size], val.Index(i), 1, options); err != nil {
				return err
			}
			pos += size
		}
		return nil
	} else {
		return f.unpackVal(buf, val, length, options)
	}
}
