//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestCurvedBezierGraphChainSaddle(t *testing.T) {
	s, _ := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: .25}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	p, _ := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	var pieces []ExactBezierUV
	for side, n := range []int{3, 4} {
		h := make([][3]*big.Rat, n+1)
		for i := range h {
			y := big.NewRat(int64(4+side), 8)
			y.Add(y, big.NewRat(int64(i*(i-1)), int64(8*n*(n-1))))
			h[i] = [3]*big.Rat{big.NewRat(int64(i+side*n), int64(2*n)), y, big.NewRat(1, 1)}
		}
		q, e := NewExactBezierUV(h)
		if e != nil {
			t.Fatal(e)
		}
		pieces = append(pieces, q)
	}
	opts := CurvedBezierGraphChainOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .02, MaxSubdivisions: 64}}
	r, e := TessellateCurvedBezierGraphChainCertified(context.Background(), p, pieces, new(big.Rat), opts)
	if e != nil {
		t.Fatal(e)
	}
	target := big.NewRat(1, 2500)
	if r.FillSquaredDistance.Cmp(target) > 0 || r.SurfaceSquaredDistance.Sign() <= 0 || r.InteriorRefinementPasses == 0 {
		t.Fatal("missing curved residual/refinement", r.FillSquaredDistance, r.SurfaceSquaredDistance, r.InteriorRefinementPasses)
	}
	if len(r.BoundaryLoops) != 1 || len(r.BoundaryLoops[0]) != len(r.GraphChain.Boundary) {
		t.Fatal("boundary split")
	}
	for i, k := range r.GraphChain.Boundary {
		if r.BoundaryLoops[0][i] != k {
			t.Fatal("boundary renumbered")
		}
		u, v := r.ExactUV[k][0], r.ExactUV[k][1]
		z := new(big.Rat).Quo(new(big.Rat).Mul(u, v), big.NewRat(4, 1))
		if r.ExactPoints[k][2].Cmp(z) != 0 {
			t.Fatal("source saddle changed")
		}
	}
	// Independent exact saddle evaluation at each UV triangle centroid; compare
	// the displayed triangle centroid with the certified interpolation residual.
	for i, tri := range r.Mesh.Triangles {
		u, v := new(big.Rat), new(big.Rat)
		shown := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
		for _, k := range tri {
			u.Add(u, r.ExactUV[k][0])
			v.Add(v, r.ExactUV[k][1])
			p := r.Mesh.Vertices[k]
			for axis, x := range []float64{p.X, p.Y, p.Z} {
				shown[axis].Add(shown[axis], new(big.Rat).SetFloat64(x))
			}
		}
		u.Quo(u, big.NewRat(3, 1))
		v.Quo(v, big.NewRat(3, 1))
		actual := [3]*big.Rat{u, v, new(big.Rat).Quo(new(big.Rat).Mul(u, v), big.NewRat(4, 1))}
		sq := new(big.Rat)
		for axis := range shown {
			shown[axis].Quo(shown[axis], big.NewRat(3, 1))
			d := new(big.Rat).Sub(actual[axis], shown[axis])
			sq.Add(sq, new(big.Rat).Mul(d, d))
		}
		if sq.Cmp(r.SquaredErrors[i]) > 0 {
			t.Fatal("lost independent exact sagitta", i, sq, r.SquaredErrors[i])
		}
	}
	opts.Interior.MaxTriangleChecks = 1
	if denied, e := TessellateCurvedBezierGraphChainCertified(context.Background(), p, pieces, new(big.Rat), opts); e == nil || len(denied.Mesh.Vertices) != 0 {
		t.Fatal("partial output on exhausted proof")
	}
}
