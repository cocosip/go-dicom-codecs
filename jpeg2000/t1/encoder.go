package t1

import (
	"fmt"
	"math"

	"github.com/cocosip/go-dicom-codec/jpeg2000/mqc"
)

// Encoder implements EBCOT Tier-1 encoding
// Reference: ISO/IEC 15444-1:2019 Annex D
type Encoder struct {
	// Code-block dimensions
	width  int
	height int

	// Wavelet coefficients (input)
	// Stored in row-major order
	data []int32

	// State flags for each coefficient
	// Stores significance, refinement, visit flags and neighbor info
	flags []uint32

	// MQ encoder
	mqe *mqc.MQEncoder

	// Current bit-plane being encoded
	bitplane int

	// Subband orientation (0=LL, 1=HL, 2=LH, 3=HH)
	orientation int

	// Encoding parameters
	roishift         int  // ROI shift value
	cblkstyle        int  // Code-block style flags
	resetctx         bool // Reset context on each pass
	termall          bool // Terminate all passes
	segmentation     bool // Use segmentation symbols
	nmseDecFracBits  int  // Number of T1_NMSEDEC_FRACBITS already present in data
	distortionWeight float64
}

func isLazyRawPass(bitplane int, maxBitplane int, passType int, cblkstyle int) bool {
	if (cblkstyle & CblkStyleLazy) == 0 {
		return false
	}
	if passType >= 2 {
		return false
	}
	return bitplane < (maxBitplane - 3)
}

func isTerminatingPass(bitplane int, maxBitplane int, passType int, cblkstyle int) bool {
	if passType == 2 && bitplane == 0 {
		return true
	}
	if (cblkstyle & CblkStyleTermAll) != 0 {
		return true
	}
	if (cblkstyle & CblkStyleLazy) != 0 {
		if bitplane == (maxBitplane-3) && passType == 2 {
			return true
		}
		if bitplane < (maxBitplane-3) && passType > 0 {
			return true
		}
	}
	return false
}

// NewT1Encoder creates a new Tier-1 encoder
func NewT1Encoder(width, height int, cblkstyle int) *Encoder {
	// Add padding for boundary conditions (1 pixel on each side)
	paddedWidth := width + 2
	paddedHeight := height + 2

	t1 := &Encoder{
		width:            width,
		height:           height,
		flags:            make([]uint32, paddedWidth*paddedHeight),
		distortionWeight: 1,
	}

	// Parse code-block style flags
	// Reference: ISO/IEC 15444-1 Table A.18
	t1.cblkstyle = cblkstyle
	t1.resetctx = (cblkstyle & CblkStyleReset) != 0
	t1.termall = (cblkstyle & CblkStyleTermAll) != 0
	t1.segmentation = (cblkstyle & CblkStyleSegsym) != 0

	return t1
}

// SetOrientation sets the subband orientation for zero coding context lookup.
func (t1 *Encoder) SetOrientation(orient int) {
	t1.orientation = orient
}

// SetNMSEDecFractionalBits records how many OpenJPEG fractional NMSE bits are
// already present in the coefficient data supplied to the encoder.
func (t1 *Encoder) SetNMSEDecFractionalBits(bits int) {
	if bits < 0 {
		bits = 0
	}
	if bits > t1NMSEDecFracBits {
		bits = t1NMSEDecFracBits
	}
	t1.nmseDecFracBits = bits
}

// SetDistortionWeight applies OpenJPEG's weighted MSE scale to per-pass
// nmsedec values before they are used by PCRD layer allocation.
func (t1 *Encoder) SetDistortionWeight(weight float64) {
	if weight <= 0 {
		weight = 1
	}
	t1.distortionWeight = weight
}

