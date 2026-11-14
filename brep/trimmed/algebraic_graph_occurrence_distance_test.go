package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestAlgebraicGraphOccurrenceWorldDistance(t *testing.T) {
	ctx := context.Background()
	source := graphMaterialBooleanFixture(t, false, false)
	identity := [3][4]*big.Rat{}
	for i := range identity {
		for j := range identity[i] {
			identity[i][j] = new(big.Rat)
			if i == j {
				identity[i][j].SetInt64(1)
			}
		}
	}
	original, e := NewAlgebraicGraphOccurrence(ctx, source, identity, [2]int{0, 1}, AlgebraicGraphMaterialOptions{})
	if e != nil {
		t.Fatal(e)
	}
	point := queryPoint(big.NewRat(5, 16), big.NewRat(1, 2), big.NewRat(-1, 1))
	options := AlgebraicTubeLensDistanceOptions{SquaredGap: big.NewRat(1, 1000), MaxCells: 8}
	reference, e := original.query.BoundBoundaryDistance(ctx, point, options)
	if e != nil && !errors.Is(e, geom.ErrConvergence) {
		t.Fatal(e)
	}
	same, e := original.BoundBoundaryDistance(ctx, point, options)
	if e != nil || !same.HasBounds || same.Squared.Lower.Cmp(reference.Squared.Lower) != 0 || same.Squared.Upper.Cmp(reference.Squared.Upper) != 0 || same.Face != reference.Face || same.RimEdge != reference.RimEdge {
		t.Fatal("identity world metric changed source bounds", same, reference, e)
	}
	check := func(r AlgebraicGraphOccurrenceDistance, want *big.Rat) {
		t.Helper()
		if !r.HasBounds || r.Squared.Lower.Cmp(want) > 0 || r.Squared.Upper.Cmp(want) < 0 {
			t.Fatal("independent exact world distance", r.Squared, want)
		}
		if r.Face < 0 && r.RimEdge < 0 {
			t.Fatal("missing original witness owner")
		}
	}
	check(same, big.NewRat(1, 1))
	// Genuine rational shear and reflection: cap image is the plane
	// Z=(X−1/7)/5−1/11. All retained material is below it because Z includes−2z.
	pose := [3][4]*big.Rat{}
	for i := range pose {
		for j := range pose[i] {
			pose[i][j] = new(big.Rat)
		}
	}
	pose[0][0] = big.NewRat(1, 1)
	pose[0][3] = big.NewRat(1, 7)
	pose[1][0] = big.NewRat(1, 3)
	pose[1][1] = big.NewRat(1, 1)
	pose[1][3] = big.NewRat(2, 7)
	pose[2][0] = big.NewRat(1, 5)
	pose[2][2] = big.NewRat(-2, 1)
	pose[2][3] = big.NewRat(-1, 11)
	occurrence, e := NewAlgebraicGraphOccurrence(ctx, source, pose, [2]int{0, 1}, AlgebraicGraphMaterialOptions{})
	if e != nil {
		t.Fatal(e)
	}
	image := func(p [3]*big.Rat) (r [3]*big.Rat) {
		for i := range r {
			r[i] = new(big.Rat).Set(pose[i][3])
			for j := 0; j < 3; j++ {
				r[i].Add(r[i], new(big.Rat).Mul(pose[i][j], p[j]))
			}
		}
		return r
	}
	foot := image(queryPoint(big.NewRat(5, 16), big.NewRat(1, 2), new(big.Rat)))
	probe := foot
	probe[0] = new(big.Rat).Sub(foot[0], big.NewRat(1, 50))
	probe[2] = new(big.Rat).Add(foot[2], big.NewRat(1, 10))
	// Probe = foot + (−1/5,0,1)/10, hence exact global squared distance13/1250.
	// Foot lies strictly inside the retained planar lobe; the material halfspace
	// proves no roof/rim point can be nearer. Source Euclidean distance differs.
	r, e := occurrence.BoundBoundaryDistance(ctx, probe, AlgebraicTubeLensDistanceOptions{SquaredGap: big.NewRat(1, 1000000), MaxCells: 16})
	if e != nil {
		t.Fatal(e)
	}
	check(r, big.NewRat(13, 1250))
	if r.Face >= 0 {
		if _, e := source.OriginalFace(r.Face); e != nil {
			t.Fatal("unknown original face", e)
		}
	} else if _, _, e := source.SectionSegment(r.RimEdge); e != nil {
		t.Fatal("unknown original rim", e)
	}
	// Exact interval image of the SOURCE witness must agree with the returned
	// WORLD witness. This checks full mixed-coordinate metric, including shear.
	for i := 0; i < 3; i++ {
		lo, hi := new(big.Rat).Set(pose[i][3]), new(big.Rat).Set(pose[i][3])
		for j := 0; j < 3; j++ {
			x := new(big.Rat).Mul(pose[i][j], r.SourceWitness[j].Lower)
			y := new(big.Rat).Mul(pose[i][j], r.SourceWitness[j].Upper)
			if x.Cmp(y) > 0 {
				x, y = y, x
			}
			lo.Add(lo, x)
			hi.Add(hi, y)
		}
		if lo.Cmp(r.WorldWitness[i].Lower) != 0 || hi.Cmp(r.WorldWitness[i].Upper) != 0 {
			t.Fatal("world/source witness correspondence", i)
		}
	}
	limited, e := occurrence.BoundBoundaryDistance(ctx, probe, AlgebraicTubeLensDistanceOptions{SquaredGap: big.NewRat(1, 1000000000), MaxCells: 1})
	if e != nil || !limited.HasBounds || limited.Converged || limited.VisitedCells > 1 {
		t.Fatal("explicit unresolved cell limit", limited, e)
	}
	check(limited, big.NewRat(13, 1250))
	if r.Squared.Lower.Cmp(limited.Squared.Lower) <= 0 || r.Squared.Upper.Cmp(limited.Squared.Upper) > 0 {
		t.Fatal("subdivision did not tighten world enclosure")
	}
	onBoundary, e := occurrence.BoundBoundaryDistance(ctx, foot, options)
	if e != nil || !onBoundary.HasBounds || !onBoundary.Converged || onBoundary.Squared.Lower.Sign() != 0 || onBoundary.Squared.Upper.Sign() != 0 || onBoundary.Face < 0 {
		t.Fatal("exact world boundary zero", onBoundary, e)
	}
	bad, e := occurrence.BoundBoundaryDistance(ctx, probe, AlgebraicTubeLensDistanceOptions{Patch: geom.SurfacePatchOptions{MaxOperations: 1}})
	if !errors.Is(e, geom.ErrConvergence) || bad.HasBounds || bad.Squared.Upper != nil || bad.Face != -1 || bad.RimEdge != -1 {
		t.Fatal("atomic arithmetic budget", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	bad, e = occurrence.BoundBoundaryDistance(cancelled, probe, options)
	if !errors.Is(e, context.Canceled) || bad.HasBounds || bad.Squared.Upper != nil {
		t.Fatal("atomic cancel", e)
	}
	t.Logf("world shear distance[%s,%s], exact13/1250; %d cells/%d ops, converged%v", r.Squared.Lower, r.Squared.Upper, r.VisitedCells, r.ExactOperations, r.Converged)
}
