package standard

import (
	"io"
	"math/bits"
)

// HuffmanEncoder encodes data using Huffman coding
type HuffmanEncoder struct {
	w     io.Writer
	buf   []byte // Entropy bytes; written out on Flush
	bits  uint32 // Bit buffer
	nBits int    // Number of bits in buffer
}

// NewHuffmanEncoder creates a new Huffman encoder
func NewHuffmanEncoder(w io.Writer) *HuffmanEncoder {
	return &HuffmanEncoder{w: w, buf: make([]byte, 0, 1<<16)}
}

// WriteBits writes n bits
func (e *HuffmanEncoder) WriteBits(bits uint32, n int) error {
	if n == 0 {
		return nil
	}

	e.bits = (e.bits << uint(n)) | (bits & ((1 << uint(n)) - 1))
	e.nBits += n

	for e.nBits >= 8 {
		b := byte(e.bits >> uint(e.nBits-8))
		e.writeByte(b)
		e.nBits -= 8
	}

	return nil
}

// writeByte appends a byte with byte stuffing.
func (e *HuffmanEncoder) writeByte(b byte) {
	e.buf = append(e.buf, b)
	// Byte stuffing: if we write 0xFF, follow with 0x00
	if b == 0xFF {
		e.buf = append(e.buf, 0x00)
	}
	if len(e.buf) == cap(e.buf) {
		e.flushBuf()
	}
}

// flushBuf writes buffered bytes to the destination.
func (e *HuffmanEncoder) flushBuf() {
	if len(e.buf) == 0 {
		return
	}
	e.w.Write(e.buf)
	e.buf = e.buf[:0]
}

// Flush writes any remaining bits
func (e *HuffmanEncoder) Flush() error {
	if e.nBits > 0 {
		// Pad with 1s
		b := byte((e.bits << uint(8-e.nBits)) | ((1 << uint(8-e.nBits)) - 1))
		e.writeByte(b)
		e.nBits = 0
		e.bits = 0
	}
	if len(e.buf) == 0 {
		return nil
	}
	_, err := e.w.Write(e.buf)
	e.buf = e.buf[:0]
	return err
}

// HuffmanCode represents a Huffman code
type HuffmanCode struct {
	Code uint16 // The Huffman code
	Len  int    // Code length in bits
}

// BuildHuffmanCodes builds Huffman codes from a table
func BuildHuffmanCodes(table *HuffmanTable) []HuffmanCode {
	codes := make([]HuffmanCode, 256)

	code := uint16(0)
	p := 0

	for l := 0; l < 16; l++ {
		for i := 0; i < table.Bits[l]; i++ {
			if p < len(table.Values) {
				val := table.Values[p]
				codes[val] = HuffmanCode{
					Code: code,
					Len:  l + 1,
				}
				code++
				p++
			}
		}
		code <<= 1
	}

	return codes
}

// EncodeCategory encodes a coefficient category and value
func (e *HuffmanEncoder) EncodeCategory(val int) (cat int, magnitude uint32) {
	if val == 0 {
		return 0, 0
	}

	absVal := val
	if absVal < 0 {
		absVal = -absVal
	}

	// Find category (number of bits needed)
	cat = bits.Len(uint(absVal))

	// Encode value
	if val > 0 {
		magnitude = uint32(val)
	} else {
		magnitude = uint32((1 << uint(cat)) + val - 1)
	}

	return cat, magnitude
}

// EncodeLosslessDifference encodes a signed 16-bit JPEG lossless difference.
// JPEG process 14 reserves category 16 for -32768 and emits no amplitude bits
// for that value, matching libjpeg's lossless Huffman encoder.
func (e *HuffmanEncoder) EncodeLosslessDifference(diff int) (cat int, magnitude uint32) {
	if diff == -1<<15 {
		return 16, 0
	}
	return e.EncodeCategory(diff)
}
