package t2

import "fmt"

// PacketHeaderParser parses JPEG 2000 packet headers
// Reference: ISO/IEC 15444-1:2019 Annex B.10
type PacketHeaderParser struct {
	data   []byte
	reader *bioReader

	// Tag trees for this precinct
	inclTagTree *TagTreeDecoder // Inclusion tag tree
	zbpTagTree  *TagTreeDecoder // Zero bit-plane tag tree

	// Precinct dimensions (in code-blocks)
	numCBX int
	numCBY int

	// Optional explicit code-block positions (CBX/CBY) in header order
	cbPositions []cbPosition

	// Current layer being decoded
	currentLayer int

	// Code-block state (persists across packets/layers)
	codeBlockStates []*CodeBlockState

	// Packet header decoding mode
	termAll bool
}

type cbPosition struct {
	X int
	Y int
}

// CodeBlockState tracks the state of a code-block across multiple packets
type CodeBlockState struct {
	Included       bool   // Has been included in any previous packet
	FirstLayer     int    // Layer in which it was first included
	ZeroBitPlanes  int    // Number of MSB zero bit-planes
	NumPassesTotal int    // Total number of passes decoded
	DataAccum      []byte // Accumulated compressed data
	NumLenBits     int    // Length indicator bits (OpenJPEG: numlenbits)
}

type packetHeaderBand struct {
	numCBX          int
	numCBY          int
	cbPositions     []cbPosition
	inclTagTree     *TagTreeDecoder
	zbpTagTree      *TagTreeDecoder
	codeBlockStates []*CodeBlockState
}

// NewPacketHeaderParser creates a new packet header parser
func NewPacketHeaderParser(data []byte, numCBX, numCBY int) *PacketHeaderParser {
	return NewPacketHeaderParserWithState(data, numCBX, numCBY, nil, nil, nil, nil, false)
}

// NewPacketHeaderParserWithState allows reusing code-block state across packets (precinct-level state).
func NewPacketHeaderParserWithState(data []byte, numCBX, numCBY int, states []*CodeBlockState, incl *TagTreeDecoder, zbp *TagTreeDecoder, cbPositions []cbPosition, termAll bool) *PacketHeaderParser {
	// Create tag trees if not provided
	if incl == nil {
		incl = NewTagTreeDecoder(NewTagTree(numCBX, numCBY))
	}
	if zbp == nil {
		zbp = NewTagTreeDecoder(NewTagTree(numCBX, numCBY))
	}

	// Initialize or reuse code-block states
	numCB := numCBX * numCBY
	cbStates := states
	if cbStates == nil || len(cbStates) != numCB {
		cbStates = newCodeBlockStates(numCB)
	}

	return &PacketHeaderParser{
		data:            data,
		reader:          newBioReader(data),
		inclTagTree:     incl,
		zbpTagTree:      zbp,
		numCBX:          numCBX,
		numCBY:          numCBY,
		cbPositions:     cbPositions,
		codeBlockStates: cbStates,
		currentLayer:    0,
		termAll:         termAll,
	}
}

