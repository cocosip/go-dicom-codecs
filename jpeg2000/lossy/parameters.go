package lossy

import (
	"github.com/cocosip/go-dicom-codec/jpeg2000"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
)

// Ensure JPEG2000LossyParameters implements codec.Parameters
var _ codec.Parameters = (*JPEG2000LossyParameters)(nil)

// JPEG2000LossyParameters contains parameters for JPEG 2000 lossy compression.
type JPEG2000LossyParameters struct {
	// Irreversible controls wavelet type:
	// true = irreversible 9/7 (lossy), false = reversible 5/3.
	Irreversible bool

	// Rate is the fo-dicom/OpenJPEG target compression ratio. Higher values
	// request stronger compression. Default: 20.
	// When TargetRatio > 0, PCRD rate control is used and Rate is only a base
	// layer-ladder selector.
	Rate int

	// RateLevels is the fo-dicom/OpenJPEG layer ladder used with Rate.
	// Default: [1280, 640, 320, 160, 80, 40, 20, 10, 5].
	RateLevels []int

	// IsVerbose enables verbose logging in wrappers that support it.
	IsVerbose bool

	// AllowMCT enables multiple component transform for RGB input.
	AllowMCT bool

	// UpdatePhotometricInterpretation is kept for fo-dicom parameter compatibility.
	UpdatePhotometricInterpretation bool

	// EncodeSignedPixelValuesAsUnsigned is kept for fo-dicom parameter compatibility.
	EncodeSignedPixelValuesAsUnsigned bool

	// NumLevels specifies the number of wavelet decomposition levels (0-6).
	// More levels = better compression but slower encoding/decoding. Default: 5.
	NumLevels int

	// NumLayers controls JPEG 2000 quality layers (progressive refinement). Default: 1.
	NumLayers int

	// TargetRatio optionally requests a compression ratio (orig_size / compressed_size).
	// If >0, it overrides Rate-derived target ratio.
	TargetRatio float64

	// QuantStepScale scales the quantization step derived from rate ratio (>1 = more compression).
	// Default: 1.0 (no scaling).
	QuantStepScale float64

	// SubbandSteps allows explicit per-subbands quantization steps (lossy). Length must be 3*NumLevels+1 when set.
	SubbandSteps []float64

	// internal storage for compatibility with generic parameter interface
	params map[string]interface{}
}

var defaultRateLevels = []int{1280, 640, 320, 160, 80, 40, 20, 10, 5}

// NewLossyParameters creates a new JPEG2000LossyParameters with default values.
func NewLossyParameters() *JPEG2000LossyParameters {
	levels := make([]int, len(defaultRateLevels))
	copy(levels, defaultRateLevels)

	return &JPEG2000LossyParameters{
		Irreversible:                      true,
		Rate:                              20,
		RateLevels:                        levels,
		IsVerbose:                         false,
		AllowMCT:                          true,
		UpdatePhotometricInterpretation:   true,
		EncodeSignedPixelValuesAsUnsigned: false,
		NumLevels:                         5, // Default 5 decomposition levels
		NumLayers:                         1,
		TargetRatio:                       0,
		QuantStepScale:                    1.0,
		SubbandSteps:                      nil,
		params:                            make(map[string]interface{}),
	}
}

// GetParameter retrieves a parameter by name (implements codec.Parameters).
func (p *JPEG2000LossyParameters) GetParameter(name string) interface{} {
	switch name {
	case "irreversible":
		return p.Irreversible
	case "rate":
		return p.Rate
	case "rateLevels":
		return p.RateLevels
	case "isVerbose":
		return p.IsVerbose
	case "allowMCT":
		return p.AllowMCT
	case "updatePhotometricInterpretation":
		return p.UpdatePhotometricInterpretation
	case "encodeSignedPixelValuesAsUnsigned":
		return p.EncodeSignedPixelValuesAsUnsigned
	case "numLevels":
		return p.NumLevels
	case "numLayers":
		return p.NumLayers
	case "targetRatio":
		return p.TargetRatio
	case "quantStepScale":
		return p.QuantStepScale
	case "subbandSteps":
		return p.SubbandSteps
	default:
		return p.params[name]
	}
}

