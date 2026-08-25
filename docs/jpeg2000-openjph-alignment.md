# JPEG 2000 / HTJ2K Ownership and OpenJPH Alignment Design

## Status and authority

This is the single durable design and continuation document for JPEG 2000,
HTJ2K, fo-dicom.Codecs interoperability, and package restructuring.

The rejected 2026-08-24 profile/j2kcore design was removed from local and
remote `master`. The repository baseline for this design is:

```text
go-dicom-codecs: faf55bca3692155f1fde7e25e667d3aa0cece708
fo-dicom.Codecs: da3fe114fc918756285ce1f25be265e7b74360a3
fo-dicom.Codecs current reference version: 6.0.0-beta1
fo-dicom.Codecs accepted package range: [6.0.0-beta1, 7.0.0)
OpenJPH version: 0.30.1
```

Future work must update the execution ledger and evidence log in this file.
Do not restore `j2kcore.Profile`, six global strategy interfaces, or an
`HTJ2KMode`-selected shared pipeline.

## Decisions

1. Classic JPEG 2000 and HTJ2K use two concrete pipelines.
2. Classic `.90` and `.91` behavior is owned by OpenJPEG-aligned code.
3. HTJ2K `.201`, `.202`, and `.203` behavior is owned by OpenJPH-aligned code.
4. The two pipelines share only mechanisms whose observable semantics have
   been proven identical with focused vectors.
5. There is no compatibility layer for the old implementation packages
   `jpeg2000/t1`, `jpeg2000/t2`, `jpeg2000/mqc`, `jpeg2000/wavelet`, or
   `jpeg2000/colorspace`. After migration their old paths are deleted.
6. Public DICOM codec entry points remain under `jpeg2000/lossless`,
   `jpeg2000/lossy`, and `jpeg2000/htj2k`. The public `jpeg2000` facade remains
   the classic JPEG 2000 entry point.
7. Shared code contains no family selector, family fallback, default-policy
   choice, or profile-specific numeric behavior.
8. Go and CI never execute C#, C++, `dotnet`, or `Dicom.Native.dll`.
9. A local C# generator uses a managed fo-dicom.Codecs version in
   `[6.0.0-beta1, 7.0.0)` to save offline artifacts. The current reference is
   `6.0.0-beta1`; Go tests consume files only.

## Concrete pipelines

```text
DICOM .90/.91 adapter
  -> jpeg2000 public facade
  -> jpeg2000/openjpeg concrete engine
  -> jpeg2000/internal/common proven shared mechanisms

DICOM .201/.202/.203 adapter
  -> jpeg2000/htj2k
  -> jpeg2000/htj2k/openjph concrete engine
  -> jpeg2000/internal/common proven shared mechanisms
```

There is no common algorithm interface representing the whole pipeline. A
narrow interface may exist at a real consumer boundary such as byte input or
output, but it is defined beside that consumer and cannot select a codec
family.

## Ownership matrix

| Concern | Shared | OpenJPEG-owned | OpenJPH-owned | Rule |
|---|---|---|---|---|
| Frame, rectangle, tile data | Yes | No | No | Data only; no defaults or arithmetic |
| Marker constants and parsed models | Yes | Payload policy | Payload policy | Shared code parses/serializes bounded structures only |
| Bounded byte input/output | Yes | No | No | Must reject truncation and overflow |
| Codestream parser | Yes | Validation policy | Validation policy | Parser cannot infer the DICOM-selected family |
| Tile/resolution/subband geometry | Candidate | Until proven | Until proven | Extract only after both vector suites are exact |
| Code-block/precinct geometry | Candidate | Until proven | Until proven | No `isHTJ2K` branch in shared geometry |
| Progression and packet record types | Yes | Packet algorithm | Packet algorithm | Records may be shared; header bytes may not |
| Raw DICOM preprocessing | No | Classic adapter | fo-dicom YBR behavior | YBR fallback belongs to OpenJPH adapter |
| Sample traversal | No | Interleaved OpenJPEG input | OpenJPH line exchange | 16-bit and multi-component traversal remain separate |
| DC level shift | No | Concrete implementation | Concrete implementation | Operation order and representation are observable |
| RCT | Candidate | Until proven | Until proven | Share only after signed/odd-size exact vectors |
| ICT | No | OpenJPEG float path | OpenJPH float path | Coefficients, operation order, and rounding differ |
| Reversible 5/3 DWT | Candidate | Until proven | Until proven | Share only after parity-origin vectors match |
| Irreversible 9/7 DWT | No | OpenJPEG implementation | OpenJPH implementation | Numeric paths remain separate |
| QCD parsed/serialized model | Yes | Values and defaults | Values, MAGB, and Kmax | Common writer does not calculate steps |
| Quantization | No | OpenJPEG steps/rates/PCRD | OpenJPH `param_qcd` | Never coexist in one source file |
| Tier-1 | No | EBCOT and MQC | HT cleanup | These are different coding systems |
| Tier-2 | Record types only | Tag-tree/pass contribution | OpenJPH precinct preparation | Header/body construction remains separate |
| Tile-part bounded serialization | Yes | Planning policy | Resolution divisions and TLM | `Psot` writing may share; planning may not |
| CAP/MAGB/TLM | Structure only | Absent | Payload and ordering | OpenJPH-only policy |
| COM | Structure only | OpenJPEG `2.5.4` | OpenJPH `0.30.1` | Text and placement are private |
| Error values | Yes | Trigger policy | Trigger policy | No shared fallback to the other family |

Candidate-shared code remains duplicated until evidence proves equivalence.
Avoiding premature sharing is preferred to introducing family switches.

## Target package layout

```text
jpeg2000/
  encoder.go                         classic public facade
  decoder.go                         classic public facade
  parameters.go
  lossless/
  lossy/
  internal/common/
    frame.go
    geometry.go
    packet.go
    errors.go
    codestream/
      markers.go
      models.go
      parser.go
      bounded_reader.go
      bounded_writer.go
      tilepart_writer.go
  openjpeg/
    engine.go
    sample.go
    levelshift.go
    quantization.go
    codestream.go
    colorspace/
      rct.go
      ict.go
    wavelet/
      dwt53.go
      dwt97.go
    mqc/
    t1/
    t2/
  htj2k/
    codec.go
    parameters.go
    native_adapter.go
    openjph/
      engine.go
      sample.go
      levelshift.go
      quantization.go
      codestream.go
      decode.go
      colorspace/
        rct.go
        ict.go
      wavelet/
        dwt53.go
        dwt97.go
      cleanup/
        mel.go
        vlc.go
        uvlc.go
        magsgn.go
      t2/
        precinct.go
        packet_header.go
```

Required dependency direction:

```text
jpeg2000 facade -> openjpeg -> internal/common
htj2k adapter   -> openjph  -> internal/common
```

Forbidden dependencies:

- `internal/common` importing `openjpeg`, `htj2k`, or `openjph`;
- `openjpeg` importing `htj2k` or `openjph`;
- `openjph` importing `openjpeg`;
- public/shared orchestration selecting behavior using `HTJ2KMode`,
  `isHTJ2K`, a transfer syntax UID, or marker heuristics.

## OpenJPH reference behavior

The authoritative adapter is:

```text
D:\Code\dotnet-source\fo-dicom.Codecs\Codec\DicomHtJpeg2000Codec.cs
```

The authoritative OpenJPH wrapper is:

```text
D:\Code\dotnet-source\fo-dicom.Codecs\Native\Common\OpenJPH\interface\ojph_interface.cpp
```

Required wrapper behavior includes:

- YBR FULL and YBR FULL 422 conversion with the C# adapter's exception
  fallback;
- `BitsAllocated`, component count, signedness, and multi-component color
  transform mapping;
- five decomposition levels and 64x64 code-blocks;
- OpenJPH default progression when fo-dicom passes `PROG_UNKNOWN` and the
  explicit `.202` progression parameter;
- TLM enabled and tile-parts divided by resolution;
- planar mode selected from color-transform use;
- the exact 8-bit and 16-bit component-line traversal in the wrapper;
- encode failure when the codestream is not smaller than the source buffer;
- resilient OpenJPH decode and exact output traversal.

Alignment must also inspect the corresponding OpenJPH `0.30.1` source for
color transforms, DWT, QCD/MAGB/CAP, cleanup, precincts, bit stuffing,
progression, and tile-parts. Source-derived behavior is implemented in Go;
C++ is not compiled or invoked by this repository.

## Offline fo-dicom.Codecs artifact flow

The local-only generator is C#:

