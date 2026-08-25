package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/vr"
	"github.com/cocosip/go-dicom/pkg/imaging"
)

const outsideDICOMPath = "../outside.dcm"

func TestDecodeInputUsesRGBMetadataForYBRWithoutChangingEncodedDataset(t *testing.T) {
	for _, photometricInterpretation := range []string{"YBR_FULL", "YBR_FULL_422"} {
		t.Run(photometricInterpretation, func(t *testing.T) {
			encoded := dataset.New()
			if err := encoded.Add(element.NewString(
				tag.PhotometricInterpretation,
				vr.CS,
				[]string{photometricInterpretation},
			)); err != nil {
				t.Fatal(err)
			}

			decodeInput, err := decodeInputDataset(encoded)
			if err != nil {
				t.Fatalf("decodeInputDataset() error = %v", err)
			}
			if got := decodeInput.TryGetString(tag.PhotometricInterpretation); got != "RGB" {
				t.Errorf("decode input PhotometricInterpretation = %q, want RGB", got)
			}
			if got := encoded.TryGetString(tag.PhotometricInterpretation); got != photometricInterpretation {
				t.Errorf("encoded PhotometricInterpretation = %q, want %s", got, photometricInterpretation)
			}
		})
	}
}

func TestGenerateGoArtifactsAddsAllGoDirectionsWithExactFoDicomCodestreams(t *testing.T) {
	repositoryRoot := filepath.Join("..", "..")
	sourceRoot := filepath.Join(repositoryRoot, "test-data", "htj2k", "interop-v1")
	bundleRoot := filepath.Join(t.TempDir(), "interop-v1")
	copyBundle(t, sourceRoot, bundleRoot)

	if err := generateGoArtifacts(bundleRoot); err != nil {
		t.Fatalf("generateGoArtifacts() error = %v", err)
	}

	manifestBytes, err := os.ReadFile(filepath.Join(bundleRoot, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var generated bundleManifest
	if err := json.Unmarshal(manifestBytes, &generated); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range generated.Fixtures {
		for _, syntax := range fixture.Syntaxes {
			if len(syntax.GoEncodedFrames) != fixture.Image.FrameCount {
				t.Fatalf("%s/%s Go encoded frames = %d, want %d", fixture.Image.Name, syntax.Name, len(syntax.GoEncodedFrames), fixture.Image.FrameCount)
			}
			if len(syntax.GoFromGoFrames) != fixture.Image.FrameCount {
				t.Fatalf("%s/%s Go-from-Go frames = %d, want %d", fixture.Image.Name, syntax.Name, len(syntax.GoFromGoFrames), fixture.Image.FrameCount)
			}
			if len(syntax.GoFromFoFrames) != fixture.Image.FrameCount {
				t.Fatalf("%s/%s Go-from-fo frames = %d, want %d", fixture.Image.Name, syntax.Name, len(syntax.GoFromFoFrames), fixture.Image.FrameCount)
			}
			for frame := 0; frame < fixture.Image.FrameCount; frame++ {
				goEncoded := readBundleFile(t, bundleRoot, syntax.GoEncodedFrames[frame])
				foEncoded := readBundleFile(t, bundleRoot, syntax.EncodedFrames[frame])
				if !bytes.Equal(goEncoded, foEncoded) {
					t.Fatalf("%s/%s frame %d Go codestream differs from fo-dicom.Codecs", fixture.Image.Name, syntax.Name, frame)
				}
				foDecoded := readBundleFile(t, bundleRoot, syntax.DecodedFrames[frame])
				if got := readBundleFile(t, bundleRoot, syntax.GoFromGoFrames[frame]); !bytes.Equal(got, foDecoded) {
					t.Fatalf("%s/%s frame %d Go-from-Go differs from saved fo decode", fixture.Image.Name, syntax.Name, frame)
				}
				if got := readBundleFile(t, bundleRoot, syntax.GoFromFoFrames[frame]); !bytes.Equal(got, foDecoded) {
					t.Fatalf("%s/%s frame %d Go-from-fo differs from saved fo decode", fixture.Image.Name, syntax.Name, frame)
				}
			}

			parsed, err := parser.ParseFile(
				filepath.Join(bundleRoot, filepath.FromSlash(syntax.GoEncodedDicom)),
				parser.WithReadOption(parser.ReadAll),
			)
			if err != nil {
				t.Fatalf("parse %s/%s Go DICOM: %v", fixture.Image.Name, syntax.Name, err)
			}
			if parsed.TransferSyntax.UID().UID() != syntax.TransferSyntaxUID {
				t.Fatalf("%s/%s transfer syntax = %s, want %s", fixture.Image.Name, syntax.Name, parsed.TransferSyntax.UID().UID(), syntax.TransferSyntaxUID)
			}
			pixels, err := imaging.CreatePixelData(parsed.Dataset)
			if err != nil {
				t.Fatalf("read %s/%s Go DICOM pixel data: %v", fixture.Image.Name, syntax.Name, err)
			}
			if pixels.FrameCount() != fixture.Image.FrameCount {
				t.Fatalf("%s/%s DICOM frames = %d, want %d", fixture.Image.Name, syntax.Name, pixels.FrameCount(), fixture.Image.FrameCount)
			}
		}
	}
}

func TestGenerateGoArtifactsRejectsUnsafeManifestPathsBeforeFileAccess(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*bundleManifest)
		want   string
	}{
		{
			name: "source DICOM escapes bundle",
			mutate: func(manifest *bundleManifest) {
				manifest.Fixtures[0].Source.Dicom = outsideDICOMPath
			},
			want: "source DICOM",
		},
		{
			name: "source DICOM is empty",
			mutate: func(manifest *bundleManifest) {
				manifest.Fixtures[0].Source.Dicom = ""
			},
			want: "source DICOM",
		},
		{
			name: "fo encoded DICOM escapes bundle",
			mutate: func(manifest *bundleManifest) {
				manifest.Fixtures[0].Syntaxes[0].EncodedDicom = outsideDICOMPath
			},
			want: "encoded DICOM",
		},
		{
			name: "fo encoded DICOM is empty",
			mutate: func(manifest *bundleManifest) {
				manifest.Fixtures[0].Syntaxes[0].EncodedDicom = ""
			},
			want: "encoded DICOM",
		},
		{
			name: "fixture name contains traversal",
			mutate: func(manifest *bundleManifest) {
				manifest.Fixtures[0].Image.Name = "../outside"
			},
			want: "fixture name",
		},
		{
			name: "syntax name contains traversal",
			mutate: func(manifest *bundleManifest) {
				manifest.Fixtures[0].Syntaxes[0].Name = "../outside"
			},
			want: "syntax name",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			manifest := bundleManifest{
				SchemaVersion: 2,
				Fixtures: []bundleFixture{{
					Image:  bundleImage{Name: "safe", FrameCount: 1},
					Source: bundleSource{Dicom: "sources/safe.dcm"},
					Syntaxes: []bundleSyntax{{
						Name:              "htj2k-lossless",
						TransferSyntaxUID: "1.2.840.10008.1.2.4.201",
						EncodedDicom:      "fo-encoded/safe.dcm",
					}},
				}},
			}
			test.mutate(&manifest)
			if err := writeBundleManifest(root, manifest); err != nil {
				t.Fatal(err)
			}

			err := generateGoArtifacts(root)
			if err == nil || !strings.Contains(err.Error(), test.want) || !strings.Contains(err.Error(), "bundle") {
				t.Fatalf("generateGoArtifacts() error = %v, want unsafe %s bundle error", err, test.want)
			}
		})
	}
}

