package cleanup

// UVLCDecodeEntry describes a packed decode entry used by HTJ2K UVLC tables.
// The packed layout encodes total prefix/suffix lengths and per-quad prefixes/suffixes.
// Generated based on OpenJPH references.
// (ojph_block_common.cpp::uvlc_init_tables).
// These tables help decode U-VLC prefix/suffix lengths and initial-row biases.
// UVLCDecodeEntry packs per-quad prefix/suffix lengths and prefixes as:
//
//	[0:2]  total prefix length (lp0+lp1)
//	[3:6]  total suffix length (ls0+ls1)
//	[7:9]  suffix length for quad0
//	[10:12] prefix value for quad0
//	[13:15] prefix value for quad1
//
// UVLCDecodeEntry is a packed 16-bit value representing decode metadata.
type UVLCDecodeEntry uint16

// TotalPrefixLen returns the total prefix length (lp0+lp1).
func (e UVLCDecodeEntry) TotalPrefixLen() int { return int(e & 0x7) }

// TotalSuffixLen returns the total suffix length (ls0+ls1).
func (e UVLCDecodeEntry) TotalSuffixLen() int { return int((e >> 3) & 0xF) }

// U0SuffixLen returns the suffix length for quad0.
func (e UVLCDecodeEntry) U0SuffixLen() int { return int((e >> 7) & 0x7) }

// U0Prefix returns the prefix value for quad0.
func (e UVLCDecodeEntry) U0Prefix() int { return int((e >> 10) & 0x7) }

// U1Prefix returns the prefix value for quad1.
func (e UVLCDecodeEntry) U1Prefix() int { return int((e >> 13) & 0x7) }

// UVLCTbl0 holds decode entries for initial row pairs (includes MEL event).
var UVLCTbl0 [256 + 64]UVLCDecodeEntry

// UVLCTbl1 holds decode entries for non-initial rows.
var UVLCTbl1 [256]UVLCDecodeEntry

// UVLCBias stores bias for initial row pairs (u_bias).
var UVLCBias [256 + 64]uint8

func init() {
	generateUVLCTables()
}

func generateUVLCTables() {
	// dec table: index by 3-bit head (xx1/x10/100/000), value packs lp/ls/prefix.
	dec := [8]uint8{
		3 | (5 << 2) | (5 << 5), // 000 -> lp=3, ls=5, u_pfx=5
		1 | (0 << 2) | (1 << 5), // 001 -> lp=1, ls=0, u_pfx=1
		2 | (0 << 2) | (2 << 5), // 010 -> lp=2, ls=0, u_pfx=2
		1 | (0 << 2) | (1 << 5), // 011 -> lp=1, ls=0, u_pfx=1
		3 | (1 << 2) | (3 << 5), // 100 -> lp=3, ls=1, u_pfx=3
		1 | (0 << 2) | (1 << 5), // 101 -> lp=1, ls=0, u_pfx=1
		2 | (0 << 2) | (2 << 5), // 110 -> lp=2, ls=0, u_pfx=2
		1 | (0 << 2) | (1 << 5), // 111 -> lp=1, ls=0, u_pfx=1
	}

	// Initial row pairs: mode=0 => both u_off=0; mode=1/2 => one u_off;
	// mode=3 => both u_off with mel=0; mode=4 => both u_off with mel=1.
	for i := 0; i < len(UVLCTbl0); i++ {
		mode := i >> 6
		vlc := i & 0x3F
		switch mode {
		case 0:
			UVLCTbl0[i] = 0
			UVLCBias[i] = 0
		case 1, 2:
			d := dec[vlc&0x7]
			lp := int(d & 0x3)
			ls := int((d >> 2) & 0x7)
			u0suf := ls
			u0 := int(d >> 5)
			u1 := 0
			if mode == 2 {
				u0suf = 0
				u0 = 0
				u1 = int(d >> 5)
			}
			UVLCTbl0[i] = UVLCDecodeEntry(lp | (ls << 3) | (u0suf << 7) | (u0 << 10) | (u1 << 13))
			UVLCBias[i] = 0
		case 3:
			d0 := dec[vlc&0x7]
			vlc >>= d0 & 0x3
			d1 := dec[vlc&0x7]
			var lp, u0suf, ls, u0, u1 int
			if (d0 & 0x3) == 3 {
				lp = int(d0&0x3) + 1
				u0suf = int((d0 >> 2) & 0x7)
				ls = u0suf
				u0 = int(d0 >> 5)
				u1 = (vlc & 1) + 1
				UVLCBias[i] = 4
			} else {
				lp = int(d0&0x3) + int(d1&0x3)
				u0suf = int((d0 >> 2) & 0x7)
				ls = u0suf + int((d1>>2)&0x7)
				u0 = int(d0 >> 5)
				u1 = int(d1 >> 5)
				UVLCBias[i] = 0
			}
			UVLCTbl0[i] = UVLCDecodeEntry(lp | (ls << 3) | (u0suf << 7) | (u0 << 10) | (u1 << 13))
		case 4:
			d0 := dec[vlc&0x7]
			vlc >>= d0 & 0x3
			d1 := dec[vlc&0x7]
			lp := int((d0 & 0x3) + (d1 & 0x3))
			u0suf := int((d0 >> 2) & 0x7)
			ls := u0suf + int((d1>>2)&0x7)
			u0 := int((d0 >> 5) + 2)
			u1 := int((d1 >> 5) + 2)
			UVLCTbl0[i] = UVLCDecodeEntry(lp | (ls << 3) | (u0suf << 7) | (u0 << 10) | (u1 << 13))
			UVLCBias[i] = 10
		}
	}

	// Non-initial rows: mode=0/1/2/3 (no MEL event).
	for i := 0; i < len(UVLCTbl1); i++ {
		mode := i >> 6
		vlc := i & 0x3F
		switch mode {
		case 0:
			UVLCTbl1[i] = 0
		case 1, 2:
			d := dec[vlc&0x7]
			lp := int(d & 0x3)
			ls := int((d >> 2) & 0x7)
			u0suf := ls
			u0 := int(d >> 5)
			u1 := 0
			if mode == 2 {
				u0suf = 0
				u0 = 0
				u1 = int(d >> 5)
			}
			UVLCTbl1[i] = UVLCDecodeEntry(lp | (ls << 3) | (u0suf << 7) | (u0 << 10) | (u1 << 13))
		case 3:
			d0 := dec[vlc&0x7]
			vlc >>= d0 & 0x3
			d1 := dec[vlc&0x7]
			lp := int((d0 & 0x3) + (d1 & 0x3))
			u0suf := int((d0 >> 2) & 0x7)
			ls := u0suf + int((d1>>2)&0x7)
			u0 := int(d0 >> 5)
			u1 := int(d1 >> 5)
			UVLCTbl1[i] = UVLCDecodeEntry(lp | (ls << 3) | (u0suf << 7) | (u0 << 10) | (u1 << 13))
		}
	}
}
