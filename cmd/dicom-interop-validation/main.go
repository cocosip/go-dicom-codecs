// Command dicom-interop-validation performs local process-isolated DICOM codec validation.
package main

import (
	"embed"
	"encoding/binary"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	_ "github.com/cocosip/go-dicom-codecs/jpeg/baseline"
	_ "github.com/cocosip/go-dicom-codecs/jpeg/extended"
	_ "github.com/cocosip/go-dicom-codecs/jpeg/lossless"
	_ "github.com/cocosip/go-dicom-codecs/jpeg/lossless14sv1"
	_ "github.com/cocosip/go-dicom-codecs/jpeg2000/htj2k"
	_ "github.com/cocosip/go-dicom-codecs/jpeg2000/lossless"
	_ "github.com/cocosip/go-dicom-codecs/jpeg2000/lossy"
	_ "github.com/cocosip/go-dicom-codecs/jpegls/lossless"
	"github.com/cocosip/go-dicom-codecs/jpegls/nearlossless"
	_ "github.com/cocosip/go-dicom-codecs/rle"

	"github.com/cocosip/go-dicom/pkg/dicom/parser"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/dicom/writer"
	"github.com/cocosip/go-dicom/pkg/imaging"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

const (
	defaultParallelFormats     = 4
	defaultNativeWorkerProject = "cmd/fo-dicom-native-worker/fo-dicom-native-worker.csproj"

	optionFormat = "--format"
	optionStage  = "--stage"
	optionInput  = "--input"
	optionOutput = "--output"

	stagePrepare = "prepare"
	stageEncode  = "encode"
	stageDecode  = "decode"

	formatJPEGProcess1       = "jpeg-process-1"
	formatJPEGProcess2_4     = "jpeg-process-2-4"
	formatJPEGLosslessSV1    = "jpeg-lossless-14-sv1"
	formatJPEGLSLossless     = "jpeg-ls-lossless"
	formatJPEGLSNearLossless = "jpeg-ls-near-lossless"
)

type options struct {
	format        string
	parallel      int
	workdir       string
	worker        string
	stage         string
	input         string
	output        string
	fixtureDir    string
	nativeProject string
	help          bool
}

type formatDefinition struct {
	key       string
	tolerance int
}

var formatDefinitions = []formatDefinition{
	{key: "rle"},
	{key: formatJPEGProcess1, tolerance: 64},
	{key: formatJPEGProcess2_4, tolerance: 64},
	{key: "jpeg-lossless-14"},
	{key: formatJPEGLosslessSV1},
	{key: formatJPEGLSLossless},
	{key: formatJPEGLSNearLossless, tolerance: 2},
	{key: "jpeg2000-lossless"},
	{key: "jpeg2000-lossy", tolerance: 58},
	{key: "htj2k-lossless"},
	{key: "htj2k-lossless-rpcl"},
	{key: "htj2k-lossy", tolerance: 6},
}

//go:embed fixtures/*.dcm
var embeddedFixtures embed.FS

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "INTEROP|fail|%v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	options, err := parseOptions(args)
	if err != nil {
		return err
	}
	if options.help {
		printUsage()
		return nil
	}
	if options.stage != "" {
		return runStage(options)
	}
	if options.worker != "" {
		return runWorker(options)
	}
	return runOrchestrator(options)
}

