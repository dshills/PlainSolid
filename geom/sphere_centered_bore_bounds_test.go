package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestSphereCenteredBoreBounds(t *testing.T) {
	for _, v := range [][3]float64{{5, 2, .5}, {5, 2, 1.75}, {2, 1, .125}} {
		R, r, d := v[0], v[1], v[2]
		q, e := BoundSphereCenteredBore(context.Background(), R, r, d, SurfacePatchEvaluationOptions{})
		if e != nil || !q.Proven {
			t.Fatal(q, e)
		}
		bound := func(q *big.Rat) float64 { x, _ := q.Float64(); return x }
		for i := 0; i < 129; i++ {
			u := float64(i) * 2 * math.Pi / 128
			s, c := math.Sincos(u)
			E := r*r - d*d*s*s
			root := math.Sqrt(E)
			ep := -2 * d * d * s * c
			epp := -2 * d * d * (c*c - s*s)
			rho := d*c + root
			rp := -d*s + ep/(2*root)
			rpp := -d*c + epp/(2*root) - ep*ep/(4*root*root*root)
			h := math.Sqrt(R*R - rho*rho)
			hp := -rho * rp / h
			hpp := -(rp*rp+rho*rpp)/h - rho*rho*rp*rp/(h*h*h)
			lp := -rp / h
			lpp := -rpp/h + rp*hp/(h*h)
			if rho < bound(q.Rho[0])-1e-13 || rho > bound(q.Rho[1])+1e-13 || h < bound(q.Height[0])-1e-13 || h > bound(q.Height[1])+1e-13 {
				t.Fatal(v, u, rho, h)
			}
			for j, x := range []float64{rp, rpp, hp, hpp, lp, lpp} {
				limit := []*big.Rat{q.RhoFirst, q.RhoSecond, q.HeightFirst, q.HeightSecond, q.LatitudeFirst, q.LatitudeSecond}[j]
				if math.Abs(x) > bound(limit)+1e-12 {
					t.Fatal(v, u, j, x, limit)
				}
			}
		}
	}
	q, e := BoundSphereCenteredBore(context.Background(), 5, 3, 0, SurfacePatchEvaluationOptions{})
	if e != nil || q.Height[0].Cmp(big.NewRat(4, 1)) != 0 || q.Height[1].Cmp(big.NewRat(4, 1)) != 0 {
		t.Fatal(q, e)
	}
	for _, v := range []*big.Rat{q.RhoFirst, q.RhoSecond, q.HeightFirst, q.HeightSecond, q.LatitudeFirst, q.LatitudeSecond} {
		if v.Sign() != 0 {
			t.Fatal(v)
		}
	}
	for _, v := range [][3]float64{{3, 2, 1}, {5, 2, 2}} {
		q, e = BoundSphereCenteredBore(context.Background(), v[0], v[1], v[2], SurfacePatchEvaluationOptions{})
		if e != ErrDomain || q.Proven {
			t.Fatal(q, e)
		}
	}
	q, e = BoundSphereCenteredBore(context.Background(), 5, 2, .5, SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || q.Proven {
		t.Fatal(q, e)
	}
}
