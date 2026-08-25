// Command dicom-interop-validation validates a committed HTJ2K interoperability bundle offline.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultBundlePath    = "test-data/htj2k/interop-v1"
	acceptedVersionRange = "[6.0.0-beta1, 7.0.0)"
)

type manifest struct {
	SchemaVersion int        `json:"schemaVersion"`
	Codec         provenance `json:"codec"`
	Fixtures      []fixture  `json:"fixtures"`
	Artifacts     []artifact `json:"artifacts"`
}

type provenance struct {
	AssemblyName   string `json:"assemblyName"`
	ManagedVersion string `json:"managedVersion"`
	SourceCommit   string `json:"sourceCommit"`
}

type fixture struct {
	Image    imageMetadata `json:"image"`
	Source   sourceFiles   `json:"source"`
	Syntaxes []syntaxFiles `json:"syntaxes"`
}

type imageMetadata struct {
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

type sourceFiles struct {
	Dicom              string   `json:"dicom"`
	Frames             []string `json:"frames"`
	EncoderInputFrames []string `json:"encoderInputFrames"`
}

type syntaxFiles struct {
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

type artifact struct {
	Path   string `json:"path"`
	Length int64  `json:"length"`
	SHA256 string `json:"sha256"`
}

type commandOptions struct {
	bundlePath string
	help       bool
}

type matrixCoverage struct {
	monoUnsigned8  bool
	monoUnsigned16 bool
	monoSigned16   bool
	rgbUnsigned8   bool
	rgbUnsigned16  bool
	ybrFull        bool
	ybrFull422     bool
	oddDimension   bool
	multipleFrames bool
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "interop bundle validation failed: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	options, err := parseOptions(args)
	if err != nil {
		return err
	}
	if options.help {
		if _, err := fmt.Fprintln(output, "Usage: dicom-interop-validation [--bundle <directory>]"); err != nil {
			return fmt.Errorf("write help output: %w", err)
		}
		return nil
	}
	if err := validateBundle(options.bundlePath); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "validated HTJ2K interoperability bundle %s\n", options.bundlePath); err != nil {
		return fmt.Errorf("write validation output: %w", err)
	}
	return nil
}

func parseOptions(args []string) (commandOptions, error) {
	options := commandOptions{bundlePath: defaultBundlePath}
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--help", "-h":
			options.help = true
			return options, nil
		case "--bundle":
			option := args[index]
			index++
			if index >= len(args) || strings.TrimSpace(args[index]) == "" {
				return commandOptions{}, fmt.Errorf("%s requires a directory", option)
			}
			options.bundlePath = args[index]
		default:
			return commandOptions{}, fmt.Errorf("unknown argument %q", args[index])
		}
	}
	return options, nil
}

func validateBundle(root string) error {
	loaded, err := readManifest(root)
	if err != nil {
		return err
	}
	if loaded.SchemaVersion != 2 {
		return fmt.Errorf("schema version = %d, want 2", loaded.SchemaVersion)
	}
	if err := validateProvenance(loaded); err != nil {
		return err
	}

	artifacts, err := indexArtifacts(loaded.Artifacts)
	if err != nil {
		return err
	}
	references, err := validateFixtures(loaded.Fixtures, artifacts)
	if err != nil {
		return err
	}
	for referencedPath := range references {
		if _, exists := artifacts[canonicalPathKey(referencedPath)]; !exists {
			return fmt.Errorf("referenced artifact %q is not declared", referencedPath)
		}
	}
	for key, declared := range artifacts {
		if _, exists := references[declared.Path]; !exists {
			return fmt.Errorf("artifact %q is not referenced by a fixture direction", declared.Path)
		}
		if key != canonicalPathKey(declared.Path) {
			return fmt.Errorf("artifact %q has a non-canonical path key", declared.Path)
		}
	}
	if err := validateArtifactFiles(root, artifacts); err != nil {
		return err
	}
	return nil
}

func readManifest(root string) (manifest, error) {
	if strings.TrimSpace(root) == "" {
		return manifest{}, fmt.Errorf("bundle directory is required")
	}
	content, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		return manifest{}, fmt.Errorf("read manifest: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var loaded manifest
	if err := decoder.Decode(&loaded); err != nil {
		return manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return manifest{}, fmt.Errorf("decode manifest: trailing JSON value")
		}
		return manifest{}, fmt.Errorf("decode manifest trailing data: %w", err)
	}
	return loaded, nil
}

