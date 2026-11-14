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

type placedEllipseSnapshot struct {
	Filled              *EllipticalPrismTopology
	Annular             *EllipticalAnnularPrismTopology
	Placement           geom.Affine
	TransformID         int
	TransformMetadata   CylinderDatumMetadata
	Vertices, Terminals []geom.Vec3
	Faces               []int
	Rims                [2][2][4]int
}

func placedEllipseSnapshotOf(m PlacedEllipticalPrismMaterial) placedEllipseSnapshot {
	p := placedEllipseSnapshot{m.Filled, m.Annular, m.Placement, m.SourceTransformID, m.SourceTransformMetadata, m.SourceVertices, m.SourceTerminalPoints, m.NativeFaceSourceIDs, m.NativeRimEdges}
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

type placedEllipseEnvelope struct {
	Format    string
	Version   int
	Source    string
	Tolerance float64
	Native    json.RawMessage
	Data      placedEllipseSnapshot
}

func WritePlacedEllipticalPrismMaterialJSON(ctx context.Context, w io.Writer, m PlacedEllipticalPrismMaterial) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadPlacedEllipticalPrismMaterial(ctx, bytes.NewReader(m.source), Limits{}, m.Solid.Tolerance())
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
	if !bytes.Equal(a, b) || !reflect.DeepEqual(placedEllipseSnapshotOf(m), placedEllipseSnapshotOf(p)) {
		return fmt.Errorf("iges: stale placed ellipse material")
	}
	raw, e := json.Marshal(placedEllipseEnvelope{"plainsolid-iges-placed-ellipse-material", 1, string(p.source), p.Solid.Tolerance(), b, placedEllipseSnapshotOf(p)})
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
func ReadPlacedEllipticalPrismMaterialJSON(ctx context.Context, r io.Reader, l Limits) (PlacedEllipticalPrismMaterial, error) {
	bad := func() (PlacedEllipticalPrismMaterial, error) {
		return PlacedEllipticalPrismMaterial{}, fmt.Errorf("iges: invalid placed ellipse envelope")
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
	var q placedEllipseEnvelope
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&q) != nil || q.Format != "plainsolid-iges-placed-ellipse-material" || q.Version != 1 {
		return bad()
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return bad()
	}
	p, e := ReadPlacedEllipticalPrismMaterial(ctx, bytes.NewReader([]byte(q.Source)), l, q.Tolerance)
	if e != nil {
		return bad()
	}
	native, e := igesPrismBytes(p.Solid)
	if e != nil {
		return bad()
	}
	var a, b bytes.Buffer
	if json.Compact(&a, q.Native) != nil || json.Compact(&b, native) != nil || !bytes.Equal(a.Bytes(), b.Bytes()) || !reflect.DeepEqual(q.Data, placedEllipseSnapshotOf(p)) {
		return bad()
	}
	return p, nil
}
