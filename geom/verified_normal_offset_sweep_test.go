package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestVerifiedNormalOffsetSweepRetainsSixOriginalBoundaryFaces(t *testing.T) {
	ctx := context.Background()
	var controls [][4]*big.Rat
	for _, x := range []int64{0, 2} {
		for _, y := range []int64{0, 3} {
			controls = append(controls, [4]*big.Rat{big.NewRat(x, 1), big.NewRat(y, 1), new(big.Rat), big.NewRat(1, 1)})
		}
	}
	p, err := NewExactHomogeneousBezierSurface(ctx, 1, 1, controls, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	originalLow, originalHigh := big.NewRat(-1, 3), big.NewRat(1, 2)
	s, proof, err := NewVerifiedNormalOffsetSweep(ctx, p, originalLow, originalHigh,
		NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}, NormalOffsetCoverageOptions{})
	if err != nil || !proof.Complete || s.Orientation() != 1 {
		t.Fatal("unable to construct independently embedded planar swept volume", proof, err)
	}
	originalLow.SetInt64(-9)
	_, shifts := s.Domain()
	if shifts.Lower.Cmp(big.NewRat(-1, 3)) != 0 {
		t.Fatal("swept volume aliases input displacement")
	}
	pt, err := s.EvaluateRational(ctx, big.NewRat(1, 3), big.NewRat(3, 10), big.NewRat(-1, 6),
		SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
	want := [3]*big.Rat{big.NewRat(2, 3), big.NewRat(9, 10), big.NewRat(-1, 6)}
	for i, value := range want {
		if err != nil || pt.Lower[i].Cmp(value) != 0 || pt.Upper[i].Cmp(value) != 0 {
			t.Fatal("swept point lost the exact planar parameterization", i, pt, err)
		}
	}
	normals := [6]Vec3{{-1, 0, 0}, {1, 0, 0}, {0, -1, 0}, {0, 1, 0}, {0, 0, -1}, {0, 0, 1}}
	for side, n := range normals {
		f, err := s.Boundary(NormalOffsetSweepSide(side))
		if err != nil {
			t.Fatal(err)
		}
		a, b, err := f.Partials(.25, .25)
		if err != nil {
			t.Fatal(err)
		}
		normal := a.Cross(b)
		if f.ReverseWinding() {
			normal = normal.Scale(-1)
		}
		if normal.Dot(n) <= 0 || normal.Cross(n) != (Vec3{}) {
			t.Fatal("boundary must point outward", side, normal)
		}
	}
	wall, _ := s.Boundary(NormalOffsetSweepUMin)
	layer, _ := s.Boundary(NormalOffsetSweepLower)
	pair := wall.ParameterDomain()
	if pair[1].Lower.Cmp(shifts.Lower) != 0 || pair[1].Upper.Cmp(shifts.Upper) != 0 {
		t.Fatal("wall lost its exact signed displacement domain", pair)
	}
	value := big.NewRat(1, 3)
	x, err := wall.EvaluateRational(ctx, value, shifts.Lower, SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	y, err := layer.EvaluateRational(ctx, new(big.Rat), value, SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for i := range x.Lower {
		if x.Lower[i].Cmp(y.Lower[i]) != 0 || x.Upper[i].Cmp(y.Upper[i]) != 0 {
			t.Fatal("analytic boundary faces disagree at their shared original edge", x, y)
		}
	}
	if _, _, err := wall.Partials(.125, -.5); !errors.Is(err, ErrDomain) {
		t.Fatal("wall accepts displacement outside the verified volume", err)
	}
}

func TestVerifiedNormalOffsetSweepEnclosesCompleteRationalSubvolume(t *testing.T) {
	ctx := context.Background()
	p, err := normalOffsetParabolicCylinder(t, false).Trim(ParameterRectangle{.375, .625, 0, 1})
	if err != nil {
		t.Fatal(err)
	}
	s, _, err := NewVerifiedNormalOffsetSweep(ctx, p, big.NewRat(-1, 10), big.NewRat(1, 10),
		NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}, NormalOffsetCoverageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cell := ParameterRectangle{.4375, .5625, .25, .75}
	t0, t1 := big.NewRat(-1, 13), big.NewRat(1, 17)
	b, err := s.BoundsRational(ctx, cell, t0, t1, SurfacePatchOptions{})
	if err != nil || b.Lower[1].Cmp(big.NewRat(1, 4)) != 0 || b.Upper[1].Cmp(big.NewRat(3, 4)) != 0 {
		t.Fatal("sweep box lost the exact physical station interval", b, err)
	}
	// At u=1/2 the unit normal is exactly vertical and z=t. The same
	// continuous box must enclose both distinct layer endpoints.
	if b.Lower[2].Cmp(t0) > 0 || b.Upper[2].Cmp(t1) < 0 {
		t.Fatal("directed spatial bound lost a verified layer", b)
	}
	if _, err := s.BoundsRational(ctx, cell, t0, big.NewRat(1, 2), SurfacePatchOptions{}); !errors.Is(err, ErrDomain) {
		t.Fatal("bounds accepted an unproved layer", err)
	}
}
