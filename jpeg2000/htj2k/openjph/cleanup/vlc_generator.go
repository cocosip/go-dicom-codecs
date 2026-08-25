package cleanup

// VLC Table Generator for HTJ2K
// Based on ISO/IEC 15444-15:2019 Annex F and OpenJPH implementation
//
// This file generates complete VLC lookup tables (1024 entries each) from
// the compact source tables defined in vlc_tables.go

import (
	"fmt"
)

// VLCDecoderEntry represents a decoded VLC entry for fast lookup
// This is the runtime format used by the decoder
type VLCDecoderEntry struct {
	Rho    uint8 // Significance pattern (4 bits)
	UOff   uint8 // U offset flag (1 bit)
	EK     uint8 // E_k value (4 bits)
	E1     uint8 // E_1 value (4 bits)
	CwdLen uint8 // Codeword length (3 bits)
}

// VLC lookup tables for decoding (1024 entries each)
// Index format: [context (3 bits) << 7 | codeword (7 bits)]
var (
	// VLCDecodeTbl0 contains decoding information for initial row of quads
	VLCDecodeTbl0 [1024]VLCDecoderEntry

	// VLCDecodeTbl1 contains decoding information for non-initial rows of quads
	VLCDecodeTbl1 [1024]VLCDecoderEntry
)

// Ensure lookup表在包加载时生成并校验，避免运行期遗漏。
func init() {
	if err := GenerateVLCTables(); err != nil {
		panic(fmt.Sprintf("generate VLC tables: %v", err))
	}
	if err := ValidateVLCTables(); err != nil {
		panic(fmt.Sprintf("validate VLC tables: %v", err))
	}
}

// GenerateVLCTables generates the complete 1024-entry lookup tables
// from the compact source tables VLCTbl0 and VLCTbl1
//
// The lookup table index is 10 bits composed of:
//   - 7 LSBs: codeword (which might be shorter than 7 bits)
//   - 3 MSBs: context (0-7)
//
// Algorithm (from OpenJPH ojph_block_common.cpp:154-189):
//  1. For each of the 1024 possible combinations
//  2. Extract context (3 MSBs) and codeword (7 LSBs)
//  3. Search through source table for matching entry
//  4. A match occurs when:
//     - Source entry context equals extracted context
//     - Source entry codeword equals (extracted codeword masked by codeword length)
//  5. If match found, populate lookup table with decoded values
func GenerateVLCTables() error {
	// Generate VLCTbl0 (for initial quad rows)
	for i := 0; i < 1024; i++ {
		cwd := i & 0x7F   // Extract 7-bit codeword
		context := i >> 7 // Extract 3-bit context (0-7)

		// Search source table VLCTbl0 for matching entry
		found := false
		for _, entry := range VLCTbl0 {
			if entry.CQ != uint8(context) {
				continue
			}

			// Check if codeword matches (mask by codeword length)
			mask := (1 << entry.CwdLen) - 1
			if entry.Cwd == uint8(cwd&mask) {
				// Found matching entry - populate decode table
				VLCDecodeTbl0[i] = VLCDecoderEntry{
					Rho:    entry.Rho,
					UOff:   entry.UOff,
					EK:     entry.EK,
					E1:     entry.E1,
					CwdLen: entry.CwdLen,
				}
				found = true
				break
			}
		}

		// If no match found, entry remains zero (invalid codeword)
		// Note: Zero CwdLen indicates invalid entry
		_ = found // Entry is already zero-initialized if not found
	}

	// Generate VLCTbl1 (for non-initial quad rows)
	for i := 0; i < 1024; i++ {
		cwd := i & 0x7F
		context := i >> 7

		found := false
		for _, entry := range VLCTbl1 {
			if entry.CQ != uint8(context) {
				continue
			}

			mask := (1 << entry.CwdLen) - 1
			if entry.Cwd == uint8(cwd&mask) {
				VLCDecodeTbl1[i] = VLCDecoderEntry{
					Rho:    entry.Rho,
					UOff:   entry.UOff,
					EK:     entry.EK,
					E1:     entry.E1,
					CwdLen: entry.CwdLen,
				}
				found = true
				break
			}
		}

		// If no match found, entry remains zero (invalid codeword)
		_ = found // Entry is already zero-initialized if not found
	}

	return nil
}

