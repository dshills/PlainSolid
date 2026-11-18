package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestCertifiedAxisLineSplineGraphBoolean(t *testing.T) {
	ctx := context.Background()
	graph := func(du int, z func(float64, float64) float64) *geom.BSplineSurface {
		t.Helper()
		var p []geom.Vec3
		for i := range du + 1 {
			u := float64(i) / float64(du)
			for _, v := range []float64{0, 1} {
				p = append(p, geom.Vec3{X: u, Y: v, Z: z(u, v)})
			}
		}
		ku := make([]float64, 2*du+2)
		for i := du + 1; i < len(ku); i++ {
			ku[i] = 1
		}
		s, e := geom.NewBSplineSurface(du, 1, du+1, 2, p, ku, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base := graph(1, func(_, _ float64) float64 { return 0 })
	a := graph(1, func(u, v float64) float64 { return 2 + u*v + 2*u - .5 })
	b := graph(2, func(u, v float64) float64 { return 2 + u*v })
	o := intersect.SplinePlaneOptions{Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{MaxCells: 1}}}
	for i, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection, parametric.Difference} {
		s, h, e := CombineAxisLineSplineGraphs(ctx, base, a, b, op, 0, 1e-9, o, geom.GraphPointContactOptions{})
		if e != nil || h.AxisLine == nil || h.AxisLine.Coordinate.Cmp(big.NewRat(1, 4)) != 0 || h.AxisLine.OnKnot ||
			len(h.ContourEdges) != 1 || s.Topology().EulerCharacteristic != 2 {
			t.Fatal(s.Topology(), h, e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		want := [3]float64{45.0 / 16, 35.0 / 16, 9.0 / 16}[i]
		if e != nil || math.Abs(p.Volume-want) > 1e-9 {
			t.Fatal(p, want, e)
		}
	}
	a = graph(1, func(u, v float64) float64 { return 1 + u*v + 3*u })
	s, h, e := CombineAxisLineSplineGraphs(ctx, base, a, b, parametric.Union, 0, 1e-9, o, geom.GraphPointContactOptions{})
	if !errors.Is(e, geom.ErrConvergence) || h.AxisLine == nil || h.AxisLine.Coordinate.Cmp(big.NewRat(1, 3)) != 0 || s.Topology().Faces != 0 {
		t.Fatal("the nonbinary contact should be exact in the report and not rounded into a shell", s.Topology(), h, e)
	}
	material, e := NewSharedBaseSplineGraphMaterial(ctx, base, a, b, o)
	if e != nil {
		t.Fatal(e)
	}
	section, e := material.SectionEnvelopeVertical(ctx, [2]*big.Rat{big.NewRat(1, 3), big.NewRat(1, 2)}, parametric.Intersection, geom.SurfacePatchEvaluationOptions{})
	if e != nil || section.Lower.Sign() != 0 || section.Upper.Cmp(big.NewRat(13, 6)) != 0 ||
		len(section.Faces) != 2 || section.SideWall {
		t.Fatal(section, e)
	}
	for _, c := range []struct {
		p    [3]*big.Rat
		op   parametric.BooleanOp
		want parametric.Location
	}{
		{[3]*big.Rat{big.NewRat(1, 3), big.NewRat(1, 2), big.NewRat(13, 6)}, parametric.Union, parametric.Boundary},
		{[3]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2), big.NewRat(5, 2)}, parametric.Union, parametric.Inside},
		{[3]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2), big.NewRat(5, 2)}, parametric.Intersection, parametric.Outside},
		{[3]*big.Rat{big.NewRat(1, 2), big.NewRat(0, 1), big.NewRat(1, 1)}, parametric.Intersection, parametric.Boundary},
	} {
		r, e := material.LocateEnvelopeRational(ctx, c.p, c.op, geom.SurfacePatchEvaluationOptions{})
		if e != nil || r != c.want {
			t.Fatal(c, r, e)
		}
	}
	diff, present, e := material.SectionDifferenceVertical(ctx, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, geom.SurfacePatchEvaluationOptions{})
	if e != nil || !present || diff.Lower.Cmp(big.NewRat(9, 4)) != 0 || diff.Upper.Cmp(big.NewRat(11, 4)) != 0 {
		t.Fatal(diff, present, e)
	}
	if _, present, e := material.SectionDifferenceVertical(ctx, [2]*big.Rat{big.NewRat(1, 3), big.NewRat(1, 2)}, geom.SurfacePatchEvaluationOptions{}); e != nil || present {
		t.Fatal("zero-thickness difference section", present, e)
	}

	ray, e := material.IntersectEnvelopeVerticalLine(ctx, geom.Vec3{X: 1, Y: .5, Z: 6}, -3, 0, 2, parametric.Union, geom.SurfacePatchEvaluationOptions{})
	if e != nil || len(ray.Hits) != 2 || ray.Coincident == nil || ray.Hits[0].Parameter.Cmp(big.NewRat(1, 2)) != 0 ||
		ray.Hits[0].Base || !ray.Hits[1].Base || ray.Hits[1].Parameter.Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal(ray, e)
	}
	// 8*(u-1/4)*(1+u) has a strict positive quotient but is
	// quadratic in U. The exact proof must go beyond affine slopes.
	a = graph(2, func(u, v float64) float64 { return 2 + u*v + [3]float64{-2, 1, 12}[int(u*2)] })
	s, h, e = CombineAxisLineSplineGraphs(ctx, base, a, b, parametric.Union, 0, 1e-9, o, geom.GraphPointContactOptions{})
	// Lift the pair if the base touch is rejected: the proof is defined
	// for strictly positive input volumes.
	if e == nil || s.Topology().Faces != 0 {
		t.Fatal("shared base is not strictly below A", h, e)
	}
	a = graph(2, func(u, v float64) float64 { return 4 + u*v + [3]float64{-2, 1, 12}[int(u*2)] })
	b = graph(2, func(u, v float64) float64 { return 4 + u*v })
	s, h, e = CombineAxisLineSplineGraphs(ctx, base, a, b, parametric.Difference, 0, 1e-9, o, geom.GraphPointContactOptions{})
	if e != nil || h.AxisLine == nil || h.AxisLine.Coordinate.Cmp(big.NewRat(1, 4)) != 0 {
		t.Fatal(s.Topology(), h, e)
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-63.0/16) > 1e-9 {
		t.Fatal(p, e)
	}

}