func validateProvenance(loaded manifest) error {
	if loaded.Codec.AssemblyName != "fo-dicom.Codecs" {
		return fmt.Errorf("codec assembly = %q, want fo-dicom.Codecs", loaded.Codec.AssemblyName)
	}
	if !isAcceptedManagedVersion(loaded.Codec.ManagedVersion) {
		return fmt.Errorf("managed version %q is outside %s", loaded.Codec.ManagedVersion, acceptedVersionRange)
	}
	if !isLowerHex(loaded.Codec.SourceCommit, 40) {
		return fmt.Errorf("source commit must be 40 lowercase hexadecimal characters")
	}
	managedCommit, ok := managedVersionCommit(loaded.Codec.ManagedVersion)
	if !ok {
		return fmt.Errorf("managed version commit metadata must be 40 lowercase hexadecimal characters")
	}
	if managedCommit != loaded.Codec.SourceCommit {
		return fmt.Errorf("managed version commit metadata %s does not match source commit %s", managedCommit, loaded.Codec.SourceCommit)
	}
	return nil
}

func indexArtifacts(declared []artifact) (map[string]artifact, error) {
	if len(declared) == 0 {
		return nil, fmt.Errorf("manifest contains no artifacts")
	}
	indexed := make(map[string]artifact, len(declared))
	for _, item := range declared {
		if err := validateRelativePath(item.Path); err != nil {
			return nil, fmt.Errorf("artifact path %q: %w", item.Path, err)
		}
		key := canonicalPathKey(item.Path)
		if _, exists := indexed[key]; exists {
			return nil, fmt.Errorf("duplicate artifact path %q", item.Path)
		}
		if item.Length < 0 {
			return nil, fmt.Errorf("artifact %q has negative length", item.Path)
		}
		if !isLowerHex(item.SHA256, 64) {
			return nil, fmt.Errorf("artifact %q SHA256 must be 64 lowercase hexadecimal characters", item.Path)
		}
		indexed[key] = item
	}
	return indexed, nil
}

func validateFixtures(fixtures []fixture, artifacts map[string]artifact) (map[string]struct{}, error) {
	if len(fixtures) == 0 {
		return nil, fmt.Errorf("manifest contains no fixtures")
	}
	references := make(map[string]struct{}, len(artifacts))
	fixtureNames := make(map[string]struct{}, len(fixtures))
	coverage := matrixCoverage{}
	for fixtureIndex, item := range fixtures {
		if err := validateImage(item.Image); err != nil {
			return nil, fmt.Errorf("fixture %d: %w", fixtureIndex, err)
		}
		nameKey := strings.ToLower(item.Image.Name)
		if _, exists := fixtureNames[nameKey]; exists {
			return nil, fmt.Errorf("duplicate fixture name %q", item.Image.Name)
		}
		fixtureNames[nameKey] = struct{}{}
		updateCoverage(&coverage, item.Image)

		if len(item.Source.Frames) != item.Image.FrameCount {
			return nil, fmt.Errorf("fixture %q source frames = %d, want %d", item.Image.Name, len(item.Source.Frames), item.Image.FrameCount)
		}
		if len(item.Source.EncoderInputFrames) != item.Image.FrameCount {
			return nil, fmt.Errorf("fixture %q encoder input frames = %d, want %d", item.Image.Name, len(item.Source.EncoderInputFrames), item.Image.FrameCount)
		}
		if err := addReference(references, item.Source.Dicom, "sources/", ".dcm"); err != nil {
			return nil, fmt.Errorf("fixture %q source DICOM: %w", item.Image.Name, err)
		}
		for frame, framePath := range item.Source.Frames {
			if err := addReference(references, framePath, "sources/frames/", ".raw"); err != nil {
				return nil, fmt.Errorf("fixture %q source frame %d: %w", item.Image.Name, frame, err)
			}
			expected := sourceFrameLength(item.Image)
			if err := requireArtifactLength(artifacts, framePath, expected); err != nil {
				return nil, fmt.Errorf("fixture %q source frame %d: %w", item.Image.Name, frame, err)
			}
		}
		for frame, framePath := range item.Source.EncoderInputFrames {
			prefix, extension := "sources/frames/", ".raw"
			if strings.HasPrefix(item.Image.PhotometricInterpretation, "YBR_") {
				prefix, extension = "sources/encoder-input-frames/", ".rgb"
			}
			if err := addReference(references, framePath, prefix, extension); err != nil {
				return nil, fmt.Errorf("fixture %q encoder input frame %d: %w", item.Image.Name, frame, err)
			}
			if err := requireArtifactLength(artifacts, framePath, decodedFrameLength(item.Image)); err != nil {
				return nil, fmt.Errorf("fixture %q encoder input frame %d: %w", item.Image.Name, frame, err)
			}
		}

		if err := validateSyntaxes(item, artifacts, references); err != nil {
			return nil, err
		}
	}
	if err := coverage.validate(); err != nil {
		return nil, err
	}
	return references, nil
}

