package cleanup

// QuadPairDecoder implements quad-pair interleaved VLC decoding
// as specified in ISO/IEC 15444-15:2019 Clause 7.3.4
//
// Quad-pair interleaving defines the order in which CxtVLC codewords
// and U-VLC components are decoded from the VLC bit-stream.
//
// A quad-pair consists of two horizontally adjacent quads:
//   - First quad:  q1 = 2g
//   - Second quad: q2 = 2g+1
//
// Special cases:
// 1. Initial line-pair (q < QW): First row of quads in the block
// 2. When both quads in initial line-pair have ulf=1: Use Formula (4)
// 3. When first quad has uq1>2: Second quad uses simplified decoding

// QuadPairResult contains the decoded information for a quad-pair
type QuadPairResult struct {
	// First quad (q1 = 2g)
	Rho1  uint8  // Significance pattern (4 bits)
	ULF1  uint8  // Unsigned residual offset flag (0 or 1)
	Uq1   uint32 // Unsigned residual value (if ulf1=1)
	E1_1  uint8  // EMB pattern e1
	EMax1 uint8  // EMB pattern emax

	// Second quad (q2 = 2g+1)
	Rho2  uint8  // Significance pattern (4 bits)
	ULF2  uint8  // Unsigned residual offset flag (0 or 1)
	Uq2   uint32 // Unsigned residual value (if ulf2=1)
	E1_2  uint8  // EMB pattern e1
	EMax2 uint8  // EMB pattern emax

	// Metadata
	IsInitialLinePair bool // True if q1 < QW (first row)
	HasSecondQuad     bool // False if QW is odd and this is the last pair
}

// vlcQuadDecoder provides VLC decoding plus bit-level access for UVLC.
type vlcQuadDecoder interface {
	DecodeQuadWithContext(context uint8, isFirstRow bool) (uint8, uint8, uint8, uint8, bool)
	ReadBit() (uint8, error)
	ReadBitsLE(n int) (uint32, error)
}

// QuadPairDecoder decodes quad-pairs from the VLC bit-stream
type QuadPairDecoder struct {
	vlcDecoder  vlcQuadDecoder
	uvlcDecoder *UVLCDecoder
	context     *ContextComputer
	mel         *MELDecoderSpec
	QW          int // Width in quads
}

// NewQuadPairDecoder creates a new quad-pair decoder using forward VLC decoding.
func NewQuadPairDecoder(vlcData []byte, widthInQuads, heightInQuads int) *QuadPairDecoder {
	return NewQuadPairDecoderWithVLC(NewVLCDecoder(vlcData), widthInQuads, heightInQuads)
}

// NewQuadPairDecoderWithVLC creates a quad-pair decoder with a custom VLC decoder.
func NewQuadPairDecoderWithVLC(vlc vlcQuadDecoder, widthInQuads, heightInQuads int) *QuadPairDecoder {
	return &QuadPairDecoder{
		vlcDecoder:  vlc,
		uvlcDecoder: NewUVLCDecoder(vlc),
		context:     NewContextComputer(widthInQuads*2, heightInQuads*2), // Convert to samples
		QW:          widthInQuads,
	}
}

// SetMELDecoder attaches a MEL decoder for context-zero events.
func (d *QuadPairDecoder) SetMELDecoder(mel *MELDecoderSpec) {
	d.mel = mel
}

func (d *QuadPairDecoder) decodeQuadWithContext(context uint8, isInitialLinePair bool) (uint8, uint8, uint8, uint8, error) {
	if context == 0 && d.mel != nil {
		melBit, ok := d.mel.DecodeMELSym()
		if !ok {
			return 0, 0, 0, 0, ErrInsufficientData
		}
		if melBit == 0 {
			return 0, 0, 0, 0, nil
		}
	}

	rho, uOff, eK, e1, found := d.vlcDecoder.DecodeQuadWithContext(context, isInitialLinePair)
	if !found {
		return 0, 0, 0, 0, ErrInsufficientData
	}

	return rho, uOff, eK, e1, nil
}

