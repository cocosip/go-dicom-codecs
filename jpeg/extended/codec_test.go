package extended

import (
	"testing"

	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
	codecHelpers "github.com/cocosip/go-dicom-codec/codec"
)

// TestExtendedCodecInterface tests codec interface implementation
func TestExtendedCodecInterface(t *testing.T) {
	c := NewExtendedCodec(12, 85)

	// Test interface methods
	if c.Name() == "" {
		t.Error("Name() returned empty string")
	}
	t.Logf("Codec name: %s", c.Name())

	if c.TransferSyntax().UID().UID() != transfer.JPEGExtended12Bit.UID().UID() {
		t.Errorf("UID mismatch: got %s, want %s", c.TransferSyntax().UID().UID(), transfer.JPEGExtended12Bit.UID().UID())
	}
}

// TestExtendedCodecEncodeDecode8Bit tests 8-bit codec encode/decode
func TestExtendedCodecEncodeDecode8Bit(t *testing.T) {
	width, height := 64, 64
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte((i * 3) % 256)
	}

	// Create frame info with metadata
	frameInfo := &imagetypes.FrameInfo{
		Width:                     uint16(width),
		Height:                    uint16(height),
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
	}

	// Create source PixelData
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	extCodec := NewExtendedCodec(8, 85)

	// Create encoded PixelData
	encodedFrameInfo := &imagetypes.FrameInfo{
		Width:                     uint16(width),
		Height:                    uint16(height),
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
	}
	encoded := codecHelpers.NewTestPixelData(encodedFrameInfo)

	// Encode
	err := extCodec.Encode(src, encoded, nil)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Get the encoded frame data for size logging
	encodedData, _ := encoded.GetFrame(0)
	srcData, _ := src.GetFrame(0)
	t.Logf("Original size: %d bytes", len(srcData))
	t.Logf("Compressed size: %d bytes", len(encodedData))
	t.Logf("Compression ratio: %.2fx", float64(len(srcData))/float64(len(encodedData)))

	// Create decoded PixelData
	decodedFrameInfo := &imagetypes.FrameInfo{
		Width:                     uint16(width),
		Height:                    uint16(height),
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
	}
	decoded := codecHelpers.NewTestPixelData(decodedFrameInfo)

	// Decode
	err = extCodec.Decode(encoded, decoded, nil)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify dimensions
	decodedInfo := decoded.GetFrameInfo()
	if decodedInfo.Width != frameInfo.Width || decodedInfo.Height != frameInfo.Height {
		t.Errorf("Dimension mismatch: got %dx%d, want %dx%d",
			decodedInfo.Width, decodedInfo.Height, frameInfo.Width, frameInfo.Height)
	}

	// Check lossy quality
	decodedData, _ := decoded.GetFrame(0)
	maxDiff := 0
	totalDiff := 0
	for i := 0; i < len(pixelData); i++ {
		diff := int(pixelData[i]) - int(decodedData[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxDiff {
			maxDiff = diff
		}
		totalDiff += diff
	}

	avgDiff := float64(totalDiff) / float64(len(pixelData))
	t.Logf("Max pixel difference: %d", maxDiff)
	t.Logf("Average pixel difference: %.2f", avgDiff)

	t.Log("Lossy compression test passed (JPEG Extended is lossy)")
}

// TestExtendedCodecEncodeDecode12Bit tests 12-bit codec encode/decode
func TestExtendedCodecEncodeDecode12Bit(t *testing.T) {
	width, height := 64, 64
	pixelData := make([]byte, width*height*2) // 12-bit stored in 16-bit

	// Create 12-bit test data
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			val := ((x + y*2) * 16) % 4096 // 12-bit value
			idx := (y*width + x) * 2
			pixelData[idx] = byte(val & 0xFF)
			pixelData[idx+1] = byte((val >> 8) & 0xFF)
		}
	}

	// Create frame info with metadata
	frameInfo := &imagetypes.FrameInfo{
		Width:                     uint16(width),
		Height:                    uint16(height),
		BitsAllocated:             16,
		BitsStored:                12,
		HighBit:                   11,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
	}

	// Create source PixelData
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	extCodec := NewExtendedCodec(12, 85)

	// Create encoded PixelData
	encodedFrameInfo := &imagetypes.FrameInfo{
		Width:                     uint16(width),
		Height:                    uint16(height),
		BitsAllocated:             16,
		BitsStored:                12,
		HighBit:                   11,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
	}
	encoded := codecHelpers.NewTestPixelData(encodedFrameInfo)

	// Encode
	err := extCodec.Encode(src, encoded, nil)
	if err != nil {
		t.Fatalf("12-bit Encode failed: %v", err)
	}

	// Get frame data for size logging
	encodedData, _ := encoded.GetFrame(0)
	srcData, _ := src.GetFrame(0)
	t.Logf("12-bit Original size: %d bytes", len(srcData))
	t.Logf("12-bit Compressed size: %d bytes", len(encodedData))
	t.Logf("12-bit Compression ratio: %.2fx", float64(len(srcData))/float64(len(encodedData)))

	// Create decoded PixelData
	decodedFrameInfo := &imagetypes.FrameInfo{
		Width:                     uint16(width),
		Height:                    uint16(height),
		BitsAllocated:             16,
		BitsStored:                12,
		HighBit:                   11,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
	}
	decoded := codecHelpers.NewTestPixelData(decodedFrameInfo)

	// Decode
	err = extCodec.Decode(encoded, decoded, nil)
	if err != nil {
		t.Fatalf("12-bit Decode failed: %v", err)
	}

	// Verify dimensions
	decodedInfo := decoded.GetFrameInfo()
	if decodedInfo.Width != frameInfo.Width || decodedInfo.Height != frameInfo.Height {
		t.Errorf("12-bit Dimension mismatch: got %dx%d, want %dx%d",
			decodedInfo.Width, decodedInfo.Height, frameInfo.Width, frameInfo.Height)
	}

	// Check lossy quality (12-bit)
	decodedData, _ := decoded.GetFrame(0)
	maxDiff := 0
	totalDiff := 0
	numPixels := width * height
	for i := 0; i < numPixels; i++ {
		idx := i * 2
		origVal := int(pixelData[idx]) | (int(pixelData[idx+1]) << 8)
		decVal := int(decodedData[idx]) | (int(decodedData[idx+1]) << 8)

		diff := origVal - decVal
		if diff < 0 {
			diff = -diff
		}
		if diff > maxDiff {
			maxDiff = diff
		}
		totalDiff += diff
	}

	avgDiff := float64(totalDiff) / float64(numPixels)
	t.Logf("12-bit Max pixel difference: %d", maxDiff)
	t.Logf("12-bit Average pixel difference: %.2f", avgDiff)

	t.Log("12-bit lossy compression test passed")
}

