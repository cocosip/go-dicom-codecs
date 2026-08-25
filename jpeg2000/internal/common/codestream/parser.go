package codestream

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Parser parses JPEG 2000 codestreams
type Parser struct {
	*boundedReader
}

// NewParser creates a new codestream parser
func NewParser(data []byte) *Parser {
	return &Parser{
		boundedReader: newBoundedReader(data),
	}
}

// Parse parses the entire codestream
func (p *Parser) Parse() (*Codestream, error) {
	cs := &Codestream{
		Data: p.data,
	}

	// Read SOC marker
	marker, err := p.readMarker()
	if err != nil {
		return nil, fmt.Errorf("failed to read SOC: %w", err)
	}
	if marker != MarkerSOC {
		return nil, fmt.Errorf("expected SOC marker (0x%04X), got 0x%04X", MarkerSOC, marker)
	}

	// Parse main header
	if err := p.parseMainHeader(cs); err != nil {
		return nil, fmt.Errorf("failed to parse main header: %w", err)
	}

	// Parse tiles (including multi-tile-part concatenation)
	tileByIndex := make(map[int]*Tile)
	tileStates := make(map[int]*tilePartState)
	for {
		marker, err := p.peekMarker()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if marker == MarkerEOC {
			// End of codestream
			_, _ = p.readMarker() // consume EOC
			break
		}

		if marker == MarkerSOT {
			tile, err := p.parseTile(cs)
			if err != nil {
				return nil, fmt.Errorf("failed to parse tile: %w", err)
			}
			if err := mergeTilePart(cs, tileByIndex, tileStates, tile); err != nil {
				return nil, fmt.Errorf("failed to merge tile-part: %w", err)
			}
		} else {
			return nil, fmt.Errorf("unexpected marker in tile sequence: 0x%04X (%s)", marker, MarkerName(marker))
		}
	}

	return cs, nil
}

// parseMainHeader parses the main header segments
func (p *Parser) parseMainHeader(cs *Codestream) error {
	state := &mainHeaderState{}
	if err := p.consumeMainHeader(cs, state); err != nil {
		return err
	}

	// Verify required segments
	if cs.SIZ == nil {
		return fmt.Errorf("missing required SIZ segment")
	}
	if cs.COD == nil {
		return fmt.Errorf("missing required COD segment")
	}
	if cs.QCD == nil {
		return fmt.Errorf("missing required QCD segment")
	}

	return nil
}

type mainHeaderState struct {
	seenSIZ bool
	seenCOD bool
	seenQCD bool
}

func (p *Parser) consumeMainHeader(cs *Codestream, st *mainHeaderState) error {
	handlers := map[uint16]func() error{
		MarkerSIZ: func() error { return p.mainSIZ(cs, st) },
		MarkerCOD: func() error { return p.mainCOD(cs, st) },
		MarkerCOC: func() error { return p.mainCOC(cs, st) },
		MarkerQCD: func() error { return p.mainQCD(cs, st) },
		MarkerQCC: func() error { return p.mainQCC(cs, st) },
		MarkerPOC: func() error { return p.mainPOC(cs, st) },
		MarkerRGN: func() error { return p.mainRGN(cs, st) },
		MarkerCOM: func() error { return p.mainCOM(cs, st) },
		MarkerMCT: func() error { return p.mainMCT(cs, st) },
		MarkerMCC: func() error { return p.mainMCC(cs, st) },
		MarkerMCO: func() error { return p.mainMCO(cs, st) },
	}
	for {
		marker, err := p.peekMarker()
		if err != nil {
			return err
		}
		if marker == MarkerSOT || marker == MarkerEOC {
			return nil
		}
		marker, err = p.readMarker()
		if err != nil {
			return err
		}
		if h, ok := handlers[marker]; ok {
			if err := h(); err != nil {
				return err
			}
		} else {
			if !st.seenSIZ {
				return fmt.Errorf("unexpected marker before SIZ: 0x%04X (%s)", marker, MarkerName(marker))
			}
			if err := p.skipSegment(); err != nil {
				return fmt.Errorf("failed to skip segment 0x%04X: %w", marker, err)
			}
		}
	}
}

func (p *Parser) mainSIZ(cs *Codestream, st *mainHeaderState) error {
	if st.seenSIZ {
		return fmt.Errorf("duplicate SIZ segment")
	}
	siz, err := p.parseSIZ()
	if err != nil {
		return fmt.Errorf("failed to parse SIZ: %w", err)
	}
	cs.SIZ = siz
	st.seenSIZ = true
	return nil
}

func (p *Parser) mainCOD(cs *Codestream, st *mainHeaderState) error {
	if !st.seenSIZ {
		return fmt.Errorf("COD encountered before SIZ")
	}
	if st.seenCOD {
		return fmt.Errorf("duplicate COD segment")
	}
	cod, err := p.parseCOD()
	if err != nil {
		return fmt.Errorf("failed to parse COD: %w", err)
	}
	cs.COD = cod
	st.seenCOD = true
	return nil
}