// Encode encodes a code-block
//
// Performance notes:
// - Most computationally intensive part of JPEG 2000 encoding
// - Processes coefficients bit-plane by bit-plane (MSB to LSB)
// - First bit-plane starts with Cleanup, then SPP/MRP/CP for remaining bit-planes
// - Context modeling using 8-neighbor flags (cached for speed)
// - MQ encoding is the inner loop bottleneck
// - Typical workload: 32x32 block = 1024 coefficients × 12-16 bit-planes
func (t1 *Encoder) Encode(data []int32, numPasses int, roishift int) ([]byte, error) {
	if len(data) != t1.width*t1.height {
		return nil, fmt.Errorf("data size mismatch: expected %d, got %d",
			t1.width*t1.height, len(data))
	}

	t1.roishift = roishift

	// Copy data with padding
	t1.data = make([]int32, (t1.width+2)*(t1.height+2))
	paddedWidth := t1.width + 2
	for y := 0; y < t1.height; y++ {
		for x := 0; x < t1.width; x++ {
			idx := (y+1)*paddedWidth + (x + 1)
			t1.data[idx] = data[y*t1.width+x]
		}
	}

	// Determine maximum bit-plane
	maxBitplane := t1.findMaxBitplane()

	if maxBitplane < 0 {
		// All coefficients are zero
		t1.mqe = mqc.NewMQEncoder(NUMCONTEXTS)
		result := t1.mqe.Flush()
		return result, nil
	}

	// Initialize MQ encoder with OpenJPEG default context states
	t1.mqe = mqc.NewMQEncoder(NUMCONTEXTS)
	t1.mqe.SetContextState(CTXUNI, 46)
	t1.mqe.SetContextState(CTXRL, 3)
	t1.mqe.SetContextState(CTXZCSTART, 4)

	// Encode passes using OpenJPEG sequencing:
	// - First pass is Cleanup on the highest bit-plane.
	// - Subsequent bit-planes use SPP, MRP, CP.
	passIdx := 0
	passType := 2
	prevTerminated := false
	for t1.bitplane = maxBitplane; t1.bitplane >= 0 && passIdx < numPasses; {
		startBitplane := passType == 0 || (passType == 2 && passIdx == 0)
		if startBitplane {
			// Clear VISIT flags at start of each bitplane.
			paddedWidth := t1.width + 2
			paddedHeight := t1.height + 2
			for i := 0; i < paddedWidth*paddedHeight; i++ {
				t1.flags[i] &^= T1Visit
			}

			// Check if this bit-plane needs encoding
			if t1.roishift > 0 && t1.bitplane >= t1.roishift {
				passType = 0
				t1.bitplane--
				continue
			}
		}

		raw := isLazyRawPass(t1.bitplane, maxBitplane, passType, t1.cblkstyle)
		if prevTerminated {
			if raw {
				t1.mqe.BypassInitEnc()
			} else {
				t1.mqe.RestartInitEnc()
			}
			prevTerminated = false
		}

		switch passType {
		case 0:
			_ = t1.encodeSigPropPass(raw)
		case 1:
			_ = t1.encodeMagRefPass(raw)
		case 2:
			_ = t1.encodeCleanupPass()
			if t1.segmentation {
				t1.mqe.SegmarkEnc()
			}
		}

		terminated := isTerminatingPass(t1.bitplane, maxBitplane, passType, t1.cblkstyle)
		if terminated {
			if raw {
				t1.mqe.BypassFlushEnc((t1.cblkstyle & CblkStylePterm) != 0)
			} else if (t1.cblkstyle & CblkStylePterm) != 0 {
				t1.mqe.ErtermEnc()
			} else {
				t1.mqe.FlushToOutput()
			}
			prevTerminated = true
		}

		if t1.resetctx {
			t1.mqe.ResetContexts()
			t1.mqe.SetContextState(CTXUNI, 46)
			t1.mqe.SetContextState(CTXRL, 3)
			t1.mqe.SetContextState(CTXZCSTART, 4)
		}

		passIdx++
		if passType == 2 {
			passType = 0
			t1.bitplane--
		} else {
			passType++
		}
	}

	// Flush MQ encoder if last pass is not terminated
	var result []byte
	if prevTerminated {
		result = t1.mqe.GetBuffer()
	} else {
		result = t1.mqe.Flush()
	}

	return result, nil
}

// findMaxBitplane finds the maximum bit-plane that contains significant bits
func (t1 *Encoder) findMaxBitplane() int {
	maxAbs := int32(0)

	// Find maximum absolute value
	for _, val := range t1.data {
		if val < 0 {
			if -val > maxAbs {
				maxAbs = -val
			}
		} else {
			if val > maxAbs {
				maxAbs = val
			}
		}
	}

	if maxAbs == 0 {
		return -1 // All zeros
	}

	// Find the highest bit set
	bitplane := 0
	for maxAbs > 0 {
		maxAbs >>= 1
		bitplane++
	}
	return bitplane - 1
}

