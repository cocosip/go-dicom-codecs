// Copyright (c) 2025 go-dicom contributors.
// Licensed under the Microsoft Public License (MS-PL).

// Package rle implements DICOM RLE Lossless compression.
package rle

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"unsafe"

	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/imaging/codec"
	"github.com/cocosip/go-dicom/pkg/imaging/pixel"
)

var _ codec.Codec = (*Codec)(nil)

// Codec implements DICOM RLE (Run-Length Encoding) compression and decompression.
// This codec conforms to DICOM Part 5, Annex G: RLE Compression.
type Codec struct{}

// NewRLECodec creates a new RLE codec instance.
func NewRLECodec() *Codec { return &Codec{} }

// Name returns the codec name.
func (c *Codec) Name() string { return "RLE Lossless" }

// TransferSyntax returns the RLE Lossless transfer syntax.
func (c *Codec) TransferSyntax() *transfer.Syntax { return transfer.RLELossless }

// DefaultParameters returns default parameters for this codec.
func (c *Codec) DefaultParameters() codec.Parameters { return codec.NoParameters{} }

// Encode encodes pixel data from oldPixelData to newPixelData.
func (c *Codec) Encode(ctx context.Context, oldPixelData codec.FrameSource, newPixelData codec.FrameSink, parameters codec.Parameters) error {
	if oldPixelData == nil || newPixelData == nil {
		return fmt.Errorf("source and destination pixel data must not be nil")
	}

	frameInfo := oldPixelData.FrameInfo()
	frameCount := oldPixelData.FrameCount()
	for i := 0; i < frameCount; i++ {
		srcFrame, err := oldPixelData.Frame(ctx, i)
		if err != nil {
			return fmt.Errorf("failed to get frame %d: %w", i, err)
		}

		var dstFrame []byte
		if err := c.encodeFrame(srcFrame, &dstFrame, frameInfo, parameters); err != nil {
			return fmt.Errorf("failed to encode frame %d: %w", i, err)
		}
		if err := newPixelData.AddFrame(ctx, dstFrame); err != nil {
			return fmt.Errorf("failed to add frame %d: %w", i, err)
		}
	}
	return nil
}

// Decode decodes pixel data from oldPixelData to newPixelData.
func (c *Codec) Decode(ctx context.Context, oldPixelData codec.FrameSource, newPixelData codec.FrameSink, parameters codec.Parameters) error {
	if oldPixelData == nil || newPixelData == nil {
		return fmt.Errorf("source and destination pixel data must not be nil")
	}

	frameInfo := oldPixelData.FrameInfo()
	frameCount := oldPixelData.FrameCount()
	for i := 0; i < frameCount; i++ {
		srcFrame, err := oldPixelData.Frame(ctx, i)
		if err != nil {
			return fmt.Errorf("failed to get frame %d: %w", i, err)
		}

		var dstFrame []byte
		if err := c.decodeFrame(srcFrame, &dstFrame, frameInfo, parameters); err != nil {
			return fmt.Errorf("failed to decode frame %d: %w", i, err)
		}
		if err := newPixelData.AddFrame(ctx, dstFrame); err != nil {
			return fmt.Errorf("failed to add frame %d: %w", i, err)
		}
	}
	return nil
}

// segmentLayout returns the first source position and the stride between
// consecutive samples of an RLE segment.
func segmentLayout(segment, bytesAllocated, samples, pixelCount int, interleaved bool) (pos, stride int) {
	sample := segment / bytesAllocated
	sabyte := segment % bytesAllocated
	if interleaved {
		pos = sample * bytesAllocated
		stride = bytesAllocated * samples
	} else {
		pos = sample * bytesAllocated * pixelCount
		stride = bytesAllocated
	}
	pos += bytesAllocated - sabyte - 1
	return pos, stride
}

