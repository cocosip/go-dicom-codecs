package openjph

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/cocosip/go-dicom-codecs/jpeg2000/htj2k/openjph/t2"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/imaging"
)

func TestAcceptedRangeNativeDecodeMatchesFoDicom(t *testing.T) {
	root := acceptedRangeBundleRoot()
	manifest := readAcceptedRangeManifest(t, root)
	assertAcceptedRangeFixtureMatrix(t, manifest)

	for _, fixture := range manifest.Fixtures {
		fixture := fixture
		for _, syntax := range fixture.Syntaxes {
			syntax := syntax
			t.Run(fmt.Sprintf("%s/%s", fixture.Image.Name, syntax.Name), func(t *testing.T) {
				assertAcceptedRangeSavedGoDicom(t, root, fixture, syntax)
				params := DefaultEncodeParams(
					fixture.Image.Width,
					fixture.Image.Height,
					fixture.Image.SamplesPerPixel,
					fixture.Image.BitsAllocated,
					fixture.Image.Signed,
				)
				params.Lossless = syntax.Lossless
				params.Quality = 80
				params.ProgressionOrder = uint8(t2.ProgressionRPCL)
				encoder := NewEncoder(params)
				for direction, paths := range map[string][]string{
					"fo-from-fo": syntax.DecodedFrames,
					"go-encoded": syntax.GoEncodedFrames,
					"go-from-go": syntax.GoFromGoFrames,
					"go-from-fo": syntax.GoFromFoFrames,
					"fo-from-go": syntax.FoFromGoFrames,
				} {
					if len(paths) != fixture.Image.FrameCount {
						t.Fatalf("%s frame count = %d, want metadata %d", direction, len(paths), fixture.Image.FrameCount)
					}
				}
				for frame := 0; frame < fixture.Image.FrameCount; frame++ {
					frame := frame
					t.Run(fmt.Sprintf("frame-%04d", frame), func(t *testing.T) {
						native := readAcceptedRangeArtifactPath(t, root, syntax.EncodedFrames[frame])
						want := readAcceptedRangeArtifactPath(t, root, syntax.DecodedFrames[frame])
						if got := readAcceptedRangeArtifactPath(t, root, syntax.GoEncodedFrames[frame]); !bytes.Equal(got, native) {
							t.Fatalf("saved Go codestream differs from fo-dicom.Codecs at byte %d", firstAcceptedRangeByteDiff(got, native))
						}
						for direction, artifactPath := range map[string]string{
							"go-from-go": syntax.GoFromGoFrames[frame],
							"go-from-fo": syntax.GoFromFoFrames[frame],
							"fo-from-go": syntax.FoFromGoFrames[frame],
						} {
							got := readAcceptedRangeArtifactPath(t, root, artifactPath)
							assertAcceptedRangeDecodedBytes(t, fixture.Image, got, want, direction+"/fo-from-fo")
						}
						decoder := NewDecoder()
						if err := decoder.Decode(native); err != nil {
							t.Fatalf("decode Native HTJ2K frame: %v", err)
						}
						got := decoder.GetPixelData()
						assertAcceptedRangeDecodedBytes(t, fixture.Image, got, want, "fo-from-fo")

						if acceptedRangeRequiresSourceEquality(syntax, fixture.Image) {
							source := readAcceptedRangeArtifactPath(t, root, fixture.Source.Frames[frame])
							assertAcceptedRangeDecodedBytes(t, fixture.Image, got, source, "source")
						}

						encoderInput := readAcceptedRangeArtifactPath(t, root, fixture.Source.EncoderInputFrames[frame])
						goEncoded, err := encoder.Encode(encoderInput)
						if err != nil {
							t.Fatalf("encode Go HTJ2K frame: %v", err)
						}
						goDecoder := NewDecoder()
						if err := goDecoder.Decode(goEncoded); err != nil {
							t.Fatalf("decode Go HTJ2K frame: %v", err)
						}
						assertAcceptedRangeDecodedBytes(
							t, fixture.Image, goDecoder.GetPixelData(), want, "go-from-go/fo-reference",
						)
					})
				}
			})
		}
	}
}