// SetParameter sets a parameter value (implements codec.Parameters).
func (p *JPEG2000LossyParameters) SetParameter(name string, value interface{}) {
	switch name {
	case "irreversible":
		if v, ok := value.(bool); ok {
			p.Irreversible = v
		}
	case "rate":
		if v, ok := value.(int); ok {
			p.Rate = v
		}
	case "rateLevels":
		if v, ok := value.([]int); ok {
			p.RateLevels = v
		}
	case "isVerbose":
		if v, ok := value.(bool); ok {
			p.IsVerbose = v
		}
	case "allowMCT":
		if v, ok := value.(bool); ok {
			p.AllowMCT = v
		}
	case "updatePhotometricInterpretation":
		if v, ok := value.(bool); ok {
			p.UpdatePhotometricInterpretation = v
		}
	case "encodeSignedPixelValuesAsUnsigned":
		if v, ok := value.(bool); ok {
			p.EncodeSignedPixelValuesAsUnsigned = v
		}
	case "numLevels":
		if v, ok := value.(int); ok {
			p.NumLevels = v
		}
	case "numLayers":
		if v, ok := value.(int); ok {
			p.NumLayers = v
		}
	case "targetRatio":
		if v, ok := value.(float64); ok {
			p.TargetRatio = v
		}
	case "quantStepScale":
		switch v := value.(type) {
		case float64:
			p.QuantStepScale = v
		case float32:
			p.QuantStepScale = float64(v)
		}
	case "subbandSteps":
		if v, ok := value.([]float64); ok {
			p.SubbandSteps = v
		}
	default:
		p.params[name] = value
	}
}

// Validate checks if the parameters are valid and normalizes values.
func (p *JPEG2000LossyParameters) Validate() error {
	if p.Rate <= 0 {
		p.Rate = 20
	}
	if len(p.RateLevels) == 0 {
		p.RateLevels = make([]int, len(defaultRateLevels))
		copy(p.RateLevels, defaultRateLevels)
	}
	if p.NumLevels < 0 || p.NumLevels > 6 {
		p.NumLevels = 5
	}
	if p.NumLayers < 1 {
		p.NumLayers = 1
	}
	if p.QuantStepScale <= 0 {
		p.QuantStepScale = 1.0
	}
	return nil
}

// WithIrreversible sets wavelet mode and returns the parameters for chaining.
func (p *JPEG2000LossyParameters) WithIrreversible(irreversible bool) *JPEG2000LossyParameters {
	p.Irreversible = irreversible
	return p
}

// WithRate sets fo-dicom/OpenJPEG style rate and returns the parameters for chaining.
func (p *JPEG2000LossyParameters) WithRate(rate int) *JPEG2000LossyParameters {
	p.Rate = rate
	return p
}

// WithRateLevels sets fo-dicom/OpenJPEG style rate levels and returns the parameters for chaining.
func (p *JPEG2000LossyParameters) WithRateLevels(levels []int) *JPEG2000LossyParameters {
	p.RateLevels = levels
	return p
}

// WithAllowMCT sets whether MCT is enabled and returns the parameters for chaining.
func (p *JPEG2000LossyParameters) WithAllowMCT(allow bool) *JPEG2000LossyParameters {
	p.AllowMCT = allow
	return p
}

// WithNumLevels sets the number of wavelet levels and returns the parameters for chaining.
func (p *JPEG2000LossyParameters) WithNumLevels(levels int) *JPEG2000LossyParameters {
	p.NumLevels = levels
	return p
}

// WithNumLayers sets the number of quality layers and returns the parameters for chaining.
func (p *JPEG2000LossyParameters) WithNumLayers(layers int) *JPEG2000LossyParameters {
	p.NumLayers = layers
	return p
}

// WithTargetRatio sets the desired compression ratio and returns the parameters for chaining.
// ratio = original_size / compressed_size (e.g., 5 means target ~5:1 compression).
func (p *JPEG2000LossyParameters) WithTargetRatio(ratio float64) *JPEG2000LossyParameters {
	p.TargetRatio = ratio
	return p
}

// WithQuantStepScale sets the global quantization step scale (>1 increases compression)
// and returns the parameters for chaining.
func (p *JPEG2000LossyParameters) WithQuantStepScale(scale float64) *JPEG2000LossyParameters {
	p.QuantStepScale = scale
	return p
}

// WithSubbandSteps sets explicit per-subbands quantization steps and returns the parameters for chaining.
func (p *JPEG2000LossyParameters) WithSubbandSteps(steps []float64) *JPEG2000LossyParameters {
	p.SubbandSteps = steps
	return p
}

// WithMCTBindings sets multi-component transform bindings.
func (p *JPEG2000LossyParameters) WithMCTBindings(bindings []jpeg2000.MCTBindingParams) *JPEG2000LossyParameters {
	p.SetParameter("mctBindings", bindings)
	return p
}
