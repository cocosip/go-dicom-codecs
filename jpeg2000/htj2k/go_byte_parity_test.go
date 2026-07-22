package htj2k

import (
	"bytes"
	"testing"

	"github.com/cocosip/go-dicom-codecs/jpeg2000"
)

func TestGoHTJ2KEncoderMatchesFoDicomNativeFixture(t *testing.T) {
	manifest := readInteropManifest(t, htj2kInteropFixtureDir())

	for _, entry := range manifest.Fixtures {
		entry := entry
		t.Run(entry.Name, func(t *testing.T) {
			raw := readInteropFile(t, htj2kInteropFixtureDir(), entry.InputRaw)
			for name, reference := range entry.Codestreams {
				name, reference := name, reference
				t.Run(name, func(t *testing.T) {
					params := jpeg2000.DefaultEncodeParams(entry.Width, entry.Height, entry.Components, entry.BitsAllocated, entry.Signed)
					params.HTJ2KMode = true
					// OpenJPH uses RPCL when Native receives PROG_UNKNOWN.
					params.ProgressionOrder = 2
					params.BlockEncoderFactory = func(width, height int) jpeg2000.BlockEncoder {
						return NewHTEncoder(width, height)
					}

					actual, err := jpeg2000.NewEncoder(params).Encode(raw)
					if err != nil {
						t.Fatalf("Go HTJ2K encode: %v", err)
					}
					expected := codestreamThroughEOC(t, readInteropFile(t, htj2kInteropFixtureDir(), reference.Path))
					if !bytes.Equal(actual, expected) {
						diff := firstByteDiff(actual, expected)
						t.Fatalf("Go HTJ2K codestream differs from fo-dicom Native at byte %d: got=%02X want=%02X; got %d bytes, want %d", diff, actual[diff], expected[diff], len(actual), len(expected))
					}
				})
			}
		})
	}
}

func codestreamThroughEOC(t *testing.T, data []byte) []byte {
	t.Helper()
	for index := 0; index+1 < len(data); index++ {
		if data[index] == 0xFF && data[index+1] == 0xD9 {
			return data[:index+2]
		}
	}
	t.Fatal("HTJ2K reference codestream has no EOC marker")
	return nil
}
