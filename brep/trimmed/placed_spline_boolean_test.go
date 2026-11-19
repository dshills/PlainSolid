//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestPlacedSplineGraphBoolean(t *testing.T) {
	k := []float64{0, 0, 0, 1, 1, 1}
	placement := geom.Affine{{-2, .5, 0, 3}, {0, 1.5, 0, 2}, {0, 0, 1, 0}}
	build := func(c, q float64, perturb bool) *geom.BSplineSurface {
		t.Helper()
		var p []geom.Vec3
		for i := range 3 {
			for j := range 3 {
				z := c
				if i == 2 {
					z += q
				}
				v := placement.Apply(geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z})
				if perturb && i == 1 && j == 1 {
					v.X += .01
				}
				p = append(p, v)
			}
		}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	ctx := context.Background()
	base, a, b := build(0, 0, false), build(.75, 1, false), build(1, 0, false)
	s, h, e := CombinePlacedSplineGraphSolids(ctx, base, a, b, parametric.Union, 1e-8, intersect.SplinePlaneOptions{})
	if e != nil || h.TopA < 0 || h.TopB < 0 || s.Topology().EulerCharacteristic != 2 {
		t.Fatal(h, s.Topology(), e)
	}
	var data bytes.Buffer
	if e = WriteJSON(&data, s); e != nil {
		t.Fatal(e)
	}
	s, e = ReadJSON(&data, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	v := 3 * 7. / 6
	centroid := placement.Apply(geom.Vec3{X: (1./2 + 9./64) / (7. / 6), Y: .5, Z: 339. / 560})
	if e != nil || math.Abs(p.Volume-v) > 1e-8 || p.Centroid.Distance(centroid) > 1e-8 {
		t.Fatal(p, v, centroid, e)
	}
	refine := func(s *geom.BSplineSurface) *geom.BSplineSurface {
		t.Helper()
		x, e := s.InsertUKnot(.75, 1)
		if e != nil {
			t.Fatal(e)
		}
		return x
	}
	ra, e := refine(a).ReversedU()
	if e != nil {
		t.Fatal(e)
	}
	rb, e := refine(b).Transposed()
	if e != nil {
		t.Fatal(e)
	}
	rb, e = rb.ReversedV()
	if e != nil {
		t.Fatal(e)
	}
	s, _, e = CombineAlignedSplineGraphSolids(ctx, refine(base), ra, rb, parametric.Union, 1e-8, intersect.SplinePlaneOptions{})
	if e != nil || s.Topology().EulerCharacteristic != 2 {
		t.Fatal(s.Topology(), e)
	}
	p, e = s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-v) > 1e-8 || p.Centroid.Distance(centroid) > 1e-8 {
		t.Fatal(p, v, centroid, e)
	}

	s, _, e = CombinePlacedSplineGraphSolids(ctx, refine(base), refine(a), refine(b), parametric.Union, 1e-8, intersect.SplinePlaneOptions{})
	if e != nil || s.Topology().EulerCharacteristic != 2 {
		t.Fatal(s.Topology(), e)
	}
	p, e = s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-v) > 1e-8 || p.Centroid.Distance(centroid) > 1e-8 {
		t.Fatal(p, v, centroid, e)
	}
	if _, _, e = CombinePlacedSplineGraphSolids(ctx, base, build(.75, 1, true), b, parametric.Union, 1e-8, intersect.SplinePlaneOptions{}); e == nil {
		t.Fatal("accepted nonaffine or unmatched XY chart")
	}
}
