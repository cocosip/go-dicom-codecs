package openjpeg

import (
	"testing"

	"github.com/cocosip/go-dicom-codecs/jpeg2000/internal/common/codestream"
)

// TestDefaultEncodeParams tests default encoding parameters
func TestDefaultEncodeParams(t *testing.T) {
	params := DefaultEncodeParams(512, 512, 1, 8, false)

	if params.Width != 512 {
		t.Errorf("Width: got %d, want 512", params.Width)
	}

	if params.Height != 512 {
		t.Errorf("Height: got %d, want 512", params.Height)
	}

	if params.Components != 1 {
		t.Errorf("Components: got %d, want 1", params.Components)
	}

	if params.BitDepth != 8 {
		t.Errorf("BitDepth: got %d, want 8", params.BitDepth)
	}

	if params.NumLevels != 5 {
		t.Errorf("NumLevels: got %d, want 5", params.NumLevels)
	}

	if !params.Lossless {
		t.Error("Expected lossless encoding")
	}
}

// TestEncoderValidation tests parameter validation
func TestEncoderValidation(t *testing.T) {
	tests := []struct {
		name      string
		params    *EncodeParams
		wantError bool
	}{
		{
			name:      "Valid parameters",
			params:    DefaultEncodeParams(256, 256, 1, 8, false),
			wantError: false,
		},
		{
			name: "Invalid width",
			params: &EncodeParams{
				Width:           0,
				Height:          256,
				Components:      1,
				BitDepth:        8,
				NumLevels:       5,
				CodeBlockWidth:  64,
				CodeBlockHeight: 64,
				NumLayers:       1,
				EnableMCT:       true,
			},
			wantError: true,
		},
		{
			name: "Invalid components",
			params: &EncodeParams{
				Width:           256,
				Height:          256,
				Components:      0,
				BitDepth:        8,
				NumLevels:       5,
				CodeBlockWidth:  64,
				CodeBlockHeight: 64,
				NumLayers:       1,
				EnableMCT:       true,
			},
			wantError: true,
		},
		{
			name: "Invalid bit depth",
			params: &EncodeParams{
				Width:           256,
				Height:          256,
				Components:      1,
				BitDepth:        17,
				NumLevels:       5,
				CodeBlockWidth:  64,
				CodeBlockHeight: 64,
				NumLayers:       1,
				EnableMCT:       true,
			},
			wantError: true,
		},
		{
			name: "Invalid decomposition levels",
			params: &EncodeParams{
				Width:           256,
				Height:          256,
				Components:      1,
				BitDepth:        8,
				NumLevels:       7,
				CodeBlockWidth:  64,
				CodeBlockHeight: 64,
				NumLayers:       1,
				EnableMCT:       true,
			},
			wantError: true,
		},
		{
			name: "Invalid code-block size",
			params: &EncodeParams{
				Width:           256,
				Height:          256,
				Components:      1,
				BitDepth:        8,
				NumLevels:       5,
				CodeBlockWidth:  65, // Not power of 2
				CodeBlockHeight: 64,
				NumLayers:       1,
				EnableMCT:       true,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoder := NewEncoder(tt.params)
			err := encoder.validateParams()

			if tt.wantError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestEncoderBasic tests basic encoding
func TestEncoderBasic(t *testing.T) {
	tests := []struct {
		name       string
		width      int
		height     int
		components int
		bitDepth   int
		numLevels  int
	}{
		{"8x8 grayscale 0-level", 8, 8, 1, 8, 0},
		{"16x16 grayscale 1-level", 16, 16, 1, 8, 1},
		{"32x32 grayscale 2-level", 32, 32, 1, 8, 2},
		{"8x8 RGB 0-level", 8, 8, 3, 8, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test data
			numPixels := tt.width * tt.height
			componentData := make([][]int32, tt.components)
			for c := 0; c < tt.components; c++ {
				componentData[c] = make([]int32, numPixels)
				for i := 0; i < numPixels; i++ {
					componentData[c][i] = int32((i + c*10) % 256)
				}
			}

			// Create encoder
			params := DefaultEncodeParams(tt.width, tt.height, tt.components, tt.bitDepth, false)
			params.NumLevels = tt.numLevels
			encoder := NewEncoder(params)

			// Encode
			encoded, err := encoder.EncodeComponents(componentData)
			if err != nil {
				t.Fatalf("Encoding failed: %v", err)
			}

			if len(encoded) == 0 {
				t.Fatal("Encoded data is empty")
			}

			// Parse and verify codestream
			parser := codestream.NewParser(encoded)
			cs, err := parser.Parse()
			if err != nil {
				t.Fatalf("Failed to parse encoded codestream: %v", err)
			}

			// Verify SIZ segment
			if cs.SIZ == nil {
				t.Fatal("Missing SIZ segment")
			}

			if int(cs.SIZ.Xsiz) != tt.width {
				t.Errorf("Width: got %d, want %d", cs.SIZ.Xsiz, tt.width)
			}

			if int(cs.SIZ.Ysiz) != tt.height {
				t.Errorf("Height: got %d, want %d", cs.SIZ.Ysiz, tt.height)
			}

			if int(cs.SIZ.Csiz) != tt.components {
				t.Errorf("Components: got %d, want %d", cs.SIZ.Csiz, tt.components)
			}

			// Verify COD segment
			if cs.COD == nil {
				t.Fatal("Missing COD segment")
			}

			if int(cs.COD.NumberOfDecompositionLevels) != tt.numLevels {
				t.Errorf("Decomposition levels: got %d, want %d",
					cs.COD.NumberOfDecompositionLevels, tt.numLevels)
			}

			// Verify tiles
			if len(cs.Tiles) == 0 {
				t.Fatal("No tiles in codestream")
			}
		})
	}
}

// TestEncoderRoundTrip tests encode-decode round trip
func TestEncoderRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		height    int
		bitDepth  int
		numLevels int
	}{
		{"8x8 0-level", 8, 8, 8, 0},
		{"16x16 1-level", 16, 16, 8, 1},
		{"32x32 2-level", 32, 32, 8, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test data with gradient pattern
			numPixels := tt.width * tt.height
			componentData := make([][]int32, 1)
			componentData[0] = make([]int32, numPixels)

			for y := 0; y < tt.height; y++ {
				for x := 0; x < tt.width; x++ {
					idx := y*tt.width + x
					componentData[0][idx] = int32((x + y) % 256)
				}
			}

			// Encode
			params := DefaultEncodeParams(tt.width, tt.height, 1, tt.bitDepth, false)
			params.NumLevels = tt.numLevels
			encoder := NewEncoder(params)

			encoded, err := encoder.EncodeComponents(componentData)
			if err != nil {
				t.Fatalf("Encoding failed: %v", err)
			}

			// Note: Full round-trip test requires T1 encoder which is not yet implemented
			// For now, just verify the codestream is parseable
			parser := codestream.NewParser(encoded)
			_, err = parser.Parse()
			if err != nil {
				t.Fatalf("Failed to parse encoded data: %v", err)
			}
		})
	}
}

