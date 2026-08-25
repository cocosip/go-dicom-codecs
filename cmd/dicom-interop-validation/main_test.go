package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const testPhotometricMonochrome2 = "MONOCHROME2"

type testManifest struct {
	SchemaVersion int            `json:"schemaVersion"`
	Codec         testCodec      `json:"codec"`
	Fixtures      []testFixture  `json:"fixtures"`
	Artifacts     []testArtifact `json:"artifacts"`
}

type testCodec struct {
	AssemblyName   string `json:"assemblyName"`
	ManagedVersion string `json:"managedVersion"`
	SourceCommit   string `json:"sourceCommit"`
}

type testFixture struct {
	Image    testImage    `json:"image"`
	Source   testSource   `json:"source"`
	Syntaxes []testSyntax `json:"syntaxes"`
}

type testImage struct {
	Name                      string `json:"name"`
	Width                     int    `json:"width"`
	Height                    int    `json:"height"`
	SamplesPerPixel           int    `json:"samplesPerPixel"`
	BitsAllocated             int    `json:"bitsAllocated"`
	BitsStored                int    `json:"bitsStored"`
	Signed                    bool   `json:"signed"`
	PhotometricInterpretation string `json:"photometricInterpretation"`
	FrameCount                int    `json:"frameCount"`
}

type testSource struct {
	Dicom              string   `json:"dicom"`
	Frames             []string `json:"frames"`
	EncoderInputFrames []string `json:"encoderInputFrames"`
}

type testSyntax struct {
	Name              string   `json:"name"`
	TransferSyntaxUID string   `json:"transferSyntaxUid"`
	Lossless          bool     `json:"lossless"`
	EncodedDicom      string   `json:"encodedDicom"`
	EncodedFrames     []string `json:"encodedFrames"`
	DecodedDicom      string   `json:"decodedDicom"`
	DecodedFrames     []string `json:"decodedFrames"`
	GoEncodedDicom    string   `json:"goEncodedDicom"`
	GoEncodedFrames   []string `json:"goEncodedFrames"`
	GoFromGoDicom     string   `json:"goFromGoDicom"`
	GoFromGoFrames    []string `json:"goFromGoFrames"`
	GoFromFoDicom     string   `json:"goFromFoDicom"`
	GoFromFoFrames    []string `json:"goFromFoFrames"`
	FoFromGoDicom     string   `json:"foFromGoDicom,omitempty"`
	FoFromGoFrames    []string `json:"foFromGoFrames,omitempty"`
}

type testArtifact struct {
	Path   string `json:"path"`
	Length int64  `json:"length"`
	SHA256 string `json:"sha256"`
}

func TestValidateBundleAcceptsSchemaV2Matrix(t *testing.T) {
	root, _ := writeValidBundle(t)
	if err := validateBundle(root); err != nil {
		t.Fatalf("validateBundle() error = %v", err)
	}
}

func TestValidateBundleWorksWithEmptyPATH(t *testing.T) {
	root, _ := writeValidBundle(t)
	t.Setenv("PATH", "")
	if err := validateBundle(root); err != nil {
		t.Fatalf("validateBundle() with empty PATH error = %v", err)
	}
}

func TestValidateBundleRejectsUnsafeAndDuplicateArtifactPaths(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testManifest)
		want   string
	}{
		{name: "absolute", mutate: func(manifest *testManifest) {
			manifest.Artifacts[0].Path = "C:/outside.raw"
		}, want: "absolute"},
		{name: "escape", mutate: func(manifest *testManifest) {
			manifest.Artifacts[0].Path = "../outside.raw"
		}, want: "escape"},
		{name: "duplicate", mutate: func(manifest *testManifest) {
			manifest.Artifacts = append(manifest.Artifacts, manifest.Artifacts[0])
		}, want: "duplicate"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root, manifest := writeValidBundle(t)
			test.mutate(&manifest)
			writeManifest(t, root, manifest)
			assertValidationError(t, root, test.want)
		})
	}
}

