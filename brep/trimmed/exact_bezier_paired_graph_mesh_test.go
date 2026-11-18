//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestPairedBezierCurvedGraphSeam(t *testing.T) {
	slowtest.Skip(t)
	patch := func(other bool) *geom.SurfacePatch {
		var poles []geom.Vec3
		for i, g := range []float64{.5, .5, .625} {
			for j := 0; j < 2; j++ {
				v := float64(j)
				y := v
				if other {
					y = 2*g - v
				}
				poles = append(poles, geom.Vec3{X: float64(i) / 2, Y: y, Z: (v - g) / 4})
			}
		}
		s, e := geom.NewBSplineSurface(2, 1, 3, 2, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	var trims []ExactBezierUV
	for side, n := range []int{3, 4} {
		h := make([][3]*big.Rat, n+1)
		for i := range h {
			y := big.NewRat(int64(16+side*side), 32)
			y.Add(y, big.NewRat(int64(side*i), int64(16*n)))
			y.Add(y, big.NewRat(int64(i*(i-1)), int64(32*n*(n-1))))
			h[i] = [3]*big.Rat{big.NewRat(int64(i+side*n), int64(2*n)), y, big.NewRat(1, 1)}
		}
		q, e := NewExactBezierUV(h)
		if e != nil {
			t.Fatal(e)
		}
		trims = append(trims, q)
	}
	a, b := patch(false), patch(true)
	o := CurvedBezierGraphChainOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .2, MaxSubdivisions: 64}}
	r, e := TessellatePairedBezierGraphChainsCertified(context.Background(), a, trims, new(big.Rat), b, trims, new(big.Rat), o)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.SharedVertices) < 3 || r.SquaredDistance.Cmp(big.NewRat(1, 25)) > 0 {
		t.Fatal("missing seam/bound")
	}
	countEdge := func(tris [][3]int, a, b int) int {
		n := 0
		for _, tri := range tris {
			for j, x := range tri {
				if x == a && tri[(j+1)%3] == b {
					n++
				}
			}
		}
		return n
	}
	for i, pair := range r.SharedVertices {
		pa, pb := r.FaceA.Mesh.Vertices[pair[0]], r.FaceB.Mesh.Vertices[pair[1]]
		if pa != pb {
			t.Fatal("seam not bit equal")
		}
		if r.FaceA.ExactPoints[pair[0]][2].Sign() != 0 {
			t.Fatal("independent crease is z=0")
		}
		if i > 0 {
			previous := r.SharedVertices[i-1]
			if countEdge(r.FaceA.Mesh.Triangles, pair[0], previous[0]) != 1 || countEdge(r.FaceB.Mesh.Triangles, previous[1], pair[1]) != 1 {
				t.Fatal("opposite native incidence lost")
			}
		}
	}
	tight := o
	tight.Interior.Deflection = .04
	if denied, e := TessellatePairedBezierGraphChainsRefinedCertified(context.Background(), a, trims, new(big.Rat), b, trims, new(big.Rat), tight, 2); e == nil || len(denied.SharedVertices) != 0 {
		t.Fatal("unsplit curved baseline unexpectedly certified")
	}
	tight.Boundary.ClosingSubdivisions = 8
	refined, err := TessellatePairedBezierGraphChainsRefinedCertified(context.Background(), a, trims, new(big.Rat), b, trims, new(big.Rat), tight, 2)
	if err != nil {
		t.Fatal("subdivided curved baseline", err)
	}
	if refined.SquaredDistance.Cmp(big.NewRat(1, 625)) > 0 {
		t.Fatal("tight bound")
	}
	for _, face := range []CertifiedCurvedBezierGraphChain{refined.FaceA, refined.FaceB} {
		for _, chain := range face.GraphChain.ClosingVertices {
			if len(chain) != 9 {
				t.Fatal("closing ancestry")
			}
			start, end := face.GraphChain.ExactUV[chain[0]], face.GraphChain.ExactUV[chain[8]]
			for j, id := range chain {
				for axis := 0; axis < 2; axis++ {
					want := new(big.Rat).Add(start[axis], new(big.Rat).Mul(new(big.Rat).Sub(end[axis], start[axis]), big.NewRat(int64(j), 8)))
					if face.GraphChain.ExactUV[id][axis].Cmp(want) != 0 {
						t.Fatal("off-source closing sample")
					}
				}
			}
		}
	}
	for _, id := range refined.FaceB.GraphChain.ClosingVertices[0] {
		x := refined.FaceB.ExactPoints[id][0]
		wantY := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Quo(new(big.Rat).Mul(x, x), big.NewRat(4, 1)))
		wantZ := new(big.Rat).Neg(new(big.Rat).Quo(wantY, big.NewRat(8, 1)))
		if refined.FaceB.ExactPoints[id][1].Cmp(wantY) != 0 || refined.FaceB.ExactPoints[id][2].Cmp(wantZ) != 0 {
			t.Fatal("independent curved baseline model")
		}
	}
	limited := tight
	limited.Interior.MaxTriangles = 8
	if denied, e := TessellatePairedBezierGraphChainsRefinedCertified(context.Background(), a, trims, new(big.Rat), b, trims, new(big.Rat), limited, 2); e == nil || len(denied.SharedVertices) != 0 {
		t.Fatal("amount bound")
	}
	tight.Boundary.ClosingSubdivisions = 3
	if _, e := TessellateCurvedBezierGraphChainCertified(context.Background(), a, trims, new(big.Rat), tight); e == nil {
		t.Fatal("non-dyadic amount")
	}
	bad := append([]ExactBezierUV(nil), trims...)
	h := bad[0].HomogeneousControls()
	h[1][1].Add(h[1][1], big.NewRat(1, 128))
	bad[0], _ = NewExactBezierUV(h)
	if denied, e := TessellatePairedBezierGraphChainsCertified(context.Background(), a, trims, new(big.Rat), b, bad, new(big.Rat), o); e == nil || len(denied.SharedVertices) != 0 {
		t.Fatal("unequal interior accepted")
	}
	o.Interior.MaxTriangleChecks = 1
	if denied, e := TessellatePairedBezierGraphChainsCertified(context.Background(), a, trims, new(big.Rat), b, trims, new(big.Rat), o); e == nil || len(denied.SharedVertices) != 0 {
		t.Fatal("partial failure")
	}
}
