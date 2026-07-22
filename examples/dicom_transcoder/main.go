// Package main demonstrates a simple DICOM transcoder CLI.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	// Register all codecs by importing them
	_ "github.com/cocosip/go-dicom-codecs/jpeg/baseline"
	_ "github.com/cocosip/go-dicom-codecs/jpeg/extended"
	_ "github.com/cocosip/go-dicom-codecs/jpeg/lossless"
	_ "github.com/cocosip/go-dicom-codecs/jpeg/lossless14sv1"
	_ "github.com/cocosip/go-dicom-codecs/jpeg2000/htj2k"
	_ "github.com/cocosip/go-dicom-codecs/jpeg2000/lossless"
	_ "github.com/cocosip/go-dicom-codecs/jpeg2000/lossy"
	_ "github.com/cocosip/go-dicom-codecs/jpegls/lossless"
	_ "github.com/cocosip/go-dicom-codecs/jpegls/nearlossless"
	_ "github.com/cocosip/go-dicom-codecs/rle"

	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/dicom/vr"
	"github.com/cocosip/go-dicom/pkg/dicom/writer"
	"github.com/cocosip/go-dicom/pkg/imaging"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	options, err := parseToolOptions(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	if options.showHelp {
		printUsage()
		return 0
	}
	if options.inputPath == "" {
		printUsage()
		return 1
	}
	if _, err := os.Stat(options.inputPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: input DICOM file was not found: %v\n", err)
		return 1
	}

	fmt.Println("DICOM automatic compression tool")
	fmt.Println("Input: " + options.inputPath)
	parseResult, err := parser.ParseFile(options.inputPath,
		parser.WithReadOption(parser.ReadAll),
		parser.WithLargeObjectSize(100*1024*1024),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to read DICOM file: %v\n", err)
		return 1
	}

	displayImageInfo(parseResult.Dataset, parseResult.TransferSyntax)
	plan, results, err := executeCompressionPlan(
		options.inputPath,
		options.outputDirectory,
		options.format,
		parseResult.Dataset,
		parseResult.TransferSyntax,
		codec.GetGlobalRegistry(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	printCompressionResults(plan.outputDirectory, results)
	return compressionExitCode(results)
}

func executeCompressionPlan(
	inputPath, outputDirectory, targetSuffix string,
	ds *dataset.Dataset,
	sourceTS *transfer.Syntax,
	registry *codec.Registry,
) (compressionPlan, []compressionResult, error) {
	plan, err := createCompressionPlan(inputPath, outputDirectory, jpegSequentialDCTImageInfoFromDataset(ds))
	if err != nil {
		return compressionPlan{}, nil, err
	}
	items, err := selectCompressionPlanItems(plan, targetSuffix)
	if err != nil {
		return compressionPlan{}, nil, err
	}
	if err := os.MkdirAll(plan.outputDirectory, 0755); err != nil {
		return compressionPlan{}, nil, fmt.Errorf("create output directory: %w", err)
	}

	inputSize, err := getFileSize(inputPath)
	if err != nil {
		return compressionPlan{}, nil, fmt.Errorf("read input file size: %w", err)
	}
	results := make([]compressionResult, 0, len(items))
	for _, item := range items {
		if item.status == compressionResultUnsupported {
			results = append(results, compressionResult{item: item, status: item.status, message: item.message})
			continue
		}

		if err := transcodeDICOMFile(ds, item.outputPath, sourceTS, item.format.transfer, registry); err != nil {
			results = append(results, compressionResult{item: item, status: compressionResultFailed, message: err.Error()})
			continue
		}
		outputSize, err := getFileSize(item.outputPath)
		if err != nil {
			results = append(results, compressionResult{item: item, status: compressionResultFailed, message: err.Error()})
			continue
		}
		ratio := 0.0
		if outputSize > 0 {
			ratio = float64(inputSize) / float64(outputSize)
		}
		results = append(results, compressionResult{
			item:       item,
			status:     compressionResultSuccess,
			outputSize: outputSize,
			message:    fmt.Sprintf("%.2fx compression", ratio),
		})
	}
	return plan, results, nil
}

func displayImageInfo(ds *dataset.Dataset, sourceTS *transfer.Syntax) {
	fmt.Println("Image:")
	fmt.Printf("    Rows: %d\n", ds.TryGetUInt16(tag.Rows, 0))
	fmt.Printf("    Columns: %d\n", ds.TryGetUInt16(tag.Columns, 0))
	fmt.Printf("    Bits Stored: %d\n", ds.TryGetUInt16(tag.BitsStored, 0))
	fmt.Printf("    Samples Per Pixel: %d\n", ds.TryGetUInt16(tag.SamplesPerPixel, 0))
	fmt.Printf("    Photometric Interpretation: %s\n", ds.TryGetString(tag.PhotometricInterpretation))
	fmt.Printf("    Source Transfer Syntax: %s\n", sourceTS.UID().UID())
}

func printCompressionResults(outputDirectory string, results []compressionResult) {
	fmt.Println("Output directory: " + outputDirectory)
	fmt.Println()
	counts := map[compressionResultStatus]int{}
	for index, result := range results {
		fmt.Printf("[%d/%d] %s\n", index+1, len(results), result.item.format.name)
		fmt.Println("    Transfer Syntax: " + result.item.format.transfer.UID().UID())
		switch result.status {
		case compressionResultSuccess:
			fmt.Println("    Success: " + filepath.Base(result.item.outputPath))
			fmt.Printf("    Size: %s (%s)\n", formatBytes(result.outputSize), result.message)
		case compressionResultUnsupported:
			fmt.Println("    Unsupported: " + result.message)
		case compressionResultSkipped:
			fmt.Println("    Skipped: " + result.message)
		case compressionResultFailed:
			fmt.Println("    Failed: " + result.message)
		}
		counts[result.status]++
	}
	fmt.Printf(
		"\nSummary: %d succeeded, %d unsupported, %d skipped, %d failed.\n",
		counts[compressionResultSuccess],
		counts[compressionResultUnsupported],
		counts[compressionResultSkipped],
		counts[compressionResultFailed],
	)
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  dicom_transcoder <input-file> [--output-dir <directory>] [--format <format>]")
}

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
	photometricInterpretationYBRFull422                         = "YBR_FULL_422"
	compressionResultPending            compressionResultStatus = "pending"
	compressionResultSuccess            compressionResultStatus = "success"
	compressionResultUnsupported        compressionResultStatus = "unsupported"
	compressionResultSkipped            compressionResultStatus = "skipped"
	compressionResultFailed             compressionResultStatus = "failed"
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
	case "MONOCHROME1", "MONOCHROME2", "PALETTE COLOR", "RGB", "YBR_FULL", photometricInterpretationYBRFull422:
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

// transcodeDICOMFile converts a DICOM dataset from one transfer syntax to another
func transcodeDICOMFile(ds *dataset.Dataset, outputPath string, sourceTS, targetTS *transfer.Syntax, registry *codec.Registry) error {
	// Skip if already in target format
	if sourceTS.UID().UID() == targetTS.UID().UID() {
		clone := ds.Clone()
		if err := writer.WriteFile(outputPath, clone, writer.WithTransferSyntax(sourceTS)); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
		return nil
	}

	// Use go-dicom transcoder which handles encapsulated data, BOT/padding, etc.
	transcoder := codec.NewTranscoder(sourceTS, targetTS, codec.WithCodecRegistry(registry), codec.WithStrictDICOMVR(false))
	newDS, err := transcoder.Transcode(ds)
	if err != nil {
		return fmt.Errorf("transcode failed: %w", err)
	}
	if err := applyBaselinePhotometricInterpretation(newDS, targetTS); err != nil {
		return fmt.Errorf("failed to update JPEG Baseline photometric interpretation: %w", err)
	}
	if err := applyLossyImageCompressionMetadataFromPixelData(newDS, targetTS); err != nil {
		return fmt.Errorf("failed to update lossy image compression metadata: %w", err)
	}

	// Note: Codec layer automatically handles signed pixel data (PR=1) correctly:
	// - During encoding: adds offset for signed data if needed
	// - During decoding: reverses offset to restore signed data
	// - PixelRepresentation metadata is preserved unchanged
	// DO NOT modify PixelRepresentation or RescaleIntercept here!

	// Write with correct transfer syntax (also ensures File Meta includes TSUID)
	if err := writer.WriteFile(outputPath, newDS, writer.WithTransferSyntax(targetTS)); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return nil
}

func applyBaselinePhotometricInterpretation(ds *dataset.Dataset, targetTS *transfer.Syntax) error {
	if targetTS != transfer.JPEGBaseline8Bit || ds.TryGetUInt16(tag.SamplesPerPixel, 0) != 3 {
		return nil
	}

	return ds.AddOrUpdate(element.NewString(tag.PhotometricInterpretation, vr.CS, []string{photometricInterpretationYBRFull422}))
}

func applyLossyImageCompressionMetadataFromPixelData(ds *dataset.Dataset, targetTS *transfer.Syntax) error {
	if !targetTS.IsLossy() {
		return nil
	}

	pixelData, err := imaging.CreatePixelData(ds)
	if err != nil {
		return fmt.Errorf("read transcoded pixel data: %w", err)
	}

	compressedBytes := 0
	for frame := 0; frame < pixelData.FrameCount(); frame++ {
		data, err := pixelData.GetFrame(frame)
		if err != nil {
			return fmt.Errorf("read compressed frame %d: %w", frame, err)
		}
		compressedBytes += len(data)
	}

	return applyLossyImageCompressionMetadata(ds, targetTS, pixelData.Info.TotalUncompressedSize(), compressedBytes)
}

func applyLossyImageCompressionMetadata(ds *dataset.Dataset, targetTS *transfer.Syntax, uncompressedBytes, compressedBytes int) error {
	if !targetTS.IsLossy() {
		return nil
	}
	if uncompressedBytes <= 0 || compressedBytes <= 0 {
		return fmt.Errorf("invalid compression sizes: uncompressed=%d compressed=%d", uncompressedBytes, compressedBytes)
	}

	ratio := float64(uncompressedBytes) / float64(compressedBytes)
	if err := ds.AddOrUpdate(element.NewString(tag.LossyImageCompression, vr.CS, []string{"01"})); err != nil {
		return err
	}
	if err := ds.AddOrUpdate(element.NewString(tag.LossyImageCompressionRatio, vr.DS, []string{fmt.Sprintf("%.3f", ratio)})); err != nil {
		return err
	}
	return ds.AddOrUpdate(element.NewString(tag.LossyImageCompressionMethod, vr.CS, []string{targetTS.LossyCompressionMethod()}))
}

// getFileSize returns the size of a file in bytes
func getFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// formatBytes formats bytes into human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
