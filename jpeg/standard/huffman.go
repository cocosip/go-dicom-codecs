package standard

import "io"

// HuffmanTable represents a Huffman coding table
type HuffmanTable struct {
	// Number of codes of each length (1-16 bits)
	Bits [16]int
	// Values for each code, in order of code length
	Values []byte
	// Lookup tables for fast decoding
	minCode [16]int32
	maxCode [16]int32
	valPtr  [16]int32
	// Lookup table for fast decoding of short codes.
	// value: (nbits << 8) | value, -1 if not found.
	lookupTable [1024]int16
}

// Build builds lookup tables for fast Huffman decoding
func (h *HuffmanTable) Build() error {
	for i := range h.lookupTable {
		h.lookupTable[i] = -1
	}

	p := 0
	canon := 0
	for l := 0; l < 10; l++ {
		for i := 0; i < h.Bits[l]; i++ {
			if p >= len(h.Values) {
				return ErrInvalidDHT
			}
			// Fill every 10-bit value whose prefix is this canonical code.
			shift := 10 - (l + 1)
			start := canon << uint(shift)
			end := (canon + 1) << uint(shift)
			entry := int16((l+1)<<8 | int(h.Values[p]))
			for j := start; j < end; j++ {
				h.lookupTable[j] = entry
			}
			canon++
			p++
		}
		canon <<= 1
	}

	// Build min/max codes and value pointers for codes longer than 10 bits.
	code := int32(0)
	p = 0
	for l := 0; l < 16; l++ {
		if h.Bits[l] == 0 {
			h.maxCode[l] = -1
		} else {
			if p+h.Bits[l] > len(h.Values) {
				return ErrInvalidDHT
			}
			h.valPtr[l] = int32(p)
			h.minCode[l] = code
			p += h.Bits[l]
			code += int32(h.Bits[l])
			h.maxCode[l] = code - 1
		}
		code <<= 1
	}

	return nil
}

// HuffmanDecoder decodes Huffman-encoded data from an entropy-coded segment.
// The segment must still contain JPEG byte stuffing; the reader removes it.
type HuffmanDecoder struct {
	data    []byte
	pos     int
	bits    uint64 // Bit buffer, valid bits in the low nBits positions
	nBits   int    // Number of valid bits in bits
	readErr error  // Read error, if any
}

// NewHuffmanDecoder creates a new Huffman decoder over an entropy-coded
// segment.
func NewHuffmanDecoder(data []byte) *HuffmanDecoder {
	return &HuffmanDecoder{data: data}
}

// refill ensures that at least n bits are buffered.
func (d *HuffmanDecoder) refill(n int) {
	for d.nBits < n && d.readErr == nil {
		pos := d.pos
		data := d.data
		// Fast path: four plain bytes with no stuffing can be loaded at once.
		if pos+4 <= len(data) && data[pos] != 0xFF && data[pos+1] != 0xFF &&
			data[pos+2] != 0xFF && data[pos+3] != 0xFF {
			d.bits = d.bits<<32 |
				uint64(data[pos])<<24 | uint64(data[pos+1])<<16 |
				uint64(data[pos+2])<<8 | uint64(data[pos+3])
			d.nBits += 32
			d.pos += 4
			continue
		}
		if pos >= len(data) {
			d.readErr = io.ErrUnexpectedEOF
			return
		}
		b := data[pos]
		d.pos++
		if b == 0xFF {
			if d.pos >= len(data) {
				d.readErr = io.ErrUnexpectedEOF
				return
			}
			if d.data[d.pos] != 0x00 {
				// A marker ended the entropy-coded segment.
				d.readErr = ErrInvalidData
				return
			}
			d.pos++ // skip the stuffed zero
		}
		d.bits = (d.bits << 8) | uint64(b)
		d.nBits += 8
	}
}

func (d *HuffmanDecoder) readBit() (uint32, error) {
	if d.nBits == 0 {
		d.refill(1)
	}
	if d.nBits == 0 {
		if d.readErr != nil {
			return 0, d.readErr
		}
		return 0, io.ErrUnexpectedEOF
	}
	d.nBits--
	return uint32(d.bits>>uint(d.nBits)) & 1, nil
}

// ReadBit reads a single bit.
func (d *HuffmanDecoder) ReadBit() (bool, error) {
	bit, err := d.readBit()
	return bit == 1, err
}

