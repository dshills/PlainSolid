package trimmed

import (
	"context"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestMultispanEllipticalContact(t *testing.T) {
	ctx := context.Background()
	// E=1-16(u-.5)^2-64(v-.5)^2 is represented on two knot spans.
	// Both original supports are independently checked along each shared arc.
	kv := []float64{0, 0, 0, 1, 1, 1}
	makeSurface := func(h float64, upper bool) *geom.BSplineSurface {
		t.Helper()
		var p []geom.Vec3
		for i, u := range []float64{0, .25, .5, .75, 1} {
			for j, v := range []float64{0, .5, 1} {
				z := h
				if upper {
					z += 1 - [5]float64{4, 0, 0, 0, 4}[i] - [3]float64{16, -16, 16}[j]
				}
				p = append(p, geom.Vec3{X: u, Y: v, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(2, 2, 5, 3, p, []float64{0, 0, 0, .5, .5, 1, 1, 1}, kv, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	a, b, base := makeSurface(24, true), makeSurface(24, false), makeSurface(0, false)
	hint := geom.GraphEllipseHint{Center: [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)},
		RadiusSquared: [2]*big.Rat{big.NewRat(1, 16), big.NewRat(1, 64)}}
	s, h, e := CombineEllipticalContactSplineGraphs(ctx, base, a, b, parametric.Difference, 1e-9, intersect.SplinePlaneOptions{}, hint, geom.GraphPointContactOptions{})
	if e != nil || h.Ellipse == nil || len(h.Ellipse.CertifiedCells) != 2 || len(h.ContourEdges) != 4 {
		t.Fatal(h, s.Topology(), e)
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil || math.Abs(p.Volume-math.Pi/64) > 1e-8 {
		t.Fatal(p, e)
	}
}