```text
tools/fo-dicom-reference-generator/
  Program.cs
  fo-dicom-reference-generator.csproj
```

It must:

1. reference the managed fo-dicom.Codecs NuGet package in
   `[6.0.0-beta1, 7.0.0)`; never use a local project or directory reference;
2. use fo-dicom's public managed codec/transcoder API;
3. contain no direct P/Invoke and accept no Native DLL path;
4. reject a loaded managed version outside `[6.0.0-beta1, 7.0.0)`;
5. encode/decode every frame and save all results;
6. generate a complete SHA256 manifest;
7. never be called from Go or CI.

fo-dicom.Codecs may internally load its own implementation. This repository's
generator does not directly call, copy, link, distribute, or validate a Native
DLL.

The committed bundle layout is:

```text
test-data/htj2k/interop-v1/
  manifest.json
  sources/*.dcm
  sources/frames/*.raw
  fo-encoded/*.dcm
  fo-encoded/frames/*.j2c
  go-encoded/*.dcm
  go-encoded/frames/*.j2c
  decoded/go-from-go/*.raw
  decoded/go-from-fo/*.raw
  decoded/fo-from-go/*.raw
  decoded/fo-from-fo/*.raw
```

The manifest records the exact resolved package version, fo-dicom.Codecs
source commit, transfer syntax, frame metadata, and
every whole-file and per-frame hash. The version range controls compatibility;
the exact manifest version preserves reproducibility for each generated
bundle.

## Fixture matrix and acceptance

Every `.201`, `.202`, and `.203` syntax covers:

- unsigned mono 8-bit;
- unsigned mono 16-bit;
- signed mono 16-bit;
- RGB 8-bit and 16-bit;
- YBR FULL and YBR FULL 422;
- odd dimensions;
- multiple frames.

The four directions are:

1. Go encode -> Go decode, executed in CI;
2. fo-dicom encode -> Go decode, using offline fo-dicom frames in CI;
3. Go encode -> fo-dicom decode, checked from offline C# outputs;
4. fo-dicom encode -> fo-dicom decode, checked from offline C# outputs.

Lossless `.201/.202` acceptance requires exact encoded frame bytes and exact
source raw reconstruction. Lossy `.203` acceptance requires exact encoded
frame bytes and exact equality between Go-decoded and fo-dicom-decoded raw
frames; lossy output is not required to equal the source.

All frames, fragment ordering, transfer syntax, encapsulation, and per-frame
hashes are verified. Frame zero alone is never sufficient.

CI contains no C# execution. It validates the manifest and artifacts with Go,
parses saved codestreams, and compares marker, tile-part, packet, cleanup, and
decoded-pixel evidence.

## Migration rules

1. Freeze exact classic and current HTJ2K bytes before moving code.
2. Add dependency guards before creating new packages.
3. Extract only proven shared structures and bounded I/O first.
4. Move OpenJPEG implementation packages without behavior changes.
5. Build an independent OpenJPH pipeline one observable stage at a time.
6. Remove each root `HTJ2KMode` branch only after its OpenJPH replacement has
   focused tests and the frozen baseline remains green.
7. Generate the offline bundle locally after the structural split and
   use it for source-alignment RED tests.
8. Complete exact `.201/.202/.203` alignment before deleting the final legacy
   branch and old implementation path.
9. Do not retain compatibility wrappers for deleted implementation packages.

## Durable implementation sequence

This section is the implementation plan. It is intentionally maintained in
this durable document instead of a dated session plan. A future session starts
by reading the execution ledger, then resumes the first incomplete phase below.
When a phase completes, update both its ledger status and the evidence log in
the same change. Do not create another JPEG 2000/OpenJPH plan under
`docs/superpowers/plans`.

All behavioral and ownership changes use RED-GREEN tests. Structural phases
must preserve the frozen classic and pre-alignment HTJ2K bytes until a later
alignment phase explicitly replaces the HTJ2K expectation with reference
fo-dicom.Codecs evidence from the accepted version range.

Progress is tracked only in the execution ledger below. `In progress` means
the phase has a current RED/GREEN checkpoint under active development;
`Complete` requires the phase completion gate and a dated evidence entry;
`Blocked` records a reproducible external blocker without silently advancing
to a dependent phase.

Current checkpoint: A1-A13 are complete. The reopened A13 review findings and
all final quality/performance gates are closed with fresh evidence.

### Progress snapshot

Last updated: 2026-08-25

| Scope | Complete | Total | Current state |
|---|---:|---:|---|
| Architecture and interoperability design | 4 | 4 | Complete |
| Rollback and clean baseline (A0) | 1 | 1 | Complete |
| Implementation phases (A1-A13) | 13 | 13 | Complete |
| Active phase | 0 | 0 | None |

Completed design work:

- [x] Define separate concrete OpenJPEG and OpenJPH pipelines.
- [x] Classify shared, candidate-shared, and family-owned behavior.
- [x] Define the `[6.0.0-beta1, 7.0.0)` fo-dicom.Codecs offline artifact flow.
- [x] Define package migration order, interoperability matrix, and quality
  gates.

Next action: preserve the completed architecture, offline interoperability, and
Go-only hosted workflow gates for future JPEG 2000 and HTJ2K changes.

### Cross-session progress protocol

At the start of every new session:

1. Read the progress snapshot, execution ledger, current phase checklist, and
   latest evidence-log entry.
2. Resume the first unchecked item of the single `In progress` phase. If no
   phase is active, start the `Next action` recorded above.
3. Change a phase from `Not started` to `In progress` before its first code or
   test change. Keep every dependent phase `Not started`.
4. Check an item only after its stated result exists and its focused command
   has been run. Append the command and outcome to the evidence log.
5. Mark a phase `Complete` only after every item is checked and its completion
   gate passes. Then update the progress snapshot and `Next action`.
6. If work stops mid-phase, leave it `In progress` and record the exact next
   unchecked item, current failure, changed files, and last passing command in
   the evidence log.
7. Use `Blocked` only for a reproducible external blocker. Record the blocker,
   attempted commands, and the condition required to resume.

The checklist and ledger in this document are authoritative. Commit history,
chat history, or the presence of partially written files does not by itself
prove completion.

### A0 - Roll back the rejected design and establish the baseline

- [x] Reset local `master` from rejected commit `20744eb` to `faf55bc`.
- [x] Update remote `origin/master` with protected `--force-with-lease`.
- [x] Remove the six rejected commits and related uncommitted implementation.
- [x] Confirm local `master` and `origin/master` resolve to `faf55bc`.
- [x] Run `go test -count=1 ./...` successfully at the rollback baseline.
- [x] Record rollback and reference identities in the evidence log.

### A1 - Freeze exact pre-migration bytes

- [x] Add deterministic classic mono-u16 lossless and RGB-u8 lossy fixtures under
  `jpeg2000/testdata/baseline/`.
- [x] Add deterministic `.201`, `.202`, and `.203` fixtures under
  `jpeg2000/htj2k/testdata/baseline/`.
- [x] Require full-byte equality and literal SHA256 values in characterization
  tests. Name HTJ2K tests as the current Go baseline, not fo-dicom parity.
- [x] Generate the files only from unchanged baseline `faf55bc`; record lengths
  and hashes in the evidence log.
- [x] Completion gate: focused baseline tests, `go test -count=1 ./jpeg2000/...`,
  and `git diff --check` pass.

### A2 - Enforce ownership and dependency direction

- [x] Add AST-based tests that inspect non-test Go imports and identifier use with
  file and line evidence. Do not use source-text grep as the guard itself.
- [x] Enforce the required dependency direction and forbid family selectors in
  root/shared orchestration.
- [x] While migration is incomplete, keep an exact inventory of current
  violations. Any new or unrecorded violation fails; each later phase removes
  its own inventory entries, and A12 removes the migration exemption.
- [x] Completion gate: architecture tests record the exact starting violations and
  all JPEG 2000 tests pass.

### A3 - Extract only proven common mechanisms

- [x] Move marker constants, parsed models, parser, bounded reader/writer, and
  bounded tile-part serialization to `jpeg2000/internal/common/codestream`.
- [x] Move frame/rectangle/packet record data only when it contains no policy.
- [x] Characterize even and odd origins, partial tiles, clipped code-blocks,
  multiple resolution levels, truncation, overflow, and exact marker bytes.
- [x] Keep geometry that differs between the two families private; do not
  normalize a difference merely to make it shareable.
- [x] Delete replaced old common paths without forwarding packages.
- [x] Completion gate: common focused tests plus all frozen-byte tests pass.

