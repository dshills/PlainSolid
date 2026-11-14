package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/placement"
	"plainsolid/geom"
)

func TestExactPlacedAlgebraicMassOfAsymmetricBox(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	point := func(x *big.Rat) geom.ExactParameterInterval {
		return geom.ExactParameterInterval{Lower: new(big.Rat).Set(x), Upper: new(big.Rat).Set(x)}
	}
	m := AlgebraicShellMassBounds{}
	// A 2 by 3 by 4 box centered at (1,-1,2), density one.
	m.Volume = point(r(24, 1))
	ori := [3][3]int64{{170, 24, -48}, {24, 160, 48}, {-48, 48, 74}}
	for i, c := range []int64{1, -1, 2} {
		m.Centroid[i], m.FirstMoment[i] = point(r(c, 1)), point(r(24*c, 1))
		for j := 0; j < 3; j++ {
			m.Inertia[i][j] = point(r(ori[i][j], 1))
		}
	}
	matrix := [3][4]*big.Rat{
		{r(3, 5), r(0, 1), r(4, 5), r(7, 3)},
		{r(0, 1), r(1, 1), r(0, 1), r(2, 7)},
		{r(-4, 5), r(0, 1), r(3, 5), r(-1, 3)},
	}
	pose, err := placement.NewRationalRigid(matrix, 4096)
	if err != nil {
		t.Fatal(err)
	}
	matrix, err = pose.Matrix()
	if err != nil {
		t.Fatal(err)
	}
	got, err := transformAlgebraicMass(ctx, m, matrix, 4096)
	if err != nil {
		t.Fatal(err)
	}
	wantCenter := [3]*big.Rat{r(68, 15), r(-5, 7), r(1, 15)}
	// Integrate the transformed affine box parameterization. The second
	// moment at its center is R*diag(8,18,32)*Rᵀ independently of translation.
	wantInertia := [3][3]*big.Rat{
		{r(172694, 3675), r(544, 7), r(-1408, 75)},
		{r(544, 7), r(1600, 3), r(8, 7)},
		{r(-1408, 75), r(8, 7), r(2009606, 3675)},
	}
	equal := func(b geom.ExactParameterInterval, w *big.Rat) bool {
		return b.Lower != nil && b.Upper != nil && b.Lower.Cmp(w) == 0 && b.Upper.Cmp(w) == 0
	}
	if !equal(got.Volume, r(24, 1)) {
		t.Fatal(got.Volume)
	}
	for i, c := range wantCenter {
		if !equal(got.Centroid[i], c) || !equal(got.FirstMoment[i], new(big.Rat).Mul(c, r(24, 1))) {
			t.Fatal("world centroid or first moment", i, got.Centroid[i], got.FirstMoment[i])
		}
		for j, v := range wantInertia[i] {
			if !equal(got.Inertia[i][j], v) {
				t.Fatal("origin inertia", i, j, got.Inertia[i][j], v)
			}
		}
	}
	matrix[0][3] = new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 40))
	if partial, err := transformAlgebraicMass(ctx, m, matrix, 64); !errors.Is(err, geom.ErrConvergence) || partial.Volume.Lower != nil {
		t.Fatal("coefficient exhaustion must publish no placed mass", partial, err)
	}
	cancel, cancelNow := context.WithCancel(ctx)
	cancelNow()
	if partial, err := transformAlgebraicMass(cancel, m, matrix, 4096); !errors.Is(err, context.Canceled) || partial.Volume.Lower != nil {
		t.Fatal("cancellation must publish no mass", partial, err)
	}
}
