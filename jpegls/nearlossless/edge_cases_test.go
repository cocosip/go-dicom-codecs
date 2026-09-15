package nearlossless

import (
	"testing"

	"context"
	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	pixel "github.com/cocosip/go-dicom/pkg/imaging/pixel"
)

// TestSinglePixelImage tests encoding/decoding of a 1x1 image
func TestSinglePixelImage(t *testing.T) {
	c := NewJPEGLSNearLosslessCodec(2)

	testCases := []struct {
		name  string
		near  int
		pixel byte
	}{
		{"NEAR=0 pixel=0", 0, 0},
		{"NEAR=0 pixel=127", 0, 127},
		{"NEAR=0 pixel=255", 0, 255},
		{"NEAR=3 pixel=100", 3, 100},
		{"NEAR=5 pixel=200", 5, 200},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pixelData := []byte{tc.pixel}

			frameInfo := &codec.FrameInfo{
				Width:  1,
				Height: 1,

				SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
			}

			src := codecHelpers.NewTestPixelData(frameInfo)
			if err := src.AddFrame(context.Background(), pixelData); err != nil {
				t.Fatalf("AddFrame failed: %v", err)
			}

			params := NewNearLosslessParameters().WithNEAR(tc.near)

			encoded := codecHelpers.NewTestPixelData(frameInfo)
			if err := c.Encode(context.Background(), src, encoded, params); err != nil {
				t.Fatalf("Encode() failed: %v", err)
			}

			decoded := codecHelpers.NewTestPixelData(frameInfo)
			if err := c.Decode(context.Background(), encoded, decoded, nil); err != nil {
				t.Fatalf("Decode() failed: %v", err)
			}

			decodedFrame, err := decoded.Frame(context.Background(), 0)
			if err != nil {
				t.Fatalf("GetFrame failed: %v", err)
			}

			diff := int(decodedFrame[0]) - int(pixelData[0])
			if diff < 0 {
				diff = -diff
			}
			if diff > tc.near {
				t.Errorf("Pixel error=%d exceeds NEAR=%d", diff, tc.near)
			}

			if tc.near == 0 && diff > 0 {
				t.Errorf("Lossless mode has error=%d, want 0", diff)
			}
		})
	}
}

// TestSingleLineImage tests encoding/decoding of a 1-row image
func TestSingleLineImage(t *testing.T) {
	c := NewJPEGLSNearLosslessCodec(2)

	testCases := []struct {
		name  string
		width int
		near  int
	}{
		{"NEAR=0 width=1", 1, 0},
		{"NEAR=0 width=10", 10, 0},
		{"NEAR=0 width=100", 100, 0},
		{"NEAR=3 width=50", 50, 3},
		{"NEAR=5 width=64", 64, 5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pixelData := make([]byte, tc.width)
			for i := range pixelData {
				pixelData[i] = byte((i * 13) % 256)
			}

			frameInfo := &codec.FrameInfo{
				Width:  uint16(tc.width),
				Height: 1,

				SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
			}

			src := codecHelpers.NewTestPixelData(frameInfo)
			if err := src.AddFrame(context.Background(), pixelData); err != nil {
				t.Fatalf("AddFrame failed: %v", err)
			}

			params := NewNearLosslessParameters().WithNEAR(tc.near)

			encoded := codecHelpers.NewTestPixelData(frameInfo)
			if err := c.Encode(context.Background(), src, encoded, params); err != nil {
				t.Fatalf("Encode() failed: %v", err)
			}

			decoded := codecHelpers.NewTestPixelData(frameInfo)
			if err := c.Decode(context.Background(), encoded, decoded, nil); err != nil {
				t.Fatalf("Decode() failed: %v", err)
			}

			decodedFrame, err := decoded.Frame(context.Background(), 0)
			if err != nil {
				t.Fatalf("GetFrame failed: %v", err)
			}

			maxError := 0
			for i := 0; i < len(pixelData); i++ {
				diff := int(decodedFrame[i]) - int(pixelData[i])
				if diff < 0 {
					diff = -diff
				}
				if diff > maxError {
					maxError = diff
				}
				if diff > tc.near {
					t.Errorf("Pixel %d: error=%d exceeds NEAR=%d", i, diff, tc.near)
					break
				}
			}

			if tc.near == 0 && maxError > 0 {
				t.Errorf("Lossless mode has error=%d, want 0", maxError)
			}
		})
	}
}

// TestSingleColumnImage tests encoding/decoding of a 1-column image
func TestSingleColumnImage(t *testing.T) {
	c := NewJPEGLSNearLosslessCodec(2)

	testCases := []struct {
		name   string
		height int
		near   int
	}{
		{"NEAR=0 height=1", 1, 0},
		{"NEAR=0 height=10", 10, 0},
		{"NEAR=0 height=100", 100, 0},
		{"NEAR=3 height=50", 50, 3},
		{"NEAR=5 height=64", 64, 5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pixelData := make([]byte, tc.height)
			for i := range pixelData {
				pixelData[i] = byte((i * 17) % 256)
			}

			frameInfo := &codec.FrameInfo{
				Width:  1,
				Height: uint16(tc.height),

				SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
			}

			src := codecHelpers.NewTestPixelData(frameInfo)
			if err := src.AddFrame(context.Background(), pixelData); err != nil {
				t.Fatalf("AddFrame failed: %v", err)
			}

			params := NewNearLosslessParameters().WithNEAR(tc.near)

			encoded := codecHelpers.NewTestPixelData(frameInfo)
			if err := c.Encode(context.Background(), src, encoded, params); err != nil {
				t.Fatalf("Encode() failed: %v", err)
			}

			decoded := codecHelpers.NewTestPixelData(frameInfo)
			if err := c.Decode(context.Background(), encoded, decoded, nil); err != nil {
				t.Fatalf("Decode() failed: %v", err)
			}

			decodedFrame, err := decoded.Frame(context.Background(), 0)
			if err != nil {
				t.Fatalf("GetFrame failed: %v", err)
			}

			maxError := 0
			for i := 0; i < len(pixelData); i++ {
				diff := int(decodedFrame[i]) - int(pixelData[i])
				if diff < 0 {
					diff = -diff
				}
				if diff > maxError {
					maxError = diff
				}
				if diff > tc.near {
					t.Errorf("Pixel %d: error=%d exceeds NEAR=%d", i, diff, tc.near)
					break
				}
			}

			if tc.near == 0 && maxError > 0 {
				t.Errorf("Lossless mode has error=%d, want 0", maxError)
			}
		})
	}
}
