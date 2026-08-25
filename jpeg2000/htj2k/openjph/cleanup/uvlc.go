package cleanup

import "fmt"

// UVLCEncoder implements U-VLC (Unsigned Variable Length Code) encoding
// for HTJ2K as specified in ISO/IEC 15444-15:2019 Annex F.3
//
// The U-VLC encoder converts unsigned residual values into prefix,
// suffix, and extension components according to Table 3.

// UVLCCodeword represents a complete U-VLC codeword
type UVLCCodeword struct {
	Prefix    uint8 // u_pfx: prefix value (1, 2, 3, or 5)
	Suffix    uint8 // u_sfx: suffix value
	Extension uint8 // u_ext: extension value
	PrefixLen int   // lp: number of prefix bits
	SuffixLen int   // ls: number of suffix bits
	ExtLen    int   // le: number of extension bits
}

// EncodeUVLC encodes an unsigned residual value according to Table 3
//
// Parameters:
//   - u: Unsigned residual value (0 for no encoding needed)
//
// Returns:
//   - UVLCCodeword with prefix, suffix, extension components
//
// Reference: ISO/IEC 15444-15:2019, Clause 7.3.6, Table 3
func EncodeUVLC(u uint32) UVLCCodeword {
	if u == 0 {
		// No encoding needed (ulf=0 case)
		return UVLCCodeword{}
	}

	var cwd UVLCCodeword

	// Determine encoding based on Table 3
	if u == 1 {
		// u=1: prefix="1", u_pfx=1, no suffix/ext
		cwd.Prefix = 1
		cwd.PrefixLen = 1
		return cwd
	}

	if u == 2 {
		// u=2: prefix="01", u_pfx=2, no suffix/ext
		cwd.Prefix = 2
		cwd.PrefixLen = 2
		return cwd
	}

	if u >= 3 && u <= 4 {
		// u=3,4: prefix="001", u_pfx=3, 1-bit suffix
		cwd.Prefix = 3
		cwd.PrefixLen = 3
		cwd.Suffix = uint8(u - 3) // u_sfx = u - 3
		cwd.SuffixLen = 1
		return cwd
	}

	if u >= 5 && u <= 32 {
		// u=5-32: prefix="000", u_pfx=5, 5-bit suffix
		cwd.Prefix = 5
		cwd.PrefixLen = 3
		cwd.Suffix = uint8(u - 5) // u_sfx = u - 5
		cwd.SuffixLen = 5
		return cwd
	}

	// u >= 33: prefix="000", u_pfx=5, 5-bit suffix, 4-bit extension
	// Formula: u = 5 + u_sfx + 4*u_ext
	// Where u_sfx >= 28 when extension is needed
	cwd.Prefix = 5
	cwd.PrefixLen = 3

	// For u >= 33:
	// u = 5 + u_sfx + 4*u_ext
	// u - 5 = u_sfx + 4*u_ext
	// Since u_sfx can be at most 31 (5 bits), and extension starts at u_sfx=28:
	// For u=33: u_sfx=28, u_ext=0
	// For u=34: u_sfx=29, u_ext=0
	// ...
	// For u=36: u_sfx=31, u_ext=0
	// For u=37: u_sfx=28, u_ext=1 (wraps around)

	uMinus5 := u - 5

	if uMinus5 < 28 {
		// No extension needed
		cwd.Suffix = uint8(uMinus5)
		cwd.SuffixLen = 5
	} else {
		// Extension needed (u_sfx >= 28)
		// u - 5 = u_sfx + 4*u_ext
		// Solve for u_sfx and u_ext
		cwd.Extension = uint8((uMinus5 - 28) / 4)
		cwd.Suffix = 28 + uint8((uMinus5-28)%4)
		cwd.SuffixLen = 5
		cwd.ExtLen = 4
	}

	return cwd
}

// EncodeUVLCInitialPair encodes unsigned residual for initial line-pair
// where both quads have ulf=1 (uses Formula 4)
//
// Formula (4): u_in = u - 2 where u = 2 + u_pfx + u_sfx + 4*u_ext
//
// Parameters:
//   - u: Unsigned residual value (should be >= 2 for initial pair)
//
// Returns:
//   - UVLCCodeword with encoding for (u - 2)
func EncodeUVLCInitialPair(u uint32) UVLCCodeword {
	if u < 2 {
		// Invalid: initial pair should have u >= 2
		return UVLCCodeword{}
	}

	// Encode (u - 2) using standard U-VLC table
	return EncodeUVLC(u - 2)
}

