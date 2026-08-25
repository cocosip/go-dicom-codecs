package t2

import (
	"fmt"
)

// TagTree represents a hierarchical data structure for encoding/decoding
// inclusion and zero bitplane information in JPEG 2000 packet headers.
// Implementation follows ISO/IEC 15444-1 Section B.10.2 and OpenJPEG's tgt.c
type TagTree struct {
	width        int       // Width of the leaf level (number of code-blocks)
	height       int       // Height of the leaf level
	levels       int       // Number of levels in the tree
	nodes        [][]int   // Node values at each level [level][index]
	states       [][]int   // Node states (decoded value so far) [level][index]
	levelWidths  []int     // Width of each level [level]
	levelHeights []int     // Height of each level [level]

	// Encoding state (matches OpenJPEG's opj_tgt_node_t)
	low          [][]int   // Lower bound known so far [level][index]
	known        [][]bool  // Whether exact value is known [level][index]
}

// NewTagTree creates a new tag tree with the given leaf dimensions
func NewTagTree(width, height int) *TagTree {
	if width <= 0 || height <= 0 {
		width, height = 1, 1
	}

	tt := &TagTree{
		width:  width,
		height: height,
	}

	// Calculate number of levels needed
	// Each level reduces dimensions by half (rounded up)
	// Level 0 = leaf, Level N-1 = root (1x1)
	levels := 0
	w, h := width, height
	for w > 1 || h > 1 {
		levels++
		w = (w + 1) / 2
		h = (h + 1) / 2
	}
	levels++ // Include root level
	tt.levels = levels

	// Pre-calculate dimensions for each level
	tt.levelWidths = make([]int, levels)
	tt.levelHeights = make([]int, levels)

	w, h = width, height
	for i := 0; i < levels; i++ {
		tt.levelWidths[i] = w
		tt.levelHeights[i] = h
		w = (w + 1) / 2
		h = (h + 1) / 2
	}

	// Allocate nodes, states, and encoding state for each level
	tt.nodes = make([][]int, levels)
	tt.states = make([][]int, levels)
	tt.low = make([][]int, levels)
	tt.known = make([][]bool, levels)

	for i := 0; i < levels; i++ {
		size := tt.levelWidths[i] * tt.levelHeights[i]
		tt.nodes[i] = make([]int, size)
		tt.states[i] = make([]int, size)
		tt.low[i] = make([]int, size)
		tt.known[i] = make([]bool, size)

		// Initialize all node values to 999 (matches OpenJPEG's opj_tgt_reset)
		// Initialize all states to 0 (no bits decoded yet)
		// Initialize all low values to 0
		// Initialize all known flags to false
		for j := 0; j < size; j++ {
			tt.nodes[i][j] = 999  // High initial value for SetValue to work
			tt.states[i][j] = 0
			tt.low[i][j] = 0
			tt.known[i][j] = false
		}
	}

	return tt
}

// BitReader interface for reading bits from packet header
type BitReader interface {
	ReadBit() (int, error)
}

// BitWriter interface for writing bits to packet header
type BitWriter interface {
	WriteBit(bit int) error
}

// Decode decodes the tag tree value for the specified leaf position (x, y)
// up to the threshold value. Returns the decoded value.
// Implementation follows OpenJPEG's opj_tgt_decode().
func (tt *TagTree) Decode(br BitReader, x, y, threshold int) (int, error) {
	if x < 0 || x >= tt.width || y < 0 || y >= tt.height {
		return 0, fmt.Errorf("tag tree position out of bounds: (%d,%d) not in [0,%d)x[0,%d)",
			x, y, tt.width, tt.height)
	}

	type nodePos struct {
		level int
		idx   int
	}
	stack := make([]nodePos, 0, tt.levels)

	px, py := x, y
	for level := 0; level < tt.levels; level++ {
		idx := py*tt.levelWidths[level] + px
		stack = append(stack, nodePos{level: level, idx: idx})
		if level < tt.levels-1 {
			px /= 2
			py /= 2
		}
	}

	low := 0
	for i := len(stack) - 1; i >= 0; i-- {
		node := stack[i]
		level := node.level
		idx := node.idx

		if low > tt.low[level][idx] {
			tt.low[level][idx] = low
		} else {
			low = tt.low[level][idx]
		}

		for low < threshold && low < tt.nodes[level][idx] {
			bit, err := br.ReadBit()
			if err != nil {
				return tt.nodes[level][idx], err
			}
			if bit != 0 {
				tt.nodes[level][idx] = low
			} else {
				low++
			}
		}

		tt.low[level][idx] = low
	}

	leaf := stack[0]
	return tt.nodes[leaf.level][leaf.idx], nil
}