func (c *Codec) encodeFrame(src []byte, dst *[]byte, info codec.FrameInfo, _ codec.Parameters) error {
	if len(src) == 0 {
		return fmt.Errorf("source frame data must not be empty")
	}

	pixelCount := int(info.Width) * int(info.Height)
	if pixelCount <= 0 {
		return fmt.Errorf("invalid frame dimensions %dx%d", info.Width, info.Height)
	}
	bitsAllocated := int(info.BitDepth.BitsAllocated)
	if bitsAllocated <= 0 {
		return fmt.Errorf("invalid bits allocated: %d", bitsAllocated)
	}
	bytesAllocated := (bitsAllocated + 7) / 8
	samples := int(info.SamplesPerPixel)
	numberOfSegments := bytesAllocated * samples
	if numberOfSegments < 1 || numberOfSegments > 15 {
		return fmt.Errorf("invalid number of RLE segments: %d", numberOfSegments)
	}
	isInterleaved := info.PlanarConfiguration == pixel.InterleavedPlanar

	// Validate every segment once; the encoder loop can then skip per-pixel
	// bounds checks.
	for s := 0; s < numberOfSegments; s++ {
		pos, stride := segmentLayout(s, bytesAllocated, samples, pixelCount, isInterleaved)
		end := pos + (pixelCount-1)*stride
		if end >= len(src) {
			return fmt.Errorf("read position %d exceeds frame buffer length %d", end, len(src))
		}
	}

	// One literal control byte per 128 pixels is the worst case, plus a little
	// room for segment padding and replicate controls.
	capacity := 64 + numberOfSegments*(pixelCount+(pixelCount+127)/128+2) + 2
	encoder := newRLEEncoderWithCapacity(capacity)

	for s := 0; s < numberOfSegments; s++ {
		pos, stride := segmentLayout(s, bytesAllocated, samples, pixelCount, isInterleaved)
		encoder.NextSegment()
		encoder.EncodeSegment(src, pos, stride, pixelCount)
		encoder.Flush()
	}

	encoder.MakeEvenLength()
	*dst = encoder.GetBuffer()
	return nil
}

func (c *Codec) decodeFrame(src []byte, dst *[]byte, info codec.FrameInfo, _ codec.Parameters) error {
	if len(src) == 0 {
		return fmt.Errorf("source frame data must not be empty")
	}

	pixelCount := int(info.Width) * int(info.Height)
	if pixelCount <= 0 {
		return fmt.Errorf("invalid frame dimensions %dx%d", info.Width, info.Height)
	}
	bitsAllocated := int(info.BitDepth.BitsAllocated)
	if bitsAllocated <= 0 {
		return fmt.Errorf("invalid bits allocated: %d", bitsAllocated)
	}
	bytesAllocated := (bitsAllocated + 7) / 8
	samples := int(info.SamplesPerPixel)
	numberOfSegments := bytesAllocated * samples
	if numberOfSegments < 1 || numberOfSegments > 15 {
		return fmt.Errorf("invalid number of RLE segments: %d", numberOfSegments)
	}
	isInterleaved := info.PlanarConfiguration == pixel.InterleavedPlanar

	frameSize := bytesAllocated * samples * int(info.Width) * int(info.Height)
	if (frameSize & 1) == 1 {
		frameSize++
	}
	frameData := make([]byte, frameSize)

	decoder, err := newRLEDecoder(src)
	if err != nil {
		return fmt.Errorf("failed to create RLE decoder: %w", err)
	}
	if decoder.NumberOfSegments != numberOfSegments {
		return fmt.Errorf("unexpected number of RLE segments: got %d, expected %d", decoder.NumberOfSegments, numberOfSegments)
	}

	for s := 0; s < numberOfSegments; s++ {
		pos, stride := segmentLayout(s, bytesAllocated, samples, pixelCount, isInterleaved)
		if pos+(pixelCount-1)*stride >= frameSize {
			return fmt.Errorf("RLE segment %d exceeds output buffer length %d", s, frameSize)
		}
		if err := decoder.DecodeSegment(s, frameData, pos, stride); err != nil {
			return fmt.Errorf("failed to decode segment %d: %w", s, err)
		}
	}

	*dst = frameData
	return nil
}

type rleEncoder struct {
	count     int
	offsets   [15]uint32
	out       []byte
	prevByte  int
	repeatCnt int
	litCtrl   int
	litLen    int
}

func newRLEEncoder() *rleEncoder { return newRLEEncoderWithCapacity(0) }

func newRLEEncoderWithCapacity(capacity int) *rleEncoder {
	if capacity < 64 {
		capacity = 64
	}
	return &rleEncoder{
		out:      make([]byte, 64, capacity),
		prevByte: -1,
		litCtrl:  -1,
	}
}

