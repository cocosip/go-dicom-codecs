package standard

import (
	"math/rand"
	"testing"
)

// scanOptimalHuffmanTable is the straightforward O(n^2) construction the heap
// version must reproduce bit for bit.
func scanOptimalHuffmanTable(frequencies [256]uint64) *HuffmanTable {
	var freq [257]uint64
	copy(freq[:], frequencies[:])

	var bits [maxHuffmanCodeLength + 1]int
	var codeSize [257]int
	var others [257]int
	for i := range others {
		others[i] = -1
	}

	freq[256] = 1

	for {
		c1 := smallestFrequencySymbolScan(freq, -1)
		c2 := smallestFrequencySymbolScan(freq, c1)
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
			bits[size]--
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

func smallestFrequencySymbolScan(freq [257]uint64, excluded int) int {
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

func TestOptimalHuffmanMatchesScan(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for trial := 0; trial < 300; trial++ {
		var freq [256]uint64
		symbols := 1 + rng.Intn(256)
		for i := 0; i < symbols; i++ {
			// Skewed values create plenty of ties.
			freq[rng.Intn(256)] += uint64(1 + rng.Intn(4))
		}
		want := scanOptimalHuffmanTable(freq)
		got := BuildOptimalHuffmanTable(freq)
		if len(want.Values) != len(got.Values) {
			t.Fatalf("trial %d: value count %d, want %d", trial, len(got.Values), len(want.Values))
		}
		if want.Bits != got.Bits {
			t.Fatalf("trial %d: bits %v, want %v", trial, got.Bits, want.Bits)
		}
		for i := range want.Values {
			if want.Values[i] != got.Values[i] {
				t.Fatalf("trial %d: value %d is %d, want %d", trial, i, got.Values[i], want.Values[i])
			}
		}
	}
}
