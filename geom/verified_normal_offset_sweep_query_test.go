package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestVerifiedNormalOffsetSweepLocatesExactPlanarPoint(t *testing.T) {
	ctx := context.Background()
	var poles [][4]*big.Rat
	for _, x := range []int64{0, 2} {
		for _, y := range []int64{0, 3} {
			poles = append(poles, [4]*big.Rat{big.NewRat(x, 1), big.NewRat(y, 1), new(big.Rat), big.NewRat(1, 1)})
		}
	}
	p, err := NewExactHomogeneousBezierSurface(ctx, 1, 1, poles, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	lo, hi := big.NewRat(-1, 3), big.NewRat(2, 7)
	s, _, err := NewVerifiedNormalOffsetSweep(ctx, p, lo, hi,
		NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}, NormalOffsetCoverageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	q := [3]*big.Rat{big.NewRat(2, 3), big.NewRat(6, 5), big.NewRat(1, 11)}
	r, err := s.LocateRational(ctx, q, NormalOffsetSweepQueryOptions{})
	if err != nil || r.Location != NormalOffsetSweepInterior || r.VisitedCells != 1 || len(r.Unresolved) != 0 {
		t.Fatal("did not prove interior of exact planar slab", r, err)
	}
	for i, expected := range [3]*big.Rat{big.NewRat(1, 3), big.NewRat(2, 5), q[2]} {
		if r.Parameters[i].Lower.Cmp(expected) != 0 || r.Parameters[i].Upper.Cmp(expected) != 0 {
			t.Fatal("wrong rational inverse", i, r)
		}
	}
	q[2] = new(big.Rat).Add(hi, big.NewRat(1, 1<<50))
	r, err = s.LocateRational(ctx, q, NormalOffsetSweepQueryOptions{})
	if err != nil || r.Location != NormalOffsetSweepExterior || len(r.Unresolved) != 0 {
		t.Fatal("did not exclude an exterior point close to the slab", r, err)
	}
	q[2] = hi
	r, err = s.LocateRational(ctx, q, NormalOffsetSweepQueryOptions{MaxDepth: 7})
	if err != nil || r.Location != NormalOffsetSweepBoundary || len(r.Unresolved) != 0 ||
		r.Parameters[2].Lower.Cmp(hi) != 0 || r.Parameters[2].Upper.Cmp(hi) != 0 {
		t.Fatal("did not certify the original boundary layer", r, err)
	}
	short, cancel := context.WithCancel(ctx)
	cancel()
	for _, attempt := range []struct {
		ctx  context.Context
		opts NormalOffsetSweepQueryOptions
		want error
	}{
		{short, NormalOffsetSweepQueryOptions{}, context.Canceled},
		{ctx, NormalOffsetSweepQueryOptions{Patch: SurfacePatchOptions{MaxOperations: 12}}, ErrConvergence},
	} {
		r, err = s.LocateRational(attempt.ctx, q, attempt.opts)
		if !errors.Is(err, attempt.want) || r.Location != NormalOffsetSweepUnresolved || len(r.Unresolved) != 1 ||
			r.Unresolved[0].UV != p.Domain() || r.Unresolved[0].Displacement.Lower.Cmp(lo) != 0 {
			t.Fatal("lost unresolved cell after interruption", r, err)
		}
	}
}

func TestVerifiedNormalOffsetSweepLocatesCurvedPoint(t *testing.T) {
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
	// The parabolic-cylinder apex has a vertical normal. The exact point
	// has known physical parameters without constructing an approximate inverse.
	want := [3]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 3), big.NewRat(1, 20)}
	q := [3]*big.Rat{new(big.Rat), want[1], want[2]}
	bounded, err := s.LocateRational(ctx, q, NormalOffsetSweepQueryOptions{MaxDepth: 1})
	if !errors.Is(err, ErrConvergence) || bounded.Location != NormalOffsetSweepUnresolved || len(bounded.Unresolved) == 0 {
		t.Fatal("insufficient refinement must retain possible inverses", bounded, err)
	}
	covered := false
	for _, cell := range bounded.Unresolved {
		covered = covered || cell.UV.UMin <= .5 && cell.UV.UMax >= .5 && cell.UV.VMin <= 1.0/3 && cell.UV.VMax >= 1.0/3 &&
			cell.Displacement.Lower.Cmp(want[2]) <= 0 && cell.Displacement.Upper.Cmp(want[2]) >= 0
	}
	if !covered {
		t.Fatal("short work lost the exact inverse station", bounded)
	}
	r, err := s.LocateRational(ctx, q, NormalOffsetSweepQueryOptions{})
	if err != nil || r.Location != NormalOffsetSweepInterior || len(r.Unresolved) != 0 {
		t.Fatal("did not prove a point inside the curved sweep", r, err)
	}
	for i, w := range want {
		if r.Parameters[i].Lower.Cmp(w) > 0 || r.Parameters[i].Upper.Cmp(w) < 0 {
			t.Fatal("curved inverse enclosure lost known parameter", i, r)
		}
	}
	q[2] = big.NewRat(3, 8)
	r, err = s.LocateRational(ctx, q, NormalOffsetSweepQueryOptions{})
	if err != nil || r.Location != NormalOffsetSweepExterior {
		t.Fatal("did not exclude point well above curved sweep", r, err)
	}
	// At u=3/7 the unshifted point on z=x² is rational, but the
	// source unit normal used to certify its neighbourhood is not.
	want = [3]*big.Rat{big.NewRat(3, 7), big.NewRat(8, 13), new(big.Rat)}
	q = [3]*big.Rat{big.NewRat(-1, 7), want[1], big.NewRat(1, 49)}
	r, err = s.LocateRational(ctx, q, NormalOffsetSweepQueryOptions{})
	if err != nil || r.Location != NormalOffsetSweepInterior {
		t.Fatal("did not certify point with a nonvertical source normal", r, err)
	}
	for i, w := range want {
		if r.Parameters[i].Lower.Cmp(w) > 0 || r.Parameters[i].Upper.Cmp(w) < 0 {
			t.Fatal("non-apex inverse enclosure lost known parameter", i, r)
		}
	}
}
