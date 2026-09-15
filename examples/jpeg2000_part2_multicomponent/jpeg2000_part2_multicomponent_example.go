// Package main demonstrates JPEG2000 Part-2 multicomponent transforms.
package main

import (
	"fmt"

	"context"
	codecHelpers "github.com/cocosip/go-dicom-codecs/codec"
	j2k "github.com/cocosip/go-dicom-codecs/jpeg2000"
	lossless "github.com/cocosip/go-dicom-codecs/jpeg2000/lossless"
	lossy "github.com/cocosip/go-dicom-codecs/jpeg2000/lossy"
	dicomcodec "github.com/cocosip/go-dicom/pkg/imaging/codec"
	pixel "github.com/cocosip/go-dicom/pkg/imaging/pixel"
)

func main() {
	fmt.Println("=== JPEG 2000 Part 2 Multi-component Example ===")

	// Build a binding: components {0,1}, identity matrix, offsets {+5,-5}
	b := j2k.NewMCTBinding().
		Assoc(2). // Matrix then Offset
		Components([]uint16{0, 1}).
		Matrix([][]float64{{1, 0}, {0, 1}}).
		Inverse([][]float64{{1, 0}, {0, 1}}).
		Offsets([]int32{5, -5}).
		ElementType(1).  // float32
		MCOPrecision(1). // reversible flag
		Build()

	// Lossy path
	{
		p := lossy.NewLossyParameters().WithRate(90).WithMCTBindings([]j2k.MCTBindingParams{b})
		enc := lossy.NewCodecWithRate(90)

		// Prepare dummy PixelData (RGB 8x8)
		w, h, comps := 8, 8, 3
		n := w * h
		src := make([]byte, n*comps)
		frameInfo := &dicomcodec.FrameInfo{
			Width:  uint16(w),
			Height: uint16(h),

			SamplesPerPixel: uint16(comps), BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation("MONOCHROME2"),
		}
		pdIn := codecHelpers.NewTestPixelData(frameInfo)
		if err := pdIn.AddFrame(context.Background(), src); err != nil {
			fmt.Println("add frame error:", err)
			return
		}
		pdOut := codecHelpers.NewTestPixelData(frameInfo)
		if err := enc.Encode(context.Background(), pdIn, pdOut, p); err != nil {
			fmt.Println("encode error:", err)
			return
		}
		fmt.Println("Lossy encode with Part 2 bindings completed (markers written)")
	}

	// Lossless path
	{
		p := lossless.NewLosslessParameters().WithNumLevels(0).WithMCTBindings([]j2k.MCTBindingParams{b})
		enc := lossless.NewCodec()

		// Prepare dummy PixelData (2 components 8x8)
		w, h, comps := 8, 8, 2
		n := w * h
		src := make([]byte, n*comps)
		frameInfo := &dicomcodec.FrameInfo{
			Width:  uint16(w),
			Height: uint16(h),

			SamplesPerPixel: uint16(comps), BitDepth: pixel.BitDepth{BitsAllocated: 8, BitsStored: 8, HighBit: 7, IsSigned: pixel.Representation(0).IsSigned()}, PixelRepresentation: pixel.Representation(0), PlanarConfiguration: pixel.PlanarConfiguration(0), PhotometricInterpretation: *pixel.MustParsePhotometricInterpretation("MONOCHROME2"),
		}
		pdIn := codecHelpers.NewTestPixelData(frameInfo)
		if err := pdIn.AddFrame(context.Background(), src); err != nil {
			fmt.Println("add frame error:", err)
			return
		}
		pdOut := codecHelpers.NewTestPixelData(frameInfo)
		if err := enc.Encode(context.Background(), pdIn, pdOut, p); err != nil {
			fmt.Println("encode error:", err)
			return
		}
		fmt.Println("Lossless encode with Part 2 bindings completed (markers written)")
	}
}
