package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
)

type toolOptions struct {
	inputPath       string
	outputDirectory string
	format          string
	showHelp        bool
}

func parseToolOptions(args []string) (toolOptions, error) {
	if len(args) == 0 || isHelpOption(args[0]) {
		return toolOptions{showHelp: true}, nil
	}

	var options toolOptions
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if isHelpOption(arg) {
			return toolOptions{showHelp: true}, nil
		}
		switch arg {
		case "--output-dir", "-o":
			index++
			if index >= len(args) {
				return toolOptions{}, fmt.Errorf("--output-dir requires a directory path")
			}
			options.outputDirectory = trimShellQuotes(args[index])
		case "--format":
			index++
			if index >= len(args) {
				return toolOptions{}, fmt.Errorf("--format requires a format name")
			}
			options.format = trimShellQuotes(args[index])
		default:
			if strings.HasPrefix(arg, "-") {
				return toolOptions{}, fmt.Errorf("unknown option %s", arg)
			}
			if options.inputPath != "" {
				return toolOptions{}, fmt.Errorf("only one input file can be compressed at a time")
			}
			options.inputPath = trimShellQuotes(arg)
		}
	}
	return options, nil
}

func isHelpOption(value string) bool {
	return strings.EqualFold(value, "--help") ||
		strings.EqualFold(value, "-h") ||
		strings.EqualFold(value, "/?")
}

func trimShellQuotes(value string) string {
	return strings.Trim(strings.TrimSpace(value), "\"'")
}

type compressionTargetFormat struct {
	name       string
	transfer   *transfer.Syntax
	suffix     string
	isLossless bool
}

var compressionTargetFormats = []compressionTargetFormat{
	{name: "RLE Lossless", transfer: transfer.RLELossless, suffix: "rle", isLossless: true},
	{name: "JPEG Baseline Process 1", transfer: transfer.JPEGBaseline8Bit, suffix: "jpeg_baseline", isLossless: false},
	{name: "JPEG Extended Process 2/4", transfer: transfer.JPEGProcess2_4, suffix: "jpeg_process2_4", isLossless: false},
	{name: "JPEG Lossless Process 14", transfer: transfer.JPEGLossless, suffix: "jpeg_lossless_14", isLossless: true},
	{name: "JPEG Lossless Process 14 SV1", transfer: transfer.JPEGLosslessSV1, suffix: "jpeg_lossless_sv1", isLossless: true},
	{name: "JPEG-LS Lossless", transfer: transfer.JPEGLSLossless, suffix: "jpegls_lossless", isLossless: true},
	{name: "JPEG-LS Near-Lossless", transfer: transfer.JPEGLSNearLossless, suffix: "jpegls_near_lossless", isLossless: false},
	{name: "JPEG 2000 Lossless", transfer: transfer.JPEG2000Lossless, suffix: "j2k_lossless", isLossless: true},
	{name: "JPEG 2000 Lossy", transfer: transfer.JPEG2000Lossy, suffix: "j2k_lossy", isLossless: false},
	{name: "HTJ2K Lossless", transfer: transfer.HTJ2KLossless, suffix: "htj2k_lossless", isLossless: true},
	{name: "HTJ2K Lossless RPCL", transfer: transfer.HTJ2KLosslessRPCL, suffix: "htj2k_lossless_rpcl", isLossless: true},
	{name: "HTJ2K Lossy", transfer: transfer.HTJ2K, suffix: "htj2k_lossy", isLossless: false},
}

type jpegSequentialDCTImageInfo struct {
	bitsAllocated             uint16
	bitsStored                uint16
	samplesPerPixel           uint16
	photometricInterpretation string
}

type compressionResultStatus string

const (
	compressionResultPending     compressionResultStatus = "pending"
	compressionResultSuccess     compressionResultStatus = "success"
	compressionResultUnsupported compressionResultStatus = "unsupported"
	compressionResultSkipped     compressionResultStatus = "skipped"
	compressionResultFailed      compressionResultStatus = "failed"
)

type compressionPlanItem struct {
	format     compressionTargetFormat
	outputPath string
	status     compressionResultStatus
	message    string
}