func (e *rleEncoder) NextSegment() {
	e.Flush()
	if (len(e.out) & 1) == 1 {
		e.out = append(e.out, 0x00)
	}
	e.offsets[e.count] = uint32(len(e.out))
	e.count++
}

func (e *rleEncoder) Encode(b byte) {
	if int(b) == e.prevByte {
		e.repeatCnt++
		return
	}
	e.emitRun(e.prevByte, e.repeatCnt, false)
	e.prevByte = int(b)
	e.repeatCnt = 1
}

// EncodeSegment emits all runs of one segment except the final run, which is
// left pending for Flush. src[start], src[start+stride], ... are the samples.
func (e *rleEncoder) EncodeSegment(src []byte, start, stride, count int) {
	if count <= 0 {
		return
	}
	prev := src[start]
	run := 1
	pos := start + stride
	for p := 1; p < count; p++ {
		b := src[pos]
		if b == prev {
			run++
		} else {
			e.emitRun(int(prev), run, false)
			prev = b
			run = 1
		}
		pos += stride
	}
	e.prevByte = int(prev)
	e.repeatCnt = run
}

func (e *rleEncoder) emitRun(b, run int, atEnd bool) {
	if run <= 0 {
		return
	}
	if !atEnd && run < 3 {
		e.appendLiteral(byte(b), run)
		return
	}
	if atEnd && run == 1 {
		e.appendLiteral(byte(b), 1)
		return
	}

	e.closeLiteral()
	for run > 128 {
		e.emitReplicate(byte(b), 128)
		run -= 128
	}
	if run >= 2 && (atEnd || run >= 3) {
		e.emitReplicate(byte(b), run)
	} else if run > 0 {
		e.appendLiteral(byte(b), run)
	}
}

func (e *rleEncoder) appendLiteral(b byte, count int) {
	for i := 0; i < count; i++ {
		if e.litLen == 128 {
			e.out[e.litCtrl] = 127
			e.litCtrl = -1
			e.litLen = 0
		}
		if e.litCtrl < 0 {
			e.litCtrl = len(e.out)
			e.out = append(e.out, 0)
		}
		e.out = append(e.out, b)
		e.litLen++
	}
}

func (e *rleEncoder) closeLiteral() {
	if e.litCtrl >= 0 && e.litLen > 0 {
		e.out[e.litCtrl] = byte(e.litLen - 1)
	}
	e.litCtrl = -1
	e.litLen = 0
}

func (e *rleEncoder) emitReplicate(b byte, count int) {
	e.out = append(e.out, byte(257-count), b)
}

func (e *rleEncoder) Flush() {
	e.emitRun(e.prevByte, e.repeatCnt, true)
	e.closeLiteral()
	e.prevByte = -1
	e.repeatCnt = 0
}

func (e *rleEncoder) MakeEvenLength() {
	if (len(e.out) & 1) == 1 {
		e.out = append(e.out, 0x00)
	}
}

func (e *rleEncoder) GetBuffer() []byte {
	e.Flush()
	binary.LittleEndian.PutUint32(e.out[0:4], uint32(e.count))
	for i := 0; i < 15; i++ {
		binary.LittleEndian.PutUint32(e.out[4+i*4:], e.offsets[i])
	}
	return e.out
}

type rleDecoder struct {
	NumberOfSegments int
	offsets          [15]int
	data             []byte
}

func newRLEDecoder(data []byte) (*rleDecoder, error) {
	if len(data) < 64 {
		return nil, fmt.Errorf("RLE data too short: need at least 64 bytes, got %d", len(data))
	}
	dec := &rleDecoder{data: data}
	numSegments := int(binary.LittleEndian.Uint32(data[0:4]))
	if numSegments < 1 || numSegments > 15 {
		return nil, fmt.Errorf("invalid number of RLE segments: %d (must be 1-15)", numSegments)
	}
	dec.NumberOfSegments = numSegments
	for i := 0; i < 15; i++ {
		offset := int(binary.LittleEndian.Uint32(data[4+i*4:]))
		if i < numSegments && offset > len(data) {
			return nil, fmt.Errorf("RLE segment %d offset %d exceeds data length %d", i, offset, len(data))
		}
		dec.offsets[i] = offset
	}
	return dec, nil
}

