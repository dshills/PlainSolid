package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestCertifyProjectiveSurfaceHeightOrderSubULPAndCoincidence(t *testing.T) {
	ctx := context.Background()
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	b := projectiveOverlapTestChart(t, false, false)
	want := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	same, err := CertifyProjectiveSurfaceHeightOrder(ctx, a, b, [2]int{0, 1}, GeneralSurfaceHeightOptions{})
	if err != nil || !same.Complete || same.Sign != 0 ||
		same.Difference.Lower.Sign() != 0 || same.Difference.Upper.Sign() != 0 {
		t.Fatal("fractional original chart of the same lift must prove equal", same, err)
	}
	for i, p := range b.net {
		b.net[i][2] = new(big.Rat).Add(p[2], new(big.Rat).Mul(p[3], want))
	}
	for i, p := range [][2]*SurfacePatch{{b, a}, {a, b}} {
		expected := new(big.Rat).Set(want)
		if i == 1 {
			expected.Neg(expected)
		}
		r, err := CertifyProjectiveSurfaceHeightOrder(ctx, p[0], p[1], [2]int{0, 1}, GeneralSurfaceHeightOptions{})
		if err != nil || !r.Complete || r.Sign != expected.Sign() ||
			r.Difference.Lower.Cmp(expected) != 0 || r.Difference.Upper.Cmp(expected) != 0 {
			t.Fatal("signed bound must retain original sub-ULP lift", i, r, err)
		}
		short, err := CertifyProjectiveSurfaceHeightOrder(ctx, p[0], p[1], [2]int{0, 1},
			GeneralSurfaceHeightOptions{MaxOperations: r.ExactOperations - 1})
		if !errors.Is(err, ErrConvergence) || short.Complete || short.Difference.Lower != nil {
			t.Fatal("no partial hull may escape a short joint work budget", i, short, err)
		}
	}
}

func TestCertifyProjectiveSurfaceHeightOrderOppositeInterior(t *testing.T) {
	ctx := context.Background()
	base := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	var poles []Vec3
	for u, z := range [3]float64{1, -2, 1} {
		for v := range 2 {
			poles = append(poles, Vec3{float64(u) / 2, float64(v), z})
		}
	}
	raw, err := NewBSplineSurface(2, 1, 3, 2, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	curved, err := NewSurfacePatch(raw, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := CertifyProjectiveSurfaceHeightOrder(ctx, curved, base, [2]int{0, 1}, GeneralSurfaceHeightOptions{})
	if !errors.Is(err, ErrConvergence) || r.Complete || r.Difference.Lower == nil ||
		r.Difference.Lower.Cmp(big.NewRat(-1, 2)) > 0 || r.Difference.Upper.Cmp(big.NewRat(1, 1)) < 0 {
		t.Fatal("consistent original corners do not prove a strict order", r, err)
	}
}
