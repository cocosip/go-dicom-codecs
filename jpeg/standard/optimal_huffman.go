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

	// The merge order the original scan produces is: lowest frequency first,
	// breaking ties toward the highest symbol index. A heap with that ordering
	// selects exactly the same pair every round, in O(n log n).
	heap := make(frequencyHeap, 0, 257)
	for symbol, value := range freq {
		if value != 0 {
			heap = append(heap, frequencyEntry{value, symbol})
		}
	}
	heap.init()

	for len(heap) >= 2 {
		c1 := heap.pop()
		c2 := heap.pop()

		freq[c1] += freq[c2]
		freq[c2] = 0

		incrementCodeSize(codeSize[:], others[:], c1)
		others[lastBranchSymbol(others[:], c1)] = c2
		incrementCodeSize(codeSize[:], others[:], c2)

		heap.push(frequencyEntry{freq[c1], c1})
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

// frequencyEntry is one live symbol of the optimal table construction.
type frequencyEntry struct {
	freq   uint64
	symbol int
}

// frequencyHeap keeps the smallest frequency on top, breaking ties toward the
// highest symbol index so the merge order matches libjpeg.
type frequencyHeap []frequencyEntry

func (h *frequencyHeap) less(i, j int) bool {
	if (*h)[i].freq != (*h)[j].freq {
		return (*h)[i].freq < (*h)[j].freq
	}
	return (*h)[i].symbol > (*h)[j].symbol
}

func (h *frequencyHeap) init() {
	for i := len(*h)/2 - 1; i >= 0; i-- {
		h.down(i)
	}
}

func (h *frequencyHeap) push(e frequencyEntry) {
	*h = append(*h, e)
	h.up(len(*h) - 1)
}

// pop removes and returns the top entry.
func (h *frequencyHeap) pop() int {
	last := len(*h) - 1
	top := (*h)[0].symbol
	(*h)[0] = (*h)[last]
	*h = (*h)[:last]
	if last > 0 {
		h.down(0)
	}
	return top
}

func (h *frequencyHeap) up(i int) {
	for i > 0 {
		parent := (i - 1) / 2
		if !h.less(i, parent) {
			return
		}
		(*h)[i], (*h)[parent] = (*h)[parent], (*h)[i]
		i = parent
	}
}

func (h *frequencyHeap) down(i int) {
	n := len(*h)
	for {
		left := 2*i + 1
		if left >= n {
			return
		}
		smallest := left
		if right := left + 1; right < n && h.less(right, left) {
			smallest = right
		}
		if !h.less(smallest, i) {
			return
		}
		(*h)[i], (*h)[smallest] = (*h)[smallest], (*h)[i]
		i = smallest
	}
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
