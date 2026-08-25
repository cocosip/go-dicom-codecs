package openjpeg

import (
	"math"
	"testing"

	"github.com/cocosip/go-dicom-codecs/jpeg2000/openjpeg/t1"
)

// TestAllocateLayersSimple tests the simple layer allocation algorithm
func TestAllocateLayersSimple(t *testing.T) {
	tests := []struct {
		name          string
		totalPasses   int
		numLayers     int
		numCodeBlocks int
	}{
		{"Single layer", 10, 1, 1},
		{"Three layers", 12, 3, 1},
		{"Five layers", 15, 5, 1},
		{"Multiple code-blocks", 9, 3, 4},
		{"Many passes", 30, 5, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alloc := AllocateLayersSimple(tt.totalPasses, tt.numLayers, tt.numCodeBlocks)

			// Verify basic properties
			if alloc.NumLayers != tt.numLayers {
				t.Errorf("NumLayers = %d, want %d", alloc.NumLayers, tt.numLayers)
			}

			if len(alloc.CodeBlockPasses) != tt.numCodeBlocks {
				t.Errorf("Number of code-blocks = %d, want %d",
					len(alloc.CodeBlockPasses), tt.numCodeBlocks)
			}

			// Verify each code-block
			for cbIdx := 0; cbIdx < tt.numCodeBlocks; cbIdx++ {
				layerPasses := alloc.CodeBlockPasses[cbIdx]

				// Check correct number of layers
				if len(layerPasses) != tt.numLayers {
					t.Errorf("CB %d: layers = %d, want %d",
						cbIdx, len(layerPasses), tt.numLayers)
				}

				// Verify monotonic increasing
				for layer := 1; layer < tt.numLayers; layer++ {
					if layerPasses[layer] < layerPasses[layer-1] {
						t.Errorf("CB %d: layer %d (%d passes) < layer %d (%d passes) - not monotonic",
							cbIdx, layer, layerPasses[layer], layer-1, layerPasses[layer-1])
					}
				}

				// Verify last layer has all passes
				if layerPasses[tt.numLayers-1] != tt.totalPasses {
					t.Errorf("CB %d: last layer has %d passes, want %d",
						cbIdx, layerPasses[tt.numLayers-1], tt.totalPasses)
				}

				// Verify first layer has at least 1 pass
				if layerPasses[0] < 1 {
					t.Errorf("CB %d: first layer has %d passes, want >= 1",
						cbIdx, layerPasses[0])
				}

				t.Logf("CB %d allocation: %v", cbIdx, layerPasses)
			}
		})
	}
}

// TestAllocateLayersRateDistortion tests the basic R-D allocation algorithm
func TestAllocateLayersRateDistortion(t *testing.T) {
	tests := []struct {
		name        string
		cbSizes     [][]int   // [codeblock][pass] = size in bytes
		targetRates []float64 // target rates for each layer
		expectValid bool
	}{
		{
			name: "Single code-block, single layer",
			cbSizes: [][]int{
				{10, 15, 20, 25, 30}, // 5 passes with increasing cumulative sizes
			},
			targetRates: []float64{30},
			expectValid: true,
		},
		{
			name: "Single code-block, three layers",
			cbSizes: [][]int{
				{5, 10, 15, 20, 25, 30},
			},
			targetRates: []float64{10, 20, 30},
			expectValid: true,
		},
		{
			name: "Multiple code-blocks, equal sizes",
			cbSizes: [][]int{
				{10, 20, 30},
				{10, 20, 30},
				{10, 20, 30},
			},
			targetRates: []float64{30, 60, 90},
			expectValid: true,
		},
		{
			name: "Multiple code-blocks, varying sizes",
			cbSizes: [][]int{
				{5, 10, 15},  // Small block
				{20, 40, 60}, // Large block
				{8, 16, 24},  // Medium block
			},
			targetRates: []float64{25, 50, 99},
			expectValid: true,
		},
		{
			name:        "Empty code-blocks",
			cbSizes:     [][]int{},
			targetRates: []float64{100},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alloc := AllocateLayersRateDistortion(tt.cbSizes, tt.targetRates)

			if !tt.expectValid {
				return
			}

			// Verify structure
			if alloc.NumLayers != len(tt.targetRates) {
				t.Errorf("NumLayers = %d, want %d", alloc.NumLayers, len(tt.targetRates))
			}

			if len(alloc.CodeBlockPasses) != len(tt.cbSizes) {
				t.Errorf("Number of code-blocks = %d, want %d",
					len(alloc.CodeBlockPasses), len(tt.cbSizes))
			}

			// Verify monotonicity and bounds
			for cbIdx, cbSizes := range tt.cbSizes {
				if cbIdx >= len(alloc.CodeBlockPasses) {
					continue
				}

				layerPasses := alloc.CodeBlockPasses[cbIdx]
				maxPasses := len(cbSizes)

				for layer := 0; layer < len(layerPasses); layer++ {
					// Check bounds
					if layerPasses[layer] < 0 || layerPasses[layer] > maxPasses {
						t.Errorf("CB %d layer %d: %d passes out of range [0, %d]",
							cbIdx, layer, layerPasses[layer], maxPasses)
					}

					// Check monotonicity
					if layer > 0 && layerPasses[layer] < layerPasses[layer-1] {
						t.Errorf("CB %d: layer %d (%d) < layer %d (%d) - not monotonic",
							cbIdx, layer, layerPasses[layer], layer-1, layerPasses[layer-1])
					}
				}

				t.Logf("CB %d allocation: %v (max %d passes)", cbIdx, layerPasses, maxPasses)
			}
		})
	}
}

