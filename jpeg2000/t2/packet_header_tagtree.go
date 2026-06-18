package t2

import (
	"fmt"
	"sort"

	"github.com/cocosip/go-dicom-codec/jpeg2000/t1"
)

// encodePacketHeaderWithTagTree encodes a packet header using tag-tree encoding
// This matches OpenJPEG's approach and achieves much better compression
// encodePacketHeaderWithTagTree encodes a packet header using tag-tree encoding.
// params: precinct - target precinct; layer - current layer; _ - reserved
// returns: header bytes, block inclusions and error
func (pe *PacketEncoder) encodePacketHeaderWithTagTree(precinct *Precinct, layer int, _ int) ([]byte, []CodeBlockIncl, error) {
	cbIncls := make([]CodeBlockIncl, 0)
	pe.ensurePrecinctTrees(precinct)
	if layer == 0 {
		precinct.InclTree.ResetEncoding()
		precinct.ZBPTree.ResetEncoding()
	}
	for _, cb := range precinct.CodeBlocks {
		if !cb.Included {
			included, _, _ := pe.layerContribution(cb, layer)
			if included {
				precinct.InclTree.SetValue(cb.CBX, cb.CBY, layer)
			}
		}
		if layer == 0 {
			precinct.ZBPTree.SetValue(cb.CBX, cb.CBY, cb.ZeroBitPlanes)
		}
	}
	bitBuf := newBioWriter()
	if len(precinct.CodeBlocks) == 0 {
		bitBuf.writeBit(0)
		return bitBuf.flush(), cbIncls, nil
	}
	bitBuf.writeBit(1)
	for _, cb := range precinct.CodeBlocks {
		included, newPasses, layerData := pe.layerContribution(cb, layer)
		firstIncl := !cb.Included && included
		cbIncl := CodeBlockIncl{Included: included, FirstInclusion: firstIncl}
		proceed, err := pe.writeInclusionAndZBP(bitBuf, precinct, cb, layer, included)
		if err != nil {
			return nil, nil, err
		}
		if !proceed {
			cbIncls = append(cbIncls, cbIncl)
			continue
		}
		cbIncl.NumPasses = newPasses
		if err := encodeNumPasses(bitBuf, newPasses); err != nil {
			return nil, nil, fmt.Errorf("failed to encode number of passes: %w", err)
		}
		cbIncl.Data = layerData
		cbIncl.DataLength = len(layerData)
		cbIncl.PassLengths = pe.layerPassLengths(cb, layer)
		cbIncl.UseTERMALL = cb.UseTERMALL
		prevPasses, totalPasses := pe.computePrevAndTotalPasses(cb, layer, newPasses)
		termAll := cb.UseTERMALL
		passLens := buildPassLengths(cb.PassLengths, cb.Passes)
		if termAll && (passLens == nil || totalPasses > len(passLens)) {
			termAll = false
		}
		encodeCodeBlockLengths(bitBuf, cb, cbIncl.DataLength, prevPasses, newPasses, termAll, passLens)
		cbIncls = append(cbIncls, cbIncl)
	}
	return bitBuf.flush(), cbIncls, nil
}

// ensurePrecinctTrees prepares precinct dimensions and tag trees.
// params: precinct - target precinct
// returns: none
func (pe *PacketEncoder) ensurePrecinctTrees(precinct *Precinct) {
	if precinct.NumCodeBlocksX == 0 || precinct.NumCodeBlocksY == 0 {
		maxX, maxY := 0, 0
		for _, cb := range precinct.CodeBlocks {
			if cb.CBX+1 > maxX {
				maxX = cb.CBX + 1
			}
			if cb.CBY+1 > maxY {
				maxY = cb.CBY + 1
			}
		}
		precinct.NumCodeBlocksX = maxX
		precinct.NumCodeBlocksY = maxY
	}
	if precinct.InclTree == nil || precinct.ZBPTree == nil {
		precinct.InclTree = NewTagTree(precinct.NumCodeBlocksX, precinct.NumCodeBlocksY)
		precinct.ZBPTree = NewTagTree(precinct.NumCodeBlocksX, precinct.NumCodeBlocksY)
	}
}