// EncodePrefixBits converts uPfx value to prefix bit pattern
//
// Prefix patterns (reading order):
//   - uPfx=1: "1"    (1 bit) - first bit is 1
//   - uPfx=2: "01"   (2 bits) - first bit is 0, second bit is 1
//   - uPfx=3: "001"  (3 bits) - first bit is 0, second bit is 0, third bit is 1
//   - uPfx=5: "000"  (3 bits) - first bit is 0, second bit is 0, third bit is 0
//
// Returns bits in LSB-first encoding order for emitVLCBits.
// For LSB-first, bit0 is written first, bit1 second, etc.
// So to produce reading pattern "001", we need:
//   - bit0 (written first, read first) = 0
//   - bit1 (written second, read second) = 0
//   - bit2 (written third, read third) = 1
//   - Value = (1 << 2) = 0x4
func EncodePrefixBits(uPfx uint8) (bits uint32, length int) {
	switch uPfx {
	case 1:
		return 0b1, 1 // "1": bit0=1 → value 0x1
	case 2:
		return 0b10, 2 // "01": bit0=0, bit1=1 → value 0x2
	case 3:
		return 0b100, 3 // "001": bit0=0, bit1=0, bit2=1 → value 0x4
	case 5:
		return 0b000, 3 // "000": bit0=0, bit1=0, bit2=0 → value 0x0
	default:
		return 0, 0
	}
}

// TotalLength returns the total bit length of a U-VLC codeword
func (c *UVLCCodeword) TotalLength() int {
	return c.PrefixLen + c.SuffixLen + c.ExtLen
}

// EncodeToStream encodes the U-VLC codeword to a bit stream writer
//
// The encoding order is:
// 1. Prefix bits
// 2. Suffix bits (if any, in little-endian)
// 3. Extension bits (if any, in little-endian)
//
// Parameters:
//   - writer: Bit stream writer (e.g., VLC encoder's emitVLCBits)
func (c *UVLCCodeword) EncodeToStream(writer BitStreamWriter) error {
	// Encode prefix
	if c.PrefixLen > 0 {
		prefixBits, prefixLen := EncodePrefixBits(c.Prefix)
		if err := writer.WriteBits(prefixBits, prefixLen); err != nil {
			return err
		}
	}

	// Encode suffix (little-endian)
	if c.SuffixLen > 0 {
		if err := writer.WriteBits(uint32(c.Suffix), c.SuffixLen); err != nil {
			return err
		}
	}

	// Encode extension (little-endian)
	if c.ExtLen > 0 {
		if err := writer.WriteBits(uint32(c.Extension), c.ExtLen); err != nil {
			return err
		}
	}

	return nil
}

// BitStreamWriter interface for writing bits to VLC stream
type BitStreamWriter interface {
	WriteBits(bits uint32, length int) error
}

// UVLCEncoder wraps U-VLC encoding functionality for use in Encoder
type UVLCEncoder struct {
	writer BitStreamWriter
}

// NewUVLCEncoder creates a new U-VLC encoder
func NewUVLCEncoder() *UVLCEncoder {
	return &UVLCEncoder{}
}

// SetWriter sets the bit stream writer
func (u *UVLCEncoder) SetWriter(writer BitStreamWriter) {
	u.writer = writer
}

// EncodeUVLC encodes a U-VLC value using the standard formula
func (u *UVLCEncoder) EncodeUVLC(value int, isInitialLinePair bool) error {
	var cwd UVLCCodeword

	if isInitialLinePair {
		cwd = EncodeUVLCInitialPair(uint32(value))
	} else {
		cwd = EncodeUVLC(uint32(value))
	}

	if u.writer != nil {
		return cwd.EncodeToStream(u.writer)
	}
	return nil
}