func (p *Parser) mainCOC(cs *Codestream, st *mainHeaderState) error {
	if !st.seenSIZ {
		return fmt.Errorf("COC encountered before SIZ")
	}
	if !st.seenCOD {
		return fmt.Errorf("COC encountered before COD")
	}
	coc, err := p.parseCOC(cs.SIZ)
	if err != nil {
		return fmt.Errorf("failed to parse COC: %w", err)
	}
	if cs.COC == nil {
		cs.COC = make(map[uint16]*COCSegment)
	}
	if existing, ok := cs.COC[coc.Component]; ok && !cocEqual(existing, coc) {
		return fmt.Errorf("duplicate COC for component %d", coc.Component)
	}
	cs.COC[coc.Component] = coc
	return nil
}

func (p *Parser) mainQCD(cs *Codestream, st *mainHeaderState) error {
	if !st.seenSIZ {
		return fmt.Errorf("QCD encountered before SIZ")
	}
	if st.seenQCD {
		return fmt.Errorf("duplicate QCD segment")
	}
	qcd, err := p.parseQCD()
	if err != nil {
		return fmt.Errorf("failed to parse QCD: %w", err)
	}
	cs.QCD = qcd
	st.seenQCD = true
	return nil
}

func (p *Parser) mainQCC(cs *Codestream, st *mainHeaderState) error {
	if !st.seenSIZ {
		return fmt.Errorf("QCC encountered before SIZ")
	}
	if !st.seenQCD {
		return fmt.Errorf("QCC encountered before QCD")
	}
	qcc, err := p.parseQCC(cs.SIZ)
	if err != nil {
		return fmt.Errorf("failed to parse QCC: %w", err)
	}
	if cs.QCC == nil {
		cs.QCC = make(map[uint16]*QCCSegment)
	}
	if existing, ok := cs.QCC[qcc.Component]; ok && !qccEqual(existing, qcc) {
		return fmt.Errorf("duplicate QCC for component %d", qcc.Component)
	}
	cs.QCC[qcc.Component] = qcc
	return nil
}

func (p *Parser) mainPOC(cs *Codestream, st *mainHeaderState) error {
	if !st.seenSIZ {
		return fmt.Errorf("POC encountered before SIZ")
	}
	if !st.seenCOD {
		return fmt.Errorf("POC encountered before COD")
	}
	poc, err := p.parsePOC(cs.SIZ)
	if err != nil {
		return fmt.Errorf("failed to parse POC: %w", err)
	}
	cs.POC = append(cs.POC, *poc)
	return nil
}

func (p *Parser) mainRGN(cs *Codestream, st *mainHeaderState) error {
	if !st.seenSIZ {
		return fmt.Errorf("RGN encountered before SIZ")
	}
	rgn, err := p.parseRGN(cs.SIZ)
	if err != nil {
		return fmt.Errorf("failed to parse RGN: %w", err)
	}
	cs.RGN = append(cs.RGN, *rgn)
	return nil
}

func (p *Parser) mainCOM(cs *Codestream, st *mainHeaderState) error {
	if !st.seenSIZ {
		return fmt.Errorf("COM encountered before SIZ")
	}
	com, err := p.parseCOM()
	if err != nil {
		return fmt.Errorf("failed to parse COM: %w", err)
	}
	cs.COM = append(cs.COM, *com)
	return nil
}

func (p *Parser) mainMCT(cs *Codestream, st *mainHeaderState) error {
	if !st.seenSIZ {
		return fmt.Errorf("MCT encountered before SIZ")
	}
	seg, err := p.parseMCT()
	if err != nil {
		return fmt.Errorf("failed to parse MCT: %w", err)
	}
	cs.MCT = append(cs.MCT, *seg)
	return nil
}

func (p *Parser) mainMCC(cs *Codestream, st *mainHeaderState) error {
	if !st.seenSIZ {
		return fmt.Errorf("MCC encountered before SIZ")
	}
	seg, err := p.parseMCC()
	if err != nil {
		return fmt.Errorf("failed to parse MCC: %w", err)
	}
	cs.MCC = append(cs.MCC, *seg)
	return nil
}

func (p *Parser) mainMCO(cs *Codestream, st *mainHeaderState) error {
	if !st.seenSIZ {
		return fmt.Errorf("MCO encountered before SIZ")
	}
	seg, err := p.parseMCO()
	if err != nil {
		return fmt.Errorf("failed to parse MCO: %w", err)
	}
	cs.MCO = append(cs.MCO, *seg)
	return nil
}

// parseTile parses a single tile
func (p *Parser) parseTile(cs *Codestream) (*Tile, error) {
	tileStart := p.offset
	// Read SOT
	marker, err := p.readMarker()
	if err != nil {
		return nil, err
	}
	if marker != MarkerSOT {
		return nil, fmt.Errorf("expected SOT, got 0x%04X", marker)
	}

	sot, err := p.parseSOT()
	if err != nil {
		return nil, err
	}

	tile := &Tile{
		Index: int(sot.Isot),
		SOT:   sot,
	}

	// Parse tile header segments until SOD
	if err := p.parseTileHeader(cs, tile); err != nil {
		return nil, err
	}

	// Read tile data using Psot length when available.
	tile.Data = p.readTileDataWithLength(tileStart, sot.Psot)

	return tile, nil
}

