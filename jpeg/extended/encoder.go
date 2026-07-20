package extended

// Encode encodes pixel data to JPEG Extended format (SOF1)
// components: 1 for grayscale, 3 for RGB
// bitDepth: 8 or 12 bits per sample
// quality: 1-100, where 100 is best quality
func Encode(pixelData []byte, width, height, components, bitDepth, quality int) ([]byte, error) {
	if bitDepth == sequential12Precision {
		return encodeSequential12(pixelData, width, height, components, quality)
	}

	return EncodeSimple(pixelData, width, height, components, bitDepth, quality)
}
