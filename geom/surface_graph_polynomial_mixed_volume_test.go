package geom

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"testing"
)

func TestIntegrateSplineGraphPolynomialMixedVolumesOriginalUnits(t *testing.T) {
	ctx := context.Background()
	a, b, _ := polynomialPairGraphFixture(t, 0, mixedGraphGap(1, 4))
	original, e := IntegrateSplineGraphPolynomialMixedVolumes(ctx, a, b, 0, GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	transform := Translation(Vec3{X: 2, Y: -1}).Mul(Scaling(Vec3{X: 3, Y: 2, Z: 1}))
	sources := [2]*SurfacePatch{a, b}
	for i, p := range sources {
		q, e := p.Transformed(transform)
		if e != nil {
			t.Fatal(e)
		}
		sources[i] = q
		q.knotsU = append([]float64(nil), p.knotsU...)
		q.knotsV = append([]float64(nil), p.knotsV...)
		for j := range q.knotsU {
			q.knotsU[j] = 2 + 3*q.knotsU[j]
		}
		for j := range q.knotsV {
			q.knotsV[j] = -1 + 2*q.knotsV[j]
		}
	}
	scaled, e := IntegrateSplineGraphPolynomialMixedVolumes(ctx, sources[0], sources[1], 0, GraphPointContactOptions{})
	if e != nil || len(scaled.Volumes) != len(original.Volumes) || len(scaled.Arrangement.CertifiedCells) != 4 {
		t.Fatal("original physical chart measure", scaled, e)
	}
	for i, v := range original.Volumes {
		want := new(big.Rat).Mul(v, big.NewRat(6, 1))
		if scaled.Volumes[i].Cmp(want) != 0 {
			t.Fatal("independent chart area factor", i, scaled.Volumes[i], want)
		}
	}
}

func TestIntegrateSplineGraphPolynomialMixedVolumesRejectsOtherCharts(t *testing.T) {
	ctx := context.Background()
	a, b, _ := polynomialPairGraphFixture(t, 0, mixedGraphGap(3, 4))
	pair := [2]*SurfacePatch{a, b}
	for i, p := range pair {
		var e error
		pair[i], e = p.Transformed(Scaling(Vec3{X: 2, Y: 1, Z: 1}))
		if e != nil {
			t.Fatal(e)
		}
	}
	checkScope := func(name string, a, b *SurfacePatch) {
		t.Helper()
		arrangement, e := DiscoverSplineGraphPolynomialMixedContact(ctx, a, b, 0, GraphPointContactOptions{})
		if e != nil || !arrangement.Proven {
			t.Fatal(name, "fixture must prove signed geometry before the measure gate", e)
		}
		measure, e := IntegrateSplineGraphPolynomialMixedVolumes(ctx, a, b, 0, GraphPointContactOptions{})
		if !errors.Is(e, ErrConvergence) || !reflect.DeepEqual(measure, SplineGraphPolynomialMixedVolumes{}) {
			t.Fatal(name, "unsupported physical measure returned a partial volume", measure, e)
		}
	}
	checkScope("nonidentity XY", pair[0], pair[1])

	// Author identity XY with weight W=1+u. The physical height gap is
	// (v-f)(v-g)/W, while the homogeneous numerator retains the exact
	// mixed arrangement. This exercises the polynomial measure gate.
	_, _, branches := polynomialPairGraphFixture(t, 0, mixedGraphGap(1, 4))
	budget := &polynomialBudget{maxOperations: 1000, maxBits: 1024}
	product, e := polynomialProduct(branches[0], branches[1], budget)
	if e != nil {
		t.Fatal(e)
	}
	sum, e := graphPolynomialSum(branches[0], branches[1], 1, budget)
	if e != nil {
		t.Fatal(e)
	}
	const degree = 8
	bern := func(p rationalPolynomial, i int64) *big.Rat {
		value := new(big.Rat)
		for j := int64(0); j <= i && j < int64(len(p)); j++ {
			basis := new(big.Rat).SetFrac(new(big.Int).Binomial(i, j), new(big.Int).Binomial(degree, j))
			value.Add(value, basis.Mul(basis, p[j]))
		}
		return value
	}
	for owner := range pair {
		var controls [][4]*big.Rat
		for i := int64(0); i <= degree; i++ {
			u := big.NewRat(i, degree)
			w := new(big.Rat).Add(big.NewRat(1, 1), u)
			x := new(big.Rat).Add(u, big.NewRat(i*(i-1), degree*(degree-1)))
			for j := int64(0); j < 3; j++ {
				v := big.NewRat(j, 2)
				y := new(big.Rat).Mul(v, w)
				z := new(big.Rat).Mul(big.NewRat(3, 1), w)
				if owner == 0 {
					z.Add(z, bern(product, i)).Sub(z, new(big.Rat).Mul(v, bern(sum, i)))
					if j == 2 {
						z.Add(z, big.NewRat(1, 1))
					}
				}
				controls = append(controls, [4]*big.Rat{x, y, z, w})
			}
		}
		pair[owner], e = NewExactHomogeneousBezierSurface(ctx, degree, 2, controls, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
	}
	checkScope("varying positive denominator", pair[0], pair[1])
}
