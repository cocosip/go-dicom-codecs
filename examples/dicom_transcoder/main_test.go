package main

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/dicom/vr"
)

func TestParseToolOptionsMatchesPureCodecsContract(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    toolOptions
		wantErr string
	}{
		{
			name: "input with output directory and format",
			args: []string{"image.dcm", "--output-dir", "out", "--format", "j2k_lossless"},
			want: toolOptions{inputPath: "image.dcm", outputDirectory: "out", format: "j2k_lossless"},
		},
		{
			name: "short output directory option",
			args: []string{"image.dcm", "-o", "out"},
			want: toolOptions{inputPath: "image.dcm", outputDirectory: "out"},
		},
		{
			name: "help",
			args: []string{"--help"},
			want: toolOptions{showHelp: true},
		},
		{
			name:    "unknown option",
			args:    []string{"image.dcm", "--unknown"},
			wantErr: "unknown option --unknown",
		},
		{
			name:    "multiple inputs",
			args:    []string{"one.dcm", "two.dcm"},
			wantErr: "only one input file can be compressed at a time",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseToolOptions(tt.args)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("parseToolOptions() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseToolOptions() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseToolOptions() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCompressionTargetFormatsMatchPureCodecsOrder(t *testing.T) {
	want := []string{
		"rle",
		"jpeg_baseline",
		"jpeg_process2_4",
		"jpeg_lossless_14",
		"jpeg_lossless_sv1",
		"jpegls_lossless",
		"jpegls_near_lossless",
		"j2k_lossless",
		"j2k_lossy",
		"htj2k_lossless",
		"htj2k_lossless_rpcl",
		"htj2k_lossy",
	}

	got := make([]string, len(compressionTargetFormats))
	for index, format := range compressionTargetFormats {
		got[index] = format.suffix
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("compression target suffixes = %v, want %v", got, want)
	}
}

func TestCreateCompressionPlanUsesPureCodecsOutputPaths(t *testing.T) {
	inputPath := filepath.Join("input", "study.dcm")
	plan, err := createCompressionPlan(inputPath, "", jpegSequentialDCTImageInfo{})
	if err != nil {
		t.Fatalf("createCompressionPlan() error = %v", err)
	}

	if got, want := plan.outputDirectory, filepath.Join("input", "study_compressed"); got != want {
		t.Errorf("output directory = %q, want %q", got, want)
	}
	if got, want := plan.items[0].outputPath, filepath.Join("input", "study_compressed", "study_rle.dcm"); got != want {
		t.Errorf("first output path = %q, want %q", got, want)
	}
	if got, want := plan.items[len(plan.items)-1].outputPath, filepath.Join("input", "study_compressed", "study_htj2k_lossy.dcm"); got != want {
		t.Errorf("last output path = %q, want %q", got, want)
	}
}

func TestCreateCompressionPlanMarksUnsupportedSequentialDCT(t *testing.T) {
	plan, err := createCompressionPlan("study.dcm", "", jpegSequentialDCTImageInfo{
		bitsAllocated:             16,
		bitsStored:                16,
		samplesPerPixel:           1,
		photometricInterpretation: "MONOCHROME2",
	})
	if err != nil {
		t.Fatalf("createCompressionPlan() error = %v", err)
	}

	for _, item := range plan.items {
		switch item.format.suffix {
		case "jpeg_baseline":
			if item.status != compressionResultPending {
				t.Errorf("JPEG Baseline status = %v, want pending", item.status)
			}
		case "jpeg_process2_4":
			if item.status != compressionResultUnsupported {
				t.Errorf("JPEG Process 2/4 status = %v, want unsupported", item.status)
			}
			if item.message == "" {
				t.Error("JPEG Process 2/4 unsupported result has no reason")
			}
		}
	}
}

func TestSelectCompressionPlanItemsMatchesFormatSuffixCaseInsensitively(t *testing.T) {
	plan, err := createCompressionPlan("study.dcm", "", jpegSequentialDCTImageInfo{})
	if err != nil {
		t.Fatalf("createCompressionPlan() error = %v", err)
	}

	items, err := selectCompressionPlanItems(plan, "J2K_LOSSLESS")
	if err != nil {
		t.Fatalf("selectCompressionPlanItems() error = %v", err)
	}
	if len(items) != 1 || items[0].format.suffix != "j2k_lossless" {
		t.Errorf("selected items = %#v, want only j2k_lossless", items)
	}

	if _, err := selectCompressionPlanItems(plan, "unknown"); err == nil || err.Error() != "unknown format unknown" {
		t.Errorf("unknown format error = %v, want unknown format unknown", err)
	}
}

func TestJPEGSequentialDCTImageInfoFromDataset(t *testing.T) {
	ds := dataset.New()
	for _, elem := range []element.Element{
		element.NewUnsignedShort(tag.BitsAllocated, []uint16{16}),
		element.NewUnsignedShort(tag.BitsStored, []uint16{12}),
		element.NewUnsignedShort(tag.SamplesPerPixel, []uint16{1}),
		element.NewString(tag.PhotometricInterpretation, vr.CS, []string{"MONOCHROME2"}),
	} {
		if err := ds.Add(elem); err != nil {
			t.Fatal(err)
		}
	}

	got := jpegSequentialDCTImageInfoFromDataset(ds)
	want := jpegSequentialDCTImageInfo{
		bitsAllocated:             16,
		bitsStored:                12,
		samplesPerPixel:           1,
		photometricInterpretation: "MONOCHROME2",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("JPEG sequential DCT image info = %#v, want %#v", got, want)
	}
}

func TestCompressionExitCodeOnlyFailsWhenEverySelectedTargetFails(t *testing.T) {
	tests := []struct {
		name    string
		results []compressionResult
		want    int
	}{
		{
			name: "success and failure",
			results: []compressionResult{
				{status: compressionResultSuccess},
				{status: compressionResultFailed},
			},
			want: 0,
		},
		{
			name:    "all failed",
			results: []compressionResult{{status: compressionResultFailed}},
			want:    1,
		},
		{
			name:    "unsupported",
			results: []compressionResult{{status: compressionResultUnsupported}},
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compressionExitCode(tt.results); got != tt.want {
				t.Errorf("compressionExitCode() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestApplyBaselinePhotometricInterpretationMatchesFoDicom(t *testing.T) {
	ds := dataset.New()
	if err := ds.Add(element.NewUnsignedShort(tag.SamplesPerPixel, []uint16{3})); err != nil {
		t.Fatal(err)
	}
	if err := ds.Add(element.NewString(tag.PhotometricInterpretation, vr.CS, []string{"RGB"})); err != nil {
		t.Fatal(err)
	}

	if err := applyBaselinePhotometricInterpretation(ds, transfer.JPEGBaseline8Bit); err != nil {
		t.Fatalf("applyBaselinePhotometricInterpretation() error = %v", err)
	}
	if got := ds.TryGetString(tag.PhotometricInterpretation); got != "YBR_FULL_422" {
		t.Errorf("PhotometricInterpretation = %q, want YBR_FULL_422", got)
	}
}

func TestApplyLossyImageCompressionMetadataMatchesTransferSyntax(t *testing.T) {
	ds := dataset.New()
	if err := applyLossyImageCompressionMetadata(ds, transfer.JPEGBaseline8Bit, 1243, 200); err != nil {
		t.Fatalf("applyLossyImageCompressionMetadata() error = %v", err)
	}

	if got := ds.TryGetString(tag.LossyImageCompression); got != "01" {
		t.Errorf("LossyImageCompression = %q, want 01", got)
	}
	if got := ds.TryGetString(tag.LossyImageCompressionRatio); got != "6.215" {
		t.Errorf("LossyImageCompressionRatio = %q, want 6.215", got)
	}
	if got := ds.TryGetString(tag.LossyImageCompressionMethod); got != "ISO_10918_1" {
		t.Errorf("LossyImageCompressionMethod = %q, want ISO_10918_1", got)
	}
}
