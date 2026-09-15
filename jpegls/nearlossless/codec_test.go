package nearlossless

import (
	"bytes"
	"strings"
	"testing"

	"context"
	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	pixel "github.com/cocosip/go-dicom/pkg/imaging/pixel"
)

func TestCodecRGBUsesSampleInterleave(t *testing.T) {
	const (
		width  = 4
		height = 3
		near   = 2
	)

	pixelData := make([]byte, width*height*3)
	for i := range pixelData {
		pixelData[i] = byte(i * 13)
	}
	frameInfo := &codec.FrameInfo{
		Width:  width,
		Height: height,

		SamplesPerPixel: 3, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation("RGB"),
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}
	params := NewNearLosslessParameters().WithNEAR(near)
	encoded := codecHelpers.NewTestPixelData(frameInfo)
	if err := NewJPEGLSNearLosslessCodec(near).Encode(context.Background(), src, encoded, params); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	frame, err := encoded.Frame(context.Background(), 0)
	if err != nil {
		t.Fatalf("GetFrame failed: %v", err)
	}

	sos := bytes.Index(frame, []byte{0xFF, 0xDA})
	if sos < 0 || len(frame) < sos+14 {
		t.Fatalf("encoded frame does not contain a complete SOS segment")
	}
	if got := frame[sos+11]; got != near {
		t.Errorf("SOS NEAR = %d, want %d", got, near)
	}
	if got := frame[sos+12]; got != 2 {
		t.Errorf("SOS ILV = %d, want 2 (sample interleaved)", got)
	}
}

func TestDefaultEncodeOmitsLSEPresetParameters(t *testing.T) {
	encoded, err := Encode(make([]byte, 8*8), 8, 8, 1, 8, 3)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if bytes.Contains(encoded, []byte{0xff, 0xf8}) {
		t.Fatal("Encode() emitted an LSE preset-parameter segment for standard defaults")
	}
}

func TestEncodeEndsAtEOI(t *testing.T) {
	encoded, err := Encode([]byte{0, 8, 16, 24, 32, 40}, 3, 2, 1, 8, 3)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if len(encoded) < 2 || !bytes.Equal(encoded[len(encoded)-2:], []byte{0xff, 0xd9}) {
		t.Errorf("raw JPEG-LS output must end at EOI, got suffix %x", encoded[max(0, len(encoded)-4):])
	}
}

// TestCodecInterface verifies that Codec implements codec.Codec
func TestCodecInterface(_ *testing.T) {
	var _ codec.Codec = (*JPEGLSNearLosslessCodec)(nil)
}

func TestDefaultCodecUsesFoDicomNearLosslessErrorBound(t *testing.T) {
	params := NewNearLosslessParameters()
	if params.NEAR != 3 {
		t.Fatalf("default NEAR = %d, want 3 to match fo-dicom", params.NEAR)
	}
}

// TestCodecRegistration tests that the codec is registered in the global registry
func TestCodecRegistration(t *testing.T) {
	// Ensure codec is registered
	RegisterJPEGLSNearLosslessCodec(3)

	// Retrieve from global registry
	registry := codec.GlobalRegistry()
	c, exists := registry.Lookup(transfer.JPEGLSNearLossless)
	if !exists {
		t.Fatal("Codec not found in registry")
	}

	// Basic checks
	if !strings.HasPrefix(c.Name(), "JPEG-LS Near-Lossless") {
		t.Errorf("Unexpected codec name: %s", c.Name())
	}
	if c.TransferSyntax().UID().UID() != transfer.JPEGLSNearLossless.UID().UID() {
		t.Errorf("Unexpected UID: %s", c.TransferSyntax().UID().UID())
	}
}

// TestCodecUID tests the UID method
func TestCodecUID(t *testing.T) {
	c := NewJPEGLSNearLosslessCodec(2)
	expectedUID := transfer.JPEGLSNearLossless.UID().UID()
	if c.TransferSyntax().UID().UID() != expectedUID {
		t.Errorf("UID() = %s, want %s", c.TransferSyntax().UID().UID(), expectedUID)
	}
}