func parseOptions(args []string) (options, error) {
	result := options{parallel: defaultParallelFormats, nativeProject: defaultNativeWorkerProject}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--help" || arg == "-h" {
			result.help = true
			return result, nil
		}
		if !strings.HasPrefix(arg, "--") {
			return options{}, fmt.Errorf("unknown argument %q", arg)
		}
		i++
		if i >= len(args) {
			return options{}, fmt.Errorf("%s requires a value", arg)
		}
		value := strings.Trim(strings.TrimSpace(args[i]), "\"'")
		switch arg {
		case optionFormat:
			if !isKnownFormat(value) {
				return options{}, fmt.Errorf("unknown format %q", value)
			}
			result.format = value
		case "--parallel":
			parallel, err := strconv.Atoi(value)
			if err != nil || parallel <= 0 {
				return options{}, fmt.Errorf("--parallel requires a positive integer")
			}
			result.parallel = parallel
		case "--workdir":
			if value == "" {
				return options{}, fmt.Errorf("--workdir requires a directory")
			}
			result.workdir = value
		case "--worker":
			if !isKnownFormat(value) {
				return options{}, fmt.Errorf("unknown worker format %q", value)
			}
			result.worker = value
		case optionStage:
			if value != stagePrepare && value != stageEncode && value != stageDecode {
				return options{}, fmt.Errorf("--stage must be prepare, encode, or decode")
			}
			result.stage = value
		case optionInput:
			result.input = value
		case optionOutput:
			result.output = value
		case "--fixture-dir":
			result.fixtureDir = value
		case "--fo-native-project":
			if value == "" {
				return options{}, fmt.Errorf("--fo-native-project requires a project path")
			}
			result.nativeProject = value
		default:
			return options{}, fmt.Errorf("unknown option %s", arg)
		}
	}
	if result.worker != "" && result.format != "" {
		return options{}, fmt.Errorf("--worker and --format cannot be used together")
	}
	return result, nil
}

func childArgs(stage, format, input, output string) []string {
	return []string{
		optionStage, stage,
		optionFormat, format,
		optionInput, input,
		optionOutput, output,
	}
}

func nativeDecodeArgs(projectPath, input, output string) []string {
	return []string{"run", "--project", projectPath, "--", stageDecode, input, output}
}

func printUsage() {
	fmt.Println("Usage: dicom-interop-validation [--format <key>] [--parallel <n>] [--workdir <path>] [--fo-native-project <path>]")
}

func isKnownFormat(key string) bool {
	for _, definition := range formatDefinitions {
		if definition.key == key {
			return true
		}
	}
	return false
}

func fixtureNames() ([]string, error) {
	paths, err := fs.Glob(embeddedFixtures, "fixtures/*.dcm")
	if err != nil {
		return nil, fmt.Errorf("list embedded fixtures: %w", err)
	}
	names := make([]string, 0, len(paths))
	for _, fixturePath := range paths {
		names = append(names, path.Base(fixturePath))
	}
	sort.Strings(names)
	return names, nil
}

func readFixture(name string) ([]byte, error) {
	data, err := embeddedFixtures.ReadFile(path.Join("fixtures", name))
	if err != nil {
		return nil, fmt.Errorf("read embedded fixture %q: %w", name, err)
	}
	return data, nil
}

func transcodeFile(inputPath, outputPath string, targetSyntax *transfer.Syntax) error {
	registerInteropCodecs()
	parsed, err := parser.ParseFile(inputPath, parser.WithReadOption(parser.ReadAll))
	if err != nil {
		return fmt.Errorf("parse input DICOM: %w", err)
	}
	transcoder := codec.NewTranscoder(
		parsed.TransferSyntax,
		targetSyntax,
		codec.WithCodecRegistry(codec.GetGlobalRegistry()),
		codec.WithStrictDICOMVR(false),
	)
	transcoded, err := transcoder.Transcode(parsed.Dataset)
	if err != nil {
		return fmt.Errorf("transcode %s to %s: %w", parsed.TransferSyntax.UID().UID(), targetSyntax.UID().UID(), err)
	}
	if err := writer.WriteFile(outputPath, transcoded, writer.WithTransferSyntax(targetSyntax)); err != nil {
		return fmt.Errorf("write output DICOM: %w", err)
	}
	return nil
}

func registerInteropCodecs() {
	// PureCodecs validates near-lossless JPEG-LS with AllowedError=2.
	nearlossless.RegisterJPEGLSNearLosslessCodec(2)
}