// ParseHeader parses a single packet header
// Returns the packet information including code-block contributions
func (php *PacketHeaderParser) ParseHeader() (*Packet, error) {
	packet := &Packet{
		LayerIndex:     php.currentLayer,
		CodeBlockIncls: make([]CodeBlockIncl, 0),
	}

	// Read packet present bit (SOP marker bit in some implementations)
	// For now, assume packet is present if we have data
	if php.reader.bytesRead() >= len(php.data) {
		packet.HeaderPresent = false
		return packet, nil
	}

	// Read empty packet flag (1 bit)
	emptyBit, err := php.readBit()
	if err != nil {
		return nil, fmt.Errorf("failed to read empty packet bit: %w", err)
	}

	packet.HeaderPresent = (emptyBit == 1)

	if !packet.HeaderPresent {
		// Empty packet
		return packet, nil
	}

	// Parse code-block inclusion information
	headerStart := php.reader.bytesRead()

	// For each code-block in the precinct
	var positions []cbPosition
	if len(php.cbPositions) > 0 {
		positions = php.cbPositions
	} else {
		positions = make([]cbPosition, 0, php.numCBX*php.numCBY)
		for cby := 0; cby < php.numCBY; cby++ {
			for cbx := 0; cbx < php.numCBX; cbx++ {
				positions = append(positions, cbPosition{X: cbx, Y: cby})
			}
		}
	}

	for _, pos := range positions {
		cbx := pos.X
		cby := pos.Y
		if cbx < 0 || cbx >= php.numCBX || cby < 0 || cby >= php.numCBY {
			continue
		}
		cbIdx := cby*php.numCBX + cbx
		cbState := php.codeBlockStates[cbIdx]

		var cbIncl CodeBlockIncl

		// Decode inclusion
		if !cbState.Included {
			included, firstLayer, err := php.inclTagTree.DecodeInclusion(
				cbx, cby, php.currentLayer, php.readBit)
			if err != nil {
				return nil, fmt.Errorf("failed to decode inclusion for CB[%d,%d]: %w", cbx, cby, err)
			}
			cbIncl.Included = included
			if !included {
				packet.CodeBlockIncls = append(packet.CodeBlockIncls, cbIncl)
				continue
			}

			// First time this code-block is included
			cbIncl.FirstInclusion = true
			cbState.Included = true
			cbState.FirstLayer = firstLayer
			cbState.NumLenBits = 3

			// Decode zero bit-planes using tag-tree (matches encoder/OpenJPEG)
			zbp, err := php.zbpTagTree.DecodeZeroBitPlanes(cbx, cby, php.readBit)
			if err != nil {
				return nil, fmt.Errorf("failed to decode ZBP for CB[%d,%d]: %w", cbx, cby, err)
			}
			cbState.ZeroBitPlanes = zbp
			cbIncl.ZeroBitplanes = zbp
		} else {
			// Already included in previous layer: read a single inclusion bit
			bit, err := php.readBit()
			if err != nil {
				return nil, fmt.Errorf("failed to decode inclusion bit for CB[%d,%d]: %w", cbx, cby, err)
			}
			cbIncl.Included = bit == 1
			if !cbIncl.Included {
				packet.CodeBlockIncls = append(packet.CodeBlockIncls, cbIncl)
				continue
			}
			cbIncl.FirstInclusion = false
			cbIncl.ZeroBitplanes = cbState.ZeroBitPlanes
		}

		// Decode number of coding passes
		numPasses, err := php.decodeNumPasses()
		if err != nil {
			return nil, fmt.Errorf("failed to decode num passes for CB[%d,%d]: %w", cbx, cby, err)
		}
		cbIncl.NumPasses = numPasses
		cbState.NumPassesTotal += numPasses

		// Decode length of code-block contribution
		dataLength, passLens, err := php.decodeDataLength(numPasses, cbState)
		if err != nil {
			return nil, fmt.Errorf("failed to decode data length for CB[%d,%d]: %w", cbx, cby, err)
		}
		cbIncl.DataLength = dataLength
		if len(passLens) > 0 {
			cbIncl.PassLengths = passLens
			cbIncl.UseTERMALL = php.termAll
		}

		packet.CodeBlockIncls = append(packet.CodeBlockIncls, cbIncl)
	}

	// Align to byte boundary
	if err := php.alignToByte(); err != nil {
		return nil, fmt.Errorf("failed to align to byte: %w", err)
	}

	// Header ends here
	packet.Header = php.data[headerStart:php.reader.bytesRead()]

	return packet, nil
}

// readBit reads a single bit from the bitstream
func (php *PacketHeaderParser) readBit() (int, error) {
	return php.reader.readBit()
}

func (php *PacketHeaderParser) decodeNumPasses() (int, error) {
	return decodeNumPassesWithReader(php.reader)
}

func (php *PacketHeaderParser) decodeDataLength(numPasses int, cbState *CodeBlockState) (int, []int, error) {
	return decodeDataLengthWithReader(php.reader, numPasses, cbState, php.termAll)
}

// readBits reads multiple bits from the bitstream
func (php *PacketHeaderParser) readBits(n int) (int, error) {
	return php.reader.readBits(n)
}

// alignToByte aligns the bit position to the next byte boundary
func (php *PacketHeaderParser) alignToByte() error {
	return php.reader.alignToByte()
}

// SetLayer sets the current layer index for decoding
func (php *PacketHeaderParser) SetLayer(layer int) {
	php.currentLayer = layer
}