// parseTileHeader consumes tile-part header markers until SOD and updates tile
func (p *Parser) parseTileHeader(cs *Codestream, tile *Tile) error {
	handlers := map[uint16]func() error{
		MarkerCOD: func() error { return p.handleCOD(tile) },
		MarkerCOC: func() error { return p.handleCOC(cs, tile) },
		MarkerQCD: func() error { return p.handleQCD(tile) },
		MarkerQCC: func() error { return p.handleQCC(cs, tile) },
		MarkerPOC: func() error { return p.handlePOC(cs, tile) },
		MarkerRGN: func() error { return p.handleRGN(cs, tile) },
		MarkerMCT: func() error { return p.handleMCT(cs) },
		MarkerMCC: func() error { return p.handleMCC(cs) },
		MarkerMCO: func() error { return p.handleMCO(cs) },
	}
	for {
		marker, err := p.peekMarker()
		if err != nil {
			return err
		}
		if marker == MarkerSOD {
			_, _ = p.readMarker()
			return nil
		}
		marker, err = p.readMarker()
		if err != nil {
			return err
		}
		if h, ok := handlers[marker]; ok {
			if err := h(); err != nil {
				return err
			}
		} else {
			if err := p.skipSegment(); err != nil {
				return err
			}
		}
	}
}

func (p *Parser) handleCOD(tile *Tile) error {
	cod, err := p.parseCOD()
	if err != nil {
		return err
	}
	tile.COD = cod
	return nil
}

func (p *Parser) handleCOC(cs *Codestream, tile *Tile) error {
	if cs == nil || cs.SIZ == nil {
		return fmt.Errorf("COC encountered before SIZ")
	}
	coc, err := p.parseCOC(cs.SIZ)
	if err != nil {
		return err
	}
	if tile.COC == nil {
		tile.COC = make(map[uint16]*COCSegment)
	}
	if existing, ok := tile.COC[coc.Component]; ok && !cocEqual(existing, coc) {
		return fmt.Errorf("duplicate tile COC for component %d", coc.Component)
	}
	tile.COC[coc.Component] = coc
	return nil
}

func (p *Parser) handleQCD(tile *Tile) error {
	qcd, err := p.parseQCD()
	if err != nil {
		return err
	}
	tile.QCD = qcd
	return nil
}

func (p *Parser) handleQCC(cs *Codestream, tile *Tile) error {
	if cs == nil || cs.SIZ == nil {
		return fmt.Errorf("QCC encountered before SIZ")
	}
	qcc, err := p.parseQCC(cs.SIZ)
	if err != nil {
		return err
	}
	if tile.QCC == nil {
		tile.QCC = make(map[uint16]*QCCSegment)
	}
	if existing, ok := tile.QCC[qcc.Component]; ok && !qccEqual(existing, qcc) {
		return fmt.Errorf("duplicate tile QCC for component %d", qcc.Component)
	}
	tile.QCC[qcc.Component] = qcc
	return nil
}

func (p *Parser) handlePOC(cs *Codestream, tile *Tile) error {
	if cs == nil || cs.SIZ == nil {
		return fmt.Errorf("POC encountered before SIZ")
	}
	poc, err := p.parsePOC(cs.SIZ)
	if err != nil {
		return err
	}
	tile.POC = append(tile.POC, *poc)
	return nil
}

func (p *Parser) handleRGN(cs *Codestream, tile *Tile) error {
	var siz *SIZSegment
	if cs != nil {
		siz = cs.SIZ
	}
	rgn, err := p.parseRGN(siz)
	if err != nil {
		return err
	}
	tile.RGN = append(tile.RGN, rgn)
	return nil
}

func (p *Parser) handleMCT(cs *Codestream) error {
	seg, err := p.parseMCT()
	if err != nil {
		return err
	}
	if cs != nil {
		cs.MCT = append(cs.MCT, *seg)
	}
	return nil
}

func (p *Parser) handleMCC(cs *Codestream) error {
	seg, err := p.parseMCC()
	if err != nil {
		return err
	}
	if cs != nil {
		cs.MCC = append(cs.MCC, *seg)
	}
	return nil
}

func (p *Parser) handleMCO(cs *Codestream) error {
	seg, err := p.parseMCO()
	if err != nil {
		return err
	}
	if cs != nil {
		cs.MCO = append(cs.MCO, *seg)
	}
	return nil
}

type tilePartState struct {
	nextTP uint8
	total  uint8
}