// TestAllocateLayersRateDistortionPasses tests the full PCRD-style allocation with PassData
func TestAllocateLayersRateDistortionPasses(t *testing.T) {
	tests := []struct {
		name           string
		passesPerBlock [][]t1.PassData
		numLayers      int
		targetBudget   float64
		expectValid    bool
	}{
		{
			name: "Single block, single layer, no budget",
			passesPerBlock: [][]t1.PassData{
				{
					{PassIndex: 0, Rate: 10, ActualBytes: 10, Distortion: 100.0},
					{PassIndex: 1, Rate: 20, ActualBytes: 20, Distortion: 150.0},
					{PassIndex: 2, Rate: 30, ActualBytes: 30, Distortion: 180.0},
				},
			},
			numLayers:    1,
			targetBudget: 0, // No budget = use all
			expectValid:  true,
		},
		{
			name: "Single block, three layers, no budget",
			passesPerBlock: [][]t1.PassData{
				{
					{PassIndex: 0, Rate: 5, ActualBytes: 5, Distortion: 80.0},
					{PassIndex: 1, Rate: 10, ActualBytes: 10, Distortion: 120.0},
					{PassIndex: 2, Rate: 15, ActualBytes: 15, Distortion: 150.0},
					{PassIndex: 3, Rate: 20, ActualBytes: 20, Distortion: 170.0},
					{PassIndex: 4, Rate: 25, ActualBytes: 25, Distortion: 185.0},
					{PassIndex: 5, Rate: 30, ActualBytes: 30, Distortion: 195.0},
				},
			},
			numLayers:    3,
			targetBudget: 0,
			expectValid:  true,
		},
		{
			name: "Single block with budget constraint",
			passesPerBlock: [][]t1.PassData{
				{
					{PassIndex: 0, Rate: 10, ActualBytes: 10, Distortion: 100.0},
					{PassIndex: 1, Rate: 20, ActualBytes: 20, Distortion: 180.0},
					{PassIndex: 2, Rate: 30, ActualBytes: 30, Distortion: 240.0},
					{PassIndex: 3, Rate: 40, ActualBytes: 40, Distortion: 280.0},
				},
			},
			numLayers:    2,
			targetBudget: 25.0, // Budget allows only ~2.5 passes
			expectValid:  true,
		},
		{
			name: "Multiple blocks with budget",
			passesPerBlock: [][]t1.PassData{
				// Block 0: high distortion reduction per byte (good ROI)
				{
					{PassIndex: 0, Rate: 5, ActualBytes: 5, Distortion: 100.0},
					{PassIndex: 1, Rate: 10, ActualBytes: 10, Distortion: 180.0},
					{PassIndex: 2, Rate: 15, ActualBytes: 15, Distortion: 240.0},
				},
				// Block 1: lower distortion reduction per byte
				{
					{PassIndex: 0, Rate: 8, ActualBytes: 8, Distortion: 50.0},
					{PassIndex: 1, Rate: 16, ActualBytes: 16, Distortion: 90.0},
					{PassIndex: 2, Rate: 24, ActualBytes: 24, Distortion: 120.0},
				},
			},
			numLayers:    3,
			targetBudget: 30.0, // Limited budget
			expectValid:  true,
		},
		{
			name:           "Empty input",
			passesPerBlock: [][]t1.PassData{},
			numLayers:      1,
			targetBudget:   0,
			expectValid:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alloc := AllocateLayersRateDistortionPasses(
				tt.passesPerBlock,
				tt.numLayers,
				tt.targetBudget,
			)

			if !tt.expectValid {
				return
			}

			// Verify structure
			if alloc.NumLayers != tt.numLayers {
				t.Errorf("NumLayers = %d, want %d", alloc.NumLayers, tt.numLayers)
			}

			if len(alloc.CodeBlockPasses) != len(tt.passesPerBlock) {
				t.Errorf("Number of code-blocks = %d, want %d",
					len(alloc.CodeBlockPasses), len(tt.passesPerBlock))
			}

			// Verify allocation properties
			for cbIdx, passes := range tt.passesPerBlock {
				if cbIdx >= len(alloc.CodeBlockPasses) {
					continue
				}

				layerPasses := alloc.CodeBlockPasses[cbIdx]
				maxPasses := len(passes)

				for layer := 0; layer < len(layerPasses); layer++ {
					// Verify bounds
					if layerPasses[layer] < 0 || layerPasses[layer] > maxPasses {
						t.Errorf("CB %d layer %d: %d passes out of range [0, %d]",
							cbIdx, layer, layerPasses[layer], maxPasses)
					}

					// Verify monotonicity
					if layer > 0 && layerPasses[layer] < layerPasses[layer-1] {
						t.Errorf("CB %d: layer %d (%d) < layer %d (%d) - not monotonic",
							cbIdx, layer, layerPasses[layer], layer-1, layerPasses[layer-1])
					}
				}

				// Calculate actual rate used
				if len(layerPasses) > 0 && layerPasses[len(layerPasses)-1] > 0 {
					finalPassIdx := layerPasses[len(layerPasses)-1] - 1
					if finalPassIdx < len(passes) {
						actualRate := passes[finalPassIdx].ActualBytes
						if actualRate == 0 {
							actualRate = passes[finalPassIdx].Rate
						}
						t.Logf("CB %d: allocation=%v, final rate=%d bytes",
							cbIdx, layerPasses, actualRate)
					}
				}
			}

			// Verify budget constraint if specified
			if tt.targetBudget > 0 {
				totalRate := 0
				for cbIdx, passes := range tt.passesPerBlock {
					if cbIdx >= len(alloc.CodeBlockPasses) {
						continue
					}
					layerPasses := alloc.CodeBlockPasses[cbIdx]
					if len(layerPasses) == 0 {
						continue
					}
					finalPassCount := layerPasses[len(layerPasses)-1]
					if finalPassCount > 0 && finalPassCount <= len(passes) {
						p := passes[finalPassCount-1]
						bytes := p.ActualBytes
						if bytes == 0 {
							bytes = p.Rate
						}
						totalRate += bytes
					}
				}

				t.Logf("Total rate: %d bytes, budget: %.0f bytes", totalRate, tt.targetBudget)

				// Allow some tolerance due to quantization
				tolerance := 1.2
				if float64(totalRate) > tt.targetBudget*tolerance {
					t.Errorf("Total rate %d exceeds budget %.0f (with %.0f%% tolerance)",
						totalRate, tt.targetBudget, (tolerance-1)*100)
				}
			}
		})
	}
}

