package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestSurfacePairDistanceParallelAndContact(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 2} })
	r, e := BoundSurfacePairDistance(a, b, SurfacePairDistanceOptions{})
	if e != nil || !r.Complete || !r.HasWitness || r.SquaredDistanceMin != 4 || r.SquaredDistanceMax != 4 {
		t.Fatal(r, e)
	}
	r, e = BoundSurfacePairDistance(a, a, SurfacePairDistanceOptions{})
	if e != nil || !r.Complete || r.SquaredDistanceMax != 0 {
		t.Fatal(r, e)
	}
}
func TestSurfacePairDistanceRationalCurved(t *testing.T) {
	a, _ := NewSurfacePatch(quarterCylinder(t), SurfacePatchOptions{})
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{3, 3 + u, 2 * v} })
	r, e := BoundSurfacePairDistance(a, b, SurfacePairDistanceOptions{SquaredTolerance: .05, MaxOperations: 8000000})
	if e != nil || !r.Complete || r.SquaredDistanceMin > 5.02943725152286 || r.SquaredDistanceMax < 5.02943725152285 {
		t.Fatal(r, e)
	}
}
func TestSurfacePairDistanceLimitsRetainBounds(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u + v} })
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u + v + .3} })
	for _, o := range []SurfacePairDistanceOptions{{MaxPairs: 1}, {MaxOperations: 1}, {MaxDepth: 1}} {
		r, e := BoundSurfacePairDistance(a, b, o)
		if !errors.Is(e, ErrConvergence) || r.Complete || r.SquaredDistanceMin > .03 || r.SquaredDistanceMax < .03 {
			t.Fatal(r, e)
		}
	}
}

func TestSurfacePairDistanceExactSubnormalObliqueGap(t *testing.T) {
	ctx := context.Background()
	e := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 700))
	patch := func(gap *big.Rat) *SurfacePatch {
		t.Helper()
		var poles [][4]*big.Rat
		for _, u := range []int64{1, 2} {
			for _, v := range []int64{0, 1} {
				x := new(big.Rat).Add(big.NewRat(u, 1), gap)
				y := new(big.Rat).Sub(big.NewRat(u, 1), gap)
				poles = append(poles, [4]*big.Rat{x, y, big.NewRat(v, 1), big.NewRat(1, 1)})
			}
		}
		p, err := NewExactHomogeneousBezierSurface(ctx, 1, 1, poles, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	a, b := patch(new(big.Rat)), patch(e)
	ba, err := a.Bounds()
	if err != nil {
		t.Fatal(err)
	}
	bb, err := b.Bounds()
	if err != nil {
		t.Fatal(err)
	}
	for axis := range 3 {
		if coordinate(ba.Max, axis) < coordinate(bb.Min, axis) || coordinate(bb.Max, axis) < coordinate(ba.Min, axis) {
			t.Fatal("oblique control requires overlapping axis projections", axis)
		}
	}
	want := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(e, e))
	tolerance := new(big.Rat).Quo(want, big.NewRat(19, 1))
	for _, pair := range [2][2]*SurfacePatch{{a, b}, {b, a}} {
		r, err := BoundSurfacePairDistanceRational(ctx, pair[0], pair[1], tolerance, SurfacePairDistanceOptions{MaxPairs: 1})
		if err != nil || !r.Complete || !r.HasWitness || r.Squared.Lower.Cmp(want) != 0 || r.Squared.Upper.Cmp(want) != 0 ||
			r.SquaredDistanceMin != 0 || r.SquaredDistanceMax != math.SmallestNonzeroFloat64 {
			t.Fatal("exact oblique gap was not certified on the original pair", r, err)
		}
		for axis := range 3 {
			var delta big.Rat
			delta.Sub(r.WitnessA[axis], r.WitnessB[axis]).Abs(&delta)
			expected := new(big.Rat)
			if axis < 2 {
				expected = e
			}
			if delta.Cmp(expected) != 0 {
				t.Fatal("witness rounded away a tiny Cartesian coordinate difference", axis, delta)
			}
		}
	}
	contact, err := BoundSurfacePairDistanceRational(ctx, a, a, tolerance, SurfacePairDistanceOptions{})
	if err != nil || !contact.Complete || contact.Squared.Upper.Sign() != 0 {
		t.Fatal("coincident source should still have zero distance", contact, err)
	}
}

func TestSurfacePairDistanceExactRetainsPartialWitness(t *testing.T) {
	ctx := context.Background()
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u + v} })
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u + v + .25} })
	r, err := BoundSurfacePairDistanceRational(ctx, a, b, big.NewRat(1, 1_000_000), SurfacePairDistanceOptions{MaxPairs: 1})
	want := big.NewRat(1, 48)
	if !errors.Is(err, ErrConvergence) || r.Complete || !r.HasWitness || r.Squared.Lower.Cmp(want) > 0 || r.Squared.Upper.Cmp(want) < 0 {
		t.Fatal("short exact search must preserve bounds of the global distance", r, err)
	}
	squared := new(big.Rat)
	for axis := range 3 {
		delta := new(big.Rat).Sub(r.WitnessA[axis], r.WitnessB[axis])
		squared.Add(squared, delta.Mul(delta, delta))
	}
	if squared.Cmp(r.Squared.Upper) != 0 {
		t.Fatal("published upper bound must have the exact returned sample pair", r, squared)
	}
}
