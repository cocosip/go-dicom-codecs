package cleanup

// Context computation for HTJ2K VLC decoding.
// The context follows OpenJPH's implementation of ITU-T T.814 equations.
// Reference: ojph_block_decoder32.cpp lines 870-1070

// ContextComputer computes decoding contexts for HTJ2K VLC operations.
// It tracks quad significance (rho) and auxiliary sigma buffer for neighbor rules.
type ContextComputer struct {
	width  int
	height int
	numQX  int
	numQY  int

	// rho[qy][qx] stores the 4-bit significance pattern for each quad.
	rho [][]uint8

	// sigma stores the VLC decode result (lower byte) for context computation
	// This matches OpenJPH's scratch buffer which stores decoded VLC entries
	sigma []uint16
}

// NewContextComputer creates a new context computer for a block.
func NewContextComputer(width, height int) *ContextComputer {
	numQX := (width + 1) / 2
	numQY := (height + 1) / 2

	rho := make([][]uint8, numQY)
	for i := range rho {
		rho[i] = make([]uint8, numQX)
	}

	return &ContextComputer{
		width:  width,
		height: height,
		numQX:  numQX,
		numQY:  numQY,
		rho:    rho,
		sigma:  make([]uint16, numQX*numQY),
	}
}

// SetSignificant marks a sample as significant or not.
func (c *ContextComputer) SetSignificant(x, y int, significant bool) {
	if x < 0 || x >= c.width || y < 0 || y >= c.height {
		return
	}
	qx := x / 2
	qy := y / 2
	bit := uint8(1 << ((x%2)*2 + (y % 2)))
	if significant {
		c.rho[qy][qx] |= bit
		return
	}
	c.rho[qy][qx] &^= bit
}

// IsSignificant checks if a sample is significant.
func (c *ContextComputer) IsSignificant(x, y int) bool {
	if x < 0 || x >= c.width || y < 0 || y >= c.height {
		return false
	}
	qx := x / 2
	qy := y / 2
	bit := uint8(1 << ((x%2)*2 + (y % 2)))
	return (c.rho[qy][qx] & bit) != 0
}

// ComputeContext computes the VLC context for a quad at (qx, qy).
// Context is a 3-bit value in [0..7].
func (c *ContextComputer) ComputeContext(qx, qy int, isFirstRow bool) uint8 {
	if qx < 0 || qx >= c.numQX || qy < 0 || qy >= c.numQY {
		return 0
	}
	if isFirstRow || qy == 0 {
		if qx == 0 {
			return 0
		}
		left := c.rho[qy][qx-1]
		return ((left >> 1) & 0x7) | (left & 0x1)
	}

	var left uint8
	var aboveLeft uint8
	var above uint8
	var aboveRight uint8

	if qx > 0 {
		left = c.rho[qy][qx-1]
		aboveLeft = c.rho[qy-1][qx-1]
	}
	above = c.rho[qy-1][qx]
	if qx+1 < c.numQX {
		aboveRight = c.rho[qy-1][qx+1]
	}

	c0 := ((aboveLeft >> 3) & 0x1) | ((above >> 1) & 0x1)
	c1 := ((left >> 2) & 0x1) | ((left >> 3) & 0x1)
	c2 := ((above >> 3) & 0x1) | ((aboveRight >> 1) & 0x1)

	return c0 | (c1 << 1) | (c2 << 2)
}

// ComputeContextDetailed is kept for API compatibility.
func (c *ContextComputer) ComputeContextDetailed(qx, qy int, isFirstRow bool) uint8 {
	return c.ComputeContext(qx, qy, isFirstRow)
}

// UpdateQuadSignificance stores the rho pattern for a quad.
func (c *ContextComputer) UpdateQuadSignificance(qx, qy int, rho uint8) {
	if qx < 0 || qx >= c.numQX || qy < 0 || qy >= c.numQY {
		return
	}
	c.rho[qy][qx] = rho & 0xF
}

// SetQuadVLC stores the VLC decode result for a quad (for OpenJPH-style context)
// This is needed for accurate context computation in subsequent quads
func (c *ContextComputer) SetQuadVLC(qx, qy int, vlcEntry uint16) {
	if qx >= 0 && qx < c.numQX && qy >= 0 && qy < c.numQY {
		c.sigma[qy*c.numQX+qx] = vlcEntry
	}
}

// GetQuadVLC retrieves the stored VLC result for a quad
func (c *ContextComputer) GetQuadVLC(qx, qy int) uint16 {
	if qx >= 0 && qx < c.numQX && qy >= 0 && qy < c.numQY {
		return c.sigma[qy*c.numQX+qx]
	}
	return 0
}

// ComputeInitialRowContext computes context for initial row (y=0) using equation 1 from ITU-T.814
// Reference: OpenJPH ojph_block_decoder32.cpp line 906, 937
// Formula: c_q = ((t0 & 0x10) << 3) | ((t0 & 0xE0) << 2)
// where t0 is the VLC decode result from the previous quad
func (c *ContextComputer) ComputeInitialRowContext(qx int, prevVLC uint16) uint8 {
	if qx == 0 {
		return 0
	}
	// Extract context from previous quad's VLC result
	// bit 4 (u_off) -> bit 7, bits 5-7 (rho high) -> bits 7-9
	context := ((prevVLC & 0x10) << 3) | ((prevVLC & 0xE0) << 2)
	return uint8((context >> 7) & 0x07)
}

// ComputeSubsequentRowContext computes context for non-initial rows using equation 2 from ITU-T.814
// Reference: OpenJPH ojph_block_decoder32.cpp lines 984-1030, 1062-1064
// For first quad in row (qx=0):
//
//	c_q = sigma_q(n, ne, nf)
//
// For subsequent quads (qx>0):
//
//	c_q = sigma_q(w, sw, nw, n, ne, nf)
func (c *ContextComputer) ComputeSubsequentRowContext(qx, qy int, prevVLC uint16) uint8 {
	if qy == 0 {
		return c.ComputeInitialRowContext(qx, prevVLC)
	}

	context := uint16(0)
	sstr := c.numQX // stride in sigma array (quads per row)

	if qx == 0 {
		// First quad in row: use north row neighbors only
		// Reference: lines 993-994
		idx := qy*sstr + qx
		// sigma_q (n, ne)
		if idx >= sstr {
			nSigma := c.sigma[idx-sstr]     // north
			neSigma := c.sigma[idx-sstr+1]  // northeast
			context |= (nSigma & 0xA0) << 2 // bits 5,7 from north
			if qx+1 < c.numQX {
				context |= (neSigma & 0x20) << 4 // bit 5 from northeast
			}
		}
	} else {
		// Subsequent quads: use west and north neighbors
		// Reference: lines 1025-1030
		// sigma_q (w, sw) from previous quad result
		context = ((prevVLC & 0x40) << 2) | ((prevVLC & 0x80) << 1)

		idx := qy*sstr + qx
		// sigma_q (nw) - northwest
		if idx >= sstr && qx > 0 {
			nwSigma := c.sigma[idx-sstr-1]
			context |= nwSigma & 0x80 // bit 7 from northwest
		}

		// sigma_q (n, ne, nf) from north row
		if idx >= sstr {
			nSigma := c.sigma[idx-sstr]     // north
			context |= (nSigma & 0xA0) << 2 // bits 5,7 from north

			// Far northeast (nf) is at position qx+2
			if qx+2 < c.numQX {
				nfSigma := c.sigma[idx-sstr+2]
				context |= (nfSigma & 0x20) << 4 // bit 5 from far northeast
			}
		}
	}

	return uint8((context >> 7) & 0x07)
}