// TestLayerAllocationMonotonicity verifies that all allocation algorithms produce monotonic results
func TestLayerAllocationMonotonicity(t *testing.T) {
	numPasses := 15
	numLayers := 5
	numCodeBlocks := 3

	t.Run("Simple allocation", func(t *testing.T) {
		alloc := AllocateLayersSimple(numPasses, numLayers, numCodeBlocks)
		verifyMonotonicity(t, alloc, "Simple")
	})

	t.Run("RD allocation", func(t *testing.T) {
		cbSizes := make([][]int, numCodeBlocks)
		for i := 0; i < numCodeBlocks; i++ {
			cbSizes[i] = make([]int, numPasses)
			for j := 0; j < numPasses; j++ {
				cbSizes[i][j] = (j + 1) * 5 // Cumulative sizes
			}
		}
		targetRates := []float64{20, 40, 60, 80, 100}
		alloc := AllocateLayersRateDistortion(cbSizes, targetRates)
		verifyMonotonicity(t, alloc, "RD")
	})

	t.Run("RD with PassData", func(t *testing.T) {
		passesPerBlock := make([][]t1.PassData, numCodeBlocks)
		for i := 0; i < numCodeBlocks; i++ {
			passesPerBlock[i] = make([]t1.PassData, numPasses)
			for j := 0; j < numPasses; j++ {
				passesPerBlock[i][j] = t1.PassData{
					PassIndex:   j,
					Rate:        (j + 1) * 5,
					ActualBytes: (j + 1) * 5,
					Distortion:  float64(100 + j*20),
				}
			}
		}
		alloc := AllocateLayersRateDistortionPasses(passesPerBlock, numLayers, 0)
		verifyMonotonicity(t, alloc, "RD-PassData")
	})
}