// DecodeQuadPair decodes a single quad-pair according to Clause 7.3.4
//
// Parameters:
//   - g: Quad-pair index (q1=2g, q2=2g+1)
//   - qy: Quad row (for determining initial line-pair)
//
// Returns:
//   - QuadPairResult containing decoded information for both quads
//   - error if decoding fails
func (d *QuadPairDecoder) DecodeQuadPair(g int, qy int) (*QuadPairResult, error) {
	result := &QuadPairResult{
		IsInitialLinePair: qy == 0, // Initial line-pair is first row
		HasSecondQuad:     (2*g + 1) < d.QW,
	}

	q1 := 2 * g      // First quad index
	q2 := 2*g + 1    // Second quad index
	qx1 := q1 % d.QW // First quad x position
	qx2 := q2 % d.QW // Second quad x position

	// Step 1: Decode first quad's CxtVLC codeword
	ctx1 := d.context.ComputeContext(qx1, qy, result.IsInitialLinePair)
	rho1, uOff1, eK1, e1_1, err := d.decodeQuadWithContext(ctx1, result.IsInitialLinePair)
	if err != nil {
		return nil, err
	}

	result.Rho1 = rho1
	result.ULF1 = uOff1
	result.E1_1 = e1_1
	result.EMax1 = eK1

	// Update significance map for first quad
	d.context.UpdateQuadSignificance(qx1, qy, rho1)

	// Step 2: If no second quad (odd width), decode U-VLC for the first quad only.
	if !result.HasSecondQuad {
		uq1, _, err := d.uvlcDecoder.DecodePair(result.ULF1, 0, result.IsInitialLinePair, 0)
		if err != nil {
			return nil, err
		}
		result.Uq1 = uq1
		return result, nil
	}

	// Step 3: Decode second quad's CxtVLC codeword
	ctx2 := d.context.ComputeContext(qx2, qy, result.IsInitialLinePair)
	rho2, uOff2, eK2, e1_2, err := d.decodeQuadWithContext(ctx2, result.IsInitialLinePair)
	if err != nil {
		return nil, err
	}

	result.Rho2 = rho2
	result.ULF2 = uOff2
	result.E1_2 = e1_2
	result.EMax2 = eK2

	// Update significance map for second quad
	d.context.UpdateQuadSignificance(qx2, qy, rho2)

	// Step 4: Decode U-VLC for the quad pair using OpenJPH table rules.
	melEvent := 0
	if result.IsInitialLinePair && result.ULF1 == 1 && result.ULF2 == 1 && d.mel != nil {
		bit, ok := d.mel.DecodeMELSym()
		if !ok {
			return nil, ErrInsufficientData
		}
		melEvent = bit
	}

	uq1, uq2, err := d.uvlcDecoder.DecodePair(result.ULF1, result.ULF2, result.IsInitialLinePair, melEvent)
	if err != nil {
		return nil, err
	}
	result.Uq1 = uq1
	result.Uq2 = uq2

	return result, nil
}

// DecodeAllQuadPairs decodes all quad-pairs in a block
//
// Parameters:
//   - heightInQuads: Number of quad rows
//
// Returns:
//   - Slice of QuadPairResult for all quad-pairs in scan order
//   - error if decoding fails
func (d *QuadPairDecoder) DecodeAllQuadPairs(heightInQuads int) ([]*QuadPairResult, error) {
	// Calculate total number of quad-pairs
	// Each row has ceil(QW/2) quad-pairs
	pairsPerRow := (d.QW + 1) / 2
	totalPairs := pairsPerRow * heightInQuads

	results := make([]*QuadPairResult, 0, totalPairs)

	// Scan by rows (quad-pairs are processed row by row)
	for qy := 0; qy < heightInQuads; qy++ {
		for g := 0; g < pairsPerRow; g++ {
			pair, err := d.DecodeQuadPair(g, qy)
			if err != nil {
				return nil, err
			}
			results = append(results, pair)
		}
	}

	return results, nil
}

// GetQuadInfo extracts individual quad information from quad-pair results
//
// This helper function converts quad-pair results into per-quad information
// for easier integration with existing block decoder logic
func GetQuadInfo(pair *QuadPairResult, quadIndex int) (rho uint8, ulf uint8, uq uint32, e1 uint8, emax uint8) {
	if quadIndex == 0 {
		// First quad (q1)
		return pair.Rho1, pair.ULF1, pair.Uq1, pair.E1_1, pair.EMax1
	}
	// Second quad (q2)
	return pair.Rho2, pair.ULF2, pair.Uq2, pair.E1_2, pair.EMax2
}
