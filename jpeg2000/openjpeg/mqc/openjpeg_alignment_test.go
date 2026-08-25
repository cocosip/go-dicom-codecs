package mqc

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

type openjpegMQCEntry struct {
	qe      uint32
	mps     int
	nmpsIdx int
	nlpsIdx int
}

type openjpegMQCTables struct {
	qe     [47]uint32
	nmps   [47]uint8
	nlps   [47]uint8
	switchB [47]uint8
}

func loadOpenJPEGMQCTables(t *testing.T) openjpegMQCTables {
	t.Helper()
	source := readOpenJPEGFile(t, "mqc.c")
	entries := parseOpenJPEGMQCEntries(t, source)
	return buildOpenJPEGMQCTables(t, entries)
}

func parseOpenJPEGMQCEntries(t *testing.T, source string) []openjpegMQCEntry {
	t.Helper()
	start := strings.Index(source, "static const opj_mqc_state_t mqc_states")
	if start == -1 {
		t.Fatal("mqc_states table not found in OpenJPEG mqc.c")
	}
	block := source[start:]
	open := strings.Index(block, "{")
	if open == -1 {
		t.Fatal("mqc_states opening brace not found")
	}
	closeIdx := strings.Index(block, "};")
	if closeIdx == -1 {
		t.Fatal("mqc_states closing brace not found")
	}
	block = block[open+1 : closeIdx]

	re := regexp.MustCompile(`\{\s*(0x[0-9a-fA-F]+)\s*,\s*([01])\s*,\s*&mqc_states\[(\d+)\]\s*,\s*&mqc_states\[(\d+)\]\s*\}`)
	matches := re.FindAllStringSubmatch(block, -1)
	if len(matches) != 94 {
		t.Fatalf("expected 94 MQC state entries, got %d", len(matches))
	}

	entries := make([]openjpegMQCEntry, 0, len(matches))
	for _, match := range matches {
		qe, err := strconv.ParseUint(match[1], 0, 32)
		if err != nil {
			t.Fatalf("invalid Qe value %q: %v", match[1], err)
		}
		mps, err := strconv.Atoi(match[2])
		if err != nil {
			t.Fatalf("invalid MPS value %q: %v", match[2], err)
		}
		nmpsIdx, err := strconv.Atoi(match[3])
		if err != nil {
			t.Fatalf("invalid NMPS index %q: %v", match[3], err)
		}
		nlpsIdx, err := strconv.Atoi(match[4])
		if err != nil {
			t.Fatalf("invalid NLPS index %q: %v", match[4], err)
		}
		entries = append(entries, openjpegMQCEntry{
			qe:      uint32(qe),
			mps:     mps,
			nmpsIdx: nmpsIdx,
			nlpsIdx: nlpsIdx,
		})
	}
	return entries
}

func buildOpenJPEGMQCTables(t *testing.T, entries []openjpegMQCEntry) openjpegMQCTables {
	t.Helper()
	if len(entries) != 94 {
		t.Fatalf("expected 94 MQC entries, got %d", len(entries))
	}

	var tables openjpegMQCTables
	for i := 0; i < 47; i++ {
		e0 := entries[i*2]
		e1 := entries[i*2+1]
		if e0.qe != e1.qe {
			t.Fatalf("state %d Qe mismatch: 0x%04X vs 0x%04X", i, e0.qe, e1.qe)
		}
		if e0.mps != 0 || e1.mps != 1 {
			t.Fatalf("state %d unexpected MPS values: %d/%d", i, e0.mps, e1.mps)
		}

		nmps0 := e0.nmpsIdx / 2
		nmps1 := e1.nmpsIdx / 2
		nlps0 := e0.nlpsIdx / 2
		nlps1 := e1.nlpsIdx / 2
		if nmps0 != nmps1 || nlps0 != nlps1 {
			t.Fatalf("state %d transition mismatch: nmps %d/%d nlps %d/%d", i, nmps0, nmps1, nlps0, nlps1)
		}

		switch0 := 0
		if (e0.nlpsIdx%2) != e0.mps {
			switch0 = 1
		}
		switch1 := 0
		if (e1.nlpsIdx%2) != e1.mps {
			switch1 = 1
		}
		if switch0 != switch1 {
			t.Fatalf("state %d switch mismatch: %d vs %d", i, switch0, switch1)
		}

		tables.qe[i] = e0.qe
		tables.nmps[i] = uint8(nmps0)
		tables.nlps[i] = uint8(nlps0)
		tables.switchB[i] = uint8(switch0)
	}

	return tables
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
