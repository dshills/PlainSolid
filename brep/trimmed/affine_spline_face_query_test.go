package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"testing"
)

func affineQueryFaceFixture(t *testing.T) Face {
	t.Helper()
	s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 3, Z: 1}, {X: 2, Z: 1}, {X: 2, Y: 3, Z: 2}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	uses := func(loop []ExactQuadraticUV) []Use {
		out := make([]Use, len(loop))
		for i, q := range loop {
			_, ends := q.Endpoints()
			out[i] = Use{Edge: i, Curve: q, UVStart: ends[0], UVEnd: ends[1]}
		}
		return out
	}
	f := Face{Surface: s, Outer: uses(nativeNonbinaryRoundedLoop(t, big.NewRat(3, 8), false)), Holes: [][]Use{uses(nativeNonbinaryRoundedLoop(t, big.NewRat(1, 8), true))}}
	// Exact degree elevation exercises native cubic controls with nonbinary
	// coordinates, without converting the original quadratic geometry to floats.
	q := f.Outer[0].Curve.(ExactQuadraticUV)
	h := q.HomogeneousControls()
	elev := make([][3]*big.Rat, 4)
	for j := 0; j < 3; j++ {
		elev[0][j] = new(big.Rat).Set(h[0][j])
		elev[3][j] = new(big.Rat).Set(h[2][j])
		elev[1][j] = new(big.Rat).Quo(new(big.Rat).Add(h[0][j], new(big.Rat).Mul(big.NewRat(2, 1), h[1][j])), big.NewRat(3, 1))
		elev[2][j] = new(big.Rat).Quo(new(big.Rat).Add(h[2][j], new(big.Rat).Mul(big.NewRat(2, 1), h[1][j])), big.NewRat(3, 1))
	}
	cubic, e := NewExactBezierUV(elev)
	if e != nil {
		t.Fatal(e)
	}
	f.Outer[0].Curve = cubic
	return f
}
func TestAffineSplineFaceRationalModelQuery(t *testing.T) {
	ctx := context.Background()
	f := affineQueryFaceFixture(t)
	q, e := NewAffineSplineFaceQuery(ctx, f, AffineSplineFaceQueryOptions{})
	if e != nil {
		t.Fatal(e)
	}
	point := func(u, v *big.Rat) [3]*big.Rat {
		return [3]*big.Rat{new(big.Rat).Mul(big.NewRat(2, 1), u), new(big.Rat).Mul(big.NewRat(3, 1), v), new(big.Rat).Add(u, v)}
	}
	for _, test := range []struct {
		u, v *big.Rat
		want trimvalidate.Location
	}{{big.NewRat(1, 3), big.NewRat(1, 2), trimvalidate.Inside}, {big.NewRat(1, 2), big.NewRat(1, 2), trimvalidate.Outside}, {big.NewRat(7, 8), big.NewRat(1, 2), trimvalidate.Boundary}, {big.NewRat(2, 1), big.NewRat(1, 2), trimvalidate.Outside}} {
		r, e := q.ClassifyRationalPoint(ctx, point(test.u, test.v), geom.AffineChartInverseOptions{})
		if e != nil || !r.Proven || !r.OnPlane || r.Location != test.want || r.UV[0].Cmp(test.u) != 0 || r.UV[1].Cmp(test.v) != 0 {
			t.Fatal(r, e)
		}
	}
	p := point(big.NewRat(1, 3), big.NewRat(1, 2))
	p[2].Add(p[2], new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100)))
	r, e := q.ClassifyRationalPoint(ctx, p, geom.AffineChartInverseOptions{})
	if e != nil || !r.Proven || r.OnPlane || r.Location != trimvalidate.Outside || r.UV[0] != nil {
		t.Fatal(r, e)
	}
	normal := q.Plane()
	if normal[0].Sign() != -1 || normal[1].Sign() != -1 || normal[2].Sign() != 1 {
		t.Fatal(normal)
	}
	f.Reversed = true
	rev, e := NewAffineSplineFaceQuery(ctx, f, AffineSplineFaceQueryOptions{})
	if e != nil {
		t.Fatal(e)
	}
	other := rev.Plane()
	for i := range normal {
		if new(big.Rat).Add(normal[i], other[i]).Sign() != 0 {
			t.Fatal(normal, other)
		}
	}
	normal[0].SetInt64(99)
	if q.Plane()[0].Sign() != -1 {
		t.Fatal("plane aliases")
	}
	f.Outer[0].UVEnd.X += .01
	if _, e = NewAffineSplineFaceQuery(ctx, f, AffineSplineFaceQueryOptions{}); e == nil {
		t.Fatal("disconnected face accepted")
	}
	if _, e = NewAffineSplineFaceQuery(ctx, affineQueryFaceFixture(t), AffineSplineFaceQueryOptions{Trim: trimvalidate.Options{MaxEdges: 1}}); e == nil {
		t.Fatal("edge limit ignored")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = q.ClassifyRationalPoint(cancelled, p, geom.AffineChartInverseOptions{}); e == nil {
		t.Fatal("cancel ignored")
	}
}
