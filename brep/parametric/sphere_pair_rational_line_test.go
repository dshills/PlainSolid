package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestNativeSpherePairRationalLine(t *testing.T) {
	ctx := context.Background()
	s, e := NewSpherePairSet(ctx, Ball{Center: geom.Vec3{X: 1, Y: 2, Z: 3}, Radius: 5}, Ball{Center: geom.Vec3{X: 3, Y: 2, Z: 3}, Radius: 5}, Union, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	r := big.NewRat
	line, e := s.ClipRationalLine(ctx, [3]*big.Rat{r(4, 3), r(2, 1), r(3, 1)}, [3]*big.Rat{r(2, 3), r(0, 1), r(0, 1)}, r(-10, 1), r(12, 1), geom.ToroidLineOptions{ParameterTolerance: 1e-9})
	if e != nil || len(line.Intervals) != 1 || line.Intervals[0].StartBoundary != 1 || line.Intervals[0].EndBoundary != 2 {
		t.Fatal(line, e)
	}
	for i, root := range []*geom.ExactAlgebraicRoot{line.Intervals[0].Start, line.Intervals[0].End} {
		value := int64(-8)
		if i == 1 {
			value = 10
		}
		want, _, e := geom.NewExactRationalRoot(ctx, r(value, 1), geom.ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		cmp, e := root.Compare(ctx, want, geom.ExactCurvePlaneEventOptions{})
		if e != nil || !cmp.Proven || cmp.Order != 0 {
			t.Fatal(cmp, e)
		}
	}
	owners := s.SourceFaces()
	if len(owners[0]) != 1 || len(owners[1]) != 1 || owners[0][0] == owners[1][0] {
		t.Fatal("native original source identity")
	}
}
