package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestTranslatedCylinderTorusCavityMaterial(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	s, e := NewTranslatedCylinderTorusCavityMaterial(5, 2, 10, 0, 7, 1, 1, 3)
	if e != nil {
		t.Fatal(e)
	}
	l, m, e := s.ClassifyRational(ctx, [3]*big.Rat{r(8, 1), r(1, 1), r(3, 1)}, SurfacePatchOptions{})
	if e != nil || l != BoreBoundary || m != TorusCylinderTorus {
		t.Fatal(l, m, e)
	}
	p := [3]*big.Rat{r(0, 1), r(1, 1), r(3, 1)}
	d := [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}
	line, e := s.ClipRationalLine(ctx, p, d, r(-11, 1), r(11, 1), ToroidLineOptions{ParameterTolerance: 1e-9})
	if e != nil || len(line.Intervals) != 3 {
		t.Fatal(e, line)
	}
	source, z, _ := s.ShiftedSource()
	z.SetInt64(99)
	base, e := source.ClipRationalLine(ctx, [3]*big.Rat{r(0, 1), r(1, 1), r(0, 1)}, d, r(-11, 1), r(11, 1), ToroidLineOptions{ParameterTolerance: 1e-9})
	if e != nil || len(base.Intervals) != len(line.Intervals) {
		t.Fatal(e)
	}
	if _, e = NewTranslatedCylinderTorusCavityMaterial(5, 2, 10, 1, 7, 1, 1, 3); e == nil {
		t.Fatal("axial contact")
	}
	if _, _, e = s.ClassifyRational(ctx, p, SurfacePatchOptions{MaxOperations: 1}); e == nil {
		t.Fatal("budget")
	}
}
