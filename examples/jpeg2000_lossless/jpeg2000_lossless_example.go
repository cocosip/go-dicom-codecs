// Package main provides examples for using the JPEG 2000 lossless codec.
package main

import (
	"fmt"

	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
	_ "github.com/cocosip/go-dicom-codecs/jpeg2000/lossless"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	pixel "github.com/cocosip/go-dicom/pkg/imaging/pixel"
)

func main() {
	fmt.Println("=== JPEG 2000 Lossless Codec Usage Example ===")

	// Example 1: Registry-based usage
	fmt.Println("Example 1: Registry-based decoding (JPEG 2000 Lossless)")
	registryUsageExample()
	fmt.Println()

	// Example 2: Codec information
	fmt.Println("Example 2: Codec information")
	codecInfoExample()
	fmt.Println()
}

func registryUsageExample() {
	// Get codec from registry
	// The codec is automatically registered via init() when the package is imported
	registry := codec.GlobalRegistry()
	j2kCodec, exists := registry.Lookup(transfer.JPEG2000Lossless)
	if !exists {
		fmt.Println("JPEG 2000 Lossless codec not found in registry")
		return
	}

	fmt.Printf("Retrieved codec: %s\n", j2kCodec.Name())
	fmt.Printf("Transfer Syntax UID: %s\n", j2kCodec.TransferSyntax().UID().UID())
	fmt.Println()

	// Note: Actual decoding requires valid JPEG 2000 compressed data
	// This example demonstrates the API usage pattern

	// In a real scenario, you would:
	// 1. Load JPEG 2000 compressed pixel data from a DICOM file
	// 2. Create source PixelData with compressed data and metadata
	// 3. Decode to get uncompressed pixel data

	fmt.Println("Example workflow:")
	fmt.Println("  1. Import package: _ \"github.com/cocosip/go-dicom-codecs/jpeg2000/lossless\"")
	fmt.Println("  2. Get codec: registry.Lookup(transfer.JPEG2000Lossless)")
	fmt.Println("  3. Create src PixelData with compressed JPEG 2000 data")
	fmt.Println("  4. Call codec.Decode(src, dst, nil)")
	fmt.Println("  5. Use dst.Data for uncompressed pixel data")
	fmt.Println()

	// Example structure (with placeholder data)
	frameInfo := &codec.FrameInfo{
		Width:  512,
		Height: 512,

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 16, BitsStored: 12, HighBit: 11, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation("MONOCHROME2"),
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	// Data would contain actual JPEG 2000 codestream
	// src.AddFrame(j2kCompressedData)
	_ = src // Placeholder for example

	fmt.Printf("Example source metadata:\n")
	fmt.Printf("  Dimensions: %dx%d\n", frameInfo.Width, frameInfo.Height)
	fmt.Printf("  Bit depth: %d bits (allocated: %d)\n", frameInfo.BitDepth.BitsStored, frameInfo.BitDepth.BitsAllocated)
	fmt.Printf("  Components: %d\n", frameInfo.SamplesPerPixel)
	fmt.Printf("  Photometric: %s\n", frameInfo.PhotometricInterpretation.Value)
	fmt.Println()

	fmt.Println("To decode:")
	fmt.Println("  dst := codecHelpers.NewTestPixelData(frameInfo)")
	fmt.Println("  err := j2kCodec.Decode(src, dst, nil)")
	fmt.Println("  if err == nil {")
	fmt.Println("    data, _ := dst.GetFrame(0)")
	fmt.Println("    // data contains uncompressed pixel data")
	fmt.Println("  }")
}

func codecInfoExample() {
	registry := codec.GlobalRegistry()
	j2kCodec, exists := registry.Lookup(transfer.JPEG2000Lossless)
	if !exists {
		fmt.Println("Codec not found")
		return
	}

	fmt.Printf("Codec Name: %s\n", j2kCodec.Name())

	ts := j2kCodec.TransferSyntax()
	fmt.Printf("Transfer Syntax:\n")
	fmt.Printf("  UID: %s\n", ts.UID().UID())

	fmt.Println()
	fmt.Println("Capabilities:")
	fmt.Println("  鉁?Decode: Supported (JPEG 2000 Part 1 codestreams)")
	fmt.Println("  鉁?Encode: Not yet implemented")
	fmt.Println()
	fmt.Println("Supported features:")
	fmt.Println("  鈥?Grayscale and RGB images")
	fmt.Println("  鈥?8-bit and 16-bit pixel data")
	fmt.Println("  鈥?5/3 reversible wavelet transform")
	fmt.Println("  鈥?EBCOT Tier-1 decoding (MQ arithmetic coding)")
	fmt.Println("  鈥?EBCOT Tier-2 packet parsing")
	fmt.Println("  鈥?Single-tile codestreams")
	fmt.Println()
	fmt.Println("Limitations (current MVP):")
	fmt.Println("  鈥?Multi-tile images: Only first tile decoded")
	fmt.Println("  鈥?9/7 irreversible wavelet: Not supported")
	fmt.Println("  鈥?ROI coding: Not fully implemented")
	fmt.Println("  鈥?Encoding: Not yet implemented")
}