// Reset resets the parser to the beginning
func (php *PacketHeaderParser) Reset() {
	php.reader = newBioReader(php.data)
	php.currentLayer = 0
	php.inclTagTree.Reset()
	php.zbpTagTree.Reset()

	// Reset code-block states
	for _, cbState := range php.codeBlockStates {
		cbState.Included = false
		cbState.FirstLayer = -1
		cbState.ZeroBitPlanes = 0
		cbState.NumPassesTotal = 0
		cbState.DataAccum = cbState.DataAccum[:0]
		cbState.NumLenBits = 0
	}
}

// Position returns the current byte position
func (php *PacketHeaderParser) Position() int {
	return php.reader.bytesRead()
}

func newCodeBlockStates(numCB int) []*CodeBlockState {
	cbStates := make([]*CodeBlockState, numCB)
	for i := range cbStates {
		cbStates[i] = &CodeBlockState{
			Included:      false,
			FirstLayer:    -1,
			ZeroBitPlanes: 0,
			DataAccum:     make([]byte, 0),
			NumLenBits:    0,
		}
	}
	return cbStates
}

func normalizePacketHeaderBand(band *packetHeaderBand) {
	if band == nil || band.numCBX <= 0 || band.numCBY <= 0 {
		return
	}
	if band.inclTagTree == nil || band.inclTagTree.Width() != band.numCBX || band.inclTagTree.Height() != band.numCBY {
		band.inclTagTree = NewTagTreeDecoder(NewTagTree(band.numCBX, band.numCBY))
	}
	if band.zbpTagTree == nil || band.zbpTagTree.Width() != band.numCBX || band.zbpTagTree.Height() != band.numCBY {
		band.zbpTagTree = NewTagTreeDecoder(NewTagTree(band.numCBX, band.numCBY))
	}
	numCB := band.numCBX * band.numCBY
	if band.codeBlockStates == nil || len(band.codeBlockStates) != numCB {
		band.codeBlockStates = newCodeBlockStates(numCB)
	}
}

func parsePacketHeaderMulti(data []byte, layer int, bands []*packetHeaderBand, termAll bool) ([]byte, []CodeBlockIncl, int, bool, error) {
	reader := newBioReader(data)
	if reader.bytesRead() >= len(data) {
		return nil, nil, 0, false, nil
	}

	headerStart := reader.bytesRead()
	emptyBit, err := reader.readBit()
	if err != nil {
		return nil, nil, reader.bytesRead(), false, fmt.Errorf("failed to read empty packet bit: %w", err)
	}

	headerPresent := emptyBit == 1
	if !headerPresent {
		header := data[headerStart:reader.bytesRead()]
		return header, nil, reader.bytesRead(), false, nil
	}

	cbIncls := make([]CodeBlockIncl, 0)

	for _, band := range bands {
		if band == nil || band.numCBX <= 0 || band.numCBY <= 0 {
			continue
		}
		normalizePacketHeaderBand(band)

		positions := band.cbPositions
		if len(positions) == 0 {
			positions = make([]cbPosition, 0, band.numCBX*band.numCBY)
			for cby := 0; cby < band.numCBY; cby++ {
				for cbx := 0; cbx < band.numCBX; cbx++ {
					positions = append(positions, cbPosition{X: cbx, Y: cby})
				}
			}
		}

		for _, pos := range positions {
			cbx := pos.X
			cby := pos.Y
			if cbx < 0 || cbx >= band.numCBX || cby < 0 || cby >= band.numCBY {
				continue
			}
			cbIdx := cby*band.numCBX + cbx
			cbState := band.codeBlockStates[cbIdx]

			var cbIncl CodeBlockIncl

			if !cbState.Included {
				included, firstLayer, err := band.inclTagTree.DecodeInclusion(
					cbx, cby, layer, reader.readBit)
				if err != nil {
					return nil, nil, reader.bytesRead(), true, fmt.Errorf("failed to decode inclusion for CB[%d,%d]: %w", cbx, cby, err)
				}

				cbIncl.Included = included
				if !included {
					cbIncls = append(cbIncls, cbIncl)
					continue
				}

				cbIncl.FirstInclusion = true
				cbState.Included = true
				cbState.FirstLayer = firstLayer
				cbState.NumLenBits = 3

				zbp, err := band.zbpTagTree.DecodeZeroBitPlanes(cbx, cby, reader.readBit)
				if err != nil {
					return nil, nil, reader.bytesRead(), true, fmt.Errorf("failed to decode ZBP for CB[%d,%d]: %w", cbx, cby, err)
				}
				cbState.ZeroBitPlanes = zbp
				cbIncl.ZeroBitplanes = zbp
			} else {
				bit, err := reader.readBit()
				if err != nil {
					return nil, nil, reader.bytesRead(), true, fmt.Errorf("failed to decode inclusion bit for CB[%d,%d]: %w", cbx, cby, err)
				}
				cbIncl.Included = bit == 1
				if !cbIncl.Included {
					cbIncls = append(cbIncls, cbIncl)
					continue
				}
				cbIncl.FirstInclusion = false
				cbIncl.ZeroBitplanes = cbState.ZeroBitPlanes
			}

			numPasses, err := decodeNumPassesWithReader(reader)
			if err != nil {
				return nil, nil, reader.bytesRead(), true, fmt.Errorf("failed to decode num passes for CB[%d,%d]: %w", cbx, cby, err)
			}
			cbIncl.NumPasses = numPasses
			cbState.NumPassesTotal += numPasses

			dataLength, passLens, err := decodeDataLengthWithReader(reader, numPasses, cbState, termAll)
			if err != nil {
				return nil, nil, reader.bytesRead(), true, fmt.Errorf("failed to decode data length for CB[%d,%d]: %w", cbx, cby, err)
			}
			cbIncl.DataLength = dataLength
			if len(passLens) > 0 {
				cbIncl.PassLengths = passLens
				cbIncl.UseTERMALL = termAll
			}

			cbIncls = append(cbIncls, cbIncl)
		}
	}

	if err := reader.alignToByte(); err != nil {
		return nil, nil, reader.bytesRead(), true, fmt.Errorf("failed to align to byte: %w", err)
	}

	header := data[headerStart:reader.bytesRead()]
	return header, cbIncls, reader.bytesRead(), true, nil
}

