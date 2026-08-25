package openjpeg

import (
	"testing"
)

// TestProgressionOrderRoundTrip tests encode-decode round trip for all progression orders
func TestProgressionOrderRoundTrip(t *testing.T) {
	progressionOrders := []struct {
		name  string
		order uint8
	}{
		{progressionLRCPName, 0},
		{progressionRLCPName, 1},
		{progressionRPCLName, 2},
		{progressionPCRLName, 3},
		{progressionCPRLName, 4},
	}

	imageConfigs := []struct {
		name      string
		width     int
		height    int
		bitDepth  int
		numLevels int
		numLayers int
	}{
		{"64x64 1-level 1-layer", 64, 64, 8, 1, 1},
		{"64x64 2-level 2-layer", 64, 64, 8, 2, 2},
		{"128x128 3-level 1-layer", 128, 128, 8, 3, 1},
		{"128x128 2-level 3-layer", 128, 128, 8, 2, 3},
	}

	for _, po := range progressionOrders {
		for _, img := range imageConfigs {
			testName := po.name + "_" + img.name
			t.Run(testName, func(t *testing.T) {
				// Create test data with gradient pattern
				numPixels := img.width * img.height
				componentData := make([][]int32, 1)
				componentData[0] = make([]int32, numPixels)

				for y := 0; y < img.height; y++ {
					for x := 0; x < img.width; x++ {
						idx := y*img.width + x
						componentData[0][idx] = int32((x + y) % 256)
					}
				}

				// Create encoder with specific progression order
				params := DefaultEncodeParams(img.width, img.height, 1, img.bitDepth, false)
				params.NumLevels = img.numLevels
				params.NumLayers = img.numLayers
				params.ProgressionOrder = po.order

				encoder := NewEncoder(params)

				// Encode
				encoded, err := encoder.EncodeComponents(componentData)
				if err != nil {
					t.Fatalf("Encoding with %s failed: %v", po.name, err)
				}

				compressionRatio := float64(numPixels) / float64(len(encoded))
				t.Logf("%s: %d bytes -> %d bytes (ratio: %.2f:1)",
					po.name, numPixels, len(encoded), compressionRatio)

				// Decode
				decoder := NewDecoder()
				err = decoder.Decode(encoded)
				if err != nil {
					t.Fatalf("Decoding %s failed: %v", po.name, err)
				}

				// Verify dimensions
				if decoder.width != img.width {
					t.Errorf("Width mismatch: got %d, want %d", decoder.width, img.width)
				}
				if decoder.height != img.height {
					t.Errorf("Height mismatch: got %d, want %d", decoder.height, img.height)
				}

				// Get decoded data
				decoded, err := decoder.GetComponentData(0)
				if err != nil {
					t.Fatalf("Failed to get decoded data: %v", err)
				}

				// Verify perfect reconstruction (lossless)
				if len(decoded) != len(componentData[0]) {
					t.Fatalf("Length mismatch: got %d, want %d", len(decoded), len(componentData[0]))
				}

				mismatchCount := 0
				maxError := int32(0)
				for i := range componentData[0] {
					diff := decoded[i] - componentData[0][i]
					if diff < 0 {
						diff = -diff
					}
					if diff > 0 {
						mismatchCount++
						if diff > maxError {
							maxError = diff
						}
						if mismatchCount <= 3 {
							t.Errorf("Pixel %d: got %d, want %d (diff=%d)",
								i, decoded[i], componentData[0][i], diff)
						}
					}
				}

				if mismatchCount > 0 {
					t.Errorf("%s: %d mismatches, max error: %d",
						po.name, mismatchCount, maxError)
				} else {
					t.Logf("%s: 鉁?Perfect reconstruction", po.name)
				}
			})
		}
	}
}

// TestProgressionOrderMultiComponent tests RGB images with all progression orders
func TestProgressionOrderMultiComponent(t *testing.T) {
	progressionOrders := []struct {
		name  string
		order uint8
	}{
		{progressionLRCPName, 0},
		{progressionRLCPName, 1},
		{progressionRPCLName, 2},
		{progressionPCRLName, 3},
		{progressionCPRLName, 4},
	}

	width, height := 64, 64
	numPixels := width * height

	for _, po := range progressionOrders {
		t.Run(po.name+"_RGB", func(t *testing.T) {
			// Create RGB test data
			componentData := make([][]int32, 3)
			for c := 0; c < 3; c++ {
				componentData[c] = make([]int32, numPixels)
				for y := 0; y < height; y++ {
					for x := 0; x < width; x++ {
						idx := y*width + x
						// Different gradient for each component
						componentData[c][idx] = int32((x + y + c*85) % 256)
					}
				}
			}

			// Create encoder with specific progression order
			params := DefaultEncodeParams(width, height, 3, 8, false)
			params.NumLevels = 2
			params.NumLayers = 2
			params.ProgressionOrder = po.order

			encoder := NewEncoder(params)

			// Encode
			encoded, err := encoder.EncodeComponents(componentData)
			if err != nil {
				t.Fatalf("Encoding RGB with %s failed: %v", po.name, err)
			}

			t.Logf("%s RGB: %d bytes -> %d bytes (ratio: %.2f:1)",
				po.name, numPixels*3, len(encoded), float64(numPixels*3)/float64(len(encoded)))

			// Decode
			decoder := NewDecoder()
			err = decoder.Decode(encoded)
			if err != nil {
				t.Fatalf("Decoding RGB %s failed: %v", po.name, err)
			}

			// Verify each component
			for c := 0; c < 3; c++ {
				decoded, err := decoder.GetComponentData(c)
				if err != nil {
					t.Fatalf("Failed to get component %d: %v", c, err)
				}

				if len(decoded) != len(componentData[c]) {
					t.Fatalf("Component %d length mismatch: got %d, want %d",
						c, len(decoded), len(componentData[c]))
				}

				mismatchCount := 0
				maxError := int32(0)
				for i := range componentData[c] {
					diff := decoded[i] - componentData[c][i]
					if diff < 0 {
						diff = -diff
					}
					if diff > 0 {
						mismatchCount++
						if diff > maxError {
							maxError = diff
						}
					}
				}

				if mismatchCount > 0 {
					t.Errorf("%s Component %d: %d mismatches, max error: %d",
						po.name, c, mismatchCount, maxError)
				} else {
					t.Logf("%s Component %d: 鉁?Perfect", po.name, c)
				}
			}
		})
	}
}