// EncodeUVLCSimplified encodes U-VLC in simplified mode (for second quad when first has uq>2)
func (u *UVLCEncoder) EncodeUVLCSimplified(value int) error {
	// Simplified encoding: 1 bit encoding (value - 1)
	// value can be 1 or 2, encoded as bit 0 or 1
	bit := value - 1
	if u.writer != nil {
		return u.writer.WriteBits(uint32(bit), 1)
	}
	return nil
}

// EncodeWithTable 尝试使用 UVLC 表驱动编码；若无法匹配返回 ok=false。
// 这里采用表提供的 prefix/suffix 长度进行编码，偏置按公式处理。
func (u *UVLCEncoder) EncodeWithTable(value int, uOff uint8, isInitialPair bool, melBit int) (bool, error) {
	if uOff == 0 || u.writer == nil {
		return false, nil
	}

	// 使用表信息推导前缀/后缀长度（单 quad 视角）
	mode := 1
	if isInitialPair {
		if melBit == 0 {
			mode = 3
		} else {
			mode = 4
		}
	}

	// 选择任意有效表项
	entry := UVLCDecodeEntry(0)
	for head := 0; head < 64; head++ {
		if isInitialPair {
			entry = UVLCTbl0[(mode<<6)|head]
		} else {
			entry = UVLCTbl1[(mode<<6)|head]
		}
		if entry != 0 {
			break
		}
	}
	if entry == 0 {
		return false, nil
	}

	u0Prefix := entry.U0Prefix()
	u0SufLen := entry.U0SuffixLen()

	// 计算有效 u 值（初始行对减 2）
	uVal := value
	if isInitialPair {
		uVal -= 2
	}
	if uVal < 0 {
		return false, nil
	}

	// 写 prefix
	prefixBits, prefixLen := EncodePrefixBits(uint8(u0Prefix))
	if prefixLen == 0 {
		return false, nil
	}
	if err := u.writer.WriteBits(prefixBits, prefixLen); err != nil {
		return true, err
	}

	// 写 suffix
	if u0SufLen > 0 {
		u0suf := uint32(uVal - u0Prefix)
		u0suf &= (1 << u0SufLen) - 1
		if err := u.writer.WriteBits(u0suf, u0SufLen); err != nil {
			return true, err
		}
	}

	// 初始行对扩展（近似：当 uVal >=28 时写 4bit ext）
	if isInitialPair && uVal >= 28 {
		ext := uint32((uVal - 28) / 4)
		if err := u.writer.WriteBits(ext, 4); err != nil {
			return true, err
		}
	}

	return true, nil
}

