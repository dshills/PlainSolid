package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestVerifiedNormalOffsetSweepFaceBoundsOutwardPlanes(t *testing.T) {
	ctx := context.Background()
	var net [][4]*big.Rat
	for u := range 2 {
		for v := range 2 {
			net = append(net, [4]*big.Rat{big.NewRat(int64(u), 1), big.NewRat(int64(v), 1), new(big.Rat), big.NewRat(1, 1)})
		}
	}
	p, err := NewExactHomogeneousBezierSurface(ctx, 1, 1, net, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	lo, hi := big.NewRat(-1, 7), big.NewRat(2, 7)
	s, proof, err := NewVerifiedNormalOffsetSweep(ctx, p, lo, hi, NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}},
		NormalOffsetCoverageOptions{})
	if err != nil || !proof.Complete {
		t.Fatal("unproved source sweep", err)
	}
	for side := NormalOffsetSweepUMin; side <= NormalOffsetSweepUpper; side++ {
		f, err := s.Boundary(side)
		if err != nil {
			t.Fatal(err)
		}
		a, b := lo, hi
		if side == NormalOffsetSweepLower {
			b = lo
		}
		if side == NormalOffsetSweepUpper {
			a = hi
		}
		box, err := f.NormalBoundsRational(ctx, p.Domain(), a, b, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(side, err)
		}
		for axis := range box.Lower {
			want := int64(0)
			if axis == int(side)/2 {
				want = int64(2*(side%2)) - 1
			}
			if box.Lower[axis].Cmp(big.NewRat(want, 1)) != 0 || box.Upper[axis].Cmp(big.NewRat(want, 1)) != 0 {
				t.Fatal("outward exact plane normal changed", side, axis, box.Lower, box.Upper)
			}
		}
		miss, err := f.NormalBoundsRational(ctx, p.Domain(), a, b, SurfacePatchOptions{MaxOperations: box.ExactOperations - 1})
		if !errors.Is(err, ErrConvergence) || miss.Lower[0] != nil || miss.ExactOperations < 1 {
			t.Fatal("normal proof budget published partial coefficients", side, err)
		}
	}
	wall, _ := s.Boundary(NormalOffsetSweepUMin)
	if _, err := wall.NormalBoundsRational(ctx, ParameterRectangle{UMin: .125, UMax: .25, VMax: 1}, lo, hi,
		SurfacePatchOptions{}); !errors.Is(err, ErrDomain) {
		t.Fatal("normal box accepted a collar missing the source wall", err)
	}
	top, _ := s.Boundary(NormalOffsetSweepUpper)
	if _, err := top.NormalBoundsRational(ctx, p.Domain(), lo, hi, SurfacePatchOptions{}); !errors.Is(err, ErrDomain) {
		t.Fatal("normal box accepted an interval off its original layer", err)
	}
}

func TestVerifiedNormalOffsetSweepWallBoundsIncludeDisplacementDerivative(t *testing.T) {
	ctx := context.Background()
	var net [][4]*big.Rat
	for u := int64(0); u < 3; u++ {
		for v := int64(0); v < 3; v++ {
			z := big.NewRat(2*u*(u-1)+2*v*(v-1)+u*v, 64)
			net = append(net, [4]*big.Rat{big.NewRat(u, 2), big.NewRat(v, 2), z, big.NewRat(1, 1)})
		}
	}
	p, err := NewExactHomogeneousBezierSurface(ctx, 2, 2, net, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	s, proof, err := NewVerifiedNormalOffsetSweep(ctx, p, big.NewRat(-1, 10), big.NewRat(1, 10),
		NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}, NormalOffsetCoverageOptions{})
	if err != nil || !proof.Complete {
		t.Fatal("unproved curved source", err)
	}
	face, _ := s.Boundary(NormalOffsetSweepUMin)
	const width = 1. / 65536
	rect := ParameterRectangle{UMax: width, VMin: .5, VMax: .5 + width}
	var boxes [2]RationalSurfaceOffsetPoint
	for i, distance := range [2]*big.Rat{new(big.Rat), big.NewRat(1, 10)} {
		boxes[i], err = face.NormalBoundsRational(ctx, rect, distance, distance, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		v := .5 + width/2
		g := Vec3{X: -v / 16, Y: -v / 8, Z: 1}
		gPrime := Vec3{X: -1. / 16, Y: -1. / 8}
		unit := g.Scale(1 / g.Length())
		derivative := gPrime.Sub(unit.Scale(gPrime.Dot(unit))).Scale(1 / g.Length())
		d, _ := distance.Float64()
		free := (Vec3{Y: 1, Z: v / 8}).Add(derivative.Scale(d))
		actual := unit.Cross(free)
		actual = actual.Scale(1 / actual.Length())
		lo, hi := boxes[i].Bounds.Min, boxes[i].Bounds.Max
		if actual.X < lo.X || actual.X > hi.X || actual.Y < lo.Y || actual.Y > hi.Y ||
			actual.Z < lo.Z || actual.Z > hi.Z {
			t.Fatal("direct differentiating and normalizing check escaped wall enclosure", i, actual, boxes[i].Bounds)
		}
	}
	if boxes[0].Lower[1].Cmp(boxes[1].Upper[1]) <= 0 && boxes[1].Lower[1].Cmp(boxes[0].Upper[1]) <= 0 {
		t.Fatal("thin wall boxes did not resolve the actual displacement-dependent turn", boxes)
	}
}

func TestVerifiedNormalOffsetSweepLayerNormalsWithNegativeChartOrientation(t *testing.T) {
	ctx := context.Background()
	p, err := normalOffsetParabolicCylinder(t, false).Trim(ParameterRectangle{UMin: .46875, UMax: .53125, VMax: 1})
	if err != nil {
		t.Fatal(err)
	}
	// The original free derivative has flipped throughout this interval
	// beyond the earlier focal layer, which is outside the proved sweep.
	lo, hi := big.NewRat(3, 4), big.NewRat(4, 5)
	view := NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}, Reversed: [3]bool{true}}
	s, proof, err := NewVerifiedNormalOffsetSweep(ctx, p, lo, hi, view, NormalOffsetCoverageOptions{})
	if err != nil || !proof.Complete || s.Orientation() != -1 {
		t.Fatal("independent negative chart orientation was not authenticated", proof, err)
	}
	lower, _ := s.Boundary(NormalOffsetSweepLower)
	upper, _ := s.Boundary(NormalOffsetSweepUpper)
	a, err := lower.NormalBoundsRational(ctx, p.Domain(), lo, lo, SurfacePatchOptions{})
	if err != nil || a.Upper[2].Sign() >= 0 {
		t.Fatal("outward lower face must oppose the positive displacement direction", a.Bounds, err)
	}
	b, err := upper.NormalBoundsRational(ctx, p.Domain(), hi, hi, SurfacePatchOptions{})
	if err != nil || b.Lower[2].Sign() <= 0 {
		t.Fatal("outward upper face must follow the positive displacement direction", b.Bounds, err)
	}
}
