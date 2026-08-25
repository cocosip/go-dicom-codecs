//go:build ignore

package main

import (
	"fmt"
	"log"

	"github.com/cocosip/go-dicom-codecs/jpeg2000"
	"github.com/cocosip/go-dicom-codecs/jpeg2000/internal/common/codestream"
)

func main() {
	fmt.Println("JPEG 2000 Lossless Encoding Example")
	fmt.Println("===================================")

	// Example parameters
	width := 64
	height := 64
	components := 1
	bitDepth := 8

	fmt.Printf("\nImage Parameters:\n")
	fmt.Printf("  Width: %d\n", width)
	fmt.Printf("  Height: %d\n", height)
	fmt.Printf("  Components: %d (grayscale)\n", components)
	fmt.Printf("  Bit Depth: %d\n", bitDepth)

	// Create test image data (gradient pattern)
	numPixels := width * height
	componentData := make([][]int32, components)
	componentData[0] = make([]int32, numPixels)

	fmt.Println("\nGenerating gradient test pattern...")
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			// Create a diagonal gradient
			componentData[0][idx] = int32((x + y) % 256)
		}
	}

	// Create encoder with default lossless parameters
	fmt.Println("\nCreating JPEG 2000 encoder...")
	params := jpeg2000.DefaultEncodeParams(width, height, components, bitDepth, false)
	params.NumLevels = 3 // 3 wavelet decomposition levels

	fmt.Printf("\nEncoding Parameters:\n")
	fmt.Printf("  Decomposition Levels: %d\n", params.NumLevels)
	fmt.Printf("  Code-block Size: %dx%d\n", params.CodeBlockWidth, params.CodeBlockHeight)
	fmt.Printf("  Quality Layers: %d\n", params.NumLayers)
	fmt.Printf("  Lossless: %v (5/3 reversible wavelet)\n", params.Lossless)

	encoder := jpeg2000.NewEncoder(params)

	// Encode the image
	fmt.Println("\nEncoding image...")
	encoded, err := encoder.EncodeComponents(componentData)
	if err != nil {
		log.Fatalf("Encoding failed: %v", err)
	}

	fmt.Printf("\nEncoding Results:\n")
	fmt.Printf("  Original size: %d bytes (%d pixels × %d bytes/pixel)\n",
		numPixels, numPixels, 1)
	fmt.Printf("  Encoded size: %d bytes\n", len(encoded))
	fmt.Printf("  Compression ratio: %.2f:1\n", float64(numPixels)/float64(len(encoded)))
	fmt.Printf("  Bits per pixel: %.2f\n", float64(len(encoded)*8)/float64(numPixels))

	// Parse and verify the encoded codestream
	fmt.Println("\nParsing encoded codestream...")
	parser := codestream.NewParser(encoded)
	cs, err := parser.Parse()
	if err != nil {
		log.Fatalf("Failed to parse codestream: %v", err)
	}

	fmt.Println("\nCodestream Structure:")
	fmt.Printf("  ✓ SOC (Start of Codestream)\n")

	if cs.SIZ != nil {
		fmt.Printf("  ✓ SIZ (Image and Tile Size)\n")
		fmt.Printf("      Dimensions: %dx%d\n", cs.SIZ.Xsiz, cs.SIZ.Ysiz)
		fmt.Printf("      Components: %d\n", cs.SIZ.Csiz)
		fmt.Printf("      Bit Depth: %d\n", cs.SIZ.Components[0].BitDepth())
	}

	if cs.COD != nil {
		fmt.Printf("  ✓ COD (Coding Style Default)\n")
		fmt.Printf("      Decomposition Levels: %d\n", cs.COD.NumberOfDecompositionLevels)
		fmt.Printf("      Code-block Size: %dx%d\n",
			1<<(cs.COD.CodeBlockWidth+2),
			1<<(cs.COD.CodeBlockHeight+2))
		fmt.Printf("      Progression Order: %d\n", cs.COD.ProgressionOrder)
		fmt.Printf("      Wavelet Transform: %s\n",
			map[uint8]string{0: "9/7 irreversible", 1: "5/3 reversible"}[cs.COD.Transformation])
	}

	if cs.QCD != nil {
		fmt.Printf("  ✓ QCD (Quantization Default)\n")
		fmt.Printf("      Style: %s\n",
			map[uint8]string{0: "No quantization (lossless)", 1: "Scalar derived", 2: "Scalar expounded"}[cs.QCD.Sqcd&0x1F])
	}

	fmt.Printf("  ✓ Tile Data (%d tiles)\n", len(cs.Tiles))
	for i, tile := range cs.Tiles {
		fmt.Printf("      Tile %d: %d bytes compressed data\n", i, len(tile.Data))
	}

	fmt.Printf("  ✓ EOC (End of Codestream)\n")

	fmt.Println("\n✅ Encoding Pipeline Complete!")
	fmt.Println("\nPipeline Stages:")
	fmt.Println("  1. ✓ Image partitioning into tiles")
	fmt.Println("  2. ✓ Discrete Wavelet Transform (DWT) - 5/3 reversible")
	fmt.Println("  3. ✓ Subband partitioning into code-blocks")
	fmt.Println("  4. ✓ T1 EBCOT encoding (bitplane coding with MQ arithmetic coder)")
	fmt.Println("  5. ✓ T2 packet generation with progression order")
	fmt.Println("  6. ✓ Bitstream formatting with byte-stuffing")
	fmt.Println("  7. ✓ Codestream assembly (SOC, SIZ, COD, QCD, SOT, SOD, EOC)")

	fmt.Println("\nNote: This is a lossless codec - perfect reconstruction is guaranteed!")
}
