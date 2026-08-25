# HTJ2K (High-Throughput JPEG 2000) Implementation

This package implements the HTJ2K block coder based on **ISO/IEC 15444-15:2019 / ITU-T T.814**.

## Implementation Status

### ✅ Completed Components

1. **MEL (Adaptive Run-Length Coder)** - `mel_spec.go`
   - Fully spec-compliant implementation
   - State machine with 13 states (MEL_E table)
   - Based on Clause 7.3.3 of ISO/IEC 15444-15

2. **MagSgn (Magnitude-Sign) Encoder/Decoder** - `magsgn.go`
   - Bit-level packing and unpacking
   - Little-endian bit order

3. **Framework and Data Structures**
   - HT block encoder/decoder structure
   - Quad-based scanning pattern
   - Segment assembly/disassembly

### ⚠️ Partial Implementation

1. **VLC (Variable Length Coding)** - `vlc_tables.go`
   - CxtVLC_table_0 and CxtVLC_table_1 are loaded from OpenJPH (Annex C)
   - Encoding/decoding integration is still partial (not production-ready)

2. **HT Cleanup Pass**
   - Core algorithm structure implemented
   - **TODO**: Full integration with complete VLC tables
   - **TODO**: Quad-pair interleaving and HT cleanup wiring (Clause 7.3.4)

## Technical Details

### MEL Algorithm

The MEL encoder uses a 13-state adaptive run-length coder:

```
State (k): 0  1  2  3  4  5  6  7  8  9  10 11 12
Exponent:  0  0  0  1  1  1  2  2  2  3  3  4  5
```

### VLC Tables

HTJ2K uses context-adaptive VLC tables (`CxtVLC_table_0` and `CxtVLC_table_1`) defined in Annex C.

Each entry contains:
- Context value (neighbor significance pattern)
- Rho (significance state)
- Unsigned offset (u_off)
- E_k value
- Codeword (little-endian)
- Code length

### HT Segments

HT code-blocks contain three byte-streams:

1. **MagSgn**: Magnitude and sign bits (grows forward)
2. **MEL**: Run-length coded symbols (grows forward)
3. **VLC**: Variable-length codes (grows backward)

## Usage Example

```go
// Create HT encoder
encoder := t1ht.NewHTEncoder(width, height)

// Encode code-block
data := []int32{...} // Wavelet coefficients
encoded, err := encoder.Encode(data, numPasses, roishift)

// Create HT decoder
decoder := t1ht.NewHTDecoder(width, height)

// Decode code-block
decoded, err := decoder.Decode(encoded, numPasses)
```

## Future Work

To achieve full HTJ2K compliance:

1. **VLC Table Validation** (High Priority)
   - Keep CxtVLC tables synced with OpenJPH source
   - Expand lookup-generation coverage

2. **Context Integration** (High Priority)
   - Validate context selection against OpenJPH edge cases
   - Wire context usage through the full HT cleanup pass

3. **Quad-Pair Interleaving** (Medium Priority)
   - Implement VLC bit-stream interleaving (Clause 7.3.4)

4. **U-VLC Decoding** (Medium Priority)
   - Implement unsigned residual VLC decoding (Clause 7.3.6)

5. **Testing and Validation**
   - Test against reference HTJ2K implementations (OpenJPH, OpenJPEG)
   - Validate with conformance test images

## DICOM Interoperability Fixtures

The schema-v2 bundle under `test-data/htj2k/interop-v1` covers DICOM transfer
syntaxes `.201`, `.202`, and `.203` across mono, RGB, YBR, 8/16-bit, signed,
odd-dimension, and multi-frame inputs. Its manifest records the exact NuGet
`fo-dicom.Codecs` version/source provenance and every artifact SHA256.

The local C# generator uses the managed `fo-dicom.Codecs` NuGet package only.
It generates reference files outside CI. Go and CI never execute C#, dotnet,
C++, or a native library; they consume and validate the committed files.

Pure-Go tests verify:

1. Exact Go/fo-dicom.Codecs codestream equality for every frame and syntax.
2. Go-to-Go, fo-to-Go, Go-to-fo, and fo-to-fo decoded raw equality.
3. DICOM transfer syntax, encapsulated frame order, and reconstructed frame bytes.
4. Main markers, TLM, tile-parts, packets, inclusion state, and cleanup blocks.
5. Manifest paths, lengths, hashes, fixture coverage, and codec provenance.

## References

- **ISO/IEC 15444-15:2019**: Information technology — JPEG 2000 image coding system — Part 15: High-Throughput JPEG 2000
- **ITU-T T.814 (06/2019)**: Same standard, ITU-T designation
- **OpenJPH**: Reference C++ implementation by Aous Naman
- **OpenJPEG 2.5+**: Includes HTJ2K decoding support

## License

This implementation is based on the publicly available ISO/IEC 15444-15 standard and is provided for educational and research purposes.
