// Package main shows pixel extraction examples.
package main

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"

	"github.com/cocosip/go-dicom/pkg/dicom/parser"

	// Register codecs
	"context"
	_ "github.com/cocosip/go-dicom-codecs/jpeg/baseline"
	_ "github.com/cocosip/go-dicom-codecs/jpeg/extended"
	_ "github.com/cocosip/go-dicom-codecs/jpeg/lossless"
	_ "github.com/cocosip/go-dicom-codecs/jpeg/lossless14sv1"
	_ "github.com/cocosip/go-dicom-codecs/jpeg2000/lossless"
	_ "github.com/cocosip/go-dicom-codecs/jpeg2000/lossy"
	_ "github.com/cocosip/go-dicom-codecs/jpegls/lossless"
	"github.com/cocosip/go-dicom/pkg/imaging/pixeldata"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: extract_pixels <input.dcm>")
		fmt.Println("  Extracts raw pixel data from DICOM file")
		fmt.Println("  Output will be saved as <input>_raw_pixels.bin")
		os.Exit(1)
	}

	path := os.Args[1]

	// Generate output filename based on input filename
	ext := filepath.Ext(path)
	baseName := strings.TrimSuffix(filepath.Base(path), ext)
	outDir := filepath.Dir(path)
	outRaw := filepath.Join(outDir, baseName+"_raw_pixels.bin")

	res, err := parser.ParseFile(path, parser.WithReadOption(parser.ReadAll))
	if err != nil {
		panic(err)
	}
	ds := res.Dataset
	pd, err := pixeldata.FromDataset(ds)
	if err != nil {
		panic(err)
	}
	frame0, err := pd.Frame(context.Background(), 0)
	if err != nil {
		panic(err)
	}

	info := pd.Info
	fmt.Printf("width=%d height=%d comps=%d bitsStored=%d bitsAllocated=%d pixelRep=%d frames=%d len=%d crc32=%08x\n",
		info.Width, info.Height, info.SamplesPerPixel, info.BitsStored, info.BitsAllocated, info.PixelRepresentation, pd.FrameCount(), len(frame0), crc32.ChecksumIEEE(frame0))

	fmt.Printf("first 10 samples (little endian): ")
	for i := 0; i < 10 && (i*2+1) < len(frame0); i++ {
		v := binary.LittleEndian.Uint16(frame0[i*2 : i*2+2])
		fmt.Printf("%d ", v)
	}
	fmt.Println()

	if err := os.WriteFile(outRaw, frame0, 0644); err != nil {
		panic(err)
	}
	fmt.Println("wrote", outRaw)
}