func validateSyntaxes(item fixture, artifacts map[string]artifact, references map[string]struct{}) error {
	required := map[string]struct {
		name     string
		lossless bool
	}{
		"1.2.840.10008.1.2.4.201": {name: "htj2k-lossless", lossless: true},
		"1.2.840.10008.1.2.4.202": {name: "htj2k-lossless-rpcl", lossless: true},
		"1.2.840.10008.1.2.4.203": {name: "htj2k-lossy", lossless: false},
	}
	seen := make(map[string]struct{}, len(item.Syntaxes))
	for _, syntax := range item.Syntaxes {
		definition, exists := required[syntax.TransferSyntaxUID]
		if !exists {
			return fmt.Errorf("fixture %q has unsupported transfer syntax %q", item.Image.Name, syntax.TransferSyntaxUID)
		}
		if _, duplicate := seen[syntax.TransferSyntaxUID]; duplicate {
			return fmt.Errorf("fixture %q has duplicate transfer syntax %q", item.Image.Name, syntax.TransferSyntaxUID)
		}
		seen[syntax.TransferSyntaxUID] = struct{}{}
		if err := validateSyntax(item, syntax, definition, artifacts, references); err != nil {
			return err
		}
	}
	for uid := range required {
		if _, exists := seen[uid]; !exists {
			return fmt.Errorf("fixture %q is missing transfer syntax %s direction", item.Image.Name, uid)
		}
	}
	return nil
}