func TestValidateBundleRejectsMissingFramesAndSyntaxes(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testManifest)
		want   string
	}{
		{name: "source frame", mutate: func(manifest *testManifest) {
			manifest.Fixtures[0].Source.Frames = nil
		}, want: "source frames"},
		{name: "encoded frame", mutate: func(manifest *testManifest) {
			manifest.Fixtures[0].Syntaxes[0].EncodedFrames = nil
		}, want: "encoded frames"},
		{name: "decoded frame", mutate: func(manifest *testManifest) {
			manifest.Fixtures[0].Syntaxes[0].DecodedFrames = nil
		}, want: "decoded frames"},
		{name: "Go encoded frame", mutate: func(manifest *testManifest) {
			manifest.Fixtures[0].Syntaxes[0].GoEncodedFrames = nil
		}, want: "Go encoded frames"},
		{name: "Go-from-Go frame", mutate: func(manifest *testManifest) {
			manifest.Fixtures[0].Syntaxes[0].GoFromGoFrames = nil
		}, want: "Go-from-Go frames"},
		{name: "Go-from-fo frame", mutate: func(manifest *testManifest) {
			manifest.Fixtures[0].Syntaxes[0].GoFromFoFrames = nil
		}, want: "Go-from-fo frames"},
		{name: "partial fo-from-Go direction", mutate: func(manifest *testManifest) {
			manifest.Fixtures[0].Syntaxes[0].FoFromGoDicom = "decoded/fo-from-go/incomplete.dcm"
		}, want: "fo-from-Go frames"},
		{name: "syntax direction", mutate: func(manifest *testManifest) {
			manifest.Fixtures[0].Syntaxes = manifest.Fixtures[0].Syntaxes[:2]
		}, want: "transfer syntax"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root, manifest := writeValidBundle(t)
			test.mutate(&manifest)
			writeManifest(t, root, manifest)
			assertValidationError(t, root, test.want)
		})
	}
}

func TestValidateBundleRejectsIncompleteFixtureMatrix(t *testing.T) {
	root, manifest := writeValidBundle(t)
	manifest.Fixtures = append(manifest.Fixtures[:4], manifest.Fixtures[5:]...)
	writeManifest(t, root, manifest)
	assertValidationError(t, root, "RGB 16-bit")
}

func TestValidateBundleRejectsIncorrectArtifactLengthAndHash(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testManifest)
		want   string
	}{
		{name: "length", mutate: func(manifest *testManifest) {
			manifest.Artifacts[0].Length++
		}, want: "length"},
		{name: "sha256", mutate: func(manifest *testManifest) {
			manifest.Artifacts[0].SHA256 = strings.Repeat("0", 64)
		}, want: "sha256"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root, manifest := writeValidBundle(t)
			test.mutate(&manifest)
			writeManifest(t, root, manifest)
			assertValidationError(t, root, test.want)
		})
	}
}

func TestValidateBundleRejectsInvalidSchemaAndProvenance(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testManifest)
		want   string
	}{
		{name: "schema", mutate: func(manifest *testManifest) { manifest.SchemaVersion = 1 }, want: "schema version"},
		{name: "assembly", mutate: func(manifest *testManifest) { manifest.Codec.AssemblyName = "other" }, want: "assembly"},
		{name: "version below range", mutate: func(manifest *testManifest) {
			manifest.Codec.ManagedVersion = "6.0.0-beta0"
		}, want: "managed version"},
		{name: "version above range", mutate: func(manifest *testManifest) {
			manifest.Codec.ManagedVersion = "7.0.0"
		}, want: "managed version"},
		{name: "commit", mutate: func(manifest *testManifest) {
			manifest.Codec.SourceCommit = "not-a-commit"
		}, want: "source commit"},
		{name: "managed version without commit metadata", mutate: func(manifest *testManifest) {
			manifest.Codec.ManagedVersion = "6.0.0"
		}, want: "commit metadata"},
		{name: "managed version with invalid commit metadata", mutate: func(manifest *testManifest) {
			manifest.Codec.ManagedVersion = "6.0.0+build"
		}, want: "commit metadata"},
		{name: "managed version commit mismatch", mutate: func(manifest *testManifest) {
			manifest.Codec.SourceCommit = strings.Repeat("b", 40)
		}, want: "does not match source commit"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root, manifest := writeValidBundle(t)
			test.mutate(&manifest)
			writeManifest(t, root, manifest)
			assertValidationError(t, root, test.want)
		})
	}
}

func TestValidateBundleRejectsUndeclaredAndUnreferencedArtifacts(t *testing.T) {
	t.Run("undeclared file", func(t *testing.T) {
		root, _ := writeValidBundle(t)
		writeFile(t, root, "fo-encoded/untracked.dcm", []byte("extra"))
		assertValidationError(t, root, "not declared")
	})

	t.Run("unreferenced digest", func(t *testing.T) {
		root, manifest := writeValidBundle(t)
		path := "fo-encoded/unreferenced.dcm"
		content := []byte("extra")
		writeFile(t, root, path, content)
		manifest.Artifacts = append(manifest.Artifacts, digest(path, content))
		writeManifest(t, root, manifest)
		assertValidationError(t, root, "not referenced")
	})
}

