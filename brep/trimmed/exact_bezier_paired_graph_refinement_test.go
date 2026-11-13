package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestRefinedBezierCurvedScaledSeam(t *testing.T) {
	_, a, _, _ := bezierSeamFixture(t, 3)
	h := a.HomogeneousControls()
	for i := range h {
		h[i][0].Mul(h[i][0], big.NewRat(8, 1))
	}
	b, e := NewExactBezierUV(h)
	if e != nil {
		t.Fatal(e)
	}
	patch := func(scale float64) *geom.SurfacePatch {
		s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: .25}}, []float64{0, 0, scale, scale}, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	pa, pb := patch(1), patch(8)
	o := CurvedBezierGraphChainOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .02}}
	left, e := TessellateCurvedBezierGraphChainCertified(context.Background(), pa, []ExactBezierUV{a}, new(big.Rat), o)
	if e != nil {
		t.Fatal(e)
	}
	right, e := TessellateCurvedBezierGraphChainCertified(context.Background(), pb, []ExactBezierUV{b}, new(big.Rat), o)
	if e != nil {
		t.Fatal(e)
	}
	if graphChainPartitionsEqual(left, right) {
		t.Fatal("expected initial mismatch")
	}
	r, e := TessellatePairedBezierGraphChainsRefinedCertified(context.Background(), pa, []ExactBezierUV{a}, new(big.Rat), pb, []ExactBezierUV{b}, new(big.Rat), o, 2)
	if e != nil {
		t.Fatal(e)
	}
	if !graphChainPartitionsEqual(r.FaceA, r.FaceB) || r.SquaredDistance.Cmp(big.NewRat(1, 2500)) > 0 {
		t.Fatal("certificate")
	}
	for _, v := range r.SharedVertices {
		p := r.FaceA.Mesh.Vertices[v[0]]
		if p != r.FaceB.Mesh.Vertices[v[1]] {
			t.Fatal("shared display")
		}
		exact := r.FaceA.ExactPoints[v[0]]
		want := new(big.Rat).Quo(new(big.Rat).Mul(exact[0], exact[1]), big.NewRat(4, 1))
		if exact[2].Cmp(want) != 0 {
			t.Fatal("independent saddle model")
		}
	}
	bad := b.HomogeneousControls()
	bad[1][1].Add(bad[1][1], big.NewRat(1, 64))
	unequal, err := NewExactBezierUV(bad)
	if err != nil {
		t.Fatal(err)
	}
	if r, e := TessellatePairedBezierGraphChainsRefinedCertified(context.Background(), pa, []ExactBezierUV{a}, new(big.Rat), pb, []ExactBezierUV{unequal}, new(big.Rat), o, 2); e == nil || len(r.SharedVertices) != 0 {
		t.Fatal("same endpoints unequal interior")
	}
	tiny := o
	tiny.Boundary.Display.MaxSegments = 1
	if r, e := TessellatePairedBezierGraphChainsRefinedCertified(context.Background(), pa, []ExactBezierUV{a}, new(big.Rat), pb, []ExactBezierUV{b}, new(big.Rat), tiny, 2); e == nil || len(r.SharedVertices) != 0 {
		t.Fatal("budget partial success")
	}
	for _, rounds := range []int{0, 9} {
		if r, e := TessellatePairedBezierGraphChainsRefinedCertified(context.Background(), pa, []ExactBezierUV{a}, new(big.Rat), pb, []ExactBezierUV{b}, new(big.Rat), o, rounds); e == nil || len(r.SharedVertices) != 0 {
			t.Fatal("round limit")
		}
	}
}