### A4 - Establish the concrete OpenJPEG engine

- [x] Move MQC and EBCOT Tier-1 to `jpeg2000/openjpeg/mqc` and
  `jpeg2000/openjpeg/t1` without arithmetic or pass-order changes.
- [x] Split OpenJPEG tag-tree/pass-contribution Tier-2 into
  `jpeg2000/openjpeg/t2`; shared code may retain neutral records only.
- [x] Move OpenJPEG color transforms, DWT, quantization, rate control, marker
  policy, and codestream orchestration under `jpeg2000/openjpeg`.
- [x] Make the classic public facade and `.90/.91` codecs map parameters directly
  into this concrete engine. It must not accept HT cleanup, CAP, or TLM policy.
- [x] Delete old `mqc`, `t1`, `t2`, `colorspace`, and `wavelet` package paths as
  their consumers migrate; add no aliases or compatibility wrappers.
- [x] Completion gate: MQC/T1/T2/color/wavelet focused suites and exact classic and
  pre-alignment HTJ2K baselines pass after each move.

### A5 - Establish the independent OpenJPH engine and adapter

- [x] Create `jpeg2000/htj2k/openjph` with its own config, engine, sample traversal,
  codestream policy, cleanup coding, packet path, and decoder boundary.
- [x] Switch `jpeg2000/htj2k` directly to this engine; it must not construct the
  classic `jpeg2000` encoder or import OpenJPEG code.
- [x] Move existing HT cleanup, MEL, VLC, UVLC, and MagSgn code under OpenJPH
  ownership mechanically before changing observable bytes.
- [x] Implement the fo-dicom adapter's YBR FULL/YBR FULL 422 conversion and
  exception fallback, bit allocation, component count, signedness, transform
  mapping, and exact 8/16-bit component-line traversal.
- [x] Completion gate: the concrete dependency guard passes and `.201/.202/.203`
  still match the frozen pre-alignment Go baseline.

### A6 - Align OpenJPH color and wavelet arithmetic

- [x] Implement OpenJPH-owned RCT, ICT, reversible 5/3, and irreversible 9/7 from
  OpenJPH `0.30.1` operation order and boundary behavior.
- [x] Test signed and unsigned level shift, odd origins/dimensions, boundaries,
  float operation order, and final rounding with hand-derived literals.
- [x] Promote RCT or 5/3 into common only if the complete OpenJPEG and OpenJPH
  vector suites prove exact identity; otherwise keep both implementations.
- [x] Completion gate: focused arithmetic vectors pass and divergence against the
  saved fo-dicom frames is localized to later codestream stages.

### A7 - Align OpenJPH quantization, headers, and cleanup coding

- [x] Implement OpenJPH QCD state, MAGB/Kmax, CAP payload, marker order, and COM
  behavior without reusing OpenJPEG quantization policy.
- [x] Align cleanup precision and exact MEL/VLC/UVLC/MagSgn bytes for `.201`,
  `.202`, and `.203` while leaving OpenJPEG MQC/T1 untouched.
- [x] Derive expected marker and cleanup literals from committed offline reference
  frames and record their hashes in the manifest/evidence log.
- [x] Completion gate: marker and code-block byte parity passes across the complete
  reference fixture matrix.

### A8 - Align OpenJPH packets, TLM, and tile-parts

- [x] Implement OpenJPH precinct preparation, packet header/body construction,
  inclusion and missing-MSB tag trees, pass lengths, and `0xFF` bit stuffing.
- [x] Implement default progression when fo-dicom supplies `PROG_UNKNOWN`, explicit
  `.202` progression, TLM entries, `Psot`, and tile-parts divided by resolution.
- [x] Keep these algorithms separate from OpenJPEG Tier-2.
- [x] Completion gate: every committed `.201/.202/.203` frame matches the
  codestream generated by the manifest-recorded fo-dicom.Codecs reference
  version byte for byte.

### A9 - Align the independent OpenJPH decoder

- [x] Implement cleanup decode, dequantization, inverse DWT, inverse color
  transform, signedness handling, and exact output traversal in the concrete
  OpenJPH decoder.
- [x] Select the decoder from the DICOM transfer syntax adapter, never from marker
  heuristics or a shared family switch.
- [x] Completion gate: Go decodes all fo-dicom frames; lossless frames equal source
  raw data except the manifest-proven Native YBR conversion and 16-bit
  multi-component traversal cases, while every frame equals fo-dicom-decoded raw
  data. Go encode/decode also passes for every fixture frame.

### A10 - Generate versioned offline interoperability artifacts

- [x] Add the local-only C# generator at `tools/fo-dicom-reference-generator` and a
  pure-Go manifest validator under `cmd/dicom-interop-validation`.
- [x] Use the public managed fo-dicom.Codecs API; accept only
  `[6.0.0-beta1, 7.0.0)`, record the exact resolved version, and expose no
  direct P/Invoke or Native DLL option.
- [x] Make the validator reject path escapes, absolute paths, duplicates,
  missing frames/directions, bad SHA256 values, and versions outside the
  accepted range. It must work with an empty `PATH` and not import `os/exec`.
- [x] Generate every matrix case and all frames locally. Commit DICOM, `.j2c`, raw,
  schema, and manifest artifacts only; never commit build output or DLLs.
- [x] Completion gate: the pure-Go validator verifies all artifact hashes and
  provenance without executing C#, C++, dotnet, or a native library.

### A11 - Prove four-way interoperability

- [x] Execute and record Go encode -> Go decode in Go tests.
- [x] Consume saved fo-dicom encode -> Go decode frames in Go tests.
- [x] Consume locally saved Go encode -> fo-dicom decode and fo-dicom encode ->
  fo-dicom decode results from the artifact bundle.
- [x] Verify every frame, fragment order, transfer syntax, encapsulation, marker,
  tile-part, packet, cleanup block, compressed hash, and decoded raw hash.
- [x] Completion gate: all fixture cases satisfy the lossless/lossy acceptance
  rules and exact compressed-byte requirements stated above.

### A12 - Delete the mixed legacy implementation

- [x] Remove all remaining `HTJ2KMode`, `isHTJ2K`, marker-family inference, block
  factories, family fallbacks, and mixed OpenJPH helpers from classic files.
- [x] Remove all OpenJPEG helpers from HTJ2K files and enable every final
  architecture guard without a migration exemption.
- [x] Delete any remaining flat legacy implementation directories. No deprecated
  forwarding API is retained because compatibility is not a requirement.
- [x] Completion gate: final dependency guards, exact-byte tests, interoperability
  tests, and `go test -count=1 ./jpeg2000/...` pass.

### A13 - Run final quality and performance gates

- [x] Close the independent review findings for typed-nil parameters, manifest
  path containment, provenance commit binding, and Go-only command/validator CI.
- [x] Run fresh functional, race/coverage, vet, lint, and whitespace commands
  locally with Go 1.27 and task-specific Go 1.27 caches. The module and hosted
  workflows remain on Go 1.25.
- [x] Benchmark classic and HTJ2K encode/decode for lossless/lossy, mono/RGB, and
  8/16-bit inputs with timed repeated samples. Investigate material regressions
  without weakening parity.
- [x] Record command outcomes, fixture provenance, pass/fail counts, and stable
  benchmark results in the evidence log. Completion requires fresh recorded
  evidence; skipped or environment-blocked gates remain explicitly incomplete.

## Verification gates

Every task runs its focused RED/GREEN test plus:

```powershell
go test -count=1 ./jpeg2000/...
go test -count=1 ./cmd/dicom-interop-validation
go test -count=1 ./...
go vet ./codec/... ./cmd/... ./jpeg/... ./jpeg2000/... ./jpegls/... ./examples/...
golangci-lint run --timeout=10m ./codec/... ./cmd/... ./jpeg/... ./jpeg2000/... ./jpegls/... ./examples/...
go run ./cmd/dicom-interop-validation --bundle test-data/htj2k/interop-v1
git diff --check
```

Final verification also runs the repository race/coverage command and the
classic/HTJ2K benchmark matrix.

## Execution ledger

