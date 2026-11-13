package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestPartiallyCoincidentSplineGraphEnvelope(t *testing.T) {
	ctx := context.Background()
	makeGraph := func(us, vs []float64, du, dv int, z func(float64, float64) float64) *geom.BSplineSurface {
		t.Helper()
		knots := func(p []float64, degree int) []float64 {
			k := make([]float64, degree+len(p)+1)
			for i := 0; i < len(k); i++ {
				if i > degree {
					k[i] = 1
				}
			}
			if degree == 1 && len(p) == 3 {
				k[2] = p[1]
			}
			return k
		}
		var p []geom.Vec3
		for _, u := range us {
			for _, v := range vs {
				p = append(p, geom.Vec3{X: u, Y: v, Z: z(u, v)})
			}
		}
		s, e := geom.NewBSplineSurface(du, dv, len(us), len(vs), p, knots(us, du), knots(vs, dv), nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base := makeGraph([]float64{0, 1}, []float64{0, 1}, 1, 1, func(_, _ float64) float64 { return 0 })
	a := makeGraph([]float64{0, .5, 1}, []float64{0, 1}, 1, 1, func(u, v float64) float64 { return 2 + u*v + max(u-.5, 0) })
	b := makeGraph([]float64{0, .5, 1}, []float64{0, .5, 1}, 2, 2, func(u, v float64) float64 { return 2 + u*v })
	o := intersect.SplinePlaneOptions{Trace: geom.SurfaceTraceOptions{Arcs: geom.SurfacePlaneArcOptions{MaxCells: 1}}}
	for i, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection} {
		s, h, e := CombinePartiallyCoincidentSplineGraphs(ctx, base, a, b, op, 1e-9, o, geom.GraphPointContactOptions{})
		if e != nil || h.ContactPartition == nil || len(h.CoincidentFaces) != 1 || len(h.FacesA) == 0 || len(h.FacesB) == 0 ||
			len(h.ContourEdges) != 1 || s.Topology().EulerCharacteristic != 2 {
			t.Fatal(s.Topology(), h, e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		want := [2]float64{19.0 / 8, 9.0 / 4}[i]
		if e != nil || math.Abs(p.Volume-want) > 1e-9 {
			t.Fatal(p, want, e)
		}
	}
	mat, e := NewSharedBaseSplineGraphMaterial(ctx, base, a, b, o)
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range []struct {
		u, z *big.Rat
		want parametric.Location
	}{
		{big.NewRat(1, 2), big.NewRat(9, 4), parametric.Boundary},
		{big.NewRat(1, 4), big.NewRat(17, 8), parametric.Outside},
	} {
		loc, e := mat.LocateDifferenceRationalWithKnotPartition(ctx, [3]*big.Rat{c.u, big.NewRat(1, 2), c.z},
			geom.GraphPointContactOptions{}, geom.SurfacePatchEvaluationOptions{})
		if e != nil || loc != c.want {
			t.Fatal(c, loc, e)
		}
	}

	if _, h, e := CombineSplineGraphSolids(ctx, base, b, a, parametric.Union, 1e-9, o); e != nil || h.ContactPartition == nil {
		t.Fatal("automatic partial coincidence", h, e)
	}
	s, h, e := CombineSplineGraphSolids(ctx, base, a, b, parametric.Difference, 1e-9, o)
	if e != nil || h.SignedDifference == nil || len(h.ContourEdges) != 1 || len(h.WallFaces) != 3 {
		t.Fatal("automatic partial coincidence difference", s.Topology(), h, e)
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-.125) > 1e-9 {
		t.Fatal(p, e)
	}
	s, h, e = CombinePartiallyCoincidentSplineGraphs(ctx, base, b, a, parametric.Difference, 1e-9, o, geom.GraphPointContactOptions{})
	if e != nil || h.SignedDifference == nil || s.Topology().Faces != 0 {
		t.Fatal("empty reversed difference", s.Topology(), h, e)
	}

}

func TestPartiallyCoincidentSplineGraphDifferenceComponents(t *testing.T) {
	ctx := context.Background()
	points := []float64{0, .25, .5, .75, 1}
	knots := []float64{0, 0, .25, .5, .75, 1, 1}
	build := func(z func(int, int) float64) *geom.BSplineSurface {
		t.Helper()
		var p []geom.Vec3
		for i, u := range points {
			for j, v := range points {
				p = append(p, geom.Vec3{X: u, Y: v, Z: z(i, j)})
			}
		}
		s, e := geom.NewBSplineSurface(1, 1, 5, 5, p, knots, knots, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base := build(func(_, _ int) float64 { return 0 })
	b := build(func(_, _ int) float64 { return 2 })
	for _, c := range []struct {
		name                  string
		z                     func(int, int) float64
		volume                float64
		contour, walls, euler int
	}{
		{"enclosed island", func(i, j int) float64 {
			if i == 2 && j == 2 {
				return 3
			}
			return 2
		}, 1.0 / 16, 8, 0, 2},
		{"coincident hole", func(i, j int) float64 {
			if i == 0 || j == 0 || i == 4 || j == 4 {
				return 3
			}
			return 2
		}, 7.0 / 16, 8, 16, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			a := build(c.z)
			s, h, e := CombinePartiallyCoincidentSplineGraphs(ctx, base, a, b, parametric.Difference, 1e-9, intersect.SplinePlaneOptions{}, geom.GraphPointContactOptions{})
			if e != nil || h.SignedDifference == nil || len(h.ContourEdges) != c.contour || len(h.WallFaces) != c.walls ||
				s.Topology().EulerCharacteristic != c.euler {
				t.Fatal(s.Topology(), h, e)
			}
			p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-6})
			if e != nil || math.Abs(p.Volume-c.volume) > 1e-8 {
				t.Fatal(p, c.volume, e)
			}
			if c.euler == 0 {
				_, h, e = CombinePartiallyCoincidentSplineGraphs(ctx, base, a, b, parametric.Union, 1e-9, intersect.SplinePlaneOptions{}, geom.GraphPointContactOptions{})
				if e != nil || len(h.CoincidentRegions.Components) != 1 || len(h.PartitionEdges) == 0 ||
					len(h.ContourEdges)+len(h.PartitionEdges) != 20 || len(h.CoincidentFaces) != 1 ||
					h.PartitionFaceByCell[5] != h.PartitionFaceByCell[10] {
					t.Fatal("coincident grid history", h, e)
				}
			}
		})
	}
	// The selected cells meet along a line where the solid thickness is
	// zero. Gluing those material interiors would create a pinched shell.
	a := build(func(i, j int) float64 {
		if (i == 1 || i == 3) && j == 2 {
			return 3
		}
		return 2
	})
	s, _, e := CombinePartiallyCoincidentSplineGraphs(ctx, base, a, b, parametric.Difference, 1e-9, intersect.SplinePlaneOptions{}, geom.GraphPointContactOptions{})
	if !errors.Is(e, geom.ErrConvergence) || s.Topology().Faces != 0 {
		t.Fatal("pinched material was accepted", s.Topology(), e)
	}
}
