package document

import (
	"bytes"
	"io"
	"plainsolid/exchange/step"
)

// ReadNestedRationalMappedPlanarSTEP retains exactly one shared intermediate
// representation level. Its explicit two-step source paths distinguish repeated
// leaf item IDs; exact composed poses remain separate from floating document nodes.
func ReadNestedRationalMappedPlanarSTEP(r io.Reader, l step.Limits) (*RationalMappedPlanarDocument, error) {
	if r == nil {
		return nil, rationalMappedDocumentError()
	}
	a, e := step.ReadNestedRationalMappedPlanarMaterials(r, l)
	if e != nil {
		return nil, e
	}
	var source bytes.Buffer
	if e = step.WriteNestedRationalMappedPlanarSource(&source, a.Source, l); e != nil {
		return nil, e
	}
	flat := step.RationalMappedPlanarMaterials{Source: a.Source, Name: a.Name, LengthUnit: a.LengthUnit, Bodies: a.Bodies, MapOrder: a.MapOrder}
	paths := [][2]step.MappedSourceStep{}
	for _, u := range a.Occurrences {
		flat.Occurrences = append(flat.Occurrences, u.Occurrence)
		paths = append(paths, u.Path)
	}
	d, e := newRationalMappedPlanarDocument(flat, source.Bytes())
	if e != nil {
		return nil, e
	}
	d.paths = paths
	return d, nil
}

// ReadNestedRationalMappedPlanarDocument reads the separately tagged version1
// nested envelope. The prior flat native reader deliberately rejects this format.
func ReadNestedRationalMappedPlanarDocument(r io.Reader, maxBytes int64, l step.Limits) (*RationalMappedPlanarDocument, error) {
	return readRationalMappedPlanarDocument(r, maxBytes, l, 1)
}

// OccurrencePaths returns copied outer/inner source ancestry, aligned with
// Occurrences. Flat documents return nil. IDs remain original source IDs.
func (d *RationalMappedPlanarDocument) OccurrencePaths() [][2]step.MappedSourceStep {
	if d == nil {
		return nil
	}
	return append([][2]step.MappedSourceStep(nil), d.paths...)
}

// BakeOccurrenceIndex selects an expanded occurrence by its stable source order.
// For nested documents, BakeOccurrence(itemID) rejects ambiguous reused leaf IDs;
// this method and OccurrencePaths provide an explicit occurrence selection.
func (d *RationalMappedPlanarDocument) BakeOccurrenceIndex(index int) (step.Body, error) {
	if d == nil || index < 0 || index >= len(d.model.Occurrences) {
		return step.Body{}, rationalMappedDocumentError()
	}
	view := *d
	view.paths = nil
	view.deepPaths = nil
	view.dagPaths = nil
	view.model = d.model
	view.model.Occurrences = []step.ExactMappedOccurrence{d.model.Occurrences[index]}
	return view.BakeOccurrence(view.model.Occurrences[0].ItemID)
}
