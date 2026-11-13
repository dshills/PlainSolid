package document

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"plainsolid/exchange/step"
)

type partitionedLensProvenance struct {
	Body     int64
	Faces    [2]int64
	Pieces   []step.LensSeamPiece
	Geometry string
}

// ReadPartitionedSplineLensSTEP retains original STEP seam pieces and both
// chart-cell lineages as asset metadata alongside the canonical native material.
// It imports one identity-placed body; no assembly or source/native edge identity
// equivalence is inferred.
func ReadPartitionedSplineLensSTEP(r io.Reader, l step.Limits, o step.CurvedOvalOptions, historyLimit int) (*Document, error) {
	m, e := step.ReadPartitionedSplineLensBody(r, l, o)
	if e != nil {
		return nil, e
	}
	return fromVerifiedPartitionedLens(m, historyLimit)
}
func fromVerifiedPartitionedLens(m step.PartitionedSplineLens, historyLimit int) (*Document, error) {
	d, e := fromVerifiedSplineLens(m.Body, historyLimit)
	if e != nil {
		return nil, e
	}
	s, _ := d.Snapshot()
	digest, e := lensGeometryDigest(m.Body.Solid)
	if e != nil {
		return nil, e
	}
	raw, e := json.Marshal(partitionedLensProvenance{m.Body.SourceBodyID, m.Body.SourceFaceIDs, m.Pieces, digest})
	if e != nil {
		return nil, e
	}
	label := s.Labels["step:body-000001"]
	delete(label.Attributes, "step.lens.provenance")
	delete(label.Attributes, "step.lens.binding")
	label.Attributes["step.lens.partitioned.provenance"] = Attribute{Kind: "string", Text: string(raw)}
	label.Attributes["step.lens.partitioned.binding"] = Attribute{Kind: "string", Text: lensProvenanceDigest(raw)}
	s.Labels["step:body-000001"] = label
	return New(s, historyLimit)
}

// FromPartitionedSplineLensSTEP proves supplied geometry by staging an exact
// STEP graph. Its provenance refers to that staged graph, not caller-supplied IDs.
func FromPartitionedSplineLensSTEP(m step.SplineLensBody, historyLimit int) (*Document, error) {
	var b bytes.Buffer
	if e := step.WritePartitionedSplineLensBody(&b, m, step.CurvedOvalOptions{}); e != nil {
		return nil, e
	}
	return ReadPartitionedSplineLensSTEP(&b, step.Limits{}, step.CurvedOvalOptions{}, historyLimit)
}

// WritePartitionedSplineLensSTEP exports one unchanged canonical native lens
// with the original certified seam partition. Names and units may be edited;
// placements, extra instances and stale geometry or source metadata fail.
func (s State) WritePartitionedSplineLensSTEP(w io.Writer) error {
	return s.writeSplineLensSTEP(w, true)
}

func writePartitionedLensWithProvenance(w io.Writer, m step.SplineLensBody, p partitionedLensProvenance) error {
	var b bytes.Buffer
	if e := step.WritePartitionedSplineLensBody(&b, m, step.CurvedOvalOptions{}); e != nil {
		return e
	}
	rebuilt, e := step.ReadPartitionedSplineLensBody(bytes.NewReader(b.Bytes()), step.Limits{}, step.CurvedOvalOptions{})
	if e != nil {
		return e
	}
	if len(rebuilt.Pieces) != len(p.Pieces) {
		return fmt.Errorf("document: stale lens seam partition")
	}
	for i, a := range p.Pieces {
		b := rebuilt.Pieces[i]
		if a.Start == nil || a.End == nil || a.Quarter != b.Quarter || a.Start.Cmp(b.Start) != 0 || a.End.Cmp(b.End) != 0 || a.Cells != b.Cells {
			return fmt.Errorf("document: stale lens seam lineage")
		}
	}
	n, e := w.Write(b.Bytes())
	if e != nil {
		return e
	}
	if n != b.Len() {
		return io.ErrShortWrite
	}
	return nil
}
