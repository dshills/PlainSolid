package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestVerifiedNormalOffsetSweepEnclosesNoncircularBoundaryArea(t *testing.T) {
	ctx := context.Background()
	sweeps, _ := placedSweepDistanceCylinder(t)
	sweep := sweeps[0]
	q := big.NewRat
	if r, err := sweep.BoundaryAreaRational(ctx, NormalOffsetSweepAreaOptions{}); !errors.Is(err, ErrDomain) ||
		len(r.Faces) != 0 {
		t.Fatal("fixture must exercise the general directed query", r, err)
	}
	o := NormalOffsetSweepAreaEnclosureOptions{NormalOffsetSweepAreaOptions: NormalOffsetSweepAreaOptions{AbsoluteError: q(1, 1000)}}
	r, err := sweep.EncloseBoundaryAreaRational(ctx, o)
	if err != nil || !r.Complete || len(r.Faces) != 6 ||
		new(big.Rat).Sub(r.Bounds.Upper, r.Bounds.Lower).Cmp(o.AbsoluteError) > 0 {
		t.Fatal("expected a full directed six-face cover of the original parabolic cylinder", r, err)
	}
	// The local source has unit ruling length and physical profile
	// z=y²/64 for -1<=y<=1. The proper rational placement preserves
	// the area. An independent series encloses its base profile
	// length L and its total signed curvature theta=2 atan(1/32).
	length, theta := referenceParabolicSweepLengthAndTurn()
	lower, upper := q(-1, 11), q(2, 13)
	thickness := new(big.Rat).Sub(upper, lower)
	moment := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(upper, upper), new(big.Rat).Mul(lower, lower)), q(2, 1))
	end := referenceSweepArcCombination(length, theta, thickness, new(big.Rat).Neg(moment))
	radial := exactInterval{thickness, thickness}
	cap0 := referenceSweepArcCombination(length, theta, q(1, 1), new(big.Rat).Neg(lower))
	cap1 := referenceSweepArcCombination(length, theta, q(1, 1), new(big.Rat).Neg(upper))
	expected := [6]exactInterval{end, end, radial, radial, cap0, cap1}
	sum := exactInterval{new(big.Rat), new(big.Rat)}
	for i, face := range r.Faces {
		want := expected[i]
		if face.Side != NormalOffsetSweepSide(i) ||
			face.Bounds.Lower.Cmp(want.lo) > 0 || face.Bounds.Upper.Cmp(want.hi) < 0 {
			t.Fatal("directed face misses the independently integrated physical area", i, face, want)
		}
		sum.lo.Add(sum.lo, want.lo)
		sum.hi.Add(sum.hi, want.hi)
		t.Logf("original side %d area in [%s,%s]", face.Side, face.Bounds.Lower.FloatString(9), face.Bounds.Upper.FloatString(9))
	}
	if r.Bounds.Lower.Cmp(sum.lo) > 0 || r.Bounds.Upper.Cmp(sum.hi) < 0 {
		t.Fatal("directed total misses the independent physical surface area", r.Bounds, sum)
	}
	t.Logf("noncircular translated and rotated sweep area [%s,%s], %d cells, %d operations",
		r.Bounds.Lower.FloatString(12), r.Bounds.Upper.FloatString(12), r.VisitedCells, r.ExactOperations)

	coarse := o
	coarse.MaxCells = 6
	coarseResult, err := sweep.EncloseBoundaryAreaRational(ctx, coarse)
	if !errors.Is(err, ErrConvergence) || coarseResult.Complete || len(coarseResult.Faces) != 6 ||
		coarseResult.Bounds.Lower == nil || coarseResult.Bounds.Upper == nil ||
		coarseResult.Bounds.Lower.Cmp(r.Bounds.Lower) > 0 || coarseResult.Bounds.Upper.Cmp(r.Bounds.Upper) < 0 {
		t.Fatal("full root cover should survive insufficient refinement with a coarser total", coarseResult, err)
	}
	coarse.MaxCells--
	partial, err := sweep.EncloseBoundaryAreaRational(ctx, coarse)
	if !errors.Is(err, ErrConvergence) || partial.Complete || len(partial.Faces) != 5 ||
		partial.Bounds.Lower != nil || partial.Bounds.Upper != nil {
		t.Fatal("uncovered faces cannot contribute to a global enclosure", partial, err)
	}
	shortArithmetic := o
	shortArithmetic.Patch.MaxOperations = coarseResult.ExactOperations
	beforeSplit, err := sweep.EncloseBoundaryAreaRational(ctx, shortArithmetic)
	if !errors.Is(err, ErrConvergence) || len(beforeSplit.Faces) != 6 || beforeSplit.Bounds.Lower == nil {
		t.Fatal("expected the full root cover under the same rounding budget", beforeSplit, err)
	}
	shortArithmetic.Patch.MaxOperations += 100
	partial, err = sweep.EncloseBoundaryAreaRational(ctx, shortArithmetic)
	if !errors.Is(err, ErrConvergence) || partial.Complete || len(partial.Faces) != 6 ||
		partial.Bounds.Lower.Cmp(beforeSplit.Bounds.Lower) != 0 || partial.Bounds.Upper.Cmp(beforeSplit.Bounds.Upper) != 0 {
		t.Fatal("aborted split must preserve the previous full cover", partial, beforeSplit, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	partial, err = sweep.EncloseBoundaryAreaRational(cancelled, o)
	if !errors.Is(err, context.Canceled) || partial.Complete || len(partial.Faces) != 0 || partial.Bounds.Lower != nil {
		t.Fatal("cancelled request published an area", partial, err)
	}
}

func TestVerifiedNormalOffsetSweepEnclosesDoublyCurvedBoundaryArea(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	a, a2 := q(1, 32), q(1, 1024)
	linear := [3]*big.Rat{q(-1, 32), q(0, 1), a}
	square := [3]*big.Rat{a2, q(-1, 1024), a2}
	var controls [][4]*big.Rat
	for i, x := range linear {
		for j, y := range linear {
			r2 := new(big.Rat).Add(square[i], square[j])
			controls = append(controls, [4]*big.Rat{new(big.Rat).Add(x, x), new(big.Rat).Add(y, y),
				new(big.Rat).Sub(q(1, 1), r2), new(big.Rat).Add(q(1, 1), r2)})
		}
	}
	patch, err := NewExactHomogeneousBezierSurface(ctx, 2, 2, controls, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	sweep, proof, err := NewVerifiedNormalOffsetSweep(ctx, patch, q(-1, 11), q(2, 13),
		NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}, NormalOffsetCoverageOptions{})
	if err != nil || !proof.Complete {
		t.Fatal("authored rational spherical sector must verify", proof, err)
	}
	tol := q(1, 2000)
	options := NormalOffsetSweepAreaEnclosureOptions{NormalOffsetSweepAreaOptions: NormalOffsetSweepAreaOptions{AbsoluteError: tol}}
	r, err := sweep.EncloseBoundaryAreaRational(ctx, options)
	if err != nil || !r.Complete || len(r.Faces) != 6 || new(big.Rat).Sub(r.Bounds.Upper, r.Bounds.Lower).Cmp(tol) > 0 {
		t.Fatal("doubly curved original area was not enclosed to the requested width", r, err)
	}
	// Unit stereographic area has density 4/(1+x²+y²)², and along
	// an original side the density is 2/(1+a²+y²). Use independent
	// alternating polynomial sums, not the shape's offset derivatives.
	area, edge := referenceStereographicAreaAndEdge(a, a2)
	lo, hi := q(10, 11), q(15, 13)
	shiftFactor := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(hi, hi), new(big.Rat).Mul(lo, lo)), q(2, 1))
	total := exactInterval{new(big.Rat), new(big.Rat)}
	for i, face := range r.Faces {
		factor, base := shiftFactor, edge
		if i >= int(NormalOffsetSweepLower) {
			base = area
			radius := lo
			if i == int(NormalOffsetSweepUpper) {
				radius = hi
			}
			factor = new(big.Rat).Mul(radius, radius)
		}
		wantLo, wantHi := new(big.Rat).Mul(base.lo, factor), new(big.Rat).Mul(base.hi, factor)
		if face.Side != NormalOffsetSweepSide(i) || face.Bounds.Lower.Cmp(wantLo) > 0 || face.Bounds.Upper.Cmp(wantHi) < 0 {
			t.Fatal("weighted spherical face missed the independent integral", i, face, wantLo, wantHi)
		}
		total.lo.Add(total.lo, wantLo)
		total.hi.Add(total.hi, wantHi)
	}
	if r.Bounds.Lower.Cmp(total.lo) > 0 || r.Bounds.Upper.Cmp(total.hi) < 0 {
		t.Fatal("weighted sphere total misses the independently summed enclosure", r.Bounds, total)
	}
	t.Logf("weighted doubly curved sphere area [%s,%s], %d cells, %d operations",
		r.Bounds.Lower.FloatString(12), r.Bounds.Upper.FloatString(12), r.VisitedCells, r.ExactOperations)
}

