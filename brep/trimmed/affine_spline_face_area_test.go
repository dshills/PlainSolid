//go:build research

package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
	"time"
)

func TestAffineSplineFaceSourceArea(t *testing.T) {
	ctx := context.Background()
	f := affineQueryFaceFixture(t)
	q, e := NewAffineSplineFaceQuery(ctx, f, AffineSplineFaceQueryOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := q.AreaBounds(ctx, big.NewRat(1, 1000000), 0)
	if e != nil {
		t.Fatal(e)
	}
	want := big.NewRat(343, 108)
	if r.ModelArea.Lower.Cmp(want) != 0 || r.ModelArea.Upper.Cmp(want) != 0 || r.GramSquared.Cmp(big.NewRat(49, 1)) != 0 {
		t.Fatal(r, want)
	}
	f.Reversed = true
	rev, e := NewAffineSplineFaceQuery(ctx, f, AffineSplineFaceQueryOptions{})
	if e != nil {
		t.Fatal(e)
	}
	back, e := rev.AreaBounds(ctx, big.NewRat(1, 1000000), 0)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 3; i++ {
		if new(big.Rat).Add(r.AreaVector[i].Lower, back.AreaVector[i].Upper).Sign() != 0 {
			t.Fatal(r, back)
		}
	}
	if back.ModelArea.Lower.Cmp(want) != 0 {
		t.Fatal(back)
	}
}
func TestAffineSplineFaceRationalCircleArea(t *testing.T) {
	ctx := context.Background()
	f := affineQueryFaceFixture(t)
	f.Outer = nil
	f.Holes = nil
	for k := 0; k < 4; k++ {
		h := [3][3]*big.Rat{{big.NewRat(1, 4), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 4), big.NewRat(1, 4), big.NewRat(1, 1)}, {new(big.Rat), big.NewRat(1, 2), big.NewRat(2, 1)}}
		for i := range h {
			for j := 0; j < k; j++ {
				h[i][0], h[i][1] = new(big.Rat).Neg(h[i][1]), h[i][0]
			}
			for axis := 0; axis < 2; axis++ {
				h[i][axis].Add(h[i][axis], new(big.Rat).Quo(h[i][2], big.NewRat(2, 1)))
			}
		}
		c, e := NewExactQuadraticUV(h)
		if e != nil {
			t.Fatal(e)
		}
		_, ends := c.Endpoints()
		f.Outer = append(f.Outer, Use{Edge: k, Curve: c, UVStart: ends[0], UVEnd: ends[1]})
	}
	q, e := NewAffineSplineFaceQuery(ctx, f, AffineSplineFaceQueryOptions{})
	if e != nil {
		t.Fatal(e)
	}
	start := time.Now()
	r, e := q.AreaBounds(ctx, big.NewRat(1, 10000), 0)
	if e != nil {
		t.Fatal(e)
	}
	lo, _ := r.ModelArea.Lower.Float64()
	hi, _ := r.ModelArea.Upper.Float64()
	want := 7 * math.Pi / 16
	if lo > want || hi < want || hi-lo > 1e-4 {
		t.Fatal(lo, hi, want)
	}
	t.Logf("circle area width %.9g; work %d splits %d time %v", hi-lo, r.Work, r.Subdivisions, time.Since(start))
	start = time.Now()
	tight, e := q.AreaBounds(ctx, big.NewRat(1, 100000), 0)
	if e != nil {
		t.Fatal(e)
	}
	width, _ := new(big.Rat).Sub(tight.ModelArea.Upper, tight.ModelArea.Lower).Float64()
	if width > 1e-5 {
		t.Fatal(width)
	}
	t.Logf("tight circle width %.9g; work %d splits %d time %v", width, tight.Work, tight.Subdivisions, time.Since(start))
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	bad, e := q.AreaBounds(cancelled, big.NewRat(1, 100), 0)
	if e == nil || bad.ModelArea.Lower != nil {
		t.Fatal(bad, e)
	}
	// A different affine metric has irrational area scale sqrt(61).
	s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 3, Z: 2}, {X: 2, Z: 1}, {X: 2, Y: 3, Z: 3}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	f.Surface = s
	q, e = NewAffineSplineFaceQuery(ctx, f, AffineSplineFaceQueryOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e = q.AreaBounds(ctx, big.NewRat(1, 10000), 0)
	if e != nil {
		t.Fatal(e)
	}
	lo, _ = r.ModelArea.Lower.Float64()
	hi, _ = r.ModelArea.Upper.Float64()
	want = math.Sqrt(61) * math.Pi / 16
	if lo > want || hi < want {
		t.Fatal(lo, hi, want)
	}
	if _, e = q.AreaBounds(ctx, big.NewRat(1, 10000), 1); e == nil {
		t.Fatal("coarse Gram root accepted")
	}
}
