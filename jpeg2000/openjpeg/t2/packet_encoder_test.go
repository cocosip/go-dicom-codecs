package t2

import (
	"testing"
)

// TestPacketEncoderCreation tests encoder creation
func TestPacketEncoderCreation(t *testing.T) {
	enc := NewPacketEncoder(1, 1, 1, ProgressionLRCP)
	if enc == nil {
		t.Fatal("Failed to create packet encoder")
	}

	if enc.numComponents != 1 {
		t.Errorf("numComponents: got %d, want 1", enc.numComponents)
	}
	if enc.numLayers != 1 {
		t.Errorf("numLayers: got %d, want 1", enc.numLayers)
	}
	if enc.numResolutions != 1 {
		t.Errorf("numResolutions: got %d, want 1", enc.numResolutions)
	}
}

// TestPacketEncoderEmptyPacket tests encoding an empty packet
func TestPacketEncoderEmptyPacket(t *testing.T) {
	enc := NewPacketEncoder(1, 1, 1, ProgressionLRCP)

	// Encode without adding any code-blocks
	packets, err := enc.EncodePackets()
	if err != nil {
		t.Fatalf("EncodePackets failed: %v", err)
	}

	// Should produce empty result or handle gracefully
	t.Logf("Encoded %d packets", len(packets))
}

// TestPacketEncoderSingleCodeBlock tests encoding with a single code-block
func TestPacketEncoderSingleCodeBlock(t *testing.T) {
	enc := NewPacketEncoder(1, 1, 1, ProgressionLRCP)

	// Create a code-block with some data
	cb := &PrecinctCodeBlock{
		Index:          0,
		X0:             0,
		Y0:             0,
		X1:             32,
		Y1:             32,
		Included:       false,
		NumPassesTotal: 3,
		ZeroBitPlanes:  0,
		Data:           []byte{0x12, 0x34, 0x56, 0x78},
	}

	// Add to precinct
	enc.AddCodeBlock(0, 0, 0, cb)

	// Encode
	packets, err := enc.EncodePackets()
	if err != nil {
		t.Fatalf("EncodePackets failed: %v", err)
	}

	if len(packets) != 1 {
		t.Fatalf("Expected 1 packet, got %d", len(packets))
	}

	packet := packets[0]
	t.Logf("Packet: layer=%d, res=%d, comp=%d, precinct=%d",
		packet.LayerIndex, packet.ResolutionLevel, packet.ComponentIndex, packet.PrecinctIndex)
	t.Logf("Header: %d bytes, Body: %d bytes", len(packet.Header), len(packet.Body))

	if !packet.HeaderPresent {
		t.Error("Expected HeaderPresent=true")
	}

	if len(packet.Body) != 4 {
		t.Errorf("Body length: got %d, want 4", len(packet.Body))
	}
}

// TestPacketEncoderMultipleCodeBlocks tests encoding with multiple code-blocks
func TestPacketEncoderMultipleCodeBlocks(t *testing.T) {
	enc := NewPacketEncoder(1, 1, 1, ProgressionLRCP)

	// Add multiple code-blocks
	for i := 0; i < 4; i++ {
		cb := &PrecinctCodeBlock{
			Index:          i,
			X0:             i * 32,
			Y0:             0,
			X1:             (i + 1) * 32,
			Y1:             32,
			Included:       false,
			NumPassesTotal: 3,
			ZeroBitPlanes:  0,
			Data:           []byte{byte(i), byte(i + 1)},
		}
		enc.AddCodeBlock(0, 0, 0, cb)
	}

	// Encode
	packets, err := enc.EncodePackets()
	if err != nil {
		t.Fatalf("EncodePackets failed: %v", err)
	}

	if len(packets) != 1 {
		t.Fatalf("Expected 1 packet, got %d", len(packets))
	}

	packet := packets[0]
	t.Logf("Encoded packet with %d code-block contributions", len(packet.CodeBlockIncls))

	// Check that all code-blocks are included
	if len(packet.CodeBlockIncls) != 4 {
		t.Errorf("Expected 4 code-block inclusions, got %d", len(packet.CodeBlockIncls))
	}
}

