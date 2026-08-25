package cleanup

import "errors"

// Common errors for HTJ2K decoding
var (
	// ErrInsufficientData indicates not enough data to complete decoding
	ErrInsufficientData = errors.New("htj2k: insufficient data")

	// ErrInvalidData indicates data is malformed or invalid
	ErrInvalidData = errors.New("htj2k: invalid data")

	// ErrInvalidBlockSize indicates block dimensions are invalid
	ErrInvalidBlockSize = errors.New("htj2k: invalid block size")

	// ErrInvalidSegment indicates segment structure is invalid
	ErrInvalidSegment = errors.New("htj2k: invalid segment")
)
