package lossless

import (
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

// Ensure JPEGLosslessParameters implements codec.Parameters
var _ codec.Parameters = (*JPEGLosslessParameters)(nil)

// JPEGLosslessParameters contains parameters for JPEG Lossless compression
type JPEGLosslessParameters struct {
	// Predictor selects the prediction algorithm (0-7)
	// - 0: Auto-select predictor (default)
	// - 1: No prediction (A)
	// - 2: Horizontal prediction (A)
	// - 3: Vertical prediction (B)
	// - 4: Diagonal prediction (A+B-C)
	// - 5: A + (B-C)/2
	// - 6: B + (A-C)/2
	// - 7: (A+B)/2
	//
	// For medical imaging, predictor 1 (horizontal) is commonly used and recommended.
	Predictor int

	// internal storage for compatibility with generic parameter interface
	params map[string]interface{}
}

// NewLosslessParameters creates a new JPEGLosslessParameters with default values
func NewLosslessParameters() *JPEGLosslessParameters {
	return &JPEGLosslessParameters{
		Predictor: 0, // Default auto-select
		params:    make(map[string]interface{}),
	}
}

// Clone returns an independent copy of the parameters.
func (p *JPEGLosslessParameters) Clone() codec.Parameters {
	if p == nil {
		return (*JPEGLosslessParameters)(nil)
	}
	clone := *p
	clone.params = make(map[string]interface{}, len(p.params))
	for key, value := range p.params {
		clone.params[key] = value
	}
	return &clone
}

// GetParameter retrieves a parameter by name (implements codec.Parameters)
func (p *JPEGLosslessParameters) GetParameter(name string) interface{} {
	switch name {
	case "predictor":
		return p.Predictor
	default:
		// Check custom parameters
		return p.params[name]
	}
}

// SetParameter sets a parameter value (implements codec.Parameters)
func (p *JPEGLosslessParameters) SetParameter(name string, value interface{}) {
	switch name {
	case "predictor":
		if v, ok := value.(int); ok {
			p.Predictor = v
		}
	default:
		// Store as custom parameter
		p.params[name] = value
	}
}

// Validate checks if the parameters are valid and adjusts them if needed
func (p *JPEGLosslessParameters) Validate() error {
	// Predictor must be in range 0-7
	if p.Predictor < 0 || p.Predictor > 7 {
		p.Predictor = 0 // Reset to default (auto-select)
	}
	return nil
}

// WithPredictor sets the predictor and returns the parameters for chaining
func (p *JPEGLosslessParameters) WithPredictor(predictor int) *JPEGLosslessParameters {
	p.Predictor = predictor
	return p
}