| ID | Deliverable | Items | Status | Evidence | Next action |
|---|---|---:|---|---|---|
| A0 | Remove rejected history and verify baseline | 6/6 | Complete | 2026-08-24 log | None |
| A1 | Freeze classic and current HTJ2K exact bytes | 5/5 | Complete | 2026-08-25 A1 log | None |
| A2 | Add dependency and ownership guards | 4/4 | Complete | 2026-08-25 A2 log | None |
| A3 | Extract proven common parser, I/O, models, and geometry | 6/6 | Complete | 2026-08-25 A3 log | None |
| A4 | Establish the concrete OpenJPEG engine | 6/6 | Complete | 2026-08-25 A4 completion log | None |
| A5 | Establish the independent OpenJPH engine and adapter | 5/5 | Complete | 2026-08-25 A5 completion log | None |
| A6 | Align OpenJPH color and wavelet arithmetic | 4/4 | Complete | 2026-08-25 A6 completion log | None |
| A7 | Align QCD, MAGB, CAP, and cleanup coding | 4/4 | Complete | 2026-08-25 A7 full-matrix completion log | None |
| A8 | Align precincts, packets, TLM, and tile-parts | 4/4 | Complete | 2026-08-25 A8 completion log | None |
| A9 | Align decoder and output traversal | 3/3 | Complete | 2026-08-25 decoder completion log | None |
| A10 | Build the ranged-version C# generator and offline bundle | 5/5 | Complete | 2026-08-25 A10 completion log | None |
| A11 | Prove exact bytes and four-way interoperability | 5/5 | Complete | 2026-08-25 A11 completion log | None |
| A12 | Delete mixed branches and old implementation paths | 4/4 | Complete | 2026-08-25 A12 completion log | None |
| A13 | Run final quality and benchmark gates | 4/4 | Complete | 2026-08-25 final review-remediation log | None |

## Evidence log

### 2026-08-25 - A13 review remediation and final gates complete

- Typed-nil `*htj2k.Parameters` now uses default parameters in both encode and
  decode. The focused regression performs a real lossless encode/decode and
  verifies exact reconstructed pixels.
- The Go artifact generator rejects empty, absolute, volume-qualified,
  backslash, non-canonical, and traversal-controlled manifest paths. Fixture
  and syntax names must be single safe segments. Actual source/fo DICOM reads,
  generated DICOM/frame writes, and manifest reads/writes use `os.Root`, so an
  in-bundle symlink cannot redirect access outside the bundle. Symlink tests
  are active on Linux; this Windows host skipped them because it lacks symlink
  privilege, while all lexical and generator-entry containment tests passed.
- The pure-Go validator requires the `managedVersion` `+<40-lowerhex>` metadata
  to equal `sourceCommit`; accepted NuGet range membership alone is not treated
  as reproducible source provenance. The committed bundle remains valid.
- CI, release, and CodeQL include `./cmd/...`; CI and release explicitly run the
  committed bundle validator. Hosted workflows execute only Go and remain on
  Go `1.25`, matching `go.mod` (`go 1.25.0`). They do not execute C#, dotnet,
  C++, DLLs, or the local-only reference generator.
- Fresh local verification used installed `go1.27.0`, `GOTOOLCHAIN=local`,
  `.codex-go-cache/a13-go127`, and `.codex-go-cache/a13-lint-go127`.
  `go test -count=1 ./...`, race/short/coverage including `./cmd/...`, `go vet`,
  the pure-Go bundle validator, and `git diff --check` passed. Full
  golangci-lint `v2.13.1` reported `0 issues`; whitespace output contained only
  Windows LF-to-CRLF warnings.
- The 40-case benchmark matrix ran with `-benchtime=200ms -count=3` on
  Windows/amd64, Intel Core Ultra 9 185H. Throughput ranges were classic
  lossless encode/decode `2.08-3.73 / 4.10-15.13 MB/s`, classic lossy
  `1.70-3.82 / 10.67-18.68 MB/s`, and HTJ2K `.201/.202/.203`
  `18.13-80.98 / 4.61-26.96 MB/s`. All timed samples passed.
- A final independent scoped review found no remaining Critical or Important
  findings. A13 is complete at 4/4; A1-A13 are complete at 13/13.

### 2026-08-25 - A13 reopened by final independent review

- The final independent review found four unresolved issues: HTJ2K decode
  panics for typed-nil `*Parameters`; the Go artifact generator permits
  manifest-controlled paths and names to escape the bundle; provenance does
  not bind `managedVersion` build metadata to `sourceCommit`; and hosted
  workflows omit `./cmd/...` plus the pure-Go bundle validator.
- A13 is reopened at 0/4. A1-A12 remain complete. The exact next action is to
  add and run focused RED tests for the three runtime/validation findings.
- Local verification commands use the installed `go1.27.0`,
  `GOTOOLCHAIN=local`, and repository caches `.codex-go-cache/a13-go127` and
  `.codex-go-cache/a13-lint-go127`. The module language baseline and GitHub
  Actions toolchain remain Go `1.25`.
- The earlier `-benchtime=1x` matrix is retained only as a smoke run. A13 needs
  a timed, repeated benchmark before completion.

### 2026-08-25 - Initial A13 quality and performance checkpoint (superseded)

- Local final verification uses the installed Go `1.27` and its dedicated
  repository caches. GitHub CI, release, CodeQL, and the `go.mod` language
  baseline remain on Go `1.25`. golangci-lint `v2.13.1` uses `goinstall` in
  hosted CI so it is built by the workflow's selected Go toolchain.
- Fresh Go 1.27 `go test -count=1 ./...` passed. The required
  `-race -short -timeout 30m -coverprofile=coverage.out -covermode=atomic`
  command passed across codec, JPEG, JPEG 2000, JPEG-LS, and examples packages
  using `.codex-go-cache/a13-go127`.
- Fresh Go 1.27 `go vet` passed. Go 1.27-built golangci-lint reported
  `0 issues`. `git diff --check` returned exit 0; its only output was the
  repository's existing Windows LF-to-CRLF conversion warning.
- The final pure-Go validator accepted `test-data/htj2k/interop-v1`, including
  all 306 declared artifacts, the NuGet-only generator project contract, and
  `fo-dicom.Codecs` provenance
  `6.0.0+fc2df0efaa9acdee7b3640f821665107630933e8`.
- Added and ran a 40-case Go 1.27 benchmark smoke matrix with `-benchtime=1x` on
  Windows/amd64, Intel Core Ultra 9 185H: classic lossless and lossy each ran
  encode/decode for Mono8, Mono16, RGB8, and RGB16; HTJ2K lossless, lossless
  RPCL, and lossy ran the same eight directions. All cases passed. This is the
  first complete matrix smoke run, so no stable performance baseline or unsupported historical performance
  comparison is claimed; exact-byte and decoded-artifact gates were unchanged.
- Observed throughput ranges were classic lossless encode `2.25-4.61 MB/s`
  and decode `7.90-15.00 MB/s`, classic lossy encode `1.72-3.64 MB/s` and
  decode `9.98-20.18 MB/s`, and HTJ2K encode `17.32-66.52 MB/s` and decode
  `4.61-26.03 MB/s`.
- The local-only NuGet generator tests passed `24/24`. GitHub workflow scans
  found no C#, dotnet, native worker, DLL, or generator execution; hosted CI
  remains Go-only and consumes only committed offline artifacts.
- A13 is complete at 4/4. A1-A13 are complete at 13/13.

### 2026-08-25 - A12 legacy removal complete; A13 started

- Replaced the empty migration whitelist with the permanent
  `TestJPEG2000ArchitectureHasNoFamilyViolations` zero-violation guard. The
  final ownership, dependency-direction, and family-policy guards passed.
- Deleted the superseded schema-v1 interoperability bundle and pre-alignment
  byte fixtures/tests, all flat legacy implementation paths, and the obsolete
  `cmd/fo-dicom-native-worker` source plus its ignored build output.
- The pure-Go validator now parses the local generator project and requires one
  NuGet `fo-dicom.Codecs` `PackageReference` in `[6.0.0-beta1, 7.0.0)`. It
  rejects generator `ProjectReference` and direct DLL assembly references.
  The generator test project references only the generator project itself.
- The root README now describes the concrete OpenJPEG/OpenJPH ownership and the
  completed schema-v2 exact-byte and four-way HTJ2K evidence rather than the
  removed mixed/experimental pipeline.
- Fresh `go test -count=1 ./jpeg2000/...` and
  `go test -count=1 ./cmd/dicom-interop-go-generator
  ./cmd/dicom-interop-validation` passed. The pure-Go command also validated
  the committed `interop-v1` bundle and current generator dependency contract.
- A12 is complete at 4/4. A13 is `In progress`; its next action is the final
  functional, race/coverage, vet, lint, whitespace, and benchmark matrix.

### 2026-08-25 - A11 four-way interoperability complete; A12 started