func formatTransferSyntax(key string) (*transfer.Syntax, error) {
	syntaxes := map[string]*transfer.Syntax{
		"rle":                    transfer.RLELossless,
		formatJPEGProcess1:       transfer.JPEGBaseline8Bit,
		formatJPEGProcess2_4:     transfer.JPEGProcess2_4,
		"jpeg-lossless-14":       transfer.JPEGLossless,
		formatJPEGLosslessSV1:    transfer.JPEGLosslessSV1,
		formatJPEGLSLossless:     transfer.JPEGLSLossless,
		formatJPEGLSNearLossless: transfer.JPEGLSNearLossless,
		"jpeg2000-lossless":      transfer.JPEG2000Lossless,
		"jpeg2000-lossy":         transfer.JPEG2000Lossy,
		"htj2k-lossless":         transfer.HTJ2KLossless,
		"htj2k-lossless-rpcl":    transfer.HTJ2KLosslessRPCL,
		"htj2k-lossy":            transfer.HTJ2K,
	}
	syntax, ok := syntaxes[key]
	if !ok {
		return nil, fmt.Errorf("unknown format %q", key)
	}
	return syntax, nil
}

func runStage(options options) error {
	if options.input == "" || options.output == "" || options.format == "" {
		return fmt.Errorf("--stage requires --format, --input, and --output")
	}
	target := transfer.ExplicitVRLittleEndian
	if options.stage == "encode" {
		var err error
		target, err = formatTransferSyntax(options.format)
		if err != nil {
			return err
		}
	}
	return transcodeFile(options.input, options.output, target)
}

type imageData struct {
	width, height, bitsAllocated, bitsStored, samples, representation uint16
	frames                                                            [][]byte
}

func loadImage(path string) (imageData, error) {
	parsed, err := parser.ParseFile(path, parser.WithReadOption(parser.ReadAll))
	if err != nil {
		return imageData{}, fmt.Errorf("parse %s: %w", path, err)
	}
	pixelData, err := imaging.CreatePixelData(parsed.Dataset)
	if err != nil {
		return imageData{}, fmt.Errorf("read PixelData from %s: %w", path, err)
	}
	info := pixelData.Info
	result := imageData{
		width: info.Width, height: info.Height, bitsAllocated: info.BitsAllocated,
		bitsStored: info.BitsStored, samples: info.SamplesPerPixel, representation: uint16(info.PixelRepresentation),
		frames: make([][]byte, pixelData.FrameCount()),
	}
	for frame := range result.frames {
		result.frames[frame], err = pixelData.GetFrame(frame)
		if err != nil {
			return imageData{}, fmt.Errorf("read frame %d from %s: %w", frame, path, err)
		}
	}
	return result, nil
}

func supportsFormat(image imageData, format string) bool {
	if image.bitsAllocated != 8 && image.bitsAllocated != 16 {
		return false
	}
	if image.samples != 1 && image.samples != 3 {
		return false
	}
	return format != formatJPEGProcess1 && format != formatJPEGProcess2_4 ||
		(image.bitsAllocated == 8 && image.bitsStored == 8)
}

func supportsNativeDecode(image imageData, format string) bool {
	if format != formatJPEGLSLossless && format != formatJPEGLSNearLossless {
		return true
	}
	return len(image.frames) == 1
}

func compareImages(expected, actual imageData, tolerance int) error {
	if expected.width != actual.width || expected.height != actual.height ||
		expected.bitsAllocated != actual.bitsAllocated || expected.bitsStored != actual.bitsStored ||
		expected.samples != actual.samples || expected.representation != actual.representation {
		return fmt.Errorf("decoded image metadata differs")
	}
	if expected.bitsAllocated != 8 && expected.bitsAllocated != 16 {
		return fmt.Errorf("unsupported sample width %d", expected.bitsAllocated)
	}
	if len(expected.frames) != len(actual.frames) {
		return fmt.Errorf("frame count differs: got %d, want %d", len(actual.frames), len(expected.frames))
	}
	bytesPerSample := int(expected.bitsAllocated / 8)
	for frame := range expected.frames {
		if len(expected.frames[frame]) != len(actual.frames[frame]) {
			return fmt.Errorf("frame %d length differs: got %d, want %d", frame, len(actual.frames[frame]), len(expected.frames[frame]))
		}
		for offset := 0; offset < len(expected.frames[frame]); offset += bytesPerSample {
			want := readSample(expected.frames[frame][offset:], expected.bitsAllocated, expected.representation)
			got := readSample(actual.frames[frame][offset:], actual.bitsAllocated, actual.representation)
			difference := want - got
			if difference < 0 {
				difference = -difference
			}
			if difference > tolerance {
				return fmt.Errorf("frame %d sample %d differs by %d, tolerance %d: got %d, want %d", frame, offset/bytesPerSample, difference, tolerance, got, want)
			}
		}
	}
	return nil
}

