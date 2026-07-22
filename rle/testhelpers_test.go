package rle

import (
	"fmt"

	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
)

type testPixelData struct {
	frames                    [][]byte
	width                     uint16
	height                    uint16
	bitsAllocated             uint16
	bitsStored                uint16
	highBit                   uint16
	samplesPerPixel           uint16
	pixelRepresentation       uint16
	planarConfiguration       uint16
	photometricInterpretation string
	encapsulated              bool
}

func newTestPixelData(info *imagetypes.FrameInfo) *testPixelData {
	return &testPixelData{
		frames:                    make([][]byte, 0),
		width:                     info.Width,
		height:                    info.Height,
		bitsAllocated:             info.BitsAllocated,
		bitsStored:                info.BitsStored,
		highBit:                   info.HighBit,
		samplesPerPixel:           info.SamplesPerPixel,
		pixelRepresentation:       info.PixelRepresentation,
		planarConfiguration:       info.PlanarConfiguration,
		photometricInterpretation: info.PhotometricInterpretation,
	}
}

func (pd *testPixelData) GetFrame(frameIndex int) ([]byte, error) {
	if frameIndex < 0 || frameIndex >= len(pd.frames) {
		return nil, fmt.Errorf("frame index %d out of range [0, %d)", frameIndex, len(pd.frames))
	}
	return pd.frames[frameIndex], nil
}

func (pd *testPixelData) AddFrame(frameData []byte) error {
	pd.frames = append(pd.frames, frameData)
	return nil
}

func (pd *testPixelData) FrameCount() int { return len(pd.frames) }

func (pd *testPixelData) GetFrameInfo() *imagetypes.FrameInfo {
	return &imagetypes.FrameInfo{
		Width:                     pd.width,
		Height:                    pd.height,
		BitsAllocated:             pd.bitsAllocated,
		BitsStored:                pd.bitsStored,
		HighBit:                   pd.highBit,
		SamplesPerPixel:           pd.samplesPerPixel,
		PixelRepresentation:       pd.pixelRepresentation,
		PlanarConfiguration:       pd.planarConfiguration,
		PhotometricInterpretation: pd.photometricInterpretation,
	}
}

func (pd *testPixelData) IsEncapsulated() bool { return pd.encapsulated }
