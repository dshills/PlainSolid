package document

import (
	"bytes"
	"plainsolid/exchange/step"
)

// WithOuterPlacements returns a separately validated nested document with edited
// exact outer poses. Inner source uses and native local assets remain unchanged.
// The receiver is immutable. Flat documents are deliberately unsupported here.
func (d *RationalMappedPlanarDocument) WithOuterPlacements(placements []step.RationalMappedPlacement, l step.Limits) (*RationalMappedPlanarDocument, error) {
	if d == nil || len(d.paths) == 0 {
		return nil, rationalMappedDocumentError()
	}
	source, e := step.ReadNestedRationalMappedPlanarMaterials(bytes.NewReader(d.source), l)
	if e != nil {
		return nil, e
	}
	var raw bytes.Buffer
	if e = step.WriteNestedRationalMappedOuterPlacements(&raw, source.Source, placements, l); e != nil {
		return nil, e
	}
	next, e := ReadNestedRationalMappedPlanarSTEP(&raw, l)
	if e != nil {
		return nil, e
	}
	if !bytes.Equal(d.native, next.native) {
		return nil, rationalMappedDocumentError()
	}
	return next, nil
}
