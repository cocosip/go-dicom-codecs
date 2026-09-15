package baseline

import (
	"context"
	"fmt"
	"testing"

	dicomcodec "github.com/cocosip/go-dicom/pkg/imaging/codec"
	pixel "github.com/cocosip/go-dicom/pkg/imaging/pixel"
)

func TestCodecEncodeReportsYCbCrOutputMetadata(t *testing.T) {
	info := &dicomcodec.FrameInfo{
		Width:  8,
		Height: 8,

		SamplesPerPixel: 3, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricRGB),
	}
	source := newMetadataPixelData(info, false)
	if err := source.AddFrame(context.Background(), make([]byte, 8*8*3)); err != nil {
		t.Fatal(err)
	}
	encoded := newMetadataPixelData(info, true)

	if err := NewBaselineCodec(90).Encode(context.Background(), source, encoded, nil); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if got := encoded.FrameInfo().PhotometricInterpretation.Value; got != "YBR_FULL_422" {
		t.Fatalf("encoded PhotometricInterpretation = %q, want YBR_FULL_422", got)
	}
}

func TestCodecDecodeReportsRGBInterleavedOutputMetadata(t *testing.T) {
	encodeInfo := &dicomcodec.FrameInfo{
		Width:  8,
		Height: 8,

		SamplesPerPixel: 3, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricRGB),
	}
	source := newMetadataPixelData(encodeInfo, false)
	if err := source.AddFrame(context.Background(), make([]byte, 8*8*3)); err != nil {
		t.Fatal(err)
	}
	encoded := newMetadataPixelData(encodeInfo, true)
	codec := NewBaselineCodec(90)
	if err := codec.Encode(context.Background(), source, encoded, nil); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	decodeInfo := *encodeInfo
	decodeInfo.PhotometricInterpretation = *pixel.YbrFull422
	decodeInfo.PlanarConfiguration = 1
	decoded := newMetadataPixelData(&decodeInfo, false)
	if err := codec.Decode(context.Background(), encoded, decoded, nil); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	got := decoded.FrameInfo()
	if got.PhotometricInterpretation.Value != photometricRGB {
		t.Fatalf("decoded PhotometricInterpretation = %q, want RGB", got.PhotometricInterpretation.Value)
	}
	if got.PlanarConfiguration != 0 {
		t.Fatalf("decoded PlanarConfiguration = %d, want 0", got.PlanarConfiguration)
	}
}

type metadataPixelData struct {
	frames       [][]byte
	info         dicomcodec.FrameInfo
	encapsulated bool
}

func newMetadataPixelData(info *dicomcodec.FrameInfo, encapsulated bool) *metadataPixelData {
	return &metadataPixelData{info: *info, encapsulated: encapsulated}
}

func (pd *metadataPixelData) Frame(_ context.Context, index int) ([]byte, error) {
	if index < 0 || index >= len(pd.frames) {
		return nil, fmt.Errorf("frame %d out of range", index)
	}
	return pd.frames[index], nil
}

func (pd *metadataPixelData) AddFrame(_ context.Context, frame []byte) error {
	pd.frames = append(pd.frames, append([]byte(nil), frame...))
	return nil
}

func (pd *metadataPixelData) FrameCount() int { return len(pd.frames) }

func (pd *metadataPixelData) FrameInfo() dicomcodec.FrameInfo { return pd.info }

func (pd *metadataPixelData) SetFrameInfo(info dicomcodec.FrameInfo) error {
	if err := info.Validate(); err != nil {
		return err
	}
	pd.info = info
	return nil
}

func (pd *metadataPixelData) Encapsulated() bool { return pd.encapsulated }

var _ dicomcodec.FrameSource = (*metadataPixelData)(nil)
var _ dicomcodec.FrameSink = (*metadataPixelData)(nil)
