package trimmed

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func projectiveOrderedRoofFixture(t *testing.T, z, w, extent float64) Face {
	t.Helper()
	knots := []float64{0, 0, 1, 1}
	var poles []geom.Vec3
	var weights []float64
	for ix := range 2 {
		for iy := range 2 {
			x, y := float64(ix)*extent, float64(iy)
			poles = append(poles, geom.Vec3{X: x, Y: y, Z: z + x*y})
			weights = append(weights, 1+(w-1)*y)
		}
	}
	s, err := geom.NewBSplineSurface(1, 1, 2, 2, poles, knots, knots, weights)
	if err != nil {
		t.Fatal(err)
	}
	p := [4]geom.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}
	f := Face{Surface: s}
	for i := range p {
		f.Outer = append(f.Outer, Use{Edge: i, UVStart: p[i], UVEnd: p[(i+1)%4]})
	}
	return f
}

func TestBuildOrderedProjectiveSplineRoofSolidNestedMaterial(t *testing.T) {
	ctx := context.Background()
	upper, lower := projectiveOrderedRoofFixture(t, 1.5, 2, 1), projectiveOrderedRoofFixture(t, 1, 1, 1)
	for _, axes := range [][2]int{{0, 1}, {1, 0}} {
		for _, tc := range []struct {
			a, b     Face
			op       parametric.BooleanOp
			roofs    [2]int
			base     int
			volume   float64
			centroid float64
		}{
			{upper, lower, parametric.Intersection, [2]int{-1, 0}, 5, 1.25, 29.0 / 45},
			{lower, upper, parametric.Union, [2]int{-1, 0}, 5, 1.75, 8.0 / 9},
			{upper, lower, parametric.Difference, [2]int{0, 1}, -1, .5, 1.5},
		} {
			q, err := BuildOrderedProjectiveSplineRoofSolid(ctx, tc.a, tc.b, axes, 0, tc.op, ProjectiveRoofSolidOptions{})
			if err != nil || !q.Complete || q.Empty || q.BaseFace != tc.base || q.Section != -1 || q.Roofs != tc.roofs {
				t.Fatal("exact ordered result and original face ownership", axes, tc.op, q.Roofs, q.BaseFace, err)
			}
			if len(q.Solid.faces) != 6 || len(q.Solid.edges) != 12 || len(q.Solid.vertices) != 8 || len(q.Walls) != 4 {
				t.Fatal("closed rectangle topology", axes, tc.op)
			}
			props, err := q.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if err != nil || math.Abs(props.Volume-tc.volume) > 1e-10 || math.Abs(props.Centroid.Z-tc.centroid) > 1e-10 {
				t.Fatal("original ordered material must match direct height integrals", axes, tc.op, props, err)
			}
		}
		for _, pair := range [][2]Face{{lower, upper}, {lower, projectiveOrderedRoofFixture(t, 1, 2, 1)}} {
			q, err := BuildOrderedProjectiveSplineRoofSolid(ctx, pair[0], pair[1], axes, 0, parametric.Difference, ProjectiveRoofSolidOptions{})
			if err != nil || !q.Complete || !q.Empty || len(q.Solid.faces) != 0 || q.Section != -1 {
				t.Fatal("strict containment and coincidence give no regularized material", axes, q, err)
			}
		}
	}
	partial := projectiveOrderedRoofFixture(t, 1, 1, .5)
	q, err := BuildOrderedProjectiveSplineRoofSolid(ctx, upper, partial, [2]int{0, 1}, 0, parametric.Union, ProjectiveRoofSolidOptions{})
	if !errors.Is(err, geom.ErrConvergence) || q.Complete {
		t.Fatal("height order over a strict overlap cannot drop an original exterior region", q, err)
	}
}
