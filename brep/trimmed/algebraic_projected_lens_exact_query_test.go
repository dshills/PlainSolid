package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/geom"
)

func TestProjectedLensQueriesUseOriginalExactRoof(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	l := projectedExactRoofLens(t, true)
	bounds := [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}
	line, e := l.ClipMaterialLine(ctx, geom.Vec3{Y: .5, Z: 9.0 / 256}, geom.Vec3{X: 1}, bounds, AlgebraicLensLineOptions{})
	if e != nil || !line.Complete || len(line.Intervals) != 1 || len(line.Isolated) != 0 {
		t.Fatal("line clipped by original exact roof", line, e)
	}
	iv := line.Intervals[0]
	for i, h := range [2]AlgebraicTubeLensLineEvent{iv.Start, iv.End} {
		root, e := h.Parameter.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
		// X=1/4 and X=3/4 from R²-Z=1/16.
		if e != nil || root.Kind != geom.ExactRootKindRational || root.Rational.Cmp(big.NewRat(int64(1+2*i), 4)) != 0 || h.BoundaryMask != 1 {
			t.Fatal("original roof line ownership", i, root, h.BoundaryMask, e)
		}
	}
	section, e := l.SectionProjectionPlane(ctx, [2]*big.Rat{new(big.Rat), big.NewRat(1, 2)},
		[2]*big.Rat{big.NewRat(1, 1), new(big.Rat)}, bounds, AlgebraicLensLineOptions{})
	if e != nil || !section.Complete || len(section.Regions) != 1 || len(section.Isolated) != 0 {
		t.Fatal("through-axis plane section", section, e)
	}
	r := section.Regions[0]
	for i, p := range r.Parameter {
		root, e := p.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
		if e != nil || root.Kind != geom.ExactRootKindRational || root.Rational.Cmp(big.NewRat(int64(3+10*i), 16)) != 0 {
			t.Fatal("exact plane rim terminal", i, root, e)
		}
	}
	if !r.Curves[1].Complete || len(r.Curves[1].Cells) != 1 {
		t.Fatal("missing original exact roof section")
	}
	want := [3]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2), big.NewRat(25, 256)}
	dist, e := l.BoundBoundaryDistance(ctx, want, AlgebraicTubeLensDistanceOptions{MaxCells: 2})
	if e != nil || !dist.Converged || dist.WitnessChart != 1 || dist.Squared.Lower.Sign() != 0 || dist.Squared.Upper.Sign() != 0 {
		t.Fatal("exact crown distance", dist, e)
	}
	limited := AlgebraicLensLineOptions{Roots: geom.ExactCurvePlaneEventOptions{MaxOperations: 10}}
	line, e = l.ClipMaterialLine(ctx, geom.Vec3{Y: .5, Z: 9.0 / 256}, geom.Vec3{X: 1}, bounds, limited)
	if !errors.Is(e, geom.ErrConvergence) || line.Complete || len(line.Intervals) != 0 || len(line.Isolated) != 0 {
		t.Fatal("line failure exposed partial incidence", line, e)
	}
}