// TestEncoderMultiComponent tests multi-component encoding
func TestEncoderMultiComponent(t *testing.T) {
	width, height := 16, 16
	numPixels := width * height

	// Create RGB data
	componentData := make([][]int32, 3)
	for c := 0; c < 3; c++ {
		componentData[c] = make([]int32, numPixels)
		for i := 0; i < numPixels; i++ {
			componentData[c][i] = int32((i + c*50) % 256)
		}
	}

	// Encode
	params := DefaultEncodeParams(width, height, 3, 8, false)
	encoder := NewEncoder(params)

	encoded, err := encoder.EncodeComponents(componentData)
	if err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}

	// Verify
	parser := codestream.NewParser(encoded)
	cs, err := parser.Parse()
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if int(cs.SIZ.Csiz) != 3 {
		t.Errorf("Components: got %d, want 3", cs.SIZ.Csiz)
	}

	// Verify MCT is enabled for RGB
	if cs.COD.MultipleComponentTransform != 1 {
		t.Errorf("MCT: got %d, want 1 for RGB", cs.COD.MultipleComponentTransform)
	}
}

// TestEncoderPixelDataConversion tests pixel data conversion
func TestEncoderPixelDataConversion(t *testing.T) {
	width, height := 8, 8
	numPixels := width * height

	t.Run("8-bit grayscale", func(t *testing.T) {
		// Create 8-bit pixel data
		pixelData := make([]byte, numPixels)
		for i := range pixelData {
			pixelData[i] = byte(i % 256)
		}

		params := DefaultEncodeParams(width, height, 1, 8, false)
		encoder := NewEncoder(params)

		encoded, err := encoder.Encode(pixelData)
		if err != nil {
			t.Fatalf("Encoding failed: %v", err)
		}

		if len(encoded) == 0 {
			t.Fatal("Encoded data is empty")
		}
	})

	t.Run("16-bit grayscale", func(t *testing.T) {
		// Create 16-bit pixel data (little-endian)
		pixelData := make([]byte, numPixels*2)
		for i := 0; i < numPixels; i++ {
			val := uint16(i * 100)
			pixelData[i*2] = byte(val)
			pixelData[i*2+1] = byte(val >> 8)
		}

		params := DefaultEncodeParams(width, height, 1, 12, false)
		encoder := NewEncoder(params)

		encoded, err := encoder.Encode(pixelData)
		if err != nil {
			t.Fatalf("Encoding failed: %v", err)
		}

		if len(encoded) == 0 {
			t.Fatal("Encoded data is empty")
		}
	})

	t.Run("8-bit RGB", func(t *testing.T) {
		// Create RGB pixel data (interleaved)
		pixelData := make([]byte, numPixels*3)
		for i := 0; i < numPixels; i++ {
			pixelData[i*3] = byte(i % 256)         // R
			pixelData[i*3+1] = byte((i + 1) % 256) // G
			pixelData[i*3+2] = byte((i + 2) % 256) // B
		}

		params := DefaultEncodeParams(width, height, 3, 8, false)
		encoder := NewEncoder(params)

		encoded, err := encoder.Encode(pixelData)
		if err != nil {
			t.Fatalf("Encoding failed: %v", err)
		}

		if len(encoded) == 0 {
			t.Fatal("Encoded data is empty")
		}
	})
}

