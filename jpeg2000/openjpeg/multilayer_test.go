package openjpeg

import (
	"testing"
)

// TestMultiLayerEncoding tests basic multi-layer encoding functionality
func TestMultiLayerEncoding(t *testing.T) {
	// Create test image
	width, height := 64, 64
	numPixels := width * height
	pixelData := make([]byte, numPixels)
	for i := 0; i < numPixels; i++ {
		pixelData[i] = byte(i % 256)
	}

	// Test with 3 layers
	params := DefaultEncodeParams(width, height, 1, 8, false)
	params.NumLayers = 3
	params.Lossless = true

	encoder := NewEncoder(params)
	encoded, err := encoder.Encode(pixelData)
	if err != nil {
		t.Fatalf("Multi-layer encoding failed: %v", err)
	}

	if len(encoded) == 0 {
		t.Fatal("Encoded data is empty")
	}

	t.Logf("Multi-layer encoding successful")
	t.Logf("  Layers: %d", params.NumLayers)
	t.Logf("  Encoded size: %d bytes", len(encoded))
	t.Logf("  Compression ratio: %.2f:1", float64(numPixels)/float64(len(encoded)))
}

// TestLayerAllocationSimple tests the simple layer allocation algorithm
func TestLayerAllocationSimple(t *testing.T) {
	tests := []struct {
		name          string
		totalPasses   int
		numLayers     int
		numCodeBlocks int
	}{
		{"1 Layer", 10, 1, 1},
		{"3 Layers", 10, 3, 1},
		{"5 Layers", 15, 5, 1},
		{"3 Layers Multiple CB", 10, 3, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alloc := AllocateLayersSimple(tt.totalPasses, tt.numLayers, tt.numCodeBlocks)

			if alloc.NumLayers != tt.numLayers {
				t.Errorf("NumLayers = %d, want %d", alloc.NumLayers, tt.numLayers)
			}

			if len(alloc.CodeBlockPasses) != tt.numCodeBlocks {
				t.Errorf("CodeBlockPasses length = %d, want %d",
					len(alloc.CodeBlockPasses), tt.numCodeBlocks)
			}

			// Verify monotonic increasing passes per layer
			for cb := 0; cb < tt.numCodeBlocks; cb++ {
				layerPasses := alloc.CodeBlockPasses[cb]
				if len(layerPasses) != tt.numLayers {
					t.Errorf("CB %d: layer passes length = %d, want %d",
						cb, len(layerPasses), tt.numLayers)
				}

				for layer := 1; layer < tt.numLayers; layer++ {
					if layerPasses[layer] < layerPasses[layer-1] {
						t.Errorf("CB %d: layer %d has %d passes < layer %d has %d passes (not monotonic)",
							cb, layer, layerPasses[layer], layer-1, layerPasses[layer-1])
					}
				}

				// Last layer should have all passes
				if layerPasses[tt.numLayers-1] != tt.totalPasses {
					t.Errorf("CB %d: last layer has %d passes, want %d",
						cb, layerPasses[tt.numLayers-1], tt.totalPasses)
				}

				t.Logf("CB %d passes per layer: %v", cb, layerPasses)
			}
		})
	}
}

// TestMultiLayerDifferentQualities tests multi-layer encoding with different quality settings
func TestMultiLayerDifferentQualities(t *testing.T) {
	width, height := 128, 128
	numPixels := width * height
	pixelData := make([]byte, numPixels)

	// Create gradient pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelData[y*width+x] = byte((x + y) % 256)
		}
	}

	layerCounts := []int{1, 2, 3, 5}

	for _, numLayers := range layerCounts {
		t.Run(testName(numLayers), func(t *testing.T) {
			params := DefaultEncodeParams(width, height, 1, 8, false)
			params.NumLayers = numLayers
			params.Lossless = true
			params.NumLevels = 5

			encoder := NewEncoder(params)
			encoded, err := encoder.Encode(pixelData)
			if err != nil {
				t.Fatalf("Encoding with %d layers failed: %v", numLayers, err)
			}

			compressionRatio := float64(numPixels) / float64(len(encoded))
			t.Logf("%d layers: size=%d bytes, ratio=%.2f:1",
				numLayers, len(encoded), compressionRatio)

			// Verify codestream can be decoded
			decoder := NewDecoder()
			if err := decoder.Decode(encoded); err != nil {
				t.Fatalf("Decoding %d-layer image failed: %v", numLayers, err)
			}

			if decoder.Width() != width || decoder.Height() != height {
				t.Errorf("Decoded dimensions %dx%d, want %dx%d",
					decoder.Width(), decoder.Height(), width, height)
			}

			// Verify pixel values (lossless should be perfect)
			decoded := decoder.GetPixelData()
			if len(decoded) != len(pixelData) {
				t.Errorf("Decoded length %d, want %d", len(decoded), len(pixelData))
			}
			maxError := 0
			for i := 0; i < len(pixelData) && i < len(decoded); i++ {
				diff := int(pixelData[i]) - int(decoded[i])
				if diff < 0 {
					diff = -diff
				}
				if diff > maxError {
					maxError = diff
				}
			}
			// Multi-layer lossless should achieve perfect reconstruction (0 error)
			if maxError > 0 {
				t.Errorf("Lossless decoding has error %d, want 0 (perfect reconstruction)", maxError)
			}
		})
	}
}

