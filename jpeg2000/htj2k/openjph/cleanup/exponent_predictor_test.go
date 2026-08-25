package cleanup

import "testing"

func TestMagnitudeExponent(t *testing.T) {
	tests := []struct {
		magnitude uint32
		expected  int
	}{
		{0, 0},     // μ=0 -> E=0
		{1, 1},     // μ=1 -> E=1 (2^0 < 1 <= 2^1)
		{2, 2},     // μ=2 -> E=2 (2^1 < 2 <= 2^2)
		{3, 2},     // μ=3 -> E=2 (2^1 < 3 <= 2^2)
		{4, 3},     // μ=4 -> E=3 (2^2 < 4 <= 2^3)
		{7, 3},     // μ=7 -> E=3 (2^2 < 7 <= 2^3)
		{8, 4},     // μ=8 -> E=4
		{15, 4},    // μ=15 -> E=4
		{16, 5},    // μ=16 -> E=5
		{127, 7},   // μ=127 -> E=7
		{128, 8},   // μ=128 -> E=8
		{255, 8},   // μ=255 -> E=8
		{256, 9},   // μ=256 -> E=9
		{1023, 10}, // μ=1023 -> E=10
		{1024, 11}, // μ=1024 -> E=11
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := MagnitudeExponent(tt.magnitude)
			if got != tt.expected {
				t.Errorf("MagnitudeExponent(%d) = %d, want %d", tt.magnitude, got, tt.expected)
			}
		})
	}
}

func TestQuadMaxExponent(t *testing.T) {
	tests := []struct {
		name     string
		mag0     uint32
		mag1     uint32
		mag2     uint32
		mag3     uint32
		maxE     int
		sigCount int
	}{
		{
			name:     "All zeros",
			mag0:     0,
			mag1:     0,
			mag2:     0,
			mag3:     0,
			maxE:     0,
			sigCount: 0,
		},
		{
			name:     "One significant",
			mag0:     5,
			mag1:     0,
			mag2:     0,
			mag3:     0,
			maxE:     3, // E(5) = 3
			sigCount: 1,
		},
		{
			name:     "Two significant",
			mag0:     3,
			mag1:     7,
			mag2:     0,
			mag3:     0,
			maxE:     3, // max(E(3), E(7)) = max(2, 3) = 3
			sigCount: 2,
		},
		{
			name:     "All significant",
			mag0:     1,
			mag1:     2,
			mag2:     4,
			mag3:     8,
			maxE:     4, // E(8) = 4
			sigCount: 4,
		},
		{
			name:     "Large values",
			mag0:     255,
			mag1:     127,
			mag2:     63,
			mag3:     31,
			maxE:     8, // E(255) = 8
			sigCount: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maxE, sigCount := QuadMaxExponent(tt.mag0, tt.mag1, tt.mag2, tt.mag3)

			if maxE != tt.maxE {
				t.Errorf("QuadMaxExponent() maxE = %d, want %d", maxE, tt.maxE)
			}

			if sigCount != tt.sigCount {
				t.Errorf("QuadMaxExponent() sigCount = %d, want %d", sigCount, tt.sigCount)
			}
		})
	}
}

func TestExponentPredictorComputer_FirstRow(t *testing.T) {
	// Test that first row always returns Kq = 1
	epc := NewExponentPredictorComputer(8, 8)

	// Set some exponents (shouldn't matter for first row)
	epc.SetQuadExponents(0, 0, 5, 2)
	epc.SetQuadExponents(1, 0, 7, 3)
	epc.SetQuadExponents(2, 0, 3, 1)

	// All first row quads should have Kq = 1
	for qx := 0; qx < 8; qx++ {
		Kq := epc.ComputePredictor(qx, 0)
		if Kq != 1 {
			t.Errorf("ComputePredictor(%d, 0) = %d, want 1", qx, Kq)
		}
	}
}