// EncodePair encodes a UVLC pair matching the decoder's DecodePair logic.
// This uses the precomputed UVLC tables to find the right codeword.
// uOff0/uOff1: offset flags for the two quads.
// u0/u1: unsigned residual values to encode.
// initialPair: true for first row quad pairs.
// melEvent: 0 or 1 (only used when initialPair && both uOff=1).
func (u *UVLCEncoder) EncodePair(uOff0, uOff1 uint8, u0, u1 int, initialPair bool, melEvent int) error {
	if u.writer == nil {
		return nil
	}

	mode := int(uOff0) + 2*int(uOff1)
	if mode == 0 {
		return nil
	}

	if initialPair && mode == 3 && melEvent > 0 {
		mode = 4
	}

	var table []UVLCDecodeEntry
	if initialPair {
		table = UVLCTbl0[:]
	} else {
		table = UVLCTbl1[:]
	}

	// For mode 4 (initial pair with mel=1), subtract bias of 2 from each
	targetU0 := uint32(u0)
	targetU1 := uint32(u1)
	// No bias subtraction needed: mode=4 table entries already include +2 in prefixes

	// Find the best table entry matching our target values
	bestLen := 999
	bestHead := -1
	bestEntry := UVLCDecodeEntry(0)

	for head := 0; head < 64; head++ {
		entry := table[(mode<<6)|head]
		if entry == 0 {
			continue
		}

		u0pfx := uint32(entry.U0Prefix())
		u1pfx := uint32(entry.U1Prefix())
		u0sufLen := entry.U0SuffixLen()
		u1sufLen := entry.TotalSuffixLen() - u0sufLen

		// Check if this entry can encode our target values
		if targetU0 < u0pfx {
			continue
		}
		u0suf := targetU0 - u0pfx
		if u0sufLen == 0 && u0suf != 0 {
			continue
		}
		if u0sufLen > 0 && u0suf >= (1<<u0sufLen) {
			continue
		}

		if targetU1 < u1pfx {
			continue
		}
		u1suf := targetU1 - u1pfx
		if u1sufLen == 0 && u1suf != 0 {
			continue
		}
		if u1sufLen > 0 && u1suf >= (1<<u1sufLen) {
			continue
		}

		totalLen := entry.TotalPrefixLen() + entry.TotalSuffixLen()
		if totalLen < bestLen {
			bestLen = totalLen
			bestHead = head
			bestEntry = entry
		}
	}

	if bestHead < 0 {
		return fmt.Errorf("no UVLC table entry for mode=%d u0=%d u1=%d", mode, targetU0, targetU1)
	}

	// Encode prefix: write head bits up to TotalPrefixLen (LSB first)
	prefixLen := bestEntry.TotalPrefixLen()
	if err := u.writer.WriteBits(uint32(bestHead), prefixLen); err != nil {
		return err
	}

	// Encode suffix
	sufLen := bestEntry.TotalSuffixLen()
	if sufLen > 0 {
		u0sufLen := bestEntry.U0SuffixLen()
		u0suf := targetU0 - uint32(bestEntry.U0Prefix())
		u1suf := targetU1 - uint32(bestEntry.U1Prefix())
		suffix := u0suf | (u1suf << u0sufLen)

		if err := u.writer.WriteBits(suffix, sufLen); err != nil {
			return err
		}
	}

	// Extension handling for large values
	if initialPair {
		bias := UVLCBias[(mode<<6)|bestHead]
		u0Bias := uint32(bias & 0x3)
		u1Bias := uint32((bias >> 2) & 0x3)

		if err := u.encodeExtension(targetU0, u0Bias, true); err != nil {
			return err
		}
		if err := u.encodeExtension(targetU1, u1Bias, true); err != nil {
			return err
		}
	} else {
		if err := u.encodeExtension(targetU0, 0, false); err != nil {
			return err
		}
		if err := u.encodeExtension(targetU1, 0, false); err != nil {
			return err
		}
	}

	return nil
}

func (u *UVLCEncoder) encodeExtension(val uint32, bias uint32, useBias bool) error {
	threshold := uint32(32)
	if useBias {
		if val <= bias || val-bias <= threshold {
			return nil
		}
	} else if val <= threshold {
		return nil
	}
	// Write 4-bit extension
	ext := (val - threshold) >> 2
	if ext >= 16 {
		ext = 15
	}
	return u.writer.WriteBits(ext, 4)
}

// HasTableEntry checks if a UVLC table entry exists for the given parameters.
func (u *UVLCEncoder) HasTableEntry(uOff0, uOff1 uint8, u0, u1 int, initialPair bool, melEvent int) bool {
	mode := int(uOff0) + 2*int(uOff1)
	if mode == 0 {
		return true
	}
	if initialPair && mode == 3 && melEvent > 0 {
		mode = 4
	}

	var table []UVLCDecodeEntry
	if initialPair {
		table = UVLCTbl0[:]
	} else {
		table = UVLCTbl1[:]
	}

	targetU0 := uint32(u0)
	targetU1 := uint32(u1)

	for head := 0; head < 64; head++ {
		entry := table[(mode<<6)|head]
		if entry == 0 {
			continue
		}
		u0pfx := uint32(entry.U0Prefix())
		u1pfx := uint32(entry.U1Prefix())
		u0sufLen := entry.U0SuffixLen()
		u1sufLen := entry.TotalSuffixLen() - u0sufLen

		if targetU0 < u0pfx {
			continue
		}
		u0suf := targetU0 - u0pfx
		if u0sufLen == 0 && u0suf != 0 {
			continue
		}
		if u0sufLen > 0 && u0suf >= (1<<u0sufLen) {
			continue
		}
		if targetU1 < u1pfx {
			continue
		}
		u1suf := targetU1 - u1pfx
		if u1sufLen == 0 && u1suf != 0 {
			continue
		}
		if u1sufLen > 0 && u1suf >= (1<<u1sufLen) {
			continue
		}
		return true
	}
	return false
}
