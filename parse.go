package struc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// 标签格式示例：struc:"int32,big,sizeof=Data,skip,sizefrom=Len"
// 标签选项说明：
// - int32: 字段类型
// - big/little: 字节序
// - sizeof=Field: 指定字段大小来源
// - skip: 跳过该字段
// - sizefrom=Field: 指定长度来源字段
//
// Tag format example: struc:"int32,big,sizeof=Data,skip,sizefrom=Len"
// Tag options explanation:
// - int32: field type
// - big/little: byte order
// - sizeof=Field: specify size source field
// - skip: skip this field
// - sizefrom=Field: specify length source field

// strucTag 定义了结构体字段标签的解析结果
// 包含了字段的类型、字节序、大小引用等信息
//
// strucTag defines the parsed result of struct field tags
// Contains field type, byte order, size reference and other information
type strucTag struct {
	Type     string           // 字段类型（如 int32, uint8 等）/ Field type (e.g., int32, uint8)
	Order    binary.ByteOrder // 字节序（大端或小端）/ Byte order (big or little endian)
	Sizeof   string           // 大小引用字段名 / Size reference field name
	Skip     bool             // 是否跳过该字段 / Whether to skip this field
	Sizefrom string           // 长度来源字段名 / Length source field name
}

// parseStrucTag 解析结构体字段的标签
// 支持 struc 和 struct 两种标签名（向后兼容）
//
// parseStrucTag parses the tags of struct fields
// Supports both 'struc' and 'struct' tag names (backward compatibility)
func parseStrucTag(fieldTag reflect.StructTag) *strucTag {
	// 初始化标签结构体，默认使用大端字节序
	// Initialize tag struct with big-endian as default
	parsedTag := &strucTag{
		Order: binary.BigEndian,
	}

	// 获取 struc 标签，如果不存在则尝试获取 struct 标签（容错处理）
	// Get struc tag, fallback to struct tag if not found (error tolerance)
	tagString := fieldTag.Get("struc")
	if tagString == "" {
		tagString = fieldTag.Get("struct")
	}

	// 解析标签字符串中的每个选项
	// Parse each option in the tag string
	for _, option := range strings.Split(tagString, ",") {
		if strings.HasPrefix(option, "sizeof=") {
			// 解析 sizeof 选项，指定字段大小来源
			// Parse sizeof option, specifying size source field
			parts := strings.SplitN(option, "=", 2)
			parsedTag.Sizeof = parts[1]
		} else if strings.HasPrefix(option, "sizefrom=") {
			// 解析 sizefrom 选项，指定长度来源字段
			// Parse sizefrom option, specifying length source field
			parts := strings.SplitN(option, "=", 2)
			parsedTag.Sizefrom = parts[1]
		} else if option == "big" {
			// 设置大端字节序
			// Set big-endian byte order
			parsedTag.Order = binary.BigEndian
		} else if option == "little" {
			// 设置小端字节序
			// Set little-endian byte order
			parsedTag.Order = binary.LittleEndian
		} else if option == "skip" {
			// 设置跳过标志
			// Set skip flag
			parsedTag.Skip = true
		} else if option != "" {
			// 设置字段类型
			// Set field type
			parsedTag.Type = option
		}
	}
	return parsedTag
}

// arrayLengthParseRegex 用于匹配数组长度的正则表达式
// 格式：[数字]，如 [5]、[]
//
// arrayLengthParseRegex is a regular expression for matching array length
// Format: [number], e.g., [5], []
var arrayLengthParseRegex = regexp.MustCompile(`^\[(\d*)\]`)

