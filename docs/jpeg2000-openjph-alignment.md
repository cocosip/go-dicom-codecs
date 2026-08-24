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
D:\dotnet-source-code\fo-dicom.Codecs\Codec\DicomHtJpeg2000Codec.cs
```

The authoritative OpenJPH wrapper is:

```text
D:\dotnet-source-code\fo-dicom.Codecs\Native\Common\OpenJPH\interface\ojph_interface.cpp
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

1. reference a caller-supplied fo-dicom.Codecs project or managed package in
   `[6.0.0-beta1, 7.0.0)`;
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
source commit, generator source hash, transfer syntax, frame metadata, and
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

Current checkpoint: the durable design and execution sequence are written;
rollback baseline A0 is complete; no implementation phase has started yet.

### Progress snapshot

Last updated: 2026-08-25

| Scope | Complete | Total | Current state |
|---|---:|---:|---|
| Architecture and interoperability design | 4 | 4 | Complete |
| Rollback and clean baseline (A0) | 1 | 1 | Complete |
| Implementation phases (A1-A13) | 0 | 13 | Not started |
| Active phase | 0 | 1 | None |

Completed design work:

- [x] Define separate concrete OpenJPEG and OpenJPH pipelines.
- [x] Classify shared, candidate-shared, and family-owned behavior.
- [x] Define the `[6.0.0-beta1, 7.0.0)` fo-dicom.Codecs offline artifact flow.
- [x] Define package migration order, interoperability matrix, and quality
  gates.

Next action: start A1 by adding the classic exact-byte RED characterization
test. Do not start A2 or restructure packages before A1 is complete.

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

- [ ] Add deterministic classic mono-u16 lossless and RGB-u8 lossy fixtures under
  `jpeg2000/testdata/baseline/`.
- [ ] Add deterministic `.201`, `.202`, and `.203` fixtures under
  `jpeg2000/htj2k/testdata/baseline/`.
- [ ] Require full-byte equality and literal SHA256 values in characterization
  tests. Name HTJ2K tests as the current Go baseline, not fo-dicom parity.
- [ ] Generate the files only from unchanged baseline `faf55bc`; record lengths
  and hashes in the evidence log.
- [ ] Completion gate: focused baseline tests, `go test -count=1 ./jpeg2000/...`,
  and `git diff --check` pass.

### A2 - Enforce ownership and dependency direction

- [ ] Add AST-based tests that inspect non-test Go imports and identifier use with
  file and line evidence. Do not use source-text grep as the guard itself.
- [ ] Enforce the required dependency direction and forbid family selectors in
  root/shared orchestration.
- [ ] While migration is incomplete, keep an exact inventory of current
  violations. Any new or unrecorded violation fails; each later phase removes
  its own inventory entries, and A12 removes the migration exemption.
- [ ] Completion gate: architecture tests record the exact starting violations and
  all JPEG 2000 tests pass.

### A3 - Extract only proven common mechanisms

- [ ] Move marker constants, parsed models, parser, bounded reader/writer, and
  bounded tile-part serialization to `jpeg2000/internal/common/codestream`.
- [ ] Move frame/rectangle/packet record data only when it contains no policy.
- [ ] Characterize even and odd origins, partial tiles, clipped code-blocks,
  multiple resolution levels, truncation, overflow, and exact marker bytes.
- [ ] Keep geometry that differs between the two families private; do not
  normalize a difference merely to make it shareable.
- [ ] Delete replaced old common paths without forwarding packages.
- [ ] Completion gate: common focused tests plus all frozen-byte tests pass.

### A4 - Establish the concrete OpenJPEG engine

- [ ] Move MQC and EBCOT Tier-1 to `jpeg2000/openjpeg/mqc` and
  `jpeg2000/openjpeg/t1` without arithmetic or pass-order changes.
- [ ] Split OpenJPEG tag-tree/pass-contribution Tier-2 into
  `jpeg2000/openjpeg/t2`; shared code may retain neutral records only.
- [ ] Move OpenJPEG color transforms, DWT, quantization, rate control, marker
  policy, and codestream orchestration under `jpeg2000/openjpeg`.
- [ ] Make the classic public facade and `.90/.91` codecs map parameters directly
  into this concrete engine. It must not accept HT cleanup, CAP, or TLM policy.
