package t1

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestOpenJPEGLUTAlignment(t *testing.T) {
	zc := parseOpenJPEGLUT(t, "lut_ctxno_zc", 2048)
	sc := parseOpenJPEGLUT(t, "lut_ctxno_sc", 256)
	spb := parseOpenJPEGLUT(t, "lut_spb", 256)

	for i, v := range zc {
		if lutCtxnoZc[i] != uint8(v) {
			t.Fatalf("lut_ctxno_zc[%d] mismatch: got %d want %d", i, lutCtxnoZc[i], v)
		}
	}
	for i, v := range sc {
		if lutCtxnoSc[i] != uint8(v) {
			t.Fatalf("lut_ctxno_sc[%d] mismatch: got 0x%X want 0x%X", i, lutCtxnoSc[i], v)
		}
	}
	for i, v := range spb {
		if lutSpb[i] != v {
			t.Fatalf("lut_spb[%d] mismatch: got %d want %d", i, lutSpb[i], v)
		}
	}
}

func TestZeroCodingContextLogic(t *testing.T) {
	testCases := []struct {
		name     string
		flags    uint32
		expected uint8
	}{
		{name: "NoNeighbors", flags: 0, expected: 0},
		{name: "WestOnly", flags: T1SigW, expected: 5},
		{name: "NorthOnly", flags: T1SigN, expected: 3},
		{name: "WestEast", flags: T1SigW | T1SigE, expected: 8},
		{name: "NorthSouth", flags: T1SigN | T1SigS, expected: 4},
		{name: "CardinalFour", flags: T1SigN | T1SigS | T1SigW | T1SigE, expected: 8},
		{
			name:     "AllNeighbors",
			flags:    T1SigN | T1SigS | T1SigW | T1SigE | T1SigNW | T1SigNE | T1SigSW | T1SigSE,
			expected: 8,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := getZeroCodingContext(tc.flags, 0); got != tc.expected {
				t.Fatalf("context mismatch: got %d want %d", got, tc.expected)
			}
		})
	}
}

func TestMagnitudeRefinementContext(t *testing.T) {
	testCases := []struct {
		name     string
		flags    uint32
		expected uint8
	}{
		{name: "NoNeighbors", flags: 0, expected: 14},
		{name: "AnyNeighbor", flags: T1SigW, expected: 15},
		{name: "RefinedNoNeighbors", flags: T1Refine, expected: 16},
		{name: "RefinedWithNeighbors", flags: T1Refine | T1SigW, expected: 16},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := getMagRefinementContext(tc.flags); got != tc.expected {
				t.Fatalf("context mismatch: got %d want %d", got, tc.expected)
			}
		})
	}
}

func TestSignCodingContextExtraction(t *testing.T) {
	testCases := []struct {
		name     string
		flags    uint32
		expected uint8
	}{
		{
			name:     "AllPositive",
			flags:    T1SigE | T1SigW | T1SigN | T1SigS,
			expected: 0xD,
		},
		{
			name:     "EastWestNegative",
			flags:    T1SigE | T1SigW | T1SignE | T1SignW,
			expected: 0xC,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := getSignCodingContext(tc.flags); got != tc.expected {
				t.Fatalf("context mismatch: got %d want %d", got, tc.expected)
			}
		})
	}
}

func TestContextConstants(t *testing.T) {
	tests := []struct {
		name  string
		start int
		end   int
		count int
	}{
		{name: "ZeroCoding", start: CTXZCSTART, end: CTXZCEND, count: 9},
		{name: "SignCoding", start: CTXSCSTART, end: CTXSCEND, count: 5},
		{name: "MagnitudeRefinement", start: CTXMRSTART, end: CTXMREND, count: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.end - tt.start + 1; got != tt.count {
				t.Fatalf("context count mismatch: got %d want %d", got, tt.count)
			}
		})
	}

	if NUMCONTEXTS != 19 {
		t.Fatalf("NUM_CONTEXTS = %d, want 19", NUMCONTEXTS)
	}
	if CTXRL != 17 {
		t.Fatalf("CTX_RL = %d, want 17", CTXRL)
	}
	if CTXUNI != 18 {
		t.Fatalf("CTX_UNI = %d, want 18", CTXUNI)
	}
}

func TestStateFlagDefinitions(t *testing.T) {
	if T1Sig != 0x0001 {
		t.Fatalf("T1Sig = 0x%04X, want 0x0001", T1Sig)
	}
	if T1Refine != 0x0002 {
		t.Fatalf("T1Refine = 0x%04X, want 0x0002", T1Refine)
	}
	if T1Visit != 0x0004 {
		t.Fatalf("T1Visit = 0x%04X, want 0x0004", T1Visit)
	}

	expected := []struct {
		flag uint32
		name string
	}{
		{T1SigN, "T1SigN"},
		{T1SigS, "T1SigS"},
		{T1SigW, "T1SigW"},
		{T1SigE, "T1SigE"},
		{T1SigNW, "T1SigNW"},
		{T1SigNE, "T1SigNE"},
		{T1SigSW, "T1SigSW"},
		{T1SigSE, "T1SigSE"},
	}

	var mask uint32
	for _, n := range expected {
		if n.flag == 0 {
			t.Fatalf("%s flag is zero", n.name)
		}
		if n.flag&(n.flag-1) != 0 {
			t.Fatalf("%s flag is not a single bit: 0x%X", n.name, n.flag)
		}
		mask |= n.flag
	}

	if T1SigNeighbors != mask {
		t.Fatalf("T1SigNeighbors = 0x%X, want 0x%X", T1SigNeighbors, mask)
	}
}

func parseOpenJPEGLUT(t *testing.T, name string, expectedLen int) []int {
	t.Helper()
	source := readOpenJPEGFile(t, "t1_luts.h")
	start := strings.Index(source, name)
	if start == -1 {
		t.Fatalf("OpenJPEG %s not found", name)
	}
	section := source[start:]
	open := strings.Index(section, "{")
	if open == -1 {
		t.Fatalf("OpenJPEG %s opening brace not found", name)
	}
	endIdx := strings.Index(section, "};")
	if endIdx == -1 {
		t.Fatalf("OpenJPEG %s closing brace not found", name)
	}
	section = section[open+1 : endIdx]

	re := regexp.MustCompile(`0x[0-9a-fA-F]+|\d+`)
	matches := re.FindAllString(section, -1)
	values := make([]int, 0, len(matches))
	for _, match := range matches {
		value, err := strconv.ParseInt(match, 0, 32)
		if err != nil {
			t.Fatalf("invalid %s value %q: %v", name, match, err)
		}
		values = append(values, int(value))
	}
	if expectedLen > 0 && len(values) != expectedLen {
		t.Fatalf("OpenJPEG %s length = %d, want %d", name, len(values), expectedLen)
	}
	return values
}

func readOpenJPEGFile(t *testing.T, name string) string {
	t.Helper()
	root := findRepoRoot(t)
	path := filepath.Join(root, "fo-dicom-codec-code", "Native", "Common", "OpenJPEG", name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("OpenJPEG source not available (fo-dicom-codec-code not checked out): %s", path)
		}
		t.Fatalf("read OpenJPEG file %s: %v", path, err)
	}
	return string(data)
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to determine test file location")
	}
	dir := filepath.Dir(file)
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("go.mod not found while locating repo root")
	return ""
}
