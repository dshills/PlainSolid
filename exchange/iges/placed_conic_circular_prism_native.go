package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"plainsolid/geom"
	"reflect"
)

type placedCircularConicSnapshot struct {
	Filled              *EllipticalPrismTopology
	Annular             *EllipticalAnnularPrismTopology
	Placement           geom.Affine
	TransformID         int
	TransformMetadata   CylinderDatumMetadata
	Vertices, Terminals []geom.Vec3
	Faces               []int
	Rims                [2][2][4]int
}

func placedCircularConicSnapshotOf(m PlacedConicCircularPrismMaterial) placedCircularConicSnapshot {
	p := placedCircularConicSnapshot{m.Filled, m.Annular, m.Placement, m.SourceTransformID, m.SourceTransformMetadata, m.SourceVertices, m.SourceTerminalPoints, m.NativeFaceSourceIDs, m.NativeRimEdges}
	if p.Filled != nil {
		x := *p.Filled
		x.source = nil
		p.Filled = &x
	}
	if p.Annular != nil {
		x := *p.Annular
		x.source = nil
		p.Annular = &x
	}
	return p
}

type placedCircularConicEnvelope struct {
	Format    string
	Version   int
	Source    string
	Tolerance float64
	Native    json.RawMessage
	Data      placedCircularConicSnapshot
}

func WritePlacedConicCircularPrismMaterialJSON(ctx context.Context, w io.Writer, m PlacedConicCircularPrismMaterial) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadPlacedConicCircularPrismMaterial(ctx, bytes.NewReader(m.source), Limits{}, m.Solid.Tolerance())
	if e != nil {
		return e
	}
	a, e := igesPrismBytes(m.Solid)
	if e != nil {
		return e
	}
	b, e := igesPrismBytes(p.Solid)
	if e != nil {
		return e
	}
	if !bytes.Equal(a, b) || !reflect.DeepEqual(placedCircularConicSnapshotOf(m), placedCircularConicSnapshotOf(p)) {
		return fmt.Errorf("iges: stale placed ellipse material")
	}
	raw, e := json.Marshal(placedCircularConicEnvelope{"plainsolid-iges-placed-circular-conic-material", 1, string(p.source), p.Solid.Tolerance(), b, placedCircularConicSnapshotOf(p)})
	if e != nil {
		return e
	}
	if len(raw) > 512<<10 {
		return geom.ErrConvergence
	}
	raw = append(raw, '\n')
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}
func ReadPlacedConicCircularPrismMaterialJSON(ctx context.Context, r io.Reader, l Limits) (PlacedConicCircularPrismMaterial, error) {
	bad := func() (PlacedConicCircularPrismMaterial, error) {
		return PlacedConicCircularPrismMaterial{}, fmt.Errorf("iges: invalid placed ellipse envelope")
	}
	if ctx == nil || r == nil {
		return bad()
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	limit := min(l.MaxBytes, int64(512<<10))
	raw, e := io.ReadAll(io.LimitReader(r, limit+1))
	if e != nil || int64(len(raw)) > limit {
		return bad()
	}
	var q placedCircularConicEnvelope
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&q) != nil || q.Format != "plainsolid-iges-placed-circular-conic-material" || q.Version != 1 {
		return bad()
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return bad()
	}
	p, e := ReadPlacedConicCircularPrismMaterial(ctx, bytes.NewReader([]byte(q.Source)), l, q.Tolerance)
	if e != nil {
		return bad()
	}
	native, e := igesPrismBytes(p.Solid)
	if e != nil {
		return bad()
	}
	var a, b bytes.Buffer
	if json.Compact(&a, q.Native) != nil || json.Compact(&b, native) != nil || !bytes.Equal(a.Bytes(), b.Bytes()) || !reflect.DeepEqual(q.Data, placedCircularConicSnapshotOf(p)) {
		return bad()
	}
	return p, nil
}

// WritePlacedConicCircularPrismSource returns the authenticated original file,
// after re-proving native geometry, exact placement and source ancestry.
func WritePlacedConicCircularPrismSource(ctx context.Context, w io.Writer, m PlacedConicCircularPrismMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadPlacedConicCircularPrismMaterial(ctx, bytes.NewReader(m.source), Limits{}, m.Solid.Tolerance())
	if e != nil {
		return e
	}
	a, e := igesPrismBytes(m.Solid)
	if e != nil {
		return e
	}
	b, e := igesPrismBytes(p.Solid)
	if e != nil {
		return e
	}
	if !bytes.Equal(a, b) || !reflect.DeepEqual(placedCircularConicSnapshotOf(m), placedCircularConicSnapshotOf(p)) {
		return geom.ErrInvalid
	}
	n, e := w.Write(p.source)
	if e == nil && n != len(p.source) {
		e = io.ErrShortWrite
	}
	return e
}