func mergeTilePart(cs *Codestream, tiles map[int]*Tile, states map[int]*tilePartState, part *Tile) error {
	if part == nil || part.SOT == nil {
		return fmt.Errorf("missing SOT for tile-part")
	}
	idx := part.Index
	state, ok := states[idx]
	if !ok {
		if part.SOT.TPsot != 0 {
			return fmt.Errorf("tile %d: first tile-part index is %d", idx, part.SOT.TPsot)
		}
		state = &tilePartState{nextTP: part.SOT.TPsot + 1, total: part.SOT.TNsot}
		states[idx] = state
	} else {
		if part.SOT.TPsot != state.nextTP {
			return fmt.Errorf("tile %d: unexpected tile-part index %d (expected %d)", idx, part.SOT.TPsot, state.nextTP)
		}
		if state.total != 0 && part.SOT.TNsot != 0 && part.SOT.TNsot != state.total {
			return fmt.Errorf("tile %d: mismatched TNsot %d (expected %d)", idx, part.SOT.TNsot, state.total)
		}
		if state.total == 0 && part.SOT.TNsot != 0 {
			state.total = part.SOT.TNsot
		}
		state.nextTP++
	}
	if state.total != 0 && state.nextTP > state.total {
		return fmt.Errorf("tile %d: tile-part count exceeded (TNsot=%d)", idx, state.total)
	}

	existing := tiles[idx]
	if existing == nil {
		tiles[idx] = part
		if cs != nil {
			cs.Tiles = append(cs.Tiles, part)
		}
		return nil
	}
	if existing.SOT != nil && existing.SOT.TNsot == 0 && state.total != 0 {
		existing.SOT.TNsot = state.total
	}

	if err := mergeCODSection(existing, part, idx); err != nil {
		return err
	}
	if err := mergeQCDSection(existing, part, idx); err != nil {
		return err
	}
	if err := mergeCOCSection(existing, part, idx); err != nil {
		return err
	}
	if err := mergeQCCSection(existing, part, idx); err != nil {
		return err
	}
	if err := mergePOCSection(existing, part, idx); err != nil {
		return err
	}
	if err := mergeRGNSection(existing, part, idx); err != nil {
		return err
	}

	if len(part.Data) > 0 {
		existing.Data = append(existing.Data, part.Data...)
	}

	return nil
}

func mergeCODSection(existing, part *Tile, idx int) error {
	if part.COD != nil {
		if existing.COD == nil {
			existing.COD = part.COD
		} else if !codEqual(existing.COD, part.COD) {
			return fmt.Errorf("tile %d: COD differs between tile-parts", idx)
		}
	}
	return nil
}

func mergeQCDSection(existing, part *Tile, idx int) error {
	if part.QCD != nil {
		if existing.QCD == nil {
			existing.QCD = part.QCD
		} else if !qcdEqual(existing.QCD, part.QCD) {
			return fmt.Errorf("tile %d: QCD differs between tile-parts", idx)
		}
	}
	return nil
}

func mergeCOCSection(existing, part *Tile, idx int) error {
	if len(part.COC) > 0 {
		if existing.COC == nil {
			existing.COC = make(map[uint16]*COCSegment)
		}
		for comp, coc := range part.COC {
			if prior, ok := existing.COC[comp]; ok {
				if !cocEqual(prior, coc) {
					return fmt.Errorf("tile %d: COC differs for component %d", idx, comp)
				}
				continue
			}
			existing.COC[comp] = coc
		}
	}
	return nil
}

func mergeQCCSection(existing, part *Tile, idx int) error {
	if len(part.QCC) > 0 {
		if existing.QCC == nil {
			existing.QCC = make(map[uint16]*QCCSegment)
		}
		for comp, qcc := range part.QCC {
			if prior, ok := existing.QCC[comp]; ok {
				if !qccEqual(prior, qcc) {
					return fmt.Errorf("tile %d: QCC differs for component %d", idx, comp)
				}
				continue
			}
			existing.QCC[comp] = qcc
		}
	}
	return nil
}

func mergePOCSection(existing, part *Tile, idx int) error {
	if len(part.POC) > 0 {
		if len(existing.POC) == 0 {
			existing.POC = append(existing.POC, part.POC...)
		} else if !pocEqual(existing.POC, part.POC) {
			return fmt.Errorf("tile %d: POC differs between tile-parts", idx)
		}
	}
	return nil
}

func mergeRGNSection(existing, part *Tile, idx int) error {
	if len(part.RGN) > 0 {
		if len(existing.RGN) == 0 {
			existing.RGN = append(existing.RGN, part.RGN...)
		} else if !rgnEqual(existing.RGN, part.RGN) {
			return fmt.Errorf("tile %d: RGN differs between tile-parts", idx)
		}
	}
	return nil
}

// parseRGN parses the RGN marker segment (ROI).
func (p *Parser) parseRGN(siz *SIZSegment) (*RGNSegment, error) {
	length, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	compBytes := componentIndexSize(siz)
	minLen := uint16(4 + compBytes)
	if length < minLen {
		return nil, fmt.Errorf("invalid RGN length: %d", length)
	}

	crgn, err := p.readComponentIndex(siz)
	if err != nil {
		return nil, err
	}
	srgn, err := p.readUint8()
	if err != nil {
		return nil, err
	}
	sprgn, err := p.readUint8()
	if err != nil {
		return nil, err
	}

	// Skip remaining bytes if any
	remain := int(length) - int(minLen)
	if remain > 0 {
		if err := p.read(make([]byte, remain)); err != nil {
			return nil, err
		}
	}

	return &RGNSegment{
		Crgn:  crgn,
		Srgn:  srgn,
		SPrgn: sprgn,
	}, nil
}

