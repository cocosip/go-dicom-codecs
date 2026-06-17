package htj2k

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	codecHelpers "github.com/cocosip/go-dicom-codec/codec"
	"github.com/cocosip/go-dicom-codec/jpeg2000"
	"github.com/cocosip/go-dicom-codec/jpeg2000/t2"
	"github.com/cocosip/go-dicom/pkg/imaging/imagetypes"
)

func TestWriteOpenJPHInteropFixture(t *testing.T) {
	outDir := os.Getenv("HTJ2K_INTEROP_FIXTURE_DIR")
	if outDir == "" {
		t.Skip("set HTJ2K_INTEROP_FIXTURE_DIR to write OpenJPH interop fixture files")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	frameInfo := &imagetypes.FrameInfo{
		Width:                     128,
		Height:                    128,
		BitsAllocated:             8,
		BitsStored:                8,
		HighBit:                   7,
		SamplesPerPixel:           1,
		PixelRepresentation:       0,
		PlanarConfiguration:       0,
		PhotometricInterpretation: photometricMonochrome2,
	}
	raw := makeGradient(128 * 128)
	src := codecHelpers.NewTestPixelData(frameInfo)
	if err := src.AddFrame(raw); err != nil {
		t.Fatalf("AddFrame failed: %v", err)
	}
	dst := codecHelpers.NewTestPixelData(frameInfo)
	if err := NewLosslessCodec().Encode(src, dst, nil); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	encoded, err := dst.GetFrame(0)
	if err != nil {
		t.Fatalf("GetFrame failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(outDir, "go_htj2k_128.j2c"), encoded, 0o644); err != nil {
		t.Fatalf("write codestream failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "raw_128.bin"), raw, 0o644); err != nil {
		t.Fatalf("write raw failed: %v", err)
	}
}

func TestDecodeOpenJPHInteropFixture(t *testing.T) {
	outDir := os.Getenv("HTJ2K_INTEROP_FIXTURE_DIR")
	if outDir == "" {
		t.Skip("set HTJ2K_INTEROP_FIXTURE_DIR to read OpenJPH interop fixture files")
	}

	codestream, err := os.ReadFile(filepath.Join(outDir, "openjph_htj2k_128.j2c"))
	if err != nil {
		t.Skipf("OpenJPH fixture unavailable: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(outDir, "raw_128.bin"))
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
		first := -1
		for i := range got {
			if got[i] != raw[i] {
				first = i
				break
			}
		}
		t.Fatalf("decoded OpenJPH fixture differs at byte %d: got=%d want=%d", first, got[first], raw[first])
	}
}
