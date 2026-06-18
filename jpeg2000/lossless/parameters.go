package lossless

import (
	"github.com/cocosip/go-dicom-codec/jpeg2000"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

// Ensure JPEG2000LosslessParameters implements codec.Parameters
var _ codec.Parameters = (*JPEG2000LosslessParameters)(nil)

// JPEG2000LosslessParameters contains parameters for JPEG 2000 Lossless compression
type JPEG2000LosslessParameters struct {
	// NumLevels controls the number of wavelet decomposition levels (0-6)
	// - 0: No decomposition (minimal compression, fastest)
	// - 1: Single-level decomposition
	// - 3: Medium levels (good balance)
	// - 5: Default, recommended for most images
	// - 6: Maximum levels (best compression for large images)
	//
	// More levels generally provide better compression but require more computation.
	// For small images (< 128x128), use fewer levels (1-3).
	// For large images (>= 512x512), use more levels (5-6).
	NumLevels int

	// AllowMCT controls whether a multi-component transform (RCT/ICT or custom MCT) is applied.
	// OpenJPEG enables MCT for RGB only when AllowMCT is true.
	AllowMCT bool

	// Rate is the fo-dicom/OpenJPEG style target rate parameter.
	// Effective target ratio ~= Rate * BitsStored / BitsAllocated. Default: 20.
	Rate int

	// RateLevels is the fo-dicom/OpenJPEG layer ladder used with Rate.
	// Default: [1280, 640, 320, 160, 80, 40, 20, 10, 5].
	RateLevels []int

	// ProgressionOrder mirrors the JPEG 2000 progression order (0=LRCP,1=RLCP,2=RPCL,3=PCRL,4=CPRL).
	ProgressionOrder uint8

	// NumLayers controls the number of quality layers. For lossless, OpenJPEG may emit multiple layers
	// before a final lossless layer when rate control is requested.
	NumLayers int

	// TargetRatio requests a target compression ratio; used by PCRD when >0.
	TargetRatio float64

	// UsePCRDOpt toggles PCRD-style layer allocation when layering/TargetRatio is requested.
	UsePCRDOpt bool

	// AppendLosslessLayer adds a final lossless layer (rate=0) after target-rate layers,
	// mirroring OpenJPEG behavior when Rate>0 in lossless syntax.
	AppendLosslessLayer bool

	// internal storage for compatibility with generic parameter interface
	params map[string]interface{}
}

var defaultRateLevels = []int{1280, 640, 320, 160, 80, 40, 20, 10, 5}

// NewLosslessParameters creates a new JPEG2000LosslessParameters with default values
func NewLosslessParameters() *JPEG2000LosslessParameters {
	levels := make([]int, len(defaultRateLevels))
	copy(levels, defaultRateLevels)
	return &JPEG2000LosslessParameters{
		NumLevels:           5, // Default 5 decomposition levels (recommended)
		AllowMCT:            true,
		Rate:                20,
		RateLevels:          levels,
		ProgressionOrder:    0, // LRCP
		NumLayers:           1, // Single layer by default
		TargetRatio:         0, // No target ratio
		UsePCRDOpt:          false,
		AppendLosslessLayer: true,
		params:              make(map[string]interface{}),
	}
}

// GetParameter retrieves a parameter by name (implements codec.Parameters)
func (p *JPEG2000LosslessParameters) GetParameter(name string) interface{} {
	switch name {
	case "numLevels":
		return p.NumLevels
	case "allowMCT":
		return p.AllowMCT
	case "rate":
		return p.Rate
	case "rateLevels":
		return p.RateLevels
	case "progressionOrder":
		return p.ProgressionOrder
	case "numLayers":
		return p.NumLayers
	case "targetRatio":
		return p.TargetRatio
	case "usePCRDOpt":
		return p.UsePCRDOpt
	case "appendLosslessLayer":
		return p.AppendLosslessLayer
	default:
		// Check custom parameters
		return p.params[name]
	}
}

// SetParameter sets a parameter value (implements codec.Parameters)
func (p *JPEG2000LosslessParameters) SetParameter(name string, value interface{}) {
	switch name {
	case "numLevels":
		if v, ok := value.(int); ok {
			p.NumLevels = v
		}
	case "allowMCT":
		if v, ok := value.(bool); ok {
			p.AllowMCT = v
		}
	case "rate":
		if v, ok := value.(int); ok {
			p.Rate = v
		}
	case "rateLevels":
		if v, ok := value.([]int); ok {
			p.RateLevels = v
		}
	case "progressionOrder":
		switch v := value.(type) {
		case uint8:
			p.ProgressionOrder = v
		case int:
			if v >= 0 {
				p.ProgressionOrder = uint8(v)
			}
		}
	case "numLayers":
		if v, ok := value.(int); ok {
			p.NumLayers = v
		}
	case "targetRatio":
		switch v := value.(type) {
		case float64:
			p.TargetRatio = v
		case float32:
			p.TargetRatio = float64(v)
		case int:
			p.TargetRatio = float64(v)
		}
	case "usePCRDOpt":
		if v, ok := value.(bool); ok {
			p.UsePCRDOpt = v
		}
	case "appendLosslessLayer":
		if v, ok := value.(bool); ok {
			p.AppendLosslessLayer = v
		}
	default:
		// Store as custom parameter
		p.params[name] = value
	}
}

// Validate checks if the parameters are valid and adjusts them if needed
func (p *JPEG2000LosslessParameters) Validate() error {
	// NumLevels must be in range 0-6
	if p.NumLevels < 0 || p.NumLevels > 6 {
		p.NumLevels = 5 // Reset to default
	}
	if p.NumLayers < 1 {
		p.NumLayers = 1
	}
	if p.Rate < 0 {
		p.Rate = 0
	}
	if p.Rate > 0 && len(p.RateLevels) == 0 {
		p.RateLevels = make([]int, len(defaultRateLevels))
		copy(p.RateLevels, defaultRateLevels)
	}
	if p.ProgressionOrder > 4 {
		p.ProgressionOrder = 0
	}
	if p.TargetRatio < 0 {
		p.TargetRatio = 0
	}
	if p.AppendLosslessLayer && p.NumLayers < 2 && p.TargetRatio > 0 {
		// Ensure at least two layers when requesting a final lossless layer with a target ratio
		p.NumLayers = 2
	}
	return nil
}

// WithNumLevels sets the number of decomposition levels and returns the parameters for chaining
func (p *JPEG2000LosslessParameters) WithNumLevels(numLevels int) *JPEG2000LosslessParameters {
	p.NumLevels = numLevels
	return p
}

// WithAllowMCT toggles default MCT (RCT/ICT or custom matrix application)
func (p *JPEG2000LosslessParameters) WithAllowMCT(allow bool) *JPEG2000LosslessParameters {
	p.AllowMCT = allow
	return p
}

// WithRate sets fo-dicom/OpenJPEG style rate and returns the parameters for chaining.
func (p *JPEG2000LosslessParameters) WithRate(rate int) *JPEG2000LosslessParameters {
	p.Rate = rate
	return p
}

// WithRateLevels sets fo-dicom/OpenJPEG style rate levels and returns the parameters for chaining.
func (p *JPEG2000LosslessParameters) WithRateLevels(levels []int) *JPEG2000LosslessParameters {
	p.RateLevels = levels
	return p
}

// WithProgression sets the progression order (0-4)
func (p *JPEG2000LosslessParameters) WithProgression(order uint8) *JPEG2000LosslessParameters {
	p.ProgressionOrder = order
	return p
}

// WithNumLayers sets number of quality layers
func (p *JPEG2000LosslessParameters) WithNumLayers(layers int) *JPEG2000LosslessParameters {
	p.NumLayers = layers
	return p
}

// WithTargetRatio sets target compression ratio
func (p *JPEG2000LosslessParameters) WithTargetRatio(ratio float64) *JPEG2000LosslessParameters {
	p.TargetRatio = ratio
	return p
}

// WithPCRD enables PCRD optimization flag
func (p *JPEG2000LosslessParameters) WithPCRD(enable bool) *JPEG2000LosslessParameters {
	p.UsePCRDOpt = enable
	return p
}

// WithAppendLosslessLayer toggles adding a final lossless layer (rate=0)
func (p *JPEG2000LosslessParameters) WithAppendLosslessLayer(enable bool) *JPEG2000LosslessParameters {
	p.AppendLosslessLayer = enable
	return p
}

// WithMCTBindings sets multi-component transform bindings.
func (p *JPEG2000LosslessParameters) WithMCTBindings(bindings []jpeg2000.MCTBindingParams) *JPEG2000LosslessParameters {
	p.SetParameter("mctBindings", bindings)
	return p
}
