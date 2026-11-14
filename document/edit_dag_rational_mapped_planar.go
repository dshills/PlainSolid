package document

import (
	"bytes"
	"plainsolid/exchange/step"
)

// WithDAGRootPlacements returns a new DAG document with selected unshared root
// use poses changed. Internal/shared definitions require the explicit method below.
func (d *RationalMappedPlanarDocument) WithDAGRootPlacements(ps []step.RationalMappedPlacement, l step.Limits) (*RationalMappedPlanarDocument, error) {
	return d.withDAGPlacements(ps, l, false)
}

// WithDAGSharedPlacements explicitly changes an internal source use definition
// on every incoming occurrence path. The receiver and shared local assets remain
// unchanged; no occurrence-specific clone is silently substituted.
func (d *RationalMappedPlanarDocument) WithDAGSharedPlacements(ps []step.RationalMappedPlacement, l step.Limits) (*RationalMappedPlanarDocument, error) {
	return d.withDAGPlacements(ps, l, true)
}
func (d *RationalMappedPlanarDocument) withDAGPlacements(ps []step.RationalMappedPlacement, l step.Limits, shared bool) (*RationalMappedPlanarDocument, error) {
	if d == nil || len(d.dagPaths) == 0 {
		return nil, rationalMappedDocumentError()
	}
	a, e := step.ReadDAGRationalMappedPlanarMaterials(bytes.NewReader(d.source), l)
	if e != nil {
		return nil, e
	}
	write := step.WriteDAGRationalMappedRootPlacements
	if shared {
		write = step.WriteDAGRationalMappedSharedPlacements
	}
	var raw bytes.Buffer
	if e = write(&raw, a.Source, ps, l); e != nil {
		return nil, e
	}
	next, e := ReadDAGRationalMappedPlanarSTEP(&raw, l)
	if e != nil {
		return nil, e
	}
	if !bytes.Equal(next.native, d.native) {
		return nil, rationalMappedDocumentError()
	}
	return next, nil
}
