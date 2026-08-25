package cleanup

// ExponentPredictor computes exponent predictors Kq and exponent bounds Uq
// for HTJ2K quads as specified in ISO/IEC 15444-15:2019 Clause 7.3.7.
//
// The exponent predictor Kq is combined with the unsigned residual uq to
// produce the exponent bound Uq:
//
//   Uq = Kq + uq
//
// Where Uq bounds all sample magnitude exponents in quad q:
//   En <= Uq for all n in {4q, 4q+1, 4q+2, 4q+3}

// ExponentPredictorComputer maintains state for computing exponent predictors.
type ExponentPredictorComputer struct {
	width     int      // Width in quads
	height    int      // Height in quads
	exponents [][]int  // Maximum magnitude exponent for each quad
	gamma     [][]bool // Gamma[qx][qy] indicates if quad has > 1 significant sample
}

// NewExponentPredictorComputer creates a new exponent predictor computer.
func NewExponentPredictorComputer(widthInQuads, heightInQuads int) *ExponentPredictorComputer {
	exponents := make([][]int, heightInQuads)
	gamma := make([][]bool, heightInQuads)
	for i := range exponents {
		exponents[i] = make([]int, widthInQuads)
		gamma[i] = make([]bool, widthInQuads)
	}

	return &ExponentPredictorComputer{
		width:     widthInQuads,
		height:    heightInQuads,
		exponents: exponents,
		gamma:     gamma,
	}
}

// SetQuadExponents sets the maximum magnitude exponent for a quad
// and determines gamma (whether quad has more than one significant sample).
//
// Parameters:
//   - qx, qy: Quad coordinates
//   - maxExponent: Maximum magnitude exponent across all 4 samples in quad
//   - significantCount: Number of significant samples in quad (0-4)
func (e *ExponentPredictorComputer) SetQuadExponents(qx, qy int, maxExponent int, significantCount int) {
	if qx >= 0 && qx < e.width && qy >= 0 && qy < e.height {
		e.exponents[qy][qx] = maxExponent
		// Gamma is 1 if quad has more than one significant sample.
		// Formula (6): gamma_q = 1 if |{n in quad q : sigma_n = 1}| > 1
		e.gamma[qy][qx] = significantCount > 1
	}
}

// ComputePredictor computes the exponent predictor Kq for a quad.
// OpenJPH uses the top row's maximum exponent only (no left neighbor).
//
// For first row of quads (qy = 0):
//
//	Kq = 1
//
// For other quads (qy > 0):
//   - If gamma_q = 0 (<= 1 significant sample), Kq = 1
//   - If gamma_q = 1 (> 1 significant sample), Kq = max(1, E_top-1)
//
// Where:
//   - E_top is the maximum exponent of the quad directly above
//   - gamma_q is 1 if current quad has > 1 significant sample, 0 otherwise
//
// Returns:
//   - Kq: The exponent predictor value (>= 1)
func (e *ExponentPredictorComputer) ComputePredictor(qx, qy int) int {
	// For first row (y=0), Kq = 1
	if qy == 0 {
		return 1
	}

	if !e.gamma[qy][qx] {
		return 1
	}

	eTop := 0
	if qy > 0 {
		eTop = e.exponents[qy-1][qx]
	}

	Kq := eTop - 1
	if Kq < 1 {
		Kq = 1
	}
	return Kq
}

// ComputeExponentBound computes the exponent bound Uq for a quad.
// Formula: Uq = Kq + uq
//
// Parameters:
//   - qx, qy: Quad coordinates
//   - uq: Unsigned residual value decoded from U-VLC
//
// Returns:
//   - Uq: The exponent bound for this quad
func (e *ExponentPredictorComputer) ComputeExponentBound(qx, qy int, uq uint32) int {
	Kq := e.ComputePredictor(qx, qy)
	Uq := Kq + int(uq)
	return Uq
}

// MagnitudeExponent computes the magnitude exponent En from magnitude mu_n.
// Formula (F.1): En = floor(log2(mu_n)) + 1 for mu_n > 0, En = 0 for mu_n = 0.
//
// This is computed by counting the number of bits needed to represent mu_n,
// which is equivalent to floor(log2(mu_n)) + 1 for mu_n > 0.
//
// Parameters:
//   - magnitude: The sample magnitude mu_n (unsigned)
//
// Returns:
//   - En: The magnitude exponent (0 if magnitude is 0)
func MagnitudeExponent(magnitude uint32) int {
	if magnitude == 0 {
		return 0
	}

	// Count the number of bits needed to represent magnitude.
	exponent := 0
	temp := magnitude
	for temp > 0 {
		exponent++
		temp >>= 1
	}

	return exponent
}

// QuadMaxExponent computes the maximum magnitude exponent for a quad.
// Given 4 sample magnitudes, returns max(E0, E1, E2, E3).
//
// Parameters:
//   - mag0, mag1, mag2, mag3: Magnitudes of the 4 samples in the quad
//
// Returns:
//   - maxE: Maximum exponent across all 4 samples
//   - sigCount: Number of significant samples (magnitude > 0)
func QuadMaxExponent(mag0, mag1, mag2, mag3 uint32) (maxE int, sigCount int) {
	exponents := [4]int{
		MagnitudeExponent(mag0),
		MagnitudeExponent(mag1),
		MagnitudeExponent(mag2),
		MagnitudeExponent(mag3),
	}

	maxE = 0
	sigCount = 0

	for _, e := range exponents {
		if e > maxE {
			maxE = e
		}
		if e > 0 {
			sigCount++
		}
	}

	return maxE, sigCount
}
