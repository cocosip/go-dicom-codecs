# go-dicom-codecs

A Go library providing image compression/decompression codecs for medical imaging (DICOM), including RLE, JPEG, JPEG-LS, and JPEG 2000 families.

## Features

### RLE Family
- ✅ **RLE Lossless** [1.2.840.10008.1.2.5]

### JPEG Family
- ✅ **JPEG Baseline** (Process 1) - Lossy, 8-bit [1.2.840.10008.1.2.4.50]
- ✅ **JPEG Extended** (Process 2 & 4) - Lossy, 8/12-bit [1.2.840.10008.1.2.4.51]
- ✅ **JPEG Lossless** (Process 14) - All 7 predictors [1.2.840.10008.1.2.4.57]
- ✅ **JPEG Lossless SV1** (Process 14, Selection 1) - Predictor 1 only [1.2.840.10008.1.2.4.70]

### JPEG-LS Family
- ✅ **JPEG-LS Lossless** [1.2.840.10008.1.2.4.80]
- ✅ **JPEG-LS Near-Lossless** [1.2.840.10008.1.2.4.81]

### JPEG 2000 Family
- ✅ **JPEG 2000 Lossless** [1.2.840.10008.1.2.4.90]
- ✅ **JPEG 2000** (Lossy or Lossless) [1.2.840.10008.1.2.4.91]
- ✅ **JPEG 2000 Multi-component Lossless** [1.2.840.10008.1.2.4.92]
- ✅ **JPEG 2000 Multi-component** [1.2.840.10008.1.2.4.93]

### HTJ2K (High-Throughput JPEG 2000) Family
- ✅ **HTJ2K Lossless** [1.2.840.10008.1.2.4.201]
- ✅ **HTJ2K RPCL Lossless** [1.2.840.10008.1.2.4.202]
- ✅ **HTJ2K** (Lossy) [1.2.840.10008.1.2.4.203]

HTJ2K uses an independent OpenJPH-aligned Go engine. The committed schema-v2
interop bundle covers `.201`, `.202`, and `.203` across mono, RGB, YBR, signed,
odd-dimension, and multi-frame inputs. Go CI validates exact per-frame
codestream parity and four-way decoded artifacts generated locally with the
NuGet `fo-dicom.Codecs` reference. See
[`jpeg2000/htj2k/README.md`](jpeg2000/htj2k/README.md) for details.

## Installation

```bash
go get github.com/cocosip/go-dicom-codecs
```

## Architecture

The library is organized into the following packages:

- `codec/` - Shared errors and test helpers
- `rle/` - DICOM RLE Lossless codec (DICOM Part 5, Annex G)
- `jpeg/` - JPEG family implementations
  - `jpeg/standard/` - Shared low-level primitives: DCT, IDCT, Huffman tables/encoder/decoder, JPEG markers, bit reader/writer
  - `jpeg/baseline/` - JPEG Baseline codec (8-bit)
  - `jpeg/extended/` - JPEG Extended codec (8/12-bit)
  - `jpeg/lossless/` - JPEG Lossless codec (all 7 predictors)
  - `jpeg/lossless14sv1/` - JPEG Lossless SV1 codec (predictor 1 only)
- `jpegls/` - JPEG-LS implementations
  - `jpegls/runmode/` - Shared run-mode coding utilities
  - `jpegls/lossless/` - JPEG-LS Lossless codec (LOCO-I + Golomb-Rice)
  - `jpegls/nearlossless/` - JPEG-LS Near-Lossless codec
- `jpeg2000/` - JPEG 2000 public facade and codec adapters
  - `jpeg2000/lossless/` - JPEG 2000 Lossless codec (UIDs .90, .92)
  - `jpeg2000/lossy/` - JPEG 2000 Lossy codec (UIDs .91, .93)
  - `jpeg2000/openjpeg/` - Concrete classic JPEG 2000 engine
  - `jpeg2000/htj2k/` - HTJ2K DICOM adapters (UIDs .201-.203)
  - `jpeg2000/htj2k/openjph/` - Independent OpenJPH-aligned HTJ2K engine
  - `jpeg2000/internal/common/` - Family-neutral parser, I/O, models, and proven shared geometry

