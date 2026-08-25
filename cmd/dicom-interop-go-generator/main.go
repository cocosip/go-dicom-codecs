// Command dicom-interop-go-generator adds reproducible Go directions to an offline HTJ2K bundle.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/cocosip/go-dicom-codecs/jpeg2000/htj2k"
	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/dicom/vr"
	"github.com/cocosip/go-dicom/pkg/dicom/writer"
	"github.com/cocosip/go-dicom/pkg/imaging"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

const defaultBundleRoot = "test-data/htj2k/interop-v1"

type bundleManifest struct {
	SchemaVersion         int              `json:"schemaVersion"`
	Codec                 bundleProvenance `json:"codec"`
	GeneratorSourceSHA256 string           `json:"generatorSourceSha256"`
	Fixtures              []bundleFixture  `json:"fixtures"`
	Artifacts             []artifactDigest `json:"artifacts"`
}

type bundleProvenance struct {
	AssemblyName   string `json:"assemblyName"`
	ManagedVersion string `json:"managedVersion"`
	SourceCommit   string `json:"sourceCommit"`
}

type bundleFixture struct {
	Image    bundleImage    `json:"image"`
	Source   bundleSource   `json:"source"`
	Syntaxes []bundleSyntax `json:"syntaxes"`
}

type bundleImage struct {
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

type bundleSource struct {
	Dicom              string   `json:"dicom"`
	Frames             []string `json:"frames"`
	EncoderInputFrames []string `json:"encoderInputFrames"`
}

type bundleSyntax struct {
	Name              string   `json:"name"`
	TransferSyntaxUID string   `json:"transferSyntaxUid"`
	Lossless          bool     `json:"lossless"`
	EncodedDicom      string   `json:"encodedDicom"`
	EncodedFrames     []string `json:"encodedFrames"`
	DecodedDicom      string   `json:"decodedDicom"`
	DecodedFrames     []string `json:"decodedFrames"`

	GoEncodedDicom  string   `json:"goEncodedDicom"`
	GoEncodedFrames []string `json:"goEncodedFrames"`
	GoFromGoDicom   string   `json:"goFromGoDicom"`
	GoFromGoFrames  []string `json:"goFromGoFrames"`
	GoFromFoDicom   string   `json:"goFromFoDicom"`
	GoFromFoFrames  []string `json:"goFromFoFrames"`
	FoFromGoDicom   string   `json:"foFromGoDicom,omitempty"`
	FoFromGoFrames  []string `json:"foFromGoFrames,omitempty"`
}

type artifactDigest struct {
	Path   string `json:"path"`
	Length int64  `json:"length"`
	SHA256 string `json:"sha256"`
}

func main() {
	bundleRoot := defaultBundleRoot
	if len(os.Args) == 3 && os.Args[1] == "--bundle" {
		bundleRoot = os.Args[2]
	} else if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: dicom-interop-go-generator [--bundle <directory>]")
		os.Exit(2)
	}
	if err := generateGoArtifacts(bundleRoot); err != nil {
		fmt.Fprintf(os.Stderr, "generate Go interoperability artifacts: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("generated Go HTJ2K interoperability artifacts in %s\n", bundleRoot)
}

func generateGoArtifacts(root string) error {
	loaded, err := readBundleManifest(root)
	if err != nil {
		return err
	}
	if loaded.SchemaVersion != 2 {
		return fmt.Errorf("schema version = %d, want 2", loaded.SchemaVersion)
	}
	if err := validateBundlePaths(root, loaded); err != nil {
		return err
	}

	artifacts := make(map[string]artifactDigest, len(loaded.Artifacts))
	for _, artifact := range loaded.Artifacts {
		artifacts[artifact.Path] = artifact
	}
	for fixtureIndex := range loaded.Fixtures {
		fixture := &loaded.Fixtures[fixtureIndex]
		source, err := parseBundleDICOM(root, fixture.Source.Dicom)
		if err != nil {
			return fmt.Errorf("parse source %q: %w", fixture.Image.Name, err)
		}
		for syntaxIndex := range fixture.Syntaxes {
			syntax := &fixture.Syntaxes[syntaxIndex]
			targetSyntax, err := transferSyntax(syntax.TransferSyntaxUID)
			if err != nil {
				return fmt.Errorf("fixture %q: %w", fixture.Image.Name, err)
			}
			if err := generateSyntaxDirections(root, fixture, syntax, source.Dataset, source.TransferSyntax, targetSyntax, artifacts); err != nil {
				return err
			}
		}
	}

	loaded.Artifacts = loaded.Artifacts[:0]
	for _, artifact := range artifacts {
		loaded.Artifacts = append(loaded.Artifacts, artifact)
	}
	sort.Slice(loaded.Artifacts, func(i, j int) bool {
		return loaded.Artifacts[i].Path < loaded.Artifacts[j].Path
	})
	return writeBundleManifest(root, loaded)
}

func generateSyntaxDirections(
	root string,
	fixture *bundleFixture,
	syntax *bundleSyntax,
	sourceDataset *dataset.Dataset,
	sourceSyntax, targetSyntax *transfer.Syntax,
	artifacts map[string]artifactDigest,
) error {
	baseName := fixture.Image.Name + "-" + syntax.Name
	syntax.GoEncodedDicom = "go-encoded/" + baseName + ".dcm"
	syntax.GoEncodedFrames = framePaths("go-encoded/frames/"+baseName, fixture.Image.FrameCount, ".j2c")
	syntax.GoFromGoDicom = "decoded/go-from-go/" + baseName + ".dcm"
	syntax.GoFromGoFrames = framePaths("decoded/go-from-go/"+baseName, fixture.Image.FrameCount, ".raw")
	syntax.GoFromFoDicom = "decoded/go-from-fo/" + baseName + ".dcm"
	syntax.GoFromFoFrames = framePaths("decoded/go-from-fo/"+baseName, fixture.Image.FrameCount, ".raw")

	goEncoded, err := transcode(sourceDataset, sourceSyntax, targetSyntax)
	if err != nil {
		return fmt.Errorf("encode Go %s/%s: %w", fixture.Image.Name, syntax.Name, err)
	}
	if err := writeDatasetArtifact(root, syntax.GoEncodedDicom, goEncoded, targetSyntax, artifacts); err != nil {
		return err
	}
	if err := writeCodestreamArtifacts(root, syntax.GoEncodedFrames, goEncoded, artifacts); err != nil {
		return fmt.Errorf("write Go encoded frames for %s/%s: %w", fixture.Image.Name, syntax.Name, err)
	}

	goDecodeInput, err := decodeInputDataset(goEncoded)
	if err != nil {
		return fmt.Errorf("prepare Go-from-Go %s/%s: %w", fixture.Image.Name, syntax.Name, err)
	}
	goFromGo, err := transcode(goDecodeInput, targetSyntax, transfer.ExplicitVRLittleEndian)
	if err != nil {
		return fmt.Errorf("decode Go-from-Go %s/%s: %w", fixture.Image.Name, syntax.Name, err)
	}
	if err := writeDatasetArtifact(root, syntax.GoFromGoDicom, goFromGo, transfer.ExplicitVRLittleEndian, artifacts); err != nil {
		return err
	}
	if err := writeRawArtifacts(root, syntax.GoFromGoFrames, goFromGo, fixture.Image, artifacts); err != nil {
		return fmt.Errorf("write Go-from-Go frames for %s/%s: %w", fixture.Image.Name, syntax.Name, err)
	}

	foEncoded, err := parseBundleDICOM(root, syntax.EncodedDicom)
	if err != nil {
		return fmt.Errorf("parse fo-dicom.Codecs encoding %s/%s: %w", fixture.Image.Name, syntax.Name, err)
	}
	foDecodeInput, err := decodeInputDataset(foEncoded.Dataset)
	if err != nil {
		return fmt.Errorf("prepare Go-from-fo %s/%s: %w", fixture.Image.Name, syntax.Name, err)
	}
	goFromFo, err := transcode(foDecodeInput, foEncoded.TransferSyntax, transfer.ExplicitVRLittleEndian)
	if err != nil {
		return fmt.Errorf("decode Go-from-fo %s/%s: %w", fixture.Image.Name, syntax.Name, err)
	}
	if err := writeDatasetArtifact(root, syntax.GoFromFoDicom, goFromFo, transfer.ExplicitVRLittleEndian, artifacts); err != nil {
		return err
	}
	if err := writeRawArtifacts(root, syntax.GoFromFoFrames, goFromFo, fixture.Image, artifacts); err != nil {
		return fmt.Errorf("write Go-from-fo frames for %s/%s: %w", fixture.Image.Name, syntax.Name, err)
	}
	return nil
}

func decodeInputDataset(encoded *dataset.Dataset) (*dataset.Dataset, error) {
	decodeInput := encoded.DeepClone()
	photometricInterpretation := decodeInput.TryGetString(tag.PhotometricInterpretation)
	if photometricInterpretation != "YBR_FULL" && photometricInterpretation != "YBR_FULL_422" {
		return decodeInput, nil
	}
	if err := decodeInput.AddOrUpdate(element.NewString(
		tag.PhotometricInterpretation,
		vr.CS,
		[]string{"RGB"},
	)); err != nil {
		return nil, err
	}
	return decodeInput, nil
}

func transcode(source *dataset.Dataset, sourceSyntax, targetSyntax *transfer.Syntax) (*dataset.Dataset, error) {
	transcoder := codec.NewTranscoder(
		sourceSyntax,
		targetSyntax,
		codec.WithCodecRegistry(codec.GetGlobalRegistry()),
		codec.WithStrictDICOMVR(false),
	)
	return transcoder.Transcode(source)
}

func transferSyntax(uid string) (*transfer.Syntax, error) {
	switch uid {
	case transfer.HTJ2KLossless.UID().UID():
		return transfer.HTJ2KLossless, nil
	case transfer.HTJ2KLosslessRPCL.UID().UID():
		return transfer.HTJ2KLosslessRPCL, nil
	case transfer.HTJ2K.UID().UID():
		return transfer.HTJ2K, nil
	default:
		return nil, fmt.Errorf("unsupported transfer syntax %q", uid)
	}
}

func writeDatasetArtifact(root, slashPath string, data *dataset.Dataset, syntax *transfer.Syntax, artifacts map[string]artifactDigest) error {
	if _, err := resolveWithin(root, slashPath); err != nil {
		return fmt.Errorf("DICOM artifact path %q: %w", slashPath, err)
	}
	var content bytes.Buffer
	if err := writer.Write(&content, data, writer.WithTransferSyntax(syntax)); err != nil {
		return fmt.Errorf("serialize DICOM artifact %q: %w", slashPath, err)
	}
	rootFS, err := os.OpenRoot(root)
	if err != nil {
		return fmt.Errorf("open bundle root: %w", err)
	}
	defer func() { _ = rootFS.Close() }()
	if err := rootFS.MkdirAll(path.Dir(slashPath), 0755); err != nil {
		return err
	}
	if err := rootFS.WriteFile(slashPath, content.Bytes(), 0644); err != nil {
		return fmt.Errorf("write DICOM artifact %q: %w", slashPath, err)
	}
	artifacts[slashPath] = newArtifactDigest(slashPath, content.Bytes())
	return nil
}

func writeCodestreamArtifacts(root string, paths []string, data *dataset.Dataset, artifacts map[string]artifactDigest) error {
	pixels, err := imaging.CreatePixelData(data)
	if err != nil {
		return err
	}
	if pixels.FrameCount() != len(paths) {
		return fmt.Errorf("encoded frame count = %d, want %d", pixels.FrameCount(), len(paths))
	}
	for frame, slashPath := range paths {
		content, err := pixels.GetFrame(frame)
		if err != nil {
			return err
		}
		content, err = codestreamThroughEOC(content)
		if err != nil {
			return fmt.Errorf("frame %d: %w", frame, err)
		}
		if err := writeBytesArtifact(root, slashPath, content, artifacts); err != nil {
			return err
		}
	}
	return nil
}

func writeRawArtifacts(root string, paths []string, data *dataset.Dataset, image bundleImage, artifacts map[string]artifactDigest) error {
	pixels, err := imaging.CreatePixelData(data)
	if err != nil {
		return err
	}
	if pixels.FrameCount() != len(paths) {
		return fmt.Errorf("decoded frame count = %d, want %d", pixels.FrameCount(), len(paths))
	}
	expectedLength := image.Width * image.Height * image.SamplesPerPixel * image.BitsAllocated / 8
	for frame, slashPath := range paths {
		content, err := pixels.GetFrame(frame)
		if err != nil {
			return err
		}
		if len(content) < expectedLength {
			return fmt.Errorf("frame %d length = %d, want at least %d", frame, len(content), expectedLength)
		}
		if err := writeBytesArtifact(root, slashPath, content[:expectedLength], artifacts); err != nil {
			return err
		}
	}
	return nil
}

func writeBytesArtifact(root, slashPath string, content []byte, artifacts map[string]artifactDigest) error {
	if _, err := resolveWithin(root, slashPath); err != nil {
		return fmt.Errorf("artifact path %q: %w", slashPath, err)
	}
	rootFS, err := os.OpenRoot(root)
	if err != nil {
		return fmt.Errorf("open bundle root: %w", err)
	}
	defer func() { _ = rootFS.Close() }()
	if err := rootFS.MkdirAll(path.Dir(slashPath), 0755); err != nil {
		return err
	}
	if err := rootFS.WriteFile(slashPath, content, 0644); err != nil {
		return err
	}
	artifacts[slashPath] = newArtifactDigest(slashPath, content)
	return nil
}

func framePaths(prefix string, count int, extension string) []string {
	paths := make([]string, count)
	for frame := range paths {
		paths[frame] = fmt.Sprintf("%s-frame-%04d%s", prefix, frame, extension)
	}
	return paths
}

func validateBundlePaths(root string, loaded bundleManifest) error {
	validatePaths := func(context string, paths ...string) error {
		for _, slashPath := range paths {
			if slashPath == "" {
				continue
			}
			if _, err := resolveWithin(root, slashPath); err != nil {
				return fmt.Errorf("%s path %q must stay within bundle: %w", context, slashPath, err)
			}
		}
		return nil
	}

	for fixtureIndex, fixture := range loaded.Fixtures {
		if err := validatePathSegment(fixture.Image.Name); err != nil {
			return fmt.Errorf("fixture name %q must be a safe bundle path segment: %w", fixture.Image.Name, err)
		}
		if err := validateRequiredBundlePath(root, fmt.Sprintf("fixture %d source DICOM", fixtureIndex), fixture.Source.Dicom); err != nil {
			return err
		}
		if err := validatePaths(
			fmt.Sprintf("fixture %d source frame", fixtureIndex),
			append(append([]string(nil), fixture.Source.Frames...), fixture.Source.EncoderInputFrames...)...,
		); err != nil {
			return err
		}
		for syntaxIndex, syntax := range fixture.Syntaxes {
			if err := validatePathSegment(syntax.Name); err != nil {
				return fmt.Errorf("fixture %q syntax name %q must be a safe bundle path segment: %w", fixture.Image.Name, syntax.Name, err)
			}
			if err := validateRequiredBundlePath(root, fmt.Sprintf("fixture %d syntax %d encoded DICOM", fixtureIndex, syntaxIndex), syntax.EncodedDicom); err != nil {
				return err
			}
			if err := validatePaths(
				fmt.Sprintf("fixture %d syntax %d artifact", fixtureIndex, syntaxIndex),
				syntax.DecodedDicom,
				syntax.GoEncodedDicom,
				syntax.GoFromGoDicom,
				syntax.GoFromFoDicom,
				syntax.FoFromGoDicom,
			); err != nil {
				return err
			}
			artifactPaths := make([]string, 0,
				len(syntax.EncodedFrames)+len(syntax.DecodedFrames)+len(syntax.GoEncodedFrames)+
					len(syntax.GoFromGoFrames)+len(syntax.GoFromFoFrames)+len(syntax.FoFromGoFrames),
			)
			artifactPaths = append(artifactPaths, syntax.EncodedFrames...)
			artifactPaths = append(artifactPaths, syntax.DecodedFrames...)
			artifactPaths = append(artifactPaths, syntax.GoEncodedFrames...)
			artifactPaths = append(artifactPaths, syntax.GoFromGoFrames...)
			artifactPaths = append(artifactPaths, syntax.GoFromFoFrames...)
			artifactPaths = append(artifactPaths, syntax.FoFromGoFrames...)
			if err := validatePaths(fmt.Sprintf("fixture %d syntax %d frame", fixtureIndex, syntaxIndex), artifactPaths...); err != nil {
				return err
			}
		}
	}
	for _, artifact := range loaded.Artifacts {
		if err := validatePaths("declared artifact", artifact.Path); err != nil {
			return err
		}
	}
	return nil
}

func validateRequiredBundlePath(root, context, slashPath string) error {
	if _, err := resolveWithin(root, slashPath); err != nil {
		return fmt.Errorf("%s path %q must stay within bundle: %w", context, slashPath, err)
	}
	return nil
}

func validatePathSegment(value string) error {
	if value == "" || value == "." || value == ".." {
		return fmt.Errorf("segment is empty or reserved")
	}
	if strings.ContainsAny(value, `/\\`) || looksLikeWindowsVolume(value) {
		return fmt.Errorf("segment contains a path separator or volume")
	}
	return nil
}

func resolveWithin(root, slashPath string) (string, error) {
	if slashPath == "" || slashPath == "." || slashPath == ".." {
		return "", fmt.Errorf("path is empty or reserved")
	}
	if strings.ContainsRune(slashPath, '\\') {
		return "", fmt.Errorf("path must use forward slashes")
	}
	if path.IsAbs(slashPath) || filepath.IsAbs(slashPath) || looksLikeWindowsVolume(slashPath) {
		return "", fmt.Errorf("path is absolute")
	}
	cleaned := path.Clean(slashPath)
	if cleaned != slashPath || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("path is not a canonical relative path")
	}

	rootAbsolute, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve bundle root: %w", err)
	}
	candidate, err := filepath.Abs(filepath.Join(rootAbsolute, filepath.FromSlash(slashPath)))
	if err != nil {
		return "", fmt.Errorf("resolve bundle path: %w", err)
	}
	relative, err := filepath.Rel(rootAbsolute, candidate)
	if err != nil {
		return "", fmt.Errorf("compare bundle path: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("path escapes bundle")
	}
	return candidate, nil
}

func looksLikeWindowsVolume(value string) bool {
	return len(value) >= 2 && ((value[0] >= 'a' && value[0] <= 'z') || (value[0] >= 'A' && value[0] <= 'Z')) && value[1] == ':'
}

func parseBundleDICOM(root, slashPath string) (*parser.ParseResult, error) {
	if _, err := resolveWithin(root, slashPath); err != nil {
		return nil, fmt.Errorf("DICOM path %q: %w", slashPath, err)
	}
	rootFS, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("open bundle root: %w", err)
	}
	defer func() { _ = rootFS.Close() }()
	file, err := rootFS.Open(slashPath)
	if err != nil {
		return nil, err
	}
	parsed, parseErr := parser.Parse(file, parser.WithReadOption(parser.ReadAll))
	closeErr := file.Close()
	if parseErr != nil {
		return nil, parseErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return parsed, nil
}

func codestreamThroughEOC(content []byte) ([]byte, error) {
	if len(content) < 4 || !bytes.Equal(content[:2], []byte{0xff, 0x4f}) {
		return nil, fmt.Errorf("codestream is missing SOC")
	}
	for offset := 2; offset+1 < len(content); offset++ {
		if content[offset] == 0xff && content[offset+1] == 0xd9 {
			return append([]byte(nil), content[:offset+2]...), nil
		}
	}
	return nil, fmt.Errorf("codestream is missing EOC")
}

func newArtifactDigest(path string, content []byte) artifactDigest {
	sum := sha256.Sum256(content)
	return artifactDigest{Path: path, Length: int64(len(content)), SHA256: hex.EncodeToString(sum[:])}
}

func readBundleManifest(root string) (bundleManifest, error) {
	rootFS, err := os.OpenRoot(root)
	if err != nil {
		return bundleManifest{}, fmt.Errorf("open bundle root: %w", err)
	}
	defer func() { _ = rootFS.Close() }()
	content, err := rootFS.ReadFile("manifest.json")
	if err != nil {
		return bundleManifest{}, err
	}
	var loaded bundleManifest
	if err := json.Unmarshal(content, &loaded); err != nil {
		return bundleManifest{}, err
	}
	return loaded, nil
}

func writeBundleManifest(root string, loaded bundleManifest) error {
	content, err := json.MarshalIndent(loaded, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	rootFS, err := os.OpenRoot(root)
	if err != nil {
		return fmt.Errorf("open bundle root: %w", err)
	}
	defer func() { _ = rootFS.Close() }()
	return rootFS.WriteFile("manifest.json", content, 0644)
}