// TestPacketEncoderMultipleLayers tests encoding with multiple quality layers
func TestPacketEncoderMultipleLayers(t *testing.T) {
	numLayers := 3
	enc := NewPacketEncoder(1, numLayers, 1, ProgressionLRCP)

	// Add a code-block
	cb := &PrecinctCodeBlock{
		Index:          0,
		X0:             0,
		Y0:             0,
		X1:             32,
		Y1:             32,
		Included:       false,
		NumPassesTotal: 9, // 3 layers × 3 passes
		ZeroBitPlanes:  0,
		Data:           []byte{0x12, 0x34, 0x56},
	}
	enc.AddCodeBlock(0, 0, 0, cb)

	// Encode
	packets, err := enc.EncodePackets()
	if err != nil {
		t.Fatalf("EncodePackets failed: %v", err)
	}

	// Should have one packet per layer
	if len(packets) != numLayers {
		t.Errorf("Expected %d packets, got %d", numLayers, len(packets))
	}

	for i, packet := range packets {
		if packet.LayerIndex != i {
			t.Errorf("Packet %d: expected layer %d, got %d", i, i, packet.LayerIndex)
		}
		t.Logf("Layer %d packet: header=%d bytes, body=%d bytes",
			i, len(packet.Header), len(packet.Body))
	}
}

// TestPacketEncoderMultipleResolutions tests encoding with multiple resolutions
func TestPacketEncoderMultipleResolutions(t *testing.T) {
	numResolutions := 3
	enc := NewPacketEncoder(1, 1, numResolutions, ProgressionLRCP)

	// Add code-blocks at different resolutions
	for res := 0; res < numResolutions; res++ {
		cb := &PrecinctCodeBlock{
			Index:          0,
			X0:             0,
			Y0:             0,
			X1:             32,
			Y1:             32,
			Included:       false,
			NumPassesTotal: 3,
			ZeroBitPlanes:  0,
			Data:           []byte{byte(res)},
		}
		enc.AddCodeBlock(0, res, 0, cb)
	}

	// Encode
	packets, err := enc.EncodePackets()
	if err != nil {
		t.Fatalf("EncodePackets failed: %v", err)
	}

	// Should have one packet per resolution
	if len(packets) != numResolutions {
		t.Errorf("Expected %d packets, got %d", numResolutions, len(packets))
	}

	for i, packet := range packets {
		if packet.ResolutionLevel != i {
			t.Errorf("Packet %d: expected resolution %d, got %d", i, i, packet.ResolutionLevel)
		}
	}
}

// TestPacketEncoderRLCP tests RLCP progression order
func TestPacketEncoderRLCP(t *testing.T) {
	enc := NewPacketEncoder(2, 2, 2, ProgressionRLCP)

	// Add code-blocks for different component/resolution combinations
	for comp := 0; comp < 2; comp++ {
		for res := 0; res < 2; res++ {
			cb := &PrecinctCodeBlock{
				Index:          0,
				X0:             0,
				Y0:             0,
				X1:             32,
				Y1:             32,
				Included:       false,
				NumPassesTotal: 3,
				ZeroBitPlanes:  0,
				Data:           []byte{byte(comp*10 + res)},
			}
			enc.AddCodeBlock(comp, res, 0, cb)
		}
	}

	// Encode
	packets, err := enc.EncodePackets()
	if err != nil {
		t.Fatalf("EncodePackets failed: %v", err)
	}

	t.Logf("Encoded %d packets in RLCP order", len(packets))

	// Verify RLCP order: Resolution, then Layer, then Component
	expectedOrder := []struct {
		res  int
		comp int
	}{
		{0, 0}, {0, 1}, // Resolution 0, both components, layer 0
		{0, 0}, {0, 1}, // Resolution 0, both components, layer 1
		{1, 0}, {1, 1}, // Resolution 1, both components, layer 0
		{1, 0}, {1, 1}, // Resolution 1, both components, layer 1
	}

	for i, expected := range expectedOrder {
		if i >= len(packets) {
			break
		}
		p := packets[i]
		if p.ResolutionLevel != expected.res || p.ComponentIndex != expected.comp {
			t.Errorf("Packet %d: expected (R=%d,C=%d), got (R=%d,C=%d)",
				i, expected.res, expected.comp, p.ResolutionLevel, p.ComponentIndex)
		}
	}
}
