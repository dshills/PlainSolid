package fillet

import (
	"bytes"
	"context"
	"math"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestBorePreservesRoundedCylinderSupports(t *testing.T) {
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
	result, e := BoreWrappedRoundedCylinder(ctx, source, geom.Vec2{X: .5, Y: .25}, 1, o)
	if e != nil {
		t.Fatal(e)
	}
	if result.FloorFace != -1 || result.FloorZ != nil {
		t.Fatal("through bore acquired floor", result)
	}
	out := result.Solid
	if out.Topology().Faces != source.Topology().Faces+1 || out.Topology().EulerCharacteristic != 0 {
		t.Fatal(out.Topology())
	}
	if !reflect.DeepEqual(originalV, out.Vertices()[:len(originalV)]) || !reflect.DeepEqual(originalE, out.Edges()[:len(originalE)]) {
		t.Fatal("original entity identity changed")
	}
	for i, f := range out.Faces()[:len(originalF)] {
		if f.Surface != originalF[i].Surface || !reflect.DeepEqual(f.Outer, originalF[i].Outer) || f.Reversed != originalF[i].Reversed || !reflect.DeepEqual(result.SourceFaces[i], []int{i}) {
			t.Fatal("original support/face changed", i)
		}
	}
	if len(result.SourceFaces[result.BoreFace]) != 0 {
		t.Fatal("new wall falsely attributed")
	}
	for end, id := range result.SourceCaps {
		if len(out.Faces()[id].Holes) != 1 || len(result.BoreRims[end]) != 4 {
			t.Fatal(result)
		}
	}
	after, e := out.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(after.Volume-(before.Volume-8*math.Pi)) > 1e-7 || math.Abs(after.Area-(before.Area+14*math.Pi)) > 1e-7 {
		t.Fatal(before, after)
	}
	if math.Abs(after.Centroid.X+.5*8*math.Pi/after.Volume) > 1e-7 || math.Abs(after.Centroid.Y+.25*8*math.Pi/after.Volume) > 1e-7 {
		t.Fatal(after.Centroid)
	}
	var b bytes.Buffer
	if e = trimmed.WriteJSON(&b, out); e != nil {
		t.Fatal(e)
	}
	back, e := trimmed.ReadJSONContext(ctx, &b, trimmed.JSONLimits{})
	if e != nil || back.Topology().Faces != out.Topology().Faces {
		t.Fatal(e)
	}
	for _, radius := range []float64{2, 3, 4, math.NaN()} {
		// radius2 at offset1 touches the smaller flat cap disk of radius3.
		bad, e := BoreWrappedRoundedCylinder(ctx, source, geom.Vec2{X: 1}, radius, o)
		if e == nil || bad.Solid.Topology().Faces != 0 {
			t.Fatal("invalid cap clearance", radius, e)
		}
	}
	if !reflect.DeepEqual(source.Faces(), originalF) {
		t.Fatal("input mutated")
	}
}

func TestRoundedCylinderBoreAtomicLimits(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 4, 8, o)
	if e != nil {
		t.Fatal(e)
	}
	s, _, e := trimmed.NewWrappedCylinder(ctx, w, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	// The narrow API requires a proved existing blend, not arbitrary stock.
	if out, e := BoreWrappedRoundedCylinder(ctx, s, geom.Vec2{}, 1, o); e == nil || out.Solid.Topology().Faces != 0 {
		t.Fatal("unrounded source", e)
	}
	s, _, e = trimmed.FilletWrappedCylinderRims(ctx, s, 0, 1, o)
	if e != nil {
		t.Fatal(e)
	}
	if out, e := BoreWrappedRoundedCylinder(ctx, s, geom.Vec2{}, 1, o); e != nil || out.Solid.Topology().EulerCharacteristic != 0 {
		t.Fatal("one rounded end", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if out, e := BoreWrappedRoundedCylinder(cancelled, s, geom.Vec2{}, 1, o); e != context.Canceled || out.Solid.Topology().Faces != 0 {
		t.Fatal(e)
	}
	if out, e := BoreWrappedRoundedCylinder(ctx, s, geom.Vec2{}, 1, geom.SurfacePatchOptions{MaxOperations: 1}); e == nil || out.Solid.Topology().Faces != 0 {
		t.Fatal("proof limit", e)
	}
}
