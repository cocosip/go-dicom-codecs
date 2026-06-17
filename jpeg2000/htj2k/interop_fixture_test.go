package htj2k

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	codecHelpers "github.com/cocosip/go-dicom-codec/codec"
	"github.com/cocosip/go-dicom-codec/jpeg2000"
	"github.com/cocosip/go-dicom-codec/jpeg2000/t2"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
)

const defaultOpenJPHInteropFixtureDir = "testdata/interop"

func TestWriteOpenJPHInteropFixture(t *testing.T) {
	outDir := os.Getenv("HTJ2K_INTEROP_FIXTURE_DIR")
	if outDir == "" {
		t.Skip("set HTJ2K_INTEROP_FIXTURE_DIR to write OpenJPH interop fixture files")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	for _, fixture := range interopFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			src := codecHelpers.NewTestPixelData(fixture.frameInfo)
			if err := src.AddFrame(fixture.raw); err != nil {
				t.Fatalf("AddFrame failed: %v", err)
			}
			dst := codecHelpers.NewTestPixelData(fixture.frameInfo)
			if err := NewLosslessCodec().Encode(src, dst, nil); err != nil {
				t.Fatalf("Encode failed: %v", err)
			}
			encoded, err := dst.GetFrame(0)
			if err != nil {
				t.Fatalf("GetFrame failed: %v", err)
			}

			if err := os.WriteFile(filepath.Join(outDir, fixture.goJ2C), encoded, 0o644); err != nil {
				t.Fatalf("write codestream failed: %v", err)
			}
			if err := os.WriteFile(filepath.Join(outDir, fixture.rawFile), fixture.raw, 0o644); err != nil {
				t.Fatalf("write raw failed: %v", err)
			}
		})
	}
}

func TestDecodeOpenJPHInteropFixture(t *testing.T) {
	outDir := openJPHInteropFixtureDir()

	for _, fixture := range interopFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			codestream, err := os.ReadFile(filepath.Join(outDir, fixture.openJPHJ2C))
			if err != nil {
				t.Fatalf("OpenJPH fixture unavailable: %v", err)
			}
			raw, err := os.ReadFile(filepath.Join(outDir, fixture.rawFile))
			if err != nil {
				t.Fatalf("read raw failed: %v", err)
			}

			decoder := jpeg2000.NewDecoder()
			decoder.SetBlockDecoderFactory(func(width, height int, _ int) t2.BlockDecoder {
				return NewHTDecoder(width, height)
			})
			if err := decoder.Decode(codestream); err != nil {
				t.Fatalf("Decode OpenJPH codestream failed: %v", err)
			}
			if got := decoder.GetPixelData(); !bytes.Equal(got, raw) {
				first := firstByteDiff(got, raw)
				t.Fatalf("decoded OpenJPH fixture differs at byte %d: got=%d want=%d", first, got[first], raw[first])
			}
		})
	}
}

func openJPHInteropFixtureDir() string {
	if dir := os.Getenv("HTJ2K_INTEROP_FIXTURE_DIR"); dir != "" {
		return dir
	}
	return defaultOpenJPHInteropFixtureDir
}

type interopFixture struct {
	name       string
	frameInfo  *imagetypes.FrameInfo
	raw        []byte
	rawFile    string
	goJ2C      string
	openJPHJ2C string
}

func interopFixtures() []interopFixture {
	return []interopFixture{
		{
			name: "8-bit mono 128",
			frameInfo: &imagetypes.FrameInfo{
				Width:                     128,
				Height:                    128,
				BitsAllocated:             8,
				BitsStored:                8,
				HighBit:                   7,
				SamplesPerPixel:           1,
				PixelRepresentation:       0,
				PlanarConfiguration:       0,
				PhotometricInterpretation: photometricMonochrome2,
			},
			raw:        makeGradient(128 * 128),
			rawFile:    "raw_128.bin",
			goJ2C:      "go_htj2k_128.j2c",
			openJPHJ2C: "openjph_htj2k_128.j2c",
		},
		{
			name: "16-bit mono 128",
			frameInfo: &imagetypes.FrameInfo{
				Width:                     128,
				Height:                    128,
				BitsAllocated:             16,
				BitsStored:                16,
				HighBit:                   15,
				SamplesPerPixel:           1,
				PixelRepresentation:       0,
				PlanarConfiguration:       0,
				PhotometricInterpretation: photometricMonochrome2,
			},
			raw:        makeGradient16(128 * 128),
			rawFile:    "raw_128_u16le.bin",
			goJ2C:      "go_htj2k_128_u16.j2c",
			openJPHJ2C: "openjph_htj2k_128_u16.j2c",
		},
	}
}

func makeGradient16(samples int) []byte {
	data := make([]byte, samples*2)
	for i := 0; i < samples; i++ {
		binary.LittleEndian.PutUint16(data[i*2:], uint16((i*257+i/3)&0xFFFF))
	}
	return data
}

func firstByteDiff(a, b []byte) int {
	limit := len(a)
	if len(b) < limit {
		limit = len(b)
	}
	for i := 0; i < limit; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	if len(a) != len(b) {
		return limit
	}
	return -1
}
