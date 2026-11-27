package standard

// EncodeDC encodes a DC coefficient difference
func EncodeDC(bw *BitWriter, diff int, dcCodes []HuffmanCode) error {
	// Compute category and bits
	cat, bits := bw.EncodeCategory(diff)

	// Write category using Huffman code
	if cat < 0 || cat >= len(dcCodes) {
		return ErrHuffmanDecode
	}

	code := dcCodes[cat]
	if err := bw.WriteBits(uint32(code.Code), code.Len); err != nil {
		return err
	}

	// Write magnitude bits
	if cat > 0 {
		if err := bw.WriteBits(bits, cat); err != nil {
			return err
		}
	}

	return nil
}

// EncodeAC encodes AC coefficients in zigzag order
func EncodeAC(bw *BitWriter, block [64]int, acCodes []HuffmanCode) error {
	// Process AC coefficients in zigzag order
	runLength := 0

	for k := 1; k < 64; k++ {
		coef := block[ZigZag[k]]

		if coef == 0 {
			runLength++
			if k == 63 {
				// EOB (End of Block)
				code := acCodes[0x00]
				if err := bw.WriteBits(uint32(code.Code), code.Len); err != nil {
					return err
				}
			}
			continue
		}

		// Output ZRL (Zero Run Length) codes if runLength >= 16
		for runLength >= 16 {
			code := acCodes[0xF0] // ZRL
			if err := bw.WriteBits(uint32(code.Code), code.Len); err != nil {
				return err
			}
			runLength -= 16
		}

		// Encode the coefficient
		cat, bits := bw.EncodeCategory(coef)
		symbol := (runLength << 4) | cat

		if symbol < 0 || symbol >= len(acCodes) {
			return ErrHuffmanDecode
		}

		code := acCodes[symbol]
		if err := bw.WriteBits(uint32(code.Code), code.Len); err != nil {
			return err
		}

		if cat > 0 {
			if err := bw.WriteBits(bits, cat); err != nil {
				return err
			}
		}

		runLength = 0
	}

	return nil
}

// DecodeDC decodes a DC coefficient difference
func DecodeDC(br *BitReader, dcTable *HuffmanTable) (int, error) {
	// Decode category
	category, err := br.Decode(dcTable)
	if err != nil {
		return 0, err
	}

	cat := int(category)
	if cat == 0 {
		return 0, nil
	}

	// Receive magnitude bits
	diff, err := br.ReceiveExtend(cat)
	if err != nil {
		return 0, err
	}

	return diff, nil
}

// DecodeAC decodes AC coefficients in zigzag order
func DecodeAC(br *BitReader, block []int, acTable *HuffmanTable) error {
	// Decode AC coefficients
	k := 1
	for k < 64 {
		symbol, err := br.Decode(acTable)
		if err != nil {
			return err
		}

		if symbol == 0x00 {
			// EOB (End of Block)
			// Fill remaining with zeros
			for k < 64 {
				block[ZigZag[k]] = 0
				k++
			}
			break
		}

		if symbol == 0xF0 {
			// ZRL (16 zeros)
			for i := 0; i < 16 && k < 64; i++ {
				block[ZigZag[k]] = 0
				k++
			}
			continue
		}

		// Extract run length and category
		runLength := int(symbol >> 4)
		category := int(symbol & 0x0F)

		// Skip zeros
		for i := 0; i < runLength && k < 64; i++ {
			block[ZigZag[k]] = 0
			k++
		}

		if k >= 64 {
			break
		}

		// Decode coefficient
		if category > 0 {
			val, err := br.ReceiveExtend(category)
			if err != nil {
				return err
			}
			block[ZigZag[k]] = val
		} else {
			block[ZigZag[k]] = 0
		}

		k++
	}

	return nil
}

// BitWriter wraps HuffmanEncoder with a simpler interface
type BitWriter struct {
	enc *HuffmanEncoder
}

// NewBitWriter creates a new BitWriter
func NewBitWriter(w *Writer) *BitWriter {
	return &BitWriter{
		enc: NewHuffmanEncoder(w),
	}
}

// WriteBits writes n bits
func (bw *BitWriter) WriteBits(bits uint32, n int) error {
	return bw.enc.WriteBits(bits, n)
}

// EncodeCategory computes category and magnitude bits for a value
func (bw *BitWriter) EncodeCategory(val int) (cat int, bits uint32) {
	return bw.enc.EncodeCategory(val)
}

// Flush flushes remaining bits
func (bw *BitWriter) Flush() error {
	return bw.enc.Flush()
}

// BitReader wraps HuffmanDecoder with a simpler interface
type BitReader struct {
	dec *HuffmanDecoder
}

// NewBitReader creates a new BitReader over an entropy-coded segment.
func NewBitReader(data []byte) *BitReader {
	return &BitReader{
		dec: NewHuffmanDecoder(data),
	}
}

// Decode decodes a symbol using Huffman table
func (br *BitReader) Decode(table *HuffmanTable) (byte, error) {
	return br.dec.Decode(table)
}

// ReceiveExtend receives and extends magnitude bits
func (br *BitReader) ReceiveExtend(ssss int) (int, error) {
	return br.dec.ReceiveExtend(ssss)
}
