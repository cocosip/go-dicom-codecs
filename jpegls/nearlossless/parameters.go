package nearlossless

import (
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

// Ensure JPEGLSNearLosslessParameters implements codec.Parameters
var _ codec.Parameters = (*JPEGLSNearLosslessParameters)(nil)

const paramNear = "near"

// JPEGLSNearLosslessParameters contains parameters for JPEG-LS Near-Lossless compression
type JPEGLSNearLosslessParameters struct {
	// NEAR controls the maximum allowed error per pixel (0-255)
	// - 0:   Lossless (no error, perfect reconstruction)
	// - 1:   Near-lossless, max error ±1 per pixel
	// - 2:   Near-lossless, max error ±2 per pixel (default)
	// - 3-5: Low error, good compression
	// - 10:  Medium error, higher compression
	// - 20+: High error, maximum compression
	NEAR int

	// internal storage for compatibility with generic parameter interface
	params map[string]interface{}
}

// NewNearLosslessParameters creates a new JPEGLSNearLosslessParameters with default values
func NewNearLosslessParameters() *JPEGLSNearLosslessParameters {
	return &JPEGLSNearLosslessParameters{
		NEAR:   3, // Default near-lossless error bound used by fo-dicom
		params: make(map[string]interface{}),
	}
}

// Clone returns an independent copy of the parameters.
func (p *JPEGLSNearLosslessParameters) Clone() codec.Parameters {
	if p == nil {
		return (*JPEGLSNearLosslessParameters)(nil)
	}
	clone := *p
	clone.params = make(map[string]interface{}, len(p.params))
	for key, value := range p.params {
		clone.params[key] = value
	}
	return &clone
}

// GetParameter retrieves a parameter by name (implements codec.Parameters)
func (p *JPEGLSNearLosslessParameters) GetParameter(name string) interface{} {
	switch name {
	case paramNear:
		return p.NEAR
	default:
		// Check custom parameters
		return p.params[name]
	}
}

// SetParameter sets a parameter value (implements codec.Parameters)
func (p *JPEGLSNearLosslessParameters) SetParameter(name string, value interface{}) {
	switch name {
	case paramNear:
		if v, ok := value.(int); ok {
			p.NEAR = v
		}
	default:
		// Store as custom parameter
		p.params[name] = value
	}
}

// Validate checks if the parameters are valid and adjusts them if needed
func (p *JPEGLSNearLosslessParameters) Validate() error {
	// NEAR must be in range 0-255
	if p.NEAR < 0 || p.NEAR > 255 {
		p.NEAR = 3 // Reset to default
	}
	return nil
}

// WithNEAR sets the NEAR parameter and returns the parameters for chaining
func (p *JPEGLSNearLosslessParameters) WithNEAR(near int) *JPEGLSNearLosslessParameters {
	p.NEAR = near
	return p
}