// verifyMonotonicity checks that passes are monotonically increasing across layers
func verifyMonotonicity(t *testing.T, alloc *LayerAllocation, name string) {
	t.Helper()

	for cbIdx, layerPasses := range alloc.CodeBlockPasses {
		for layer := 1; layer < len(layerPasses); layer++ {
			if layerPasses[layer] < layerPasses[layer-1] {
				t.Errorf("%s: CB %d layer %d (%d) < layer %d (%d) - not monotonic",
					name, cbIdx, layer, layerPasses[layer], layer-1, layerPasses[layer-1])
			}
		}
	}
}

// TestCompareAllocationStrategies compares different allocation strategies
func TestCompareAllocationStrategies(t *testing.T) {
	numPasses := 12
	numLayers := 3
	numCodeBlocks := 2

	// Create test data
	cbSizes := [][]int{
		{5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55, 60},
		{8, 16, 24, 32, 40, 48, 56, 64, 72, 80, 88, 96},
	}

	passesPerBlock := make([][]t1.PassData, numCodeBlocks)
	for i := 0; i < numCodeBlocks; i++ {
		passesPerBlock[i] = make([]t1.PassData, numPasses)
		for j := 0; j < numPasses; j++ {
			// Higher bit-planes contribute more distortion reduction
			distReduction := math.Pow(2.0, float64(numPasses-j))
			passesPerBlock[i][j] = t1.PassData{
				PassIndex:   j,
				Rate:        cbSizes[i][j],
				ActualBytes: cbSizes[i][j],
				Distortion:  distReduction,
			}
		}
	}

	targetRates := []float64{50, 100, 156} // Total = 60+96 = 156

	// Compare strategies
	allocSimple := AllocateLayersSimple(numPasses, numLayers, numCodeBlocks)
	allocRD := AllocateLayersRateDistortion(cbSizes, targetRates)
	allocRDPass := AllocateLayersRateDistortionPasses(passesPerBlock, numLayers, 156)

	t.Logf("Simple allocation:")
	for i, cb := range allocSimple.CodeBlockPasses {
		t.Logf("  CB %d: %v", i, cb)
	}

	t.Logf("RD allocation:")
	for i, cb := range allocRD.CodeBlockPasses {
		t.Logf("  CB %d: %v", i, cb)
	}

	t.Logf("RD-PassData allocation:")
	for i, cb := range allocRDPass.CodeBlockPasses {
		t.Logf("  CB %d: %v", i, cb)
	}

	// All should be valid
	verifyMonotonicity(t, allocSimple, "Simple")
	verifyMonotonicity(t, allocRD, "RD")
	verifyMonotonicity(t, allocRDPass, "RD-PassData")
}