func referenceStereographicAreaAndEdge(a, a2 *big.Rat) (area, edge exactInterval) {
	q := big.NewRat
	beta := new(big.Rat).Add(q(1, 1), a2)
	betaPower := new(big.Rat).Set(beta)
	power := new(big.Rat).Set(a)
	moments := make([]*big.Rat, 6)
	for i := range moments {
		moments[i] = new(big.Rat).Mul(power, q(2, int64(2*i+1)))
		power.Mul(power, a2)
	}
	areaSum, edgeSum := new(big.Rat), new(big.Rat)
	for k, moment := range moments {
		areaTerm := new(big.Rat)
		for i := 0; i <= k; i++ {
			m := new(big.Rat).Mul(moments[i], moments[k-i])
			m.Mul(m, new(big.Rat).SetInt(new(big.Int).Binomial(int64(k), int64(i))))
			areaTerm.Add(areaTerm, m)
		}
		areaTerm.Mul(areaTerm, q(int64(4*(k+1)), 1))
		edgeTerm := new(big.Rat).Quo(new(big.Rat).Mul(q(2, 1), moment), betaPower)
		if k%2 == 0 {
			areaSum.Add(areaSum, areaTerm)
			edgeSum.Add(edgeSum, edgeTerm)
			area.hi, edge.hi = new(big.Rat).Set(areaSum), new(big.Rat).Set(edgeSum)
		} else {
			areaSum.Sub(areaSum, areaTerm)
			edgeSum.Sub(edgeSum, edgeTerm)
			area.lo, edge.lo = new(big.Rat).Set(areaSum), new(big.Rat).Set(edgeSum)
		}
		betaPower.Mul(betaPower, beta)
	}
	return area, edge
}