func readSample(data []byte, bitsAllocated, representation uint16) int {
	if bitsAllocated == 8 {
		if representation != 0 {
			return int(int8(data[0]))
		}
		return int(data[0])
	}
	value := binary.LittleEndian.Uint16(data)
	if representation != 0 {
		return int(int16(value))
	}
	return int(value)
}

func runOrchestrator(options options) error {
	if _, err := os.Stat(options.nativeProject); err != nil {
		return fmt.Errorf("fo-dicom Native worker project is unavailable at %q: %w", options.nativeProject, err)
	}
	workdir := options.workdir
	temporary := workdir == ""
	if temporary {
		var err error
		workdir, err = os.MkdirTemp("", "dicom-interop-validation-")
		if err != nil {
			return fmt.Errorf("create temporary workdir: %w", err)
		}
	} else if err := os.MkdirAll(workdir, 0755); err != nil {
		return fmt.Errorf("create workdir: %w", err)
	}
	failed := true
	defer func() {
		if temporary && !failed {
			_ = os.RemoveAll(workdir)
		}
	}()

	fixtureDir := filepath.Join(workdir, "fixtures")
	if err := extractFixtures(fixtureDir); err != nil {
		return err
	}
	selected := formatDefinitions
	if options.format != "" {
		selected = nil
		for _, definition := range formatDefinitions {
			if definition.key == options.format {
				selected = append(selected, definition)
			}
		}
	}
	fmt.Printf("INTEROP|start|formats=%d|parallel=%d|workdir=%s\n", len(selected), options.parallel, workdir)
	results := make(chan error, len(selected))
	semaphore := make(chan struct{}, options.parallel)
	var group sync.WaitGroup
	for _, definition := range selected {
		definition := definition
		group.Add(1)
		go func() {
			defer group.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			results <- startWorker(definition.key, fixtureDir, workdir, options.nativeProject)
		}()
	}
	group.Wait()
	close(results)
	failures := 0
	for err := range results {
		if err != nil {
			failures++
			fmt.Fprintln(os.Stderr, err)
		}
	}
	fmt.Printf("INTEROP|summary|formats=%d|failed=%d\n", len(selected), failures)
	if failures != 0 {
		return fmt.Errorf("interop validation failed; artifacts retained at %s", workdir)
	}
	failed = false
	return nil
}

func extractFixtures(destination string) error {
	if err := os.MkdirAll(destination, 0755); err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	names, err := fixtureNames()
	if err != nil {
		return err
	}
	for _, name := range names {
		data, err := readFixture(name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(destination, name), data, 0600); err != nil {
			return fmt.Errorf("extract fixture %s: %w", name, err)
		}
	}
	return nil
}

func startWorker(format, fixtureDir, workdir, nativeProject string) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable for %s: %w", format, err)
	}
	command := exec.Command(executable, "--worker", format, "--fixture-dir", fixtureDir, "--workdir", workdir, "--fo-native-project", nativeProject)
	output, err := command.CombinedOutput()
	if len(output) > 0 {
		fmt.Print(string(output))
	}
	if err != nil {
		return fmt.Errorf("INTEROP|worker|format=%s|error=%w", format, err)
	}
	return nil
}

