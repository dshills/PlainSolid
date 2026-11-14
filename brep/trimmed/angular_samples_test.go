package trimmed

import (
	"context"
	"math"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestCylinderAngularCheckIncludesFacetEnds(t *testing.T) {
	frame, err := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	surface, err := geom.NewCylinderSurface(frame, 3)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		span   float64
		refine bool
	}{{math.Pi / 4, true}, {math.Pi / 8, false}} {
		tri := samples{uv: [3]geom.Vec2{{}, {X: tc.span}, {X: tc.span, Y: 10}}, edge: [3]trimSide{internalSide(), internalSide(), internalSide()}}
		var p [3]geom.Vec3
		for i, uv := range tri.uv {
			p[i], err = surface.Eval(uv.X, uv.Y)
			if err != nil {
				t.Fatal(err)
			}
		}
		err = check(surface, tri, p, .5, .2)
		if tc.refine && err != errRefine {
			t.Fatalf("coarse facet accepted: %v", err)
		}
		if !tc.refine && err != nil {
			t.Fatalf("qualified facet rejected: %v", err)
		}
	}
}

func TestBoreMeshAngularSamples(t *testing.T) {
	base, err := brep.Box(geom.Vec3{X: -10, Y: -5}, geom.Vec3{X: 10, Y: 5, Z: 10})
	if err != nil {
		t.Fatal(err)
	}
	body, _, err := BoreCylinder(base, 0, 1, geom.Vec3{}, 3, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	for _, angle := range []float64{.2, .1} {
		m, err := body.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: angle})
		if err != nil {
			t.Fatal(err)
		}
		walls := 0
		for _, tri := range m.Triangles {
			p := [3]geom.Vec3{m.Vertices[tri[0]], m.Vertices[tri[1]], m.Vertices[tri[2]]}
			if p[0].Z == p[1].Z && p[1].Z == p[2].Z {
				continue
			}
			cylinder := true
			for _, v := range p {
				if math.Abs(math.Hypot(v.X, v.Y)-3) > 1e-8 {
					cylinder = false
				}
			}
			if !cylinder {
				continue
			}
			walls++
			facet, err := normal(p[1].Sub(p[0]), p[2].Sub(p[0]))
			if err != nil {
				t.Fatal(err)
			}
			for _, v := range p {
				analytic := geom.Vec3{X: -v.X / 3, Y: -v.Y / 3}
				deviation := math.Atan2(analytic.Cross(facet).Length(), analytic.Dot(facet))
				if deviation > angle+1e-12 {
					t.Fatalf("facet endpoint angle%g exceeds%g", deviation, angle)
				}
			}
		}
		if walls == 0 {
			t.Fatal("no cylinder triangles checked")
		}
		if err = m.ValidateSolid(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSampledCylinderFallbackAngularBound(t *testing.T) {
	base, err := brep.Box(geom.Vec3{X: -40, Y: -25}, geom.Vec3{X: 40, Y: 25, Z: 4})
	if err != nil {
		t.Fatal(err)
	}
	body, _, err := BoreCylinder(base, 0, 1, geom.Vec3{X: 10, Y: 8}, 2, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	// Exercise the sampled-trim fallback directly. A polygon fan on this wall
	// used to create skew facets whose normals did not converge within budget.
	m, err := body.refineFrom(context.Background(), 3, 3, .05, .2)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, tri := range m.Triangles {
		p := [3]geom.Vec3{m.Vertices[tri[0]], m.Vertices[tri[1]], m.Vertices[tri[2]]}
		if p[0].Z == p[1].Z && p[1].Z == p[2].Z {
			continue
		}
		wall := true
		for _, v := range p {
			if math.Abs(math.Hypot(v.X-10, v.Y-8)-2) > 1e-8 {
				wall = false
			}
		}
		if !wall {
			continue
		}
		checked++
		facet, err := normal(p[1].Sub(p[0]), p[2].Sub(p[0]))
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range p {
			analytic := geom.Vec3{X: -(v.X - 10) / 2, Y: -(v.Y - 8) / 2}
			if angle := math.Atan2(analytic.Cross(facet).Length(), analytic.Dot(facet)); angle > .2+1e-12 {
				t.Fatalf("fallback facet normal angle %g", angle)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no wall triangles checked")
	}
}