// encodeSigPropPass encodes the Significance Propagation Pass
// This pass encodes coefficients that:
// - Are not yet significant
// - Have at least one significant neighbor
func (t1 *Encoder) encodeSigPropPass(raw bool) int {
	paddedWidth := t1.width + 2
	nmsedec := 0

	// JPEG 2000 passes are stripe-coded: process 4-row groups, then columns, then rows in stripe.
	for k := 0; k < t1.height; k += 4 {
		for x := 0; x < t1.width; x++ {
			for dy := 0; dy < 4 && k+dy < t1.height; dy++ {
				y := k + dy
				idx := (y+1)*paddedWidth + (x + 1)
				flags := t1.flags[idx]

				// Skip if already significant
				if flags&T1Sig != 0 {
					continue
				}

				// Check if has significant neighbor
				if flags&T1SigNeighbors == 0 {
					continue
				}

				// Check if coefficient is significant at this bit-plane
				absVal := t1.data[idx]
				if absVal < 0 {
					absVal = -absVal
				}
				isSig := (absVal >> uint(t1.bitplane)) & 1

				// Encode significance bit
				ctx := getZeroCodingContext(flags, t1.orientation)
				if raw {
					t1.mqe.BypassEncode(int(isSig))
				} else {
					t1.mqe.Encode(int(isSig), int(ctx))
				}

				// Mark as visited in SPP regardless of significance result (OpenJPEG PI flag behavior).
				t1.flags[idx] |= T1Visit

				if isSig != 0 {
					nmsedec += t1.getNMSEDecSig(absVal)

					// Coefficient becomes significant
					// Encode sign bit with prediction
					signBit := 0
					if t1.data[idx] < 0 {
						signBit = 1
						t1.flags[idx] |= T1Sign
					}

					if raw {
						t1.mqe.BypassEncode(signBit)
					} else {
						signCtx := getSignCodingContext(flags)
						signPred := getSignPrediction(flags)
						encodedSign := signBit ^ signPred
						t1.mqe.Encode(encodedSign, int(signCtx))
					}

					// Mark as significant (VISIT already set for this SPP sample).
					t1.flags[idx] |= T1Sig

					// Update neighbor flags
					t1.updateNeighborFlags(x, y, idx)
				}
				// Note: Do not clear VISIT here - it prevents MRP from re-processing
			}
		}
	}

	return nmsedec
}

// encodeMagRefPass encodes the Magnitude Refinement Pass
// This pass refines coefficients that are already significant
func (t1 *Encoder) encodeMagRefPass(raw bool) int {
	paddedWidth := t1.width + 2
	nmsedec := 0

	// JPEG 2000 passes are stripe-coded: process 4-row groups, then columns, then rows in stripe.
	for k := 0; k < t1.height; k += 4 {
		for x := 0; x < t1.width; x++ {
			for dy := 0; dy < 4 && k+dy < t1.height; dy++ {
				y := k + dy
				idx := (y+1)*paddedWidth + (x + 1)
				flags := t1.flags[idx]

				// Only refine significant coefficients not visited in this bit-plane
				if (flags&T1Sig) == 0 || (flags&T1Visit) != 0 {
					continue
				}

				// Get refinement bit at current bit-plane
				absVal := t1.data[idx]
				if absVal < 0 {
					absVal = -absVal
				}
				refBit := (absVal >> uint(t1.bitplane)) & 1

				// Encode refinement bit
				ctx := getMagRefinementContext(flags)
				nmsedec += t1.getNMSEDecRef(absVal)
				if raw {
					t1.mqe.BypassEncode(int(refBit))
				} else {
					t1.mqe.Encode(int(refBit), int(ctx))
				}

				// Mark as refined (OpenJPEG MU flag behavior).
				t1.flags[idx] |= T1Refine
			}
		}
	}

	return nmsedec
}