Auto-registration happens via blank imports. Including any codec package side-effect registers it with the go-dicom global registry:

```go
import _ "github.com/cocosip/go-dicom-codecs/jpeg/baseline"
```

## Usage

### Using the Codec Registry

Codecs integrate with [go-dicom](https://github.com/cocosip/go-dicom)'s global codec registry:

```go
package main

import (
    _ "github.com/cocosip/go-dicom-codecs/jpeg/baseline" // Auto-register

    "github.com/cocosip/go-dicom/pkg/dicom/transfer"
    "github.com/cocosip/go-dicom/pkg/imaging/codec"
)

func main() {
    registry := codec.GetGlobalRegistry()
    c, exists := registry.GetCodec(transfer.JPEGBaseline)
    if !exists {
        panic("codec not found")
    }

    // Encode: c.Encode(srcPixelData, dstPixelData, parameters)
    // Decode: c.Decode(srcPixelData, dstPixelData, parameters)
}
```

### Direct Package Usage

Each codec package exposes low-level `Encode`/`Decode` functions that work with raw `[]byte` pixel data.

#### JPEG Baseline

```go
import "github.com/cocosip/go-dicom-codecs/jpeg/baseline"

// Encode with quality 85 (1-100)
jpegData, err := baseline.Encode(pixelData, width, height, components, 85)

// Decode
decoded, w, h, comp, err := baseline.Decode(jpegData)
```

#### JPEG Extended (8/12-bit)

```go
import "github.com/cocosip/go-dicom-codecs/jpeg/extended"

// Encode 12-bit grayscale with quality 80
jpegData, err := extended.Encode(pixelData, width, height, 1, 12, 80)

// Decode
decoded, w, h, comp, bitDepth, err := extended.Decode(jpegData)
```

#### JPEG Lossless (All Predictors)

```go
import "github.com/cocosip/go-dicom-codecs/jpeg/lossless"

// Encode with predictor 4 (recommended, best compression)
jpegData, err := lossless.Encode(pixelData, width, height, components, bitDepth, 4)

// Decode
decoded, w, h, comp, bits, err := lossless.Decode(jpegData)
```

#### JPEG Lossless SV1 (Predictor 1 only)

```go
import "github.com/cocosip/go-dicom-codecs/jpeg/lossless14sv1"

// Encode (perfect reconstruction)
jpegData, err := lossless14sv1.Encode(pixelData, width, height, components, bitDepth)

// Decode
decoded, w, h, comp, bits, err := lossless14sv1.Decode(jpegData)
```

#### JPEG-LS Lossless

```go
import "github.com/cocosip/go-dicom-codecs/jpegls/lossless"

// LOCO-I algorithm
jpegLSData, err := lossless.Encode(pixelData, width, height, components, bitDepth)

// Decode
decoded, w, h, comp, bits, err := lossless.Decode(jpegLSData)
```

#### JPEG-LS Near-Lossless

```go
import "github.com/cocosip/go-dicom-codecs/jpegls/nearlossless"

// NEAR=3 guarantees maximum error of ±3 per pixel
jpegLSData, err := nearlossless.Encode(pixelData, width, height, components, bitDepth, 3)

// Decode — returns (pixels, w, h, comp, bits, actualNear, err)
decoded, w, h, comp, bits, actualNear, err := nearlossless.Decode(jpegLSData)
```

#### RLE Lossless

```go
import _ "github.com/cocosip/go-dicom-codecs/rle" // Auto-register

// Use via the go-dicom registry (transfer.RLELossless)
```

### JPEG 2000

JPEG 2000 encoding and decoding are available through the low-level `jpeg2000` package or the codec sub-packages.

#### Via codec sub-packages (DICOM registry)

```go
import (
    _ "github.com/cocosip/go-dicom-codecs/jpeg2000/lossless" // DICOM UIDs .90, .92
    _ "github.com/cocosip/go-dicom-codecs/jpeg2000/lossy"    // DICOM UIDs .91, .93
)
```

#### Direct encoding

```go
import "github.com/cocosip/go-dicom-codecs/jpeg2000"

params := jpeg2000.DefaultEncodeParams(width, height, components, bitDepth, false)
params.Lossless = true
params.NumLevels = 5

encoder := jpeg2000.NewEncoder(params)
compressed, err := encoder.Encode(pixelData)

decoder := jpeg2000.NewDecoder()
err = decoder.Decode(compressed)
```

#### Lossy with quality control

```go
params := jpeg2000.DefaultEncodeParams(width, height, 1, 8, false)
params.Lossless = false
params.Quality = 80       // 1–100
params.NumLevels = 5
params.TargetRatio = 10.0 // 10:1 compression ratio
params.UsePCRDOpt = true  // PCRD-opt rate-distortion truncation
```

#### Region of Interest (ROI)

```go
// Single rectangle ROI — higher quality than background
params.ROI = &jpeg2000.ROIParams{
    X0: 156, Y0: 156, Width: 200, Height: 200,
    Shift: 5, // Background compressed 2^5× more aggressively
}

// Multiple ROI regions
params.ROIConfig = &jpeg2000.ROIConfig{
    DefaultShift: 5,
    ROIs: []jpeg2000.ROIRegion{
        {ID: "lesion1", Rect: &jpeg2000.ROIParams{X0: 100, Y0: 100, Width: 80, Height: 80}, Shift: 6},
        {ID: "lesion2", Rect: &jpeg2000.ROIParams{X0: 300, Y0: 200, Width: 60, Height: 60}, Shift: 5},
    },
}

// Mask-based ROI (arbitrary shape)
params.ROIConfig = &jpeg2000.ROIConfig{
    ROIs: []jpeg2000.ROIRegion{
        {Shape: jpeg2000.ROIShapeMask, MaskWidth: width, MaskHeight: height, MaskData: mask, Shift: 5},
    },
}
```

#### Multi-layer and progressive

```go
params.NumLayers = 4
params.ProgressionOrder = 2 // RPCL — resolution-first progression
params.AppendLosslessLayer = true
```

### HTJ2K

HTJ2K uses the DICOM registry adapter and a separate OpenJPH-aligned Go
engine. Importing the package registers all three HTJ2K transfer syntaxes:

```go
import _ "github.com/cocosip/go-dicom-codecs/jpeg2000/htj2k" // DICOM UIDs .201-.203
```

To construct an adapter directly, use `htj2k.NewLosslessCodec()`,
`htj2k.NewLosslessRPCLCodec()`, or `htj2k.NewCodec(quality)`. The lossy codec
accepts a quality from 1 to 100 (80 by default); `htj2k.Parameters` also
exposes code-block dimensions and the number of wavelet decomposition levels.

## Codec Details

### JPEG Baseline
- **UID**: 1.2.840.10008.1.2.4.50
- **Compression**: Lossy DCT-based
- **Bit Depth**: 8-bit
- **Color Spaces**: Grayscale, RGB (auto-converted to YCbCr)
- **Options**: Quality (1-100)
- **Typical Compression**: 4-10x (quality dependent)

### JPEG Extended
- **UID**: 1.2.840.10008.1.2.4.51
- **Compression**: Lossy DCT-based
- **Bit Depth**: 8-bit and 12-bit
- **Color Spaces**: Grayscale, RGB
- **Options**: Quality (1-100)
- **Typical Compression**: 2-13x (quality and bit-depth dependent)
- **Status**: ✅ Production ready

### JPEG Lossless (All Predictors)
- **UID**: 1.2.840.10008.1.2.4.57
- **Compression**: Lossless prediction-based (7 predictors)
- **Bit Depth**: 2-16 bits (8-11 bit fully tested)
- **Color Spaces**: Grayscale, RGB
- **Predictors**:
  - Predictor 1 (Left): 1.90x compression
  - Predictor 2 (Above): 1.53x compression
  - Predictor 3 (Above-Left): 1.50x compression
  - **Predictor 4 (Ra+Rb-Rc): 3.64x compression** ⭐ Recommended
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
- **Status**: ✅ Production ready (all NEAR values 0-255 supported)

### JPEG 2000
- **UIDs**: 1.2.840.10008.1.2.4.90–.93
- **Wavelet**: 5/3 (lossless), 9/7 (lossy)
- **Bit Depth**: 8-16 bits, signed/unsigned
- **Color Spaces**: Grayscale, RGB (RCT/ICT transform)
- **Advanced features**:
  - Multi-tile encoding (`TileWidth`/`TileHeight`)
  - Multi-layer quality progression (`NumLayers`, `LayerRates`)
  - PCRD-opt rate-distortion truncation (`UsePCRDOpt`, `TargetRatio`)
  - Progression orders: LRCP, RLCP, RPCL, PCRL, CPRL
  - Region of Interest (ROI): rectangle, multiple regions, bitmap mask
  - Multi-component transforms (Part 2 MCT/MCC/MCO markers)
  - Custom precinct sizes and code-block dimensions
- **Status**: ✅ Production ready

### HTJ2K (High-Throughput JPEG 2000)
- **UIDs**:
  - `.201`: HTJ2K Lossless
  - `.202`: HTJ2K Lossless with RPCL progression
  - `.203`: HTJ2K lossy
- **Engine**: Independent pure-Go engine aligned with OpenJPH and
  ISO/IEC 15444-15 / ITU-T T.814
- **Wavelet**: Reversible 5/3 for lossless transfer syntaxes; irreversible
  9/7 for `.203` lossy encoding
- **Pixel data**: Mono, RGB, and YBR inputs; 8-bit and 16-bit, signed and
  unsigned coverage in the committed interoperability bundle
- **Encoding controls**: Lossy quality (1-100; default 80), code-block width
  and height (default 64x64), and 0-6 wavelet decomposition levels (default 5)
- **Progression**: `.202` forwards the configured LRCP, RLCP, RPCL, PCRL, or
  CPRL progression order; `.201` and `.203` use the OpenJPH-aligned RPCL
  default
- **Interoperability**: The committed `fo-dicom.Codecs` fixture bundle verifies
  exact per-frame codestreams and four-way decoded raw-pixel equality for all
  three transfer syntaxes
- **Status**: ✅ Production ready

### RLE Lossless
- **UID**: 1.2.840.10008.1.2.5
- **Compression**: Lossless run-length encoding (DICOM Part 5, Annex G)
- **Bit Depth**: Any (8/16-bit typical)
- **Color Spaces**: Grayscale, RGB, multi-channel
- **Supports**: Interleaved and planar pixel organization
- **Status**: ✅ Production ready

## Performance

Benchmarks measured on **Intel Core Ultra 9 185H**, Windows, `go test -bench=BenchmarkCodec -benchtime=3s`.
Images are **512×512 grayscale 8-bit** unless noted.

### RLE

| Operation | Time/op | Throughput |
|-----------|---------|-----------|
| Encode    | ~0.86ms | 306 MB/s  |
| Decode    | ~0.07ms | 3998 MB/s |

### JPEG Family

| Codec              | Encode time | Encode MB/s | Decode time | Decode MB/s |
|--------------------|-------------|-------------|-------------|-------------|
| JPEG Baseline      | ~6.8ms      | 38 MB/s     | ~30.7ms     | 9 MB/s      |
| JPEG Extended      | ~5.7ms      | 46 MB/s     | ~3.0ms      | 88 MB/s     |
| JPEG Lossless      | ~7.7ms      | 34 MB/s     | ~23ms       | 11 MB/s     |
| JPEG Lossless SV1  | ~7.7ms      | 34 MB/s     | ~22ms       | 12 MB/s     |

### JPEG-LS Family

| Codec                    | Encode time | Encode MB/s | Decode time | Decode MB/s |
|--------------------------|-------------|-------------|-------------|-------------|
| JPEG-LS Lossless         | ~6.2ms      | 42 MB/s     | ~7.9ms      | 33 MB/s     |
| JPEG-LS Near-Lossless    | ~10ms       | 26 MB/s     | ~8.5ms      | 31 MB/s     |

### JPEG 2000 Family

| Codec              | Encode time | Encode MB/s | Decode time | Decode MB/s |
|--------------------|-------------|-------------|-------------|-------------|
| JPEG 2000 Lossless | ~15.8ms     | 4.2 MB/s    | ~9.3ms      | 7.0 MB/s    |
| JPEG 2000 Lossy    | ~25.3ms     | 2.6 MB/s    | ~6.8ms      | 9.6 MB/s    |

### HTJ2K Family (256×256 images)

| Codec              | Encode time | Encode MB/s | Decode time | Decode MB/s |
|--------------------|-------------|-------------|-------------|-------------|
| HTJ2K Lossless     | ~2.9ms      | 22 MB/s     | ~12.5ms     | 5.2 MB/s    |
| HTJ2K RPCL Lossless| ~2.8ms      | 23 MB/s     | ~12.3ms     | 5.3 MB/s    |
| HTJ2K Lossy        | ~4.2ms      | 16 MB/s     | ~21.3ms     | 3.1 MB/s    |

Run `go test -bench=BenchmarkCodec -benchtime=3s ./...` to reproduce on your platform.

## Examples

See the [examples/](examples/) directory for complete working examples:

- `basic/` - Basic JPEG Baseline encode/decode (grayscale and RGB)
- `lossless/` - JPEG Lossless usage
- `all_codecs/` - Comprehensive example using all codecs
- `jpeg2000_basic/` - JPEG 2000 encode/decode fundamentals
- `jpeg2000_lossless/` - JPEG 2000 Lossless via codec registry
- `jpeg2000_progressive/` - Progressive decode and multi-layer encoding
- `jpeg2000_roi/` - Region of Interest encoding (rectangle, multiple, mask)
- `jpeg2000_part2_multicomponent/` - Multi-component (Part 2) encoding
- `export_png/` - Export decoded DICOM frames to PNG
- `extract_pixels/` - Extract raw pixel data from DICOM
- `dicom_transcoder/` - Transcode between DICOM transfer syntaxes
- `external_codec/` - Register a custom external codec

## Offline HTJ2K Interoperability Validation

`cmd/dicom-interop-validation` validates the committed schema-v2 HTJ2K bundle
without executing C#, dotnet, or a native library. It verifies the accepted
fo-dicom.Codecs version provenance, the complete fixture and transfer syntax
matrix, safe relative artifact paths, file lengths, and artifact SHA256 hashes.

Run it from the repository root:

```powershell
go run ./cmd/dicom-interop-validation
```

The default bundle is `test-data/htj2k/interop-v1`. Override its path when
validating a staged bundle:

```powershell
go run ./cmd/dicom-interop-validation --bundle <bundle-directory>
```

The validator contains no process launcher and also works as a compiled
executable when `PATH` is empty.

## Testing

```bash
# Run all tests
go test ./...

# Run a single package's tests
go test ./jpeg/baseline/...

# Run a specific test
go test -run TestName ./jpeg/baseline/...

# Run with verbose output
go test -v ./...

# Run benchmarks
go test -bench=. ./...
```

## Note

This library focuses solely on codec implementation. DICOM-specific concerns (encapsulation, fragmentation, metadata) are handled by [go-dicom](https://github.com/cocosip/go-dicom).

## Roadmap

See [TODO.md](TODO.md) for detailed development plans.

## License

MIT License

## Contributing

Contributions are welcome! Please submit issues or pull requests.
