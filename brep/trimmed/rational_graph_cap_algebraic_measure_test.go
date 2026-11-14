package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestAlgebraicGraphCapRationalWeightsArea(t *testing.T) {
	ctx := context.Background()
	// The rational native quadrant is (1-t²,2t)/(1+t²) for t∈[0,1].
	// Clip above y=1/2, keeping a horizontal segment, an axial segment,
	// and the original rational arc from x=√3/2 to x=0.
	p := func(x, y, w int64) [3]*big.Rat {
		return [3]*big.Rat{big.NewRat(x, 1), big.NewRat(y, 1), big.NewRat(w, 1)}
	}
	b, c := p(2, 0, 2), p(0, 2, 2)
	row := []projectedGraphSectionPiece{
		{control: [][3]*big.Rat{p(0, 0, 2), p(1, 0, 2), b}},
		{control: [][3]*big.Rat{p(1, 0, 1), p(1, 1, 1), c}},
		{control: [][3]*big.Rat{c, p(0, 1, 2), p(0, 0, 2)}},
	}
	var ring trimvalidate.Loop
	for _, x := range row {
		s, e := trimvalidate.ExactBezier(x.control)
		if e != nil {
			t.Fatal(e)
		}
		ring = append(ring, s)
	}
	o := RationalGraphShellRegionOptions{}
	validated, _ := o.Topology.WithDefaults()
	r, e := certifyProjectedRationalGraphRegions(ctx, [][]projectedGraphSectionPiece{row}, []trimvalidate.Loop{ring}, horizontalPolynomialPlane(new(big.Rat)), validated)
	if e != nil {
		t.Fatal(e)
	}
	cut := [3]geom.Vec3{{Y: .5}, {Y: .5, Z: 1}, {X: 1, Y: .5}}
	clip, e := r.ClipAlgebraicHalfspace(ctx, cut, true, o)
	if e != nil || len(clip.Wires) != 1 || len(clip.Wires[0]) != 3 {
		t.Fatal("irrational contacts on a rational quadrant", clip, e)
	}
	checkAlgebraicGraphCapClosure(t, clip)
	width := big.NewRat(1, 1<<20)
	if a, e := clip.BoundPolynomialArea(ctx, width, o); !errors.Is(e, geom.ErrUnrepresentable) || a.Projection.Lower != nil {
		t.Fatal("the original polynomial query remains strict", a, e)
	}
	piece := RationalGraphShellRegionOptions{Topology: trimvalidate.Options{MaxWork: 4000000, MaxSubdivisions: 4096, MaxBits: 32768}}
	got, e := clip.BoundArea(ctx, width, piece)
	// Integral sqrt(1-y²) on [1/2,1] is π/6 - √3/8. These are
	// independent rational lower/upper bounds with margin under 1e-12.
	wantLo, _ := new(big.Rat).SetString("307092424652/1000000000000")
	wantHi, _ := new(big.Rat).SetString("307092424654/1000000000000")
	if e != nil || got.Projection.Lower.Cmp(wantLo) > 0 || got.Projection.Upper.Cmp(wantHi) < 0 ||
		new(big.Rat).Sub(got.Projection.Upper, got.Projection.Lower).Cmp(width) > 0 || got.Projection.Subdivisions < 1 {
		t.Fatal("failed certified rational quarter-segment area", got, e)
	}
	piece.Topology.MaxSubdivisions = 1
	if a, e := clip.BoundArea(ctx, width, piece); !errors.Is(e, geom.ErrConvergence) || a.Projection.Lower != nil {
		t.Fatal("the exhaustion result must be atomic", a, e)
	}
}