// TestExtendedCodecRGB tests RGB encoding/decoding via codec interface
func TestExtendedCodecRGB(t *testing.T) {
	width, height := 32, 32
	components := 3
	pixelData := make([]byte, width*height*components)

	// Create RGB test data
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * components
			pixelData[offset+0] = byte(x * 8)       // R
			pixelData[offset+1] = byte(y * 8)       // G
			pixelData[offset+2] = byte((x + y) * 4) // B
		}
	}

	// Create frame info with metadata
	frameInfo := &imagetypes.FrameInfo{
		Width:                     uint16(width),
		Height:                    uint16(height),
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           uint16(components),
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "RGB",
	}

	// Create source PixelData
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	extCodec := NewExtendedCodec(8, 85)

	// Create encoded PixelData
	encodedFrameInfo := &imagetypes.FrameInfo{
		Width:                     uint16(width),
		Height:                    uint16(height),
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           uint16(components),
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "RGB",
	}
	encoded := codecHelpers.NewTestPixelData(encodedFrameInfo)

	// Encode
	err := extCodec.Encode(src, encoded, nil)
	if err != nil {
		t.Fatalf("RGB Encode failed: %v", err)
	}

	// Get frame data for size logging
	encodedData, _ := encoded.GetFrame(0)
	srcData, _ := src.GetFrame(0)
	t.Logf("RGB Original size: %d bytes", len(srcData))
	t.Logf("RGB Compressed size: %d bytes", len(encodedData))
	t.Logf("RGB Compression ratio: %.2fx", float64(len(srcData))/float64(len(encodedData)))

	// Create decoded PixelData
	decodedFrameInfo := &imagetypes.FrameInfo{
		Width:                     uint16(width),
		Height:                    uint16(height),
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           uint16(components),
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "RGB",
	}
	decoded := codecHelpers.NewTestPixelData(decodedFrameInfo)

	// Decode
	err = extCodec.Decode(encoded, decoded, nil)
	if err != nil {
		t.Fatalf("RGB Decode failed: %v", err)
	}

	// Verify dimensions
	decodedInfo := decoded.GetFrameInfo()
	if decodedInfo.Width != frameInfo.Width || decodedInfo.Height != frameInfo.Height {
		t.Errorf("RGB Dimension mismatch: got %dx%d, want %dx%d",
			decodedInfo.Width, decodedInfo.Height, frameInfo.Width, frameInfo.Height)
	}

	if decodedInfo.SamplesPerPixel != frameInfo.SamplesPerPixel {
		t.Errorf("RGB SamplesPerPixel mismatch: got %d, want %d",
			decodedInfo.SamplesPerPixel, frameInfo.SamplesPerPixel)
	}

	// Check lossy quality
	decodedData, _ := decoded.GetFrame(0)
	maxDiff := 0
	for i := 0; i < len(pixelData); i++ {
		diff := int(pixelData[i]) - int(decodedData[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxDiff {
			maxDiff = diff
		}
	}

	t.Logf("RGB Max pixel difference: %d", maxDiff)
	t.Log("RGB lossy compression completed")
}

// TestExtendedCodecWithParameters tests parameter override
func TestExtendedCodecWithParameters(t *testing.T) {
	width, height := 32, 32
	pixelData := make([]byte, width*height)
	for i := range pixelData {
		pixelData[i] = byte(i % 256)
	}

	// Create frame info with metadata
	frameInfo := &imagetypes.FrameInfo{
		Width:                     uint16(width),
		Height:                    uint16(height),
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
	}

	// Create source PixelData
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	extCodec := NewExtendedCodec(8, 50)

	// Test with parameter override (quality 95)
	params := codec.NewBaseParameters()
	params.SetParameter("quality", 95)

	// Create encoded PixelData
	encodedFrameInfo := &imagetypes.FrameInfo{
		Width:                     uint16(width),
		Height:                    uint16(height),
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
	}
	encoded := codecHelpers.NewTestPixelData(encodedFrameInfo)

	err := extCodec.Encode(src, encoded, params)
	if err != nil {
		t.Fatalf("Encode with parameters failed: %v", err)
	}

	// Get frame data for size logging
	encodedData, _ := encoded.GetFrame(0)
	t.Logf("Compressed with quality 95: %d bytes", len(encodedData))

	// Create decoded PixelData
	decodedFrameInfo := &imagetypes.FrameInfo{
		Width:                     uint16(width),
		Height:                    uint16(height),
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
	}
	decoded := codecHelpers.NewTestPixelData(decodedFrameInfo)

	err = extCodec.Decode(encoded, decoded, nil)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Higher quality should result in smaller differences
	decodedData, _ := decoded.GetFrame(0)
	maxDiff := 0
	for i := 0; i < len(pixelData); i++ {
		diff := int(pixelData[i]) - int(decodedData[i])
		if diff < 0 {
			diff = -diff
		}
		if diff > maxDiff {
			maxDiff = diff
		}
	}

	t.Logf("Max difference with quality 95: %d", maxDiff)
	t.Log("Parameters override test completed")
}

// TestExtendedCodecRegistry tests codec registry integration
func TestExtendedCodecRegistry(t *testing.T) {
	// Register codec
	RegisterExtendedCodec(12, 85)

	// Get codec from registry
	registry := codec.GetGlobalRegistry()
	c, exists := registry.GetCodec(transfer.JPEGExtended12Bit)
	if !exists {
		t.Fatal("Codec not found in registry")
	}

	t.Logf("Retrieved codec name: %s", c.Name())

	// Test encode/decode via registry
	width, height := 32, 32
	pixelData := make([]byte, width*height*2)
	for i := 0; i < width*height; i++ {
		val := (i * 16) % 4096
		pixelData[i*2] = byte(val & 0xFF)
		pixelData[i*2+1] = byte((val >> 8) & 0xFF)
	}

	// Create frame info with metadata
	frameInfo := &imagetypes.FrameInfo{
		Width:                     uint16(width),
		Height:                    uint16(height),
		BitsAllocated:             16,
		BitsStored:                12,
		HighBit:                   11,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
	}

	// Create source PixelData
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	// Create encoded PixelData
	encodedFrameInfo := &imagetypes.FrameInfo{
		Width:                     uint16(width),
		Height:                    uint16(height),
		BitsAllocated:             16,
		BitsStored:                12,
		HighBit:                   11,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
	}
	encoded := codecHelpers.NewTestPixelData(encodedFrameInfo)

	if err := c.Encode(src, encoded, nil); err != nil {
		t.Fatalf("Registry codec encode failed: %v", err)
	}

	// Create decoded PixelData
	decodedFrameInfo := &imagetypes.FrameInfo{
		Width:                     uint16(width),
		Height:                    uint16(height),
		BitsAllocated:             16,
		BitsStored:                12,
		HighBit:                   11,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
	}
	decoded := codecHelpers.NewTestPixelData(decodedFrameInfo)

	if err := c.Decode(encoded, decoded, nil); err != nil {
		t.Fatalf("Registry codec decode failed: %v", err)
	}

	decodedInfo := decoded.GetFrameInfo()
	t.Logf("Registry codec test passed: %dx%d image", decodedInfo.Width, decodedInfo.Height)
}

func TestExtendedCodecRejects16Bit(t *testing.T) {
	width, height := 16, 16
	pixelData := make([]byte, width*height*2)

	frameInfo := &imagetypes.FrameInfo{
		Width:                     uint16(width),
		Height:                    uint16(height),
		BitsAllocated:             16,
		BitsStored:                16,
		HighBit:                   15,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
	}

	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(pixelData); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}

	encodedFrameInfo := &imagetypes.FrameInfo{
		Width:                     uint16(width),
		Height:                    uint16(height),
		BitsAllocated:             16,
		BitsStored:                16,
		HighBit:                   15,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "MONOCHROME2",
	}
	encoded := codecHelpers.NewTestPixelData(encodedFrameInfo)

	extCodec := NewExtendedCodec(12, 85)
	if err := extCodec.Encode(src, encoded, nil); err == nil {
		t.Fatalf("expected error when encoding 16-bit data with JPEG Extended")
	}
}