// parseSIZ parses the SIZ marker segment
func (p *Parser) parseSIZ() (*SIZSegment, error) {
	length, err := p.readUint16()
	if err != nil {
		return nil, err
	}

	siz := &SIZSegment{}

	if siz.Rsiz, err = p.readUint16(); err != nil {
		return nil, err
	}
	if siz.Xsiz, err = p.readUint32(); err != nil {
		return nil, err
	}
	if siz.Ysiz, err = p.readUint32(); err != nil {
		return nil, err
	}
	if siz.XOsiz, err = p.readUint32(); err != nil {
		return nil, err
	}
	if siz.YOsiz, err = p.readUint32(); err != nil {
		return nil, err
	}
	if siz.XTsiz, err = p.readUint32(); err != nil {
		return nil, err
	}
	if siz.YTsiz, err = p.readUint32(); err != nil {
		return nil, err
	}
	if siz.XTOsiz, err = p.readUint32(); err != nil {
		return nil, err
	}
	if siz.YTOsiz, err = p.readUint32(); err != nil {
		return nil, err
	}
	if siz.Csiz, err = p.readUint16(); err != nil {
		return nil, err
	}

	// Read component sizing information
	siz.Components = make([]ComponentSize, siz.Csiz)
	for i := range siz.Components {
		if siz.Components[i].Ssiz, err = p.readUint8(); err != nil {
			return nil, err
		}
		if siz.Components[i].XRsiz, err = p.readUint8(); err != nil {
			return nil, err
		}
		if siz.Components[i].YRsiz, err = p.readUint8(); err != nil {
			return nil, err
		}
	}

	// Verify length
	expectedLength := 38 + 3*int(siz.Csiz)
	if int(length) != expectedLength {
		return nil, fmt.Errorf("SIZ segment length mismatch: expected %d, got %d", expectedLength, length)
	}

	return siz, nil
}

// parseCOD parses the COD marker segment
func (p *Parser) parseCOD() (*CODSegment, error) {
	length, err := p.readUint16()
	if err != nil {
		return nil, err
	}

	cod := &CODSegment{}
	start := p.offset

	if cod.Scod, err = p.readUint8(); err != nil {
		return nil, err
	}
	if cod.ProgressionOrder, err = p.readUint8(); err != nil {
		return nil, err
	}
	if cod.NumberOfLayers, err = p.readUint16(); err != nil {
		return nil, err
	}
	if cod.MultipleComponentTransform, err = p.readUint8(); err != nil {
		return nil, err
	}
	numLevels, cbw, cbh, cbStyle, transform, precincts, err := p.parseCodingStyleParams(cod.Scod)
	if err != nil {
		return nil, err
	}
	cod.NumberOfDecompositionLevels = numLevels
	cod.CodeBlockWidth = cbw
	cod.CodeBlockHeight = cbh
	cod.CodeBlockStyle = cbStyle
	cod.Transformation = transform
	cod.PrecinctSizes = precincts

	consumed := p.offset - start
	expected := int(length) - 2
	if consumed > expected {
		return nil, fmt.Errorf("COD segment length mismatch: expected %d, got %d", expected, consumed)
	}
	if consumed < expected {
		p.offset += expected - consumed
	}

	return cod, nil
}

// parseCOC parses the COC marker segment (component coding style).
func (p *Parser) parseCOC(siz *SIZSegment) (*COCSegment, error) {
	length, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	start := p.offset
	comp, err := p.readComponentIndex(siz)
	if err != nil {
		return nil, err
	}
	scoc, err := p.readUint8()
	if err != nil {
		return nil, err
	}
	numLevels, cbw, cbh, cbStyle, transform, precincts, err := p.parseCodingStyleParams(scoc)
	if err != nil {
		return nil, err
	}
	coc := &COCSegment{
		Component:                   comp,
		Scoc:                        scoc,
		NumberOfDecompositionLevels: numLevels,
		CodeBlockWidth:              cbw,
		CodeBlockHeight:             cbh,
		CodeBlockStyle:              cbStyle,
		Transformation:              transform,
		PrecinctSizes:               precincts,
	}
	consumed := p.offset - start
	expected := int(length) - 2
	if consumed > expected {
		return nil, fmt.Errorf("COC segment length mismatch: expected %d, got %d", expected, consumed)
	}
	if consumed < expected {
		p.offset += expected - consumed
	}
	return coc, nil
}

