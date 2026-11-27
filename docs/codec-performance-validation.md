# DICOM Codec Performance and Validation

## Scope and Order

Compare the pure-Go codecs in this module with the codecs shipped by
[fo-dicom.Codecs](https://github.com/fo-dicom/fo-dicom.Codecs). Work through the
families in this order. Do not start the next family until every transfer
syntax in the current family passes the performance gate and the correctness
checks.

| Order | Family | Transfer syntaxes | Reference implementation |
| --- | --- | --- | --- |
| 1 | RLE | RLE Lossless | Managed C# codec in `Codec/DicomRleCodec.cs` |
| 2 | JPEG | Process 1, Process 2/4, Process 14, Process 14 SV1 | libjpeg-turbo via `Dicom.Native` |
| 3 | JPEG-LS | Lossless, Near-Lossless | CharLS via `Dicom.Native` |
| 4 | JPEG 2000 | Lossless, Lossy | OpenJPEG via `Dicom.Native` |

HTJ2K is outside this pass. If it is added later, give it the same gate and
record its reference codestream size separately because the native reader can
emit padding.

## Constraints

- The codec packages (`codec/`, `rle/`, `jpeg/`, `jpegls/`, `jpeg2000/`) must
  remain pure Go. Do not add cgo, `import "C"`, or a new module dependency for
  a codec or for a benchmark path.
- `CGO_ENABLED=0 go build ./...` must succeed after every codec change.
- Existing dependencies may be used, but a new dependency is not a substitute
  for a small, correct implementation. Do not vendor a native library.
- The reference harness under `.cache/benchcmp/` is developer-only tooling and
  is not committed. It may call .NET and Python, but nothing under `rle/`,
  `jpeg/`, `jpegls/`, or `jpeg2000/` may import it.

## Shared Input and Timing Rules

- Extract Pixel Data once from the project corpus and the additional
  `dicom-test2` corpus. Save one raw buffer per frame, including every frame
  in multi-frame files. Keep frames with different content even when their
  dimensions match. Decode an already compressed source once and record its
  source transfer syntax.
- Record the sample ID, corpus-relative name, frame index, Rows, Columns,
  Samples per Pixel, Bits Allocated, Bits Stored, High Bit, Pixel
  Representation, Photometric Interpretation, Planar Configuration, byte
  order, pixel byte count, and SHA-256.
- Apply byte-order conversion, plane reordering, and unused-bit handling once
  during extraction. Preserve sample values, signedness, and color
  interpretation. Check the input length, SHA-256, and metadata before each
  implementation runs.
- List each frame, transfer syntax, and parameter set as a separate case.
  Cover small and large frames, 8/12/16-bit data, grayscale and color,
  signedness, and multi-frame data. Record the effective options, including
  quality, subsampling, Huffman strategy, predictor, point transform, restart
  interval, NEAR, decomposition levels, code blocks, layers, and progression
  order.
- Build both implementations in Release mode on the same machine. Record the
  architecture, Go and .NET versions, native-library revision, and active
  SIMD paths. Pin both processes to the same logical CPU when the platform
  allows it.
- Time one implementation per process. Finish the whole family for one
  implementation and wait for the process to exit before starting the other.
  Keep the case order, warm-up, and batch policy equal on both sides. Record
  process start and end times.
- Time operations on preloaded buffers. DICOM parsing, disk I/O, process
  startup, hashing, and comparisons happen outside the timed interval. For
  decode, both sides receive the identical frozen codestream.
- Warm up three operations per case and direction. Run five timed rounds.
  Each round runs a nominal batch and doubles the batch until the round lasts
  at least 5 ms. Normalize by the actual operation count. Take the minimum
  round time for each implementation; that removes scheduler, GC, and timer
  granularity noise while preserving the codec cost. Save every round, not
  only the minimum.
- Record the total elapsed time per file and direction. Do not use MB/s as
  the acceptance statistic.

## Standard Validation Workflow

1. **Benchmark encoding.** Time Go and the reference on the same raw Pixel
   Data and effective settings. Compare the minimum round time per operation
   across the five rounds. Keep the same batch policy and warm-up on both
   sides.
2. **Check the encoded payload.** Outside the timer, compare the complete
   encoded frame payloads: length, SHA-256, frame boundaries, fragment
   headers, and container padding. For RLE, byte equality with the fo-dicom
   managed encoder is expected and currently observed. For the native JPEG,
   JPEG-LS, and JPEG 2000 encoders, byte equality is not a correctness
   requirement; a different but valid codestream is acceptable if the
   cross-decode checks below pass. Record whether the payloads match.
3. **Cross-decode both codestreams.** Feed each encoder output to both
   decoders. Require matching output dimensions, length, bit depth,
   signedness, color interpretation, and planar layout. For lossless
   syntaxes, require byte-identical Pixel Data and equality with the source.
   For lossy syntaxes, record the reconstruction error under the agreed
   settings. Save both codestreams when a cross-decode fails.
4. **Benchmark decoding.** Both implementations decode the same reference
   codestream. Time the decoder on preloaded bytes and compare the minimum
   round time.
5. **Apply the performance gate.** For each case and direction, compute

   ```
   ratio = Go minimum time / reference minimum time
   ```

   A ratio of `1.10` or less passes. A ratio below `1.00` means Go is faster.
   The gate applies per frame, syntax, configuration, and direction; do not
   pass a family on an average. After all cases pass, run the
   [final checks](#retests-and-final-checks).

## Family Notes and Settings

### RLE

- The reference is the managed `DicomRleCodec.cs` implementation. The C#
  harness contains a direct port with the fo-dicom buffer types replaced by
  `byte[]`, so DICOM container overhead is excluded on both sides.
- Layout rules: one segment per byte plane, most-significant byte first,
  segment offsets are even, and the final payload is padded to even length.
  Segments 0..n map to samples and bytes exactly as the reference does.
- Compare the encoded bytes. The Go encoder currently matches the reference
  byte for byte, including literal/replicate boundaries, segment offsets,
  and padding. Keep that check in the harness.
- Decode the reference-encoded corpus with both decoders and require exact
  reconstruction. Also round-trip Go-encoded frames through the Go decoder.
- Exercise run lengths on both sides of the 128-byte PackBits boundary,
  8/16-bit samples, one and three samples per pixel, interleaved and planar
  layouts, odd pixel counts, and multi-frame data.
- The Go decoder uses pointer arithmetic only for strided writes after each
  run has been bounds-checked. This is a deliberate hot-path optimization;
  it must not be extended to unchecked input or output.

### JPEG

- The reference is libjpeg-turbo through `Dicom.Native`.
- Cover Process 1, Process 2/4, Process 14, and Process 14 SV1. Align the
  effective settings: quality, chroma subsampling (the fo-dicom lossy color
  default is 4:2:2, while earlier Go runs used 4:4:4), RGB versus YCbCr
  output, optimized versus standard Huffman tables, restart interval,
  predictor, and point transform.
- Lossless syntaxes must restore the source exactly. Lossy syntaxes compare
  pixels with the agreed error metric; do not require byte equality with
  libjpeg-turbo output.
- Decode timing uses a frozen reference codestream. If the Go encoder output
  is not byte-identical, cross-decode it with the reference decoder before
  accepting a decode result.

### JPEG-LS

- The reference is CharLS through `Dicom.Native`.
- Cover lossless and near-lossless with the agreed NEAR values, 8/12/16-bit
  samples, signed and unsigned data, grayscale and color.
- For lossless, require exact pixels. For near-lossless, verify that the
  maximum sample error stays within NEAR. When signed samples cross the sign
  boundary, measure the error on the signed values as well as the unsigned
  code values.
- Decode timing uses a frozen reference codestream; cross-decode Go output
  with the reference decoder.

### JPEG 2000

- The reference is OpenJPEG through `Dicom.Native`.
- Cover lossless and lossy, 8/12/16-bit data, grayscale and color, multiple
  decomposition levels, code-block sizes, layers, progression orders, and
  rate settings. Record every effective parameter.
- Lossless streams must restore the source exactly. Lossy streams use the
  agreed reconstruction error.
- Decode timing uses a frozen reference codestream. Cross-decode both
  directions before accepting the result.

## Current RLE Baseline

One run on Windows 10.0.26300, Intel Core Ultra 9 185H, Go 1.27.4, .NET
10.0.403, Release builds. Five rounds per case and direction, each round at
least 5 ms, minimum round time shown. `ratio` is Go time divided by C# time.
All 21 cases pass; the worst ratio is 0.97. Geomean: encode 0.51, decode 0.69.

| Case | Raw bytes | Go enc ms | C# enc ms | enc ratio | Go dec ms | C# dec ms | dec ratio |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| mono8_128x128 | 16,384 | 0.021 | 0.048 | 0.43 | 0.007 | 0.013 | 0.59 |
| mono8_256x256 | 65,536 | 0.059 | 0.127 | 0.47 | 0.017 | 0.037 | 0.46 |
| mono8_512x512 | 262,144 | 0.246 | 0.499 | 0.49 | 0.138 | 0.195 | 0.71 |
| mono8_960x540 | 518,400 | 0.412 | 0.843 | 0.49 | 0.300 | 0.333 | 0.90 |
| mono8_1920x1080 | 2,073,600 | 6.102 | 8.325 | 0.73 | 1.772 | 2.248 | 0.79 |
| mono16_128x128 | 32,768 | 0.044 | 0.074 | 0.59 | 0.015 | 0.017 | 0.89 |
| mono16_256x256 | 131,072 | 0.131 | 0.259 | 0.51 | 0.043 | 0.088 | 0.49 |
| mono16_512x512 | 524,288 | 0.562 | 1.072 | 0.52 | 0.211 | 0.289 | 0.73 |
| mono16_888x459 | 815,184 | 1.981 | 2.134 | 0.93 | 0.579 | 0.742 | 0.78 |
| mono12_256x256 | 131,072 | 0.127 | 0.392 | 0.32 | 0.034 | 0.122 | 0.28 |
| mono12_512x512 | 524,288 | 0.540 | 1.445 | 0.37 | 0.211 | 0.351 | 0.60 |
| mono12_888x459 | 815,184 | 1.226 | 1.790 | 0.69 | 0.713 | 0.738 | 0.97 |
| rgb8_128x128 | 49,152 | 0.066 | 0.107 | 0.62 | 0.021 | 0.026 | 0.82 |
| rgb8_256x256 | 196,608 | 0.190 | 0.391 | 0.49 | 0.091 | 0.106 | 0.86 |
| rgb8_512x512 | 786,432 | 0.809 | 1.561 | 0.52 | 0.376 | 0.392 | 0.96 |
| rgb8_960x540 | 1,555,200 | 1.335 | 2.560 | 0.52 | 0.753 | 0.835 | 0.90 |
| rgb8_1920x1080 | 6,220,800 | 19.878 | 21.600 | 0.92 | 6.595 | 10.744 | 0.61 |
| rgb16_96x80 | 46,080 | 0.068 | 0.122 | 0.55 | 0.019 | 0.050 | 0.38 |
| mono16_96x80 | 15,360 | 0.016 | 0.056 | 0.29 | 0.008 | 0.009 | 0.91 |
| mono8_96x80 | 7,680 | 0.006 | 0.014 | 0.45 | 0.004 | 0.004 | 0.84 |
| rgb8_96x80 | 23,040 | 0.013 | 0.040 | 0.34 | 0.008 | 0.011 | 0.71 |

The C# numbers are from the same run and move with CPU state; rerun both
harnesses before quoting the table. The RLE encoder currently matches the
reference byte for byte on every case.

JPEG, JPEG-LS, and JPEG 2000 remain to be benchmarked and optimized. Their
tables belong here once their families are complete.

## Retests and Final Checks

- Report pass, fail, missing, and incompatible reference cases separately.
  Use aggregate figures for trends only; the gate applies per case.
- Repeat borderline or unstable cases with the same builds, CPU, raw input,
  and shared codestream. Increase the round length equally on both sides.
  Keep the original rounds and log any independently verified interference.
  If interference invalidates a round, repeat that round for both
  implementations. Combine valid rounds by operation count and use the
  minimum valid round for the gate.
- Optimize only cases above the tolerance. Rerun the family's full matrix and
  the byte/cross-decode checks after changing codec behavior or performance.
- Test both public DICOM paths: decode Go-encoded files with fo-dicom and
  fo-dicom-encoded files with Go. Check the transfer syntax, frame
  boundaries, Pixel Data, dimensions, bit depth, signedness, Photometric
  Interpretation, and Planar Configuration.
- Run formatting, vet, tests, and lint after the interoperability checks:

  ```powershell
  gofmt -l ./codec ./rle ./jpeg ./jpegls ./jpeg2000 ./examples
  go vet ./codec/... ./rle/... ./jpeg/... ./jpegls/... ./jpeg2000/... ./examples/...
  go test ./codec/... ./rle/... ./jpeg/... ./jpegls/... ./jpeg2000/... ./examples/...
  golangci-lint run --timeout=10m ./codec/... ./rle/... ./jpeg/... ./jpegls/... ./jpeg2000/... ./examples/...
  ```

- Archive the extraction manifest, build and codec settings, input and
  codestream hashes, per-round data, summaries, and cross-decode results
  under the ignored `.cache/benchcmp/` directory. Do not create another cache
  directory. Exclude source DICOM files, extracted pixels, and patient
  metadata from shared replay packages.

## Running the Benchmarks

The harness lives under the ignored `.cache/benchcmp/` directory and is not
committed. It needs .NET 10, Python 3, and the fo-dicom.Codecs checkout with
its native `Dicom.Native.dll`.

```powershell
# 1. Extract raw frames and write the manifest.
go run .cache/benchcmp/dataset/main.go -out .cache/benchcmp/data

# 2. Build and run the managed/native reference. This also writes the
#    reference codestreams used by the Go decode benchmark.
dotnet build .cache/benchcmp/csharp/BenchCmp.csproj -c Release
dotnet .cache/benchcmp/csharp/bin/Release/net10.0/BenchCmp.dll `
  --data .cache/benchcmp/data --family rle

# 3. Run the Go codec on the same manifest and reference codestreams.
go run .cache/benchcmp/gobench/main.go -data .cache/benchcmp/data -family rle

# 4. Compare elapsed time per file and direction. The CSV is optional.
python .cache/benchcmp/compare.py `
  .cache/benchcmp/data/csharp-results-rle.jsonl `
  .cache/benchcmp/data/go-results-rle.jsonl `
  --csv .cache/benchcmp/data/rle-compare.csv
```

Replace `rle` with `jpeg`, `jpegls`, or `jpeg2000` once those families are
ready. Keep the two implementations in separate processes and run them
serially.

For focused work, `go test -bench` is useful for a codec kernel, but its
numbers are not the acceptance result. The acceptance result is the
file-level comparison from the two harnesses above.

## Notes from Earlier Tests

- Earlier runs of this document were copied from a Rust project. They used
  Rust, Cargo, `1.05`, and MB/s. The gate here is Go versus the fo-dicom
  reference, a `1.10` ratio, and elapsed time per file and direction.
- Very short operations produced zero readings on the Windows Go timer when a
  round lasted less than a few milliseconds. Rounds now last at least 5 ms,
  and the harness doubles the batch when necessary.
- The Go RLE encoder writes literal runs directly into the final buffer and
  splits them at the 128-byte PackBits boundary. The decoder bounds-checks
  each run before doing strided writes. This removed the temporary buffer,
  repeated copies, and per-byte bounds checks that dominated 16-bit and RGB
  decoding.
- RLE output sizes and segment offsets must match the reference exactly.
  Encoders for the native codec families are not required to emit the same
  bytes as libjpeg-turbo, CharLS, or OpenJPEG; cross-decode and pixel checks
  decide correctness there.
- The native HTJ2K path can emit substantial empty padding. If HTJ2K is
  added, identify padding from the codestream structure rather than
  zero-valued bytes, and record file size separately from codec speed.