// encodeCleanupPass encodes the Cleanup Pass
// This pass encodes all remaining coefficients not encoded in previous passes
// IMPORTANT: Process in VERTICAL order (column-first) with 4-row groups for RL encoding
// This matches OpenJPEG's opj_t1_enc_clnpass() implementation
func (t1 *Encoder) encodeCleanupPass() int {
	paddedWidth := t1.width + 2
	nmsedec := 0

	// Process in groups of 4 rows (vertical RL encoding)
	for k := 0; k < t1.height; k += 4 {
		for i := 0; i < t1.width; i++ {
			// Try run-length encoding for this column (4 vertical coefficients)
			// Only if all 4 rows are available
			if k+3 < t1.height {
				// Check if run-length coding can be applied to this 4-coeff vertical run
				canUseRL := true
				rlSigPos := -1 // Position (0-3) of first significant coeff in vertical run

				for dy := 0; dy < 4; dy++ {
					y := k + dy
					idx := (y+1)*paddedWidth + (i + 1)

					// Skip if already visited
					if (t1.flags[idx] & T1Visit) != 0 {
						canUseRL = false
						break
					}

					// Check if coefficient or neighbors are already significant
					if (t1.flags[idx]&T1Sig) != 0 || (t1.flags[idx]&T1SigNeighbors) != 0 {
						canUseRL = false
						break
					}

					// Check if this coefficient is significant at current bitplane
					if rlSigPos == -1 {
						absVal := t1.data[idx]
						if absVal < 0 {
							absVal = -absVal
						}
						if ((absVal >> uint(t1.bitplane)) & 1) != 0 {
							rlSigPos = dy
						}
					}
				}

				if canUseRL {
					// Encode run-length bit (0 = all insignificant, 1 = at least one significant)
					rlBit := 0
					if rlSigPos >= 0 {
						rlBit = 1
					}
					t1.mqe.Encode(rlBit, CTXRL)

					if rlBit == 0 {
						continue // Move to next column
					}

					// Encode runlen index with uniform context
					runlenMSB := (rlSigPos >> 1) & 1
					runlenLSB := rlSigPos & 1
					t1.mqe.Encode(runlenMSB, CTXUNI)
					t1.mqe.Encode(runlenLSB, CTXUNI)

					// In RL path, the first sample at runlen is implicitly significant
					partial := true
					for dy := rlSigPos; dy < 4; dy++ {
						y := k + dy
						idx := (y+1)*paddedWidth + (i + 1)
						flags := t1.flags[idx]

						if (flags&T1Visit) != 0 || (flags&T1Sig) != 0 {
							t1.flags[idx] &^= T1Visit
							continue
						}

						isSig := 0
						if partial {
							isSig = 1
							partial = false
						} else {
							absVal := t1.data[idx]
							if absVal < 0 {
								absVal = -absVal
							}
							isSig = int((absVal >> uint(t1.bitplane)) & 1)

							// Encode significance bit
							ctx := getZeroCodingContext(flags, t1.orientation)
							t1.mqe.Encode(isSig, int(ctx))
						}

						if isSig != 0 {
							absVal := t1.data[idx]
							if absVal < 0 {
								absVal = -absVal
							}
							nmsedec += t1.getNMSEDecSig(absVal)

							// Encode sign bit with prediction (same as OpenJPEG clnpass)
							signBit := 0
							if t1.data[idx] < 0 {
								signBit = 1
								t1.flags[idx] |= T1Sign
							}
							signCtx := getSignCodingContext(flags)
							signPred := getSignPrediction(flags)
							encodedSign := signBit ^ signPred
							t1.mqe.Encode(encodedSign, int(signCtx))

							// Mark as significant. Cleanup pass does not keep PI/VISIT set.
							t1.flags[idx] |= T1Sig

							// Update neighbor flags
							t1.updateNeighborFlags(i, y, idx)
						}

						// Match OpenJPEG PI behavior: cleanup pass clears PI/VISIT after handling a sample.
						t1.flags[idx] &^= T1Visit
					}

					continue // RL encoding handled this column, move to next
				}
			}

			// Normal processing (not part of RL encoding, or partial row group)
			for dy := 0; dy < 4 && k+dy < t1.height; dy++ {
				y := k + dy
				idx := (y+1)*paddedWidth + (i + 1)
				flags := t1.flags[idx]

				if (flags&T1Visit) != 0 || (flags&T1Sig) != 0 {
					t1.flags[idx] &^= T1Visit
					continue
				}

				// Check if coefficient is significant at this bit-plane
				absVal := t1.data[idx]
				if absVal < 0 {
					absVal = -absVal
				}
				isSig := int((absVal >> uint(t1.bitplane)) & 1)

				// Encode significance bit
				ctx := getZeroCodingContext(flags, t1.orientation)
				t1.mqe.Encode(isSig, int(ctx))

				if isSig != 0 {
					nmsedec += t1.getNMSEDecSig(absVal)

					// Encode sign bit with prediction (same as OpenJPEG clnpass)
					signBit := 0
					if t1.data[idx] < 0 {
						signBit = 1
						t1.flags[idx] |= T1Sign
					}
					signCtx := getSignCodingContext(flags)
					signPred := getSignPrediction(flags)
					encodedSign := signBit ^ signPred
					t1.mqe.Encode(encodedSign, int(signCtx))

					// Mark as significant. Cleanup pass does not keep PI/VISIT set.
					t1.flags[idx] |= T1Sig

					// Update neighbor flags
					t1.updateNeighborFlags(i, y, idx)
				}

				// Match OpenJPEG PI behavior: cleanup pass clears PI/VISIT after handling a sample.
				t1.flags[idx] &^= T1Visit
			}
		}
	}

	return nmsedec
}