// parseStructField 解析单个结构体字段，返回字段描述符和标签信息
// 处理字段的类型、数组/切片、指针等特性
//
// parseStructField parses a single struct field, returns field descriptor and tag info
// Handles field type, array/slice, pointer and other characteristics
func parseStructField(structField reflect.StructField) (fieldDesc *Field, fieldTag *strucTag, err error) {
	// 解析字段标签
	// Parse field tag
	fieldTag = parseStrucTag(structField.Tag)
	var ok bool

	// 从对象池获取 Field 对象
	// Get Field object from pool
	fieldDesc = acquireField()

	// 初始化字段描述符
	// Initialize field descriptor
	fieldDesc.Name = structField.Name
	fieldDesc.Length = 1
	fieldDesc.ByteOrder = fieldTag.Order
	fieldDesc.IsSlice = false
	fieldDesc.kind = structField.Type.Kind()

	// 处理特殊类型：数组、切片和指针
	// Handle special types: arrays, slices and pointers
	switch fieldDesc.kind {
	case reflect.Array:
		fieldDesc.IsSlice = true
		fieldDesc.IsArray = true
		fieldDesc.Length = structField.Type.Len()
		fieldDesc.kind = structField.Type.Elem().Kind()
	case reflect.Slice:
		fieldDesc.IsSlice = true
		fieldDesc.Length = -1
		fieldDesc.kind = structField.Type.Elem().Kind()
	case reflect.Ptr:
		fieldDesc.IsPointer = true
		fieldDesc.kind = structField.Type.Elem().Kind()
	}

	// 检查是否为自定义类型
	// Check if it's a custom type
	tempValue := reflect.New(structField.Type)
	if _, ok := tempValue.Interface().(Custom); ok {
		fieldDesc.Type = CustomType
		return
	}

	// 获取默认类型
	// Get default type
	var defTypeOk bool
	fieldDesc.defType, defTypeOk = typeKindToType[fieldDesc.kind]

	// 从结构体标签中查找类型
	// Find type in struct tag
	pureType := arrayLengthParseRegex.ReplaceAllLiteralString(fieldTag.Type, "")
	if fieldDesc.Type, ok = typeStrToType[pureType]; ok {
		fieldDesc.Length = 1
		// 解析数组长度
		// Parse array length
		matches := arrayLengthParseRegex.FindAllStringSubmatch(fieldTag.Type, -1)
		if len(matches) > 0 && len(matches[0]) > 1 {
			fieldDesc.IsSlice = true
			lengthStr := matches[0][1]
			if lengthStr == "" {
				fieldDesc.Length = -1 // 动态长度切片 / Dynamic length slice
			} else {
				fieldDesc.Length, err = strconv.Atoi(lengthStr)
			}
		}
		return
	}

	// 处理特殊类型 Size_t 和 Off_t
	// Handle special types Size_t and Off_t
	switch structField.Type {
	case reflect.TypeOf(Size_t(0)):
		fieldDesc.Type = SizeType
	case reflect.TypeOf(Off_t(0)):
		fieldDesc.Type = OffType
	default:
		if defTypeOk {
			fieldDesc.Type = fieldDesc.defType
		} else {
			// 如果发生错误，需要释放 Field 对象
			// If error occurs, need to release Field object
			releaseField(fieldDesc)
			err = fmt.Errorf("struc: Could not resolve field '%v' type '%v'.", structField.Name, structField.Type)
			fieldDesc = nil
		}
	}
	return
}

// handleSizeofTag 处理字段的 sizeof 标签
// 返回错误如果引用的字段不存在
//
// handleSizeofTag handles the sizeof tag of a field
// Returns error if the referenced field does not exist
func handleSizeofTag(fieldDesc *Field, fieldTag *strucTag, structType reflect.Type, field reflect.StructField, sizeofMap map[string][]int) error {
	if fieldTag.Sizeof != "" {
		targetField, ok := structType.FieldByName(fieldTag.Sizeof)
		if !ok {
			return fmt.Errorf("struc: `sizeof=%s` field does not exist", fieldTag.Sizeof)
		}
		fieldDesc.Sizeof = targetField.Index
		sizeofMap[fieldTag.Sizeof] = field.Index
	}
	return nil
}

// handleSizefromTag 处理字段的 sizefrom 标签
// 返回错误如果引用的字段不存在
//
// handleSizefromTag handles the sizefrom tag of a field
// Returns error if the referenced field does not exist
func handleSizefromTag(fieldDesc *Field, fieldTag *strucTag, structType reflect.Type, field reflect.StructField, sizeofMap map[string][]int) error {
	if sizefrom, ok := sizeofMap[field.Name]; ok {
		fieldDesc.Sizefrom = sizefrom
	}
	if fieldTag.Sizefrom != "" {
		sourceField, ok := structType.FieldByName(fieldTag.Sizefrom)
		if !ok {
			return fmt.Errorf("struc: `sizefrom=%s` field does not exist", fieldTag.Sizefrom)
		}
		fieldDesc.Sizefrom = sourceField.Index
	}
	return nil
}

// handleNestedStruct 处理嵌套结构体字段
// 递归解析嵌套结构体的字段
//
// handleNestedStruct handles nested struct fields
// Recursively parses fields of nested structs
func handleNestedStruct(fieldDesc *Field, field reflect.StructField) error {
	if fieldDesc.Type == Struct {
		fieldType := field.Type
		if fieldDesc.IsPointer {
			fieldType = fieldType.Elem()
		}
		if fieldDesc.IsSlice {
			fieldType = fieldType.Elem()
		}
		tempValue := reflect.New(fieldType)
		nestedFields, err := parseFields(tempValue.Elem())
		if err != nil {
			return err
		}
		fieldDesc.NestFields = nestedFields
	}
	return nil
}