func runWorker(options options) error {
	if options.fixtureDir == "" || options.workdir == "" {
		return fmt.Errorf("--worker requires --fixture-dir and --workdir")
	}
	if _, err := os.Stat(options.nativeProject); err != nil {
		return fmt.Errorf("fo-dicom Native worker project is unavailable at %q: %w", options.nativeProject, err)
	}
	definition, err := findFormat(options.worker)
	if err != nil {
		return err
	}
	names, err := fixtureNames()
	if err != nil {
		return err
	}
	executed := 0
	for _, name := range names {
		sourcePath := filepath.Join(options.fixtureDir, name)
		fixtureDir := filepath.Join(options.workdir, definition.key, strings.TrimSuffix(name, filepath.Ext(name)))
		if err := os.MkdirAll(fixtureDir, 0755); err != nil {
			return fmt.Errorf("create fixture workdir: %w", err)
		}
		prepared := filepath.Join(fixtureDir, "prepared.dcm")
		encoded := filepath.Join(fixtureDir, "encoded.dcm")
		decoded := filepath.Join(fixtureDir, "decoded.dcm")
		if err := runNativeDecode(options.nativeProject, sourcePath, prepared); err != nil {
			return fmt.Errorf("%s %s prepare: %w", definition.key, name, err)
		}
		source, err := loadImage(prepared)
		if err != nil {
			return err
		}
		if !supportsFormat(source, definition.key) {
			fmt.Printf("INTEROP|skip|fixture=%s|format=%s|reason=unsupported-image\n", name, definition.key)
			continue
		}
		if !supportsNativeDecode(source, definition.key) {
			fmt.Printf("INTEROP|skip|fixture=%s|format=%s|direction=go-to-native|reason=native-jpeg-ls-multiframe-baseline-corruption\n", name, definition.key)
			continue
		}
		if err := runChildStage(stageEncode, definition.key, prepared, encoded); err != nil {
			return fmt.Errorf("%s %s encode: %w", definition.key, name, err)
		}
		if err := runNativeDecode(options.nativeProject, encoded, decoded); err != nil {
			return fmt.Errorf("%s %s decode: %w", definition.key, name, err)
		}
		actual, err := loadImage(decoded)
		if err != nil {
			return err
		}
		if err := compareImages(source, actual, definition.tolerance); err != nil {
			return fmt.Errorf("%s %s: %w", definition.key, name, err)
		}
		executed++
	}
	fmt.Printf("INTEROP|pass|format=%s|fixtures=%d\n", definition.key, executed)
	return nil
}

func findFormat(key string) (formatDefinition, error) {
	for _, definition := range formatDefinitions {
		if definition.key == key {
			return definition, nil
		}
	}
	return formatDefinition{}, fmt.Errorf("unknown format %q", key)
}

func runChildStage(stage, format, input, output string) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	command := exec.Command(executable, childArgs(stage, format, input, output)...)
	outputText, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("child %s failed: %w: %s", stage, err, strings.TrimSpace(string(outputText)))
	}
	return nil
}

func runNativeDecode(projectPath, input, output string) error {
	command := exec.Command("dotnet", nativeDecodeArgs(projectPath, input, output)...)
	outputText, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("fo-dicom Native decode failed: %w: %s", err, strings.TrimSpace(string(outputText)))
	}
	return nil
}

func compareFrames(expected, actual [][]byte, tolerance int) error {
	if len(expected) != len(actual) {
		return fmt.Errorf("frame count differs: got %d, want %d", len(actual), len(expected))
	}
	for frame := range expected {
		if len(expected[frame]) != len(actual[frame]) {
			return fmt.Errorf("frame %d length differs: got %d, want %d", frame, len(actual[frame]), len(expected[frame]))
		}
		for offset := range expected[frame] {
			difference := int(expected[frame][offset]) - int(actual[frame][offset])
			if difference < 0 {
				difference = -difference
			}
			if difference > tolerance {
				return fmt.Errorf("frame %d byte %d differs by %d, tolerance %d: got %d, want %d", frame, offset, difference, tolerance, actual[frame][offset], expected[frame][offset])
			}
		}
	}
	return nil
}
