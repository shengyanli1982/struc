[English](./README.md) | 中文

# struc v2

[![Go Report Card](https://goreportcard.com/badge/github.com/shengyanli1982/struc/v2)](https://goreportcard.com/report/github.com/shengyanli1982/struc/v2)
[![Build Status](https://github.com/shengyanli1982/struc/actions/workflows/test.yaml/badge.svg)](https://github.com/shengyanli1982/struc/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/shengyanli1982/struc/v2.svg)](https://pkg.go.dev/github.com/shengyanli1982/struc/v2)

Struc v2 是一个 Go 语言库，用于使用 C 风格的结构体定义来打包和解包二进制数据。它为 `encoding/binary` 提供了一个更便捷的替代方案，无需编写大量的样板代码。

本项目兼容 "github.com/lunixbochs/struc" 的接口调用。

[查看 struc 与 encoding/binary 的对比](https://bochs.info/p/cxvm9)

## 特性

-   简单的结构体标签配置
-   支持多种数值类型和数组
-   字段间自动大小追踪
-   可配置的字节序
-   通过反射缓存实现高性能
-   全面的测试覆盖

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

type Example struct {
    Length int    `struc:"int32,sizeof=Data"`  // 自动追踪 Data 长度
    Data   string                              // 将被打包为字节
    Values []int  `struc:"[]int16,little"`     // 小端序 int16 切片
    Fixed  [4]int `struc:"[4]int32"`          // 固定大小的 int32 数组
}

func main() {
    var buf bytes.Buffer

    // 打包结构体
    data := &Example{
        Data:   "hello",
        Values: []int{1, 2, 3},
        Fixed:  [4]int{4, 5, 6, 7},
    }
    if err := struc.Pack(&buf, data); err != nil {
        panic(err)
    }

    // 解包结构体
    result := &Example{}
    if err := struc.Unpack(&buf, result); err != nil {
        panic(err)
    }
}
```

## 结构体标签格式

结构体标签格式为：`` `struc:"type,endian,sizeof=Field"` ``

组成部分：

-   `type`：二进制类型（如 `int32`、`[]int16`）
-   `endian`：字节序（`big` 或 `little`，默认为 `big`）
-   `sizeof=Field`：将该数值字段链接到另一个字段的长度

示例：

```go
type Message struct {
    Size    int      `struc:"int32,sizeof=Payload"`
    Payload []byte
    Flags   uint16   `struc:"little"`  // 小端序 uint16
    Reserved [4]byte `struc:"[4]pad"`  // 4 字节填充
}
```

## 支持的类型

基本类型：

-   `bool` - 1 字节
-   `byte`/`uint8`/`int8` - 1 字节
-   `uint16`/`int16` - 2 字节
-   `uint32`/`int32` - 4 字节
-   `uint64`/`int64` - 8 字节
-   `float32` - 4 字节
-   `float64` - 8 字节
-   `string` - 长度前缀的字节序列
-   `[]byte` - 原始字节

数组/切片类型：

-   固定大小数组：`[N]type`
-   动态切片：`[]type`（需要 `sizeof` 字段）

特殊类型：

-   `pad` - 用于对齐/填充的空字节

## 性能

与标准库 `encoding/binary` 和手动编码的基准测试对比：

```bash
goos: windows
goarch: amd64
pkg: github.com/shengyanli1982/struc/v2
cpu: 12th Gen Intel(R) Core(TM) i5-12400F
BenchmarkArrayEncode-12          3470301               347.2 ns/op           113 B/op          3 allocs/op
BenchmarkSliceEncode-12          3088372               381.7 ns/op           114 B/op          4 allocs/op
BenchmarkArrayDecode-12          2215333               538.4 ns/op           184 B/op         18 allocs/op
BenchmarkSliceDecode-12          1969315               613.5 ns/op           224 B/op         20 allocs/op
BenchmarkEncode-12               2700584               439.4 ns/op           152 B/op          4 allocs/op
BenchmarkStdlibEncode-12         6337509               190.2 ns/op           136 B/op          3 allocs/op
BenchmarkManualEncode-12        48160663                24.73 ns/op           64 B/op          1 allocs/op
BenchmarkDecode-12               2846366               421.4 ns/op            96 B/op          5 allocs/op
BenchmarkStdlibDecode-12         6370437               186.8 ns/op            80 B/op          3 allocs/op
BenchmarkManualDecode-12        100000000               12.06 ns/op            8 B/op          1 allocs/op
BenchmarkFullEncode-12            668888              1758 ns/op             472 B/op          8 allocs/op
BenchmarkFullDecode-12            633552              1862 ns/op             312 B/op         26 allocs/
```

## 注意事项

-   私有字段在打包/解包时会被忽略
-   裸切片类型必须有对应的 `sizeof` 字段
-   所有数值类型都支持大端序和小端序编码
-   库会缓存反射数据以提高性能

## 许可证

MIT 许可证 - 详见 LICENSE 文件