// parseQCC parses the QCC marker segment (component quantization).
func (p *Parser) parseQCC(siz *SIZSegment) (*QCCSegment, error) {
	length, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	start := p.offset
	comp, err := p.readComponentIndex(siz)
	if err != nil {
		return nil, err
	}
	sqcc, err := p.readUint8()
	if err != nil {
		return nil, err
	}
	dataLen := int(length) - 3 - componentIndexSize(siz)
	if dataLen < 0 {
		return nil, fmt.Errorf("invalid QCC length: %d", length)
	}
	spqcc := make([]byte, dataLen)
	if dataLen > 0 {
		if err := p.read(spqcc); err != nil {
			return nil, err
		}
	}
	consumed := p.offset - start
	expected := int(length) - 2
	if consumed > expected {
		return nil, fmt.Errorf("QCC segment length mismatch: expected %d, got %d", expected, consumed)
	}
	if consumed < expected {
		p.offset += expected - consumed
	}
	return &QCCSegment{Component: comp, Sqcc: sqcc, SPqcc: spqcc}, nil
}

// parsePOC parses the POC marker segment.
func (p *Parser) parsePOC(siz *SIZSegment) (*POCSegment, error) {
	length, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	remaining := int(length) - 2
	compBytes := componentIndexSize(siz)
	entryLen := 5 + 2*compBytes
	if remaining < entryLen || remaining%entryLen != 0 {
		return nil, fmt.Errorf("invalid POC length: %d", length)
	}
	entries := make([]POCEntry, 0, remaining/entryLen)
	for i := 0; i < remaining/entryLen; i++ {
		rs, err := p.readUint8()
		if err != nil {
			return nil, err
		}
		csVal, err := p.readComponentIndex(siz)
		if err != nil {
			return nil, err
		}
		ly, err := p.readUint16()
		if err != nil {
			return nil, err
		}
		re, err := p.readUint8()
		if err != nil {
			return nil, err
		}
		ceVal, err := p.readComponentIndex(siz)
		if err != nil {
			return nil, err
		}
		pp, err := p.readUint8()
		if err != nil {
			return nil, err
		}
		entries = append(entries, POCEntry{
			RSpoc:  rs,
			CSpoc:  csVal,
			LYEpoc: ly,
			REpoc:  re,
			CEpoc:  ceVal,
			Ppoc:   pp,
		})
	}
	return &POCSegment{Entries: entries}, nil
}

func (p *Parser) parseCodingStyleParams(scod uint8) (uint8, uint8, uint8, uint8, uint8, []PrecinctSize, error) {
	numLevels, err := p.readUint8()
	if err != nil {
		return 0, 0, 0, 0, 0, nil, err
	}
	cbw, err := p.readUint8()
	if err != nil {
		return 0, 0, 0, 0, 0, nil, err
	}
	cbh, err := p.readUint8()
	if err != nil {
		return 0, 0, 0, 0, 0, nil, err
	}
	cbStyle, err := p.readUint8()
	if err != nil {
		return 0, 0, 0, 0, 0, nil, err
	}
	transform, err := p.readUint8()
	if err != nil {
		return 0, 0, 0, 0, 0, nil, err
	}

	var precincts []PrecinctSize
	if (scod & 0x01) != 0 {
		count := int(numLevels) + 1
		precincts = make([]PrecinctSize, count)
		for i := 0; i < count; i++ {
			ppxppy, err := p.readUint8()
			if err != nil {
				return 0, 0, 0, 0, 0, nil, err
			}
			precincts[i].PPx = ppxppy & 0x0F
			precincts[i].PPy = ppxppy >> 4
		}
	}
	return numLevels, cbw, cbh, cbStyle, transform, precincts, nil
}

// parseQCD parses the QCD marker segment
func (p *Parser) parseQCD() (*QCDSegment, error) {
	length, err := p.readUint16()
	if err != nil {
		return nil, err
	}

	qcd := &QCDSegment{}

	if qcd.Sqcd, err = p.readUint8(); err != nil {
		return nil, err
	}

	// Read quantization step size values
	dataLength := int(length) - 3 // length includes itself (2) and Sqcd (1)
	qcd.SPqcd = make([]byte, dataLength)
	if err := p.read(qcd.SPqcd); err != nil {
		return nil, err
	}

	return qcd, nil
}

// parseCOM parses the COM marker segment
func (p *Parser) parseCOM() (*COMSegment, error) {
	length, err := p.readUint16()
	if err != nil {
		return nil, err
	}

	com := &COMSegment{}

	if com.Rcom, err = p.readUint16(); err != nil {
		return nil, err
	}

	dataLength := int(length) - 4 // length includes itself (2) and Rcom (2)
	com.Data = make([]byte, dataLength)
	if err := p.read(com.Data); err != nil {
		return nil, err
	}

	return com, nil
}

