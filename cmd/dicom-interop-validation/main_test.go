package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
)

func TestParseOptionsRejectsInvalidParallelism(t *testing.T) {
	_, err := parseOptions([]string{"--parallel", "0"})
	if err == nil || !strings.Contains(err.Error(), "positive") {
		t.Fatalf("parseOptions() error = %v, want positive parallelism error", err)
	}
}

func TestParseOptionsSelectsOneFormat(t *testing.T) {
	got, err := parseOptions([]string{optionFormat, formatJPEGLosslessSV1})
	if err != nil {
		t.Fatalf("parseOptions() error = %v", err)
	}
	if got.format != formatJPEGLosslessSV1 {
		t.Fatalf("format = %q, want jpeg-lossless-14-sv1", got.format)
	}
	if got.parallel != 4 {
		t.Fatalf("parallel = %d, want 4", got.parallel)
	}
	if got.nativeProject != defaultNativeWorkerProject {
		t.Fatalf("nativeProject = %q, want %q", got.nativeProject, defaultNativeWorkerProject)
	}
}

func TestParseOptionsRejectsUnknownFormat(t *testing.T) {
	_, err := parseOptions([]string{optionFormat, "not-a-codec"})
	if err == nil || !strings.Contains(err.Error(), "unknown format") {
		t.Fatalf("parseOptions() error = %v, want unknown format error", err)
	}
}

func TestFixtureNamesAreStable(t *testing.T) {
	got, err := fixtureNames()
	if err != nil {
		t.Fatalf("fixtureNames() error = %v", err)
	}
	want := []string{"sample-01.dcm", "sample-02.dcm", "sample-03.dcm", "sample-04.dcm", "sample-05.dcm"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fixtureNames() = %v, want %v", got, want)
	}
}

func TestReadFixtureReturnsDICOMBytes(t *testing.T) {
	data, err := readFixture("sample-01.dcm")
	if err != nil {
		t.Fatalf("readFixture() error = %v", err)
	}
	if len(data) < 132 || string(data[128:132]) != "DICM" {
		t.Fatalf("readFixture() returned non-DICOM data of length %d", len(data))
	}
}

func TestCompareFramesHonorsTolerance(t *testing.T) {
	if err := compareFrames([][]byte{{10, 20}}, [][]byte{{11, 19}}, 1); err != nil {
		t.Fatalf("compareFrames() error = %v", err)
	}
	if err := compareFrames([][]byte{{10, 20}}, [][]byte{{12, 19}}, 1); err == nil {
		t.Fatal("compareFrames() error = nil, want tolerance failure")
	}
	if err := compareFrames([][]byte{{10, 20}}, [][]byte{{10, 21}}, 0); err == nil {
		t.Fatal("compareFrames() error = nil, want exact comparison failure")
	}
	if !bytes.Equal([]byte{10, 20}, []byte{10, 20}) {
		t.Fatal("test fixture sanity check failed")
	}
}

func TestCompareImagesUses16BitSampleDifference(t *testing.T) {
	expected := imageData{bitsAllocated: 16, frames: [][]byte{{0xFC, 0x00}}}
	actual := imageData{bitsAllocated: 16, frames: [][]byte{{0x00, 0x01}}}
	if err := compareImages(expected, actual, 4); err != nil {
		t.Fatalf("compareImages() error = %v, want sample difference of 4", err)
	}
}

func TestChildArgsCarryStageAndPaths(t *testing.T) {
	got := childArgs(stageEncode, formatJPEGLosslessSV1, "source.dcm", "encoded.dcm")
	want := []string{optionStage, stageEncode, optionFormat, formatJPEGLosslessSV1, optionInput, "source.dcm", optionOutput, "encoded.dcm"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("childArgs() = %v, want %v", got, want)
	}
}

func TestNativeDecodeArgsUseFoDicomWorker(t *testing.T) {
	got := nativeDecodeArgs("worker.csproj", "input.dcm", "output.dcm")
	want := []string{"run", "--project", "worker.csproj", "--", stageDecode, "input.dcm", "output.dcm"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("nativeDecodeArgs() = %v, want %v", got, want)
	}
}

func TestEncodeStageWritesRequestedTransferSyntax(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.dcm")
	encoded := filepath.Join(dir, "encoded.dcm")
	data, err := readFixture("sample-01.dcm")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := transcodeFile(source, encoded, transfer.JPEGLossless); err != nil {
		t.Fatalf("transcodeFile() error = %v", err)
	}
	result, err := parser.ParseFile(encoded, parser.WithReadOption(parser.ReadAll))
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	if result.TransferSyntax != transfer.JPEGLossless {
		t.Fatalf("transfer syntax = %s, want %s", result.TransferSyntax.UID().UID(), transfer.JPEGLossless.UID().UID())
	}
}

func TestFormatTransferSyntax(t *testing.T) {
	got, err := formatTransferSyntax("htj2k-lossless-rpcl")
	if err != nil {
		t.Fatalf("formatTransferSyntax() error = %v", err)
	}
	if got != transfer.HTJ2KLosslessRPCL {
		t.Fatalf("formatTransferSyntax() = %s, want %s", got.UID().UID(), transfer.HTJ2KLosslessRPCL.UID().UID())
	}
}

func TestJPEG2000LossyToleranceMatchesPureReference(t *testing.T) {
	definition, err := findFormat("jpeg2000-lossy")
	if err != nil {
		t.Fatalf("findFormat() error = %v", err)
	}
	if definition.tolerance != 58 {
		t.Fatalf("jpeg2000-lossy tolerance = %d, want 58", definition.tolerance)
	}
}

func TestSupportsNativeDecodeSkipsMultiFrameJPEGLS(t *testing.T) {
	singleFrame := imageData{bitsAllocated: 8, samples: 1, frames: [][]byte{{1}}}
	multiFrame := imageData{bitsAllocated: 8, samples: 1, frames: [][]byte{{1}, {2}}}
	if !supportsNativeDecode(singleFrame, formatJPEGLSLossless) {
		t.Fatal("single-frame JPEG-LS should be supported by Native")
	}
	if supportsNativeDecode(multiFrame, formatJPEGLSLossless) {
		t.Fatal("multi-frame JPEG-LS lossless should be skipped for Native")
	}
	if supportsNativeDecode(multiFrame, formatJPEGLSNearLossless) {
		t.Fatal("multi-frame JPEG-LS near-lossless should be skipped for Native")
	}
	if !supportsNativeDecode(multiFrame, "jpeg2000-lossless") {
		t.Fatal("multi-frame JPEG 2000 should remain supported by Native")
	}
}

func TestPrepareStageNormalizesCompressedFixture(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.dcm")
	prepared := filepath.Join(dir, "prepared.dcm")
	data, err := readFixture("sample-03.dcm")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	stageOptions, err := parseOptions([]string{optionStage, stagePrepare, optionFormat, formatJPEGLosslessSV1, optionInput, source, optionOutput, prepared})
	if err != nil {
		t.Fatalf("parseOptions() error = %v", err)
	}
	if err := runStage(stageOptions); err != nil {
		t.Fatalf("runStage() error = %v", err)
	}
	result, err := parser.ParseFile(prepared, parser.WithReadOption(parser.ReadAll))
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	if result.TransferSyntax != transfer.ExplicitVRLittleEndian {
		t.Fatalf("transfer syntax = %s, want Explicit VR Little Endian", result.TransferSyntax.UID().UID())
	}
}
