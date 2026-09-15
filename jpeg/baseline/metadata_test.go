package baseline

import (
	"context"
	"errors"
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

func TestCodecEncodeReportsYCbCrOutputMetadataToSinkOnly(t *testing.T) {
	info := &dicomcodec.FrameInfo{
		Width: 8, Height: 8,
		SamplesPerPixel: 3, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricRGB),
	}
	source := newMetadataPixelData(info, false)
	if err := source.AddFrame(context.Background(), make([]byte, 8*8*3)); err != nil {
		t.Fatal(err)
	}
	destination := &metadataFrameSink{}

	if err := NewBaselineCodec(90).Encode(context.Background(), source, destination, nil); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if got := destination.info.PhotometricInterpretation.Value; got != "YBR_FULL_422" {
		t.Fatalf("encoded PhotometricInterpretation = %q, want YBR_FULL_422", got)
	}
	if got := destination.info.PlanarConfiguration; got != pixel.InterleavedPlanar {
		t.Fatalf("encoded PlanarConfiguration = %d, want %d", got, pixel.InterleavedPlanar)
	}
	if destination.info.Width != info.Width || destination.info.Height != info.Height {
		t.Fatalf("encoded dimensions = %dx%d, want %dx%d", destination.info.Width, destination.info.Height, info.Width, info.Height)
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

func TestCodecDecodeReportsRGBInterleavedOutputMetadataToSinkOnly(t *testing.T) {
	encodeInfo := &dicomcodec.FrameInfo{
		Width: 8, Height: 8,
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
	decodeInfo.PlanarConfiguration = pixel.PlanarPlanar
	encodedSource := newMetadataPixelData(&decodeInfo, true)
	if err := encodedSource.AddFrame(context.Background(), encoded.frames[0]); err != nil {
		t.Fatal(err)
	}
	destination := &metadataFrameSink{}
	if err := codec.Decode(context.Background(), encodedSource, destination, nil); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if got := destination.info.PhotometricInterpretation.Value; got != photometricRGB {
		t.Fatalf("decoded PhotometricInterpretation = %q, want RGB", got)
	}
	if got := destination.info.PlanarConfiguration; got != pixel.InterleavedPlanar {
		t.Fatalf("decoded PlanarConfiguration = %d, want %d", got, pixel.InterleavedPlanar)
	}
	if destination.info.Width != decodeInfo.Width || destination.info.Height != decodeInfo.Height {
		t.Fatalf("decoded dimensions = %dx%d, want %dx%d", destination.info.Width, destination.info.Height, decodeInfo.Width, decodeInfo.Height)
	}
}

func TestCodecEncodeReturnsSinkMetadataError(t *testing.T) {
	info := &dicomcodec.FrameInfo{
		Width: 8, Height: 8,
		SamplesPerPixel: 3, BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.InterleavedPlanar, PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation(photometricRGB),
	}
	source := newMetadataPixelData(info, false)
	if err := source.AddFrame(context.Background(), make([]byte, 8*8*3)); err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("set frame info failed")
	destination := &metadataFrameSink{setFrameInfoErr: wantErr}

	err := NewBaselineCodec(90).Encode(context.Background(), source, destination, nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Encode() error = %v, want %v", err, wantErr)
	}
}

type metadataPixelData struct {
	frames       [][]byte
	info         dicomcodec.FrameInfo
	encapsulated bool
}

type metadataFrameSink struct {
	frames          [][]byte
	info            dicomcodec.FrameInfo
	setFrameInfoErr error
}

func (sink *metadataFrameSink) AddFrame(_ context.Context, frame []byte) error {
	sink.frames = append(sink.frames, append([]byte(nil), frame...))
	return nil
}

func (sink *metadataFrameSink) SetFrameInfo(info dicomcodec.FrameInfo) error {
	if sink.setFrameInfoErr != nil {
		return sink.setFrameInfoErr
	}
	if err := info.Validate(); err != nil {
		return err
	}
	sink.info = info
	return nil
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
var _ dicomcodec.FrameSink = (*metadataFrameSink)(nil)