func TestParallelLineSplineGraphBoolean(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	surface := func(axis int, coefficients []float64, lift float64) *geom.BSplineSurface {
		t.Helper()
		var p []geom.Vec3
		for i := range len(coefficients) {
			x := float64(i) / float64(len(coefficients)-1)
			for _, y := range []float64{0, 1} {
				u, v := x, y
				if axis == 1 {
					u, v = y, x
				}
				p = append(p, geom.Vec3{X: u, Y: v, Z: lift + coefficients[i]*(1+y) + u*v})
			}
		}
		n := len(coefficients)
		k := make([]float64, 2*n)
		for i := n; i < len(k); i++ {
			k[i] = 1
		}
		d := []float64{0, 0, 1, 1}
		if axis == 1 {
			// Transpose the small control net without changing the chart.
			r := make([]geom.Vec3, len(p))
			for i := range n {
				for j, y := range []float64{0, 1} {
					x := float64(i) / float64(n-1)
					r[j*n+i] = geom.Vec3{X: y, Y: x, Z: lift + coefficients[i]*(1+y) + x*y}
				}
			}
			p = r
		}
		var s *geom.BSplineSurface
		var e error
		if axis == 0 {
			s, e = geom.NewBSplineSurface(n-1, 1, n, 2, p, k, d, nil)
		} else {
			s, e = geom.NewBSplineSurface(1, n-1, 2, n, p, d, k, nil)
		}
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base := surface(0, []float64{0, 0}, 0)
	o := intersect.SplinePlaneOptions{Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{MaxCells: 1}}}
	for axis := range 2 {
		// 16(t-1/4)(t-3/4)(1+s) changes sign in three bands.
		a, b := surface(axis, []float64{3, -5, 3}, 8), surface(axis, []float64{0, 0}, 8)
		for i, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection, parametric.Difference} {
			s, h, e := CombineParallelLineSplineGraphs(ctx, base, a, b, op, axis, 1e-9, o, geom.GraphFactorAxisLineOptions{})
			wantShells := 1
			if op == parametric.Difference {
				wantShells = 2
			}
			if e != nil || h.ParallelLines == nil || !h.ParallelLines.Proven ||
				len(h.ContourEdges) != 2 || len(s.Topology().Shells) != wantShells {
				t.Fatal(axis, op, h, s.Topology(), e)
			}
			p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
			want := [3]float64{9, 7.5, 1}[i]
			if e != nil || math.Abs(p.Volume-want) > 1e-9 {
				t.Fatal(axis, op, p, want, e)
			}
		}
		if _, h, e := CombineSplineGraphSolids(ctx, base, a, b, parametric.Union, 1e-9, o); e != nil || h.ParallelLines == nil {
			t.Fatal("automatic parallel fallback", h, e)
		}
	}
}

