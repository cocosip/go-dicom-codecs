# go-dicom-codecs

A Go library providing image compression/decompression codecs for medical imaging (DICOM), including JPEG, JPEG-LS, and JPEG 2000 families.

## Features

### RLE Family
- **RLE Lossless** [1.2.840.10008.1.2.5]

### JPEG Family
- ✅ **JPEG Baseline** (Process 1) - Lossy, 8-bit [1.2.840.10008.1.2.4.50]
- ✅ **JPEG Extended** (Process 2 & 4) - Lossy, 8/12-bit [1.2.840.10008.1.2.4.51]
- ✅ **JPEG Lossless** (Process 14, Selection 1) - All 7 predictors [1.2.840.10008.1.2.4.57]
- ✅ **JPEG Lossless SV1** (Process 14, Selection 1) - Predictor 1 only [1.2.840.10008.1.2.4.70]

### JPEG-LS Family
- ✅ **JPEG-LS Lossless** [1.2.840.10008.1.2.4.80]
- ✅ **JPEG-LS Near-Lossless** [1.2.840.10008.1.2.4.81]

### JPEG 2000 Family
- ✅ **JPEG 2000 Lossless** [1.2.840.10008.1.2.4.90]
- ✅ **JPEG 2000** (Lossy or Lossless) [1.2.840.10008.1.2.4.91]
- ✅ **JPEG 2000 Multi-component Lossless** [1.2.840.10008.1.2.4.92]
- ✅ **JPEG 2000 Multi-component** [1.2.840.10008.1.2.4.93]

### HTJ2K (High-Throughput JPEG 2000) Family 🚧
- 🔬 **HTJ2K Lossless** [1.2.840.10008.1.2.4.201] - *Experimental*
- 🔬 **HTJ2K RPCL Lossless** [1.2.840.10008.1.2.4.202] - *Experimental*
- 🔬 **HTJ2K** (Lossy/Lossless) [1.2.840.10008.1.2.4.203] - *Experimental*

