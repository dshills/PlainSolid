package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestSkewOrthogonalCylinders(t *testing.T) {
	for _, c := range []struct {
		rx, rz, d         float64
		kind              SkewOrthogonalCylinderTopology
		components, nodes int
	}{
		{1, 1, 3, SkewCylinderEmpty, 0, 0}, {1, 1, 2, SkewCylinderIsolatedPoint, 1, 1}, {1, 1, 1, SkewCylinderOneLoop, 1, 4}, {1, 3, .5, SkewCylinderTwoLoops, 2, 4}, {1, 2, 1, SkewCylinderOneNode, 1, 3}, {1, 1, 0, SkewCylinderTwoNodes, 1, 2}, {1, 1, -1, SkewCylinderOneLoop, 1, 4},
	} {
		q, e := IntersectSkewOrthogonalCylinders(context.Background(), Vec3{}, c.rx, Vec3{Y: c.d}, c.rz, SurfacePatchEvaluationOptions{})
		if e != nil || !q.Complete || q.Topology != c.kind || len(q.Components) != c.components || len(q.Nodes) != c.nodes {
			t.Fatal(c, q, e)
		}
		if len(q.Arcs) > 0 {
			for arc := 0; arc < 4; arc++ {
				for _, u := range []float64{.2, .5, .8} {
					p, v, e := q.EvaluateArc(arc, u)
					if e != nil || math.Abs(p.Y*p.Y+p.Z*p.Z-c.rx*c.rx) > 1e-12 || math.Abs(p.X*p.X+(p.Y-c.d)*(p.Y-c.d)-c.rz*c.rz) > 1e-12 || math.Abs(p.Y*v.Y+p.Z*v.Z) > 1e-12 || math.Abs(p.X*v.X+(p.Y-c.d)*v.Y) > 1e-12 {
						t.Fatal(c, p, v, e)
					}
				}
			}
		}
	}
	q, e := IntersectSkewOrthogonalCylinders(context.Background(), Vec3{}, 1, Vec3{Y: math.Nextafter(2, 3)}, 1, SurfacePatchEvaluationOptions{})
	if e != nil || q.Topology != SkewCylinderEmpty {
		t.Fatal(q, e)
	}
	q, e = IntersectSkewOrthogonalCylinders(context.Background(), Vec3{}, 1, Vec3{Y: math.Nextafter(2, 1)}, 1, SurfacePatchEvaluationOptions{})
	if e != nil || q.Topology != SkewCylinderOneLoop || q.RelativeY[0].Cmp(big.NewRat(1, 1)) >= 0 {
		t.Fatal(q, e)
	}
	q, e = IntersectSkewOrthogonalCylinders(context.Background(), Vec3{}, 1, Vec3{Y: 1}, 1, SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || q.Complete || len(q.Nodes) != 0 {
		t.Fatal(q, e)
	}
}