// TestCodecName tests the Name method
func TestCodecName(t *testing.T) {
	c := NewJPEGLSNearLosslessCodec(2)
	if !strings.HasPrefix(c.Name(), "JPEG-LS Near-Lossless") {
		t.Errorf("Name() unexpected: %s", c.Name())
	}
}

// TestOptionsValidate tests the Options.Validate method
// Validate NEAR parameter handling via codec.Parameters
func TestParameterNearValues(t *testing.T) {
	c := NewJPEGLSNearLosslessCodec(2)

	width, height := 32, 32
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	// Create source PixelData
	frameInfo := &codec.FrameInfo{
		Width:  uint16(width),
		Height: uint16(height),

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	cases := []struct {
		name    string
		near    int
		wantErr bool
	}{
		{"NEAR=0", 0, false},
		{"NEAR=3", 3, false},
		{"NEAR=255", 255, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params := NewNearLosslessParameters().WithNEAR(tc.near)
			encoded := codecHelpers.NewTestPixelData(frameInfo)
			err := c.Encode(context.Background(), src, encoded, params)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Encode error=%v, wantErr=%v", err, tc.wantErr)
			}
			frame, frameErr := encoded.Frame(context.Background(), 0)
			if !tc.wantErr && (frameErr != nil || len(frame) == 0) {
				t.Error("encoded data is empty")
			}
		})
	}
}

