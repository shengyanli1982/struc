package struc

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"strings"
)

// 格式映射表定义了 Go 类型到二进制格式字符的映射关系
// Format mapping table defines the mapping from Go types to binary format characters
var formatMap = map[Type]string{
	Int8:    "b", // signed char (有符号字符)
	Uint8:   "B", // unsigned char (无符号字符)
	Int16:   "h", // short (短整数)
	Uint16:  "H", // unsigned short (无符号短整数)
	Int32:   "i", // int (整数)
	Uint32:  "I", // unsigned int (无符号整数)
	Int64:   "q", // long long (长整数)
	Uint64:  "Q", // unsigned long long (无符号长整数)
	Float32: "f", // float (单精度浮点数)
	Float64: "d", // double (双精度浮点数)
	String:  "s", // char[] (字符数组)
	Bool:    "?", // _Bool (布尔值)
	Pad:     "x", // padding (填充字节)
}

// GetFormatString 返回结构体的格式字符串，用于描述二进制数据的布局。
// 格式类似于 Python 的 struct 模块，例如 "<10sHHb"。
//
// GetFormatString returns a format string that describes the binary layout of a struct.
// The format is similar to Python's struct module, e.g., "<10sHHb".
func GetFormatString(data interface{}) (string, error) {
	// 获取并验证输入数据
	// Get and validate input data
	value, err := validateInput(data)
	if err != nil {
		return "", err
	}

	// 解析字段
	// Parse fields
	fields, err := parseFields(value)
	if err != nil && value.NumField() > 0 {
		return "", fmt.Errorf("failed to parse fields: %w", err)
	}

	// 确定字节序并生成格式字符串
	// Determine endianness and generate format string
	return buildFormatString(fields)
}

// validateInput 验证输入数据并返回结构体的反射值。
// 如果输入不是结构体或结构体指针，则返回错误。
//
// validateInput validates the input data and returns the reflection value of the struct.
// Returns an error if the input is not a struct or pointer to struct.
func validateInput(data interface{}) (reflect.Value, error) {
	value := reflect.ValueOf(data)

	// 解引用所有指针
	// Dereference all pointers
	for value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return reflect.Value{}, fmt.Errorf("data must be a struct or pointer to struct")
		}
		value = value.Elem()
	}

	// 确保是结构体类型
	// Ensure it's a struct type
	if value.Kind() != reflect.Struct {
		return reflect.Value{}, fmt.Errorf("data must be a struct or pointer to struct")
	}

	return value, nil
}

// buildFormatString 构建格式字符串。
// 首先确定字节序，然后生成所有字段的格式。
//
// buildFormatString builds the format string.
// First determines the endianness, then generates the format for all fields.
func buildFormatString(fields Fields) (string, error) {
	var format strings.Builder

	// 确定字节序
	// Determine endianness
	endianness := determineEndianness(fields)
	format.WriteString(endianness)

	// 生成字段格式
	// Generate field formats
	if err := formatFields(&format, fields); err != nil {
		return "", err
	}

	return format.String(), nil
}

// determineEndianness 确定整个结构体的字节序。
// 如果任何字段指定了小端序，则整个结构体使用小端序。
//
// determineEndianness determines the endianness for the entire struct.
// If any field specifies little-endian, the entire struct uses little-endian.
func determineEndianness(fields Fields) string {
	for _, field := range fields {
		if field != nil && field.ByteOrder == binary.LittleEndian {
			return "<"
		}
	}
	return ">"
}

// formatFields 处理字段集合的格式化。
// 遍历所有字段，为每个字段生成对应的格式字符。
//
// formatFields handles the formatting of a collection of fields.
// Iterates through all fields, generating corresponding format characters for each field.
func formatFields(format *strings.Builder, fields Fields) error {
	for _, field := range fields {
		if field == nil {
			continue
		}

		if err := formatField(format, field); err != nil {
			return err
		}
	}
	return nil
}

