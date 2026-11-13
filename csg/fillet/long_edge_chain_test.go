package fillet

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/collision"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestActualPlainCADLongEdgeChain(t *testing.T) {
	bore := trimmed.ExtrusionCircle{Center: geom.Vec2{X: 5, Y: 5}, Radius: 1}
	pocket := trimmed.ExtrusionRectangle{Min: geom.Vec2{X: 10, Y: 5}, Max: geom.Vec2{X: 13, Y: 8}}
	boss := trimmed.ExtrusionBoss{Footprint: trimmed.ExtrusionRectangle{Min: geom.Vec2{X: 15, Y: 5}, Max: geom.Vec2{X: 17, Y: 7}}, Height: 3, Upper: true}
	r, e := TreatLongCapEdgesWithFeatures(context.Background(), 20, 10, 10, 1, 1, 1, bore, pocket, boss, Options{})
	if e != nil {
		t.Fatal(e)
	}
	want := 2000 - 20*(1-math.Pi/4) - 10 - 10*math.Pi - 90 + 12
	m, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(m.Volume-want) > 1e-7 || math.Abs(m.Area-r.NominalSurfaceArea) > 1e-7 {
		t.Fatal("PC2 actual chain mass", e, m, want)
	}
	old := r.Stock.Solid
	if !reflect.DeepEqual(old.Vertices(), r.Solid.Vertices()[:len(old.Vertices())]) || !reflect.DeepEqual(old.Edges(), r.Solid.Edges()[:len(old.Edges())]) {
		t.Fatal("changed old topology")
	}
	for i, f := range old.Faces() {
		if i != r.Stock.TopFace && i != r.Stock.BottomFace && !reflect.DeepEqual(f, r.Solid.Faces()[i]) {
			t.Fatal("lost original treated surface", i)
		}
	}
	if len(r.Solid.Faces()) != len(old.Faces())+13 {
		t.Fatal("ancestry count")
	}
	chi := len(r.Solid.Vertices()) - len(r.Solid.Edges())
	for _, f := range r.Solid.Faces() {
		chi += 1 - len(f.Holes)
	}
	if chi != -2 {
		t.Fatal("expected genus2", chi)
	}
	var data bytes.Buffer
	if e := trimmed.WriteJSON(&data, r.Solid); e != nil {
		t.Fatal(e)
	}
	loaded, e := trimmed.ReadJSON(&data, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	mass, e := loaded.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(mass.Volume-want) > 1e-7 {
		t.Fatal("reload", e, mass)
	}
	mesh, e := loaded.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .05, MaxSubdivisions: 128, MaxTriangles: 200000})
	if e != nil {
		t.Fatal("mesh", e)
	}
	if e := mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	report, e := collision.SelfIntersectionsBVHContext(context.Background(), mesh, collision.Options{})
	if e != nil || !report.Complete || len(report.Contacts) != 0 {
		t.Fatal("contacts", e, len(report.Contacts))
	}
}

func TestActualLongEdgeChainRejects(t *testing.T) {
	bore := trimmed.ExtrusionCircle{Center: geom.Vec2{X: 5, Y: 5}, Radius: 1}
	pocket := trimmed.ExtrusionRectangle{Min: geom.Vec2{X: 10, Y: 5}, Max: geom.Vec2{X: 13, Y: 8}}
	boss := trimmed.ExtrusionBoss{Footprint: trimmed.ExtrusionRectangle{Min: geom.Vec2{X: 15, Y: 5}, Max: geom.Vec2{X: 17, Y: 7}}, Height: 3, Upper: true}
	for i := 0; i < 4; i++ {
		b, p, j := bore, pocket, boss
		switch i {
		case 0:
			b.Center.Y = 2
		case 1:
			p.Min.X = 5
		case 2:
			j.Upper = false
		case 3:
			j.Footprint = p
		}
		r, e := TreatLongCapEdgesWithFeatures(context.Background(), 20, 10, 10, 1, 1, 1, b, p, j, Options{})
		if e == nil || len(r.Solid.Faces()) != 0 {
			t.Fatal("accepted", i)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := TreatLongCapEdgesWithFeatures(ctx, 20, 10, 10, 1, 1, 1, bore, pocket, boss, Options{}); e == nil {
		t.Fatal("cancel")
	}
}
