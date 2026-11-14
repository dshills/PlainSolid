package document

import (
	"bytes"
	"io"
	"plainsolid/exchange/step"
)

// ReadTwoLevelRationalMappedPlanarSTEP retains exactly two shared mapping group
// levels and three-step source ancestry in the separate exact-pose document.
func ReadTwoLevelRationalMappedPlanarSTEP(r io.Reader, l step.Limits) (*RationalMappedPlanarDocument, error) {
	if r == nil {
		return nil, rationalMappedDocumentError()
	}
	a, e := step.ReadTwoLevelRationalMappedPlanarMaterials(r, l)
	if e != nil {
		return nil, e
	}
	var source bytes.Buffer
	if e = step.WriteTwoLevelRationalMappedPlanarSource(&source, a.Source, l); e != nil {
		return nil, e
	}
	flat := step.RationalMappedPlanarMaterials{Source: a.Source, Name: a.Name, LengthUnit: a.LengthUnit, Bodies: a.Bodies, MapOrder: a.MapOrder}
	paths := [][3]step.MappedSourceStep{}
	for _, u := range a.Occurrences {
		flat.Occurrences = append(flat.Occurrences, u.Occurrence)
		paths = append(paths, u.Path)
	}
	d, e := newRationalMappedPlanarDocument(flat, source.Bytes())
	if e != nil {
		return nil, e
	}
	d.deepPaths = paths
	return d, nil
}

// ReadTwoLevelRationalMappedPlanarDocument reads only the explicitly tagged
// two-level version1 envelope; flat and one-level envelopes are not inferred.
func ReadTwoLevelRationalMappedPlanarDocument(r io.Reader, maxBytes int64, l step.Limits) (*RationalMappedPlanarDocument, error) {
	return readRationalMappedPlanarDocument(r, maxBytes, l, 2)
}

// TwoLevelOccurrencePaths returns copied three-step ancestry aligned with
// Occurrences. Shallower documents return nil. Use BakeOccurrenceIndex for an
// explicit expanded occurrence; repeated leaf IDs are never silently selected.
func (d *RationalMappedPlanarDocument) TwoLevelOccurrencePaths() [][3]step.MappedSourceStep {
	if d == nil {
		return nil
	}
	return append([][3]step.MappedSourceStep(nil), d.deepPaths...)
}