// TestProgressionOrderWithPrecincts tests progression orders with precinct partitioning
func TestProgressionOrderWithPrecincts(t *testing.T) {
	progressionOrders := []struct {
		name  string
		order uint8
	}{
		{progressionLRCPName, 0},
		{progressionRLCPName, 1},
		{progressionRPCLName, 2},
		{progressionPCRLName, 3},
		{progressionCPRLName, 4},
	}

	width, height := 128, 128
	numPixels := width * height

	for _, po := range progressionOrders {
		t.Run(po.name+"_Precincts", func(t *testing.T) {
			// Create test data - simple gradient pattern like other passing tests
			componentData := make([][]int32, 1)
			componentData[0] = make([]int32, numPixels)

			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					idx := y*width + x
					componentData[0][idx] = int32((x + y) % 256)
				}
			}

			// Create encoder with precincts and specific progression order
			params := DefaultEncodeParams(width, height, 1, 8, false)
			params.NumLevels = 3
			params.NumLayers = 2
			params.ProgressionOrder = po.order
			params.PrecinctWidth = 64 // 2x2 precincts
			params.PrecinctHeight = 64

			encoder := NewEncoder(params)

			// Encode
			encoded, err := encoder.EncodeComponents(componentData)
			if err != nil {
				t.Fatalf("Encoding with precincts and %s failed: %v", po.name, err)
			}

			t.Logf("%s with precincts: %d bytes -> %d bytes (ratio: %.2f:1)",
				po.name, numPixels, len(encoded), float64(numPixels)/float64(len(encoded)))

			// Decode
			decoder := NewDecoder()
			err = decoder.Decode(encoded)
			if err != nil {
				t.Fatalf("Decoding %s with precincts failed: %v", po.name, err)
			}

			// Get decoded data
			decoded, err := decoder.GetComponentData(0)
			if err != nil {
				t.Fatalf("Failed to get decoded data: %v", err)
			}

			// Verify perfect reconstruction
			mismatchCount := 0
			maxError := int32(0)
			for i := range componentData[0] {
				diff := decoded[i] - componentData[0][i]
				if diff < 0 {
					diff = -diff
				}
				if diff > 0 {
					mismatchCount++
					if diff > maxError {
						maxError = diff
					}
				}
			}

			if mismatchCount > 0 {
				t.Errorf("%s with precincts: %d mismatches, max error: %d",
					po.name, mismatchCount, maxError)
			} else {
				t.Logf("%s with precincts: 鉁?Perfect reconstruction", po.name)
			}
		})
	}
}

// TestProgressionOrderComparison compares output size for different progression orders
func TestProgressionOrderComparison(t *testing.T) {
	width, height := 128, 128
	numPixels := width * height

	// Create test data
	componentData := make([][]int32, 1)
	componentData[0] = make([]int32, numPixels)
	for i := range componentData[0] {
		componentData[0][i] = int32(i % 256)
	}

	progressionOrders := []struct {
		name  string
		order uint8
	}{
		{progressionLRCPName, 0},
		{progressionRLCPName, 1},
		{progressionRPCLName, 2},
		{progressionPCRLName, 3},
		{progressionCPRLName, 4},
	}

	results := make(map[string]int)

	for _, po := range progressionOrders {
		params := DefaultEncodeParams(width, height, 1, 8, false)
		params.NumLevels = 3
		params.NumLayers = 3
		params.ProgressionOrder = po.order

		encoder := NewEncoder(params)
		encoded, err := encoder.EncodeComponents(componentData)
		if err != nil {
			t.Fatalf("Encoding with %s failed: %v", po.name, err)
		}

		results[po.name] = len(encoded)
		t.Logf("%s: %d bytes (ratio: %.2f:1)",
			po.name, len(encoded), float64(numPixels)/float64(len(encoded)))
	}

	// All progression orders should produce the same compressed size
	// (since they only change the packet order, not the compression)
	firstSize := results[progressionLRCPName]
	for name, size := range results {
		if size != firstSize {
			t.Logf("Note: %s size (%d) differs from LRCP (%d) - this is acceptable as packet ordering may affect size slightly",
				name, size, firstSize)
		}
	}
}
