package fillet

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestSelectiveUnequalFilletIndependent(t *testing.T) {
	ctx := context.Background()
	profile := []geom.Vec2{{}, {X: 4}, {X: 4, Y: 3}, {Y: 3}}
	selection := []CornerRadius{{Corner: 2, Radius: .5}, {Corner: 0, Radius: .25}}
	r, e := PrismCorners(ctx, profile, 0, 2, selection, Options{})
	if e != nil {
		t.Fatal(e)
	}
	c := 1 - math.Pi/4
	area := 12 - c*(.25*.25+.5*.5)
	perimeter := 14 - (2-math.Pi/2)*(.25+.5)
	if math.Abs(r.NominalArea-area) > 1e-13 || math.Abs(r.NominalPerimeter-perimeter) > 1e-13 || math.Abs(r.NominalVolume-2*area) > 1e-13 {
		t.Fatal("nominal formulas", r.NominalArea, r.NominalPerimeter, r.NominalVolume)
	}
	// Integrate the removed square-minus-quarter-disk first moments.
	k := (5./6 - math.Pi/4) / c
	want := geom.Vec3{X: (24 - c*.25*.25*(k*.25) - c*.5*.5*(4-k*.5)) / area, Y: (18 - c*.25*.25*(k*.25) - c*.5*.5*(3-k*.5)) / area, Z: 1}
	p, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-2*area) > 1e-8 || math.Abs(p.Area-(2*area+2*perimeter)) > 1e-8 || p.Centroid.Sub(want).Length() > 1e-9 {
		t.Fatal("independent mass", p, want, e)
	}
	if len(r.Corners) != 4 || r.Corners[1] != nil || r.Corners[3] != nil || r.Corners[0] == nil || r.Corners[2] == nil || len(r.StraightFaces) != 4 {
		t.Fatal("selection ancestry")
	}
	faces, edges, vertices := r.Solid.Faces(), r.Solid.Edges(), r.Solid.Vertices()
	seen := map[int]bool{}
	add := func(id int) {
		if id < 0 || id >= len(faces) || seen[id] {
			t.Fatal("duplicate/invalid face ancestry", id)
		}
		seen[id] = true
	}
	for _, id := range r.Caps {
		add(id)
	}
	for _, id := range r.StraightFaces {
		add(id)
	}
	for _, s := range selection {
		corner := r.Corners[s.Corner]
		add(corner.CylinderFace)
		cylinder, ok := faces[corner.CylinderFace].Surface.(geom.CylinderSurface)
		if !ok {
			t.Fatal("fillet lost analytic cylinder")
		}
		frame, radius := cylinder.Parameters()
		if math.Abs(radius-s.Radius) > 1e-14 || math.Abs(frame.Origin().X-corner.Center.X) > 1e-12 || math.Abs(frame.Origin().Y-corner.Center.Y) > 1e-12 || math.Abs(corner.Sweep-math.Pi/2) > 1e-14 || math.Abs(corner.Setback-s.Radius) > 1e-14 {
			t.Fatal("corner mapping", corner)
		}
		for level, id := range []int{corner.BottomArc, corner.TopArc} {
			if id < 0 || id >= len(edges) {
				t.Fatal("arc index", id)
			}
			edge := edges[id]
			a, b := vertices[edge.Start], vertices[edge.End]
			if math.Abs(a.Z-2*float64(level)) > 1e-12 || math.Abs(b.Z-a.Z) > 1e-12 {
				t.Fatal("cap arc ancestry", a, b)
			}
			match := func(p geom.Vec3, q geom.Vec2) bool { return math.Hypot(p.X-q.X, p.Y-q.Y) < 1e-9 }
			if !(match(a, corner.Incoming) && match(b, corner.Outgoing) || match(b, corner.Incoming) && match(a, corner.Outgoing)) {
				t.Fatal("arc does not meet tangent endpoints", a, b)
			}
		}
	}
	if len(seen) != len(faces) || len(faces) != 8 || r.Solid.Topology().EulerCharacteristic != 2 {
		t.Fatal("incomplete topology ancestry")
	}
	for _, corner := range []geom.Vec2{profile[1], profile[3]} {
		found := false
		for _, edge := range edges {
			a, b := vertices[edge.Start], vertices[edge.End]
			if a.X == corner.X && a.Y == corner.Y && b.X == corner.X && b.Y == corner.Y && math.Abs(a.Z-b.Z) == 2 {
				found = true
			}
		}
		if !found {
			t.Fatal("unselected sharp edge lost", corner)
		}
	}
	mesh, e := r.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .002, MaxSubdivisions: 128, MaxTriangles: 100000})
	if e != nil {
		t.Fatal(e)
	}
	if e = mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if math.Abs(mesh.SignedVolume()-2*area) > .01 {
		t.Fatal("mesh mass", mesh.SignedVolume())
	}
}

func TestSelectiveUnequalFilletIndependentRejection(t *testing.T) {
	profile := []geom.Vec2{{}, {X: 4}, {X: 4, Y: 3}, {Y: 3}}
	for _, selection := range [][]CornerRadius{{{Corner: 0, Radius: .25}, {Corner: 0, Radius: .5}}, {{Corner: 0, Radius: 4}}, {{Corner: 0, Radius: 0}}, {{Corner: -1, Radius: .25}}} {
		r, e := PrismCorners(context.Background(), profile, 0, 2, selection, Options{})
		if e == nil || len(r.Solid.Faces()) != 0 {
			t.Fatal("invalid selection accepted/partial result", selection, e)
		}
	}
	if _, e := PrismCorners(context.Background(), profile, 0, 2, []CornerRadius{{Corner: 0, Radius: 4}}, Options{}); !errors.Is(e, ErrRadius) {
		t.Fatal("large radius error", e)
	}
}
