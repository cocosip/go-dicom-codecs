package extended

import (
	"bytes"
	"image"
	"image/jpeg"

	"github.com/cocosip/go-dicom-codec/jpeg/baseline"
	"github.com/cocosip/go-dicom-codec/jpeg/standard"
)

// EncodeSimple encodes 8-bit JPEG Extended input through the Baseline encoder.
// Twelve-bit input uses the native Sequential DCT implementation.
func EncodeSimple(pixelData []byte, width, height, components, bitDepth, quality int) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, standard.ErrInvalidDimensions
	}
	if components != 1 && components != 3 {
		return nil, standard.ErrInvalidComponents
	}
	if bitDepth != 8 && bitDepth != sequential12Precision {
		return nil, standard.ErrInvalidPrecision
	}
	if quality < 1 || quality > 100 {
		return nil, standard.ErrInvalidQuality
	}
	if bitDepth == sequential12Precision {
		return encodeSequential12(pixelData, width, height, components, quality)
	}
	return baseline.Encode(pixelData, width, height, components, quality)
}

// DecodeSimple decodes JPEG Extended data. Twelve-bit SOF1 frames use the
// native Sequential DCT implementation; 8-bit frames use Go's JPEG decoder.
func DecodeSimple(jpegData []byte) (pixelData []byte, width, height, components, bitDepth int, err error) {
	if detectBitDepth(jpegData) == sequential12Precision {
		return decodeSequential12(jpegData)
	}

	imageData, err := jpeg.Decode(bytes.NewReader(jpegData))
	if err != nil {
		return nil, 0, 0, 0, 0, err
	}
	bounds := imageData.Bounds()
	width, height = bounds.Dx(), bounds.Dy()

	switch typed := imageData.(type) {
	case *image.Gray:
		components = 1
		pixelData = append([]byte(nil), typed.Pix...)
	case *image.YCbCr:
		components = 3
		pixelData = make([]byte, width*height*3)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				r, g, b, _ := typed.At(x, y).RGBA()
				offset := (y*width + x) * 3
				pixelData[offset], pixelData[offset+1], pixelData[offset+2] = byte(r>>8), byte(g>>8), byte(b>>8)
			}
		}
	default:
		components = 3
		pixelData = make([]byte, width*height*3)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				r, g, b, _ := imageData.At(x, y).RGBA()
				offset := (y*width + x) * 3
				pixelData[offset], pixelData[offset+1], pixelData[offset+2] = byte(r>>8), byte(g>>8), byte(b>>8)
			}
		}
	}

	return pixelData, width, height, components, 8, nil
}

func detectBitDepth(data []byte) int {
	for i := 0; i < len(data)-5; i++ {
		if data[i] != 0xff {
			continue
		}
		marker := data[i+1]
		if marker >= 0xc0 && marker <= 0xc3 {
			if int(data[i+4]) == sequential12Precision {
				return sequential12Precision
			}
			return 8
		}
	}
	return 8
}
