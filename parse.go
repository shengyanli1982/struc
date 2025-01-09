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

// 用于匹配数组长度的正则表达式
// Regular expression for matching array length
var typeArrayLenRegex = regexp.MustCompile(`^\[(\d*)\]`)

// parseField 解析单个结构体字段，返回字段描述符和标签信息
// parseField parses a single struct field, returns field descriptor and tag info
func parseField(f reflect.StructField) (fd *Field, tag *strucTag, err error) {
	// 解析字段标签
	// Parse field tag
	tag = parseStrucTag(f.Tag)
	var ok bool

	// 初始化字段描述符
	// Initialize field descriptor
	fd = &Field{
		Name:  f.Name,
		Len:   1,
		Order: tag.Order,
		Slice: false,
		kind:  f.Type.Kind(),
	}

	// 处理特殊类型：数组、切片和指针
	// Handle special types: arrays, slices and pointers
	switch fd.kind {
	case reflect.Array:
		fd.Slice = true
		fd.Array = true
		fd.Len = f.Type.Len()
		fd.kind = f.Type.Elem().Kind()
	case reflect.Slice:
		fd.Slice = true
		fd.Len = -1
		fd.kind = f.Type.Elem().Kind()
	case reflect.Ptr:
		fd.Ptr = true
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
	pureType := typeArrayLenRegex.ReplaceAllLiteralString(tag.Type, "")
	if fd.Type, ok = typeStrToType[pureType]; ok {
		fd.Len = 1
		// 解析数组长度
		// Parse array length
		match := typeArrayLenRegex.FindAllStringSubmatch(tag.Type, -1)
		if len(match) > 0 && len(match[0]) > 1 {
			fd.Slice = true
			first := match[0][1]
			if first == "" {
				fd.Len = -1 // 动态长度切片 / Dynamic length slice
			} else {
				fd.Len, err = strconv.Atoi(first)
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
			err = fmt.Errorf("struc: Could not resolve field '%v' type '%v'.", f.Name, f.Type)
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
		f, tag, err := parseField(field)
		if tag.Skip {
			continue // 跳过标记为 skip 的字段 / Skip fields marked with skip
		}
		if err != nil {
			return nil, err
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
				return nil, fmt.Errorf("struc: `sizefrom=%s` field does not exist", tag.Sizefrom)
			}
			f.Sizefrom = source.Index
		}

		// 验证切片长度
		// Validate slice length
		if f.Len == -1 && f.Sizefrom == nil {
			return nil, fmt.Errorf("struc: field `%s` is a slice with no length or sizeof field", field.Name)
		}

		// 递归处理嵌套结构体
		// Recursively handle nested structs
		if f.Type == Struct {
			typ := field.Type
			if f.Ptr {
				typ = typ.Elem()
			}
			if f.Slice {
				typ = typ.Elem()
			}
			f.Fields, err = parseFieldsLocked(reflect.New(typ))
			if err != nil {
				return nil, err
			}
		}

		fields[i] = f
	}
	return fields, nil
}

// Cache for parsed fields to improve performance
// 缓存已解析的字段以提高性能
var (
	// structFieldCache 存储每个结构体类型的已解析字段
	// structFieldCache stores parsed fields for each struct type
	structFieldCache = sync.Map{}

	// parseLock 防止同一类型的并发解析
	// parseLock prevents concurrent parsing of the same type
	parseLock sync.Mutex
)

// fieldCacheLookup 查找类型的缓存字段
// fieldCacheLookup looks up cached fields for a type
func fieldCacheLookup(t reflect.Type) Fields {
	if cached, ok := structFieldCache.Load(t); ok {
		return cached.(Fields)
	}
	return nil
}

// parseFields 解析结构体字段并缓存结果
// 这是解析结构体字段的主入口函数，它实现了缓存机制以提高性能
//
// parseFields parses struct fields and caches the result
// This is the main entry point for parsing struct fields, implementing a caching mechanism for better performance
func parseFields(v reflect.Value) (Fields, error) {
	// 解引用指针，直到获取到非指针类型
	// 这个步骤确保我们总是处理实际的值类型
	//
	// Dereference pointers until we get a non-pointer type
	// This step ensures we always work with actual value types
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()

	// 快速路径：首先检查缓存
	// 如果找到缓存的字段信息，直接返回
	//
	// Fast path: check cache first
	// If cached fields info is found, return it directly
	if cached := fieldCacheLookup(t); cached != nil {
		return cached, nil
	}

	// 慢速路径：加锁解析字段
	// 这里使用互斥锁确保并发安全
	//
	// Slow path: parse fields with lock
	// Using mutex to ensure thread safety
	parseLock.Lock()
	defer parseLock.Unlock()

	// 获取锁后再次检查缓存
	// 这是一个双重检查锁定模式，避免重复解析
	//
	// Double-check cache after acquiring lock
	// This is a double-checked locking pattern to avoid redundant parsing
	if cached := fieldCacheLookup(t); cached != nil {
		return cached, nil
	}

	// 解析字段并更新缓存
	// 这是实际的解析工作，完成后会存储到缓存中
	//
	// Parse fields and update cache
	// This is where the actual parsing happens, results will be stored in cache
	fields, err := parseFieldsLocked(v)
	if err != nil {
		return nil, err
	}

	// 将解析结果存储到缓存中
	// Store parsing results in cache
	structFieldCache.Store(t, fields)
	return fields, nil
}