func testName(layers int) string {
	if layers == 1 {
		return "1 layer"
	}
	return string(rune('0'+layers)) + " layers"
}

// TestSingleLayerLossyEncoding tests single-layer lossy encoding as baseline
func TestSingleLayerLossyEncoding(t *testing.T) {
	width, height := 64, 64
	numPixels := width * height
	pixelData := make([]byte, numPixels)
	for i := 0; i < numPixels; i++ {
		pixelData[i] = byte(i % 256)
	}

	params := DefaultEncodeParams(width, height, 1, 8, false)
	params.NumLayers = 1 // Single layer
	params.Lossless = false
	params.Quality = 80
	params.NumLevels = 5

	encoder := NewEncoder(params)
	encoded, err := encoder.Encode(pixelData)
	if err != nil {
		t.Fatalf("Single-layer lossy encoding failed: %v", err)
	}

	t.Logf("Single-layer lossy encoding successful")
	t.Logf("  Layers: %d", params.NumLayers)
	t.Logf("  Quality: %d", params.Quality)
	t.Logf("  Encoded size: %d bytes", len(encoded))
	t.Logf("  Compression ratio: %.2f:1", float64(numPixels)/float64(len(encoded)))

	// Decode and verify
	decoder := NewDecoder()
	if err := decoder.Decode(encoded); err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}

	decoded := decoder.GetPixelData()
	if len(decoded) != len(pixelData) {
		t.Errorf("Decoded length %d, want %d", len(decoded), len(pixelData))
	}

	// Calculate error (lossy, so some error expected)
	maxError := 0
	totalError := 0
	for i := 0; i < len(pixelData); i++ {
		diff := int(pixelData[i]) - int(decoded[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError {
			maxError = diff
		}
		totalError += diff
	}
	avgError := float64(totalError) / float64(len(pixelData))

	t.Logf("  Max error: %d pixels", maxError)
	t.Logf("  Avg error: %.2f pixels", avgError)
}

// TestMultiLayerLossyEncoding tests multi-layer with lossy encoding
func TestMultiLayerLossyEncoding(t *testing.T) {
	width, height := 64, 64
	numPixels := width * height
	pixelData := make([]byte, numPixels)
	for i := 0; i < numPixels; i++ {
		pixelData[i] = byte(i % 256)
	}

	params := DefaultEncodeParams(width, height, 1, 8, false)
	params.NumLayers = 3
	params.Lossless = false
	params.Quality = 80
	params.NumLevels = 5

	encoder := NewEncoder(params)
	encoded, err := encoder.Encode(pixelData)
	if err != nil {
		t.Fatalf("Multi-layer lossy encoding failed: %v", err)
	}

	t.Logf("Multi-layer lossy encoding successful")
	t.Logf("  Layers: %d", params.NumLayers)
	t.Logf("  Quality: %d", params.Quality)
	t.Logf("  Encoded size: %d bytes", len(encoded))
	t.Logf("  Compression ratio: %.2f:1", float64(numPixels)/float64(len(encoded)))

	// Decode and verify
	decoder := NewDecoder()
	if err := decoder.Decode(encoded); err != nil {
		t.Fatalf("Decoding failed: %v", err)
	}

	decoded := decoder.GetPixelData()
	if len(decoded) != len(pixelData) {
		t.Errorf("Decoded length %d, want %d", len(decoded), len(pixelData))
	}

	// Calculate error (lossy, so some error expected)
	maxError := 0
	totalError := 0
	for i := 0; i < len(pixelData); i++ {
		diff := int(pixelData[i]) - int(decoded[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError {
			maxError = diff
		}
		totalError += diff
	}
	avgError := float64(totalError) / float64(len(pixelData))

	t.Logf("  Max error: %d pixels", maxError)
	t.Logf("  Avg error: %.2f pixels", avgError)

	// Note: Error comparison with single-layer baseline in TestSingleLayerLossyEncoding
}