func TestCommandSourceDoesNotImportOSExec(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, imported := range parsed.Imports {
		if strings.Trim(imported.Path.Value, `"`) == "os/exec" {
			t.Fatal("main.go imports os/exec; offline validation must not execute processes")
		}
	}
}

func assertValidationError(t *testing.T, root, want string) {
	t.Helper()
	err := validateBundle(root)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(want)) {
		t.Fatalf("validateBundle() error = %v, want error containing %q", err, want)
	}
}

func writeValidBundle(t *testing.T) (string, testManifest) {
	t.Helper()
	root := t.TempDir()
	manifest := testManifest{
		SchemaVersion: 2,
		Codec: testCodec{
			AssemblyName:   "fo-dicom.Codecs",
			ManagedVersion: "6.0.0+80d8103d394aeed8ce70141c742d7d53620ef90e",
			SourceCommit:   "80d8103d394aeed8ce70141c742d7d53620ef90e",
		},
	}

	images := []testImage{
		{Name: "mono-s16-odd", Width: 3, Height: 5, SamplesPerPixel: 1, BitsAllocated: 16, BitsStored: 16, Signed: true, PhotometricInterpretation: testPhotometricMonochrome2, FrameCount: 1},
		{Name: "mono-u8", Width: 2, Height: 2, SamplesPerPixel: 1, BitsAllocated: 8, BitsStored: 8, PhotometricInterpretation: testPhotometricMonochrome2, FrameCount: 1},
		{Name: "mono-u16-multiframe", Width: 2, Height: 2, SamplesPerPixel: 1, BitsAllocated: 16, BitsStored: 16, PhotometricInterpretation: testPhotometricMonochrome2, FrameCount: 3},
		{Name: "rgb-u8", Width: 2, Height: 2, SamplesPerPixel: 3, BitsAllocated: 8, BitsStored: 8, PhotometricInterpretation: "RGB", FrameCount: 1},
		{Name: "rgb-u16", Width: 2, Height: 2, SamplesPerPixel: 3, BitsAllocated: 16, BitsStored: 16, PhotometricInterpretation: "RGB", FrameCount: 1},
		{Name: "ybr-full-u8", Width: 2, Height: 2, SamplesPerPixel: 3, BitsAllocated: 8, BitsStored: 8, PhotometricInterpretation: "YBR_FULL", FrameCount: 1},
		{Name: "ybr-full-422-u8", Width: 2, Height: 2, SamplesPerPixel: 3, BitsAllocated: 8, BitsStored: 8, PhotometricInterpretation: "YBR_FULL_422", FrameCount: 1},
	}
	artifactByPath := make(map[string]testArtifact)
	addArtifact := func(path string, content []byte) {
		if _, exists := artifactByPath[path]; exists {
			return
		}
		writeFile(t, root, path, content)
		artifactByPath[path] = digest(path, content)
	}

	for _, image := range images {
		fixture := testFixture{Image: image}
		fixture.Source.Dicom = "sources/" + image.Name + ".dcm"
		addArtifact(fixture.Source.Dicom, []byte("DICOM:"+image.Name))
		for frame := 0; frame < image.FrameCount; frame++ {
			framePath := "sources/frames/" + image.Name + "-frame-" + fourDigits(frame) + ".raw"
			frameLength := image.Width * image.Height * image.SamplesPerPixel * image.BitsAllocated / 8
			if image.PhotometricInterpretation == "YBR_FULL_422" {
				frameLength = image.Width * image.Height * 2
			}
			addArtifact(framePath, make([]byte, frameLength))
			fixture.Source.Frames = append(fixture.Source.Frames, framePath)
			encoderPath := framePath
			if strings.HasPrefix(image.PhotometricInterpretation, "YBR_") {
				encoderPath = "sources/encoder-input-frames/" + image.Name + "-frame-" + fourDigits(frame) + ".rgb"
				addArtifact(encoderPath, make([]byte, image.Width*image.Height*image.SamplesPerPixel))
			}
			fixture.Source.EncoderInputFrames = append(fixture.Source.EncoderInputFrames, encoderPath)
		}

		definitions := []struct {
			name, uid string
			lossless  bool
		}{
			{name: "htj2k-lossless", uid: "1.2.840.10008.1.2.4.201", lossless: true},
			{name: "htj2k-lossless-rpcl", uid: "1.2.840.10008.1.2.4.202", lossless: true},
			{name: "htj2k-lossy", uid: "1.2.840.10008.1.2.4.203", lossless: false},
		}
		for _, definition := range definitions {
			syntax := testSyntax{Name: definition.name, TransferSyntaxUID: definition.uid, Lossless: definition.lossless}
			baseName := image.Name + "-" + definition.name
			syntax.EncodedDicom = "fo-encoded/" + image.Name + "-" + definition.name + ".dcm"
			syntax.DecodedDicom = "decoded/fo-from-fo/" + image.Name + "-" + definition.name + ".dcm"
			syntax.GoEncodedDicom = "go-encoded/" + baseName + ".dcm"
			syntax.GoFromGoDicom = "decoded/go-from-go/" + baseName + ".dcm"
			syntax.GoFromFoDicom = "decoded/go-from-fo/" + baseName + ".dcm"
			addArtifact(syntax.EncodedDicom, []byte("ENCODED:"+definition.name))
			addArtifact(syntax.DecodedDicom, []byte("DECODED:"+definition.name))
			addArtifact(syntax.GoEncodedDicom, []byte("GO-ENCODED:"+definition.name))
			addArtifact(syntax.GoFromGoDicom, []byte("GO-FROM-GO:"+definition.name))
			addArtifact(syntax.GoFromFoDicom, []byte("GO-FROM-FO:"+definition.name))
			for frame := 0; frame < image.FrameCount; frame++ {
				encodedPath := "fo-encoded/frames/" + image.Name + "-" + definition.name + "-frame-" + fourDigits(frame) + ".j2c"
				decodedPath := "decoded/fo-from-fo/" + image.Name + "-" + definition.name + "-frame-" + fourDigits(frame) + ".raw"
				goEncodedPath := "go-encoded/frames/" + baseName + "-frame-" + fourDigits(frame) + ".j2c"
				goFromGoPath := "decoded/go-from-go/" + baseName + "-frame-" + fourDigits(frame) + ".raw"
				goFromFoPath := "decoded/go-from-fo/" + baseName + "-frame-" + fourDigits(frame) + ".raw"
				addArtifact(encodedPath, []byte{0xff, 0x4f, byte(frame), 0xff, 0xd9})
				addArtifact(decodedPath, make([]byte, image.Width*image.Height*image.SamplesPerPixel*image.BitsAllocated/8))
				addArtifact(goEncodedPath, []byte{0xff, 0x4f, byte(frame), 0xff, 0xd9})
				addArtifact(goFromGoPath, make([]byte, image.Width*image.Height*image.SamplesPerPixel*image.BitsAllocated/8))
				addArtifact(goFromFoPath, make([]byte, image.Width*image.Height*image.SamplesPerPixel*image.BitsAllocated/8))
				syntax.EncodedFrames = append(syntax.EncodedFrames, encodedPath)
				syntax.DecodedFrames = append(syntax.DecodedFrames, decodedPath)
				syntax.GoEncodedFrames = append(syntax.GoEncodedFrames, goEncodedPath)
				syntax.GoFromGoFrames = append(syntax.GoFromGoFrames, goFromGoPath)
				syntax.GoFromFoFrames = append(syntax.GoFromFoFrames, goFromFoPath)
			}
			fixture.Syntaxes = append(fixture.Syntaxes, syntax)
		}
		manifest.Fixtures = append(manifest.Fixtures, fixture)
	}

	for _, artifact := range artifactByPath {
		manifest.Artifacts = append(manifest.Artifacts, artifact)
	}
	sort.Slice(manifest.Artifacts, func(i, j int) bool {
		return manifest.Artifacts[i].Path < manifest.Artifacts[j].Path
	})
	writeManifest(t, root, manifest)
	return root, manifest
}

func digest(path string, content []byte) testArtifact {
	sum := sha256.Sum256(content)
	return testArtifact{Path: path, Length: int64(len(content)), SHA256: hex.EncodeToString(sum[:])}
}

func writeManifest(t *testing.T, root string, manifest testManifest) {
	t.Helper()
	content, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), append(content, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, root, slashPath string, content []byte) {
	t.Helper()
	filePath := filepath.Join(root, filepath.FromSlash(slashPath))
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, content, 0600); err != nil {
		t.Fatal(err)
	}
}

func fourDigits(value int) string {
	return string([]byte{'0' + byte(value/1000%10), '0' + byte(value/100%10), '0' + byte(value/10%10), '0' + byte(value%10)})
}