func validateSyntax(
	item fixture,
	syntax syntaxFiles,
	definition struct {
		name     string
		lossless bool
	},
	artifacts map[string]artifact,
	references map[string]struct{},
) error {
	if syntax.Name != definition.name || syntax.Lossless != definition.lossless {
		return fmt.Errorf("fixture %q transfer syntax %s metadata is inconsistent", item.Image.Name, syntax.TransferSyntaxUID)
	}
	if len(syntax.EncodedFrames) != item.Image.FrameCount {
		return fmt.Errorf("fixture %q transfer syntax %s encoded frames = %d, want %d", item.Image.Name, syntax.TransferSyntaxUID, len(syntax.EncodedFrames), item.Image.FrameCount)
	}
	if len(syntax.DecodedFrames) != item.Image.FrameCount {
		return fmt.Errorf("fixture %q transfer syntax %s decoded frames = %d, want %d", item.Image.Name, syntax.TransferSyntaxUID, len(syntax.DecodedFrames), item.Image.FrameCount)
	}
	if len(syntax.GoEncodedFrames) != item.Image.FrameCount {
		return fmt.Errorf("fixture %q transfer syntax %s Go encoded frames = %d, want %d", item.Image.Name, syntax.TransferSyntaxUID, len(syntax.GoEncodedFrames), item.Image.FrameCount)
	}
	if len(syntax.GoFromGoFrames) != item.Image.FrameCount {
		return fmt.Errorf("fixture %q transfer syntax %s Go-from-Go frames = %d, want %d", item.Image.Name, syntax.TransferSyntaxUID, len(syntax.GoFromGoFrames), item.Image.FrameCount)
	}
	if len(syntax.GoFromFoFrames) != item.Image.FrameCount {
		return fmt.Errorf("fixture %q transfer syntax %s Go-from-fo frames = %d, want %d", item.Image.Name, syntax.TransferSyntaxUID, len(syntax.GoFromFoFrames), item.Image.FrameCount)
	}
	hasFoFromGoDicom := strings.TrimSpace(syntax.FoFromGoDicom) != ""
	hasFoFromGoFrames := len(syntax.FoFromGoFrames) != 0
	if hasFoFromGoDicom != hasFoFromGoFrames {
		return fmt.Errorf("fixture %q transfer syntax %s fo-from-Go frames and DICOM must be provided together", item.Image.Name, syntax.TransferSyntaxUID)
	}
	if hasFoFromGoFrames && len(syntax.FoFromGoFrames) != item.Image.FrameCount {
		return fmt.Errorf("fixture %q transfer syntax %s fo-from-Go frames = %d, want %d", item.Image.Name, syntax.TransferSyntaxUID, len(syntax.FoFromGoFrames), item.Image.FrameCount)
	}
	if err := addReference(references, syntax.EncodedDicom, "fo-encoded/", ".dcm"); err != nil {
		return fmt.Errorf("fixture %q encoded DICOM: %w", item.Image.Name, err)
	}
	if err := addReference(references, syntax.DecodedDicom, "decoded/fo-from-fo/", ".dcm"); err != nil {
		return fmt.Errorf("fixture %q decoded DICOM: %w", item.Image.Name, err)
	}
	if err := addReference(references, syntax.GoEncodedDicom, "go-encoded/", ".dcm"); err != nil {
		return fmt.Errorf("fixture %q Go encoded DICOM: %w", item.Image.Name, err)
	}
	if err := addReference(references, syntax.GoFromGoDicom, "decoded/go-from-go/", ".dcm"); err != nil {
		return fmt.Errorf("fixture %q Go-from-Go DICOM: %w", item.Image.Name, err)
	}
	if err := addReference(references, syntax.GoFromFoDicom, "decoded/go-from-fo/", ".dcm"); err != nil {
		return fmt.Errorf("fixture %q Go-from-fo DICOM: %w", item.Image.Name, err)
	}
	if hasFoFromGoDicom {
		if err := addReference(references, syntax.FoFromGoDicom, "decoded/fo-from-go/", ".dcm"); err != nil {
			return fmt.Errorf("fixture %q fo-from-Go DICOM: %w", item.Image.Name, err)
		}
	}
	for frame, framePath := range syntax.EncodedFrames {
		if err := addReference(references, framePath, "fo-encoded/frames/", ".j2c"); err != nil {
			return fmt.Errorf("fixture %q encoded frame %d: %w", item.Image.Name, frame, err)
		}
	}
	for frame, framePath := range syntax.GoEncodedFrames {
		if err := addReference(references, framePath, "go-encoded/frames/", ".j2c"); err != nil {
			return fmt.Errorf("fixture %q Go encoded frame %d: %w", item.Image.Name, frame, err)
		}
	}
	for frame, framePath := range syntax.DecodedFrames {
		if err := addReference(references, framePath, "decoded/fo-from-fo/", ".raw"); err != nil {
			return fmt.Errorf("fixture %q decoded frame %d: %w", item.Image.Name, frame, err)
		}
		if err := requireArtifactLength(artifacts, framePath, decodedFrameLength(item.Image)); err != nil {
			return fmt.Errorf("fixture %q decoded frame %d: %w", item.Image.Name, frame, err)
		}
	}
	decodedDirections := []struct {
		name   string
		prefix string
		paths  []string
	}{
		{name: "Go-from-Go", prefix: "decoded/go-from-go/", paths: syntax.GoFromGoFrames},
		{name: "Go-from-fo", prefix: "decoded/go-from-fo/", paths: syntax.GoFromFoFrames},
		{name: "fo-from-Go", prefix: "decoded/fo-from-go/", paths: syntax.FoFromGoFrames},
	}
	for _, direction := range decodedDirections {
		for frame, framePath := range direction.paths {
			if err := addReference(references, framePath, direction.prefix, ".raw"); err != nil {
				return fmt.Errorf("fixture %q %s frame %d: %w", item.Image.Name, direction.name, frame, err)
			}
			if err := requireArtifactLength(artifacts, framePath, decodedFrameLength(item.Image)); err != nil {
				return fmt.Errorf("fixture %q %s frame %d: %w", item.Image.Name, direction.name, frame, err)
			}
		}
	}
	return nil
}

