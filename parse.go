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

// strucTag 定义了结构体字段标签的解析结果
// 包含了字段的类型、字节序、大小引用等信息
type strucTag struct {
	Type     string           // 字段类型（如 int32, uint8 等）
	Order    binary.ByteOrder // 字节序（大端或小端）
	Sizeof   string           // 大小引用字段名
	Skip     bool             // 是否跳过该字段
	Sizefrom string           // 长度来源字段名
}

// parseStrucTag 解析结构体字段的标签
// 支持 struc 和 struct 两种标签名（向后兼容）
func parseStrucTag(fieldTag reflect.StructTag) *strucTag {
	// 初始化标签结构体，默认使用大端字节序
	parsedTag := &strucTag{
		Order: binary.BigEndian,
	}

	// 获取 struc 标签，如果不存在则尝试获取 struct 标签
	tagString := fieldTag.Get("struc")
	if tagString == "" {
		tagString = fieldTag.Get("struct")
	}

	// 处理 "-" 标签，表示完全忽略该字段
	if tagString == "-" {
		parsedTag.Skip = true
		return parsedTag
	}

	for _, option := range strings.Split(tagString, ",") {
		if strings.HasPrefix(option, "sizeof=") {
			parts := strings.SplitN(option, "=", 2)
			parsedTag.Sizeof = parts[1]
		} else if strings.HasPrefix(option, "sizefrom=") {
			parts := strings.SplitN(option, "=", 2)
			parsedTag.Sizefrom = parts[1]
		} else if option == "big" {
			parsedTag.Order = binary.BigEndian
		} else if option == "little" {
			parsedTag.Order = binary.LittleEndian
		} else if option == "skip" {
			parsedTag.Skip = true
		} else if option != "" {
			parsedTag.Type = option
		}
	}
	return parsedTag
}

// arrayLengthParseRegex 用于匹配数组长度的正则表达式: [数字]
var arrayLengthParseRegex = regexp.MustCompile(`^\[(\d*)\]`)

// parseStructField 解析单个结构体字段
func parseStructField(structField reflect.StructField) (fieldDesc *Field, fieldTag *strucTag, err error) {
	fieldTag = parseStrucTag(structField.Tag)
	var ok bool

	fieldDesc = acquireField()

	fieldDesc.Name = structField.Name
	fieldDesc.Length = 1
	fieldDesc.ByteOrder = fieldTag.Order
	fieldDesc.IsSlice = false
	fieldDesc.kind = structField.Type.Kind()

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
	tempValue := reflect.New(structField.Type)
	if _, ok := tempValue.Interface().(CustomBinaryer); ok {
		fieldDesc.Type = CustomType
		return
	}

	var defTypeOk bool
	fieldDesc.defType, defTypeOk = typeKindToType[fieldDesc.kind]

	// 从结构体标签中查找类型
	pureType := arrayLengthParseRegex.ReplaceAllLiteralString(fieldTag.Type, "")
	if fieldDesc.Type, ok = typeStrToType[pureType]; ok {
		fieldDesc.Length = 1
		// 解析数组长度
		matches := arrayLengthParseRegex.FindAllStringSubmatch(fieldTag.Type, -1)
		if len(matches) > 0 && len(matches[0]) > 1 {
			fieldDesc.IsSlice = true
			lengthStr := matches[0][1]
			if lengthStr == "" {
				fieldDesc.Length = -1 // 动态长度切片
			} else {
				fieldDesc.Length, err = strconv.Atoi(lengthStr)
			}
		}
		return
	}

	// 处理特殊类型 Size_t 和 Off_t
	switch structField.Type {
	case reflect.TypeOf(Size_t(0)):
		fieldDesc.Type = SizeType
	case reflect.TypeOf(Off_t(0)):
		fieldDesc.Type = OffType
	default:
		if defTypeOk {
			fieldDesc.Type = fieldDesc.defType
		} else {
			releaseField(fieldDesc)
			err = fmt.Errorf("struc: Could not resolve field '%v' type '%v'.", structField.Name, structField.Type)
			fieldDesc = nil
		}
	}
	return
}

// handleSizeofTag 处理字段的 sizeof 标签
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
func validateSliceLength(fieldDesc *Field, field reflect.StructField) error {
	if fieldDesc.Length == -1 && fieldDesc.Sizefrom == nil {
		return fmt.Errorf("struc: field `%s` is a slice with no length or sizeof field", field.Name)
	}
	return nil
}

// parseFieldsLocked 在加锁状态下解析结构体的所有字段
func parseFieldsLocked(structValue reflect.Value) (Fields, error) {
	for structValue.Kind() == reflect.Ptr {
		structValue = structValue.Elem()
	}
	structType := structValue.Type()

	if structValue.NumField() < 1 {
		return nil, errors.New("struc: Struct has no fields.")
	}

	sizeofMap := acquireSizeofMap()
	defer releaseSizeofMap(sizeofMap)

	fields := make(Fields, structValue.NumField())

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)

		fieldDesc, fieldTag, err := parseStructField(field)
		if err != nil {
			releaseFields(fields)
			return nil, err
		}

		if fieldTag.Skip || !structValue.Field(i).CanSet() {
			continue
		}

		fieldDesc.Index = i

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

// parsedStructFieldCache 存储每个结构体类型的已解析字段 (并发安全)
var (
	parsedStructFieldCache = sync.Map{}
)

// fieldCacheLookup 查找类型的缓存字段
func fieldCacheLookup(structType reflect.Type) Fields {
	if cached, ok := parsedStructFieldCache.Load(structType); ok {
		return cached.(Fields)
	}
	return nil
}

// parseFields 解析结构体的所有字段
// 首先尝试从缓存中获取，如果未命中则进行解析并缓存结果
func parseFields(structValue reflect.Value) (Fields, error) {
	structType := structValue.Type()
	if cached := fieldCacheLookup(structType); cached != nil {
		return cached, nil
	}

	fields, err := parseFieldsLocked(structValue)
	if err != nil {
		return nil, err
	}

	parsedStructFieldCache.Store(structType, fields)

	return fields, nil
}
