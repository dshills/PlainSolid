package trimmed

import (
	"context"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestRationalBiquadraticGridTriangleMatchesGeneralCertificate(t *testing.T) {
	ctx := context.Background()
	for _, degreeV := range []int{1, 2} {
		var h [][4]*big.Rat
		for i := 0; i <= 2; i++ {
			for j := 0; j <= degreeV; j++ {
				w := big.NewRat(int64(3+i+2*j), int64(2+j))
				h = append(h, [4]*big.Rat{
					new(big.Rat).Mul(w, big.NewRat(int64(i-1), 3)),
					new(big.Rat).Mul(w, big.NewRat(int64(i-2*j), 5)),
					new(big.Rat).Mul(w, big.NewRat(int64(i*i-j*j), 7)),
					w,
				})
			}
		}
		p, err := geom.NewExactHomogeneousBezierSurface(ctx, 2, degreeV, h, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		d := geom.ParameterRectangle{UMin: .19, UMax: .71, VMin: .37, VMax: .82}
		p, err = p.Trim(d)
		if err != nil {
			t.Fatal(err)
		}
		corners := [4][2]*big.Rat{
			{ratInset(d.UMin), ratInset(d.VMin)}, {ratInset(d.UMax), ratInset(d.VMin)},
			{ratInset(d.UMax), ratInset(d.VMax)}, {ratInset(d.UMin), ratInset(d.VMax)},
		}
		coefficients, w, ok := p.RationalBiquadraticCoefficients()
		if !ok {
			t.Fatal("missing exact power coefficients")
		}
		for half, indices := range [2][3]int{{0, 1, 2}, {0, 2, 3}} {
			var uv [3][2]*big.Rat
			var model [3][3]*big.Rat
			for i, corner := range indices {
				uv[i] = corners[corner]
				jet, err := p.EvaluateRational(ctx, uv[i][0], uv[i][1], geom.SurfacePatchEvaluationOptions{})
				if err != nil {
					t.Fatal(err)
				}
				for axis, x := range jet.Point {
					value, _ := x.Float64()
					model[i][axis] = ratInset(value)
				}
			}
			model[2][1].Add(model[2][1], big.NewRat(1, 100003))
			if gotHalf, ok := rationalBiquadraticGridHalf(d, uv); !ok || gotHalf != half {
				t.Fatal("grid diagonal ownership changed", gotHalf, half)
			}
			want := rationalBiquadraticGeneralTriangleBound(d, coefficients, w, uv, model)
			got, ok := rationalBiquadraticGridTriangleBound(coefficients, w, model, half)
			if !ok || got.Cmp(want) != 0 {
				t.Fatalf("degree V %d, half %d: bound %v differs from %v", degreeV, half, got, want)
			}
			uv[1], uv[2] = uv[2], uv[1]
			model[1], model[2] = model[2], model[1]
			if _, ok := rationalBiquadraticGridHalf(d, uv); ok {
				t.Fatal("general orientation must use the general proof")
			}
			got, ok = rationalBiquadraticRationalCorrespondenceBound(p, uv, model)
			if !ok || got.Cmp(want) != 0 {
				t.Fatal("general fallback changed under a shared barycentric permutation", got, want)
			}
		}
	}
}

func TestRationalBiquadraticGridTriangleDenominatorBudget(t *testing.T) {
	a, b := new(big.Int).Lsh(big.NewInt(1), 2500), new(big.Int).Lsh(big.NewInt(1), 2500)
	b.Sub(b, big.NewInt(1))
	values := []*big.Rat{new(big.Rat).SetFrac(big.NewInt(1), a), new(big.Rat).SetFrac(big.NewInt(1), b)}
	if _, ok := gridTriangleCommonDenominator(values); ok {
		t.Fatal("combined denominator must use the existing general proof")
	}
}