- [ ] Delete old `mqc`, `t1`, `t2`, `colorspace`, and `wavelet` package paths as
  their consumers migrate; add no aliases or compatibility wrappers.
- [ ] Completion gate: MQC/T1/T2/color/wavelet focused suites and exact classic and
  pre-alignment HTJ2K baselines pass after each move.

### A5 - Establish the independent OpenJPH engine and adapter

- [ ] Create `jpeg2000/htj2k/openjph` with its own config, engine, sample traversal,
  codestream policy, cleanup coding, packet path, and decoder boundary.
- [ ] Switch `jpeg2000/htj2k` directly to this engine; it must not construct the
  classic `jpeg2000` encoder or import OpenJPEG code.
- [ ] Move existing HT cleanup, MEL, VLC, UVLC, and MagSgn code under OpenJPH
  ownership mechanically before changing observable bytes.
- [ ] Implement the fo-dicom adapter's YBR FULL/YBR FULL 422 conversion and
  exception fallback, bit allocation, component count, signedness, transform
  mapping, and exact 8/16-bit component-line traversal.
- [ ] Completion gate: the concrete dependency guard passes and `.201/.202/.203`
  still match the frozen pre-alignment Go baseline.

### A6 - Align OpenJPH color and wavelet arithmetic

- [ ] Implement OpenJPH-owned RCT, ICT, reversible 5/3, and irreversible 9/7 from
  OpenJPH `0.30.1` operation order and boundary behavior.
- [ ] Test signed and unsigned level shift, odd origins/dimensions, boundaries,
  float operation order, and final rounding with hand-derived literals.
- [ ] Promote RCT or 5/3 into common only if the complete OpenJPEG and OpenJPH
  vector suites prove exact identity; otherwise keep both implementations.
- [ ] Completion gate: focused arithmetic vectors pass and divergence against the
  saved fo-dicom frames is localized to later codestream stages.

### A7 - Align OpenJPH quantization, headers, and cleanup coding

- [ ] Implement OpenJPH QCD state, MAGB/Kmax, CAP payload, marker order, and COM
  behavior without reusing OpenJPEG quantization policy.
- [ ] Align cleanup precision and exact MEL/VLC/UVLC/MagSgn bytes for `.201`,
  `.202`, and `.203` while leaving OpenJPEG MQC/T1 untouched.
- [ ] Derive expected marker and cleanup literals from committed offline reference
  frames and record their hashes in the manifest/evidence log.
- [ ] Completion gate: marker and code-block byte parity passes across the complete
  reference fixture matrix.

### A8 - Align OpenJPH packets, TLM, and tile-parts

- [ ] Implement OpenJPH precinct preparation, packet header/body construction,
  inclusion and missing-MSB tag trees, pass lengths, and `0xFF` bit stuffing.
- [ ] Implement default progression when fo-dicom supplies `PROG_UNKNOWN`, explicit
  `.202` progression, TLM entries, `Psot`, and tile-parts divided by resolution.
- [ ] Keep these algorithms separate from OpenJPEG Tier-2.
- [ ] Completion gate: every committed `.201/.202/.203` frame matches the
  codestream generated by the manifest-recorded fo-dicom.Codecs reference
  version byte for byte.

### A9 - Align the independent OpenJPH decoder

- [ ] Implement cleanup decode, dequantization, inverse DWT, inverse color
  transform, signedness handling, and exact output traversal in the concrete
  OpenJPH decoder.
- [ ] Select the decoder from the DICOM transfer syntax adapter, never from marker
  heuristics or a shared family switch.
- [ ] Completion gate: Go decodes all fo-dicom frames; lossless frames equal source
  raw data and lossy frames equal fo-dicom-decoded raw data. Go encode/decode
  also passes for every fixture frame.

### A10 - Generate versioned offline interoperability artifacts

- [ ] Add the local-only C# generator at `tools/fo-dicom-reference-generator` and a
  pure-Go manifest validator under `cmd/dicom-interop-validation`.
- [ ] Use the public managed fo-dicom.Codecs API; accept only
  `[6.0.0-beta1, 7.0.0)`, record the exact resolved version, and expose no
  direct P/Invoke or Native DLL option.
- [ ] Make the validator reject path escapes, absolute paths, duplicates,
  missing frames/directions, bad SHA256 values, and versions outside the
  accepted range. It must work with an empty `PATH` and not import `os/exec`.