func TestExponentPredictorComputer_NonFirstRow(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*ExponentPredictorComputer)
		qx       int
		qy       int
		expected int
		desc     string
	}{
		{
			name: "No gamma uses Kq=1",
			setup: func(epc *ExponentPredictorComputer) {
				// Top neighbor (0,0) has E'=0 (not set)
				// Current quad (0,1) has gamma=0 (<=1 significant)
				epc.SetQuadExponents(0, 1, 0, 0)
			},
			qx:       0,
			qy:       1,
			expected: 1, // Kq = 1 when gamma=0
			desc:     "Second row, gamma=0",
		},
		{
			name: "Gamma=0 ignores top",
			setup: func(epc *ExponentPredictorComputer) {
				// Set top neighbor (1,0) to E'=5
				epc.SetQuadExponents(1, 0, 5, 1)
				// Current quad (1,1) has gamma=0
				epc.SetQuadExponents(1, 1, 0, 1)
			},
			qx:       1,
			qy:       1,
			expected: 1, // Kq = 1 when gamma=0
			desc:     "Top neighbor E'=5, gamma=0",
		},
		{
			name: "Gamma=1 uses top exponent",
			setup: func(epc *ExponentPredictorComputer) {
				// Set top neighbor (2,0) to E'=5
				epc.SetQuadExponents(2, 0, 5, 1)
				// Current quad (2,1) has gamma=1
				epc.SetQuadExponents(2, 1, 0, 2)
			},
			qx:       2,
			qy:       1,
			expected: 4, // Kq = max(1, 5-1) = 4
			desc:     "Top neighbor E'=5, gamma=1",
		},
		{
			name: "Gamma=1 clamps to 1 when top is small",
			setup: func(epc *ExponentPredictorComputer) {
				// Set top neighbor (0,0) to E'=1
				epc.SetQuadExponents(0, 0, 1, 1)
				// Current quad (0,1) has gamma=1
				epc.SetQuadExponents(0, 1, 0, 2)
			},
			qx:       0,
			qy:       1,
			expected: 1, // Kq = max(1, 1-1) = 1
			desc:     "Top neighbor E'=1, gamma=1",
		},
		{
			name: "Gamma=1 with zero top exponent",
			setup: func(epc *ExponentPredictorComputer) {
				// Top neighbor (3,0) is unset -> E'=0
				// Current quad (3,1) has gamma=1
				epc.SetQuadExponents(3, 1, 0, 3)
			},
			qx:       3,
			qy:       1,
			expected: 1, // Kq = max(1, 0-1) = 1
			desc:     "Top neighbor E'=0, gamma=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			epc := NewExponentPredictorComputer(8, 8)
			tt.setup(epc)

			got := epc.ComputePredictor(tt.qx, tt.qy)
			if got != tt.expected {
				t.Errorf("ComputePredictor(%d, %d) = %d, want %d (%s)",
					tt.qx, tt.qy, got, tt.expected, tt.desc)
			}
		})
	}
}

func TestExponentPredictorComputer_ComputeExponentBound(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*ExponentPredictorComputer)
		qx       int
		qy       int
		uq       uint32
		expected int
	}{
		{
			name: "First row, uq=3",
			setup: func(epc *ExponentPredictorComputer) {
				epc.SetQuadExponents(2, 0, 0, 1)
			},
			qx:       2,
			qy:       0,
			uq:       3,
			expected: 4, // Kq=1 (first row), Uq = 1 + 3 = 4
		},
		{
			name: "Non-first row, gamma=0, uq=2",
			setup: func(epc *ExponentPredictorComputer) {
				epc.SetQuadExponents(2, 0, 7, 1)
				epc.SetQuadExponents(2, 1, 0, 1) // gamma=0
			},
			qx:       2,
			qy:       1,
			uq:       2,
			expected: 3, // Kq=1, Uq = 1 + 2 = 3
		},
		{
			name: "Non-first row, gamma=1, uq=5",
			setup: func(epc *ExponentPredictorComputer) {
				epc.SetQuadExponents(3, 1, 6, 1)
				epc.SetQuadExponents(3, 2, 0, 3) // gamma=1
			},
			qx:       3,
			qy:       2,
			uq:       5,
			expected: 10, // Kq=max(1, 6-1)=5, Uq = 5 + 5 = 10
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			epc := NewExponentPredictorComputer(8, 8)
			tt.setup(epc)

			got := epc.ComputeExponentBound(tt.qx, tt.qy, tt.uq)
			if got != tt.expected {
				t.Errorf("ComputeExponentBound(%d, %d, %d) = %d, want %d",
					tt.qx, tt.qy, tt.uq, got, tt.expected)
			}
		})
	}
}

func TestExponentPredictorComputer_CompleteScenario(t *testing.T) {
	// Test a complete 4x4 quad block scenario
	epc := NewExponentPredictorComputer(4, 4)

	// First row (qy=0): all Kq should be 1
	// Quad (0,0): magnitudes [1,2,3,4] -> E'=3, sig=4, gamma=1
	epc.SetQuadExponents(0, 0, 3, 4)
	Uq00 := epc.ComputeExponentBound(0, 0, 2) // uq=2
	if Uq00 != 3 {                            // Kq=1, Uq=1+2=3
		t.Errorf("Uq(0,0) = %d, want 3", Uq00)
	}

	// Quad (1,0): magnitudes [5,6,7,8] -> E'=4, sig=4, gamma=1
	epc.SetQuadExponents(1, 0, 4, 4)
	Uq10 := epc.ComputeExponentBound(1, 0, 3) // uq=3
	if Uq10 != 4 {                            // Kq=1, Uq=1+3=4
		t.Errorf("Uq(1,0) = %d, want 4", Uq10)
	}

	// Second row (qy=1)
	// Quad (0,1): left=none, top=(0,0)=3, gamma=0
	epc.SetQuadExponents(0, 1, 5, 1)
	Kq01 := epc.ComputePredictor(0, 1)
	if Kq01 != 1 { // gamma=0 -> Kq=1
		t.Errorf("Kq(0,1) = %d, want 1", Kq01)
	}

	// Quad (1,1): left=(0,1)=5, top=(1,0)=4, gamma=1
	epc.SetQuadExponents(1, 1, 6, 2)
	Kq11 := epc.ComputePredictor(1, 1)
	if Kq11 != 3 { // max(1, 4-1) = 3
		t.Errorf("Kq(1,1) = %d, want 3", Kq11)
	}

	Uq11 := epc.ComputeExponentBound(1, 1, 1) // uq=1
	if Uq11 != 4 {                            // Kq=3, Uq=3+1=4
		t.Errorf("Uq(1,1) = %d, want 4", Uq11)
	}
}