func validateImage(image imageMetadata) error {
	if strings.TrimSpace(image.Name) == "" {
		return fmt.Errorf("image name is required")
	}
	if image.Width <= 0 || image.Height <= 0 || image.FrameCount <= 0 {
		return fmt.Errorf("image dimensions and frame count must be positive")
	}
	if image.SamplesPerPixel != 1 && image.SamplesPerPixel != 3 {
		return fmt.Errorf("samples per pixel = %d, want 1 or 3", image.SamplesPerPixel)
	}
	if image.BitsAllocated != 8 && image.BitsAllocated != 16 {
		return fmt.Errorf("bits allocated = %d, want 8 or 16", image.BitsAllocated)
	}
	if image.BitsStored <= 0 || image.BitsStored > image.BitsAllocated {
		return fmt.Errorf("bits stored = %d is invalid for %d allocated bits", image.BitsStored, image.BitsAllocated)
	}
	return nil
}

func addReference(references map[string]struct{}, value, requiredPrefix, requiredExtension string) error {
	if err := validateRelativePath(value); err != nil {
		return err
	}
	if !strings.HasPrefix(value, requiredPrefix) || path.Ext(value) != requiredExtension {
		return fmt.Errorf("path %q must be a %s file below %s", value, requiredExtension, requiredPrefix)
	}
	references[value] = struct{}{}
	return nil
}

func requireArtifactLength(artifacts map[string]artifact, artifactPath string, expected int64) error {
	item, exists := artifacts[canonicalPathKey(artifactPath)]
	if !exists {
		return fmt.Errorf("artifact %q is not declared", artifactPath)
	}
	if item.Length != expected {
		return fmt.Errorf("artifact %q length = %d, want %d", artifactPath, item.Length, expected)
	}
	return nil
}

func sourceFrameLength(image imageMetadata) int64 {
	if image.PhotometricInterpretation == "YBR_FULL_422" {
		return int64(image.Width) * int64(image.Height) * 2
	}
	return decodedFrameLength(image)
}

func decodedFrameLength(image imageMetadata) int64 {
	return int64(image.Width) * int64(image.Height) * int64(image.SamplesPerPixel) * int64(image.BitsAllocated/8)
}

func validateArtifactFiles(root string, artifacts map[string]artifact) error {
	rootPath, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve bundle directory: %w", err)
	}
	for _, item := range artifacts {
		filePath := filepath.Join(rootPath, filepath.FromSlash(item.Path))
		info, err := os.Lstat(filePath)
		if err != nil {
			return fmt.Errorf("stat artifact %q: %w", item.Path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("artifact %q is not a regular file", item.Path)
		}
		if info.Size() != item.Length {
			return fmt.Errorf("artifact %q length = %d, manifest = %d", item.Path, info.Size(), item.Length)
		}
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("read artifact %q: %w", item.Path, err)
		}
		sum := sha256.Sum256(content)
		actual := hex.EncodeToString(sum[:])
		if actual != item.SHA256 {
			return fmt.Errorf("artifact %q SHA256 = %s, manifest = %s", item.Path, actual, item.SHA256)
		}
	}

	return filepath.WalkDir(rootPath, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if filePath == rootPath {
			return nil
		}
		relative, err := filepath.Rel(rootPath, filePath)
		if err != nil {
			return err
		}
		slashPath := filepath.ToSlash(relative)
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("bundle path %q is a symbolic link", slashPath)
		}
		if entry.IsDir() {
			lowerName := strings.ToLower(entry.Name())
			if lowerName == "bin" || lowerName == "obj" {
				return fmt.Errorf("bundle contains forbidden build directory %q", slashPath)
			}
			return nil
		}
		if slashPath == "manifest.json" {
			return nil
		}
		if _, exists := artifacts[canonicalPathKey(slashPath)]; !exists {
			return fmt.Errorf("bundle file %q is not declared in the manifest", slashPath)
		}
		return nil
	})
}