// validateSliceLength 验证切片长度
// 确保动态长度切片有对应的长度来源字段
//
// validateSliceLength validates slice length
// Ensures dynamic length slices have corresponding length source fields
func validateSliceLength(fieldDesc *Field, field reflect.StructField) error {
	if fieldDesc.Length == -1 && fieldDesc.Sizefrom == nil {
		return fmt.Errorf("struc: field `%s` is a slice with no length or sizeof field", field.Name)
	}
	return nil
}

// parseFieldsLocked 在加锁状态下解析结构体的所有字段
// 此函数处理嵌套结构体、数组和切片等复杂类型
//
// parseFieldsLocked parses all fields of a struct while locked
// This function handles complex types like nested structs, arrays and slices
func parseFieldsLocked(structValue reflect.Value) (Fields, error) {
	// 解引用指针，直到获取到非指针类型
	for structValue.Kind() == reflect.Ptr {
		structValue = structValue.Elem()
	}
	structType := structValue.Type()

	// 检查结构体是否有字段
	if structValue.NumField() < 1 {
		return nil, errors.New("struc: Struct has no fields.")
	}

	// 创建大小引用映射和字段切片
	sizeofMap := make(map[string][]int)
	fields := make(Fields, structValue.NumField())

	// 遍历所有字段
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)

		// 解析字段和标签
		fieldDesc, fieldTag, err := parseStructField(field)
		if err != nil {
			releaseFields(fields)
			return nil, err
		}

		// 跳过不需要处理的字段
		if fieldTag.Skip || !structValue.Field(i).CanSet() {
			continue
		}

		fieldDesc.Index = i

		// 处理各种标签和验证
		if err := handleSizeofTag(fieldDesc, fieldTag, structType, field, sizeofMap); err != nil {
			releaseFields(fields)
			return nil, err
		}

		if err := handleSizefromTag(fieldDesc, fieldTag, structType, field, sizeofMap); err != nil {
			releaseFields(fields)
			return nil, err
		}

		if err := validateSliceLength(fieldDesc, field); err != nil {
			releaseFields(fields)
			return nil, err
		}

		if err := handleNestedStruct(fieldDesc, field); err != nil {
			releaseFields(fields)
			return nil, err
		}

		fields[i] = fieldDesc
	}
	return fields, nil
}

// 缓存已解析的字段以提高性能
// Cache parsed fields to improve performance
var (
	// parsedStructFieldCache 存储每个结构体类型的已解析字段
	// 使用 sync.Map 保证并发安全
	//
	// parsedStructFieldCache stores parsed fields for each struct type
	// Uses sync.Map to ensure thread safety
	parsedStructFieldCache = sync.Map{}
)

// fieldCacheLookup 查找类型的缓存字段
// 使用 sync.Map 进行并发安全的缓存查找
// 如果找到缓存的字段则返回，否则返回 nil
//
// fieldCacheLookup looks up cached fields for a type
// Uses sync.Map for thread-safe cache lookup
// Returns cached fields if found, nil otherwise
func fieldCacheLookup(structType reflect.Type) Fields {
	if cached, ok := parsedStructFieldCache.Load(structType); ok {
		return cached.(Fields)
	}
	return nil
}

// parseFields 解析结构体的所有字段
// 首先尝试从缓存中获取已解析的字段
// 如果缓存未命中，则进行解析并将结果存入缓存
// 返回字段切片和可能的错误
//
// parseFields parses all fields of a struct
// First tries to get parsed fields from cache
// If cache miss, performs parsing and stores result in cache
// Returns slice of fields and possible error
func parseFields(structValue reflect.Value) (Fields, error) {
	// 从缓存中查找
	// Look up in cache
	structType := structValue.Type()
	if cached := fieldCacheLookup(structType); cached != nil {
		// 返回缓存字段的克隆，避免并发修改
		// Return a clone of cached fields to avoid concurrent modification
		return cached, nil
	}

	// 解析字段
	// Parse fields
	fields, err := parseFieldsLocked(structValue)
	if err != nil {
		return nil, err
	}

	// 将解析结果存入缓存
	// Store parsing result in cache
	parsedStructFieldCache.Store(structType, fields)

	return fields, nil
}
