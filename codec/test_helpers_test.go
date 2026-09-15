package codec

import (
	"context"
	"errors"
	"testing"
)

func TestTestPixelDataFrameReturnsRangeError(t *testing.T) {
	pixelData := &TestPixelData{}

	_, err := pixelData.Frame(context.Background(), 0)
	if err == nil {
		t.Fatal("Frame() error = nil, want out-of-range error")
	}
	if errors.Is(err, context.Canceled) {
		t.Fatalf("Frame() error = %v, want a range error", err)
	}
}

func TestTestPixelDataFrameReturnsContextError(t *testing.T) {
	pixelData := &TestPixelData{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := pixelData.Frame(ctx, 0)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Frame() error = %v, want context.Canceled", err)
	}
}
