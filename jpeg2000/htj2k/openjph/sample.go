package openjph

func reversibleSampleToInternal(sample int32, bitDepth int, isSigned bool) int32 {
	if isSigned {
		return sample
	}
	return sample - int32(1<<(bitDepth-1))
}

func irreversibleSampleToInternal(sample int32, bitDepth int, isSigned bool) float32 {
	if !isSigned {
		sample -= int32(1 << (bitDepth - 1))
	}
	mul := float32(1.0 / float64(uint64(1)<<bitDepth))
	return float32(sample) * mul
}

func irreversibleInternalToSample(value float32, bitDepth int, isSigned bool) int32 {
	mul := float32(uint64(1) << bitDepth)
	t := value * mul
	v := openJPHRound(t)
	low := int32(-int64(1 << (bitDepth - 1)))
	high := int32((int64(1) << (bitDepth - 1)) - 1)
	if t < float32(low) {
		v = low
	} else if t >= -float32(low) {
		v = high
	}
	if !isSigned {
		v += int32(1 << (bitDepth - 1))
	}
	return v
}

func openJPHRound(value float32) int32 {
	if value >= 0 {
		return int32(value + 0.5)
	}
	return int32(value - 0.5)
}