func TestExtendedCodecRejectsNilInputs(t *testing.T) {
	extCodec := NewExtendedCodec(12, 85)
	frameInfo := &imagetypes.FrameInfo{
		Width:           8,
		Height:          8,
		BitsAllocated:   8,
		BitsStored:      8,
		HighBit:         7,
		SamplesPerPixel: 1,
	}

	if err := extCodec.Encode(nil, codecHelpers.NewTestPixelData(frameInfo), nil); err == nil {
		t.Fatal("expected Encode to reject nil source")
	}
	if err := extCodec.Encode(codecHelpers.NewTestPixelData(frameInfo), nil, nil); err == nil {
		t.Fatal("expected Encode to reject nil destination")
	}
	if err := extCodec.Decode(nil, codecHelpers.NewTestPixelData(frameInfo), nil); err == nil {
		t.Fatal("expected Decode to reject nil source")
	}
	if err := extCodec.Decode(codecHelpers.NewTestPixelData(frameInfo), nil, nil); err == nil {
		t.Fatal("expected Decode to reject nil destination")
	}
}

func TestExtendedCodecRejectsNoFrames(t *testing.T) {
	extCodec := NewExtendedCodec(12, 85)
	frameInfo := &imagetypes.FrameInfo{
		Width:           8,
		Height:          8,
		BitsAllocated:   8,
		BitsStored:      8,
		HighBit:         7,
		SamplesPerPixel: 1,
	}
	src := codecHelpers.NewTestPixelData(frameInfo)
	dst := codecHelpers.NewTestPixelData(frameInfo)

	if err := extCodec.Encode(src, dst, nil); err == nil {
		t.Fatal("expected Encode to reject source with no frames")
	}
	if err := extCodec.Decode(src, dst, nil); err == nil {
		t.Fatal("expected Decode to reject source with no frames")
	}
}