func (d *rleDecoder) DecodeSegment(segment int, buffer []byte, start int, sampleOffset int) error {
	if segment < 0 || segment >= d.NumberOfSegments {
		return fmt.Errorf("segment number %d out of range [0, %d)", segment, d.NumberOfSegments)
	}
	return d.decode(buffer, start, sampleOffset, d.data, d.offsets[segment], d.segmentLength(segment))
}

func (d *rleDecoder) segmentLength(segment int) int {
	offset := d.offsets[segment]
	if segment < d.NumberOfSegments-1 {
		return d.offsets[segment+1] - offset
	}
	return len(d.data) - offset
}

// decode expands one PackBits segment into a strided plane of buffer. Each run
// is bounds-checked before it is written, so the strided writes can safely use
// unsafe pointer arithmetic to avoid a bounds check on every byte.
func (d *rleDecoder) decode(buffer []byte, start int, sampleOffset int, rleData []byte, offset int, count int) error {
	if start < 0 || start >= len(buffer) {
		return fmt.Errorf("invalid RLE output start: %d", start)
	}
	if sampleOffset <= 0 {
		return fmt.Errorf("invalid RLE sample offset: %d", sampleOffset)
	}
	if offset < 0 || count < 0 || offset > len(rleData) {
		return fmt.Errorf("invalid RLE segment bounds: offset %d, length %d", offset, count)
	}
	if offset+count > len(rleData) {
		count = len(rleData) - offset
	}

	pos := start
	end := offset + count
	bufferLength := len(buffer)
	for i := offset; i < end && pos < bufferLength; {
		control := int8(rleData[i])
		i++
		if control >= 0 {
			length := int(control) + 1
			if end-i < length {
				return fmt.Errorf("RLE literal run exceeds input buffer length")
			}
			if pos+(length-1)*sampleOffset >= bufferLength {
				return fmt.Errorf("RLE literal run exceeds output buffer length")
			}
			if sampleOffset == 1 {
				copy(buffer[pos:], rleData[i:i+length])
				i += length
				pos += length
			} else {
				// The run bound above proves that every write below is inside
				// buffer. Strided samples make the per-byte bounds check the
				// dominant cost, so write through a bounded base pointer.
				base := unsafe.Pointer(&buffer[pos])
				off := 0
				j := 0
				if sampleOffset == 2 {
					for ; j+1 < length; j += 2 {
						*(*byte)(unsafe.Add(base, off)) = rleData[i]
						*(*byte)(unsafe.Add(base, off+2)) = rleData[i+1]
						i += 2
						off += 4
					}
				}
				for ; j < length; j++ {
					*(*byte)(unsafe.Add(base, off)) = rleData[i]
					i++
					off += sampleOffset
				}
				pos += length * sampleOffset
			}
		} else if control >= -127 {
			length := int(-control) + 1
			if pos+(length-1)*sampleOffset >= bufferLength {
				return fmt.Errorf("RLE repeat run exceeds output buffer length")
			}
			if i >= end {
				return io.ErrUnexpectedEOF
			}
			b := rleData[i]
			i++
			if sampleOffset == 1 {
				segment := buffer[pos : pos+length]
				for j := range segment {
					segment[j] = b
				}
				pos += length
			} else {
				// See the literal branch: the run bound above covers all writes.
				base := unsafe.Pointer(&buffer[pos])
				off := 0
				j := 0
				if sampleOffset == 2 {
					for ; j+1 < length; j += 2 {
						*(*byte)(unsafe.Add(base, off)) = b
						*(*byte)(unsafe.Add(base, off+2)) = b
						off += 4
					}
				}
				for ; j < length; j++ {
					*(*byte)(unsafe.Add(base, off)) = b
					off += sampleOffset
				}
				pos += length * sampleOffset
			}
		}
		if i+1 >= end {
			break
		}
	}
	return nil
}

// RegisterRLECodec registers the RLE Lossless codec with the global registry.
func RegisterRLECodec() {
	registry := codec.GlobalRegistry()
	if _, err := registry.Replace(NewRLECodec()); err != nil {
		panic(err)
	}
}

func init() { RegisterRLECodec() }
