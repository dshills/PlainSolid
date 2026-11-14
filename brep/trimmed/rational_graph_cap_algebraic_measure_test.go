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
	// The nearest point to (1,1,0) on the retained quadrant is
	// (1/√2,1/√2,0), inside the rational arc's true shortened domain.
	query := [3]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1), new(big.Rat)}
	near, e := clip.NearestQuadraticRational(ctx, query, geom.ExactCurvePlaneEventOptions{})
	if e != nil || near.FaceInterior || near.Wire < 0 || near.Use < 0 || near.CurveParameter == nil ||
		clip.Wires[near.Wire][near.Use].InputPiece != 1 {
		t.Fatal("exact rational arc distance and original owner", near, e)
	}
	// D=(√2−1)²=3−2√2, so D²−6D+1 vanishes.
	identity, e := near.SquaredDistance.SignPolynomial(ctx, []*big.Rat{big.NewRat(1, 1), big.NewRat(-6, 1), big.NewRat(1, 1)}, geom.ExactCurvePlaneEventOptions{})
	if e != nil || identity.Sign != 0 {
		t.Fatal("weighted stationary root does not satisfy independent distance", identity, e)
	}
	square, e := near.SquaredDistance.Enclose(ctx, big.NewRat(1, 64), geom.ExactCurvePlaneEventOptions{})
	if e != nil || square.Interval.Upper.Cmp(big.NewRat(1, 1)) >= 0 {
		t.Fatal("weighted stationary distance should be the smaller conjugate", square, e)
	}
	piece.Topology.MaxSubdivisions = 1
	if a, e := clip.BoundArea(ctx, width, piece); !errors.Is(e, geom.ErrConvergence) || a.Projection.Lower != nil {
		t.Fatal("the exhaustion result must be atomic", a, e)
	}
}

func TestNearestPolynomialAlgebraicCap(t *testing.T) {
	ctx := context.Background()
	point := func(x, y *big.Rat) [3]*big.Rat { return [3]*big.Rat{x, y, big.NewRat(1, 1)} }
	rat := big.NewRat
	row := [][][3]*big.Rat{
		{point(rat(0, 1), rat(0, 1)), point(rat(2, 1), rat(0, 1)), point(rat(4, 1), rat(0, 1))},
		{point(rat(4, 1), rat(0, 1)), point(rat(4, 1), rat(1, 2)), point(rat(4, 1), rat(1, 1))},
		{point(rat(4, 1), rat(1, 1)), point(rat(2, 1), rat(3, 1)), point(rat(0, 1), rat(1, 1))},
		{point(rat(0, 1), rat(1, 1)), point(rat(0, 1), rat(1, 2)), point(rat(0, 1), rat(0, 1))},
	}
	var wire []projectedGraphSectionPiece
	var ring trimvalidate.Loop
	for i, controls := range row {
		s, e := trimvalidate.ExactBezier(controls)
		if e != nil {
			t.Fatal(e)
		}
		wire = append(wire, projectedGraphSectionPiece{control: controls, source: RationalGraphCapEdgeSource{Piece: i}})
		ring = append(ring, s)
	}
	opts, _ := (trimvalidate.Options{}).WithDefaults()
	r, e := certifyProjectedRationalGraphRegions(ctx, [][]projectedGraphSectionPiece{wire}, []trimvalidate.Loop{ring}, horizontalPolynomialPlane(new(big.Rat)), opts)
	if e != nil {
		t.Fatal(e)
	}
	y := 1.5
	clip, e := r.ClipAlgebraicHalfspace(ctx, [3]geom.Vec3{{Y: y}, {Y: y, Z: 1}, {X: 1, Y: y}}, true, RationalGraphShellRegionOptions{})
	if e != nil {
		t.Fatal(e)
	}
	root := geom.ExactCurvePlaneEventOptions{}
	for _, item := range []struct {
		point [3]*big.Rat
		dist  []*big.Rat
		face  bool
	}{
		{[3]*big.Rat{rat(2, 1), rat(7, 4), rat(3, 5)}, []*big.Rat{rat(-9, 25), rat(1, 1)}, true},
		{[3]*big.Rat{rat(2, 1), rat(3, 1), rat(0, 1)}, []*big.Rat{rat(-1, 1), rat(1, 1)}, false},
		// Left cut tip (2−√2,3/2,0) is nearest to (0,3/2,0).
		// Squared distance 6−4√2 is the smaller root of D²−12D+4.
		{[3]*big.Rat{rat(0, 1), rat(3, 2), rat(0, 1)}, []*big.Rat{rat(4, 1), rat(-12, 1), rat(1, 1)}, false},
	} {
		got, e := clip.NearestPolynomialRational(ctx, item.point, root)
		if e != nil || got.FaceInterior != item.face || got.SquaredDistance == nil || (!item.face && got.CurveParameter == nil) {
			t.Fatal("exact retained minimum and owner", item.point, got, e)
		}
		p, e := got.SquaredDistance.SignPolynomial(ctx, item.dist, root)
		if e != nil || p.Sign != 0 {
			t.Fatal("minimum does not satisfy its independent distance identity", item.point, p, e)
		}
		if len(item.dist) == 3 {
			interval, e := got.SquaredDistance.Enclose(ctx, rat(1, 64), root)
			if e != nil || interval.Interval.Upper.Cmp(rat(1, 1)) >= 0 {
				t.Fatal("nearest tip uses the correct quadratic conjugate", interval, e)
			}
		}
	}
}