// layerContribution returns inclusion status, new passes and layer data for a block.
// params: cb - code-block; layer - current layer
// returns: included flag, new passes count, layer data bytes
func (pe *PacketEncoder) layerContribution(cb *PrecinctCodeBlock, layer int) (bool, int, []byte) {
	included := false
	newPasses := 0
	var layerData []byte
	if cb.LayerData != nil && layer < len(cb.LayerData) {
		if layer < len(cb.LayerPasses) {
			totalPasses := cb.LayerPasses[layer]
			prevPasses := 0
			if layer > 0 {
				prevPasses = cb.LayerPasses[layer-1]
			}
			newPasses = totalPasses - prevPasses
			included = newPasses > 0
		}
		layerData = cb.LayerData[layer]
	} else {
		included = len(cb.Data) > 0
		newPasses = cb.NumPassesTotal
		layerData = cb.Data
	}
	return included, newPasses, layerData
}

// writeInclusionAndZBP writes inclusion tag-tree and zero-bitplane for first inclusion.
// params: bw - bit writer; precinct - precinct; cb - block; layer - current layer; included - inclusion flag
// returns: proceed flag (true if block continues), error if any
func (pe *PacketEncoder) writeInclusionAndZBP(bw *bioWriter, precinct *Precinct, cb *PrecinctCodeBlock, layer int, included bool) (bool, error) {
	if !cb.Included {
		threshold := layer + 1
		if err := precinct.InclTree.Encode(bw, cb.CBX, cb.CBY, threshold); err != nil {
			return false, fmt.Errorf("failed to encode inclusion tag-tree: %w", err)
		}
		if !included {
			return false, nil
		}
		if err := precinct.ZBPTree.Encode(bw, cb.CBX, cb.CBY, 999); err != nil {
			return false, fmt.Errorf("failed to encode zero-bitplane tag-tree: %w", err)
		}
		cb.Included = true
		return true, nil
	}
	if included {
		bw.writeBit(1)
		return true, nil
	}
	bw.writeBit(0)
	return false, nil
}

// encodePacketHeaderWithTagTreeMulti encodes a packet header across all bands in a precinct.
// It writes a single packet-present bit, then iterates bands in order.
// encodePacketHeaderWithTagTreeMulti encodes a packet header across all bands in precincts.
// params: precincts - list of precincts; layer - current layer
// returns: header bytes, block inclusions and error
func (pe *PacketEncoder) encodePacketHeaderWithTagTreeMulti(precincts []*Precinct, layer int) ([]byte, []CodeBlockIncl, error) {
	cbIncls := make([]CodeBlockIncl, 0)
	bitBuf := newBioWriter()

	if len(precincts) == 0 {
		bitBuf.writeBit(0)
		return bitBuf.flush(), cbIncls, nil
	}

	hasBlocks := false

	for _, precinct := range precincts {
		if precinct != nil && len(precinct.CodeBlocks) > 0 {
			hasBlocks = true
			break
		}
	}

	if !hasBlocks {
		bitBuf.writeBit(0)
		return bitBuf.flush(), cbIncls, nil
	}
	bitBuf.writeBit(1)

	for _, precinct := range precincts {
		if precinct == nil || len(precinct.CodeBlocks) == 0 {
			continue
		}
		sort.Slice(precinct.CodeBlocks, func(i, j int) bool {
			ai := precinct.CodeBlocks[i]
			aj := precinct.CodeBlocks[j]
			if ai.CBY != aj.CBY {
				return ai.CBY < aj.CBY
			}
			return ai.CBX < aj.CBX
		})

		if precinct.InclTree == nil || precinct.ZBPTree == nil ||
			precinct.InclTree.Width() != precinct.NumCodeBlocksX ||
			precinct.InclTree.Height() != precinct.NumCodeBlocksY {
			precinct.InclTree = NewTagTree(precinct.NumCodeBlocksX, precinct.NumCodeBlocksY)
			precinct.ZBPTree = NewTagTree(precinct.NumCodeBlocksX, precinct.NumCodeBlocksY)
		}

		if layer == 0 {
			precinct.InclTree.ResetEncoding()
			precinct.ZBPTree.ResetEncoding()
		}

		for _, cb := range precinct.CodeBlocks {
			if !cb.Included {
				included, _, _ := pe.layerContribution(cb, layer)
				if included {
					precinct.InclTree.SetValue(cb.CBX, cb.CBY, layer)
				}
			}
			if layer == 0 {
				precinct.ZBPTree.SetValue(cb.CBX, cb.CBY, cb.ZeroBitPlanes)
			}
		}
	}

	for _, precinct := range precincts {
		if precinct == nil || len(precinct.CodeBlocks) == 0 {
			continue
		}

		for _, cb := range precinct.CodeBlocks {
			included, newPasses, layerData := pe.layerContribution(cb, layer)
			firstIncl := !cb.Included && included

			cbIncl := CodeBlockIncl{
				Included:       included,
				FirstInclusion: firstIncl,
			}

			proceed, err := pe.writeInclusionAndZBP(bitBuf, precinct, cb, layer, included)
			if err != nil {
				return nil, nil, err
			}
			if !proceed {
				cbIncls = append(cbIncls, cbIncl)
				continue
			}

			cbIncl.NumPasses = newPasses
			if err := encodeNumPasses(bitBuf, newPasses); err != nil {
				return nil, nil, fmt.Errorf("failed to encode number of passes: %w", err)
			}

			dataLen := len(layerData)
			cbIncl.Data = layerData

			cbIncl.PassLengths = pe.layerPassLengths(cb, layer)
			cbIncl.UseTERMALL = cb.UseTERMALL

			cbIncl.DataLength = dataLen

			prevPasses, totalPasses := pe.computePrevAndTotalPasses(cb, layer, newPasses)

			termAll := cb.UseTERMALL
			passLens := buildPassLengths(cb.PassLengths, cb.Passes)
			if termAll && (passLens == nil || totalPasses > len(passLens)) {
				termAll = false
			}

			encodeCodeBlockLengths(bitBuf, cb, cbIncl.DataLength, prevPasses, newPasses, termAll, passLens)

			cbIncls = append(cbIncls, cbIncl)
		}
	}

	return bitBuf.flush(), cbIncls, nil
}

