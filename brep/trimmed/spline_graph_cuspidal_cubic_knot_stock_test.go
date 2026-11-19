package trimmed

import (
	"math/big"
	"testing"

	"plainsolid/geom"
)

func cuspidalCubicKnotStock(t *testing.T) [3]*geom.BSplineSurface {
	t.Helper()
	return cuspidalCubicStockWithLongitudinalKnot(t, 7.0/16)
}

func cuspidalCubicStockWithLongitudinalKnot(t *testing.T, knot float64) [3]*geom.BSplineSurface {
	t.Helper()
	src := cuspidalCubicCommonFixture(t)
	u := ratInset(knot)
	at := new(big.Rat).Mul(big.NewRat(3, 1), u)
	reversed, err := geom.RepresentedExactScalar(new(big.Rat).Sub(big.NewRat(1, 1), new(big.Rat).Mul(big.NewRat(4, 1), u)))
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(src); i++ {
		p, err := geom.NewSurfacePatch(src[i], geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			p, err = p.InsertUKnot(knot, 3, geom.SurfacePatchOptions{})
		} else {
			p, err = p.InsertVKnot(reversed, 1, geom.SurfacePatchOptions{})
		}
		if err != nil {
			t.Fatal(err)
		}
		refined, err := p.ToBSplineSurface()
		if err != nil {
			t.Fatal(err)
		}
		d := refined.Data()
		// The common hinge adds max(0, X-3*knot)/16. Greville
		// coordinates yield its exact coefficients in each owner chart.
		for u := range d.CountU {
			for v := range d.CountV {
				var x *big.Rat
				if i == 1 {
					// X=3u, so the degree-three Greville denominator cancels.
					x = new(big.Rat).Add(ratInset(d.KnotsU[u+1]), ratInset(d.KnotsU[u+2]))
					x.Add(x, ratInset(d.KnotsU[u+3]))
				} else {
					x = new(big.Rat).Sub(big.NewRat(1, 1), ratInset(d.KnotsV[v+1]))
					x.Mul(x, big.NewRat(3, 4))
				}
				shift := new(big.Rat).Sub(x, at)
				if shift.Sign() > 0 {
					pole := &d.Poles[u*d.CountV+v]
					z := new(big.Rat).Add(ratInset(pole.Z), shift.Mul(shift, big.NewRat(1, 16)))
					pole.Z, err = geom.RepresentedExactScalar(z)
					if err != nil {
						t.Fatal(err)
					}
				}
			}
		}
		src[i], err = geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
		if err != nil {
			t.Fatal(err)
		}
	}
	return src
}
