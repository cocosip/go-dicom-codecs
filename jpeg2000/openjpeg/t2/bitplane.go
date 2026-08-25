// Package t2 contains Tier-2 packet coding helpers.
package t2

import "github.com/cocosip/go-dicom-codecs/jpeg2000/internal/common/codestream"

func subbandIndex(numLevels, res, band int) int {
	if res < 0 || res > numLevels {
		return -1
	}
	if res == 0 {
		if band != 0 {
			return -1
		}
		return 0
	}
	if band < 1 || band > 3 {
		return -1
	}
	return 1 + (res-1)*3 + (band - 1)
}

func bandNumbpsFromQCD(qcd *codestream.QCDSegment, numLevels, res, band int) (int, bool) {
	if qcd == nil {
		return 0, false
	}
	idx := subbandIndex(numLevels, res, band)
	if idx < 0 {
		return 0, false
	}
	guardBits := qcd.GuardBits()
	qntsty := qcd.QuantizationType()
	switch qntsty {
	case 0:
		if idx >= len(qcd.SPqcd) {
			return 0, false
		}
		// OpenJPEG implementation: exponent is stored in bits 3-7 (expn<<3)
		// This differs from JPEG2000 standard (bits 0-4) but matches OpenJPEG behavior
		expn := int(qcd.SPqcd[idx] >> 3)
		return expn + guardBits - 1, true
	case 1:
		if len(qcd.SPqcd) < 2 {
			return 0, false
		}
		encoded := uint16(qcd.SPqcd[0])<<8 | uint16(qcd.SPqcd[1])
		expn := int((encoded >> 11) & 0x1F)
		if idx > 0 {
			expn -= (idx - 1) / 3
			if expn < 0 {
				expn = 0
			}
		}
		return expn + guardBits - 1, true
	case 2:
		offset := idx * 2
		if offset+1 >= len(qcd.SPqcd) {
			return 0, false
		}
		encoded := uint16(qcd.SPqcd[offset])<<8 | uint16(qcd.SPqcd[offset+1])
		expn := int((encoded >> 11) & 0x1F)
		return expn + guardBits - 1, true
	default:
		return 0, false
	}
}
