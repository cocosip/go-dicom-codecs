package codec

import (
	"context"
	"fmt"

	dicomcodec "github.com/cocosip/go-dicom/pkg/imaging/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/pixel"
)

// TestPixelData is a simple implementation of codec.FrameSource and codec.FrameSink for testing.
type TestPixelData struct {
	frames    [][]byte
	frameInfo dicomcodec.FrameInfo
}

// NewTestPixelData creates a new TestPixelData with the given frame info
func NewTestPixelData(frameInfo *dicomcodec.FrameInfo) *TestPixelData {
	return &TestPixelData{
		frames:    make([][]byte, 0),
		frameInfo: *frameInfo,
	}
}

// Frame returns the pixel data for the specified frame (0-indexed).
func (p *TestPixelData) Frame(ctx context.Context, frameIndex int) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if frameIndex < 0 || frameIndex >= len(p.frames) {
		return nil, fmt.Errorf("frame index %d out of range [0, %d)", frameIndex, len(p.frames))
	}
	return p.frames[frameIndex], nil
}

// AddFrame appends a new frame to the pixel data
func (p *TestPixelData) AddFrame(_ context.Context, frameData []byte) error {
	p.frames = append(p.frames, frameData)
	return nil
}

// FrameCount returns the number of frames in the pixel data
func (p *TestPixelData) FrameCount() int {
	return len(p.frames)
}

// FrameInfo returns frame metadata for codec operations.
func (p *TestPixelData) FrameInfo() dicomcodec.FrameInfo {
	return p.frameInfo
}

// SetFrameInfo updates frame metadata.
func (p *TestPixelData) SetFrameInfo(info dicomcodec.FrameInfo) error {
	if err := info.Validate(); err != nil {
		return err
	}
	p.frameInfo = info
	return nil
}

// Encapsulated returns true if pixel data is encapsulated (compressed).
func (p *TestPixelData) Encapsulated() bool {
	return false
}

var _ dicomcodec.FrameSource = (*TestPixelData)(nil)
var _ dicomcodec.FrameSink = (*TestPixelData)(nil)

// NewFrameInfo creates valid frame metadata for codec tests.
func NewFrameInfo(width, height, bitsAllocated, bitsStored, highBit, samples uint16, signed bool, planar pixel.PlanarConfiguration, photometric *pixel.PhotometricInterpretation) dicomcodec.FrameInfo {
	return dicomcodec.FrameInfo{
		Width:           width,
		Height:          height,
		BitDepth:        *pixel.NewBitDepth(bitsAllocated, bitsStored, highBit, signed),
		SamplesPerPixel: samples,
		PixelRepresentation: func() pixel.Representation {
			if signed {
				return pixel.SignedPixels
			}
			return pixel.UnsignedPixels
		}(),
		PlanarConfiguration: planar,
		PhotometricInterpretation: func() pixel.PhotometricInterpretation {
			if photometric == nil {
				return pixel.PhotometricInterpretation{}
			}
			return *photometric
		}(),
	}
}
