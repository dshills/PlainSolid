package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func exactSewTestFace(t *testing.T, axis int, fixed *big.Rat, shift *big.Rat, reverse bool) SewFace {
	t.Helper()
	points := [][4]*big.Rat{}
	for _, u := range []int64{0, 1} {
		for _, v := range []int64{0, 1} {
			p := [4]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}
			p[axis] = new(big.Rat).Set(fixed)
			other := []int{}
			for i := 0; i < 3; i++ {
				if i != axis {
					other = append(other, i)
				}
			}
			p[other[0]] = big.NewRat(u, 1)
			p[other[1]] = big.NewRat(v, 1)
			p[0].Add(p[0], shift)
			points = append(points, p)
		}
	}
	patch, e := geom.NewExactHomogeneousBezierSurface(context.Background(), 1, 1, points, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	s, e := geom.NewExactBezierSurface(patch)
	if e != nil {
		t.Fatal(e)
	}
	corners := [4][2]int64{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
	curves := []ExactQuadraticUV{}
	for i, a := range corners {
		b := corners[(i+1)%4]
		var c [3][3]*big.Rat
		for j := 0; j < 3; j++ {
			c[j] = [3]*big.Rat{big.NewRat((2-int64(j))*a[0]+int64(j)*b[0], 2), big.NewRat((2-int64(j))*a[1]+int64(j)*b[1], 2), big.NewRat(1, 1)}
		}
		q, e := NewExactQuadraticUV(c)
		if e != nil {
			t.Fatal(e)
		}
		curves = append(curves, q)
	}
	part, e := ExactQuadraticSewFace(context.Background(), s, curves, reverse, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return part
}
func TestExactQuadraticSewAPI(t *testing.T) {
	parts := []SewFace{}
	for axis := 0; axis < 3; axis++ {
		for side := 0; side < 2; side++ {
			reverse := side == 0
			if axis == 1 {
				reverse = !reverse
			}
			parts = append(parts, exactSewTestFace(t, axis, big.NewRat(int64(side), 1), big.NewRat(1, 3), reverse))
		}
	}
	s, _, e := SewRationalQuadratic(context.Background(), parts, SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.ValidateEdgeAgreement(context.Background(), edgecheck.Options{})
	if e != nil || !p.Complete {
		t.Fatal(e, p)
	}
	v, e := s.RationalVertices(context.Background())
	if e != nil || len(v) != 8 {
		t.Fatal(e, v)
	}
	var b bytes.Buffer
	if e = WriteJSON(&b, s); e != nil {
		t.Fatal(e)
	}
	s, e = ReadJSON(&b, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	vv, e := s.RationalVertices(context.Background())
	if e != nil || len(vv) != 8 {
		t.Fatal(e)
	}
	for i := range v {
		if !sameRationalVertex(v[i].Point, vv[i].Point) {
			t.Fatal("rounded reload")
		}
	}
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 60))
	a := exactSewTestFace(t, 2, new(big.Rat), big.NewRat(1, 1), false)
	shift := new(big.Rat).Add(big.NewRat(1, 1), tiny)
	c := exactSewTestFace(t, 2, new(big.Rat), shift, true)
	if a.Vertices[0] != c.Vertices[0] {
		t.Fatal("fixture not colliding")
	}
	if _, _, e = SewRationalQuadratic(context.Background(), []SewFace{a, c}, SewOptions{Tolerance: 1e-6}); !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("distinct exact vertices welded", e)
	}
	if _, _, e = SewRationalQuadratic(context.Background(), parts, SewOptions{Tolerance: 1e-9, MaxFaces: 5}); e == nil {
		t.Fatal("limits")
	}
}