func decodeNumPassesWithReader(reader *bioReader) (int, error) {
	bit1, err := reader.readBit()
	if err != nil {
		return 0, err
	}
	if bit1 == 0 {
		return 1, nil
	}

	bit2, err := reader.readBit()
	if err != nil {
		return 0, err
	}
	if bit2 == 0 {
		return 2, nil
	}

	val2, err := reader.readBits(2)
	if err != nil {
		return 0, err
	}
	if val2 != 3 {
		return 3 + val2, nil
	}

	val5, err := reader.readBits(5)
	if err != nil {
		return 0, err
	}
	if val5 != 31 {
		return 6 + val5, nil
	}

	val7, err := reader.readBits(7)
	if err != nil {
		return 0, err
	}
	return 37 + val7, nil
}

func decodeDataLengthWithReader(reader *bioReader, numPasses int, cbState *CodeBlockState, termAll bool) (int, []int, error) {
	if numPasses <= 0 {
		return 0, nil, nil
	}
	if cbState.NumLenBits <= 0 {
		cbState.NumLenBits = 3
	}

	increment, err := decodeCommaCodeWithReader(reader)
	if err != nil {
		return 0, nil, err
	}
	cbState.NumLenBits += increment

	totalLen := 0
	if termAll {
		passLens := make([]int, numPasses)
		for pass := 0; pass < numPasses; pass++ {
			bitCount := cbState.NumLenBits
			segLen, err := reader.readBits(bitCount)
			if err != nil {
				return 0, nil, err
			}
			passLens[pass] = segLen
			totalLen += segLen
		}
		return totalLen, passLens, nil
	}

	bitCount := cbState.NumLenBits + floorLog2(numPasses)
	segLen, err := reader.readBits(bitCount)
	if err != nil {
		return 0, nil, err
	}
	totalLen += segLen
	return totalLen, nil, nil
}

func decodeCommaCodeWithReader(reader *bioReader) (int, error) {
	n := 0
	for {
		bit, err := reader.readBit()
		if err != nil {
			return 0, err
		}
		if bit == 0 {
			break
		}
		n++
	}
	return n, nil
}
