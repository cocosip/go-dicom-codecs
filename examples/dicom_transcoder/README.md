# DICOM Automatic Compression Tool

`dicom_transcoder` reads one DICOM file and writes compressed copies for the
transfer syntaxes supported by this repository.

## Build

```powershell
go build -o dicom_transcoder.exe .
```

## Command

```text
dicom_transcoder <input-file> [--output-dir <directory>] [--format <suffix>]
```

`--output-dir` and `-o` select the output directory. Without either option,
the tool writes to `<input-base>_compressed` next to the input file.

`--format` selects one format suffix, case-insensitively. Without it, the tool
attempts every target. `--help`, `-h`, and `/?` print command usage.

```powershell
dicom_transcoder.exe D:\dicom\study1.dcm
dicom_transcoder.exe D:\dicom\study1.dcm -o D:\dicom\compressed
dicom_transcoder.exe D:\dicom\study1.dcm --format j2k_lossless
```

## Output Files

Each generated file uses this name pattern:

```text
<input-base>_<format-suffix>.dcm
```

The default target order and suffixes are:

| Format | Suffix |
| --- | --- |
| RLE Lossless | `rle` |
| JPEG Baseline Process 1 | `jpeg_baseline` |
| JPEG Extended Process 2/4 | `jpeg_process2_4` |
| JPEG Lossless Process 14 | `jpeg_lossless_14` |
| JPEG Lossless Process 14 SV1 | `jpeg_lossless_sv1` |
| JPEG-LS Lossless | `jpegls_lossless` |
| JPEG-LS Near-Lossless | `jpegls_near_lossless` |
| JPEG 2000 Lossless | `j2k_lossless` |
| JPEG 2000 Lossy | `j2k_lossy` |
| HTJ2K Lossless | `htj2k_lossless` |
| HTJ2K Lossless RPCL | `htj2k_lossless_rpcl` |
| HTJ2K Lossy | `htj2k_lossy` |

Each target reports one of these outcomes without stopping later targets:

- `Success`: the compressed DICOM file was written.
- `Unsupported`: the image attributes are not supported by that target.
- `Skipped`: the target was intentionally not executed.
- `Failed`: the codec or writer returned an error.

The command exits with status 1 only when every selected target fails.
