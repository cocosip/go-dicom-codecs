package main

import (
	"testing"

	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/dicom/vr"
)

func TestApplyBaselinePhotometricInterpretationMatchesFoDicom(t *testing.T) {
	ds := dataset.New()
	if err := ds.Add(element.NewUnsignedShort(tag.SamplesPerPixel, []uint16{3})); err != nil {
		t.Fatal(err)
	}
	if err := ds.Add(element.NewString(tag.PhotometricInterpretation, vr.CS, []string{"RGB"})); err != nil {
		t.Fatal(err)
	}

	if err := applyBaselinePhotometricInterpretation(ds, transfer.JPEGBaseline8Bit); err != nil {
		t.Fatalf("applyBaselinePhotometricInterpretation() error = %v", err)
	}
	if got := ds.TryGetString(tag.PhotometricInterpretation); got != "YBR_FULL_422" {
		t.Errorf("PhotometricInterpretation = %q, want YBR_FULL_422", got)
	}
}
