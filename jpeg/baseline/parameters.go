package baseline

import (
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

// Ensure JPEGBaselineParameters implements codec.Parameters
var _ codec.Parameters = (*JPEGBaselineParameters)(nil)

// JPEGBaselineParameters contains parameters for JPEG Baseline compression
type JPEGBaselineParameters struct {
	// Quality controls the JPEG compression quality (1-100)
	// - 100: Best quality, minimal compression
	// - 90:  High quality (default)
	// - 75:  Medium quality, good balance
	// - 50:  Lower quality, higher compression
	// - 1:   Lowest quality, maximum compression
	Quality int

	// internal storage for compatibility with generic parameter interface
	params map[string]interface{}
}

// NewBaselineParameters creates a new JPEGBaselineParameters with default values
func NewBaselineParameters() *JPEGBaselineParameters {
	return &JPEGBaselineParameters{
		Quality: 90, // Default high quality
		params:  make(map[string]interface{}),
	}
}

// GetParameter retrieves a parameter by name (implements codec.Parameters)
func (p *JPEGBaselineParameters) GetParameter(name string) interface{} {
	switch name {
	case "quality":
		return p.Quality
	default:
		// Check custom parameters
		return p.params[name]
	}
}

// SetParameter sets a parameter value (implements codec.Parameters)
func (p *JPEGBaselineParameters) SetParameter(name string, value interface{}) {
	switch name {
	case "quality":
		if v, ok := value.(int); ok {
			p.Quality = v
		}
	default:
		// Store as custom parameter
		p.params[name] = value
	}
}

// Validate checks if the parameters are valid
func (p *JPEGBaselineParameters) Validate() error {
	if p.Quality < 1 || p.Quality > 100 {
		p.Quality = 90 // Reset to default
	}
	return nil
}

// WithQuality sets the quality and returns the parameters for chaining
func (p *JPEGBaselineParameters) WithQuality(quality int) *JPEGBaselineParameters {
	p.Quality = quality
	return p
}