func referenceParabolicSweepLengthAndTurn() (length, theta exactInterval) {
	q := big.NewRat
	sumLength, sumTurn := new(big.Rat), new(big.Rat)
	binomial, power := q(1, 1), q(1, 1)
	for n := int64(0); n < 8; n++ {
		factor := q(2, 2*n+1)
		sumLength.Add(sumLength, new(big.Rat).Mul(factor, new(big.Rat).Mul(binomial, new(big.Rat).Abs(power))))
		sumTurn.Add(sumTurn, new(big.Rat).Mul(factor, new(big.Rat).Mul(power, q(1, 32))))
		binomial.Mul(binomial, q(1-2*n, 2*(n+1)))
		power.Mul(power, q(-1, 1024))
	}
	nextLength := new(big.Rat).Mul(q(2, 17), new(big.Rat).Mul(binomial, power))
	nextTurn := new(big.Rat).Mul(q(2, 17), new(big.Rat).Mul(power, q(1, 32)))
	// The next length term is negative. The next arctangent term is positive.
	return exactInterval{new(big.Rat).Add(sumLength, nextLength), sumLength},
		exactInterval{sumTurn, new(big.Rat).Add(sumTurn, nextTurn)}
}

func referenceSweepArcCombination(length, turn exactInterval, scale, correction *big.Rat) exactInterval {
	thetaLo, thetaHi := turn.lo, turn.hi
	if correction.Sign() < 0 {
		thetaLo, thetaHi = thetaHi, thetaLo
	}
	return exactInterval{
		new(big.Rat).Add(new(big.Rat).Mul(scale, length.lo), new(big.Rat).Mul(correction, thetaLo)),
		new(big.Rat).Add(new(big.Rat).Mul(scale, length.hi), new(big.Rat).Mul(correction, thetaHi)),
	}
}
