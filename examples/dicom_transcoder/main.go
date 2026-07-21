// Package main demonstrates a simple DICOM transcoder CLI.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	// Register all codecs by importing them
	_ "github.com/cocosip/go-dicom-codec/jpeg/baseline"
	_ "github.com/cocosip/go-dicom-codec/jpeg/extended"
	_ "github.com/cocosip/go-dicom-codec/jpeg/lossless"
	_ "github.com/cocosip/go-dicom-codec/jpeg/lossless14sv1"
	_ "github.com/cocosip/go-dicom-codec/jpeg2000/htj2k"
	_ "github.com/cocosip/go-dicom-codec/jpeg2000/lossless"
	_ "github.com/cocosip/go-dicom-codec/jpeg2000/lossy"
	_ "github.com/cocosip/go-dicom-codec/jpegls/lossless"
	_ "github.com/cocosip/go-dicom-codec/jpegls/nearlossless"

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

	return ds.AddOrUpdate(element.NewString(tag.PhotometricInterpretation, vr.CS, []string{"YBR_FULL_422"}))
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