func assertAcceptedRangeSavedGoDicom(
	t *testing.T,
	root string,
	fixture acceptedRangeFixture,
	syntax acceptedRangeSyntax,
) {
	t.Helper()
	parsed, err := parser.ParseFile(
		filepath.Join(root, filepath.FromSlash(syntax.GoEncodedDicom)),
		parser.WithReadOption(parser.ReadAll),
	)
	if err != nil {
		t.Fatalf("parse saved Go DICOM: %v", err)
	}
	if got := parsed.TransferSyntax.UID().UID(); got != syntax.TransferSyntaxUID {
		t.Fatalf("saved Go DICOM transfer syntax = %s, want %s", got, syntax.TransferSyntaxUID)
	}
	pixels, err := imaging.CreatePixelData(parsed.Dataset)
	if err != nil {
		t.Fatalf("read saved Go DICOM pixel data: %v", err)
	}
	if pixels.FrameCount() != fixture.Image.FrameCount {
		t.Fatalf("saved Go DICOM frame count = %d, want %d", pixels.FrameCount(), fixture.Image.FrameCount)
	}
	for frame := 0; frame < fixture.Image.FrameCount; frame++ {
		got, err := pixels.GetFrame(frame)
		if err != nil {
			t.Fatalf("read saved Go DICOM frame %d: %v", frame, err)
		}
		got, err = acceptedRangeCodestreamThroughEOC(got)
		if err != nil {
			t.Fatalf("trim saved Go DICOM frame %d: %v", frame, err)
		}
		want := readAcceptedRangeArtifactPath(t, root, syntax.GoEncodedFrames[frame])
		if !bytes.Equal(got, want) {
			t.Fatalf("saved Go DICOM frame %d fragment order differs at byte %d", frame, firstAcceptedRangeByteDiff(got, want))
		}
	}
}

func acceptedRangeCodestreamThroughEOC(content []byte) ([]byte, error) {
	if len(content) < 4 || !bytes.Equal(content[:2], []byte{0xff, 0x4f}) {
		return nil, fmt.Errorf("codestream is missing SOC")
	}
	for offset := 2; offset+1 < len(content); offset++ {
		if content[offset] == 0xff && content[offset+1] == 0xd9 {
			return content[:offset+2], nil
		}
	}
	return nil, fmt.Errorf("codestream is missing EOC")
}

func acceptedRangeRequiresSourceEquality(syntax acceptedRangeSyntax, image acceptedRangeImage) bool {
	if !syntax.Lossless {
		return false
	}
	if image.PhotometricInterpretation == "YBR_FULL" || image.PhotometricInterpretation == "YBR_FULL_422" {
		return false
	}
	// The authority's 16-bit multi-component wrapper traverses component lines
	// differently from the source. Its own decoded frame remains the strict gate.
	return image.BitsAllocated != 16 || image.SamplesPerPixel == 1
}

func assertAcceptedRangeDecodedBytes(t *testing.T, image acceptedRangeImage, got, want []byte, reference string) {
	t.Helper()
	if bytes.Equal(got, want) {
		return
	}
	first := firstAcceptedRangeByteDiff(got, want)
	bytesPerSample := max(1, image.BitsAllocated/8)
	sample := first / bytesPerSample
	component := sample % image.SamplesPerPixel
	pixel := sample / image.SamplesPerPixel
	row := pixel / image.Width
	column := pixel % image.Width
	t.Fatalf(
		"decoded bytes differ from %s at byte=%d sample=%d component=%d row=%d column=%d: Go length=%d value=%d bytes=% X reference length=%d value=%d bytes=% X",
		reference, first, sample, component, row, column,
		len(got), acceptedRangeDecodedSample(got, sample, bytesPerSample, image.Signed), prefixAt(got, first, 12),
		len(want), acceptedRangeDecodedSample(want, sample, bytesPerSample, image.Signed), prefixAt(want, first, 12),
	)
}

func acceptedRangeDecodedSample(data []byte, sample, bytesPerSample int, signed bool) int32 {
	offset := sample * bytesPerSample
	if offset < 0 || offset+bytesPerSample > len(data) {
		return 0
	}
	if bytesPerSample == 1 {
		if signed {
			return int32(int8(data[offset]))
		}
		return int32(data[offset])
	}
	value := binary.LittleEndian.Uint16(data[offset:])
	if signed {
		return int32(int16(value))
	}
	return int32(value)
}
