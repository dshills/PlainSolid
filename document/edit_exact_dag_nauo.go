package document

import (
	"bytes"
	"plainsolid/exchange/step"
)

// WithRootPlacements returns a new source-bound DAG document. The original
// document, local native assets and all unselected source occurrences are kept.
// Shared internal uses require a distinct operation and are rejected here.
func (d *ExactDAGPlanarNAUODocument) WithRootPlacements(ps []step.ExactNAUOPlacement, l step.Limits) (*ExactDAGPlanarNAUODocument, error) {
	if d == nil {
		return nil, rationalMappedDocumentError()
	}
	source, e := step.ReadExactDAGPlanarNAUO(bytes.NewReader(d.source), l)
	if e != nil {
		return nil, e
	}
	var out bytes.Buffer
	if e = step.WriteExactDAGNAUORootPlacements(&out, source.Source, ps, l); e != nil {
		return nil, e
	}
	return ReadExactDAGPlanarNAUOSTEP(&out, l)
}
