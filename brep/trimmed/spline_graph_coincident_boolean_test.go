package trimmed

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

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
