package nearlossless

import (
	"fmt"
	"testing"

	"context"
	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	pixel "github.com/cocosip/go-dicom/pkg/imaging/pixel"
)

// TestAllNEARValues tests a comprehensive range of NEAR values
func TestAllNEARValues(t *testing.T) {
	c := NewJPEGLSNearLosslessCodec(2)

	width, height := 32, 32
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte((i * 7) % 256)
	}

	// Test NEAR values: 0, 1, 2, 3, 5, 7, 10, 15, 20, 50, 100
	nearValues := []int{0, 1, 2, 3, 5, 7, 10, 15, 20, 50, 100}

	for _, near := range nearValues {
		t.Run(fmt.Sprintf("NEAR=%d", near), func(t *testing.T) {
			frameInfo := &codec.FrameInfo{
				Width:  uint16(width),
				Height: uint16(height),

				SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
			}

			src := codecHelpers.NewTestPixelData(frameInfo)
			if err := src.AddFrame(context.Background(), pixelData); err != nil {
				t.Fatalf("AddFrame failed: %v", err)
			}

			params := NewNearLosslessParameters().WithNEAR(near)

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
				if diff > near {
					t.Errorf("Pixel %d: error=%d exceeds NEAR=%d", i, diff, near)
					break
				}
			}

			t.Logf("NEAR=%d: max error=%d", near, maxError)

			if near == 0 && maxError > 0 {
				t.Errorf("Lossless mode has error=%d, want 0", maxError)
			}
		})
	}
}
