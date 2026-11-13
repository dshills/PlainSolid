package trimmed

import (
	"math/big"
	"plainsolid/geom"
	"testing"
)

func referenceRationalBiquadraticBound(p *geom.SurfacePatch, uv [3][2]*big.Rat, model [3][3]*big.Rat) (*big.Rat, bool) {
	coefficients, minimumWeight, ok := p.RationalBiquadraticCoefficients()
	if !ok {
		return nil, false
	}
	d := p.Domain()
	du := new(big.Rat).Sub(ratInset(d.UMax), ratInset(d.UMin))
	dv := new(big.Rat).Sub(ratInset(d.VMax), ratInset(d.VMin))
	var powers [2][3]trianglePower5
	for axis := 0; axis < 2; axis++ {
		values := [3]*big.Rat{}
		for i, v := range uv {
			value, origin, width := v[0], d.UMin, du
			if axis == 1 {
				value, origin, width = v[1], d.VMin, dv
			}
			values[i] = new(big.Rat).Quo(new(big.Rat).Sub(value, ratInset(origin)), width)
		}
		linear := zeroTrianglePower5()
		linear[0][0].Set(values[0])
		linear[1][0].Sub(values[1], values[0])
		linear[0][1].Sub(values[2], values[0])
		powers[axis][0] = zeroTrianglePower5()
		powers[axis][0][0][0].SetInt64(1)
		powers[axis][1] = linear
		powers[axis][2] = multiplyTrianglePower5(linear, linear)
	}
	var homogeneous [4]trianglePower5
	for k := range homogeneous {
		homogeneous[k] = zeroTrianglePower5()
	}
	for a := 0; a <= 2; a++ {
		for b := 0; b <= 2; b++ {
			term := multiplyTrianglePower5(powers[0][a], powers[1][b])
			for i := 0; i <= 5; i++ {
				for j := 0; j <= 5-i; j++ {
					for k := 0; k < 4; k++ {
						homogeneous[k][i][j].Add(homogeneous[k][i][j], new(big.Rat).Mul(term[i][j], coefficients[a][b][k]))
					}
				}
			}
		}
	}
	target := [3]exactBoundVector{exactBoundVector(model[0]), exactBoundVector(model[1]), exactBoundVector(model[2])}
	var residual [3]trianglePower5
	for k := 0; k < 3; k++ {
		affine := zeroTrianglePower5()
		affine[0][0].Set(target[0][k])
		affine[1][0].Sub(target[1][k], target[0][k])
		affine[0][1].Sub(target[2][k], target[0][k])
		product := multiplyTrianglePower5(homogeneous[3], affine)
		residual[k] = zeroTrianglePower5()
		for i := 0; i <= 5; i++ {
			for j := 0; j <= 5-i; j++ {
				residual[k][i][j].Sub(homogeneous[k][i][j], product[i][j])
			}
		}
	}
	falling := func(n, k int) int64 {
		r := int64(1)
		for i := 0; i < k; i++ {
			r *= int64(n - i)
		}
		return r
	}
	bound := new(big.Rat)
	// s^a*t^b has control (i)_a*(j)_b/(5)_(a+b) at index(i,j).
	for i := 0; i <= 5; i++ {
		for j := 0; j <= 5-i; j++ {
			var control exactBoundVector
			for k := range control {
				control[k] = new(big.Rat)
			}
			for a := 0; a <= i; a++ {
				for b := 0; b <= j; b++ {
					factor := big.NewRat(falling(i, a)*falling(j, b), falling(5, a+b))
					for k := 0; k < 3; k++ {
						control[k].Add(control[k], new(big.Rat).Mul(residual[k][a][b], factor))
					}
				}
			}
			squared := boundDot(control, control)
			if squared.Cmp(bound) > 0 {
				bound.Set(squared)
			}
		}
	}
	return bound.Quo(bound, new(big.Rat).Mul(minimumWeight, minimumWeight)), true
}

func sparseResidualFixture(t testing.TB, dense bool) (*geom.SurfacePatch, [3][2]*big.Rat, [3][3]*big.Rat) {
	t.Helper()
	var poles []geom.Vec3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			poles = append(poles, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: float64(i*j) / 4})
		}
	}
	var w []float64
	if dense {
		w = []float64{1, .25, 2, .5, 3, 1.25, 2, .75, 1}
	}
	k := []float64{0, 0, 0, 1, 1, 1}
	s, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, k, k, w)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	uv := [3][2]*big.Rat{{big.NewRat(1, 7), big.NewRat(1, 11)}, {big.NewRat(6, 7), big.NewRat(2, 11)}, {big.NewRat(3, 7), big.NewRat(9, 11)}}
	m := [3][3]*big.Rat{{big.NewRat(1, 7), big.NewRat(1, 11), big.NewRat(0, 1)}, {big.NewRat(6, 7), big.NewRat(2, 11), big.NewRat(1, 8)}, {big.NewRat(3, 7), big.NewRat(9, 11), big.NewRat(1, 2)}}
	return p, uv, m
}
func BenchmarkRationalBiquadraticResidual(b *testing.B) {
	for _, dense := range []bool{false, true} {
		name := "sparse"
		if dense {
			name = "dense"
		}
		for _, baseline := range []bool{true, false} {
			mode := "optimized"
			if baseline {
				mode = "reference"
			}
			b.Run(name+"/"+mode, func(b *testing.B) {
				p, uv, m := sparseResidualFixture(b, dense)
				fn := rationalBiquadraticRationalCorrespondenceBound
				if baseline {
					fn = referenceRationalBiquadraticBound
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, ok := fn(p, uv, m); !ok {
						b.Fatal("bound")
					}
				}
			})
		}
	}
}
func TestRationalBiquadraticSparseEquivalence(t *testing.T) {
	for _, dense := range []bool{false, true} {
		p, uv, m := sparseResidualFixture(t, dense)
		for i := 0; i < 20; i++ {
			m[0][2] = big.NewRat(int64(i-10), 32)
			got, ok := rationalBiquadraticRationalCorrespondenceBound(p, uv, m)
			want, wok := referenceRationalBiquadraticBound(p, uv, m)
			if !ok || !wok || got.Cmp(want) != 0 {
				t.Fatal("exact result differs", i, dense, got, want)
			}
			got.SetInt64(99)
			again, _ := rationalBiquadraticRationalCorrespondenceBound(p, uv, m)
			if again.Cmp(want) != 0 {
				t.Fatal("mutable cached proof")
			}
		}
	}
}