func (p *Parser) parseMCT() (*MCTSegment, error) {
	length, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	payloadLen := int(length) - 2
	if payloadLen < 6 {
		return nil, fmt.Errorf("invalid MCT length")
	}
	zmct, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	if zmct != 0 {
		return nil, fmt.Errorf("unsupported Zmct=%d", zmct)
	}
	imct, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	ymct, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	if ymct != 0 {
		return nil, fmt.Errorf("unsupported Ymct=%d", ymct)
	}
	idx := uint8(imct & 0xFF)
	at := uint8((imct >> 8) & 0x3)
	et := uint8((imct >> 10) & 0x3)
	dataLen := payloadLen - 6
	buf := make([]byte, dataLen)
	if err := p.read(buf); err != nil {
		return nil, err
	}
	return &MCTSegment{Index: idx, ElementType: MCTElementType(et), ArrayType: MCTArrayType(at), Data: buf}, nil
}

func (p *Parser) parseMCC() (*MCCSegment, error) {
	length, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	payloadLen := int(length) - 2
	if payloadLen < 7 {
		return nil, fmt.Errorf("invalid MCC length")
	}
	zmcc, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	if zmcc != 0 {
		return nil, fmt.Errorf("unsupported Zmcc=%d", zmcc)
	}
	idx, err := p.readUint8()
	if err != nil {
		return nil, err
	}
	ymcc, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	if ymcc != 0 {
		return nil, fmt.Errorf("unsupported Ymcc=%d", ymcc)
	}
	qmcc, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	if qmcc == 0 {
		return nil, fmt.Errorf("invalid MCC collections")
	}

	collectionType, err := p.readUint8()
	if err != nil {
		return nil, err
	}
	nmcci, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	compBytes := 1
	if (nmcci & 0x8000) != 0 {
		compBytes = 2
	}
	numComps := nmcci & 0x7FFF
	comps := make([]uint16, numComps)
	for i := 0; i < int(numComps); i++ {
		var v uint16
		if compBytes == 1 {
			b, err := p.readUint8()
			if err != nil {
				return nil, err
			}
			v = uint16(b)
		} else {
			v, err = p.readUint16()
			if err != nil {
				return nil, err
			}
		}
		comps[i] = v
	}

	mmcci, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	outCompBytes := 1
	if (mmcci & 0x8000) != 0 {
		outCompBytes = 2
	}
	outCompsCount := mmcci & 0x7FFF
	outComps := make([]uint16, outCompsCount)
	for i := 0; i < int(outCompsCount); i++ {
		var v uint16
		if outCompBytes == 1 {
			b, err := p.readUint8()
			if err != nil {
				return nil, err
			}
			v = uint16(b)
		} else {
			v, err = p.readUint16()
			if err != nil {
				return nil, err
			}
		}
		outComps[i] = v
	}

	b0, err := p.readUint8()
	if err != nil {
		return nil, err
	}
	b1, err := p.readUint8()
	if err != nil {
		return nil, err
	}
	b2, err := p.readUint8()
	if err != nil {
		return nil, err
	}
	tmcc := (uint32(b0) << 16) | (uint32(b1) << 8) | uint32(b2)
	reversible := ((tmcc >> 16) & 0x1) != 0
	decoIdx := uint8(tmcc & 0xFF)
	offIdx := uint8((tmcc >> 8) & 0xFF)

	consumed := 2 + 1 + 2 + 2 + 1 + 2 + (compBytes * int(numComps)) + 2 + (outCompBytes * int(outCompsCount)) + 3
	remain := payloadLen - consumed
	if remain > 0 {
		buf := make([]byte, remain)
		if err := p.read(buf); err != nil {
			return nil, err
		}
	}

	return &MCCSegment{
		Index:              idx,
		CollectionType:     collectionType,
		NumComponents:      numComps,
		ComponentIDs:       comps,
		OutputComponentIDs: outComps,
		DecorrelateIndex:   decoIdx,
		OffsetIndex:        offIdx,
		Reversible:         reversible,
	}, nil
}

func (p *Parser) parseMCO() (*MCOSegment, error) {
	length, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	payloadLen := int(length) - 2
	if payloadLen < 1 {
		return nil, fmt.Errorf("invalid MCO length")
	}
	numStages, err := p.readUint8()
	if err != nil {
		return nil, err
	}
	stageCount := int(numStages)
	stages := make([]uint8, 0, stageCount)
	for i := 0; i < stageCount; i++ {
		v, err := p.readUint8()
		if err != nil {
			return nil, err
		}
		stages = append(stages, v)
	}
	remain := payloadLen - (1 + stageCount)
	if remain > 0 {
		buf := make([]byte, remain)
		if err := p.read(buf); err != nil {
			return nil, err
		}
	}
	return &MCOSegment{NumStages: numStages, StageIndices: stages}, nil
}

// parseSOT parses the SOT marker segment
func (p *Parser) parseSOT() (*SOTSegment, error) {
	length, err := p.readUint16()
	if err != nil {
		return nil, err
	}

	if length != 10 {
		return nil, fmt.Errorf("invalid SOT segment length: %d", length)
	}

	sot := &SOTSegment{}

	if sot.Isot, err = p.readUint16(); err != nil {
		return nil, err
	}
	if sot.Psot, err = p.readUint32(); err != nil {
		return nil, err
	}
	if sot.TPsot, err = p.readUint8(); err != nil {
		return nil, err
	}
	if sot.TNsot, err = p.readUint8(); err != nil {
		return nil, err
	}

	return sot, nil
}