// ReadBits reads n bits as an unsigned integer.
func (d *HuffmanDecoder) ReadBits(n int) (uint32, error) {
	if n == 0 {
		return 0, nil
	}
	if d.nBits < n {
		d.refill(n)
	}
	if d.nBits < n {
		if d.readErr != nil {
			return 0, d.readErr
		}
		return 0, io.ErrUnexpectedEOF
	}
	d.nBits -= n
	return uint32(d.bits>>uint(d.nBits)) & ((1 << uint(n)) - 1), nil
}

// Decode decodes the next Huffman symbol.
func (d *HuffmanDecoder) Decode(table *HuffmanTable) (byte, error) {
	if d.readErr != nil && d.nBits == 0 {
		return 0, d.readErr
	}

	// Refill so the 10-bit lookup works for most symbols.
	if d.nBits < 16 {
		d.refill(16)
	}
	if d.nBits >= 10 {
		peek := uint32(d.bits>>uint(d.nBits-10)) & 0x3FF
		entry := table.lookupTable[peek]
		if entry >= 0 {
			nbits := int(entry >> 8)
			d.nBits -= nbits
			return byte(entry & 0xFF), nil
		}
	}

	// Slow path for codes longer than 10 bits.
	code := uint32(0)
	for l := 0; l < 16; l++ {
		bit, err := d.readBit()
		if err != nil {
			return 0, err
		}
		code = (code << 1) | bit

		if table.maxCode[l] >= 0 && int32(code) <= table.maxCode[l] {
			idx := table.valPtr[l] + int32(code) - table.minCode[l]
			if idx >= 0 && int(idx) < len(table.Values) {
				return table.Values[idx], nil
			}
		}
	}

	return 0, ErrHuffmanDecode
}

// ReceiveExtend decodes a coefficient value.
// This combines RECEIVE and EXTEND operations.
func (d *HuffmanDecoder) ReceiveExtend(ssss int) (int, error) {
	if ssss == 0 {
		return 0, nil
	}

	bits, err := d.ReadBits(ssss)
	if err != nil {
		return 0, err
	}

	// Extend: convert to signed value.
	val := int(bits)
	if val < (1 << uint(ssss-1)) {
		val += (-1 << uint(ssss)) + 1
	}

	return val, nil
}

// ReceiveLosslessDifference decodes a JPEG process 14 signed 16-bit
// difference. Category 16 represents -32768 without amplitude bits.
func (d *HuffmanDecoder) ReceiveLosslessDifference(category int) (int, error) {
	if category == 16 {
		return -1 << 15, nil
	}
	return d.ReceiveExtend(category)
}

// DecodeLossless reads one DC category and its difference in a single call.
// It is the hot path of JPEG lossless decoding.
func (d *HuffmanDecoder) DecodeLossless(table *HuffmanTable) (int, error) {
	if d.nBits < 16 && !d.refillFast() {
		d.refill(16)
		if d.nBits < 10 {
			return d.decodeLosslessSlow(table)
		}
	}
	entry := table.lookupTable[uint32(d.bits>>uint(d.nBits-10))&0x3FF]
	if entry >= 0 {
		d.nBits -= int(entry >> 8)
		category := int(entry & 0xFF)
		if category == 16 {
			return -1 << 15, nil
		}
		if category == 0 {
			return 0, nil
		}
		if d.nBits < category {
			d.refill(category)
			if d.nBits < category {
				if d.readErr != nil {
					return 0, d.readErr
				}
				return 0, io.ErrUnexpectedEOF
			}
		}
		d.nBits -= category
		value := int(uint32(d.bits>>uint(d.nBits)) & ((1 << uint(category)) - 1))
		if value < 1<<uint(category-1) {
			value += (-1 << uint(category)) + 1
		}
		return value, nil
	}
	return d.decodeLosslessSlow(table)
}

// refillFast loads 32 bits when the next four bytes need no unstuffing.
// It reports whether it added bits.
func (d *HuffmanDecoder) refillFast() bool {
	pos := d.pos
	if pos+4 > len(d.data) {
		return false
	}
	data := d.data
	if data[pos] == 0xFF || data[pos+1] == 0xFF || data[pos+2] == 0xFF || data[pos+3] == 0xFF {
		return false
	}
	d.bits = d.bits<<32 |
		uint64(data[pos])<<24 | uint64(data[pos+1])<<16 |
		uint64(data[pos+2])<<8 | uint64(data[pos+3])
	d.nBits += 32
	d.pos += 4
	return true
}

// decodeLosslessSlow handles codes longer than the fast lookup covers.
func (d *HuffmanDecoder) decodeLosslessSlow(table *HuffmanTable) (int, error) {
	category, err := d.Decode(table)
	if err != nil {
		return 0, err
	}
	return d.ReceiveLosslessDifference(int(category))
}
