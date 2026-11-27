package standard

// DecodeBlock decodes one sequential Huffman block into coef, applying the
// zigzag order and dequantizing with qt, and returns the updated DC predictor.
// Keeping the bit reader in one place avoids two calls per coefficient.
func (d *HuffmanDecoder) DecodeBlock(dcTable, acTable *HuffmanTable, qt *[64]int32, dcPred int, coef []int32) (int, error) {
	difference, err := d.DecodeReceive(dcTable)
	if err != nil {
		return 0, err
	}
	dcPred += difference
	coef[0] = int32(dcPred) * qt[0]

	k := 1
	for k < 64 {
		run, size, value, err := d.decodeACRunSize(acTable)
		if err != nil {
			return 0, err
		}
		if size == 0 {
			if run == 15 {
				k += 16
				continue
			}
			break
		}
		k += run
		if k >= 64 {
			return 0, ErrInvalidData
		}
		index := ZigZag[k]
		coef[index] = int32(value) * qt[index]
		k++
	}
	return dcPred, nil
}

// DecodeReceive reads one Huffman symbol and its amplitude bits.
func (d *HuffmanDecoder) DecodeReceive(table *HuffmanTable) (int, error) {
	if d.nBits < 16 && !d.refillFast() {
		d.refill(16)
		if d.nBits < 10 {
			symbol, err := d.Decode(table)
			if err != nil {
				return 0, err
			}
			return d.ReceiveExtend(int(symbol))
		}
	}
	entry := table.lookupTable[uint32(d.bits>>uint(d.nBits-10))&0x3FF]
	size := 0
	if entry >= 0 {
		d.nBits -= int(entry >> 8)
		size = int(entry & 0xFF)
	} else {
		symbol, err := d.Decode(table)
		if err != nil {
			return 0, err
		}
		size = int(symbol)
	}
	if size == 0 {
		return 0, nil
	}
	return d.receiveExtendFast(size)
}

// decodeACRunSize decodes one AC run/size symbol and, when it carries a
// value, the amplitude bits too.
func (d *HuffmanDecoder) decodeACRunSize(table *HuffmanTable) (run, size, value int, err error) {
	if d.nBits < 16 && !d.refillFast() {
		d.refill(16)
	}
	symbol := -1
	if d.nBits >= 10 {
		entry := table.lookupTable[uint32(d.bits>>uint(d.nBits-10))&0x3FF]
		if entry >= 0 {
			d.nBits -= int(entry >> 8)
			symbol = int(entry & 0xFF)
		}
	}
	if symbol < 0 {
		b, decErr := d.Decode(table)
		if decErr != nil {
			return 0, 0, 0, decErr
		}
		symbol = int(b)
	}
	run = symbol >> 4
	size = symbol & 0x0F
	if size == 0 {
		return run, 0, 0, nil
	}
	value, err = d.receiveExtendFast(size)
	return run, size, value, err
}

// receiveExtendFast reads size bits and applies the JPEG sign extension.
func (d *HuffmanDecoder) receiveExtendFast(size int) (int, error) {
	if d.nBits < size {
		d.refill(size)
		if d.nBits < size {
			if d.readErr != nil {
				return 0, d.readErr
			}
			return 0, ErrInvalidData
		}
	}
	d.nBits -= size
	value := int(uint32(d.bits>>uint(d.nBits)) & ((1 << uint(size)) - 1))
	if value < 1<<uint(size-1) {
		value += (-1 << uint(size)) + 1
	}
	return value, nil
}