// layerPassLengths computes per-layer pass lengths slice.
// params: cb - code-block; layer - current layer
// returns: per-pass lengths for the layer
func (pe *PacketEncoder) layerPassLengths(cb *PrecinctCodeBlock, layer int) []int {
	if cb.LayerData != nil && layer < len(cb.LayerPasses) {
		totalPasses := cb.LayerPasses[layer]
		prevPasses := 0
		if layer > 0 {
			prevPasses = cb.LayerPasses[layer-1]
		}
		if totalPasses <= len(cb.PassLengths) {
			layerPassLengths := make([]int, totalPasses-prevPasses)
			baseOffset := 0
			if prevPasses > 0 && prevPasses <= len(cb.PassLengths) {
				baseOffset = cb.PassLengths[prevPasses-1]
			}
			for i := prevPasses; i < totalPasses && i < len(cb.PassLengths); i++ {
				layerPassLengths[i-prevPasses] = cb.PassLengths[i] - baseOffset
			}
			return layerPassLengths
		}
		return nil
	}
	return cb.PassLengths
}

// computePrevAndTotalPasses calculates previous and total pass counts for lengths coding.
// params: cb - code-block; layer - current layer; newPasses - passes in this layer
// returns: prevPasses and totalPasses
func (pe *PacketEncoder) computePrevAndTotalPasses(cb *PrecinctCodeBlock, layer, newPasses int) (int, int) {
	prevPasses := 0
	totalPasses := newPasses
	if cb.LayerPasses != nil && layer < len(cb.LayerPasses) {
		totalPasses = cb.LayerPasses[layer]
		if layer > 0 {
			prevPasses = cb.LayerPasses[layer-1]
		}
	} else if cb.NumPassesTotal > 0 {
		prevPasses = cb.NumPassesTotal - newPasses
		if prevPasses < 0 {
			prevPasses = 0
		}
		totalPasses = prevPasses + newPasses
	}
	return prevPasses, totalPasses
}

// encodeNumPasses encodes the number of coding passes using JPEG2000 standard encoding
// Matches OpenJPEG's opj_t2_putnumpasses() in t2.c:184-198
type packetBitWriter interface {
	writeBit(int)
	writeBits(int, int)
}

