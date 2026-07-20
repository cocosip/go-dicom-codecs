package extended

import (
	"encoding/binary"
	"os"
	"testing"

	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/imaging"
)

const native12BitProcess24Path = `D:\6-native\6_jpeg_process2_4.dcm`
const native12BitSourcePath = `D:\6.dcm`

func TestDecodeNative12BitProcess24Frame(t *testing.T) {
	if _, err := os.Stat(native12BitProcess24Path); os.IsNotExist(err) {
		t.Skipf("Native JPEG Extended fixture is unavailable: %s", native12BitProcess24Path)
	}

	result, err := parser.ParseFile(native12BitProcess24Path, parser.WithReadOption(parser.ReadAll))
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	pixelData, err := imaging.CreatePixelData(result.Dataset)
	if err != nil {
		t.Fatalf("CreatePixelData() error = %v", err)
	}
	frame, err := pixelData.GetFrame(0)
	if err != nil {
		t.Fatalf("GetFrame(0) error = %v", err)
	}

	decoded, width, height, components, bitDepth, err := Decode(frame)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if width != 288 || height != 288 || components != 1 || bitDepth != 12 {
		t.Fatalf("Decode() metadata = %dx%d components=%d bitDepth=%d, want 288x288 components=1 bitDepth=12", width, height, components, bitDepth)
	}
	if want := 288 * 288 * 2; len(decoded) != want {
		t.Fatalf("Decode() byte length = %d, want %d", len(decoded), want)
	}

	sourceResult, err := parser.ParseFile(native12BitSourcePath, parser.WithReadOption(parser.ReadAll))
	if err != nil {
		t.Fatalf("ParseFile(source) error = %v", err)
	}
	sourcePixelData, err := imaging.CreatePixelData(sourceResult.Dataset)
	if err != nil {
		t.Fatalf("CreatePixelData(source) error = %v", err)
	}
	source, err := sourcePixelData.GetFrame(0)
	if err != nil {
		t.Fatalf("GetFrame(source) error = %v", err)
	}

	maximumDifference := 0
	for offset := 0; offset < len(source); offset += 2 {
		difference := int(binary.LittleEndian.Uint16(source[offset:])) - int(binary.LittleEndian.Uint16(decoded[offset:]))
		if difference < 0 {
			difference = -difference
		}
		if difference > maximumDifference {
			maximumDifference = difference
		}
	}
	if maximumDifference > 20 {
		t.Fatalf("Decode() maximum sample difference = %d, want <= 20", maximumDifference)
	}
}

func TestEncode12BitProcess24PreservesSamplesWithinOne8BitBucket(t *testing.T) {
	const width, height = 8, 8
	pixels := make([]byte, width*height*2)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			value := uint16(1000)
			if (x+y)%2 == 1 {
				value = 1008
			}
			binary.LittleEndian.PutUint16(pixels[(y*width+x)*2:], value)
		}
	}

	encoded, err := Encode(pixels, width, height, 1, 12, 100)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	decoded, _, _, _, bitDepth, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if bitDepth != 12 {
		t.Fatalf("Decode() bitDepth = %d, want 12", bitDepth)
	}

	minimum, maximum := uint16(4095), uint16(0)
	for offset := 0; offset < len(decoded); offset += 2 {
		value := binary.LittleEndian.Uint16(decoded[offset:])
		if value < minimum {
			minimum = value
		}
		if value > maximum {
			maximum = value
		}
	}
	if maximum-minimum < 4 {
		t.Fatalf("12-bit contrast was lost: min=%d max=%d", minimum, maximum)
	}
}