> **⚠️ HTJ2K Implementation Note**: The HTJ2K codecs are currently in **experimental/research status**.
> The implementation includes spec-compliant MEL (adaptive run-length) encoder/decoder and VLC tables
> extracted from OpenJPEG. Core decoding components are functional but not production-ready. For production
> use, consider established libraries like [OpenJPH](https://github.com/aous72/OpenJPH) or
> OpenJPEG 2.5+. See [`jpeg2000/htj2k/README.md`](jpeg2000/htj2k/README.md) for implementation details.

## Installation

```bash
go get github.com/cocosip/go-dicom-codecs
```

## Architecture

The library is organized into the following packages:

- `codec/` - Core codec interfaces and registry
- `rle/` - DICOM RLE Lossless codec
- `jpeg/` - JPEG family implementations
  - `jpeg/common/` - Shared utilities (Huffman, DCT, markers, etc.)
  - `jpeg/baseline/` - JPEG Baseline codec
  - `jpeg/extended/` - JPEG Extended codec (8/12-bit)
  - `jpeg/lossless/` - JPEG Lossless codec (all 7 predictors)
  - `jpeg/lossless14sv1/` - JPEG Lossless SV1 codec (predictor 1 only)
- `jpegls/` - JPEG-LS implementations
  - `jpegls/lossless/` - JPEG-LS Lossless codec
  - `jpegls/nearlossless/` - JPEG-LS Near-Lossless codec
- `jpeg2000/` - JPEG 2000 implementations (planned)

## Usage

### Local Process-Isolated Validation

`cmd/dicom-interop-validation` is a manually run diagnostic tool. It embeds
five anonymized DICOM fixtures and validates Go encoders against fo-dicom
Native Codecs. For every supported format, fo-dicom Native decodes the source
fixture to uncompressed DICOM, Go encodes that image in a separate process,
and fo-dicom Native decodes Go's output before the tool compares image metadata
and samples. It is not part of the normal test suite or CI.

Running it requires a .NET 10 SDK so that the bundled
`cmd/fo-dicom-native-worker` project can restore and run fo-dicom Native
Codecs. Use `--fo-native-project <path>` to select a different local Native
worker project.

#### Run locally

From the repository root in PowerShell, first confirm that Go and a .NET 10
SDK are available:

```powershell
go version
dotnet --list-sdks
```

The first run restores the Worker NuGet packages automatically. To restore
them explicitly before running the matrix:

```powershell
dotnet restore cmd\fo-dicom-native-worker\fo-dicom-native-worker.csproj
```

Run the full 12-format matrix and retain all produced DICOM files:

```powershell
go run ./cmd/dicom-interop-validation --parallel 4 --workdir artifacts\interop-validation
```

Run one format while investigating a failure:

```powershell
go run ./cmd/dicom-interop-validation --format jpeg-lossless-14-sv1 --parallel 1 --workdir artifacts\interop-jpeg-lossless-sv1
```

Use the bundled Native Worker by default, or provide a different local Worker
project. List the supported arguments with `--help`:

```powershell
go run ./cmd/dicom-interop-validation --help
go run ./cmd/dicom-interop-validation --fo-native-project D:\Code\fo-native-worker\fo-native-worker.csproj
```

Supported `--format` values are `rle`, `jpeg-process-1`,
`jpeg-process-2-4`, `jpeg-lossless-14`, `jpeg-lossless-14-sv1`,
`jpeg-ls-lossless`, `jpeg-ls-near-lossless`, `jpeg2000-lossless`,
`jpeg2000-lossy`, `htj2k-lossless`, `htj2k-lossless-rpcl`, and `htj2k-lossy`.

The output has these meanings:

- `INTEROP|pass|format=...` means every executed fixture for that format was
  accepted by fo-dicom Native and met the configured pixel tolerance.
- `INTEROP|skip|...` means the fixture is intentionally outside the Native
  decoder's known support, such as the JPEG-LS multi-frame Native baseline.
- `INTEROP|fail|...` means fo-dicom Native decoded the Go output but detected
  a metadata or sample mismatch. The command exits with code 1 and retains the
  work directory for analysis; this is a codec interoperability result, not
  necessarily a tool startup failure.

Each retained fixture directory contains `prepared.dcm` (Native-decoded
source), `encoded.dcm` (Go output), and `decoded.dcm` (Native-decoded Go
output).

The default run directory is removed after success and retained after failure.
Use `--workdir` to retain compressed and decoded DICOM artifacts explicitly.

### Using the Codec Registry

```go
package main

import (
    "github.com/cocosip/go-dicom-codecs/codec"
    _ "github.com/cocosip/go-dicom-codecs/jpeg/baseline" // Auto-register
)

func main() {
    // Get codec by UID
    c, err := codec.Get("1.2.840.10008.1.2.4.50")
    if err != nil {
        panic(err)
    }

    // Encode
    params := codec.EncodeParams{
        PixelData:  pixelData,
        Width:      512,
        Height:     512,
        Components: 1, // Grayscale
        BitDepth:   8,
        Options:    nil, // Use defaults
    }

    compressed, err := c.Encode(params)
    if err != nil {
        panic(err)
    }

    // Decode
    result, err := c.Decode(compressed)
    if err != nil {
        panic(err)
    }
}
```

### Direct Package Usage

```go
import "github.com/cocosip/go-dicom-codecs/jpeg/baseline"

func main() {
    // Encode with quality 85
    jpegData, err := baseline.Encode(pixelData, width, height, components, bitDepth, 85)
    if err != nil {
        panic(err)
    }

    // Decode
    decoded, w, h, comp, bits, err := baseline.Decode(jpegData)
    if err != nil {
        panic(err)
    }
}
```

### JPEG Lossless (All Predictors)

```go
import "github.com/cocosip/go-dicom-codecs/jpeg/lossless"

func main() {
    // Lossless encoding with predictor 4 (best compression)
    predictor := 4 // 1-7, or 0 for auto-select
    jpegData, err := lossless.Encode(pixelData, width, height, components, bitDepth, predictor)
    if err != nil {
        panic(err)
    }

    // Decode
    decoded, w, h, comp, bits, err := lossless.Decode(jpegData)
    if err != nil {
        panic(err)
    }
}
```

### JPEG Lossless SV1 (Predictor 1 only)

```go
import "github.com/cocosip/go-dicom-codecs/jpeg/lossless14sv1"

func main() {
    // Lossless encoding (perfect reconstruction)
    jpegData, err := lossless14sv1.Encode(pixelData, width, height, components, bitDepth)
    if err != nil {
        panic(err)
    }

    // Decode
    decoded, w, h, comp, bits, err := lossless14sv1.Decode(jpegData)
    if err != nil {
        panic(err)
    }
}
```

### JPEG-LS Lossless

```go
import "github.com/cocosip/go-dicom-codecs/jpegls/lossless"

func main() {
    // JPEG-LS lossless encoding (LOCO-I algorithm)
    jpegLSData, err := lossless.Encode(pixelData, width, height, components, bitDepth)
    if err != nil {
        panic(err)
    }

    // Decode
    decoded, w, h, comp, bits, err := lossless.Decode(jpegLSData)
    if err != nil {
        panic(err)
    }
}
```

### JPEG-LS Near-Lossless

```go
import "github.com/cocosip/go-dicom-codecs/jpegls/nearlossless"

func main() {
    // JPEG-LS near-lossless encoding with NEAR=3
    // Guarantees maximum error of ±3 per pixel
    near := 3 // Error bound (0-255), 0 = lossless
    jpegLSData, err := nearlossless.Encode(pixelData, width, height, components, bitDepth, near)
    if err != nil {
        panic(err)
    }

    // Decode
    decoded, w, h, comp, bits, actualNear, err := nearlossless.Decode(jpegLSData)
    if err != nil {
        panic(err)
    }
}
```

## Codec Details

### JPEG Baseline
- **UID**: 1.2.840.10008.1.2.4.50
- **Compression**: Lossy DCT-based
- **Bit Depth**: 8-bit
- **Color Spaces**: Grayscale, RGB (auto-converted to YCbCr)
- **Options**: Quality (1-100)
- **Typical Compression**: 4-10x (quality dependent)

### JPEG Lossless (All Predictors)
- **UID**: 1.2.840.10008.1.2.4.57
- **Compression**: Lossless prediction-based (7 predictors)
- **Bit Depth**: 2-16 bits (8-11 bit fully tested)
- **Color Spaces**: Grayscale, RGB
- **Predictors**:
  - Predictor 1 (Left): 1.90x compression
  - Predictor 2 (Above): 1.53x compression
  - Predictor 3 (Above-Left): 1.50x compression
  - **Predictor 4 (Ra+Rb-Rc): 3.64x compression** ⭐ **Recommended**
  - Predictor 5 (Adaptive): 1.91x compression
  - Predictor 6 (Adaptive): 1.89x compression
  - Predictor 7 (Average): 1.52x compression
- **Perfect Reconstruction**: Yes (0 errors)
- **Status**: ✅ Production ready

### JPEG Lossless SV1
- **UID**: 1.2.840.10008.1.2.4.70
- **Compression**: Lossless prediction-based (Predictor 1 only)
- **Bit Depth**: 2-16 bits
- **Color Spaces**: Grayscale, RGB
- **Typical Compression**: 1.90x
- **Perfect Reconstruction**: Yes (0 errors)
- **Status**: ✅ Production ready

### JPEG Extended
- **UID**: 1.2.840.10008.1.2.4.51
- **Compression**: Lossy DCT-based
- **Bit Depth**: 8-bit and 12-bit
- **Color Spaces**: Grayscale, RGB
- **Options**: Quality (1-100)
- **Typical Compression**: 2-13x (quality and bit-depth dependent)
- **Status**: ✅ Production ready

### JPEG-LS Lossless
- **UID**: 1.2.840.10008.1.2.4.80
- **Compression**: Lossless context-adaptive (LOCO-I algorithm)
- **Bit Depth**: 2-16 bits
- **Color Spaces**: Grayscale, RGB
- **Algorithm**: Context modeling + Golomb-Rice coding + MED predictor
- **Typical Compression**:
  - Grayscale 8-bit: 4.17x
  - RGB 8-bit: 2.51x
  - 12-bit: 2.94x
- **Perfect Reconstruction**: Yes (0 errors)
- **Advantages**: Better compression than JPEG Lossless, lower complexity than JPEG 2000
- **Status**: ✅ Production ready

### JPEG-LS Near-Lossless
- **UID**: 1.2.840.10008.1.2.4.81
- **Compression**: Near-lossless with configurable error bound (NEAR parameter)
- **Bit Depth**: 2-16 bits
- **Color Spaces**: Grayscale, RGB
- **NEAR Parameter**: 0-255 (maximum error per pixel)
  - NEAR=0: Lossless (identical to JPEG-LS Lossless)
  - NEAR=1-3: Visually lossless with high compression
  - NEAR=7+: Higher compression with visible differences
- **Typical Compression** (64x64 grayscale):
  - NEAR=0: 4.17x (lossless)
  - NEAR=1: 4.53x
  - NEAR=3: 5.79x
  - NEAR=7: 4.56x
  - NEAR=10: 5.08x
- **Error Guarantee**: |reconstructed - original| ≤ NEAR for every pixel
- **Use Cases**: Medical imaging with acceptable error tolerance, archival with space constraints
- **Status**: ✅ Production ready (all NEAR values 0-255 supported)

## Performance

Benchmarks on 512x512 grayscale images:

### JPEG Family
- **JPEG Baseline** - Encode: ~1.17ms, Decode: ~2.97ms
- **JPEG Extended** - Encode: ~1.2ms, Decode: ~3.0ms (8-bit)
- **JPEG Lossless** - Encode: ~12.5ms, Decode: ~8.3ms (predictor 1)
- **JPEG Lossless SV1** - Encode: ~3.65ms, Decode: ~40.2ms

### JPEG-LS Family
- **JPEG-LS Lossless** - Encode: ~15ms, Decode: ~12ms
- **JPEG-LS Near-Lossless** - Encode: ~14ms, Decode: ~11ms (NEAR=3)

*Note: Benchmarks may vary by hardware and image characteristics. Run `go test -bench=.` for your platform.*

## Examples

See the [examples/](examples/) directory for complete working examples:

- `all_codecs_example.go` - Comprehensive example using all three codecs
- `codec_usage.go` - Basic codec registry usage
- `complete_example.go` - Complete DICOM integration example

Run examples:
```bash
go run examples/all_codecs_example.go
```

## Testing

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run benchmarks
go test -bench=. ./...
```

## Note

This library focuses solely on codec implementation. DICOM-specific concerns (encapsulation, fragmentation, metadata) are handled by external DICOM libraries.

## Roadmap

See [TODO.md](TODO.md) for detailed development plans.

## License

MIT License

## Contributing

Contributions are welcome! Please submit issues or pull requests.