- The local-only C# artifact generator now has a NuGet-only
  `fo-dicom.Codecs` reference in `[6.0.0-beta1, 7.0.0)`. It has no external
  project/directory reference, DLL path, direct P/Invoke, or CI integration.
- The committed bundle records NuGet `fo-dicom.Codecs`
  `6.0.0+fc2df0efaa9acdee7b3640f821665107630933e8` and source commit
  `fc2df0efaa9acdee7b3640f821665107630933e8`. It contains seven fixtures, 21
  syntax instances, 27 frame/syntax cases, and 306 declared artifacts.
- The NuGet codec reused decoded buffers across repeated multi-frame operations.
  The local generator isolates each compressed frame in a one-frame Dataset,
  transcodes through the public fo-dicom API, and copies the returned data into
  a `MemoryByteBuffer` before the next call. Six lossless multi-frame checks
  then matched their source frames exactly.
- All 27 cases have exact Go/fo compressed codestream equality and exact
  fo-from-fo, Go-from-Go, Go-from-fo, and fo-from-Go decoded raw equality. Go
  tests also verify saved DICOM transfer syntaxes, frame order and encapsulated
  frame reconstruction, markers, TLM, tile-parts, packets, and cleanup blocks.
- The pure-Go validator passed the formal `interop-v1` bundle. The focused
  accepted-range Go tests and both Go command suites passed. The local-only
  NuGet generator suite passed 24/24; GitHub CI and release workflows contain
  only Go build/test/vet/lint/benchmark steps.
- A11 is complete at 5/5. A12 is `In progress`; its first action is removal of
  the superseded A1 pre-alignment baseline and schema-v1 interop fixture/tests.

### 2026-08-25 - A11 Go artifact generation and validation checkpoint

- Added the pure-Go `cmd/dicom-interop-go-generator`. It writes Go-encoded DICOM
  and codestreams plus Go-from-Go and Go-from-fo decoded DICOM/raw artifacts;
  it never creates or substitutes fo-from-Go evidence.
- The generator preserves saved YBR encoded DICOM metadata and changes only a
  deep-cloned in-memory decode input from `YBR_FULL` or `YBR_FULL_422` to `RGB`,
  matching the existing fo-dicom.Codecs reference-generator decode workaround.
- Extended the schema-v2 pure-Go validator to require every Go artifact
  direction and validate its path, frame count, decoded length, declared hash,
  and file hash. Future fo-from-Go fields may be absent, but partial DICOM/frame
  declarations are rejected.
- The real bundle now contains seven fixtures, 21 syntax instances, and 258
  artifacts. All 21 instances contain Go encode, Go-from-Go, and Go-from-fo
  DICOM/frame directions; none claims fo-from-Go evidence.
- `go test -count=1 ./cmd/dicom-interop-go-generator
  ./cmd/dicom-interop-validation` passed. The pure-Go validator passed against
  the real bundle, and the three accepted-range byte/structure tests passed for
  all 27 fixture/frame/syntax cases.
- A11 remains `In progress` at 2/5. Its third item remains unchecked until the
  local fo-dicom.Codecs generator produces genuine `decoded/fo-from-go`
  artifacts from the saved Go-encoded DICOM files.

### 2026-08-25 - A11 Go-only two-direction checkpoint

- A fresh pure-Go command ran
  `TestAcceptedRangeNativeDecodeMatchesFoDicom`,
  `TestAcceptedRangeCleanupBytesMatchOpenJPH`, and
  `TestAcceptedRangePacketAndTilePartStructureMatchesOpenJPH` against the
  schema-v2 bundle. It passed without C#, dotnet, C++, a DLL, or an external
  process.
- Across all seven fixtures, nine frames, and `.201/.202/.203` syntaxes, Go
  encode -> Go decode matched the saved fo-decoded frame bytes, and saved
  fo-dicom encode -> Go decode matched the same saved frame bytes. The strict
  lossless source comparison retains only the previously recorded YBR and
  16-bit multi-component authority traversal exceptions.
- The same Go command compared main-header markers, TLM entries, tile-part
  order and lengths, packet coordinates/headers/bodies, inclusion state,
  cleanup contributions, and complete codestream bytes for all 27 cases.
- A11 is `In progress` at 2/5. The next action is to add and validate saved Go
  encode -> fo-dicom.Codecs decode results from the offline bundle, consume
  files only from Go, and keep Go/CI fully offline.

### 2026-08-25 - A10 offline generator and bundle validation complete; A11 started

- `cmd/dicom-interop-validation` is now a pure-Go schema-v2 bundle validator.
  The previous process orchestration, embedded codec execution, child workers,
  dotnet invocation, and `os/exec` dependency were removed from the command.
- Focused RED cases covered absolute and escaping paths, case-insensitive
  duplicate artifacts, missing source/encoded/decoded frames, missing `.201`,
  `.202`, or `.203` directions, incorrect lengths and SHA256 values, invalid
  schema/provenance/version values, incomplete fixture coverage, undeclared
  files, unreferenced digests, and empty `PATH`.
  `go test -count=1 ./cmd/dicom-interop-validation` passed after GREEN.
- The validator independently checked all 114 artifacts in
  `test-data/htj2k/interop-v1`, including safe canonical paths, exact file
  lengths and SHA256 hashes, seven fixtures, nine source frames, all 21 syntax
  instances, and the complete bit-depth/color/signedness/odd-size/multiframe
  matrix. It accepted managed version
  `6.0.0+80d8103d394aeed8ce70141c742d7d53620ef90e` within
  `[6.0.0-beta1, 7.0.0)` and source commit
  `80d8103d394aeed8ce70141c742d7d53620ef90e`.
  A compiled validator also passed against the real bundle with `PATH` empty.
- The A10 completion gate executes only Go. It does not run C#, dotnet, C++, or
  `Dicom.Native.dll`; the local generator is provenance for the already saved
  bundle and is never a Go or CI dependency. The generator exposes no native
  library command-line option and contains no direct P/Invoke.
- The bundle tree contains no DLL, executable, PDB, `bin`, or `obj` output.
  Generator build output is excluded by `.gitignore` and is not part of the
  offline bundle.
- A10 is complete at 5/5. A11 is `In progress` at 0/5. The next action is to
  record the existing Go-to-Go and saved fo-to-Go directions, then add saved
  Go-to-fo evidence while retaining exact per-frame acceptance.

### 2026-08-25 - A9 decoder and output traversal complete; A10 started

- Irreversible cleanup decode now preserves the full reconstructed 31-bit
  magnitude and bin-center low bits. Reversible cleanup retains the existing
  `31-Kmax` right shift. The subband transfer uses
  `get_irrev_delta / 2^(31-Kmax)`, matching OpenJPH `tx_from_cb32`.
- A literal cleanup RED for quantized magnitude `512`, `Kmax=22` initially failed
  because no irreversible decoder mode existed. GREEN reconstructs the OpenJPH
  bin center `768`; the QCD `0xB6EA` HL transfer remains exactly
  `0.010974055 (0x3C33CC86)`.
- Mono `.203` became byte-exact after the cleanup transfer correction. Remaining
  8-bit RGB/YBR differences localized to integer conversion before ICT.
  OpenJPH `tile::pull` performs `ict_backward` on float lines before
  `irv_convert_to_integer`; Go now preserves inverse 9/7 float32 samples through
  ICT and performs x64 Native nearest-even conversion afterward. A focused RED
  changed the old `[26 26 26]` result to `[26 25 26]`.
- `TestAcceptedRangeNativeDecodeMatchesFoDicom` now covers both Native-to-Go and
  Go-encode-to-Go-decode for all 27 fixture/syntax frame cases. Every output is
  byte-identical to the manifest-recorded fo-decoded frame. Lossless source
  equality remains strict except the previously proven Native YBR conversion and
  16-bit multi-component traversal behavior; fo-from-fo equality is never
  relaxed.
- `go test -count=1 ./jpeg2000/htj2k/openjph/...` passed all five concrete
  OpenJPH packages. The accepted-range decoder command and focused transfer
  syntax/round-trip/ownership command passed. `git diff --check` exited zero.
- The wider `go test -count=1 ./jpeg2000/htj2k/...` still fails the intentionally
  frozen A1 pre-alignment bytes and old pre-range `interop` COM bytes. It also
  exposes a typed-nil parameter panic in `TestHTJ2KNativeHeaderContract...`, to
  be resolved before the A13 full quality gate rather than folded into A9.
- A9 is complete at 3/3. A10 is `In progress` at 0/5. The next action is the
  independent generator/validator and schema-v2 bundle validation described
  above.