func TestResolveWithinRejectsUnsafePathsCrossPlatform(t *testing.T) {
	root := t.TempDir()
	tests := []string{
		"",
		".",
		"..",
		outsideDICOMPath,
		"safe/../../outside.dcm",
		"/outside.dcm",
		"C:/outside.dcm",
		`..\outside.dcm`,
		"safe/../outside.dcm",
	}
	for _, slashPath := range tests {
		t.Run(slashPath, func(t *testing.T) {
			if resolved, err := resolveWithin(root, slashPath); err == nil {
				t.Fatalf("resolveWithin(%q) = %q, want error", slashPath, resolved)
			}
		})
	}

	resolved, err := resolveWithin(root, "sources/safe.dcm")
	if err != nil {
		t.Fatalf("resolveWithin(valid path) error = %v", err)
	}
	want := filepath.Join(root, "sources", "safe.dcm")
	if resolved != want {
		t.Fatalf("resolveWithin(valid path) = %q, want %q", resolved, want)
	}
}

func TestWriteBytesArtifactRejectsSymlinkEscape(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "bundle")
	external := filepath.Join(parent, "outside")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(external, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(root, "go-encoded")); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}

	err := writeBytesArtifact(root, "go-encoded/escape.j2c", []byte("escape"), map[string]artifactDigest{})
	if err == nil {
		t.Fatal("writeBytesArtifact() followed a symlink outside the bundle")
	}
	if _, statErr := os.Stat(filepath.Join(external, "escape.j2c")); !os.IsNotExist(statErr) {
		t.Fatalf("outside artifact exists after rejected write: %v", statErr)
	}
}

func TestWriteBundleManifestRejectsSymlinkEscape(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "bundle")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	externalManifest := filepath.Join(parent, "outside-manifest.json")
	original := []byte("external manifest must not change")
	if err := os.WriteFile(externalManifest, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalManifest, filepath.Join(root, "manifest.json")); err != nil {
		t.Skipf("file symlinks unavailable: %v", err)
	}

	if err := writeBundleManifest(root, bundleManifest{SchemaVersion: 2}); err == nil {
		t.Fatal("writeBundleManifest() followed a symlink outside the bundle")
	}
	got, err := os.ReadFile(externalManifest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("external manifest changed to %q", got)
	}
}

func copyBundle(t *testing.T, sourceRoot, destinationRoot string) {
	t.Helper()
	err := filepath.WalkDir(sourceRoot, func(sourcePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(sourceRoot, sourcePath)
		if err != nil {
			return err
		}
		destinationPath := filepath.Join(destinationRoot, relative)
		if entry.IsDir() {
			return os.MkdirAll(destinationPath, 0755)
		}
		content, err := os.ReadFile(sourcePath)
		if err != nil {
			return err
		}
		return os.WriteFile(destinationPath, content, 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func readBundleFile(t *testing.T, root, slashPath string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(slashPath)))
	if err != nil {
		t.Fatal(err)
	}
	return content
}
