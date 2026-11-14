package document

import (
	"bytes"
	"io"
	"plainsolid/exchange/step"
)

// ReadDAGRationalMappedPlanarSTEP retains bounded mixed shared-group paths and
// exact composed poses outside generic floating State.Nodes.
func ReadDAGRationalMappedPlanarSTEP(r io.Reader, l step.Limits) (*RationalMappedPlanarDocument, error) {
	if r == nil {
		return nil, rationalMappedDocumentError()
	}
	a, e := step.ReadDAGRationalMappedPlanarMaterials(r, l)
	if e != nil {
		return nil, e
	}
	var source bytes.Buffer
	if e = step.WriteDAGRationalMappedPlanarSource(&source, a.Source, l); e != nil {
		return nil, e
	}
	flat := step.RationalMappedPlanarMaterials{Source: a.Source, Name: a.Name, LengthUnit: a.LengthUnit, Bodies: a.Bodies, MapOrder: a.MapOrder}
	paths := [][]step.MappedSourceStep{}
	for _, u := range a.Occurrences {
		flat.Occurrences = append(flat.Occurrences, u.Occurrence)
		paths = append(paths, append([]step.MappedSourceStep(nil), u.Path...))
	}
	d, e := newRationalMappedPlanarDocument(flat, source.Bytes())
	if e != nil {
		return nil, e
	}
	d.dagPaths = paths
	return d, nil
}

// ReadDAGRationalMappedPlanarDocument reads the separately tagged version1 DAG
// envelope; fixed-depth native formats are not silently reinterpreted.
func ReadDAGRationalMappedPlanarDocument(r io.Reader, maxBytes int64, l step.Limits) (*RationalMappedPlanarDocument, error) {
	return readRationalMappedPlanarDocument(r, maxBytes, l, 3)
}

// DAGOccurrencePaths returns deep copies aligned with Occurrences. Reused leaf
// IDs are distinguished by the complete outer-to-inner source path.
func (d *RationalMappedPlanarDocument) DAGOccurrencePaths() [][]step.MappedSourceStep {
	if d == nil {
		return nil
	}
	out := make([][]step.MappedSourceStep, len(d.dagPaths))
	for i, p := range d.dagPaths {
		out[i] = append([]step.MappedSourceStep(nil), p...)
	}
	return out
}
