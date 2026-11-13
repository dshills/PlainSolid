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

func TestOCCTSimultaneousSplineShells(t *testing.T) {
	ref := runOracle(t, "simultaneous-spline-shell")
	ctx := context.Background()
	makeShell := func(transform geom.Affine) trimmed.Solid {
		var faces []*geom.BSplineSurface
		var domains []geom.ParameterRectangle
		var reverse []bool
		for axis := range 3 {
			for end := range 2 {
				u, v := (axis+1)%3, (axis+2)%3
				var samples [3][3]geom.Vec3
				for i := range 3 {
					for j := range 3 {
						var xyz [3]float64
						xyz[axis], xyz[u], xyz[v] = float64(end), float64(i)/2, float64(j)/2
						p := transform.Apply(geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]})
						p.Z += (p.X - p.Y) * (p.X - p.Y) / 64
						samples[i][j] = p
					}
				}
				// Convert an independent grid of exact polynomial samples
				// to quadratic Bernstein poles in each direction.
				middle := func(a, b, c geom.Vec3) geom.Vec3 { return b.Scale(2).Sub(a.Add(c).Scale(.5)) }
				for j := range 3 {
					samples[1][j] = middle(samples[0][j], samples[1][j], samples[2][j])
				}
				var points []geom.Vec3
				for i := range 3 {
					samples[i][1] = middle(samples[i][0], samples[i][1], samples[i][2])
					points = append(points, samples[i][:]...)
				}
				k := []float64{0, 0, 0, 1, 1, 1}
				f, e := geom.NewBSplineSurface(2, 2, 3, 3, points, k, k, nil)
				if e != nil {
					t.Fatal(e)
				}
				faces = append(faces, f)
				domains = append(domains, geom.ParameterRectangle{UMax: 1, VMax: 1})
				reverse = append(reverse, end == 0)
			}
		}
		s, _, e := trimmed.SewSplineRectangles(ctx, faces, domains, reverse, trimmed.SewOptions{Tolerance: 1e-9})
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	a := makeShell(geom.Identity())
	b := makeShell(geom.Affine{{0, .25, .25, .875}, {.25, 0, .25, .875}, {.25, .25, 0, .5}})
	o := trimmed.SplineShellCombineOptions{DiscoverVerticalQuadraticWarp: true, DisablePlaneDiscovery: true}
	o.Open.Trace.Coverage = intersect.SplineTubeCoverageOptions{Cell: geom.SurfaceTubeUnionCoverageOptions{MaxDepth: 16, MaxOperations: 12000000}}
	for i, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection, parametric.Difference} {
		s, h, e := trimmed.CombineSplineShells(ctx, a, b, op, o)
		if e != nil || h.Method != "vertical-quadratic-warp-arrangement" || len(h.SectionEdges) != 8 {
			t.Fatal(op, h.Method, e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		r, ok := ref[[]string{"simultaneous-union", "simultaneous-common", "simultaneous-cut"}[i]]
		if !ok || e != nil || math.Abs(p.Volume-r.Volume) > 3e-7 || math.Abs(p.Area-r.Area) > 3e-6 ||
			p.Centroid.Distance(geom.Vec3{X: r.Centroid[0], Y: r.Centroid[1], Z: r.Centroid[2]}) > 3e-6 {
			t.Fatal(op, p, r, e)
		}
	}
}
