package struc

import (
	"testing"
)

// 基本类型测试结构体
// Basic types test struct
type FormatTestBasic struct {
	A int8    `struc:"int8"`
	B uint8   `struc:"uint8"`
	C int16   `struc:"int16"`
	D uint16  `struc:"uint16,little"`
	E int32   `struc:"int32"`
	F uint32  `struc:"uint32"`
	G int64   `struc:"int64"`
	H uint64  `struc:"uint64"`
	I float32 `struc:"float32"`
	J float64 `struc:"float64"`
}

// 字符串和字节切片测试结构体
// String and byte slice test struct
type FormatTestString struct {
	CLen int    `struc:"int32,sizeof=C"` // C 的长度字段 / Length field for C
	A    string `struc:"[10]byte"`       // 固定长度字符串 / Fixed-length string
	B    []byte `struc:"[5]byte"`        // 固定长度字节切片 / Fixed-length byte slice
	C    string `struc:"[]byte"`         // 动态长度字符串 / Dynamic-length string
	DLen int    `struc:"int16,sizeof=D"` // D 的长度字段 / Length field for D
	D    []byte `struc:"[]byte"`         // 动态长度字节切片 / Dynamic-length byte slice
}

// 填充字节测试结构体
// Padding bytes test struct
type FormatTestPadding struct {
	A int32  `struc:"int32"`
	B []byte `struc:"[4]pad"` // 4字节填充 / 4-byte padding
	C uint16 `struc:"uint16"`
	D []byte `struc:"[2]pad"` // 2字节填充 / 2-byte padding
	E int64  `struc:"int64"`
}

// 大小端混合测试结构体
// Mixed endianness test struct
type FormatTestEndian struct {
	A uint16 `struc:"uint16,big"`    // 大端序 / Big-endian
	B uint32 `struc:"uint32,little"` // 小端序 / Little-endian
	C uint64 `struc:"uint64,big"`    // 大端序 / Big-endian
	D int16  `struc:"int16,little"`  // 小端序 / Little-endian
}

// 大小引用测试结构体
// Size reference test struct
type FormatTestSizeof struct {
	Size   int `struc:"int32,sizeof=Data"`
	Data   []byte
	Length int    `struc:"uint16,sizeof=Text"`
	Text   string `struc:"[]byte"`
	Count  int    `struc:"int8,sizeof=Array"`
	Array  []int  `struc:"[]int32"`
}

// 嵌套结构体测试
// Nested struct test
type FormatTestNested struct {
	Header struct {
		Size    uint32 `struc:"uint32"`
		Version uint16 `struc:"uint16"`
	}
	Body struct {
		Data []byte `struc:"[16]byte"`
		Type uint8  `struc:"uint8"`
	}
}

// 数组测试结构体
// Array test struct
type FormatTestArray struct {
	IntArray   [4]int32   `struc:"[4]int32"`
	ByteArray  [8]byte    `struc:"[8]byte"`
	FloatArray [2]float32 `struc:"[2]float32"`
}

func TestGetFormatString(t *testing.T) {
	tests := []struct {
		name    string
		data    interface{}
		want    string
		wantErr bool
		errMsg  string
	}{
		{
			name: "basic types",
			data: &FormatTestBasic{},
			want: "<bBhHiIqQfd", // 按照 formatMap 映射: Int8(b), Uint8(B), Int16(h), Uint16(H), Int32(i), Uint32(I), Int64(q), Uint64(Q), Float32(f), Float64(d)
		},
		{
			name: "string types",
			data: &FormatTestString{},
			want: ">i10s5sh", // 按照 formatMap 映射: Int32(i), [10]byte(10s), [5]byte(5s), Int16(h)
		},
		{
			name: "padding",
			data: &FormatTestPadding{},
			want: ">i4xH2xq", // 按照 formatMap 映射: Int32(i), Pad(4x), Uint16(H), Pad(2x), Int64(q)
		},
		{
			name: "mixed endianness",
			data: &FormatTestEndian{},
			want: "<HIQh", // 按照 formatMap 映射: Uint16(H), Uint32(I), Uint64(Q), Int16(h)
		},
		{
			name: "sizeof fields",
			data: &FormatTestSizeof{},
			want: ">iHb", // 按照 formatMap 映射: Int32(i), Uint16(H), Int8(b)
		},
		{
			name: "nested struct",
			data: &FormatTestNested{},
			want: ">IH16sB", // 按照 formatMap 映射: Uint32(I), Uint16(H), String(16s), Uint8(B)
		},
		{
			name: "array types",
			data: &FormatTestArray{},
			want: ">iiii8sff", // 按照 formatMap 映射: Int32(i,i,i,i), String(8s), Float32(f,f)
		},
		{
			name:    "non-struct",
			data:    123,
			wantErr: true,
			errMsg:  "data must be a struct or pointer to struct",
		},
		{
			name:    "nil pointer",
			data:    (*FormatTestBasic)(nil),
			wantErr: true,
			errMsg:  "data must be a struct or pointer to struct",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetFormatString(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetFormatString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				if err.Error() != tt.errMsg {
					t.Errorf("GetFormatString() error message = %v, want %v", err.Error(), tt.errMsg)
				}
				return
			}
			if got != tt.want {
				t.Errorf("GetFormatString() = %v, want %v", got, tt.want)
			}
		})
	}
}

// 测试特殊情况
// Test special cases
func TestGetFormatStringSpecialCases(t *testing.T) {
	// 空结构体
	// Empty struct
	type EmptyStruct struct{}
	got, err := GetFormatString(&EmptyStruct{})
	if err != nil {
		t.Errorf("GetFormatString() empty struct error = %v", err)
	}
	if got != ">" {
		t.Errorf("GetFormatString() empty struct = %v, want >", got)
	}

	// 只有填充字节的结构体
	// Struct with only padding
	type PaddingOnly struct {
		Pad1 []byte `struc:"[8]pad"`
		Pad2 []byte `struc:"[4]pad"`
	}
	got, err = GetFormatString(&PaddingOnly{})
	if err != nil {
		t.Errorf("GetFormatString() padding only error = %v", err)
	}
	if got != ">8x4x" {
		t.Errorf("GetFormatString() padding only = %v, want >8x4x", got)
	}

	// 只有大小引用字段的结构体
	// Struct with only size reference fields
	type SizeRefOnly struct {
		Size1 int `struc:"int32,sizeof=Data1"`
		Data1 []byte
		Size2 int `struc:"int16,sizeof=Data2"`
		Data2 []byte
	}
	got, err = GetFormatString(&SizeRefOnly{})
	if err != nil {
		t.Errorf("GetFormatString() size ref only error = %v", err)
	}
	if got != ">ih" {
		t.Errorf("GetFormatString() size ref only = %v, want >ih", got)
	}
}
