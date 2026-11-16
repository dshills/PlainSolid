package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestVerifiedNormalOffsetSweepPairRetainsOriginalCurvedBoundary(t *testing.T) {
	ctx := context.Background()
	lower, upper := big.NewRat(-1, 11), big.NewRat(2, 13)
	a := sweepPairParaboloid(t, false, false, 0, lower, upper)
	for _, nontrivial := range []bool{false, true} {
		name := "linear-station"
		if nontrivial {
			name = "reversed-projective-station"
		}
		t.Run(name, func(t *testing.T) {
			b := sweepPairParaboloid(t, true, nontrivial, 0, lower, upper)
			join := VerifiedNormalOffsetSweepJoin{
				Edges: [2]SurfaceEdge{SurfaceEdgeUMax, SurfaceEdgeUMin}, Axis: 0, AInteriorSign: -1,
			}
			if nontrivial {
				join.Edges[1], join.ReverseB, join.BStationRatio = SurfaceEdgeVMin, true, big.NewRat(3, 2)
			}
			pair, proof, err := NewVerifiedNormalOffsetSweepPair(ctx, a, b, join, NormalOffsetCoverageOptions{})
			if err != nil || pair == nil || !proof.Complete || !proof.Seam.Complete || !proof.Seam.SharedOriginalEdge ||
				proof.Coordinate.Cmp(big.NewRat(5, 8)) != 0 || proof.ExactOperations <= proof.Seam.ExactOperations {
				t.Fatal("complete curve, swept wall and halfspaces are required", pair, proof, err)
			}
			t.Logf("full proof %d exact operations including seam %d", proof.ExactOperations, proof.Seam.ExactOperations)
			sweeps := [2]*VerifiedNormalOffsetSweep{a, b}
			faces := pair.Boundary()
			if len(faces) != 10 {
				t.Fatal("exactly ten original outer faces are required", len(faces))
			}
			var index int
			for owner, sweep := range sweeps {
				for side := NormalOffsetSweepUMin; side <= NormalOffsetSweepUpper; side++ {
					if side == NormalOffsetSweepSide(join.Edges[owner]) {
						continue
					}
					original, _ := sweep.Boundary(side)
					face := faces[index]
					index++
					if face.Owner != owner || face.Side != side || face.Face != original {
						t.Fatal("missing original boundary identity or winding", owner, side, face)
					}
				}
			}

			wallA, _ := a.Boundary(NormalOffsetSweepUMax)
			wallB, _ := b.Boundary(NormalOffsetSweepSide(join.Edges[1]))
			if wallA.ReverseWinding() == (wallB.ReverseWinding() != join.ReverseB) {
				t.Fatal("the common wall must have opposite induced outward windings")
			}
			for _, station := range []*big.Rat{new(big.Rat), big.NewRat(1, 5), big.NewRat(1, 2), big.NewRat(4, 5), big.NewRat(1, 1)} {
				// A's original free parameter is y. For the projective
				// B chart its raw parameter is 2+2*(1-s)/(1+s/2).
				y := new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(2, 1), station), big.NewRat(1, 1))
				bRaw := y
				if nontrivial {
					s := new(big.Rat).Sub(big.NewRat(1, 1), station)
					den := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Mul(big.NewRat(1, 2), station))
					bRaw = new(big.Rat).Add(big.NewRat(2, 1), s.Mul(s.Quo(s, den), big.NewRat(2, 1)))
				}
				for _, d := range []*big.Rat{lower, big.NewRat(-1, 17), new(big.Rat), big.NewRat(1, 19), upper} {
					x, err := wallA.EvaluateRational(ctx, y, d, SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
					if err != nil {
						t.Fatal(err)
					}
					w, err := wallB.EvaluateRational(ctx, bRaw, d, SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
					if err != nil {
						t.Fatal(err)
					}
					for axis := range 3 {
						if x.Lower[axis].Cmp(w.Lower[axis]) != 0 || x.Upper[axis].Cmp(w.Upper[axis]) != 0 {
							t.Fatal("matched exact raw owner stations disagreed", station, d, axis, x, w)
						}
					}
					if x.Lower[0].Cmp(big.NewRat(5, 8)) != 0 || x.Upper[0].Cmp(big.NewRat(5, 8)) != 0 {
						t.Fatal("wall departed from its original separator", station, d, x)
					}
					free, _ := y.Float64()
					displacement, _ := d.Float64()
					length := math.Hypot(1, free/32)
					expected := [3]float64{5.0 / 8, free - displacement*free/(32*length), free*free/64 + displacement/length}
					for axis, want := range expected {
						got, _ := x.Lower[axis].Float64()
						if math.Abs(got-want) > 4e-16 {
							t.Fatal("wall point disagrees with independent physical parabola", station, d, axis, got, want)
						}
					}
				}
			}
			// Compare interior points at distinct displacement values.
			// Both controls use actual original parameters, including a
			// different fixed parameter axis and raw interval on B.
			ua, va, ub, vb := big.NewRat(5, 2), new(big.Rat), big.NewRat(5, 2), new(big.Rat)
			if nontrivial {
				ub, vb = big.NewRat(14, 5), big.NewRat(-1, 2)
			}
			pa, err := a.EvaluateRational(ctx, ua, va, upper, SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			pb, err := b.EvaluateRational(ctx, ub, vb, lower, SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if pa.Upper[0].Cmp(proof.Coordinate) >= 0 || pb.Lower[0].Cmp(proof.Coordinate) <= 0 {
				t.Fatal("unequal signed layers must stay in opposite open halfspaces", pa, pb)
			}

			// Caller mutations cannot replace authenticated station or plane
			// data, or change the original outer faces.
			proof.Coordinate.SetInt64(3)
			faces[0] = VerifiedNormalOffsetSweepBoundaryFace{}
			if join.BStationRatio != nil {
				join.BStationRatio.SetInt64(5)
				view, _ := pair.Join()
				view.BStationRatio.SetInt64(7)
			}
			kept, c := pair.Join()
			if kept.Edges != join.Edges || kept.ReverseB != join.ReverseB || kept.Axis != join.Axis ||
				kept.AInteriorSign != join.AInteriorSign || c.Cmp(big.NewRat(5, 8)) != 0 ||
				(nontrivial && kept.BStationRatio.Cmp(big.NewRat(3, 2)) != 0) || pair.Boundary()[0].Face.sweep != a {
				t.Fatal("joined original data changed after caller mutations", kept, c)
			}
		})
	}
}

func TestVerifiedNormalOffsetSweepPairRefusesUnprovedContact(t *testing.T) {
	ctx := context.Background()
	lower, upper := big.NewRat(-1, 11), big.NewRat(2, 13)
	left := sweepPairParaboloid(t, false, false, 0, lower, upper)
	right := sweepPairParaboloid(t, true, false, 0, lower, upper)
	fractional := sweepPairParaboloid(t, true, true, 0, lower, upper)
	join := VerifiedNormalOffsetSweepJoin{
		Edges: [2]SurfaceEdge{SurfaceEdgeUMax, SurfaceEdgeUMin}, Axis: 0, AInteriorSign: -1,
	}
	fractionalJoin := join
	fractionalJoin.Edges[1], fractionalJoin.ReverseB = SurfaceEdgeVMin, true
	overlapJoin := join
	overlapJoin.Edges[1] = SurfaceEdgeUMax
	for _, tc := range []struct {
		name         string
		a, b         *VerifiedNormalOffsetSweep
		join         VerifiedNormalOffsetSweepJoin
		shared, seam bool
	}{
		{"wrong interior stations with same wall ends", left, fractional, fractionalJoin, false, false},
		{"same-side sweep overlap", left, sweepPairParaboloid(t, false, false, 0, lower, upper), overlapJoin, true, true},
		{"matching edge with creased source normal", left, sweepPairParaboloid(t, true, false, 1, lower, upper), join, true, false},
		{"swept wall leaves the chosen plane",
			sweepPairParaboloid(t, false, false, 1, lower, upper), sweepPairParaboloid(t, true, false, 1, lower, upper), join, true, true},
		{"different signed intervals", left, sweepPairParaboloid(t, true, false, 0, lower, big.NewRat(1, 7)), join, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pair, proof, err := NewVerifiedNormalOffsetSweepPair(ctx, tc.a, tc.b, tc.join, NormalOffsetCoverageOptions{})
			if !errors.Is(err, ErrDomain) || pair != nil || proof.Complete || proof.Coordinate != nil ||
				proof.Seam.SharedOriginalEdge != tc.shared || proof.Seam.Complete != tc.seam {
				t.Fatal("invalid pair published a union or wrong proof state", pair, proof, err)
			}
		})
	}
	_, full, err := NewVerifiedNormalOffsetSweepPair(ctx, left, right, join, NormalOffsetCoverageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	short := NormalOffsetCoverageOptions{Patch: SurfacePatchOptions{MaxOperations: full.ExactOperations - 1}}
	pair, proof, err := NewVerifiedNormalOffsetSweepPair(ctx, left, right, join, short)
	if !errors.Is(err, ErrConvergence) || pair != nil || proof.Complete || proof.Coordinate != nil || !proof.Seam.Complete {
		t.Fatal("short shared exact work limit published a union", pair, proof, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	pair, proof, err = NewVerifiedNormalOffsetSweepPair(cancelled, left, right, join, NormalOffsetCoverageOptions{})
	if !errors.Is(err, context.Canceled) || pair != nil || proof.Complete || proof.Coordinate != nil || proof.ExactOperations != 0 {
		t.Fatal("cancelled proof published a union", pair, proof, err)
	}
}

// Each chart is independently authored with exact homogeneous controls for
// z=(x-5/8)^2/32+y^2/64+tilt*(x-5/8)/64. B can retain the original
// (y,x) chart order with reversed y and the 3/2 Möbius law on that axis.
// The raw ranges remain different from one another in that case.
func sweepPairParaboloid(t *testing.T, right, nontrivial bool, tilt int64, lower, upper *big.Rat) *VerifiedNormalOffsetSweep {
	t.Helper()
	ctx := context.Background()
	recipe := ExactBezierSurfaceRecipe{
		Version: 1, DegreeU: 2, DegreeV: 2, Domain: ParameterRectangle{2, 3, -1, 1},
	}
	projection := NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}
	if nontrivial {
		recipe.Domain = ParameterRectangle{2, 4, -1, 0}
		projection = NormalOffsetSweepProjection{Axes: [3]int{1, 0, 2}, Reversed: [3]bool{true}}
	}
	for i := int64(0); i < 3; i++ {
		for j := int64(0); j < 3; j++ {
			xi, yi, weight := i, j, big.NewRat(1, 1)
			if nontrivial {
				xi, yi = j, 2-i
				weight = []*big.Rat{big.NewRat(1, 1), big.NewRat(3, 2), big.NewRat(9, 4)}[i]
			}
			relativeX2 := xi - 2
			squareX := [3]int64{1, 0, 0}[xi]
			if right {
				relativeX2, squareX = xi, [3]int64{0, 0, 1}[xi]
			}
			x, y := big.NewRat(5+4*relativeX2, 8), big.NewRat(yi-1, 1)
			z := big.NewRat(4*squareX+2*[3]int64{1, -1, 1}[yi]+tilt*relativeX2, 128)
			recipe.Controls = append(recipe.Controls, [4]string{
				x.Mul(x, weight).RatString(), y.Mul(y, weight).RatString(), z.Mul(z, weight).RatString(), weight.RatString(),
			})
		}
	}
	source, err := ImportExactBezierSurfaceRecipe(ctx, recipe, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	sweep, proof, err := NewVerifiedNormalOffsetSweep(ctx, source, lower, upper, projection, NormalOffsetCoverageOptions{})
	if err != nil || sweep == nil || !proof.Complete {
		t.Fatal("independent source chart must embed the full signed sweep", right, nontrivial, tilt, proof, err)
	}
	return sweep
}
