package extended

import (
	"bytes"
	"image"
	"image/jpeg"

	"github.com/cocosip/go-dicom-codec/jpeg/baseline"
	"github.com/cocosip/go-dicom-codec/jpeg/standard"
)

// EncodeSimple encodes using Go's standard JPEG encoder for 8-bit
// For 12-bit, we scale down to 8-bit, encode, and mark as SOF1
func EncodeSimple(pixelData []byte, width, height, components, bitDepth, quality int) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, standard.ErrInvalidDimensions
	}

	if components != 1 && components != 3 {
		return nil, standard.ErrInvalidComponents
	}

	if bitDepth != 8 && bitDepth != 12 {
		return nil, standard.ErrInvalidPrecision
	}

	if quality < 1 || quality > 100 {
		return nil, standard.ErrInvalidQuality
	}
	if bitDepth == 8 {
		return baseline.Encode(pixelData, width, height, components, quality)
	}

	// For 12-bit data, scale to 8-bit for encoding
	var img8bit []byte
	if bitDepth == 12 {
		// Convert 12-bit to 8-bit by scaling
		numPixels := width * height * components
		img8bit = make([]byte, numPixels)
		for i := 0; i < numPixels; i++ {
			idx := i * 2
			if idx+1 < len(pixelData) {
				val16 := int(pixelData[idx]) | (int(pixelData[idx+1]) << 8)
				// Scale from 12-bit (0-4095) to 8-bit (0-255)
				val8 := (val16 * 255) / 4095
				if val8 > 255 {
					val8 = 255
				}
				img8bit[i] = byte(val8)
			}
		}
	} else {
		img8bit = pixelData
	}

	// Create image
	var img image.Image
	if components == 1 {
		// Grayscale
		grayImg := image.NewGray(image.Rect(0, 0, width, height))
		copy(grayImg.Pix, img8bit)
		img = grayImg
	} else {
		// RGB
		rgbaImg := image.NewRGBA(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				srcIdx := (y*width + x) * 3
				dstIdx := (y*width + x) * 4
				rgbaImg.Pix[dstIdx+0] = img8bit[srcIdx+0] // R
				rgbaImg.Pix[dstIdx+1] = img8bit[srcIdx+1] // G
				rgbaImg.Pix[dstIdx+2] = img8bit[srcIdx+2] // B
				rgbaImg.Pix[dstIdx+3] = 255               // A
			}
		}
		img = rgbaImg
	}

	// Encode using standard library
	var buf bytes.Buffer
	opts := &jpeg.Options{Quality: quality}
	if err := jpeg.Encode(&buf, img, opts); err != nil {
		return nil, err
	}

	jpegData := buf.Bytes()

	// If 12-bit, modify SOF0 to SOF1 and update precision
	if bitDepth == 12 {
		jpegData = convertSOF0ToSOF1(jpegData, bitDepth)
	}

	return jpegData, nil
}

// convertSOF0ToSOF1 converts SOF0 marker to SOF1 and updates precision
func convertSOF0ToSOF1(data []byte, bitDepth int) []byte {
	result := make([]byte, len(data))
	copy(result, data)

	// Find and replace SOF0 (0xFFC0) with SOF1 (0xFFC1)
	for i := 0; i < len(result)-1; i++ {
		if result[i] == 0xFF && result[i+1] == 0xC0 {
			// Found SOF0
			result[i+1] = 0xC1 // Change to SOF1

			// Update precision byte (first byte after length)
			if i+4 < len(result) {
				result[i+4] = byte(bitDepth)
			}
			break
		}
	}

	return result
}

