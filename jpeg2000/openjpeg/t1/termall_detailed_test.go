package t1

import (
	"testing"

	"github.com/cocosip/go-dicom-codecs/jpeg2000/openjpeg/mqc"
)

// TestTERMALLDetailed provides detailed debugging of TERMALL decoding
func TestTERMALLDetailed(t *testing.T) {
	// Use a very simple case: 4x4 block with only first pixel = 1
	width, height := 4, 4
	data := make([]int32, width*height)
	data[0] = 1 // Only first pixel is 1, rest are 0
	maxBitplane := CalculateMaxBitplane(data)
	numPasses := (maxBitplane * 3) + 1 // CP on top bitplane, then SPP/MRP/CP

	t.Logf("Input data: %v", data)

	// First encode with NORMAL mode for comparison
	encoderNormal := NewT1Encoder(width, height, 0)
	encodedNormal, err := encoderNormal.Encode(data, numPasses, 0)
	if err != nil {
		t.Fatalf("Normal encode failed: %v", err)
	}
	t.Logf("\nNormal mode encoding: %d bytes", len(encodedNormal))
	t.Logf("Normal data: % 02x", encodedNormal)

	// Decode normal to verify
	decoderNormal := NewT1Decoder(width, height, 0)
	if err := decoderNormal.DecodeWithBitplane(encodedNormal, numPasses, maxBitplane, 0); err != nil {
		t.Fatalf("Normal decode failed: %v", err)
	}
	decodedNormal := decoderNormal.GetData()
	t.Logf("Normal decoded: %v", decodedNormal[:16])

	// Encode with TERMALL
	encoder := NewT1Encoder(width, height, 0)
	layerBoundaries := []int{numPasses}
	passes, completeData, err := encoder.EncodeLayered(data, numPasses, 0, layerBoundaries, 0x04)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	t.Logf("\nEncoded %d passes, %d bytes total", len(passes), len(completeData))
	t.Logf("Complete data: % 02x", completeData)

	// Print each pass
	prevEnd := 0
	passLengths := make([]int, len(passes))
	for i, p := range passes {
		passLengths[i] = p.ActualBytes
		currentEnd := p.ActualBytes
		passBytes := completeData[prevEnd:currentEnd]
		t.Logf("Pass %d (bp=%d, type=%d): bytes[%d:%d] = % 02x",
			i, p.Bitplane, p.PassType, prevEnd, currentEnd, passBytes)
		prevEnd = currentEnd
	}

	// Now try to decode each pass individually and see what coefficients get set
	t.Logf("\n=== Decoding Each Pass Individually ===")

	// Decode pass 0 (CP) only
	decoder0 := NewT1Decoder(width, height, 0)
	passData0 := completeData[0:passLengths[0]]
	t.Logf("\nPass 0 data: % 02x", passData0)
	decoder0.mqc = mqc.NewMQDecoder(passData0, NUMCONTEXTS)
	// Set initial context states to match OpenJPEG
	decoder0.mqc.SetContextState(CTXUNI, 46)
	decoder0.mqc.SetContextState(CTXRL, 3)
	decoder0.mqc.SetContextState(0, 4)
	decoder0.roishift = 0
	decoder0.bitplane = maxBitplane
	decoder0.decodeCleanupPass()
	decoded0 := decoder0.GetData()
	t.Logf("After pass 0: %v", decoded0[:16])

	// Check error after the cleanup pass
	maxError := int32(0)
	decodedFinal := decoder0.GetData()
	for i := 0; i < len(data); i++ {
		diff := data[i] - decodedFinal[i]
		if diff < 0 {
			diff = -diff
		}
		if diff > maxError {
			maxError = diff
		}
	}
	t.Logf("Final error: %d", maxError)

	if maxError > 0 {
		t.Errorf("Expected perfect reconstruction, got error=%d", maxError)
	}
}
