package rle

import (
	"context"
	"fmt"

	dicomcodec "github.com/cocosip/go-dicom/pkg/imaging/codec"
)

type testPixelData struct {
	frames       [][]byte
	info         dicomcodec.FrameInfo
	encapsulated bool
}

func newTestPixelData(info *dicomcodec.FrameInfo) *testPixelData {
	return &testPixelData{
		frames: make([][]byte, 0),
		info:   *info,
	}
}

func (pd *testPixelData) Frame(_ context.Context, frameIndex int) ([]byte, error) {
	if frameIndex < 0 || frameIndex >= len(pd.frames) {
		return nil, fmt.Errorf("frame index %d out of range [0, %d)", frameIndex, len(pd.frames))
	}
	return pd.frames[frameIndex], nil
}

func (pd *testPixelData) AddFrame(_ context.Context, frameData []byte) error {
	pd.frames = append(pd.frames, frameData)
	return nil
}

func (pd *testPixelData) FrameCount() int { return len(pd.frames) }

func (pd *testPixelData) FrameInfo() dicomcodec.FrameInfo { return pd.info }

func (pd *testPixelData) SetFrameInfo(info dicomcodec.FrameInfo) error {
	if err := info.Validate(); err != nil {
		return err
	}
	pd.info = info
	return nil
}

func (pd *testPixelData) Encapsulated() bool { return pd.encapsulated }

var _ dicomcodec.FrameSource = (*testPixelData)(nil)
var _ dicomcodec.FrameSink = (*testPixelData)(nil)
