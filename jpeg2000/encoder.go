package jpeg2000

import "github.com/cocosip/go-dicom-codecs/jpeg2000/openjpeg"

// EncodeParams configures classic JPEG 2000 encoding.
type EncodeParams = openjpeg.EncodeParams

// MCTBindingParams configures a multi-component transform binding.
type MCTBindingParams = openjpeg.MCTBindingParams

// Encoder encodes classic JPEG 2000 codestreams with the OpenJPEG engine.
type Encoder = openjpeg.Encoder

// DefaultEncodeParams returns the default classic JPEG 2000 encoder parameters.
func DefaultEncodeParams(width, height, components, bitDepth int, isSigned bool) *EncodeParams {
	return openjpeg.DefaultEncodeParams(width, height, components, bitDepth, isSigned)
}

// NewEncoder creates a classic JPEG 2000 encoder.
func NewEncoder(params *EncodeParams) *Encoder {
	return openjpeg.NewEncoder(params)
}

// MCTBindingBuilder builds Part 2 multi-component transform bindings.
type MCTBindingBuilder = openjpeg.MCTBindingBuilder

// NewMCTBinding creates a multi-component transform binding builder.
func NewMCTBinding() *MCTBindingBuilder {
	return openjpeg.NewMCTBinding()
}

// QuantizationParams describes JPEG 2000 quantization state.
type QuantizationParams = openjpeg.QuantizationParams

// Quantization helpers expose the classic OpenJPEG engine calculations.
var (
	OpenJPEGRuntimeQuantizationSteps    = openjpeg.RuntimeQuantizationSteps
	CalculateQuantizationParams         = openjpeg.CalculateQuantizationParams
	CalculateOpenJPEGQuantizationParams = openjpeg.CalculateOpenJPEGQuantizationParams
	DecodeQuantizationStep              = openjpeg.DecodeQuantizationStep
	QuantizeCoefficients                = openjpeg.QuantizeCoefficients
	DequantizeCoefficients              = openjpeg.DequantizeCoefficients
)

// LayerAllocation describes code-block pass allocation across quality layers.
type LayerAllocation = openjpeg.LayerAllocation

// CodeBlockContribution describes one code-block's layer contribution.
type CodeBlockContribution = openjpeg.CodeBlockContribution

// PacketRateMeasurer measures packet bytes for rate allocation.
type PacketRateMeasurer = openjpeg.PacketRateMeasurer

// Layer allocation helpers expose the classic OpenJPEG engine algorithms.
var (
	AllocateLayersSimple                    = openjpeg.AllocateLayersSimple
	AllocateLayersRateDistortion            = openjpeg.AllocateLayersRateDistortion
	AllocateLayersRateDistortionPasses      = openjpeg.AllocateLayersRateDistortionPasses
	FindOptimalLambda                       = openjpeg.FindOptimalLambda
	ComputeLayerBudgets                     = openjpeg.ComputeLayerBudgets
	AllocateLayersWithLambda                = openjpeg.AllocateLayersWithLambda
	AllocateLayersOpenJPEGThreshold         = openjpeg.AllocateLayersOpenJPEGThreshold
	AllocateLayersOpenJPEGThresholdMeasured = openjpeg.AllocateLayersOpenJPEGThresholdMeasured
	MeasureOpenJPEGLayerSelectionBytes      = openjpeg.MeasureOpenJPEGLayerSelectionBytes
	PacketPayloadLen                        = openjpeg.PacketPayloadLen
)

// ROIParams configures a rectangular region of interest.
type ROIParams = openjpeg.ROIParams

// ROIStyle identifies a JPEG 2000 region-of-interest coding style.
type ROIStyle = openjpeg.ROIStyle

// ROIShape identifies a supported region-of-interest shape.
type ROIShape = openjpeg.ROIShape

// RoiRect describes a region-of-interest rectangle.
type RoiRect = openjpeg.RoiRect

// ROIRegion describes one configured region of interest.
type ROIRegion = openjpeg.ROIRegion

// ROIConfig configures region-of-interest encoding.
type ROIConfig = openjpeg.ROIConfig

// Point is a two-dimensional integer point used by ROI polygons.
type Point = openjpeg.Point

// ROI style and shape constants mirror the classic OpenJPEG engine.
const (
	ROIStyleMaxShift       = openjpeg.ROIStyleMaxShift
	ROIStyleGeneralScaling = openjpeg.ROIStyleGeneralScaling
	ROIShapeRectangle      = openjpeg.ROIShapeRectangle
	ROIShapePolygon        = openjpeg.ROIShapePolygon
	ROIShapeMask           = openjpeg.ROIShapeMask
)
