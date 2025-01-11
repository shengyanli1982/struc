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
// Tag format example: struc:"int32,big,sizeof=Data,skip,sizefrom=Len"

// strucTag 定义了结构体字段标签的解析结果
// strucTag defines the parsed result of struct field tags
type strucTag struct {
	Type     string           // 字段类型 / Field type
	Order    binary.ByteOrder // 字节序 / Byte order
	Sizeof   string           // 大小引用字段 / Size reference field
	Skip     bool             // 是否跳过 / Whether to skip
	Sizefrom string           // 长度来源字段 / Length source field
}

// parseStrucTag 解析结构体字段的标签
// parseStrucTag parses the tags of struct fields
func parseStrucTag(tag reflect.StructTag) *strucTag {
	// 初始化标签结构体，默认使用大端字节序
	// Initialize tag struct with big-endian as default
	t := &strucTag{
		Order: binary.BigEndian,
	}

	// 获取 struc 标签，如果不存在则尝试获取 struct 标签（容错处理）
	// Get struc tag, fallback to struct tag if not found (error tolerance)
	tagStr := tag.Get("struc")
	if tagStr == "" {
		tagStr = tag.Get("struct")
	}

	// 解析标签字符串中的每个选项
	// Parse each option in the tag string
	for _, s := range strings.Split(tagStr, ",") {
		if strings.HasPrefix(s, "sizeof=") {
			// 解析 sizeof 选项，指定字段大小来源
			// Parse sizeof option, specifying size source field
			tmp := strings.SplitN(s, "=", 2)
			t.Sizeof = tmp[1]
		} else if strings.HasPrefix(s, "sizefrom=") {
			// 解析 sizefrom 选项，指定长度来源字段
			// Parse sizefrom option, specifying length source field
			tmp := strings.SplitN(s, "=", 2)
			t.Sizefrom = tmp[1]
		} else if s == "big" {
			// 设置大端字节序
			// Set big-endian byte order
			t.Order = binary.BigEndian
		} else if s == "little" {
			// 设置小端字节序
			// Set little-endian byte order
			t.Order = binary.LittleEndian
		} else if s == "skip" {
			// 设置跳过标志
			// Set skip flag
			t.Skip = true
		} else {
			// 设置字段类型
			// Set field type
			t.Type = s
		}
	}
	return t
}

// arrayLengthParseRegex 用于匹配数组长度的正则表达式
// arrayLengthParseRegex is a regular expression for matching array length
var arrayLengthParseRegex = regexp.MustCompile(`^\[(\d*)\]`)

// parseStructField 解析单个结构体字段，返回字段描述符和标签信息
// parseStructField parses a single struct field, returns field descriptor and tag info
func parseStructField(f reflect.StructField) (fd *Field, tag *strucTag, err error) {
	// 解析字段标签
	// Parse field tag
	tag = parseStrucTag(f.Tag)
	var ok bool

	// 从对象池获取 Field 对象
	// Get Field object from pool
	fd = acquireField()

	// 初始化字段描述符
	// Initialize field descriptor
	fd.Name = f.Name
	fd.Length = 1
	fd.ByteOrder = tag.Order
	fd.IsSlice = false
	fd.kind = f.Type.Kind()

	// 处理特殊类型：数组、切片和指针
	// Handle special types: arrays, slices and pointers
	switch fd.kind {
	case reflect.Array:
		fd.IsSlice = true
		fd.IsArray = true
		fd.Length = f.Type.Len()
		fd.kind = f.Type.Elem().Kind()
	case reflect.Slice:
		fd.IsSlice = true
		fd.Length = -1
		fd.kind = f.Type.Elem().Kind()
	case reflect.Ptr:
		fd.IsPointer = true
		fd.kind = f.Type.Elem().Kind()
	}

	// 检查是否为自定义类型
	// Check if it's a custom type
	tmp := reflect.New(f.Type)
	if _, ok := tmp.Interface().(Custom); ok {
		fd.Type = CustomType
		return
	}

	// 获取默认类型
	// Get default type
	var defTypeOk bool
	fd.defType, defTypeOk = typeKindToType[fd.kind]

	// 从结构体标签中查找类型
	// Find type in struct tag
	pureType := arrayLengthParseRegex.ReplaceAllLiteralString(tag.Type, "")
	if fd.Type, ok = typeStrToType[pureType]; ok {
		fd.Length = 1
		// 解析数组长度
		// Parse array length
		match := arrayLengthParseRegex.FindAllStringSubmatch(tag.Type, -1)
		if len(match) > 0 && len(match[0]) > 1 {
			fd.IsSlice = true
			first := match[0][1]
			if first == "" {
				fd.Length = -1 // 动态长度切片 / Dynamic length slice
			} else {
				fd.Length, err = strconv.Atoi(first)
			}
		}
		return
	}

	// 处理特殊类型 Size_t 和 Off_t
	// Handle special types Size_t and Off_t
	switch f.Type {
	case reflect.TypeOf(Size_t(0)):
		fd.Type = SizeType
	case reflect.TypeOf(Off_t(0)):
		fd.Type = OffType
	default:
		if defTypeOk {
			fd.Type = fd.defType
		} else {
			// 如果发生错误，需要释放 Field 对象
			// If error occurs, need to release Field object
			releaseField(fd)
			err = fmt.Errorf("struc: Could not resolve field '%v' type '%v'.", f.Name, f.Type)
			fd = nil
		}
	}
	return
}

