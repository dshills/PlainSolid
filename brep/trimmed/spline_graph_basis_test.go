//go:build research

package trimmed

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestCombineSplineGraphDifferentRationalBases(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	flat := func(z float64) *geom.BSplineSurface {
		t.Helper()
		s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Z: z}, {Y: 1, Z: z}, {X: 1, Z: z}, {X: 1, Y: 1, Z: z}}, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	// Independently construct B(u,v) = (u,v,.75+.5*u) with homogeneous
	// denominator .75 + .75*u*(1-u), using a cubic basis in U. The
	// weights and Cartesian poles are exactly represented binary fractions.
	b, e := geom.NewBSplineSurface(3, 1, 4, 2, []geom.Vec3{
		{Z: .75}, {Y: 1, Z: .75}, {X: .25, Z: .875}, {X: .25, Y: 1, Z: .875},
		{X: .75, Z: 1.125}, {X: .75, Y: 1, Z: 1.125}, {X: 1, Z: 1.25}, {X: 1, Y: 1, Z: 1.25},
	}, []float64{0, 0, 0, 0, 1, 1, 1, 1}, k, []float64{.75, .75, 1, 1, 1, 1, .75, .75})
	if e != nil {
		t.Fatal(e)
	}
	base, a := flat(0), flat(1)
	for _, tc := range []struct {
		op     parametric.BooleanOp
		volume float64
	}{{parametric.Union, 1.0625}, {parametric.Intersection, .9375}, {parametric.Difference, .0625}} {
		s, h, e := CombineSplineGraphSolids(ctx, base, a, b, tc.op, 1e-8, intersect.SplinePlaneOptions{})
		if e != nil || len(h.ContourEdges) == 0 || s.Topology().EulerCharacteristic != 2 {
			t.Fatal(tc, h, e)
		}
		m, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(m.Volume-tc.volume) > 1e-9 {
			t.Fatal(tc, m, e)
		}
	}
	// The rational graph is accepted in either operand slot and by the
	// strictly ordered fill path used when the tops do not intersect.
	if _, _, e = CombineSplineGraphSolids(ctx, base, b, a, parametric.Union, 1e-8, intersect.SplinePlaneOptions{}); e != nil {
		t.Fatal(e)
	}
	s, _, e := FillSplineGraphPair(b, base, 1e-8, intersect.SplinePlaneOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(m.Volume-1) > 1e-9 {
		t.Fatal(m, e)
	}
	placement := geom.Identity()
	placement[0][0], placement[0][1], placement[0][3] = 2, 1, -3
	placement[1][1], placement[1][3] = .5, 2
	placed := func(s *geom.BSplineSurface, d geom.ParameterRectangle) *geom.BSplineSurface {
		t.Helper()
		p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		p, e = p.Transformed(placement)
		if e != nil {
			t.Fatal(e)
		}
		p, e = p.Reparameterize(d, false, false, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		q, e := p.ToBSplineSurface()
		if e != nil {
			t.Fatal(e)
		}
		return q
	}
	d0 := geom.ParameterRectangle{UMin: 2, UMax: 6, VMin: 3, VMax: 5}
	d1 := geom.ParameterRectangle{UMin: 10, UMax: 12, VMin: -2, VMax: 2}
	d2 := geom.ParameterRectangle{UMax: 2, VMin: 4, VMax: 8}
	x, y, z := placed(base, d0), placed(a, d1), placed(b, d2)
	s, _, e = CombinePlacedSplineGraphSolids(ctx, x, y, z, parametric.Union, 1e-8, intersect.SplinePlaneOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m, e = s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(m.Volume-1.0625) > 1e-9 {
		t.Fatal(m, e)
	}
	z, e = z.Transposed()
	if e != nil {
		t.Fatal(e)
	}
	z, e = z.ReversedV()
	if e != nil {
		t.Fatal(e)
	}
	s, _, e = CombineAlignedSplineGraphSolids(ctx, x, y, z, parametric.Intersection, 1e-8, intersect.SplinePlaneOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m, e = s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(m.Volume-.9375) > 1e-9 {
		t.Fatal(m, e)
	}
}
