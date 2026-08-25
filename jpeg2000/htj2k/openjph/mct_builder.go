package openjph

// MCTBindingBuilder builds MCTBindingParams in a fluent style.
type MCTBindingBuilder struct {
	b MCTBindingParams
}

// NewMCTBinding creates a new builder for MCTBindingParams.
func NewMCTBinding() *MCTBindingBuilder { return &MCTBindingBuilder{b: MCTBindingParams{}} }

// Assoc sets association type for the binding.
func (x *MCTBindingBuilder) Assoc(t uint8) *MCTBindingBuilder { x.b.AssocType = t; return x }

// Components sets component IDs affected by the transform.
func (x *MCTBindingBuilder) Components(ids []uint16) *MCTBindingBuilder {
	x.b.ComponentIDs = ids
	return x
}

// Matrix sets the forward transform matrix.
func (x *MCTBindingBuilder) Matrix(m [][]float64) *MCTBindingBuilder { x.b.Matrix = m; return x }

// Inverse sets the inverse transform matrix.
func (x *MCTBindingBuilder) Inverse(m [][]float64) *MCTBindingBuilder { x.b.Inverse = m; return x }

// Offsets sets per-component offsets.
func (x *MCTBindingBuilder) Offsets(o []int32) *MCTBindingBuilder { x.b.Offsets = o; return x }

// ElementType sets the element type used in arrays.
func (x *MCTBindingBuilder) ElementType(t uint8) *MCTBindingBuilder { x.b.ElementType = t; return x }

// MCOPrecision sets precision for MCO segment.
func (x *MCTBindingBuilder) MCOPrecision(p uint8) *MCTBindingBuilder { x.b.MCOPrecision = p; return x }

// NormScale sets normalization scale factor.
func (x *MCTBindingBuilder) NormScale(s float64) *MCTBindingBuilder { x.b.MCONormScale = s; return x }

// RecordOrder sets record order for MCT buffers.
func (x *MCTBindingBuilder) RecordOrder(order []uint8) *MCTBindingBuilder {
	x.b.MCTRecordOrder = order
	return x
}

// Build returns the constructed MCTBindingParams.
func (x *MCTBindingBuilder) Build() MCTBindingParams { return x.b }