// DecodeSimple decodes using Go's standard JPEG decoder
func DecodeSimple(jpegData []byte) (pixelData []byte, width, height, components, bitDepth int, err error) {
	// Detect bit depth from SOF marker
	bitDepth = detectBitDepth(jpegData)

	// Go's standard decoder does not accept SOF1. Convert it back to SOF0
	// locally after retaining the original precision for the return value.
	if bitDepth == 12 || bytes.Contains(jpegData, []byte{0xff, 0xc1}) {
		jpegData = convertSOF1ToSOF0(jpegData)
	}

	// Decode using standard library
	img, err := jpeg.Decode(bytes.NewReader(jpegData))
	if err != nil {
		return nil, 0, 0, 0, 0, err
	}

	bounds := img.Bounds()
	width = bounds.Dx()
	height = bounds.Dy()

	// Extract pixel data
	switch imgTyped := img.(type) {
	case *image.Gray:
		components = 1
		pixelData = make([]byte, width*height)
		copy(pixelData, imgTyped.Pix)

	case *image.YCbCr:
		components = 3
		// Convert YCbCr to RGB
		pixelData = make([]byte, width*height*3)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				r, g, b, _ := imgTyped.At(x, y).RGBA()
				idx := (y*width + x) * 3
				pixelData[idx+0] = byte(r >> 8)
				pixelData[idx+1] = byte(g >> 8)
				pixelData[idx+2] = byte(b >> 8)
			}
		}

	case *image.RGBA:
		components = 3
		pixelData = make([]byte, width*height*3)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				srcIdx := (y*width + x) * 4
				dstIdx := (y*width + x) * 3
				pixelData[dstIdx+0] = imgTyped.Pix[srcIdx+0]
				pixelData[dstIdx+1] = imgTyped.Pix[srcIdx+1]
				pixelData[dstIdx+2] = imgTyped.Pix[srcIdx+2]
			}
		}

	default:
		// Generic fallback
		components = 3
		pixelData = make([]byte, width*height*3)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				idx := (y*width + x) * 3
				pixelData[idx+0] = byte(r >> 8)
				pixelData[idx+1] = byte(g >> 8)
				pixelData[idx+2] = byte(b >> 8)
			}
		}
	}

	// If original was 12-bit, scale back up
	if bitDepth == 12 {
		pixelData = scaleToDepth(pixelData, components, width, height, 12)
	}

	return pixelData, width, height, components, bitDepth, nil
}

// detectBitDepth detects bit depth from SOF marker
func detectBitDepth(data []byte) int {
	// Look for SOF markers
	for i := 0; i < len(data)-5; i++ {
		if data[i] == 0xFF {
			marker := data[i+1]
			// SOF0, SOF1, SOF2, etc.
			if marker >= 0xC0 && marker <= 0xC3 {
				// Precision is at offset i+4
				if i+4 < len(data) {
					precision := int(data[i+4])
					if precision == 12 {
						return 12
					}
				}
				return 8
			}
		}
	}
	return 8 // default
}

// convertSOF1ToSOF0 converts SOF1 back to SOF0 for decoding
func convertSOF1ToSOF0(data []byte) []byte {
	result := make([]byte, len(data))
	copy(result, data)

	for i := 0; i < len(result)-1; i++ {
		if result[i] == 0xFF && result[i+1] == 0xC1 {
			// Found SOF1
			result[i+1] = 0xC0 // Change to SOF0

			// Update precision to 8-bit
			if i+4 < len(result) {
				result[i+4] = 8
			}
			break
		}
	}

	return result
}

// scaleToDepth scales pixel data to target bit depth
func scaleToDepth(data []byte, components, width, height, targetDepth int) []byte {
	if targetDepth == 8 {
		return data
	}

	// Scale 8-bit to 12-bit
	numPixels := width * height * components
	result := make([]byte, numPixels*2)

	for i := 0; i < numPixels; i++ {
		val8 := int(data[i])
		// Scale from 8-bit (0-255) to 12-bit (0-4095)
		val12 := (val8 * 4095) / 255

		idx := i * 2
		result[idx] = byte(val12 & 0xFF)
		result[idx+1] = byte((val12 >> 8) & 0xFF)
	}

	return result
}
