package lossless

import (
	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
	"github.com/cocosip/go-dicom-codecs/jpeg2000"
	"github.com/cocosip/go-dicom-codecs/jpeg2000/internal/common/codestream"

	"context"
	dicomcodec "github.com/cocosip/go-dicom/pkg/imaging/codec"
	pixel "github.com/cocosip/go-dicom/pkg/imaging/pixel"
	"testing"
)

func TestLosslessCodecWithMCTBindingsWritesMarkers(t *testing.T) {
	w, h, comps := 8, 8, 2
	n := w * h
	src := make([]byte, n*comps)
	for i := 0; i < n; i++ {
		src[2*i] = byte(i % 256)
		src[2*i+1] = byte((i * 3) % 256)
	}

	frameInfo := &dicomcodec.FrameInfo{
		Width:  uint16(w),
		Height: uint16(h),

		SamplesPerPixel: uint16(comps), BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation("MONOCHROME2"),
	}
	pdIn := codecHelpers.NewTestPixelData(frameInfo)
	if err := pdIn.AddFrame(context.Background(), src); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}
	pdOut := codecHelpers.NewTestPixelData(frameInfo)

	params := NewLosslessParameters()
	b := jpeg2000.MCTBindingParams{AssocType: 2, ComponentIDs: []uint16{0, 1}, Matrix: [][]float64{{1, 0}, {0, 1}}, Inverse: [][]float64{{1, 0}, {0, 1}}, Offsets: []int32{5, -5}, ElementType: 1, MCOPrecision: 1}
	params.SetParameter("mctBindings", []jpeg2000.MCTBindingParams{b})
	c := NewCodec()
	if err := c.Encode(context.Background(), pdIn, pdOut, params); err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	encodedData, _ := pdOut.Frame(context.Background(), 0)
	cs, err := codestream.NewParser(encodedData).Parse()
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(cs.MCT) == 0 || len(cs.MCC) == 0 {
		t.Fatalf("expected MCT/MCC present")
	}
}