- [ ] Generate every matrix case and all frames locally. Commit DICOM, `.j2c`, raw,
  schema, and manifest artifacts only; never commit build output or DLLs.
- [ ] Completion gate: the pure-Go validator verifies all artifact hashes and
  provenance without executing C#, C++, dotnet, or a native library.

### A11 - Prove four-way interoperability

- [ ] Execute and record Go encode -> Go decode in Go tests.
- [ ] Consume saved fo-dicom encode -> Go decode frames in Go tests.
- [ ] Consume locally saved Go encode -> fo-dicom decode and fo-dicom encode ->
  fo-dicom decode results from the artifact bundle.
- [ ] Verify every frame, fragment order, transfer syntax, encapsulation, marker,
  tile-part, packet, cleanup block, compressed hash, and decoded raw hash.
- [ ] Completion gate: all fixture cases satisfy the lossless/lossy acceptance
  rules and exact compressed-byte requirements stated above.

### A12 - Delete the mixed legacy implementation

- [ ] Remove all remaining `HTJ2KMode`, `isHTJ2K`, marker-family inference, block
  factories, family fallbacks, and mixed OpenJPH helpers from classic files.
- [ ] Remove all OpenJPEG helpers from HTJ2K files and enable every final
  architecture guard without a migration exemption.
- [ ] Delete any remaining flat legacy implementation directories. No deprecated
  forwarding API is retained because compatibility is not a requirement.
- [ ] Completion gate: final dependency guards, exact-byte tests, interoperability
  tests, and `go test -count=1 ./jpeg2000/...` pass.

### A13 - Run final quality and performance gates

- [ ] Run fresh functional, race/coverage, vet, lint, and whitespace commands from
  the verification section with task-specific temporary caches.
- [ ] Benchmark classic and HTJ2K encode/decode for lossless/lossy, mono/RGB, and
  8/16-bit inputs. Investigate material regressions without weakening parity.
- [ ] Record command outcomes, fixture provenance, pass/fail counts, and benchmark
  results in the evidence log.
- [ ] Completion gate: A1-A13 may be marked complete only from fresh recorded
  evidence; skipped or environment-blocked gates remain explicitly incomplete.

## Verification gates

Every task runs its focused RED/GREEN test plus:

```powershell
go test -count=1 ./jpeg2000/...
go test -count=1 ./cmd/dicom-interop-validation
go test -count=1 ./...
go vet ./codec/... ./jpeg/... ./jpeg2000/... ./jpegls/... ./examples/...
golangci-lint run --timeout=10m ./codec/... ./jpeg/... ./jpeg2000/... ./jpegls/... ./examples/...
git diff --check
```

Final verification also runs the repository race/coverage command and the
classic/HTJ2K benchmark matrix.

## Execution ledger

| ID | Deliverable | Items | Status | Evidence | Next action |
|---|---|---:|---|---|---|
| A0 | Remove rejected history and verify baseline | 6/6 | Complete | 2026-08-24 log | None |
| A1 | Freeze classic and current HTJ2K exact bytes | 0/5 | Not started | None | Add classic RED baseline test |
| A2 | Add dependency and ownership guards | 0/4 | Not started | None | Wait for A1 |
| A3 | Extract proven common parser, I/O, models, and geometry | 0/6 | Not started | None | Wait for A2 |
| A4 | Establish the concrete OpenJPEG engine | 0/6 | Not started | None | Wait for A3 |
| A5 | Establish the independent OpenJPH engine and adapter | 0/5 | Not started | None | Wait for A4 |
| A6 | Align OpenJPH color and wavelet arithmetic | 0/4 | Not started | None | Wait for A5 |
| A7 | Align QCD, MAGB, CAP, and cleanup coding | 0/4 | Not started | None | Wait for A6 |
| A8 | Align precincts, packets, TLM, and tile-parts | 0/4 | Not started | None | Wait for A7 |
| A9 | Align decoder and output traversal | 0/3 | Not started | None | Wait for A8 |
| A10 | Build the ranged-version C# generator and offline bundle | 0/5 | Not started | None | Wait for structural split |
| A11 | Prove exact bytes and four-way interoperability | 0/5 | Not started | None | Wait for A9 and A10 |
| A12 | Delete mixed branches and old implementation paths | 0/4 | Not started | None | Wait for A11 |
| A13 | Run final quality and benchmark gates | 0/4 | Not started | None | Wait for A12 |

## Evidence log

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