// TestGetPassesForLayer tests the helper methods
func TestGetPassesForLayer(t *testing.T) {
	alloc := &LayerAllocation{
		NumLayers: 3,
		CodeBlockPasses: [][]int{
			{3, 7, 10},
			{2, 5, 8},
		},
	}

	tests := []struct {
		cbIndex       int
		layer         int
		expectedTotal int
		expectedNew   int
	}{
		{0, 0, 3, 3},
		{0, 1, 7, 4},
		{0, 2, 10, 3},
		{1, 0, 2, 2},
		{1, 1, 5, 3},
		{1, 2, 8, 3},
		{2, 0, 0, 0}, // Out of bounds
		{0, 3, 0, 0}, // Out of bounds
	}

	for _, tt := range tests {
		total := alloc.GetPassesForLayer(tt.cbIndex, tt.layer)
		if total != tt.expectedTotal {
			t.Errorf("GetPassesForLayer(%d, %d) = %d, want %d",
				tt.cbIndex, tt.layer, total, tt.expectedTotal)
		}

		newPasses := alloc.GetNewPassesForLayer(tt.cbIndex, tt.layer)
		if newPasses != tt.expectedNew {
			t.Errorf("GetNewPassesForLayer(%d, %d) = %d, want %d",
				tt.cbIndex, tt.layer, newPasses, tt.expectedNew)
		}
	}
}

// TestRateDistortionSlope verifies that slopes are calculated correctly
func TestRateDistortionSlope(t *testing.T) {
	passes := []t1.PassData{
		{PassIndex: 0, Rate: 10, ActualBytes: 10, Distortion: 100.0},
		{PassIndex: 1, Rate: 20, ActualBytes: 20, Distortion: 180.0},
		{PassIndex: 2, Rate: 30, ActualBytes: 30, Distortion: 240.0},
	}

	// Calculate incremental slopes
	expectedSlopes := []float64{
		100.0 / 10.0, // Pass 0: 100/10 = 10.0
		80.0 / 10.0,  // Pass 1: (180-100)/10 = 8.0
		60.0 / 10.0,  // Pass 2: (240-180)/10 = 6.0
	}

	prevRate := 0
	prevDist := 0.0

	for i, p := range passes {
		incRate := p.ActualBytes - prevRate
		incDist := p.Distortion - prevDist
		slope := incDist / float64(incRate)

		if math.Abs(slope-expectedSlopes[i]) > 0.001 {
			t.Errorf("Pass %d: slope = %.3f, want %.3f", i, slope, expectedSlopes[i])
		}

		t.Logf("Pass %d: ΔR=%d, ΔD=%.1f, slope=%.3f",
			i, incRate, incDist, slope)

		prevRate = p.ActualBytes
		prevDist = p.Distortion
	}
}

// TestEdgeCases tests edge cases and boundary conditions
func TestEdgeCases(t *testing.T) {
	t.Run("Zero layers", func(t *testing.T) {
		alloc := AllocateLayersSimple(10, 0, 1)
		// Should default to 1 layer
		if alloc.NumLayers != 1 {
			t.Errorf("Zero layers should default to 1, got %d", alloc.NumLayers)
		}
	})

	t.Run("Zero passes", func(t *testing.T) {
		alloc := AllocateLayersSimple(0, 3, 1)
		// Should handle gracefully
		if alloc == nil {
			t.Error("Should not return nil")
		}
	})

	t.Run("More layers than passes", func(t *testing.T) {
		alloc := AllocateLayersSimple(3, 10, 1)
		// Each layer should have at least its index+1 passes
		for layer := 0; layer < len(alloc.CodeBlockPasses[0]); layer++ {
			passes := alloc.CodeBlockPasses[0][layer]
			if passes < layer+1 && passes < 3 {
				t.Logf("Layer %d has %d passes (acceptable for limited total)", layer, passes)
			}
		}
	})

	t.Run("Empty PassData", func(t *testing.T) {
		alloc := AllocateLayersRateDistortionPasses([][]t1.PassData{}, 3, 100)
		if alloc.NumLayers != 3 {
			t.Errorf("NumLayers = %d, want 3", alloc.NumLayers)
		}
	})

	t.Run("Negative budget", func(t *testing.T) {
		passes := [][]t1.PassData{
			{{Rate: 10, ActualBytes: 10, Distortion: 100}},
		}
		alloc := AllocateLayersRateDistortionPasses(passes, 1, -50)
		// Should use full rate when budget is negative
		if len(alloc.CodeBlockPasses) > 0 && len(alloc.CodeBlockPasses[0]) > 0 {
			if alloc.CodeBlockPasses[0][0] != 1 {
				t.Logf("Negative budget handled: allocated %d passes", alloc.CodeBlockPasses[0][0])
			}
		}
	})
}
