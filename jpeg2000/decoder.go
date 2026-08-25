package jpeg2000

import (
	"github.com/cocosip/go-dicom-codecs/jpeg2000/internal/common/codestream"
	"github.com/cocosip/go-dicom-codecs/jpeg2000/openjpeg"
)

// Decoder decodes classic JPEG 2000 codestreams with the OpenJPEG engine.
type Decoder = openjpeg.Decoder

// NewDecoder creates a classic JPEG 2000 decoder.
func NewDecoder() *Decoder {
	return openjpeg.NewDecoder()
}

// TileLayout describes the tile geometry derived from a SIZ segment.
type TileLayout = openjpeg.TileLayout

// TileAssembler reconstructs image samples from decoded tiles.
type TileAssembler = openjpeg.TileAssembler

// NewTileLayout creates a tile layout from a SIZ segment.
func NewTileLayout(siz *codestream.SIZSegment) *TileLayout {
	return openjpeg.NewTileLayout(siz)
}

// NewTileAssembler creates a tile assembler from a SIZ segment.
func NewTileAssembler(siz *codestream.SIZSegment) *TileAssembler {
	return openjpeg.NewTileAssembler(siz)
}
