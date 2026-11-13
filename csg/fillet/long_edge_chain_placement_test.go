package fillet

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/brep/placement"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange"
	"plainsolid/geom"
	"testing"
)

func TestActualChainPlacedNativeSTL(t *testing.T) {
	ctx := context.Background()
	r, e := TreatLongCapEdgesWithFeatures(ctx, 20, 10, 10, 1, 1, 1, trimmed.ExtrusionCircle{Center: geom.Vec2{X: 5, Y: 5}, Radius: 1}, trimmed.ExtrusionRectangle{Min: geom.Vec2{X: 10, Y: 5}, Max: geom.Vec2{X: 13, Y: 8}}, trimmed.ExtrusionBoss{Footprint: trimmed.ExtrusionRectangle{Min: geom.Vec2{X: 15, Y: 5}, Max: geom.Vec2{X: 17, Y: 7}}, Height: 3, Upper: true}, Options{})
	if e != nil {
		t.Fatal(e)
	}
	frame, e := placement.New([3]int{-2, 1, 3}, geom.Vec3{X: 32, Y: -16, Z: 8})
	if e != nil {
		t.Fatal(e)
	}
	m, _ := frame.Matrix()
	s, e := r.Solid.Transformed(m)
	if e != nil {
		t.Fatal(e)
	}
	a, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(a.Volume-b.Volume) > 1e-7 || math.Abs(a.Area-b.Area) > 1e-7 || b.Centroid.Sub(m.Apply(a.Centroid)).Length() > 1e-7 {
		t.Fatal("placed mass", a, b)
	}
	var native bytes.Buffer
	if e := trimmed.WriteJSON(&native, s); e != nil {
		t.Fatal(e)
	}
	s, e = trimmed.ReadJSON(&native, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	mesh, e := s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .05, MaxSubdivisions: 128, MaxTriangles: 200000})
	if e != nil {
		t.Fatal(e)
	}
	if e := mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	var stl bytes.Buffer
	if e := exchange.WriteSTL(&stl, mesh); e != nil {
		t.Fatal(e)
	}
	round, e := exchange.ReadSTL(&stl, exchange.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if e := round.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if round.SignedVolume() <= 0 || math.Abs(round.SignedVolume()-mesh.SignedVolume()) > 1e-3 {
		t.Fatal("positive STL volume")
	}
}
