package trimmed

import (
	"context"
	"math"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
)

func TestCombineAffineSplineShellsSimultaneousEdges(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	b, _ := makeOpenSplineBox(t, [3]float64{.5, -.5, 0}, [3]float64{1.5, .5, 1}, 0)
	for _, tc := range []struct {
		op     parametric.BooleanOp
		volume float64
	}{{parametric.Union, 1.75}, {parametric.Intersection, .25}, {parametric.Difference, .75}} {
		s, h, e := CombineSplineShells(ctx, a, b, tc.op, SplineShellCombineOptions{})
		if e != nil || h.Method != "affine-planar-arrangement" || len(s.Topology().Shells) != 1 || s.Topology().EulerCharacteristic != 2 {
			t.Fatal(tc, h.Method, s.Topology(), e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(p.Volume-tc.volume) > 1e-11 {
			t.Fatal(tc, p, e)
		}
		if len(h.GeneratedEdges) == 0 {
			t.Fatal("missing generated or subdivision edges")
		}
		if tc.op == parametric.Intersection {
			p, e := PlanarBoundaryFromAffineSplines(ctx, s, AffineSplineShellBooleanOptions{})
			if e != nil || p.Topology().EulerCharacteristic != 2 {
				t.Fatal("cannot certify the polygonally trimmed result", p.Topology(), e)
			}
			// Exercise chaining with a result whose faces are now triangles
			// on the original affine supports rather than rectangles.
			again, next, e := CombineAffineSplineShells(ctx, s, a, parametric.Intersection, AffineSplineShellBooleanOptions{})
			if e != nil || next.Method != "affine-planar-arrangement" {
				t.Fatal("cannot reuse the exact result", next.Method, e)
			}
			props, e := again.MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil || math.Abs(props.Volume-tc.volume) > 1e-11 {
				t.Fatal("chained intersection", props, e)
			}
		}
		for _, operand := range []struct {
			s    Solid
			rows [][]int
		}{{a, h.FacesA}, {b, h.FacesB}} {
			for i, row := range operand.rows {
				for _, j := range row {
					if s.faces[j].Surface != operand.s.faces[i].Surface {
						t.Fatal("lost exact spline support", i, j)
					}
				}
			}
		}
	}
}