func validateRelativePath(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("path is empty")
	}
	if strings.Contains(value, "\\") {
		return fmt.Errorf("path must use forward slashes")
	}
	if path.IsAbs(value) || filepath.IsAbs(value) || filepath.VolumeName(value) != "" || looksLikeWindowsVolume(value) {
		return fmt.Errorf("absolute paths are forbidden")
	}
	cleaned := path.Clean(value)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return fmt.Errorf("path escape is forbidden")
	}
	if cleaned != value || cleaned == "." {
		return fmt.Errorf("path is not canonical")
	}
	return nil
}

func looksLikeWindowsVolume(value string) bool {
	return len(value) >= 2 && ((value[0] >= 'a' && value[0] <= 'z') || (value[0] >= 'A' && value[0] <= 'Z')) && value[1] == ':'
}

func canonicalPathKey(value string) string {
	return strings.ToLower(value)
}

func isAcceptedManagedVersion(value string) bool {
	withoutBuild := strings.SplitN(value, "+", 2)[0]
	parts := strings.SplitN(withoutBuild, "-", 2)
	numbers := strings.Split(parts[0], ".")
	if len(numbers) != 3 {
		return false
	}
	major, majorErr := strconv.Atoi(numbers[0])
	minor, minorErr := strconv.Atoi(numbers[1])
	patch, patchErr := strconv.Atoi(numbers[2])
	if majorErr != nil || minorErr != nil || patchErr != nil || major != 6 || minor < 0 || patch < 0 {
		return false
	}
	if minor > 0 || patch > 0 || len(parts) == 1 {
		return true
	}
	prerelease := strings.ToLower(parts[1])
	if strings.HasPrefix(prerelease, "rc") {
		return true
	}
	if !strings.HasPrefix(prerelease, "beta") {
		return false
	}
	suffix := strings.TrimLeft(prerelease[len("beta"):], ".-")
	beta, err := strconv.Atoi(suffix)
	return err == nil && beta >= 1
}

func managedVersionCommit(value string) (string, bool) {
	_, metadata, found := strings.Cut(value, "+")
	return metadata, found && isLowerHex(metadata, 40)
}

func isLowerHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func updateCoverage(coverage *matrixCoverage, image imageMetadata) {
	mono := image.SamplesPerPixel == 1 && image.PhotometricInterpretation == "MONOCHROME2"
	rgb := image.SamplesPerPixel == 3 && image.PhotometricInterpretation == "RGB"
	coverage.monoUnsigned8 = coverage.monoUnsigned8 || mono && !image.Signed && image.BitsAllocated == 8
	coverage.monoUnsigned16 = coverage.monoUnsigned16 || mono && !image.Signed && image.BitsAllocated == 16
	coverage.monoSigned16 = coverage.monoSigned16 || mono && image.Signed && image.BitsAllocated == 16
	coverage.rgbUnsigned8 = coverage.rgbUnsigned8 || rgb && !image.Signed && image.BitsAllocated == 8
	coverage.rgbUnsigned16 = coverage.rgbUnsigned16 || rgb && !image.Signed && image.BitsAllocated == 16
	coverage.ybrFull = coverage.ybrFull || image.PhotometricInterpretation == "YBR_FULL"
	coverage.ybrFull422 = coverage.ybrFull422 || image.PhotometricInterpretation == "YBR_FULL_422"
	coverage.oddDimension = coverage.oddDimension || image.Width%2 != 0 || image.Height%2 != 0
	coverage.multipleFrames = coverage.multipleFrames || image.FrameCount > 1
}

func (coverage matrixCoverage) validate() error {
	missing := make([]string, 0, 9)
	checks := []struct {
		present bool
		name    string
	}{
		{coverage.monoUnsigned8, "unsigned mono 8-bit"},
		{coverage.monoUnsigned16, "unsigned mono 16-bit"},
		{coverage.monoSigned16, "signed mono 16-bit"},
		{coverage.rgbUnsigned8, "RGB 8-bit"},
		{coverage.rgbUnsigned16, "RGB 16-bit"},
		{coverage.ybrFull, "YBR FULL"},
		{coverage.ybrFull422, "YBR FULL 422"},
		{coverage.oddDimension, "odd dimensions"},
		{coverage.multipleFrames, "multiple frames"},
	}
	for _, check := range checks {
		if !check.present {
			missing = append(missing, check.name)
		}
	}
	if len(missing) != 0 {
		return fmt.Errorf("fixture matrix is missing %s", strings.Join(missing, ", "))
	}
	return nil
}
