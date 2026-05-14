# Repository Guidelines

## Project Structure & Module Organization

This is a Go module for DICOM image codecs. Core interfaces and shared error types live in `codec/`. Codec implementations are grouped by family: `jpeg/` for baseline, extended, lossless, and lossless14sv1 JPEG; `jpegls/` for lossless and near-lossless JPEG-LS; and `jpeg2000/` for JPEG 2000, HTJ2K, wavelet, MQC, tier-1, tier-2, and validation code. Usage samples are under `examples/`. Test fixtures and reference inputs are in `test-data/` and package-local `testdata/` directories. Keep new tests beside the package they verify.

## Build, Test, and Development Commands

- `go mod download` downloads module dependencies.
- `go mod verify` checks cached dependencies against `go.sum`.
- `go test ./codec/... ./jpeg/... ./jpeg2000/... ./jpegls/... ./examples/...` runs the main test set.
- `go test -race -short -timeout 30m -coverprofile=coverage.out -covermode=atomic ./codec/... ./jpeg/... ./jpeg2000/... ./jpegls/... ./examples/...` mirrors CI coverage and race checks.
- `go vet ./codec/... ./jpeg/... ./jpeg2000/... ./jpegls/... ./examples/...` runs static vet checks.
- `golangci-lint run --timeout=10m ./codec/... ./jpeg/... ./jpeg2000/... ./jpegls/... ./examples/...` runs the configured lint suite.

## Coding Style & Naming Conventions

Use standard Go formatting: run `gofmt` on edited files and keep imports organized. Package names should be short, lowercase, and match the codec area, such as `lossless`, `baseline`, `htj2k`, or `codestream`. Exported identifiers need clear doc comments when they are part of the public API. Prefer table-driven tests for codec matrices and keep transfer syntax UIDs or marker constants named descriptively.

## Testing Guidelines

Tests use Go's built-in `testing` package. Name files with `_test.go` and test functions as `TestXxx`. Add focused round-trip, boundary, malformed-input, and conformance tests for codec changes. Use `go test -run TestName ./path/...` for targeted work, then run the broader package set before submitting.

## Commit & Pull Request Guidelines

Recent history uses short conventional-style subjects, often with emoji prefixes, such as `fix: ...`, `style: ...`, `build(deps): ...`, and `ci(release): ...`. Keep commits scoped and imperative. Pull requests should describe the codec behavior changed, list verification commands run, and call out compatibility risks, especially for DICOM transfer syntax registration, lossy/lossless output, or performance-sensitive paths. Include benchmark output when touching hot encode/decode loops.
