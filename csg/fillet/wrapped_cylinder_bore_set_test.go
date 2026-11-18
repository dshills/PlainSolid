package fillet

import (
	"bytes"
	"context"
	"math"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestRoundedCylinderMixedBoreSet(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 4, 8, o)
	if e != nil {
		t.Fatal(e)
	}
	source, _, e := trimmed.NewWrappedCylinder(ctx, w, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	source, _, e = trimmed.FilletWrappedCylinderRims(ctx, source, .5, 1, o)
	if e != nil {
		t.Fatal(e)
	}
	faces := source.Faces()
	faces = append(faces[2:], faces[:2]...)
	source, e = trimmed.NewSolid(source.Vertices(), source.Edges(), faces, source.Tolerance())
	if e != nil {
		t.Fatal(e)
	}
	originalV, originalE, originalF := source.Vertices(), source.Edges(), source.Faces()
	before, e := source.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil {
		t.Fatal(e)
	}
	cuts := []RoundedCylinderBoreCut{{Center: geom.Vec2{X: -1}, Radius: .5}, {Center: geom.Vec2{X: 1}, Radius: .5, Blind: true, Depth: 3, FromUpper: true}, {Center: geom.Vec2{X: 1}, Radius: .5, Blind: true, Depth: 3}}
	result, e := BoreWrappedRoundedCylinderSet(ctx, source, cuts, o)
	if e != nil {
		t.Fatal(e)
	}
	out := result.Solid
	if out.Topology().Faces != source.Topology().Faces+5 || out.Topology().EulerCharacteristic != 0 || len(result.Cuts) != 3 {
		t.Fatal(out.Topology(), result)
	}
	if !reflect.DeepEqual(out.Vertices()[:len(originalV)], originalV) || !reflect.DeepEqual(out.Edges()[:len(originalE)], originalE) {
		t.Fatal("original topology changed")
	}
	for i, f := range out.Faces()[:len(originalF)] {
		if f.Surface != originalF[i].Surface || !reflect.DeepEqual(f.Outer, originalF[i].Outer) || !reflect.DeepEqual(result.SourceFaces[i], []int{i}) {
			t.Fatal("source face changed", i)
		}
	}
	for _, cap := range result.SourceCaps {
		if len(out.Faces()[cap].Holes) != 2 {
			t.Fatal("multiple cap ownership")
		}
	}
	seen := map[int]bool{}
	for i, h := range result.Cuts {
		if seen[h.BoreFace] || len(result.SourceFaces[h.BoreFace]) != 0 {
			t.Fatal(h)
		}
		seen[h.BoreFace] = true
		if (h.FloorFace >= 0) != cuts[i].Blind {
			t.Fatal(h)
		}
		if h.FloorFace >= 0 {
			if seen[h.FloorFace] || len(result.SourceFaces[h.FloorFace]) != 0 {
				t.Fatal(h)
			}
			seen[h.FloorFace] = true
		}
		for _, rim := range h.BoreRims {
			if len(rim) != 4 {
				t.Fatal(h)
			}
		}
	}
	after, e := out.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil {
		t.Fatal(e)
	}
	wantV := before.Volume - 3.5*math.Pi
	wantA := before.Area + 13.5*math.Pi
	if math.Abs(after.Volume-wantV) > 1e-7 || math.Abs(after.Area-wantA) > 1e-7 || math.Abs(after.Centroid.X-.5*math.Pi/wantV) > 1e-7 || math.Abs(after.Centroid.Z-(before.Volume*before.Centroid.Z-14*math.Pi)/wantV) > 1e-7 {
		t.Fatal(after, wantV, wantA)
	}
	var b bytes.Buffer
	if e = trimmed.WriteJSON(&b, out); e != nil {
		t.Fatal(e)
	}
	back, e := trimmed.ReadJSONContext(ctx, &b, trimmed.JSONLimits{})
	if e != nil || back.Topology().Faces != out.Topology().Faces {
		t.Fatal(e)
	}
	for _, bad := range [][]RoundedCylinderBoreCut{
		{cuts[0], cuts[0]},
		{{Center: geom.Vec2{X: -.5}, Radius: .5}, {Center: geom.Vec2{X: .5}, Radius: .5}},
		{{Radius: .5, Blind: true, Depth: 4}, {Radius: .5, Blind: true, Depth: 4, FromUpper: true}},
		{{Radius: .5, Blind: true, Depth: 4.5}, {Radius: .5, Blind: true, Depth: 4, FromUpper: true}},
		{{Radius: .5, Depth: 1}},
		make([]RoundedCylinderBoreCut, 17), nil,
	} {
		r, e := BoreWrappedRoundedCylinderSet(ctx, source, bad, o)
		if e == nil || r.Solid.Topology().Faces != 0 || len(r.Cuts) != 0 {
			t.Fatal("atomic overlap/limit", e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := BoreWrappedRoundedCylinderSet(cancelled, source, cuts, o); e != context.Canceled || r.Solid.Topology().Faces != 0 {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(source.Faces(), originalF) {
		t.Fatal("source mutated")
	}
}