// Width returns the width of the tag tree (number of code-blocks in x direction)
func (tt *TagTree) Width() int {
	return tt.width
}

// Height returns the height of the tag tree (number of code-blocks in y direction)
func (tt *TagTree) Height() int {
	return tt.height
}

// GetNumLevels returns the number of levels in the tag tree
func (tt *TagTree) GetNumLevels() int {
	return tt.levels
}

// GetValue returns the current value at position (x, y)
// For encoding, returns the node value set by SetValue
// For decoding, returns the decoded state value
func (tt *TagTree) GetValue(x, y int) int {
	if x < 0 || x >= tt.width || y < 0 || y >= tt.height {
		return 0
	}

	idx := y*tt.width + x
	if idx >= len(tt.nodes[0]) {
		return 0
	}

	// Return node value (used for both encoding and decoding after decode completes)
	// During decoding, states will be copied to nodes by the decoder
	return tt.nodes[0][idx]
}

// SetValue sets the value at position (x, y) and propagates to parent nodes
// This matches OpenJPEG's opj_tgt_setvalue
func (tt *TagTree) SetValue(x, y, value int) {
	if x < 0 || x >= tt.width || y < 0 || y >= tt.height {
		return
	}

	// Set value at leaf level and propagate upward
	level := 0
	px, py := x, y

	for level < tt.levels {
		idx := py*tt.levelWidths[level] + px
		if idx >= len(tt.nodes[level]) {
			break
		}

		// Only update if new value is smaller (minimum propagation)
		if tt.nodes[level][idx] > value {
			tt.nodes[level][idx] = value
		} else {
			// Parent already has smaller or equal value, stop propagation
			break
		}

		// Move to parent level
		level++
		if level < tt.levels {
			px = px / 2
			py = py / 2
		}
	}
}

// Reset resets all node states and values (for decoding a new packet or resetting the tree)
// Matches OpenJPEG's opj_tgt_reset behavior
func (tt *TagTree) Reset() {
	for level := 0; level < tt.levels; level++ {
		for i := range tt.states[level] {
			tt.nodes[level][i] = 999 // Reset to high value
			tt.states[level][i] = 0
			tt.low[level][i] = 0
			tt.known[level][i] = false
		}
	}
}

// ResetEncoding resets encoding state (low and known) for a new packet
// This matches OpenJPEG's opj_tgt_reset
func (tt *TagTree) ResetEncoding() {
	for level := 0; level < tt.levels; level++ {
		for i := range tt.low[level] {
			tt.nodes[level][i] = 999  // Initialize to high value
			tt.low[level][i] = 0
			tt.known[level][i] = false
		}
	}
}

// Encode encodes the tag tree value for the specified leaf position (x, y)
// up to the threshold value
// This matches OpenJPEG's opj_tgt_encode
func (tt *TagTree) Encode(bw BitWriter, x, y, threshold int) error {
	if x < 0 || x >= tt.width || y < 0 || y >= tt.height {
		return fmt.Errorf("tag tree position out of bounds: (%d,%d) not in [0,%d)x[0,%d)",
			x, y, tt.width, tt.height)
	}

	// Build stack from leaf to root
	type nodePos struct {
		level int
		x     int
		y     int
		idx   int
	}
	stack := make([]nodePos, 0, tt.levels)

	// Start at leaf
	px, py := x, y
	for level := 0; level < tt.levels; level++ {
		idx := py*tt.levelWidths[level] + px
		stack = append(stack, nodePos{level: level, x: px, y: py, idx: idx})
		if level < tt.levels-1 {
			px = px / 2
			py = py / 2
		}
	}

	// Process from root to leaf with low value propagation
	// This matches OpenJPEG's algorithm exactly
	low := 0 // Start with low=0 at root
	for i := len(stack) - 1; i >= 0; i-- {
		node := stack[i]
		level := node.level
		idx := node.idx

		// Propagate low value from parent to child
		// If parent's low is higher, update current node's low
		if low > tt.low[level][idx] {
			tt.low[level][idx] = low
		} else {
			low = tt.low[level][idx]
		}

		// Encode bits while low < threshold and low < node value
		for low < threshold {
			if low >= tt.nodes[level][idx] {
				// Value is exactly known
				if !tt.known[level][idx] {
					// Write 1 bit to indicate we've reached the exact value
					if err := bw.WriteBit(1); err != nil {
						return err
					}
					tt.known[level][idx] = true
				}
				break
			}
			// Write 0 bit to indicate value is greater than low
			if err := bw.WriteBit(0); err != nil {
				return err
			}
			low++
		}

		// Update low bound for current node
		tt.low[level][idx] = low
	}

	return nil
}

