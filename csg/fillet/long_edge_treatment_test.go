package fillet

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/collision"
	"testing"
)

func TestPlainCADOppositeLongEdges(t *testing.T) {
	for _, setbacks := range [][2]float64{{1, 1}, {.5, 1.5}} {
		r, e := TreatOppositeLongCapEdges(context.Background(), 20, 10, 10, 1, setbacks[0], setbacks[1], Options{})
		if e != nil {
			t.Fatal(e)
		}
		mass, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		want := 2000 - 20*(1-math.Pi/4) - 10*setbacks[0]*setbacks[1]
		if math.Abs(mass.Volume-want) > 1e-7 || math.Abs(mass.Volume-r.NominalVolume) > 1e-7 || math.Abs(mass.Area-r.NominalSurfaceArea) > 1e-7 {
			t.Fatal("PC2 mass", mass, want)
		}
		ids := []int{r.EndFaces[0], r.EndFaces[1], r.BottomFace, r.BackFace, r.TopFace, r.FrontFace, r.FilletFace, r.ChamferFace}
		seen := map[int]bool{}
		for _, id := range ids {
			if seen[id] {
				t.Fatal("ancestry alias")
			}
			seen[id] = true
		}
		if len(seen) != len(r.Solid.Faces()) {
			t.Fatal("missing ancestry")
		}
		// Independent planar chamfer equation: (Width-Y)/a+(Height-Z)/b=1.
		face := r.Solid.Faces()[r.ChamferFace]
		vertices := r.Solid.Vertices()
		edges := r.Solid.Edges()
		for _, use := range face.Outer {
			for _, id := range []int{edges[use.Edge].Start, edges[use.Edge].End} {
				p := vertices[id]
				if math.Abs((10-p.Y)/setbacks[0]+(10-p.Z)/setbacks[1]-1) > 1e-12 {
					t.Fatal("chamfer plane", p)
				}
			}
		}
		var data bytes.Buffer
		if e := trimmed.WriteJSON(&data, r.Solid); e != nil {
			t.Fatal(e)
		}
		loaded, e := trimmed.ReadJSON(&data, trimmed.JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		m, e := loaded.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .03, MaxTriangles: 100000})
		if e != nil {
			t.Fatal(e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		report, e := collision.SelfIntersectionsBVHContext(context.Background(), m, collision.Options{})
		if e != nil || !report.Complete || len(report.Contacts) != 0 {
			t.Fatal("contacts", e)
		}
	}
}
func TestOppositeLongEdgesRejects(t *testing.T) {
	for _, values := range [][3]float64{{1, 9, 1}, {1, 1, 10}, {10, 1, 1}, {1, -1, 1}, {1, 1, 0}} {
		r, e := TreatOppositeLongCapEdges(context.Background(), 20, 10, 10, values[0], values[1], values[2], Options{})
		if e == nil || len(r.Solid.Faces()) != 0 {
			t.Fatal("accepted", values)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := TreatOppositeLongCapEdges(ctx, 20, 10, 10, 1, 1, 1, Options{}); e == nil {
		t.Fatal("cancel")
	}
}