### 2026-08-25 - A9 Native decode RED and irreversible transfer checkpoint

- `TestAcceptedRangeNativeDecodeMatchesFoDicom` decodes every committed Native
  frame with the concrete Go OpenJPH decoder and reports the first differing
  byte with sample, component, row, and column coordinates. `.201/.202` match
  fo-from-fo for every frame. Every `.203` fixture initially differed; the
  representative mono-u16 frame 0 first sample was `Go=50`, `Native=0`.
- OpenJPH `tx_from_cb32` retains a sign-magnitude code-block word and multiplies
  it by `get_irrev_delta / 2^(31-Kmax)`. Go cleanup already right-shifts that
  word to a signed quantized integer, so its equivalent subband multiplier is
  exactly `get_irrev_delta`, including the OpenJPH LL/HL/LH/HH band scales
  `1/2/2/4` and excluding bit depth.
- The literal QCD `0xB6EA`, guard-bit, HL-band, and quantized-coefficient `12345`
  RED produced `359.59784 (0x43B3CC86)` instead of the OpenJPH float32 result
  `0.010974055 (0x3C33CC86)`. A second RED showed the old OpenJPEG finalizer
  returned `[0 0 0 0]` instead of the x64 Native result `[0 0 64 -64]` because
  it neither scaled by bit depth nor used SSE2/AVX2 nearest-even rounding.
- GREEN keeps dequantized coefficients in OpenJPH's normalized float32 domain,
  applies the band scale, runs inverse 9/7, and only then scales by bit depth
  and applies x64 Native nearest-even conversion. Both focused tests pass.
- The accepted-range rerun reduced mono-u16 frame 0 sample 0 from `Go=50` to
  `Go=1` against `Native=0`; mono/RGB residuals are now predominantly one
  sample value. Source comparison then showed cleanup reconstructs bin-center
  low bits that Go discards with the reversible `31-Kmax` shift. A9 remains
  `In progress` at 0/3. The next boundary is irreversible cleanup-to-float
  transfer, before inverse 9/7, ICT, or traversal.

### 2026-08-25 - A8 packet, TLM, tile-part, and exact-byte completion; A9 started

- `TestAcceptedRangePacketAndTilePartStructureMatchesOpenJPH` scans raw
  codestream structure without relying on the shared parser's merged tile view.
  Across all 27 schema-v2 frame/syntax cases it compares TLM entry values,
  tile-part count/order and `Isot/TPsot/TNsot/Psot`, tile-part headers, SOD
  boundaries and data, packet coordinates and raw headers, inclusion and
  missing-MSB values, pass counts/lengths, packet bodies, and finally the whole
  codestream. Every Go frame matched its Native/OpenJPH frame byte for byte.
- The first structural acceptance test was already green because the A7
  quantization and cleanup corrections also made the current accepted-range
  packet bodies, lengths, TLM entries, and tile-part bytes exact. It is retained
  as the A8 regression/completion gate rather than reported as a RED/GREEN
  production change.
- A separate adapter RED exposed the remaining fo-dicom progression contract:
  `.202` ignored an explicit LRCP parameter and wrote RPCL (`2`) instead of
  LRCP (`0`). GREEN adds the five public progression values and defaults to
  RPCL; only `.202` forwards the parameter, while `.201` and `.203` retain the
  effective OpenJPH `PROG_UNKNOWN` default RPCL behavior.
- `go test -count=1 ./jpeg2000/htj2k/openjph/...` passed all five OpenJPH-owned
  packages. Focused HTJ2K progression/parameter tests and the OpenJPEG-no-HT /
  OpenJPH-direct-cleanup ownership guards passed. `git diff --check` exited zero
  with Windows line-ending warnings only.
- A8 is complete at 4/4. A9 is `In progress` at 0/3. The exact next action is
  the accepted-range Native decode/output RED described above.

### 2026-08-25 - A7 full-matrix completion; A8 started

- The accepted-range generator now accepts repeated `--input` values in stable
  command-line order and writes one schema-v2 manifest containing all fixtures.
  A deterministic managed source writer adds unsigned mono 8-bit, three-frame
  unsigned mono 16-bit, RGB 8/16-bit, YBR FULL, and YBR FULL 422 sources. Together
  with the existing `888x459` signed mono 16-bit source, the bundle covers every
  required bit depth, color model, signedness, odd dimension, and multiframe case.
- `test-data/htj2k/interop-v1` records fo-dicom.Codecs
  `6.0.0+80d8103d394aeed8ce70141c742d7d53620ef90e`, source commit
  `80d8103d394aeed8ce70141c742d7d53620ef90e`, generator SHA256
  `4a31aa58d24fc2e2d84007929ec76e8631f882ed7915f5bc2b242a86ac2a14d5`,
  seven fixtures, nine source frames, 21 syntax instances, and 114 artifacts.
  Independent file existence, length, and SHA256 verification reported zero
  failures.
- A real Native YBR RED exposed an authority wrapper metadata defect: encode
  converts YBR raw bytes to RGB but retains the YBR DICOM photometric value, so
  fo-from-fo decode tries to convert the compressed codestream as YBR raw. The
  generator preserves the original fo-encoded DICOM and codestream bytes, then
  marks only its in-memory decode input as RGB. It also stores managed
  `PixelDataConverter` output as `encoderInputFrames` for engine byte tests.
- Full-matrix cleanup RED localized the remaining Go difference to adapter color
  conversion. fo-dicom evaluates each complete double expression with `+0.5`
  before one truncation; go-dicom v0.7.0 truncated chroma terms separately. The
  literal YBR FULL `(32,112,144)` case changed from Go `[54,27,5]` to Native
  `[54,26,4]`; YBR FULL 422 `(40,42,116,140)` changed from
  `[57,37,20,59,39,22]` to `[57,36,19,59,38,21]`. Focused adapter RED/GREEN
  now passes without modifying the general go-dicom dependency.
- `TestAcceptedRangeCleanupBytesMatchOpenJPH` reads schema v2 and ran 27
  per-frame cases. Main-header marker order and raw `SIZ/CAP/COD/QCD/COM`
  segments matched, and every included cleanup contribution matched for `.201`,
  `.202`, and `.203` across all seven fixtures and nine frames.
- The generator suite passed 22/22 tests. `go test -count=1
  ./jpeg2000/htj2k/openjph/...` passed all five OpenJPH-owned packages, and
  `git diff --check` exited zero. The wider `./jpeg2000/htj2k/...` and
  `./jpeg2000/...` commands still fail only the intentionally frozen A1
  pre-alignment bytes, the old pre-range `test-data/htj2k/interop` COM bytes,
  and A9 lossy decoder error bounds; all other JPEG 2000 packages pass.
- A7 is complete at 4/4. A8 is `In progress` at 0/4. The exact next action is
  the accepted-range packet/TLM/tile-part structural RED described above.

### 2026-08-25 - A7 Native SIMD quantization and cleanup parity checkpoint

- The accepted-range RED now retains packet layer/resolution/component/precinct,
  inclusion index, band, code-block coordinates and dimensions, Kmax,
  zero-bitplanes, and pass count. The sole `.203` difference decoded to R1/HL
  block `(0,0)`, local coefficient `(19,9)`: Go `121`, Native `122`.
- Go's pre-quantization coefficient is float32 bits `0x38E37023`; HL
  `delta_inv` is `0x4E0951F0`, and their float32 product is `62463.613`
  (`0x4773FF9D`). OpenJPH's generic scalar path truncates, but the authority's
  selected x64 AVX2/SSE2 `tx_to_cb32` uses `_mm256_cvtps_epi32`/
  `_mm_cvtps_epi32`, which rounds to nearest even and produces `62464`.
- A literal RED for those two float32 inputs first failed with `62463`, then
  passed after irreversible QCD-to-code-block conversion adopted the actual
  Native SIMD rounding rule. The 9/7 transform and cleanup writer were not
  changed.
- `TestAcceptedRangeCleanupBytesMatchOpenJPH` compares every included cleanup
  contribution for `.201`, `.202`, and `.203`; all three passed for the current
  `888x459`, signed 16-bit, single-frame MONOCHROME2 authority fixture.
  `go test -count=1 ./jpeg2000/htj2k/openjph/...` passed all five OpenJPH-owned
  packages with the task-local `GOCACHE`.
- A7 is `In progress` at 3/4. The completion gate remains unchecked because the
  committed bundle does not yet cover the full fixture matrix required above.
  Next action is to generate the missing accepted-range source cases and run
  marker/cleanup parity for every frame and syntax.

