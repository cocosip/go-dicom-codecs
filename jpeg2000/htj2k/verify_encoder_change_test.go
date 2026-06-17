package htj2k

import (
	"testing"
)

// TestVerifyEncoderChange verifies that encoder changes are taking effect
func TestVerifyEncoderChange(t *testing.T) {
	width := 4
	height := 4
	size := width * height

	// Create simple test data with just one significant quad
	testCoeffs := make([]int32, size)
	testCoeffs[0] = 100 // Only first sample significant

	encoded, decoded := encodeDecodeOpenJPHCleanupForTest(t, width, height, testCoeffs)

	t.Logf("Single value=100: %d bytes: %v", len(encoded), encoded)

	// Now test with all samples
	testCoeffs2 := make([]int32, size)
	for i := 0; i < size; i++ {
		testCoeffs2[i] = int32(i + 1) // All non-zero
	}

	encoded2, _ := encodeDecodeOpenJPHCleanupForTest(t, width, height, testCoeffs2)

	t.Logf("All non-zero: %d bytes: %v", len(encoded2), encoded2)

	if decoded[0] != 100 {
		t.Errorf("Expected decoded[0]=100, got %d", decoded[0])
	}

	t.Logf("✓ Simple decode works")
}
