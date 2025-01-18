English | [中文](./README_CN.md)

# struc v2

[![Go Report Card](https://goreportcard.com/badge/github.com/shengyanli1982/struc/v2)](https://goreportcard.com/report/github.com/shengyanli1982/struc/v2)
[![Build Status](https://github.com/shengyanli1982/struc/actions/workflows/test.yaml/badge.svg)](https://github.com/shengyanli1982/struc/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/shengyanli1982/struc/v2.svg)](https://pkg.go.dev/github.com/shengyanli1982/struc/v2)

A high-performance Go library for binary data serialization with C-style struct definitions.

## Why struc v2?

-   🚀 **High Performance**: Optimized binary serialization with reflection caching
-   💡 **Simple API**: Intuitive struct tag-based configuration without boilerplate code
-   🛡️ **Type Safety**: Strong type checking with comprehensive error handling
-   🔄 **Flexible Encoding**: Support for both big and little endian byte orders
-   📦 **Rich Type Support**: Handles primitive types, arrays, slices, and custom padding
-   🎯 **Zero Dependencies**: Pure Go implementation with no external dependencies

## Installation

```bash
go get github.com/shengyanli1982/struc/v2
```

## Quick Start

```go
package main

import (
    "bytes"
    "github.com/shengyanli1982/struc/v2"
)

type Message struct {
    Size    int    `struc:"int32,sizeof=Payload"`  // Automatically tracks payload size
    Payload []byte                                 // Dynamic binary data
    Flags   uint16 `struc:"little"`               // Little-endian encoding
}

func main() {
    var buf bytes.Buffer

    // Pack data
    msg := &Message{
        Payload: []byte("Hello, World!"),
        Flags:   1234,
    }
    if err := struc.Pack(&buf, msg); err != nil {
        panic(err)
    }

    // Unpack data
    result := &Message{}
    if err := struc.Unpack(&buf, result); err != nil {
        panic(err)
    }
}
```

## Features

### 1. Rich Type Support

-   Primitive types: `bool`, `int8`-`int64`, `uint8`-`uint64`, `float32`, `float64`
-   Composite types: strings, byte slices, arrays
-   Special types: padding bytes for alignment

### 2. Smart Field Tags

```go
type Example struct {
    Length  int    `struc:"int32,sizeof=Data"`   // Size tracking
    Data    []byte                               // Dynamic data
    Version uint16 `struc:"little"`              // Endianness control
    Padding [4]byte `struc:"[4]pad"`            // Explicit padding
}
```

### 3. Automatic Size Tracking

-   Automatically manages lengths of variable-sized fields
-   Eliminates manual size calculation and tracking
-   Reduces potential errors in binary protocol implementations

### 4. Performance Optimizations

-   Reflection caching for repeated operations
-   Efficient memory allocation
-   Optimized encoding/decoding paths

## Advanced Usage

### Custom Endianness

```go
type Custom struct {
    BigEndian    int32  `struc:"big"`    // Explicit big-endian
    LittleEndian int32  `struc:"little"` // Explicit little-endian
}
```

### Fixed-Size Arrays

```go
type FixedArray struct {
    Data [16]byte `struc:"[16]byte"` // Fixed-size byte array
    Ints [4]int32 `struc:"[4]int32"` // Fixed-size integer array
}
```

## Best Practices

1. **Use Appropriate Types**

    - Match Go types with their binary protocol counterparts
    - Use fixed-size arrays when the size is known
    - Use slices with `sizeof` for dynamic data

2. **Error Handling**

    - Always check returned errors from Pack/Unpack
    - Validate data sizes before processing

3. **Performance Optimization**
    - Reuse structs when possible
    - Consider using pools for frequently used structures

## Performance Benchmarks

```
goos: windows
goarch: amd64
pkg: github.com/shengyanli1982/struc/v2
cpu: 12th Gen Intel(R) Core(TM) i5-12400F
BenchmarkArrayEncode-12          3203236               373.2 ns/op           137 B/op          4 allocs/op
BenchmarkSliceEncode-12          2985786               400.9 ns/op           137 B/op          4 allocs/op
BenchmarkArrayDecode-12          3407203               349.8 ns/op            73 B/op          2 allocs/op
BenchmarkSliceDecode-12          2768002               433.5 ns/op           112 B/op          4 allocs/op
BenchmarkEncode-12               2656374               462.5 ns/op           168 B/op          4 allocs/op
BenchmarkStdlibEncode-12         6035904               206.0 ns/op           136 B/op          3 allocs/op
BenchmarkManualEncode-12        49696231                25.64 ns/op           64 B/op          1 allocs/op
BenchmarkDecode-12               2812420               421.0 ns/op           103 B/op          2 allocs/op
BenchmarkStdlibDecode-12         5953122               195.3 ns/op            80 B/op          3 allocs/op
BenchmarkManualDecode-12        100000000               12.21 ns/op            8 B/op          1 allocs/op
BenchmarkFullEncode-12           1000000              1800 ns/op             456 B/op          4 allocs/op
BenchmarkFullDecode-12            598369              1974 ns/op             327 B/op          5 allocs/op
BenchmarkFieldPool-12           19483657                62.86 ns/op          168 B/op          4 allocs/op
```

## License

MIT License - see LICENSE file for details