// TestCodecEncode tests encoding through the Codec interface
func TestCodecEncode(t *testing.T) {
	c := NewJPEGLSNearLosslessCodec(2)

	width, height := 64, 64
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	// Base frameInfo
	baseFrameInfo := &codec.FrameInfo{
		Width:  uint16(width),
		Height: uint16(height),

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
	}

	tests := []struct {
		name    string
		mutate  func(*codec.FrameInfo) (*codec.FrameInfo, codec.Parameters)
		wantErr bool
	}{
		{
			name: "Valid NEAR=0 (lossless)",
			mutate: func(fi *codec.FrameInfo) (*codec.FrameInfo, codec.Parameters) {
				p := NewNearLosslessParameters().WithNEAR(0)
				return fi, p
			},
			wantErr: false,
		},
		{
			name: "Valid NEAR=3",
			mutate: func(fi *codec.FrameInfo) (*codec.FrameInfo, codec.Parameters) {
				p := NewNearLosslessParameters().WithNEAR(3)
				return fi, p
			},
			wantErr: false,
		},
		{
			name: "Invalid width",
			mutate: func(fi *codec.FrameInfo) (*codec.FrameInfo, codec.Parameters) {
				newFi := *fi
				newFi.Width = 0
				return &newFi, nil
			},
			wantErr: true,
		},
		{
			name: "Invalid components",
			mutate: func(fi *codec.FrameInfo) (*codec.FrameInfo, codec.Parameters) {
				newFi := *fi
				newFi.SamplesPerPixel = 2
				return &newFi, nil
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// copy frameInfo to avoid mutation leakage
			frameInfo, params := tt.mutate(baseFrameInfo)
			src := codecHelpers.NewTestPixelData(frameInfo)
			if err := src.AddFrame(context.Background(), pixelData); err != nil {
				t.Fatalf("AddFrame failed: %v", err)
			}
			encoded := codecHelpers.NewTestPixelData(frameInfo)
			err := c.Encode(context.Background(), src, encoded, params)
			if (err != nil) != tt.wantErr {
				t.Errorf("Encode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			frame, frameErr := encoded.Frame(context.Background(), 0)
			if !tt.wantErr && (frameErr != nil || len(frame) == 0) {
				t.Error("Encode() returned empty data")
			}
		})
	}
}

// TestCodecDecode tests decoding through the Codec interface
func TestCodecDecode(t *testing.T) {
	c := NewJPEGLSNearLosslessCodec(2)

	width, height := 32, 32
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte((i * 7) % 256)
	}

	// Create source PixelData
	frameInfo := &codec.FrameInfo{
		Width:  uint16(width),
		Height: uint16(height),

		SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(context.Background(), pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	params := NewNearLosslessParameters().WithNEAR(3)

	encoded := codecHelpers.NewTestPixelData(frameInfo)
	if err := c.Encode(context.Background(), src, encoded, params); err != nil {
		t.Fatalf("Encode() failed: %v", err)
	}

	decoded := codecHelpers.NewTestPixelData(frameInfo)
	if err := c.Decode(context.Background(), encoded, decoded, nil); err != nil {
		t.Fatalf("Decode() failed: %v", err)
	}

	decodedInfo := decoded.FrameInfo()
	if int(decodedInfo.Width) != width || int(decodedInfo.Height) != height {
		t.Errorf("Decoded dimensions mismatch")
	}
	if decodedInfo.SamplesPerPixel != 1 || decodedInfo.BitDepth.BitsStored != 8 {
		t.Errorf("Decoded metadata mismatch")
	}

	decodedFrame, err := decoded.Frame(context.Background(), 0)
	if err != nil {
		t.Fatalf("GetFrame failed: %v", err)
	}
	// Verify error bound (NEAR=3)
	maxError := 0
	for i := 0; i < len(pixelData); i++ {
		diff := int(decodedFrame[i]) - int(pixelData[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError {
			maxError = diff
		}
	}
	if maxError > 3 {
		t.Errorf("Decode() max error = %d, want <= 3", maxError)
	}
}

// TestCodecDecodeInvalid tests decoding with invalid input
func TestCodecDecodeInvalid(t *testing.T) {
	c := NewJPEGLSNearLosslessCodec(2)

	tests := []struct {
		name string
		data []byte
	}{
		{"Empty data", []byte{}},
		{"Invalid data", []byte{0x00, 0x01, 0x02, 0x03}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frameInfo := &codec.FrameInfo{
				Width:  32,
				Height: 32,

				SamplesPerPixel: 1, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricMonochrome2),
			}
			srcEnc := codecHelpers.NewTestPixelData(frameInfo)
			if err := srcEnc.AddFrame(context.Background(), tt.data); err != nil {
				t.Fatalf("AddFrame failed: %v", err)
			}
			dst := codecHelpers.NewTestPixelData(frameInfo)
			err := c.Decode(context.Background(), srcEnc, dst, nil)
			if err == nil {
				t.Error("Decode() expected error, got nil")
			}
		})
	}
}

// TestCodecRoundTrip tests encoding and decoding round-trip
func TestCodecRoundTrip(t *testing.T) {
	c := NewJPEGLSNearLosslessCodec(2)

	tests := []struct {
		name       string
		width      int
		height     int
		components int
		near       int
	}{
		{"Grayscale NEAR=0", 32, 32, 1, 0},
		{"Grayscale NEAR=1", 32, 32, 1, 1},
		{"Grayscale NEAR=3", 32, 32, 1, 3},
		{"Grayscale NEAR=5", 32, 32, 1, 5},
		{"RGB NEAR=3", 16, 16, 3, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := tt.width * tt.height * tt.components
			pixelData := make([]byte, size)
			for i := range pixelData {
				pixelData[i] = byte((i * 7) % 256)
			}

			// Create source PixelData
			frameInfo := &codec.FrameInfo{
				Width:  uint16(tt.width),
				Height: uint16(tt.height),

				SamplesPerPixel: uint16(tt.components), BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(map[int]string{1: photometricMonochrome2, 3: "RGB"}[tt.components]),
			}
			src := codecHelpers.NewTestPixelData(frameInfo)
			if err := src.AddFrame(context.Background(), pixelData); err != nil {
				t.Fatalf("AddFrame failed: %v", err)
			}

			params := NewNearLosslessParameters().WithNEAR(tt.near)

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
				if diff > tt.near {
					t.Errorf("Pixel %d: error=%d exceeds NEAR=%d", i, diff, tt.near)
					break
				}
			}

			if tt.near == 0 && maxError > 0 {
				t.Errorf("Lossless mode has error=%d, want 0", maxError)
			}
		})
	}
}
