// Package main exports decoded DICOM pixel data to PNG.
package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"

	_ "github.com/cocosip/go-dicom-codecs/jpeg/baseline"
	_ "github.com/cocosip/go-dicom-codecs/jpeg/lossless"
	_ "github.com/cocosip/go-dicom-codecs/jpeg/lossless14sv1"
	_ "github.com/cocosip/go-dicom-codecs/jpeg2000/lossless"
	_ "github.com/cocosip/go-dicom-codecs/jpeg2000/lossy"
	_ "github.com/cocosip/go-dicom-codecs/jpegls/lossless"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/transcode"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

func decodePixels(path string) ([]byte, int, int, bool, error) {
	res, err := parser.ParseFile(path, parser.WithReadOption(parser.ReadAll))
	if err != nil {
		return nil, 0, 0, false, err
	}
	manager, err := transcode.NewManager(codec.GlobalRegistry())
	if err != nil {
		return nil, 0, 0, false, err
	}
	tr, err := manager.NewTranscoder(res.TransferSyntax, transfer.ExplicitVRLittleEndian)
	if err != nil {
		return nil, 0, 0, false, err
	}
	ds, err := tr.Transcode(context.Background(), res.Dataset)
	if err != nil {
		return nil, 0, 0, false, err
	}
	rows := int(ds.TryGetUInt16(tag.Rows, 0))
	cols := int(ds.TryGetUInt16(tag.Columns, 0))
	signed := ds.TryGetUInt16(tag.PixelRepresentation, 0) != 0
	pd, _ := ds.Get(tag.PixelData)
	switch v := pd.(type) {
	case *element.OtherByte:
		return v.GetData(), rows, cols, signed, nil
	case *element.OtherWord:
		return v.GetData(), rows, cols, signed, nil
	default:
		return nil, 0, 0, false, fmt.Errorf("unexpected pixel data type %T", pd)
	}
}
func savePNG(raw []byte, rows, cols int, signed bool, out string) error {
	img := image.NewGray(image.Rect(0, 0, cols, rows))
	// auto window: use min/max
	minv, maxv := int32(1<<30), int32(-1<<30)
	for i := 0; i+1 < len(raw); i += 2 {
		u := binary.LittleEndian.Uint16(raw[i:])
		var v int32
		if signed {
			v = int32(int16(u))
		} else {
			v = int32(u)
		}
		if v < minv {
			minv = v
		}
		if v > maxv {
			maxv = v
		}
	}
	if maxv == minv {
		maxv = minv + 1
	}
	idx := 0
	for i := 0; i+1 < len(raw); i += 2 {
		u := binary.LittleEndian.Uint16(raw[i:])
		var v int32
		if signed {
			v = int32(int16(u))
		} else {
			v = int32(u)
		}
		l := float64(v-minv) / float64(maxv-minv)
		if l < 0 {
			l = 0
		}
		if l > 1 {
			l = 1
		}
		img.Pix[idx] = uint8(l*255 + 0.5)
		idx++
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()
	return png.Encode(f, img)
}
func main() {
	files := map[string]string{
		"D:/1.dcm":                                "D:/1_orig.png",
		"D:/1_transcoded/1_jpeg_lossless.dcm":     "D:/1_lossless.png",
		"D:/1_transcoded/1_jpeg_lossless_sv1.dcm": "D:/1_lossless_sv1.png",
	}
	for in, out := range files {
		raw, r, c, signed, err := decodePixels(in)
		if err != nil {
			fmt.Println(in, "decode err", err)
			continue
		}
		if err := savePNG(raw, r, c, signed, out); err != nil {
			fmt.Println(in, "save err", err)
			continue
		}
		fmt.Println("wrote", out)
	}
}
