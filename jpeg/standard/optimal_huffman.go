package standard

const maxHuffmanCodeLength = 32

// BuildOptimalHuffmanTable builds the length-limited optimal table used by
// libjpeg when optimize_coding is enabled.
func BuildOptimalHuffmanTable(frequencies [256]uint64) *HuffmanTable {
	var freq [257]uint64
	copy(freq[:], frequencies[:])

	var bits [maxHuffmanCodeLength + 1]int
	var codeSize [257]int
	var others [257]int
	for i := range others {
		others[i] = -1
	}

	// A pseudo-symbol prevents any real symbol from receiving an all-ones code.
	freq[256] = 1

	for {
		c1 := smallestFrequencySymbol(freq, -1)
		c2 := smallestFrequencySymbol(freq, c1)
		if c2 < 0 {
			break
		}

		freq[c1] += freq[c2]
		freq[c2] = 0

		incrementCodeSize(codeSize[:], others[:], c1)
		others[lastBranchSymbol(others[:], c1)] = c2
		incrementCodeSize(codeSize[:], others[:], c2)
	}

	for _, size := range codeSize {
		if size > 0 {
			bits[size]++
		}
	}

	for size := maxHuffmanCodeLength; size > 16; size-- {
		for bits[size] > 0 {
			prefixSize := size - 2
			for bits[prefixSize] == 0 {
				prefixSize--
			}

			bits[size] -= 2
			bits[size-1]++
			bits[prefixSize+1] += 2
			bits[prefixSize]--
		}
	}

	for size := maxHuffmanCodeLength; size > 0; size-- {
		if bits[size] > 0 {
			bits[size]-- // Remove the pseudo-symbol.
			break
		}
	}

	table := &HuffmanTable{}
	for size := 1; size <= 16; size++ {
		table.Bits[size-1] = bits[size]
	}

	for size := 1; size <= maxHuffmanCodeLength; size++ {
		for symbol := 0; symbol < 256; symbol++ {
			if codeSize[symbol] == size {
				table.Values = append(table.Values, byte(symbol))
			}
		}
	}

	_ = table.Build()
	return table
}

func smallestFrequencySymbol(freq [257]uint64, excluded int) int {
	symbol := -1
	var smallest uint64
	for i, value := range freq {
		if value != 0 && i != excluded && (symbol < 0 || value <= smallest) {
			symbol = i
			smallest = value
		}
	}
	return symbol
}

func incrementCodeSize(codeSize, others []int, symbol int) {
	for symbol >= 0 {
		codeSize[symbol]++
		symbol = others[symbol]
	}
}

func lastBranchSymbol(others []int, symbol int) int {
	for others[symbol] >= 0 {
		symbol = others[symbol]
	}
	return symbol
}
