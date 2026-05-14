# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

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

# Build (verify compilation)
go build ./...
```

## Architecture

This is a pure-Go DICOM image codec library (`github.com/cocosip/go-dicom-codec`). It implements encode/decode for JPEG, JPEG-LS, and JPEG 2000 families as required for DICOM transfer syntaxes. The library handles only codec logic — DICOM encapsulation and metadata are handled externally by `github.com/cocosip/go-dicom`.

### Package Layout

```
codec/          - Shared errors (ErrCodecNotFound, ErrInvalidParameter, etc.)
                  and TestPixelData helper for tests

jpeg/
  standard/     - Shared low-level primitives: DCT, IDCT, Huffman tables/encoder/
                  decoder, JPEG markers, bit reader/writer. All JPEG family codecs
                  depend on this.
  baseline/     - JPEG Baseline (UID .50), 8-bit only
  extended/     - JPEG Extended (UID .51), 8/12-bit
  lossless/     - JPEG Lossless (UID .57), all 7 predictors
  lossless14sv1/- JPEG Lossless SV1 (UID .70), predictor 1 only

jpegls/
  runmode/      - Shared run-mode coding utilities (run interrupt, run encoding)
  lossless/     - JPEG-LS Lossless (UID .80), LOCO-I + Golomb-Rice coding
  nearlossless/ - JPEG-LS Near-Lossless (UID .81), configurable NEAR error bound

jpeg2000/       - JPEG 2000 top-level encoder (encoder.go) and decoder (decoder.go)
  codestream/   - Codestream marker parser (SIZ, COD, QCD, SOT, MCT, MCC, MCO, etc.)
  colorspace/   - RGB/YCbCr colorspace conversion
  mqc/          - MQ arithmetic coder/decoder (47-state machine)
  t1/           - Tier-1: EBCOT block coder (bit-plane coding, coding passes)
  t2/           - Tier-2: packet builder/parser, tag trees, progression orders
  wavelet/      - 5/3 (lossless) and 9/7 (lossy) DWT implementations
  htj2k/        - HTJ2K (UIDs .201-.203) — EXPERIMENTAL: MEL, MagSgn, VLC tables
  testdata/     - Synthetic image generators used across JPEG 2000 tests
  validation/   - Reference/OpenJPEG comparison tests
```

### Codec Interface Pattern

Each codec package follows this structure:

1. **Low-level functions** (`encoder.go`, `decoder.go`): `Encode(pixelBytes, width, height, components, ...) ([]byte, error)` and `Decode(data []byte) ([]byte, width, height, components, ...)`. These work with raw `[]byte` pixel data.

2. **`codec.go`**: Implements the `go-dicom` `codec.Codec` interface with `Encode(oldPixelData, newPixelData, parameters)` and `Decode(...)` that iterate over frames. Includes an `init()` that auto-registers the codec with the global go-dicom registry using its DICOM transfer syntax UID.

3. **`parameters.go`**: Codec-specific `Parameters` struct (e.g., `JPEGBaselineParameters` with `Quality int`).

Auto-registration happens via blank imports. Including any codec package side-effect registers it:
```go
import _ "github.com/cocosip/go-dicom-codec/jpeg/baseline"
```

### JPEG 2000 Internals

The JPEG 2000 encoder (`jpeg2000/encoder.go`) uses `EncodeParams` which controls:
- `Lossless bool` — selects 5/3 (lossless) vs 9/7 (lossy) wavelet
- `Quality int` — quantization quality for lossy (1–100)
- `NumLevels int` — wavelet decomposition levels (0–6)
- `NumLayers int` + `TargetRatio float64` + `UsePCRDOpt bool` — multi-layer R-D optimization (PCRD-opt per ISO/IEC 15444-1 Annex J)
- `ProgressionOrder uint8` — LRCP/RLCP/RPCL/PCRL/CPRL
- Multi-component (Part 2): MCT/MCC/MCO marker support via `mct_builder.go`

HTJ2K (`jpeg2000/htj2k/`) is experimental — core MEL/MagSgn/VLC components work but end-to-end encode/decode is not production-ready.

### Testing Conventions

- Tests generate synthetic pixel data (gradients, patterns) rather than loading external files, except for `test-data/CT1_J2KI` used in validation.
- Lossless codecs verify **perfect reconstruction** (0-error pixel comparison).
- Lossy codecs verify error bounds (e.g., max pixel error ≤ N).
- `codec/test_helpers.go` provides `TestPixelData` — use it in any package's tests instead of implementing `imagetypes.PixelData` from scratch.