### 2026-08-25 - A7 accepted-range generator, bundle, and cleanup RED checkpoint

- The local managed generator under `tools/fo-dicom-reference-generator` uses
  fo-dicom's public managed APIs and `NativeTranscoderManager`, rejects versions
  outside `[6.0.0-beta1, 7.0.0)`, and contains no Native DLL CLI option or direct
  P/Invoke. Its focused suite passed 17/17 tests, including real Native
  encode/decode of `sample-01.dcm`.
- `test-data/htj2k/interop-v1` was generated from authority commit
  `80d8103d394aeed8ce70141c742d7d53620ef90e`, managed informational version
  `6.0.0+80d8103d394aeed8ce70141c742d7d53620ef90e`, and OpenJPH `0.30.1`.
  Independent manifest verification reported 14 artifacts and zero failures.
- The one-frame, signed 16-bit MONOCHROME2 fixture is `888x459`. Native `.201`
  and `.202` frames are each 185351 bytes with SHA256
  `2c5722a3821073fa803f46f29f2497f63415a3c75db42a1afd7ce7a44c292faa`;
  `.203` is 153093 bytes with SHA256
  `2d8b6a29fe97478d3bfc8e6d23d4cfa703cede50d3fda4167df5dfdba74caeae`.
- The accepted-range `.203` cleanup RED parses both codestreams and compares
  every included code-block. Block 0 matches exactly. Block 1 has equal length
  346, equal `Scup=88`, and identical MEL/VLC suffix; its only difference is
  MagSgn byte 139 (`Go=0x80`, `Native=0x90`). Go SHA256 is
  `684f1509300dfef4da86cce93fa3b74f74d3a93e99ded5daa3bc95aa985c6db4`;
  Native SHA256 is
  `eefc318728200fc23b18520b73c47a61e3bf337f8f61a365fde22fac0650d21a`.
- This checkpoint satisfies the A7 reference prerequisite but not an A7 or A10
  checklist item. A7 remains `In progress` at 1/4. Next action is to attach
  packet/band geometry to block 1, decode the differing coefficient, and trace
  its pre-cleanup value back through irreversible 9/7 and QCD multiplication.

### 2026-08-25 - A7 OpenJPH quantization, Kmax/MAGB, CAP, and COM checkpoint

- RED/GREEN vectors derived from OpenJPH `ojph_params.cpp`,
  `ojph_subband.cpp`, `ojph_codestream_gen.cpp`, and
  `ojph_codeblock_fun.cpp` cover QCD-decoded float32 `delta`/`delta_inv`,
  `Kmax`, truncation, sign-magnitude conversion, and OR-reduced magnitude.
  The Native `.203` first-step vector remains literal: QCD `0xB718`,
  `Kmax=22`, `delta=0x30718000`, and `delta_inv=0x4E87AF70`.
- Irreversible quantization now consumes normalized float32 coefficients,
  selects each LL/HL/LH/HH encoded QCD step and band gain, rounds the float32
  product with the authority's Native SIMD nearest-even rule, and passes the
  quantized magnitude to cleanup without the reversible `31-Kmax` left shift.
  The lossless default retains that shift.
- The no-decomposition RED changed `[-0.5, 0.5]` from `[0, 0]` to
  `[-1073741824, 1073741824]`. The lossy `.203` codestream grew from the A6
  255-byte empty-cleanup result to 2038 bytes; the frozen pre-alignment Go
  baseline is 2102 bytes and is not treated as a fo-dicom reference.
- CAP now derives `MAGB` from QCD exactly as OpenJPH `param_cap` does. A lossy
  16-bit RGB RED exposed the old component-count guess as `0x002B`; GREEN writes
  the OpenJPH value `0x002A`. Version COM changed from stale `0.21.2` to the
  authoritative OpenJPH `0.30.1`, and the main-header marker order is frozen as
  `SOC,SIZ,CAP,COD,QCD,COM,TLM,SOT`.
- `go test -count=1 ./jpeg2000/htj2k/openjph/...` passed. Focused QCD/CAP/COM/
  marker-order tests and existing `.201/.202` fo-dicom exact-byte tests passed.
  `go test -count=1 ./jpeg2000/htj2k/...` now fails only the intentionally stale
  `.203` Go baseline and lossy round-trip error bounds; irreversible decoder
  dequantization remains assigned to A9.
- The live authority checkout is clean at
  `80d8103d394aeed8ce70141c742d7d53620ef90e`, reports managed version `6.0.0`,
  and contains OpenJPH `0.30.1`. This is within the approved
  `[6.0.0-beta1, 7.0.0)` range, but differs from the design-time commit recorded
  above; the generated manifest must record the actually resolved version and
  commit.
- A7 is `In progress` at 1/4. The exact next action is to implement the local
  managed reference-generator prerequisite and generate accepted-range `.203`
  cleanup/codestream artifacts. The existing `5.16.5.1` lossless-only manifest
  cannot satisfy the A7 artifact or completion gates.

### 2026-08-25 - A6 OpenJPH color and wavelet arithmetic complete; A7 started

- Scalar sample RED/GREEN vectors now cover unsigned level shift, signed input,
  irreversible normalization by `1 << bitDepth`, x64 Native nearest-even output
  rounding, and signed/unsigned clamping.
- RCT/ICT vectors use literal integer values and float32 bit patterns from
  OpenJPH `transform/ojph_colour.cpp`; the lossy encoder normalizes every
  component before applying OpenJPH ICT.
- Reversible 5/3 and irreversible 9/7 vectors cover even and odd origin parity,
  single-sample boundaries, odd/even lengths, and a `3x5` two-dimensional case
  with both origins odd. The inverse 9/7 path uses OpenJPH `K`/`K_inv`, lifting
  order, symmetric extension, and float32 operation order; the obsolete
  OpenJPEG `BUG_WEIRD_TWO_INVK` inverse path and its self-referential tests were
  removed. RCT and 5/3 remain family-owned because complete cross-family
  bit-identity has not been established.
- `go test -count=1 ./jpeg2000/htj2k/openjph/colorspace
  ./jpeg2000/htj2k/openjph/wavelet ./jpeg2000/htj2k/openjph` passed with a
  task-specific `GOCACHE`.
- `go test -count=1 ./jpeg2000/htj2k/...` passed `.201`, `.202`, and every
  OpenJPH-owned package. Its remaining failures are confined to lossy `.203`:
  the codestream is 255 bytes instead of the frozen 2102-byte pre-alignment
  baseline and round-trip pixels collapse to unsigned midpoint 128. The new
  normalized `[-0.5, 0.5)` coefficients still enter the old OpenJPEG-scaled
  quantization/code-block transfer, localizing the divergence to A7 rather than
  color or wavelet arithmetic.
- A6 is complete at 4/4. A7 is `In progress`; next action is the normalized
  irreversible subband-to-code-block RED vector suite derived from OpenJPH
  `ojph_codeblock_fun.cpp`.

### 2026-08-25 - A5 independent OpenJPH engine and adapter complete; A6 started

- The structural RED required OpenJPH to construct cleanup coding directly.
  HT cleanup, MEL, VLC, UVLC, and MagSgn now live under
  `jpeg2000/htj2k/openjph/cleanup`; copied MQC/EBCOT packages and externally
  supplied block factories were removed.
- OpenJPH owns HT marker, quantization, packet, tile-part, dequantization, and
  decoder choices directly. `TestOpenJPHOwnsCleanupDirectly` passes and the
  OpenJPH tree imports no classic OpenJPEG engine package.
- Adapter RED/GREEN tests cover YBR FULL and YBR FULL 422 conversion, conversion
  failure fallback, BitsAllocated/component/signedness/transform/lossless
  mappings, native output-smaller-than-input failure, 8-bit pixel interleaving,
  and the fo-dicom wrapper's exact 16-bit component-line traversal.
- The output-size contract changed old small-image tests: 4x4 and 16x16 success
  expectations were replaced by a focused rejection test and compressible
  64x64 round-trip coverage.
- `go test -count=1 ./jpeg2000/htj2k/openjph/cleanup`,
  `go test -count=1 -run TestCurrentGoHTJ2KPreAlignmentBytes
  ./jpeg2000/htj2k`, `go test -count=1 ./jpeg2000/htj2k/...`, and
  `go test -count=1 ./jpeg2000/...` all passed with a task-specific `GOCACHE`.
  `git diff --check` exited 0 with Windows line-ending warnings only.
