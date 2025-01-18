[English](./README.md) | 中文

# struc v2

[![Go Report Card](https://goreportcard.com/badge/github.com/shengyanli1982/struc/v2)](https://goreportcard.com/report/github.com/shengyanli1982/struc/v2)
[![Build Status](https://github.com/shengyanli1982/struc/actions/workflows/test.yaml/badge.svg)](https://github.com/shengyanli1982/struc/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/shengyanli1982/struc/v2.svg)](https://pkg.go.dev/github.com/shengyanli1982/struc/v2)

一个高性能的 Go 二进制数据序列化库，采用 C 风格的结构体定义。

## 为什么选择 struc v2？

-   🚀 **卓越性能**：优化的二进制序列化，支持反射缓存
-   💡 **简洁 API**：基于结构体标签的直观配置，无需样板代码
-   🛡️ **类型安全**：强类型检查和全面的错误处理
-   🔄 **灵活编码**：支持大端和小端字节序
-   📦 **丰富类型支持**：支持原始类型、数组、切片和自定义填充
-   🎯 **零依赖**：纯 Go 实现，无外部依赖

## 安装

```bash
go get github.com/shengyanli1982/struc/v2
```

## 快速开始

```go
package main

import (
    "bytes"
    "github.com/shengyanli1982/struc/v2"
)

type Message struct {
    Size    int    `struc:"int32,sizeof=Payload"`  // 自动追踪负载大小
    Payload []byte                                 // 动态二进制数据
    Flags   uint16 `struc:"little"`               // 小端编码
}

func main() {
    var buf bytes.Buffer

    // 打包数据
    msg := &Message{
        Payload: []byte("Hello, World!"),
        Flags:   1234,
    }
    if err := struc.Pack(&buf, msg); err != nil {
        panic(err)
    }

    // 解包数据
    result := &Message{}
    if err := struc.Unpack(&buf, result); err != nil {
        panic(err)
    }
}
```

## 特性

### 1. 丰富的类型支持

-   原始类型：`bool`、`int8`-`int64`、`uint8`-`uint64`、`float32`、`float64`
-   复合类型：字符串、字节切片、数组
-   特殊类型：用于对齐的填充字节

### 2. 智能字段标签

```go
type Example struct {
    Length  int    `struc:"int32,sizeof=Data"`   // 大小追踪
    Data    []byte                               // 动态数据
    Version uint16 `struc:"little"`              // 字节序控制
    Padding [4]byte `struc:"[4]pad"`            // 显式填充
}
```

### 3. 自动大小追踪

-   自动管理可变大小字段的长度
-   消除手动大小计算和追踪
-   减少二进制协议实现中的潜在错误

### 4. 性能优化

-   反射缓存以提高重复操作性能
-   高效的内存分配
-   优化的编码/解码路径

## 高级用法

### 自定义字节序

```go
type Custom struct {
    BigEndian    int32  `struc:"big"`    // 显式大端
    LittleEndian int32  `struc:"little"` // 显式小端
}
```

### 固定大小数组

```go
type FixedArray struct {
    Data [16]byte `struc:"[16]byte"` // 固定大小字节数组
    Ints [4]int32 `struc:"[4]int32"` // 固定大小整数数组
}
```

## 最佳实践

1. **使用适当的类型**

    - 将 Go 类型与其二进制协议对应物匹配
    - 当大小已知时使用固定大小数组
    - 对动态数据使用带 `sizeof` 的切片

2. **错误处理**

    - 始终检查 Pack/Unpack 返回的错误
    - 在处理之前验证数据大小

3. **性能优化**
    - 尽可能重用结构体
    - 考虑对频繁使用的结构使用对象池

## 性能基准测试

```
$ go test -benchmem -run=^$ -bench .
BenchmarkArrayEncode-12          3203236               373.2 ns/op           137 B/op          4 allocs/op
BenchmarkStdlibEncode-12         6035904               206.0 ns/op           136 B/op          3 allocs/op
BenchmarkManualEncode-12        49696231                25.64 ns/op           64 B/op          1 allocs/op
```

我们的基准测试为不同的编码方法提供了透明的性能指标。虽然基于反射的解决方案通常会用一些性能来换取灵活性和功能，但 `struc` 在提供丰富功能的同时保持了具有竞争力的性能表现。

## 许可证

MIT 许可证 - 详见 LICENSE 文件
