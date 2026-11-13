package fillet

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
)

func TestOptionalLongEdgesAllStages(t *testing.T) {
	bore := trimmed.ExtrusionCircle{Center: geom.Vec2{X: 5, Y: 5}, Radius: 1}
	pocket := trimmed.ExtrusionRectangle{Min: geom.Vec2{X: 10, Y: 5}, Max: geom.Vec2{X: 13, Y: 8}}
	boss := trimmed.ExtrusionBoss{Footprint: trimmed.ExtrusionRectangle{Min: geom.Vec2{X: 15, Y: 5}, Max: geom.Vec2{X: 17, Y: 7}}, Height: 3, Upper: true}
	for treatments := 0; treatments < 4; treatments++ {
		for features := 0; features < 8; features++ {
			for _, bottomFront := range []bool{false, true} {
				radius, setback := 0., 0.
				if treatments&1 != 0 {
					radius = 1
				}
				if treatments&2 != 0 {
					setback = 1
				}
				var b *trimmed.ExtrusionCircle
				var p *trimmed.ExtrusionRectangle
				var q *trimmed.ExtrusionBoss
				if features&1 != 0 {
					b = &bore
				}
				if features&2 != 0 {
					p = &pocket
				}
				if features&4 != 0 {
					q = &boss
				}
				r, e := TreatLongCapEdgesOptional(context.Background(), 20, 10, 10, radius, setback, setback, bottomFront, b, p, q, Options{})
				if e != nil {
					t.Fatalf("treatments%d features%d bottom%v: %v", treatments, features, bottomFront, e)
				}
				want := 2000 - 20*(1-math.Pi/4)*radius*radius - 10*setback*setback
				if b != nil {
					want -= 10 * math.Pi
				}
				if p != nil {
					want -= 90
				}
				if q != nil {
					want += 12
				}
				if math.Abs(r.NominalVolume-want) > 1e-9 {
					t.Fatal("independent recipe volume", r.NominalVolume, want)
				}
				m, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
				if e != nil || math.Abs(m.Volume-r.NominalVolume) > 1e-7 || math.Abs(m.Area-r.NominalSurfaceArea) > 1e-7 {
					t.Fatal("analytic integration", e, m, r.NominalVolume, r.NominalSurfaceArea)
				}
				{
					mesh, e := r.Solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 200000})
					if e != nil {
						t.Fatal("mesh", treatments, features, bottomFront, e)
					}
					if e = mesh.ValidateSolid(); e != nil {
						t.Fatal(e)
					}
				}

				if (b == nil) != (r.BoreFaces[0] == -1) || (p == nil) != (r.PocketFaces[0] == -1) || (q == nil) != (r.BossFaces[0] == -1) {
					t.Fatal("optional face ids")
				}
			}
		}
	}
}

func TestOptionalLongEdgesRejects(t *testing.T) {
	ctx := context.Background()
	for _, a := range [][6]float64{{20, 10, 10, -1, 0, 0}, {20, 10, 10, 1, 1, 0}, {20, 10, 10, 9, 1, 1}, {20, 10, 10, 1, 1, 9}, {20, 10, 10, math.Inf(1), 0, 0}} {
		if _, e := TreatLongCapEdgesOptional(ctx, a[0], a[1], a[2], a[3], a[4], a[5], true, nil, nil, nil, Options{}); e == nil {
			t.Fatal("accepted invalid", a)
		}
	}
	b := trimmed.ExtrusionCircle{Center: geom.Vec2{X: 5, Y: 1.5}, Radius: 1}
	if _, e := TreatLongCapEdgesOptional(ctx, 20, 10, 10, 1, 2, 2, true, &b, nil, nil, Options{}); e == nil {
		t.Fatal("accepted bore crossing bottom chamfer")
	}
	if _, e := TreatOppositeLongCapEdges(ctx, 20, 10, 10, 0, 1, 1, Options{}); e == nil {
		t.Fatal("weakened original constructor")
	}
	if _, e := PrismCorners(ctx, []geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}, 0, 1, nil, Options{}); e == nil {
		t.Fatal("weakened original selection")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := TreatLongCapEdgesOptional(cancelled, 20, 10, 10, 0, 0, 0, true, nil, nil, nil, Options{}); e != context.Canceled {
		t.Fatal(e)
	}
}

func TestOptionalLongEdgesBottomFrontPlacement(t *testing.T) {
	r, e := TreatLongCapEdgesOptional(context.Background(), 20, 10, 10, 1, 2, 3, true, nil, nil, nil, Options{})
	if e != nil {
		t.Fatal(e)
	}
	face := r.Solid.Faces()[r.Stock.ChamferFace]
	edges, vertices := r.Solid.Edges(), r.Solid.Vertices()
	for _, u := range face.Outer {
		p := vertices[edges[u.Edge].Start]
		if math.Abs(p.Y/2+p.Z/3-1) > 1e-12 || p.Y < 0 || p.Y > 2 || p.Z < 0 || p.Z > 3 {
			t.Fatal("chamfer is not bottom/front", p)
		}
	}
	for _, id := range []int{r.Stock.TopFace, r.Stock.BottomFace} {
		for _, u := range r.Solid.Faces()[id].Outer {
			p := vertices[edges[u.Edge].Start]
			if id == r.Stock.TopFace && p.Z != 10 || id == r.Stock.BottomFace && p.Z != 0 {
				t.Fatal("cap position", p)
			}
		}
	}
}