func encodeNumPasses(bw packetBitWriter, n int) error {
	if n == 1 {
		// 1 pass: "0" (1 bit)
		bw.writeBit(0)
	} else if n == 2 {
		// 2 passes: "10" (2 bits)
		bw.writeBits(2, 2) // value=2 (0b10), bits=2
	} else if n <= 5 {
		// 3-5 passes: "11xx" (4 bits)
		// 0xc = 0b1100, combined with (n-3) in lower 2 bits
		val := 0x0c | (n - 3)
		bw.writeBits(val, 4)
	} else if n <= 36 {
		// 6-36 passes: "1111xxxxx" (9 bits total)
		// 0x1e0 = 0b111100000 (prefix 1111, then 5 bits for value)
		// OpenJPEG: opj_bio_write(bio, 0x1e0 | (n - 6), 9)
		val := 0x1e0 | (n - 6)
		bw.writeBits(val, 9)
	} else if n <= 164 {
		// 37-164 passes: "111111111" + 7-bit value (16 bits total)
		// 0xff80 = 0b1111111110000000 (prefix 111111111, then 7 bits for value)
		// OpenJPEG: opj_bio_write(bio, 0xff80 | (n - 37), 16)
		val := 0xff80 | (n - 37)
		bw.writeBits(val, 16)
	} else {
		return fmt.Errorf("number of passes %d exceeds maximum 164", n)
	}
	return nil
}

func encodeCodeBlockLengths(bw packetBitWriter, cb *PrecinctCodeBlock, dataLen, prevPasses, newPasses int, termAll bool, passLens []int) {
	if newPasses <= 0 {
		encodeCommaCode(bw, 0)
		return
	}
	if cb.NumLenBits <= 0 {
		cb.NumLenBits = 3
	}

	// Fallback: no per-pass lengths, emit a single segment length.
	if passLens == nil || prevPasses+newPasses > len(passLens) {
		increment := (floorLog2(dataLen) + 1) - (cb.NumLenBits + floorLog2(newPasses))
		if increment < 0 {
			increment = 0
		}
		encodeCommaCode(bw, increment)
		cb.NumLenBits += increment
		bitCount := cb.NumLenBits + floorLog2(newPasses)
		bw.writeBits(dataLen, bitCount)
		return
	}

	increment := 0
	nump := 0
	segLen := 0
	lastPass := prevPasses + newPasses - 1
	for passIdx := prevPasses; passIdx <= lastPass; passIdx++ {
		nump++
		segLen += passLens[passIdx]
		terminate := codeBlockPassTerminates(cb, termAll, passIdx) || passIdx == lastPass
		if terminate {
			need := (floorLog2(segLen) + 1) - (cb.NumLenBits + floorLog2(nump))
			if need > increment {
				increment = need
			}
			segLen = 0
			nump = 0
		}
	}
	if increment < 0 {
		increment = 0
	}
	encodeCommaCode(bw, increment)
	cb.NumLenBits += increment

	nump = 0
	segLen = 0
	for passIdx := prevPasses; passIdx <= lastPass; passIdx++ {
		nump++
		segLen += passLens[passIdx]
		terminate := codeBlockPassTerminates(cb, termAll, passIdx) || passIdx == lastPass
		if terminate {
			bitCount := cb.NumLenBits + floorLog2(nump)
			bw.writeBits(segLen, bitCount)
			segLen = 0
			nump = 0
		}
	}
}

func codeBlockPassTerminates(cb *PrecinctCodeBlock, termAll bool, passIdx int) bool {
	if termAll {
		return true
	}
	if cb != nil && passIdx >= 0 && passIdx < len(cb.Passes) {
		return cb.Passes[passIdx].Terminated
	}
	return false
}

func buildPassLengths(cumulative []int, passes []t1.PassData) []int {
	if len(cumulative) > 0 {
		out := make([]int, len(cumulative))
		prev := 0
		for i, v := range cumulative {
			if v < prev {
				v = prev
			}
			out[i] = v - prev
			prev = v
		}
		return out
	}
	if len(passes) > 0 {
		out := make([]int, len(passes))
		for i, p := range passes {
			if p.Len > 0 {
				out[i] = p.Len
			} else if p.ActualBytes > 0 {
				out[i] = p.ActualBytes
			}
		}
		return out
	}
	return nil
}

func encodeCommaCode(bw packetBitWriter, n int) {
	for i := 0; i < n; i++ {
		bw.writeBit(1)
	}
	bw.writeBit(0)
}