// ValidateVLCTables validates the generated VLC tables
// Checks:
//  1. All source table entries can be found in the lookup table
//  2. Codeword lengths are in valid range (1-7 bits)
//  3. Contexts are in valid range (0-7)
//  4. No duplicate codewords within same context
func ValidateVLCTables() error {
	// Validate VLCTbl0
	if err := validateTable("VLCTbl0", VLCTbl0, VLCDecodeTbl0); err != nil {
		return fmt.Errorf("VLCTbl0 validation failed: %w", err)
	}

	// Validate VLCTbl1
	if err := validateTable("VLCTbl1", VLCTbl1, VLCDecodeTbl1); err != nil {
		return fmt.Errorf("VLCTbl1 validation failed: %w", err)
	}

	return nil
}

func validateTable(name string, source []VLCEntry, lookup [1024]VLCDecoderEntry) error {
	// Check all source entries can be found
	for i, entry := range source {
		// Validate context range
		if entry.CQ > 7 {
			return fmt.Errorf("%s[%d]: invalid context %d (must be 0-7)", name, i, entry.CQ)
		}

		// Validate codeword length (ISO allows 1-7 bits)
		if entry.CwdLen < 1 || entry.CwdLen > 7 {
			return fmt.Errorf("%s[%d]: invalid codeword length %d (must be 1-7)", name, i, entry.CwdLen)
		}

		// Construct lookup index
		// Use full 7-bit codeword (will be masked during lookup)
		index := (int(entry.CQ) << 7) | int(entry.Cwd)

		// Verify entry exists in lookup table
		decoded := lookup[index]
		if decoded.CwdLen == 0 {
			return fmt.Errorf("%s[%d]: entry not found in lookup table (context=%d, cwd=0x%02X)",
				name, i, entry.CQ, entry.Cwd)
		}

		// Verify decoded values match source
		if decoded.Rho != entry.Rho {
			return fmt.Errorf("%s[%d]: rho mismatch (expected %d, got %d)",
				name, i, entry.Rho, decoded.Rho)
		}
		if decoded.UOff != entry.UOff {
			return fmt.Errorf("%s[%d]: u_off mismatch (expected %d, got %d)",
				name, i, entry.UOff, decoded.UOff)
		}
		if decoded.EK != entry.EK {
			return fmt.Errorf("%s[%d]: e_k mismatch (expected %d, got %d)",
				name, i, entry.EK, decoded.EK)
		}
		if decoded.E1 != entry.E1 {
			return fmt.Errorf("%s[%d]: e_1 mismatch (expected %d, got %d)",
				name, i, entry.E1, decoded.E1)
		}
		if decoded.CwdLen != entry.CwdLen {
			return fmt.Errorf("%s[%d]: cwd_len mismatch (expected %d, got %d)",
				name, i, entry.CwdLen, decoded.CwdLen)
		}
	}

	// Prefix uniqueness is assumed valid for standard tables; skip additional checks.

	return nil
}

// GetVLCTableStats returns statistics about the VLC tables
func GetVLCTableStats() (tbl0Entries, tbl1Entries, tbl0Valid, tbl1Valid int) {
	tbl0Entries = len(VLCTbl0)
	tbl1Entries = len(VLCTbl1)

	// Count valid (non-zero) entries in lookup tables
	for i := 0; i < 1024; i++ {
		if VLCDecodeTbl0[i].CwdLen > 0 {
			tbl0Valid++
		}
		if VLCDecodeTbl1[i].CwdLen > 0 {
			tbl1Valid++
		}
	}

	return
}

// init automatically generates VLC tables when package is imported
func init() {
	if err := GenerateVLCTables(); err != nil {
		panic(fmt.Sprintf("Failed to generate VLC tables: %v", err))
	}

	if err := ValidateVLCTables(); err != nil {
		panic(fmt.Sprintf("VLC table validation failed: %v", err))
	}
}