- A5 is complete at 5/5. A6 is `In progress`; next action is the scalar
  OpenJPH `ojph_colour.cpp` reversible/irreversible color RED vector suite.

### 2026-08-25 - A4 concrete OpenJPEG engine complete and A5 started

- The OpenJPEG AST guard first failed with 34 HTJ2K/OpenJPH identifiers in
  dead tile-part, quantization, precinct/tag-tree, and missing-MSB helpers.
  Those declarations and the classic facade's OpenJPH quantization forwarder
  were deleted; the guard now rejects all such identifiers.
- A second structural RED found `BlockEncoderFactory` and the unused
  `BlockDecoderFactory`. OpenJPEG now constructs its EBCOT Tier-1 encoder and
  decoder directly and exposes no cleanup, CAP, or TLM policy through the
  classic facade.
- `go test -count=1 -run
  'TestClassicJPEG2000PreMigrationBytes|TestCurrentGoHTJ2KPreAlignmentBytes'
  ./jpeg2000 ./jpeg2000/htj2k` passed.
- `go test -count=1 ./jpeg2000/openjpeg/...` passed for OpenJPEG, color, MQC,
  Tier-1, Tier-2, and wavelet packages.
- `go test -count=1 ./jpeg2000/...` passed for all 18 current JPEG 2000
  packages. `git diff --check` exited 0 with Windows line-ending warnings only.
- A4 is complete at 6/6. A5 is `In progress`; next action is the structural RED
  requiring OpenJPH to own cleanup coding directly and contain no MQC/EBCOT
  implementation or externally supplied block factories.

### 2026-08-25 - A4 concrete-engine transition checkpoint

- Classic implementation and white-box tests moved under `jpeg2000/openjpeg`;
  root `jpeg2000` now forwards its public encoder, decoder, ROI, MCT,
  quantization, rate-control, and tile APIs to that concrete engine.
- Old `jpeg2000/{mqc,t1,t2,colorspace,wavelet}` paths are absent and no
  compatibility packages were added.
- RED required the concrete OpenJPEG ownership paths and later reported 33
  HT family selectors in the classic engine. The reachable OpenJPEG encode and
  decode paths now contain no family selector and the selector guard is green.
- A mechanically independent `jpeg2000/htj2k/openjph` transition engine exists,
  imports none of `jpeg2000/openjpeg`, and `htj2k/codec.go` uses it directly.
  Current-Go `.201/.202/.203` frozen bytes remain exact after the switch.
- A4 remains `In progress`: unused OpenJPH quantization, HT tile-part, and HT
  precinct helpers still need deletion from `openjpeg`; its Tier-2 split is not
  complete. The transition OpenJPH engine still contains copied MQC/EBCOT code
  that A5 must delete after moving cleanup ownership.
- Next action: strengthen the OpenJPEG ownership guard to reject every HTJ2K or
  OpenJPH declaration, delete those dead helpers, and rerun classic frozen bytes
  plus `go test -count=1 ./jpeg2000/openjpeg/...`.

### 2026-08-25 - A3 common mechanisms complete

- The entire legacy `jpeg2000/codestream` package and its tests moved to
  `jpeg2000/internal/common/codestream`; the old path and all old imports are
  absent.
- `boundedReader` preserves offset on rejected reads and reports truncation as
  `io.ErrUnexpectedEOF`; `boundedWriter` rejects capacity overflow and short
  writes.
- `WriteTilePart` serializes bounded SOT/header/SOD/data records only. Classic
  and HTJ2K code retain their own header, packet, part-count, and TLM policy.
- RED first reported the missing common ownership files, then missing bounded
  I/O and tile-part APIs. GREEN passed after the mechanical move and extraction.
- Existing parser/geometry suites continue to cover tile sizes, multi-part
  parsing, odd/partial layouts, marker bytes, and malformed input; no differing
  color, wavelet, code-block, or precinct arithmetic moved to common.
- `go test -count=1 ./jpeg2000/...` passed for all 11 current JPEG 2000 packages.
- `git diff --check` exited 0 with only Windows LF-to-CRLF warnings.
- A3 is complete. A4 is `In progress`; next action is the OpenJPEG ownership
  structural RED test.

### 2026-08-25 - A2 architecture guard complete

- The AST guard inspects every non-test Go file and reports stable
  `kind:file:line:detail` evidence.
- RED reported 33 pre-migration violations: one HTJ2K-to-classic facade import,
  one marker-based family inference, and 31 family-selector identifier uses.
- The guard also rejects future family imports, HTJ2K UID literals, and
  `CodeBlockStyle & 0x40` marker inference in root/shared orchestration.
- GREEN passed after only the exact 33-item migration inventory was recorded.
- `go test -count=1 ./jpeg2000/...` passed for all 11 JPEG 2000 packages.
- `git diff --check` exited 0; Git emitted only its Windows LF-to-CRLF warning.
- A2 is complete. A3 is `In progress`; next action is the common codestream
  ownership RED test.

### 2026-08-25 - A1 exact pre-migration bytes complete

- Classic mono-u16 lossless: 314 bytes,
  `56da3b34eeffcc02a72caa60a931f18d1bb3f5006ec148467359ce89935ed853`.
- Classic RGB-u8 lossy: 2387 bytes,
  `e72b2141404a7603c67f4b87c835c3f74610e550a3f0173f69d25dff667aba5f`.
- Current-Go HTJ2K `.201`: 424 bytes,
  `403d88c5bf41e2ccc1624413af04fdc58f062a7a4d56d5f56b1aa186ff86a171`.
- Current-Go HTJ2K `.202`: 424 bytes,
  `403d88c5bf41e2ccc1624413af04fdc58f062a7a4d56d5f56b1aa186ff86a171`.
- Current-Go HTJ2K `.203`: 2102 bytes,
  `c56ff474fe31fc28fc31ca9de165a97ee3e6a36a689887fb534dddac0abe320c`.
- RED failed only because all five committed `.j2c` fixtures were absent.
  GREEN passed after generation from the unchanged implementation and literal
  hashes were added.
- `go test -count=1 -run
  'TestClassicJPEG2000PreMigrationBytes|TestCurrentGoHTJ2KPreAlignmentBytes'
  ./jpeg2000 ./jpeg2000/htj2k` passed.
- `go test -count=1 ./jpeg2000/...` passed for all 11 JPEG 2000 packages.
- `git diff --check` exited 0; Git emitted only its Windows LF-to-CRLF warning.
- A1 is complete. A2 is `In progress`; next action is the AST guard RED test.

### 2026-08-25 - A1 started

- A1 is `In progress`; no implementation checkpoint has been claimed yet.
- The initial `go test -count=1 ./jpeg2000/...` attempt was blocked before
  compilation by access denial in the user-wide Go build cache. Subsequent
  verification uses a task-specific writable `GOCACHE`.
- Next action: add classic exact-byte characterization tests that fail because
  the committed baseline fixtures do not yet exist.

### 2026-08-25 - durable progress and version-range checkpoint

- The dated temporary implementation plan was removed. This document is the
  only design, execution, progress, and continuation authority.
- Progress tracking now includes a summary, cross-session protocol, A0-A13
  checklists, per-phase item counts, evidence state, and exact next actions.
- A0 is complete at 6/6 items. A1-A13 have not started and contain no checked
  implementation items.
- The fo-dicom.Codecs compatibility policy is
  `[6.0.0-beta1, 7.0.0)`, with `6.0.0-beta1` as the current reference. Offline
  manifests record the actual resolved version and source commit.
- This checkpoint changes documentation only; it does not authorize or begin
  A1 implementation.

### 2026-08-24 - rollback and redesign baseline

- Local and remote `master` were reset from `20744eb864c63ab578013f24c2659981760df0f7`
  to `faf55bca3692155f1fde7e25e667d3aa0cece708` with a protected
  `--force-with-lease` update.
- The six rejected 2026-08-24 commits and all related uncommitted files were
  removed from the active branch.
- The working tree was clean after rollback.
- `go test -count=1 ./...` passed at the rollback baseline.
- The current reference project reports version `6.0.0-beta1`, source commit
  `da3fe114fc918756285ce1f25be265e7b74360a3`, and OpenJPH `0.30.1`.
- The accepted fo-dicom.Codecs range is `[6.0.0-beta1, 7.0.0)`. Each offline
  bundle records its exact resolved version and source commit for
  reproducibility.
- The approved replacement uses two concrete engines, proven common
  mechanisms, no global profile abstraction, and no old-path compatibility
  wrappers.