type compressionPlan struct {
	outputDirectory string
	items           []compressionPlanItem
}

type compressionResult struct {
	item       compressionPlanItem
	status     compressionResultStatus
	outputSize int64
	message    string
}

func createCompressionPlan(inputPath, outputDirectory string, imageInfo jpegSequentialDCTImageInfo) (compressionPlan, error) {
	if strings.TrimSpace(inputPath) == "" {
		return compressionPlan{}, fmt.Errorf("input path is required")
	}
	if strings.TrimSpace(outputDirectory) == "" {
		inputDirectory := filepath.Dir(inputPath)
		baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
		outputDirectory = filepath.Join(inputDirectory, baseName+"_compressed")
	}

	baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	items := make([]compressionPlanItem, len(compressionTargetFormats))
	for index, format := range compressionTargetFormats {
		items[index] = compressionPlanItem{
			format:     format,
			outputPath: filepath.Join(outputDirectory, baseName+"_"+format.suffix+".dcm"),
			status:     compressionResultPending,
		}
		if unsupportedReason := jpegSequentialDCTUnsupportedReason(format.transfer, imageInfo); unsupportedReason != "" {
			items[index].status = compressionResultUnsupported
			items[index].message = unsupportedReason
		}
	}
	return compressionPlan{outputDirectory: outputDirectory, items: items}, nil
}

func selectCompressionPlanItems(plan compressionPlan, suffix string) ([]compressionPlanItem, error) {
	if suffix == "" {
		return plan.items, nil
	}
	for _, item := range plan.items {
		if strings.EqualFold(item.format.suffix, suffix) {
			return []compressionPlanItem{item}, nil
		}
	}
	return nil, fmt.Errorf("unknown format %s", suffix)
}

func jpegSequentialDCTUnsupportedReason(target *transfer.Syntax, imageInfo jpegSequentialDCTImageInfo) string {
	if target != transfer.JPEGBaseline8Bit && target != transfer.JPEGProcess2_4 {
		return ""
	}
	if target == transfer.JPEGBaseline8Bit && imageInfo.bitsStored > 8 {
		return ""
	}
	if target == transfer.JPEGProcess2_4 &&
		imageInfo.bitsAllocated == 16 &&
		imageInfo.bitsStored == 12 &&
		imageInfo.samplesPerPixel == 1 &&
		(imageInfo.photometricInterpretation == "MONOCHROME1" || imageInfo.photometricInterpretation == "MONOCHROME2") {
		return ""
	}
	if imageInfo.bitsAllocated != 8 || imageInfo.bitsStored != 8 {
		return fmt.Sprintf(
			"JPEG sequential DCT supports only BitsAllocated 8 and BitsStored 8; this image has BitsAllocated=%d, BitsStored=%d",
			imageInfo.bitsAllocated,
			imageInfo.bitsStored,
		)
	}
	if imageInfo.samplesPerPixel != 1 && imageInfo.samplesPerPixel != 3 {
		return "JPEG sequential DCT supports only SamplesPerPixel 1 or 3"
	}
	switch imageInfo.photometricInterpretation {
	case "MONOCHROME1", "MONOCHROME2", "PALETTE COLOR", "RGB", "YBR_FULL", "YBR_FULL_422":
		return ""
	default:
		return fmt.Sprintf("JPEG sequential DCT does not support photometric interpretation %s", imageInfo.photometricInterpretation)
	}
}

func jpegSequentialDCTImageInfoFromDataset(ds *dataset.Dataset) jpegSequentialDCTImageInfo {
	return jpegSequentialDCTImageInfo{
		bitsAllocated:             ds.TryGetUInt16(tag.BitsAllocated, 0),
		bitsStored:                ds.TryGetUInt16(tag.BitsStored, 0),
		samplesPerPixel:           ds.TryGetUInt16(tag.SamplesPerPixel, 0),
		photometricInterpretation: ds.TryGetString(tag.PhotometricInterpretation),
	}
}

func compressionExitCode(results []compressionResult) int {
	if len(results) == 0 {
		return 1
	}
	for _, result := range results {
		if result.status != compressionResultFailed {
			return 0
		}
	}
	return 1
}