func TestRepeatedAxisContactSplineGraphBoolean(t *testing.T) {
	ctx := context.Background()
	graph := func(z []float64, lift float64) *geom.BSplineSurface {
		t.Helper()
		var p []geom.Vec3
		for i, h := range z {
			for _, v := range []float64{0, 1} {
				u := float64(i) / 2
				p = append(p, geom.Vec3{X: u, Y: v, Z: lift + h*(1+v)})
			}
		}
		s, e := geom.NewBSplineSurface(2, 1, 3, 2, p, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base := graph([]float64{0, 0, 0}, 0)
	a := graph([]float64{1, -2, 4}, 5) // 9*(u-1/3)^2*(1+v)
	b := graph([]float64{0, 0, 0}, 5)
	opts := intersect.SplinePlaneOptions{Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{MaxCells: 1}}}
	for i, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection, parametric.Difference} {
		s, h, e := CombineAxisLineSplineGraphs(ctx, base, a, b, op, 0, 1e-9, opts, geom.GraphPointContactOptions{})
		if h.AxisLine == nil || h.AxisLine.Coordinate.Cmp(big.NewRat(1, 3)) != 0 || h.AxisMultiplicity != 2 {
			t.Fatal(h, e)
		}
		if op == parametric.Difference {
			if !errors.Is(e, geom.ErrDegenerate) || s.Topology().Faces != 0 {
				t.Fatal("pinched result", s.Topology(), e)
			}
			continue
		}
		if e != nil || len(h.ContourEdges) != 0 || s.Topology().Faces != 6 {
			t.Fatal(s.Topology(), h, e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(p.Volume-[]float64{6.5, 5}[i]) > 1e-9 {
			t.Fatal(p, e)
		}
	}
	s, h, e := CombineAxisLineSplineGraphs(ctx, base, b, a, parametric.Difference, 0, 1e-9, opts, geom.GraphPointContactOptions{})
	if e != nil || h.AxisMultiplicity != 2 || s.Topology().Faces != 0 {
		t.Fatal(s.Topology(), h, e)
	}
}

func TestCrossingLineSplineGraphBoolean(t *testing.T) {
	ctx := context.Background()
	makeGraph := func(du int, z func(int, int) float64) *geom.BSplineSurface {
		t.Helper()
		ku := make([]float64, 2*du+2)
		for i := du + 1; i < len(ku); i++ {
			ku[i] = 1
		}
		var p []geom.Vec3
		for i := range du + 1 {
			for j := range 2 {
				p = append(p, geom.Vec3{X: float64(i) / float64(du), Y: float64(j), Z: z(i, j)})
			}
		}
		s, e := geom.NewBSplineSurface(du, 1, du+1, 2, p, ku, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base := makeGraph(1, func(_, _ int) float64 { return 0 })
	b := makeGraph(1, func(_, _ int) float64 { return 10 })
	// A-B=8(u-1/4)(1+u)(v-1/2); height is genuinely cubic as a
	// polynomial in the two variables and outside the quadratic saddle case.
	a := makeGraph(2, func(i, j int) float64 {
		return 10 + [3]float64{-2, 1, 12}[i]*(float64(j)-.5)
	})
	o := intersect.SplinePlaneOptions{Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{MaxCells: 1}}}
	for _, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection, parametric.Difference} {
		s, h, e := CombineSplineGraphSolids(ctx, base, a, b, op, 1e-9, o)
		if h.CrossingLines == nil || h.CrossingLines.Crossing[0].Cmp(big.NewRat(1, 4)) != 0 ||
			h.CrossingLines.Crossing[1].Cmp(big.NewRat(1, 2)) != 0 {
			t.Fatal(h, e)
		}
		if op == parametric.Difference {
			if !errors.Is(e, geom.ErrDegenerate) || s.Topology().Faces != 0 {
				t.Fatal("pinched crossing subtraction", s.Topology(), e)
			}
			continue
		}
		if e != nil || len(h.ContourEdges) != 4 || len(h.FacesA) != 2 || len(h.FacesB) != 2 ||
			s.Topology().EulerCharacteristic != 2 {
			t.Fatal(s.Topology(), h, e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		want := 10 + 101.0/192
		if op == parametric.Intersection {
			want = 10 - 101.0/192
		}
		if e != nil || math.Abs(p.Volume-want) > 1e-9 {
			t.Fatal(p, want, e)
		}
	}
}

func TestContactGridSplineGraphBoolean(t *testing.T) {
	ctx := context.Background()
	graph := func(du int, z func(int, int) float64) *geom.BSplineSurface {
		t.Helper()
		ku := make([]float64, 2*du+2)
		for i := du + 1; i < len(ku); i++ {
			ku[i] = 1
		}
		var p []geom.Vec3
		for i := range du + 1 {
			for j := range 2 {
				p = append(p, geom.Vec3{X: float64(i) / float64(du), Y: float64(j), Z: z(i, j)})
			}
		}
		s, e := geom.NewBSplineSurface(du, 1, du+1, 2, p, ku, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base := graph(1, func(_, _ int) float64 { return 0 })
	b := graph(1, func(_, _ int) float64 { return 10 })
	// 16(u-1/4)(u-3/4)(v-1/2) gives six alternating sectors,
	// with the signed integral zero and absolute integral 1/4.
	a := graph(2, func(i, j int) float64 { return 10 + [3]float64{3, -5, 3}[i]*(float64(j)-.5) })
	o := intersect.SplinePlaneOptions{Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{MaxCells: 1}}}
	for _, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection, parametric.Difference} {
		s, h, e := CombineSplineGraphSolids(ctx, base, a, b, op, 1e-9, o)
		if h.ContactGrid == nil || !h.ContactGrid.Proven || len(h.ContactGrid.Cells) != 6 ||
			len(h.ContactGrid.Lines[0]) != 2 || len(h.ContactGrid.Lines[1]) != 1 {
			t.Fatal(h, e)
		}
		if op == parametric.Difference {
			if !errors.Is(e, geom.ErrDegenerate) || s.Topology().Faces != 0 {
				t.Fatal("pinched grid subtraction", s.Topology(), e)
			}
			continue
		}
		if e != nil || len(h.ContourEdges) != 7 || len(h.FacesA) != 3 || len(h.FacesB) != 3 ||
			s.Topology().EulerCharacteristic != 2 {
			t.Fatal(s.Topology(), h, e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		want := 10.125
		if op == parametric.Intersection {
			want = 9.875
		}
		if e != nil || math.Abs(p.Volume-want) > 1e-9 {
			t.Fatal(p, want, e)
		}
	}
}
