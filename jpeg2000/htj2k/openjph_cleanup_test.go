package htj2k

import (
	"math/bits"
	"testing"
)

func encodeDecodeOpenJPHCleanupForTest(t *testing.T, width, height int, coeffs []int32) ([]byte, []int32) {
	t.Helper()
	kmax := testKmaxForCoeffs(coeffs)
	encoder := NewHTEncoder(width, height)
	encoder.SetKMax(kmax)
	encoded, err := encoder.Encode(coeffs, 1, 0)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoder := NewHTDecoder(width, height)
	decoder.SetCodingContext(kmax, kmax-1)
	decoded, err := decoder.Decode(encoded, 1)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	return encoded, decoded
}

func testKmaxForCoeffs(coeffs []int32) int {
	maxMag := int32(0)
	for _, v := range coeffs {
		if v < 0 {
			v = -v
		}
		if v > maxMag {
			maxMag = v
		}
	}
	kmax := bits.Len32(uint32(maxMag)) + 1
	if kmax < 2 {
		return 2
	}
	return kmax
}

func TestOpenJPHCleanupFirstPairStaysWithinKmax(t *testing.T) {
	coeffs := []int32{
		-64, -32, 0, 37,
		-64, -32, 0, 37,
		-64, -32, 0, 37,
		-64, -32, 0, 37,
	}
	encoder := NewHTEncoder(4, 4)
	encoder.SetKMax(9)
	encoded, err := encoder.Encode(coeffs, 1, 0)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	decoded, err := decodeOpenJPHCleanup(encoded, 4, 4, 9, 8)
	if err != nil {
		t.Fatalf("decode cleanup failed: %v encoded=% X", err, encoded)
	}
	for i := range coeffs {
		if decoded[i] != coeffs[i] {
			t.Fatalf("decoded[%d] = %d, want %d encoded=% X", i, decoded[i], coeffs[i], encoded)
		}
	}
	trace, err := traceOpenJPHCleanupFirstPair(encoded)
	if err != nil {
		t.Fatalf("trace cleanup failed: %v", err)
	}
	if trace.U0+1 > 10 || trace.U1+1 > 10 {
		t.Fatalf("first pair U_q out of range: trace=%+v encoded=% X", trace, encoded)
	}
}

func TestOpenJPHCleanupSingleSampleRoundTrip(t *testing.T) {
	coeffs := []int32{64}
	encoder := NewHTEncoder(1, 1)
	encoder.SetKMax(9)
	encoded, err := encoder.Encode(coeffs, 1, 0)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := decodeOpenJPHCleanup(encoded, 1, 1, 9, 8)
	if err != nil {
		t.Fatalf("decode cleanup failed: %v", err)
	}
	trace, err := traceOpenJPHCleanupFirstPair(encoded)
	if err != nil {
		t.Fatalf("trace cleanup failed: %v", err)
	}
	t.Logf("trace=%+v encoded=% X", trace, encoded)
	if len(decoded) != len(coeffs) {
		t.Fatalf("decoded length = %d, want %d", len(decoded), len(coeffs))
	}
	if decoded[0] != coeffs[0] {
		t.Fatalf("decoded[0] = %d, want %d encoded=% X", decoded[0], coeffs[0], encoded)
	}
}

func TestOpenJPHCleanupAllOnesAcrossQuadPairs(t *testing.T) {
	width, height := 8, 8
	coeffs := make([]int32, width*height)
	for i := range coeffs {
		coeffs[i] = 1
	}

	encoded, decoded := encodeDecodeOpenJPHCleanupForTest(t, width, height, coeffs)
	for i := range coeffs {
		if decoded[i] != coeffs[i] {
			t.Fatalf("decoded[%d] = %d, want %d encoded=% X decoded=%v",
				i, decoded[i], coeffs[i], encoded, decoded[:16])
		}
	}
}

func TestOpenJPHCleanupSingleNonZeroDoesNotLeakEvents(t *testing.T) {
	width, height := 8, 8
	coeffs := make([]int32, width*height)
	coeffs[0] = 100

	encoded, decoded := encodeDecodeOpenJPHCleanupForTest(t, width, height, coeffs)
	for i := range coeffs {
		if decoded[i] != coeffs[i] {
			t.Fatalf("decoded[%d] = %d, want %d encoded=% X",
				i, decoded[i], coeffs[i], encoded)
		}
	}
}
