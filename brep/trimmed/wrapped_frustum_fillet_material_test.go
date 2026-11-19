//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestExactFilletedFrustumMaterial(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 8, 6, o)
	if e != nil {
		t.Fatal(e)
	}
	w, _, e = geom.DraftWrappedFrustum(ctx, w, 0, 8, o)
	if e != nil {
		t.Fatal(e)
	}
	s, _, e := NewWrappedFrustum(ctx, w, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	cuts := []FilletedFrustumBoreCut{{Center: geom.Vec2{X: -2}, Radius: .5}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 1}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 4, FromUpper: true}}
	q, e := PrepareFilletedFrustumMaterial(ctx, s, 15./16, 15./8, cuts, o)
	if e != nil {
		t.Fatal(e)
	}
	// Input mutation cannot change the privately re-proved cutters.
	cuts[0].Radius = 7
	rat := func(n, d int64) *big.Rat { return big.NewRat(n, d) }
	check := func(p [3]*big.Rat, where geom.BorePointLocation, owners []int) {
		t.Helper()
		got, e := q.ClassifyRational(ctx, p)
		if e != nil || got.Location != where || !reflect.DeepEqual(got.Faces, owners) {
			t.Fatal(p, got, owners, e)
		}
	}
	point := func(x, y, z int64) [3]*big.Rat { return [3]*big.Rat{rat(x, 1), rat(y, 1), rat(z, 1)} }
	check(point(0, 0, 3), geom.BoreInside, nil)
	check(point(-2, 0, 3), geom.BoreOutside, nil)
	check(point(-2, 0, 0), geom.BoreOutside, nil)
	check(point(2, 0, 0), geom.BoreOutside, nil)
	check(point(2, 0, 1), geom.BoreBoundary, []int{q.cuts[1].floor})
	check(point(2, 0, 2), geom.BoreBoundary, []int{q.cuts[2].floor})
	check([3]*big.Rat{rat(2, 1), rat(0, 1), rat(3, 2)}, geom.BoreInside, nil)
	check([3]*big.Rat{rat(-3, 2), rat(0, 1), rat(0, 1)}, geom.BoreBoundary, []int{q.fillets.Caps[0], q.cuts[0].wall})
	check([3]*big.Rat{rat(28, 3), rat(0, 1), rat(1, 1)}, geom.BoreBoundary, []int{q.fillets.Lateral})
	// Exact Pythagorean torus normals (5/13,-12/13) and (4/5,3/5)
	// lie strictly inside the two retained meridian arcs. Both boundary points
	// are evaluated algebraically without rounded surface sampling.
	normals := [2][2]*big.Rat{{rat(5, 13), rat(-12, 13)}, {rat(4, 5), rat(3, 5)}}
	eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	for end, n := range normals {
		c := q.fillets.Circles[end]
		x := new(big.Rat).Add(c.Center[0], new(big.Rat).Mul(c.Radius, n[0]))
		z := new(big.Rat).Add(c.Center[1], new(big.Rat).Mul(c.Radius, n[1]))
		check([3]*big.Rat{x, rat(0, 1), z}, geom.BoreBoundary, []int{q.fillets.Blends[end]})
		inside, outside := new(big.Rat).Sub(x, eps), new(big.Rat).Add(x, eps)
		a, _ := inside.Float64()
		b, _ := outside.Float64()
		if a != b {
			t.Fatal("probes should round identically")
		}
		check([3]*big.Rat{inside, rat(0, 1), z}, geom.BoreInside, nil)
		check([3]*big.Rat{outside, rat(0, 1), z}, geom.BoreOutside, nil)
	}
	bad := point(0, 0, 1)
	bad[0] = new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 4097))
	if r, e := q.ClassifyRational(ctx, bad); e == nil || r.Faces != nil {
		t.Fatal("budget", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := q.ClassifyRational(cancelled, point(0, 0, 1)); e != context.Canceled || r.Faces != nil {
		t.Fatal("cancel", e)
	}
	if q.Solid().Topology().Faces != 10 {
		t.Fatal("bound native solid")
	}
}