// formatField 处理单个字段的格式化。
// 根据字段类型选择不同的格式化方式。
//
// formatField handles the formatting of a single field.
// Chooses different formatting methods based on field type.
func formatField(format *strings.Builder, field *Field) error {
	// 处理嵌套结构体
	// Handle nested structs
	if field.Type == Struct {
		return formatFields(format, field.NestFields)
	}

	// 处理 sizeof 字段
	// Handle sizeof fields
	if len(field.Sizeof) > 0 {
		return formatSizeofField(format, field)
	}

	// 跳过 sizefrom 字段
	// Skip sizefrom fields
	if len(field.Sizefrom) > 0 {
		return nil
	}

	// 处理数组和切片
	// Handle arrays and slices
	if field.IsArray || field.IsSlice {
		return formatArrayField(format, field)
	}

	// 处理基本类型
	// Handle basic types
	return formatBasicField(format, field)
}

// formatSizeofField 处理 sizeof 字段的格式化。
// 将字段类型转换为对应的格式字符。
//
// formatSizeofField handles the formatting of sizeof fields.
// Converts field type to corresponding format character.
func formatSizeofField(format *strings.Builder, field *Field) error {
	formatChar, ok := formatMap[field.Type]
	if !ok {
		return fmt.Errorf("unsupported sizeof type for field %s", field.Name)
	}
	format.WriteString(formatChar)
	return nil
}

// formatArrayField 处理数组和切片字段的格式化。
// 根据数组的基本类型和长度生成格式字符串。
//
// formatArrayField handles the formatting of array and slice fields.
// Generates format string based on array's base type and length.
func formatArrayField(format *strings.Builder, field *Field) error {
	// 验证长度
	// Validate length
	if field.Length <= 0 && len(field.Sizefrom) == 0 {
		return fmt.Errorf("field `%s` is a slice with no length or sizeof field", field.Name)
	}

	// 获取基本类型
	// Get base type
	baseType := field.defType
	if baseType == 0 {
		baseType = field.Type
	}

	// 处理填充字节
	// Handle padding bytes
	if field.Type == Pad {
		format.WriteString(fmt.Sprintf("%d%s", field.Length, formatMap[Pad]))
		return nil
	}

	// 处理字节数组和字符串
	// Handle byte arrays and strings
	if baseType == Uint8 || baseType == String || field.Type == String {
		format.WriteString(fmt.Sprintf("%d%s", field.Length, formatMap[String]))
		return nil
	}

	// 处理其他类型的数组
	// Handle other array types
	return formatArrayElements(format, field.Length, baseType)
}

// formatArrayElements 处理数组元素的格式化。
// 重复生成数组元素的格式字符。
//
// formatArrayElements handles the formatting of array elements.
// Repeatedly generates format characters for array elements.
func formatArrayElements(format *strings.Builder, length int, baseType Type) error {
	formatChar, ok := formatMap[baseType]
	if !ok {
		return fmt.Errorf("unsupported array element type: %v", baseType)
	}

	for i := 0; i < length; i++ {
		format.WriteString(formatChar)
	}
	return nil
}

// formatBasicField 处理基本类型字段的格式化。
// 根据字段类型生成对应的格式字符。
//
// formatBasicField handles the formatting of basic type fields.
// Generates corresponding format character based on field type.
func formatBasicField(format *strings.Builder, field *Field) error {
	formatChar, ok := formatMap[field.Type]
	if !ok {
		return fmt.Errorf("unsupported type for field %s: %v", field.Name, field.Type)
	}

	switch field.Type {
	case String:
		if field.Length > 0 {
			format.WriteString(fmt.Sprintf("%d%s", field.Length, formatChar))
		} else if len(field.Sizefrom) == 0 {
			return fmt.Errorf("field `%s` is a string with no length or sizeof field", field.Name)
		}
	case Pad:
		if field.Length > 0 {
			format.WriteString(fmt.Sprintf("%d%s", field.Length, formatChar))
		} else {
			format.WriteString(formatChar)
		}
	default:
		format.WriteString(formatChar)
	}
	return nil
}