// TagTreeDecoder is an alias for TagTree for backward compatibility
type TagTreeDecoder = TagTree

// NewTagTreeDecoder creates a new tag tree decoder (alias for backward compatibility)
func NewTagTreeDecoder(tree *TagTree) *TagTreeDecoder {
	return tree
}

// DecodeInclusion decodes code-block inclusion information from tag tree
// Returns (included, firstLayer, error)
// If included=false, firstLayer=-1 (not yet known)
// If included=true, firstLayer is the layer where code-block is first included
func (tt *TagTree) DecodeInclusion(x, y, currentLayer int, readBit func() (int, error)) (bool, int, error) {
	// Create a bit reader wrapper
	br := &bitReaderFunc{readBit: readBit}

	// Decode tag tree value up to currentLayer+1
	value, err := tt.Decode(br, x, y, currentLayer+1)
	if err != nil {
		return false, -1, err
	}

	// If value > currentLayer, code-block is not included in this layer
	if value > currentLayer {
		return false, -1, nil
	}

	// Code-block is included, value is the first layer
	return true, value, nil
}

// DecodeZeroBitPlanes decodes the number of zero bit-planes using tag tree
func (tt *TagTree) DecodeZeroBitPlanes(x, y int, readBit func() (int, error)) (int, error) {
	// Create a bit reader wrapper
	br := &bitReaderFunc{readBit: readBit}

	// Decode tag tree value (no threshold, read until completion)
	zbp, err := tt.Decode(br, x, y, 32) // 32 is arbitrary high threshold
	if err != nil {
		return 0, err
	}

	return zbp, nil
}

// bitReaderFunc wraps a function to implement BitReader interface
type bitReaderFunc struct {
	readBit func() (int, error)
}

func (br *bitReaderFunc) ReadBit() (int, error) {
	return br.readBit()
}

// PacketBitReader wraps a byte slice and provides bit-by-bit reading
type PacketBitReader struct {
	data   []byte
	offset int  // Byte offset
	bitPos int  // Bit position within current byte (0-7, MSB first)
}

// NewPacketBitReader creates a new bit reader
func NewPacketBitReader(data []byte) *PacketBitReader {
	return &PacketBitReader{
		data:   data,
		offset: 0,
		bitPos: 0,
	}
}

// ReadBit reads a single bit (MSB first)
func (pbr *PacketBitReader) ReadBit() (int, error) {
	if pbr.offset >= len(pbr.data) {
		return 0, fmt.Errorf("end of packet data")
	}

	// Read bit at current position (MSB = bit 7)
	bit := int((pbr.data[pbr.offset] >> (7 - pbr.bitPos)) & 1)

	// Advance to next bit
	pbr.bitPos++
	if pbr.bitPos >= 8 {
		pbr.bitPos = 0
		pbr.offset++
	}

	return bit, nil
}

// ReadBits reads n bits and returns them as an integer (MSB first)
func (pbr *PacketBitReader) ReadBits(n int) (int, error) {
	if n <= 0 {
		return 0, nil
	}
	if n > 32 {
		return 0, fmt.Errorf("cannot read more than 32 bits at once")
	}

	value := 0
	for i := 0; i < n; i++ {
		bit, err := pbr.ReadBit()
		if err != nil {
			return 0, err
		}
		value = (value << 1) | bit
	}

	return value, nil
}

// ByteAlign aligns to the next byte boundary
func (pbr *PacketBitReader) ByteAlign() {
	if pbr.bitPos != 0 {
		pbr.bitPos = 0
		pbr.offset++
	}
}

// Position returns the current byte offset and bit position
func (pbr *PacketBitReader) Position() (byteOffset, bitPos int) {
	return pbr.offset, pbr.bitPos
}

// Seek sets the position to the specified byte offset
func (pbr *PacketBitReader) Seek(offset int) {
	pbr.offset = offset
	pbr.bitPos = 0
}

// Remaining returns the number of bytes remaining
func (pbr *PacketBitReader) Remaining() int {
	remaining := len(pbr.data) - pbr.offset
	if pbr.bitPos > 0 {
		remaining-- // Current byte is partially consumed
	}
	if remaining < 0 {
		return 0
	}
	return remaining
}