// Helper methods for reading data

func (p *Parser) readMarker() (uint16, error) {
	return p.readUint16()
}

func (p *Parser) peekMarker() (uint16, error) {
	if p.offset+2 > len(p.data) {
		return 0, io.EOF
	}
	marker := binary.BigEndian.Uint16(p.data[p.offset : p.offset+2])
	return marker, nil
}

func (p *Parser) skipSegment() error {
	length, err := p.readUint16()
	if err != nil {
		return err
	}
	// length includes the 2 bytes for length itself
	skip := int(length) - 2
	if p.offset+skip > len(p.data) {
		return io.EOF
	}
	p.offset += skip
	return nil
}

func (p *Parser) readTileData() []byte {
	start := p.offset

	// Read until we hit a marker (0xFF followed by non-0xFF)
	for p.offset < len(p.data) {
		if p.data[p.offset] == 0xFF && p.offset+1 < len(p.data) {
			next := p.data[p.offset+1]
			// Check if this is a valid marker (not 0xFF00)
			if next != 0x00 && next >= 0x4F {
				// This is a marker, stop here
				break
			}
		}
		p.offset++
	}

	return p.data[start:p.offset]
}

func (p *Parser) readTileDataWithLength(tileStart int, psot uint32) []byte {
	if psot == 0 {
		return p.readTileData()
	}
	consumed := p.offset - tileStart
	if int(psot) < consumed {
		return p.readTileData()
	}
	remaining := int(psot) - consumed
	if p.offset+remaining > len(p.data) {
		return p.readTileData()
	}
	start := p.offset
	p.offset += remaining
	return p.data[start:p.offset]
}

func componentIndexSize(siz *SIZSegment) int {
	if siz != nil && siz.Csiz > 256 {
		return 2
	}
	return 1
}

func (p *Parser) readComponentIndex(siz *SIZSegment) (uint16, error) {
	if componentIndexSize(siz) == 2 {
		val, err := p.readUint16()
		return val, err
	}
	val, err := p.readUint8()
	return uint16(val), err
}

func cocEqual(a, b *COCSegment) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Component != b.Component || a.Scoc != b.Scoc ||
		a.NumberOfDecompositionLevels != b.NumberOfDecompositionLevels ||
		a.CodeBlockWidth != b.CodeBlockWidth ||
		a.CodeBlockHeight != b.CodeBlockHeight ||
		a.CodeBlockStyle != b.CodeBlockStyle ||
		a.Transformation != b.Transformation {
		return false
	}
	if len(a.PrecinctSizes) != len(b.PrecinctSizes) {
		return false
	}
	for i := range a.PrecinctSizes {
		if a.PrecinctSizes[i] != b.PrecinctSizes[i] {
			return false
		}
	}
	return true
}

func qccEqual(a, b *QCCSegment) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Component != b.Component || a.Sqcc != b.Sqcc {
		return false
	}
	if len(a.SPqcc) != len(b.SPqcc) {
		return false
	}
	for i := range a.SPqcc {
		if a.SPqcc[i] != b.SPqcc[i] {
			return false
		}
	}
	return true
}

func codEqual(a, b *CODSegment) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Scod != b.Scod ||
		a.ProgressionOrder != b.ProgressionOrder ||
		a.NumberOfLayers != b.NumberOfLayers ||
		a.MultipleComponentTransform != b.MultipleComponentTransform ||
		a.NumberOfDecompositionLevels != b.NumberOfDecompositionLevels ||
		a.CodeBlockWidth != b.CodeBlockWidth ||
		a.CodeBlockHeight != b.CodeBlockHeight ||
		a.CodeBlockStyle != b.CodeBlockStyle ||
		a.Transformation != b.Transformation {
		return false
	}
	if len(a.PrecinctSizes) != len(b.PrecinctSizes) {
		return false
	}
	for i := range a.PrecinctSizes {
		if a.PrecinctSizes[i] != b.PrecinctSizes[i] {
			return false
		}
	}
	return true
}

func qcdEqual(a, b *QCDSegment) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Sqcd != b.Sqcd {
		return false
	}
	if len(a.SPqcd) != len(b.SPqcd) {
		return false
	}
	for i := range a.SPqcd {
		if a.SPqcd[i] != b.SPqcd[i] {
			return false
		}
	}
	return true
}

func rgnEqual(a, b []*RGNSegment) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] == nil || b[i] == nil {
			if a[i] != b[i] {
				return false
			}
			continue
		}
		if a[i].Crgn != b[i].Crgn || a[i].Srgn != b[i].Srgn || a[i].SPrgn != b[i].SPrgn {
			return false
		}
	}
	return true
}

func pocEqual(a, b []POCSegment) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i].Entries) != len(b[i].Entries) {
			return false
		}
		for j := range a[i].Entries {
			if a[i].Entries[j] != b[i].Entries[j] {
				return false
			}
		}
	}
	return true
}