// TestEncoderDifferentBitDepths tests various bit depths
func TestEncoderDifferentBitDepths(t *testing.T) {
	width, height := 8, 8
	numPixels := width * height

	tests := []struct {
		bitDepth int
		isSigned bool
	}{
		{8, false},
		{12, false},
		{16, false},
		{8, true},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			componentData := make([][]int32, 1)
			componentData[0] = make([]int32, numPixels)

			// Fill with test pattern
			maxVal := (1 << tt.bitDepth) - 1
			for i := 0; i < numPixels; i++ {
				componentData[0][i] = int32(i % maxVal)
			}

			params := DefaultEncodeParams(width, height, 1, tt.bitDepth, tt.isSigned)
			encoder := NewEncoder(params)

			encoded, err := encoder.EncodeComponents(componentData)
			if err != nil {
				t.Fatalf("Encoding failed: %v", err)
			}

			// Verify bit depth in codestream
			parser := codestream.NewParser(encoded)
			cs, err := parser.Parse()
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			if cs.SIZ.Components[0].BitDepth() != tt.bitDepth {
				t.Errorf("Bit depth: got %d, want %d",
					cs.SIZ.Components[0].BitDepth(), tt.bitDepth)
			}

			if cs.SIZ.Components[0].IsSigned() != tt.isSigned {
				t.Errorf("Signed: got %v, want %v",
					cs.SIZ.Components[0].IsSigned(), tt.isSigned)
			}
		})
	}
}

// TestEncoderErrors tests error conditions
func TestEncoderErrors(t *testing.T) {
	t.Run("Insufficient pixel data", func(t *testing.T) {
		params := DefaultEncodeParams(16, 16, 1, 8, false)
		encoder := NewEncoder(params)

		// Only provide half the required data
		pixelData := make([]byte, 128)
		_, err := encoder.Encode(pixelData)
		if err == nil {
			t.Error("Expected error for insufficient pixel data")
		}
	})

	t.Run("Wrong component count", func(t *testing.T) {
		params := DefaultEncodeParams(8, 8, 3, 8, false)
		encoder := NewEncoder(params)

		// Provide only 1 component instead of 3
		componentData := make([][]int32, 1)
		componentData[0] = make([]int32, 64)

		_, err := encoder.EncodeComponents(componentData)
		if err == nil {
			t.Error("Expected error for wrong component count")
		}
	})

	t.Run("Wrong pixel count", func(t *testing.T) {
		params := DefaultEncodeParams(8, 8, 1, 8, false)
		encoder := NewEncoder(params)

		// Provide wrong number of pixels
		componentData := make([][]int32, 1)
		componentData[0] = make([]int32, 32) // Should be 64

		_, err := encoder.EncodeComponents(componentData)
		if err == nil {
			t.Error("Expected error for wrong pixel count")
		}
	})
}

// TestEncoderHelperFunctions tests utility functions
func TestEncoderHelperFunctions(t *testing.T) {
	t.Run("isPowerOfTwo", func(t *testing.T) {
		tests := []struct {
			n    int
			want bool
		}{
			{1, true},
			{2, true},
			{4, true},
			{8, true},
			{16, true},
			{64, true},
			{3, false},
			{5, false},
			{65, false},
			{0, false},
		}

		for _, tt := range tests {
			if got := isPowerOfTwo(tt.n); got != tt.want {
				t.Errorf("isPowerOfTwo(%d) = %v, want %v", tt.n, got, tt.want)
			}
		}
	})

	t.Run("log2", func(t *testing.T) {
		tests := []struct {
			n    int
			want int
		}{
			{1, 0},
			{2, 1},
			{4, 2},
			{8, 3},
			{16, 4},
			{64, 6},
		}

		for _, tt := range tests {
			if got := log2(tt.n); got != tt.want {
				t.Errorf("log2(%d) = %d, want %d", tt.n, got, tt.want)
			}
		}
	})
}

func TestLossyQuality100QCDHasExplicitSteps(t *testing.T) {
	width, height := 64, 64
	data := make([][]int32, 1)
	data[0] = make([]int32, width*height)
	for i := range data[0] {
		data[0][i] = int32(i % 256)
	}

	params := DefaultEncodeParams(width, height, 1, 8, false)
	params.Lossless = false
	params.Quality = 100

	encoder := NewEncoder(params)
	encoded, err := encoder.EncodeComponents(data)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	cs, err := codestream.NewParser(encoded).Parse()
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if cs.COD == nil || cs.QCD == nil {
		t.Fatalf("missing COD/QCD segments")
	}
	if got := cs.QCD.QuantizationType(); got != 2 {
		t.Fatalf("expected scalar expounded quantization (2), got %d", got)
	}

	expectedSubbands := 3*int(cs.COD.NumberOfDecompositionLevels) + 1
	expectedBytes := expectedSubbands * 2
	if got := len(cs.QCD.SPqcd); got != expectedBytes {
		t.Fatalf("unexpected SPqcd length: got %d, want %d", got, expectedBytes)
	}
}
