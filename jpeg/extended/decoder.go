package extended

// Decode decodes JPEG Extended (SOF1) data
func Decode(jpegData []byte) (pixelData []byte, width, height, components, bitDepth int, err error) {
	if detectBitDepth(jpegData) == sequential12Precision {
		return decodeSequential12(jpegData)
	}

	return DecodeSimple(jpegData)
}
