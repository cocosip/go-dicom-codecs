package t1

import "testing"

func TestOpenJPEGLayeredEncodeDoesNotTerminateAtLayerBoundariesWhenCblkStyleIsZero(t *testing.T) {
	data := make([]int32, 64)
	for i := range data {
		data[i] = int32((i % 31) + 1)
	}

	enc := NewT1Encoder(8, 8, 0)
	passes, _, err := enc.EncodeLayered(data, 13, 0, []int{1, 13}, 0x00)
	if err != nil {
		t.Fatalf("EncodeLayered failed: %v", err)
	}
	if len(passes) < 2 {
		t.Fatalf("expected multiple passes, got %d", len(passes))
	}
	if passes[0].Terminated {
		t.Fatalf("OpenJPEG cblksty=0 must not terminate pass 0 just because it is a layer boundary")
	}
	if !passes[len(passes)-1].Terminated {
		t.Fatalf("OpenJPEG cblksty=0 must still terminate the final cleanup pass")
	}
}

func TestOpenJPEGLayeredDistortionUsesNMSEReductionLookup(t *testing.T) {
	enc := NewT1Encoder(1, 1, 0)
	passes, _, err := enc.EncodeLayered([]int32{1}, 1, 0, nil, 0x00)
	if err != nil {
		t.Fatalf("EncodeLayered failed: %v", err)
	}
	if len(passes) != 1 {
		t.Fatalf("expected one pass, got %d", len(passes))
	}

	const want = 8192.0 // OpenJPEG lut_nmsedec_sig0[1 << T1_NMSEDEC_FRACBITS]
	if passes[0].Distortion != want {
		t.Fatalf("distortion = %v, want OpenJPEG nmsedec-derived %v", passes[0].Distortion, want)
	}
}

func TestOpenJPEGLayeredAllZeroBlockHasNoPasses(t *testing.T) {
	enc := NewT1Encoder(4, 4, 0)
	passes, data, err := enc.EncodeLayered(make([]int32, 16), 1, 0, nil, 0x00)
	if err != nil {
		t.Fatalf("EncodeLayered failed: %v", err)
	}
	if len(passes) != 0 {
		t.Fatalf("all-zero block passes = %d, want 0 like OpenJPEG", len(passes))
	}
	if len(data) != 0 {
		t.Fatalf("all-zero block data length = %d, want 0", len(data))
	}
}
