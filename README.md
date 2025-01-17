English | [中文](./README_CN.md)

# struc v2

[![Go Report Card](https://goreportcard.com/badge/github.com/shengyanli1982/struc/v2)](https://goreportcard.com/report/github.com/shengyanli1982/struc/v2)
[![Build Status](https://github.com/shengyanli1982/struc/actions/workflows/test.yaml/badge.svg)](https://github.com/shengyanli1982/struc/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/shengyanli1982/struc/v2.svg)](https://pkg.go.dev/github.com/shengyanli1982/struc/v2)

Struc v2 is a Go library for packing and unpacking binary data using C-style structure definitions. It provides a more convenient alternative to `encoding/binary`, eliminating the need for extensive boilerplate code.

The project is compatible with the interface calls in "github.com/lunixbochs/struc".

[Compare struc with encoding/binary](https://bochs.info/p/cxvm9)

## Features

-   Simple struct tag-based configuration
-   Support for various numeric types and arrays
-   Automatic size tracking between fields
-   Configurable endianness
-   High performance with reflection caching
-   Comprehensive test coverage

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

type Example struct {
    Length int    `struc:"int32,sizeof=Data"`  // Automatically tracks Data length
    Data   string                              // Will be packed as bytes
    Values []int  `struc:"[]int16,little"`     // Slice of little-endian int16
    Fixed  [4]int `struc:"[4]int32"`          // Fixed-size array of int32
}

func main() {
    var buf bytes.Buffer

    // Pack structure
    data := &Example{
        Data:   "hello",
        Values: []int{1, 2, 3},
        Fixed:  [4]int{4, 5, 6, 7},
    }
    if err := struc.Pack(&buf, data); err != nil {
        panic(err)
    }

    // Unpack structure
    result := &Example{}
    if err := struc.Unpack(&buf, result); err != nil {
        panic(err)
    }
}
```

## Struct Tag Format

The struct tag format is: `` `struc:"type,endian,sizeof=Field"` ``

Components:

-   `type`: The binary type (e.g., `int32`, `[]int16`)
-   `endian`: Byte order (`big` or `little`, defaults to `big`)
-   `sizeof=Field`: Links this numeric field to another field's length

Example:

```go
type Message struct {
    Size    int      `struc:"int32,sizeof=Payload"`
    Payload []byte
    Flags   uint16   `struc:"little"`  // Little-endian uint16
    Reserved [4]byte `struc:"[4]pad"`  // 4 bytes of padding
}
```

## Supported Types

Basic Types:

-   `bool` - 1 byte
-   `byte`/`uint8`/`int8` - 1 byte
-   `uint16`/`int16` - 2 bytes
-   `uint32`/`int32` - 4 bytes
-   `uint64`/`int64` - 8 bytes
-   `float32` - 4 bytes
-   `float64` - 8 bytes
-   `string` - Length-prefixed bytes
-   `[]byte` - Raw bytes

Array/Slice Types:

-   Fixed-size arrays: `[N]type`
-   Dynamic slices: `[]type` (requires `sizeof` field)

Special Types:

-   `pad` - Null bytes for alignment/padding

## Performance

Benchmark results comparing `struc`, standard library `encoding/binary`, and manual encoding:

```bash
goos: windows
goarch: amd64
pkg: github.com/shengyanli1982/struc/v2
cpu: 12th Gen Intel(R) Core(TM) i5-12400F
BenchmarkArrayEncode-12          3369238               353.4 ns/op           113 B/op          3 allocs/op
BenchmarkSliceEncode-12          3211532               370.8 ns/op           113 B/op          3 allocs/op
BenchmarkArrayDecode-12          3399762               350.8 ns/op            56 B/op          2 allocs/op
BenchmarkSliceDecode-12          2802247               423.2 ns/op            96 B/op          4 allocs/op
BenchmarkEncode-12               2916241               419.9 ns/op           144 B/op          3 allocs/op
BenchmarkStdlibEncode-12         5687577               198.9 ns/op           136 B/op          3 allocs/op
BenchmarkManualEncode-12        59827994                24.90 ns/op           64 B/op          1 allocs/op
BenchmarkDecode-12               2764041               433.6 ns/op           112 B/op          9 allocs/op
BenchmarkStdlibDecode-12         5973495               199.0 ns/op            80 B/op          3 allocs/op
BenchmarkManualDecode-12        100918117               12.01 ns/op            8 B/op          1 allocs/op
BenchmarkFullEncode-12            736008              1752 ns/op             432 B/op          3 allocs/op
BenchmarkFullDecode-12            596174              2261 ns/op             536 B/op         54 allocs/op
BenchmarkFieldPool-12           18001530                56.40 ns/op          144 B/op          3 allocs/op
```

## Notes

-   Private fields are ignored during packing/unpacking
-   Bare slice types must have a corresponding `sizeof` field
-   All numeric types support both big and little endian encoding
-   The library caches reflection data for better performance

## License

MIT License - see LICENSE file for details
