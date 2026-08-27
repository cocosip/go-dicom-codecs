package baseline

import (
	"fmt"
	"testing"

	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
)

func TestCodecEncodeReportsYCbCrOutputMetadata(t *testing.T) {
	info := &imagetypes.FrameInfo{
		Width:                     8,
		Height:                    8,
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           3,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "RGB",
	}
	source := newMetadataPixelData(info, false)
	if err := source.AddFrame(make([]byte, 8*8*3)); err != nil {
		t.Fatal(err)
	}
	encoded := newMetadataPixelData(info, true)

	if err := NewBaselineCodec(90).Encode(source, encoded, nil); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if got := encoded.GetFrameInfo().PhotometricInterpretation; got != "YBR_FULL_422" {
		t.Fatalf("encoded PhotometricInterpretation = %q, want YBR_FULL_422", got)
	}
}

func TestCodecDecodeReportsRGBInterleavedOutputMetadata(t *testing.T) {
	encodeInfo := &imagetypes.FrameInfo{
		Width:                     8,
		Height:                    8,
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           3,
		PlanarConfiguration:       0,
		PhotometricInterpretation: "RGB",
	}
	source := newMetadataPixelData(encodeInfo, false)
	if err := source.AddFrame(make([]byte, 8*8*3)); err != nil {
		t.Fatal(err)
	}
	encoded := newMetadataPixelData(encodeInfo, true)
	codec := NewBaselineCodec(90)
	if err := codec.Encode(source, encoded, nil); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	decodeInfo := *encodeInfo
	decodeInfo.PhotometricInterpretation = "YBR_FULL_422"
	decodeInfo.PlanarConfiguration = 1
	decoded := newMetadataPixelData(&decodeInfo, false)
	if err := codec.Decode(encoded, decoded, nil); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	got := decoded.GetFrameInfo()
	if got.PhotometricInterpretation != "RGB" {
		t.Fatalf("decoded PhotometricInterpretation = %q, want RGB", got.PhotometricInterpretation)
	}
	if got.PlanarConfiguration != 0 {
		t.Fatalf("decoded PlanarConfiguration = %d, want 0", got.PlanarConfiguration)
	}
}

type metadataPixelData struct {
	frames       [][]byte
	info         imagetypes.FrameInfo
	encapsulated bool
}

func newMetadataPixelData(info *imagetypes.FrameInfo, encapsulated bool) *metadataPixelData {
	return &metadataPixelData{info: *info, encapsulated: encapsulated}
}

func (pd *metadataPixelData) GetFrame(index int) ([]byte, error) {
	if index < 0 || index >= len(pd.frames) {
		return nil, fmt.Errorf("frame %d out of range", index)
	}
	return pd.frames[index], nil
}

func (pd *metadataPixelData) AddFrame(frame []byte) error {
	pd.frames = append(pd.frames, append([]byte(nil), frame...))
	return nil
}

func (pd *metadataPixelData) FrameCount() int { return len(pd.frames) }

func (pd *metadataPixelData) GetFrameInfo() *imagetypes.FrameInfo {
	info := pd.info
	return &info
}

func (pd *metadataPixelData) SetFrameInfo(info *imagetypes.FrameInfo) {
	if info != nil {
		pd.info = *info
	}
}

func (pd *metadataPixelData) IsEncapsulated() bool { return pd.encapsulated }
