package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestSphereCylinderAngularTopology(t *testing.T) {
	for _, c := range []struct {
		d, R, r float64
		kind    SphereCylinderTopology
		roots   int
	}{
		{0, 1, 2, SphereCylinderEmpty, 0}, {0, 2, 2, SphereCylinderEquatorialCircle, 0}, {0, 3, 2, SphereCylinderTwoClosedCurves, 0},
		{2, .5, 1, SphereCylinderEmpty, 0}, {2, 1, 1, SphereCylinderIsolatedPoint, 1}, {2, 2, 1, SphereCylinderOneClosedCurve, 2}, {2, 3, 1, SphereCylinderTwoLoopsAtNode, 1}, {2, 4, 1, SphereCylinderTwoClosedCurves, 0},
	} {
		q, e := ClassifySphereCylinderAngularDomain(context.Background(), Vec3{X: c.d}, c.R, Vec3{}, Vec3{Z: 3}, c.r, SurfacePatchEvaluationOptions{})
		if e != nil || !q.Complete || q.Topology != c.kind || len(q.BoundaryRays) != c.roots {
			t.Fatal(c, q, e)
		}
		for _, ray := range q.BoundaryRays {
			// Prove ray norm and projection identity symbolically, without evaluating
			// its square root: Base·Vector=0 and |Base|²+rad*|Vector|²=1.
			bb, vv, bv, dot := new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)
			for i := 0; i < 3; i++ {
				bb.Add(bb, new(big.Rat).Mul(ray.Base[i], ray.Base[i]))
				vv.Add(vv, new(big.Rat).Mul(ray.Vector[i], ray.Vector[i]))
				bv.Add(bv, new(big.Rat).Mul(ray.Base[i], ray.Vector[i]))
				dot.Add(dot, new(big.Rat).Mul(q.Projection[i], ray.Base[i]))
			}
			if bv.Sign() != 0 || new(big.Rat).Add(bb, new(big.Rat).Mul(ray.Radicand, vv)).Cmp(big.NewRat(1, 1)) != 0 || dot.Cmp(q.Threshold) != 0 {
				t.Fatal(ray)
			}
		}
	}
	// At minimum zero (R=d+r), D=2rd(1+cos theta). Around theta=pi,
	// z=±sqrt(rd)*delta+O(delta³): distinct crossing tangent directions,
	// although the two surface normals are parallel at the singular point.
	node, e := ClassifySphereCylinderAngularDomain(context.Background(), Vec3{X: 2}, 3, Vec3{}, Vec3{Z: -1}, 1, SurfacePatchEvaluationOptions{})
	if e != nil || node.BoundaryRays[0].Base[0].Cmp(big.NewRat(-1, 1)) != 0 {
		t.Fatal(node, e)
	}
	local, e := IntersectSphereCylinderRationalRay(context.Background(), Vec3{X: 2}, 3, Vec3{}, Vec3{Z: 1}, 1, [3]*big.Rat{big.NewRat(-1, 1), new(big.Rat), new(big.Rat)}, nil, SurfacePatchEvaluationOptions{})
	if e != nil || len(local.Contacts) != 1 || !local.Contacts[0].SurfaceTangent {
		t.Fatal(local, e)
	}
	// Irrational rays x=1/4, y=±sqrt(15)/4 for d=2,R=2,r=1.
	q, e := ClassifySphereCylinderAngularDomain(context.Background(), Vec3{X: 2}, 2, Vec3{}, Vec3{Z: 1}, 1, SurfacePatchEvaluationOptions{})
	if e != nil || q.BoundaryRays[0].Base[0].Cmp(big.NewRat(1, 4)) != 0 || q.BoundaryRays[0].Radicand.Cmp(big.NewRat(15, 64)) != 0 {
		t.Fatal(q, e)
	}
	q, e = ClassifySphereCylinderAngularDomain(context.Background(), Vec3{X: 2}, 2, Vec3{}, Vec3{Z: 1}, 1, SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || q.Complete || len(q.BoundaryRays) != 0 {
		t.Fatal(q, e)
	}
}
