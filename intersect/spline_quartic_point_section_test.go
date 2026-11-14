package intersect

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func quarticSectionRootPower(r *big.Rat, n int) []*big.Rat {
	out := make([]*big.Rat, n+1)
	for i := range out {
		q := big.NewRat(1, 1)
		for j := 0; j < n-i; j++ {
			q.Mul(q, new(big.Rat).Neg(r))
		}
		q.Mul(q, new(big.Rat).SetInt(new(big.Int).Binomial(int64(n), int64(i))))
		out[i] = q
	}
	return out
}
func quarticPointSectionFixture(t *testing.T, coeff [5]int64) (Patch, Patch) {
	t.Helper()
	power := map[[2]int]*big.Rat{}
	for j, c := range coeff {
		up, vp := quarticSectionRootPower(big.NewRat(1, 3), 4-j), quarticSectionRootPower(big.NewRat(2, 5), j)
		scale := new(big.Int).Mul(new(big.Int).Exp(big.NewInt(3), big.NewInt(int64(4-j)), nil), new(big.Int).Exp(big.NewInt(5), big.NewInt(int64(j)), nil))
		for u, x := range up {
			for v, y := range vp {
				q := new(big.Rat).Mul(x, y)
				q.Mul(q, new(big.Rat).SetInt(scale))
				q.Mul(q, big.NewRat(c, 1))
				key := [2]int{u, v}
				if power[key] == nil {
					power[key] = new(big.Rat)
				}
				power[key].Add(power[key], q)
			}
		}
	}
	var poles [2][]geom.Vec3
	for i := range 5 {
		for j := range 5 {
			z := new(big.Rat)
			for ij, q := range power {
				if ij[0] > i || ij[1] > j {
					continue
				}
				u := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(ij[0])), new(big.Int).Binomial(4, int64(ij[0])))
				v := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(j), int64(ij[1])), new(big.Int).Binomial(4, int64(ij[1])))
				z.Add(z, new(big.Rat).Mul(q, new(big.Rat).Mul(u, v)))
			}
			base := big.NewRat(int64(i*j), 1024)
			z.Add(z, base)
			f, err := geom.RepresentedExactScalar(z)
			if err != nil {
				t.Fatal("source is not dyadic", i, j, z)
			}
			poles[0] = append(poles[0], geom.Vec3{X: float64(i) / 4, Y: float64(j) / 4, Z: f})
			poles[1] = append(poles[1], geom.Vec3{X: float64(i) / 4, Y: float64(j) / 4, Z: float64(i*j) / 1024})
		}
	}
	k := []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}
	var out [2]Patch
	for i := range out {
		s, e := geom.NewBSplineSurface(4, 4, 5, 5, poles[i], k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		out[i] = Patch{Surface: s, Max: geom.Vec2{X: 1, Y: 1}}
		if e != nil {
			t.Fatal(e)
		}
	}
	return out[0], out[1]
}

func TestSplineQuarticPointSectionOriginalOwnership(t *testing.T) {
	ctx := context.Background()
	a, b := quarticPointSectionFixture(t, [5]int64{2, 4, 6, 4, 2})
	uv := [2]*big.Rat{big.NewRat(1, 3), big.NewRat(2, 5)}
	call := func(a, b Patch, uv [2]*big.Rat, o geom.GraphPointContactOptions) (*SplineQuarticPointSection, error) {
		return CertifySplinePairQuarticPointSection(ctx, a, b, [2]int{0, 1}, uv, new(big.Rat), geom.SurfacePatchOptions{}, o)
	}
	r, e := call(a, b, uv, geom.GraphPointContactOptions{})
	if e != nil || !r.Complete() || r.Order() != 4 {
		t.Fatal(r, e)
	}
	owner, e := r.Owner(0)
	if e != nil || owner != a {
		t.Fatal(owner, e)
	}
	point, e := r.Point()
	if e != nil || point[2].Cmp(big.NewRat(1, 480)) != 0 {
		t.Fatal(point, e)
	}
	// A detached exact accessor and a mutable caller proposal cannot alter proof.
	point[2].SetInt64(99)
	uv[0].SetInt64(0)
	point, _ = r.Point()
	params, _ := r.Parameters(0)
	if point[2].Cmp(big.NewRat(1, 480)) != 0 || params[0].Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal("mutable result")
	}
	for _, cap := range []int{r.ExactOperations(), r.ExactOperations() - 1} {
		got, e := call(a, b, params, geom.GraphPointContactOptions{MaxOperations: cap})
		if cap == r.ExactOperations() {
			if e != nil {
				t.Fatal(e)
			}
		} else if e == nil || got != nil {
			t.Fatal("partial publication", e)
		}
	}
	swapped, e := call(b, a, params, geom.GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	_, _, side, _ := swapped.Separation()
	if side != -1 {
		t.Fatal(side)
	}
	px, e := a.Surface.(*geom.BSplineSurface).PermutedModelAxes([3]int{0, 2, 1})
	if e != nil {
		t.Fatal(e)
	}
	qx, e := b.Surface.(*geom.BSplineSurface).PermutedModelAxes([3]int{0, 2, 1})
	if e != nil {
		t.Fatal(e)
	}
	projected, e := CertifySplinePairQuarticPointSection(ctx, Patch{Surface: px, Max: a.Max}, Patch{Surface: qx, Max: b.Max}, [2]int{0, 2}, params, new(big.Rat), geom.SurfacePatchOptions{}, geom.GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	xyz, _ := projected.Point()
	axes, residual, _, _ := projected.Separation()
	if axes != [2]int{0, 2} || residual != 1 || xyz[1].Cmp(big.NewRat(1, 480)) != 0 {
		t.Fatal(xyz, axes, residual)
	}
	// Original trim domain excludes the point. No whole-support evidence reused.
	clipped := a
	clipped.Min.X = .5
	if got, e := call(clipped, b, params, geom.GraphPointContactOptions{}); e == nil || got != nil {
		t.Fatal("clipped witness accepted")
	}
	badA, badB := quarticPointSectionFixture(t, [5]int64{1, 4, 6, 4, 1})
	if got, e := call(badA, badB, params, geom.GraphPointContactOptions{}); e == nil || got != nil {
		t.Fatal("nondefinite contact accepted")
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if got, e := CertifySplinePairQuarticPointSection(cancelCtx, a, b, [2]int{0, 1}, params, new(big.Rat), geom.SurfacePatchOptions{}, geom.GraphPointContactOptions{}); e == nil || got != nil {
		t.Fatal("cancel")
	}
}
