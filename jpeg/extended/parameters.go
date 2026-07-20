package extended

import (
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

// Ensure JPEGExtendedParameters implements codec.Parameters
var _ codec.Parameters = (*JPEGExtendedParameters)(nil)

// JPEGExtendedParameters contains parameters for JPEG Extended compression
type JPEGExtendedParameters struct {
	// Quality controls the JPEG compression quality (1-100)
	// - 100: Best quality, minimal compression
	// - 85:  High quality (default)
	// - 75:  Medium quality, good balance
	// - 50:  Lower quality, higher compression
	// - 1:   Lowest quality, maximum compression
	Quality int

	// BitDepth controls the bits per sample (8 or 12)
	// - 8:  Standard 8-bit encoding
	// - 12: Extended 12-bit encoding (default, main feature of JPEG Extended)
	//
	// Note: JPEG Extended supports both 8-bit and 12-bit, distinguishing it
	// from JPEG Baseline which only supports 8-bit.
	BitDepth int

	// internal storage for compatibility with generic parameter interface
	params map[string]interface{}
}

// NewExtendedParameters creates a new JPEGExtendedParameters with default values
func NewExtendedParameters() *JPEGExtendedParameters {
	return &JPEGExtendedParameters{
		Quality:  90, // Matches fo-dicom's default DicomJpegParams quality.
		BitDepth: 12, // Default 12-bit (main feature of Extended)
		params:   make(map[string]interface{}),
	}
}

// GetParameter retrieves a parameter by name (implements codec.Parameters)
func (p *JPEGExtendedParameters) GetParameter(name string) interface{} {
	switch name {
	case "quality":
		return p.Quality
	case "bitDepth":
		return p.BitDepth
	default:
		// Check custom parameters
		return p.params[name]
	}
}

// SetParameter sets a parameter value (implements codec.Parameters)
func (p *JPEGExtendedParameters) SetParameter(name string, value interface{}) {
	switch name {
	case "quality":
		if v, ok := value.(int); ok {
			p.Quality = v
		}
	case "bitDepth":
		if v, ok := value.(int); ok {
			p.BitDepth = v
		}
	default:
		// Store as custom parameter
		p.params[name] = value
	}
}

// Validate checks if the parameters are valid and adjusts them if needed
func (p *JPEGExtendedParameters) Validate() error {
	// Quality must be in range 1-100
	if p.Quality < 1 || p.Quality > 100 {
		p.Quality = 90 // Matches fo-dicom's default DicomJpegParams quality.
	}

	// BitDepth must be 8 or 12
	if p.BitDepth != 8 && p.BitDepth != 12 {
		p.BitDepth = 12 // Reset to default
	}

	return nil
}

// WithQuality sets the quality and returns the parameters for chaining
func (p *JPEGExtendedParameters) WithQuality(quality int) *JPEGExtendedParameters {
	p.Quality = quality
	return p
}

// WithBitDepth sets the bit depth and returns the parameters for chaining
func (p *JPEGExtendedParameters) WithBitDepth(bitDepth int) *JPEGExtendedParameters {
	p.BitDepth = bitDepth
	return p
}
