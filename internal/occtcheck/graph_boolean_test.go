package occtcheck

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestOCCTSplineGraphBoolean(t *testing.T) {
	reference := runOracle(t, "graph-boolean")
	k := []float64{0, 0, 0, 1, 1, 1}
	graph := func(c, xSquare float64) *geom.BSplineSurface {
		t.Helper()
		var p []geom.Vec3
		for i := range 3 {
			for j := range 3 {
				z := c
				if i == 2 {
					z += xSquare
				}
				p = append(p, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	ctx := context.Background()
	base, a, b := graph(0, 0), graph(.75, 1), graph(1, 0)
	for _, tc := range []struct {
		name string
		op   parametric.BooleanOp
	}{{"graph-union", parametric.Union}, {"graph-intersection", parametric.Intersection}, {"graph-difference", parametric.Difference}} {
		t.Run(tc.name, func(t *testing.T) {
			s, h, e := trimmed.CombineSplineGraphSolids(ctx, base, a, b, tc.op, 1e-8, intersect.SplinePlaneOptions{})
			if e != nil || len(h.ContourEdges) == 0 || s.Topology().EulerCharacteristic != 2 {
				t.Fatal(h, s.Topology(), e)
			}
			p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			compare(t, reference[tc.name], p.Area, p.Volume, p.Centroid)
		})
	}
}

func TestOCCTSaddleSplineGraphBoolean(t *testing.T) {
	ref := runOracle(t, "saddle-graph-boolean")
	graph := func(z, s, factor float64) *geom.BSplineSurface {
		t.Helper()
		var points []geom.Vec3
		for _, u := range []float64{0, 1} {
			for _, v := range []float64{0, 1} {
				points = append(points, geom.Vec3{X: u, Y: v, Z: z + s*u*v + factor*(u-.5)*(v-.5)})
			}
		}
		k := []float64{0, 0, 1, 1}
		p, e := geom.NewBSplineSurface(1, 1, 2, 2, points, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	base, a, b := graph(0, 0, 0), graph(2, .5, 1), graph(2, .5, 0)
	ctx := context.Background()
	o := intersect.SplinePlaneOptions{Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{MaxCells: 1}}}
	for i, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection} {
		q, h, e := trimmed.CombineSplineGraphSolids(ctx, base, a, b, op, 1e-9, o)
		if e != nil || h.Saddle == nil || len(h.ContourEdges) != 4 {
			t.Fatal(h, e)
		}
		p, e := q.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		compare(t, ref[[]string{"saddle-union", "saddle-common"}[i]], p.Area, p.Volume, p.Centroid)
	}
}

func TestOCCTPartiallyCoincidentSplineGraphBoolean(t *testing.T) {
	ref := runOracle(t, "partial-graph-boolean")
	graph := func(us []float64, z func(float64, float64) float64) *geom.BSplineSurface {
		t.Helper()
		var points []geom.Vec3
		for _, u := range us {
			for _, v := range []float64{0, 1} {
				points = append(points, geom.Vec3{X: u, Y: v, Z: z(u, v)})
			}
		}
		ku := append([]float64{0}, us...)
		ku = append(ku, 1)
		p, e := geom.NewBSplineSurface(1, 1, len(us), 2, points, ku, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	base := graph([]float64{0, 1}, func(_, _ float64) float64 { return 0 })
	a := graph([]float64{0, .5, 1}, func(u, v float64) float64 { return 2 + u*v + max(u-.5, 0) })
	b := graph([]float64{0, 1}, func(u, v float64) float64 { return 2 + u*v })
	ctx := context.Background()
	for i, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection, parametric.Difference} {
		q, h, e := trimmed.CombinePartiallyCoincidentSplineGraphs(ctx, base, a, b, op, 1e-9, intersect.SplinePlaneOptions{}, geom.GraphPointContactOptions{})
		if e != nil || h.ContactPartition == nil || len(h.ContourEdges) != 1 {
			t.Fatal(h, e)
		}
		p, e := q.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		compare(t, ref[[]string{"partial-union", "partial-common", "partial-difference"}[i]], p.Area, p.Volume, p.Centroid)
		if op == parametric.Difference {
			// The pinned Boolean retains additional coincident area in this
			// input configuration. Compare its material volume and centroid
			// separately against the regularized boundary above.
			raw := ref["partial-raw-cut"]
			compare(t, raw, raw.Area, p.Volume, p.Centroid)
			if math.Abs(raw.Area-p.Area) > 1e-6 {
				t.Logf("pinned OCCT cut reports area %.12g; explicit regularized boundary %.12g", raw.Area, p.Area)
			}
		}
	}
}

func TestOCCTEllipticalSplineGraphBoolean(t *testing.T) {
	ref := runOracle(t, "elliptical-graph-boolean")
	graph := func(z, f float64) *geom.BSplineSurface {
		t.Helper()
		var p []geom.Vec3
		q := [3]float64{.25, -.25, .25}
		for i := range 3 {
			for j := range 3 {
				p = append(p, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z + f*(1-16*q[i]-64*q[j])})
			}
		}
		k := []float64{0, 0, 0, 1, 1, 1}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base, a, b := graph(0, 0), graph(24, 1), graph(24, 0)
	ctx := context.Background()
	o := intersect.SplinePlaneOptions{Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{MaxCells: 1}}}
	for i, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection, parametric.Difference} {
		q, h, e := trimmed.CombineSplineGraphSolids(ctx, base, a, b, op, 1e-9, o)
		if e != nil || h.Ellipse == nil || len(h.ContourEdges) != 4 {
			t.Fatal(h, e)
		}
		p, e := q.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if e != nil {
			t.Fatal(e)
		}
		oracle := ref[[]string{"ellipse-union", "ellipse-common", "ellipse-difference"}[i]]
		compare(t, oracle, oracle.Area, p.Volume, p.Centroid)
		// The raw trimmed-face area in pinned OCCT varies at the 1e-4
		// scale when its integration epsilon changes. Independently
		// integrate the known graph metric and all planar walls instead.
		area := ellipticalGraphIndependentArea(op)
		if math.Abs(p.Area-area) > 1e-7 {
			t.Fatalf("ellipse area: Go %.15g, independent %.15g", p.Area, area)
		}
		if math.Abs(oracle.Area-area) > 1e-6 {
			t.Logf("pinned OCCT trimmed area %.12g; independent integral %.12g", oracle.Area, area)
		}
	}
}

// Composite midpoint quadrature with Richardson extrapolation independently
// integrates the exact graph metric over the rectangle and over the ellipse.
// The radial substitution removes the square-root endpoint at ellipse sides.
func ellipticalGraphIndependentArea(op parametric.BooleanOp) float64 {
	integrate := func(n int, ellipse bool) float64 {
		var total float64
		for i := 0; i < n; i++ {
			u := (float64(i) + .5) / float64(n)
			for j := 0; j < n; j++ {
				v := (float64(j) + .5) / float64(n)
				if ellipse {
					angle := 2 * math.Pi * v
					x, y := u*.25*math.Cos(angle), u*.125*math.Sin(angle)
					total += math.Sqrt(1+1024*x*x+16384*y*y) * (math.Pi / 16) * u
				} else {
					x, y := u-.5, v-.5
					total += math.Sqrt(1 + 1024*x*x + 16384*y*y)
				}
			}
		}
		return total / float64(n*n)
	}
	area := func(ellipse bool) float64 {
		a, b, c := integrate(128, ellipse), integrate(256, ellipse), integrate(512, ellipse)
		return ((16*c - 4*b) - (b - a/4)) / 11.25
	}
	inside := area(true)
	planar := math.Pi / 32
	switch op {
	case parametric.Union:
		return 98 + inside - planar // box boundary with replaced planar ellipse
	case parametric.Intersection:
		// Outside height walls: at u=0/1 integrate 21-64(v-.5)^2;
		// at v=0/1 integrate 9-16(u-.5)^2. Plus base and roof.
		return 1 + 2*(21-64.0/12) + 2*(9-16.0/12) + area(false) - inside + planar
	default:
		return inside + planar
	}
}