// updateNeighborFlags updates the neighbor significance flags
// when a coefficient becomes significant
func (t1 *Encoder) updateNeighborFlags(x, y, idx int) {
	paddedWidth := t1.width + 2
	sign := t1.flags[idx] & T1Sign

	// Update 8 neighbors
	// Padding ensures all neighbors are valid, no boundary checks needed

	// North
	nIdx := (y)*paddedWidth + (x + 1)
	t1.flags[nIdx] |= T1SigS
	if sign != 0 {
		t1.flags[nIdx] |= T1SignS
	}

	// South
	sIdx := (y+2)*paddedWidth + (x + 1)
	t1.flags[sIdx] |= T1SigN
	if sign != 0 {
		t1.flags[sIdx] |= T1SignN
	}

	// West
	wIdx := (y+1)*paddedWidth + x
	t1.flags[wIdx] |= T1SigE
	if sign != 0 {
		t1.flags[wIdx] |= T1SignE
	}

	// East
	eIdx := (y+1)*paddedWidth + (x + 2)
	t1.flags[eIdx] |= T1SigW
	if sign != 0 {
		t1.flags[eIdx] |= T1SignW
	}

	// Northwest
	t1.flags[(y)*paddedWidth+x] |= T1SigSE

	// Northeast
	t1.flags[(y)*paddedWidth+(x+2)] |= T1SigSW

	// Southwest
	t1.flags[(y+2)*paddedWidth+x] |= T1SigNE

	// Southeast
	t1.flags[(y+2)*paddedWidth+(x+2)] |= T1SigNW
}

// ComputeDistortion computes the distortion for rate-distortion optimization
// This is a simplified version - full implementation would use MSE reduction tables
func (t1 *Encoder) ComputeDistortion() float64 {
	distortion := 0.0

	paddedWidth := t1.width + 2
	for y := 0; y < t1.height; y++ {
		for x := 0; x < t1.width; x++ {
			idx := (y+1)*paddedWidth + (x + 1)

			// Compute quantization error
			// For now, use simple MSE calculation
			// Full implementation would use pre-computed tables
			val := float64(t1.data[idx])
			distortion += val * val
		}
	}

	return distortion
}

// GetRate returns the current encoding rate (in bytes)
func (t1 *Encoder) GetRate() int {
	if t1.mqe == nil {
		return 0
	}
	// This is an approximation - actual rate would need to flush and measure
	return 0 // Placeholder
}

// SetQuantization applies quantization to the coefficients
// This modifies the input data by quantizing based on step size
func SetQuantization(data []int32, stepSize float64) {
	if stepSize <= 0 {
		return
	}

	invStepSize := 1.0 / stepSize
	for i := range data {
		// Quantize: val = sign(val) * floor(abs(val) / stepSize)
		val := float64(data[i])
		if val >= 0 {
			data[i] = int32(math.Floor(val * invStepSize))
		} else {
			data[i] = -int32(math.Floor(-val * invStepSize))
		}
	}
}