// parseFieldsLocked 在加锁状态下解析结构体的所有字段
// 此函数处理嵌套结构体、数组和切片等复杂类型
//
// parseFieldsLocked parses all fields of a struct while locked
// This function handles complex types like nested structs, arrays and slices
func parseFieldsLocked(v reflect.Value) (Fields, error) {
	// 解引用指针，直到获取到非指针类型
	// Dereference pointers until we get a non-pointer type
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()

	// 检查结构体是否有字段
	// Check if the struct has any fields
	if v.NumField() < 1 {
		return nil, errors.New("struc: Struct has no fields.")
	}

	// 创建大小引用映射和字段切片
	// Create size reference map and fields slice
	sizeofMap := make(map[string][]int)
	fields := make(Fields, v.NumField())

	// 遍历所有字段
	// Iterate through all fields
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		// 解析字段和标签
		// Parse field and tag
		f, tag, err := parseStructField(field)
		if err != nil {
			// 发生错误时释放已创建的字段
			// Release created fields when error occurs
			releaseFields(fields)
			return nil, err
		}
		if tag.Skip {
			continue // 跳过标记为 skip 的字段 / Skip fields marked with skip
		}
		if !v.Field(i).CanSet() {
			continue // 跳过不可设置的字段 / Skip fields that cannot be set
		}

		f.Index = i

		// 处理 sizeof 标签
		// Handle sizeof tag
		if tag.Sizeof != "" {
			target, ok := t.FieldByName(tag.Sizeof)
			if !ok {
				// 发生错误时释放已创建的字段
				// Release created fields when error occurs
				releaseFields(fields)
				return nil, fmt.Errorf("struc: `sizeof=%s` field does not exist", tag.Sizeof)
			}
			f.Sizeof = target.Index
			sizeofMap[tag.Sizeof] = field.Index
		}

		// 处理 sizefrom 标签
		// Handle sizefrom tag
		if sizefrom, ok := sizeofMap[field.Name]; ok {
			f.Sizefrom = sizefrom
		}
		if tag.Sizefrom != "" {
			source, ok := t.FieldByName(tag.Sizefrom)
			if !ok {
				// 发生错误时释放已创建的字段
				// Release created fields when error occurs
				releaseFields(fields)
				return nil, fmt.Errorf("struc: `sizefrom=%s` field does not exist", tag.Sizefrom)
			}
			f.Sizefrom = source.Index
		}

		// 验证切片长度
		// Validate slice length
		if f.Length == -1 && f.Sizefrom == nil {
			// 发生错误时释放已创建的字段
			// Release created fields when error occurs
			releaseFields(fields)
			return nil, fmt.Errorf("struc: field `%s` is a slice with no length or sizeof field", field.Name)
		}

		// 递归处理嵌套结构体
		// Recursively handle nested structs
		if f.Type == Struct {
			typ := field.Type
			if f.IsPointer {
				typ = typ.Elem()
			}
			if f.IsSlice {
				typ = typ.Elem()
			}
			tmp := reflect.New(typ)
			nestFields, err := parseFields(tmp.Elem())
			if err != nil {
				// 发生错误时释放已创建的字段
				// Release created fields when error occurs
				releaseFields(fields)
				return nil, err
			}
			f.NestFields = nestFields
		}

		fields[i] = f
	}
	return fields, nil
}

// Cache for parsed fields to improve performance
// 缓存已解析的字段以提高性能
var (
	// parsedStructFieldCache 存储每个结构体类型的已解析字段
	// parsedStructFieldCache stores parsed fields for each struct type
	parsedStructFieldCache = sync.Map{}

	// structParsingMutex 防止同一类型的并发解析
	// structParsingMutex prevents concurrent parsing of the same type
	structParsingMutex sync.Mutex
)

// fieldCacheLookup 查找类型的缓存字段
// fieldCacheLookup looks up cached fields for a type
func fieldCacheLookup(t reflect.Type) Fields {
	if cached, ok := parsedStructFieldCache.Load(t); ok {
		return cached.(Fields)
	}
	return nil
}

// parseFields 解析结构体的所有字段
// parseFields parses all fields of a struct
func parseFields(v reflect.Value) (Fields, error) {
	// 从缓存中查找
	// Look up in cache
	t := v.Type()
	if cached := fieldCacheLookup(t); cached != nil {
		// 返回缓存字段的克隆，避免并发修改
		// Return a clone of cached fields to avoid concurrent modification
		return cached.Clone(), nil
	}

	// 解析字段
	// Parse fields
	fields, err := parseFieldsLocked(v)
	if err != nil {
		return nil, err
	}

	// 将解析结果存入缓存
	// Store parsing result in cache
	parsedStructFieldCache.Store(t, fields.Clone())

	return fields, nil
}

// String returns a string representation of the field.
// String 返回字段的字符串表示。
