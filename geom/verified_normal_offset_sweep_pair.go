package geom

import (
	"context"
	"math/big"
)

// VerifiedNormalOffsetSweepJoin describes a join of two original boundary
// edges with equal signed displacement limits. ReverseB and BStationRatio
// specify the normalized free-parameter map as in SurfaceNormalOffsetSeamOptions.
// Axis selects a Cartesian separating coordinate. AInteriorSign is -1 or
// +1 depending on which open halfspace contains the rest of A.
type VerifiedNormalOffsetSweepJoin struct {
	Edges         [2]SurfaceEdge
	ReverseB      bool
	BStationRatio *big.Rat
	Axis          int
	AInteriorSign int
}

// VerifiedNormalOffsetSweepJoinProof records the exact original seam and
// the two full-displacement halfspace tests. Coordinate is published only
// when the join is complete.
type VerifiedNormalOffsetSweepJoinProof struct {
	Complete        bool
	Seam            SurfaceNormalOffsetSeam
	Coordinate      *big.Rat
	ExactOperations int
}

// VerifiedNormalOffsetSweepPair is the joined image of two original
// embedded sweep charts sharing exactly one whole swept wall.
type VerifiedNormalOffsetSweepPair struct {
	sweeps     [2]*VerifiedNormalOffsetSweep
	join       VerifiedNormalOffsetSweepJoin
	coordinate *big.Rat
}

// VerifiedNormalOffsetSweepBoundaryFace retains an outer analytic face,
// its original owner (A=0 or B=1), and original sweep side.
type VerifiedNormalOffsetSweepBoundaryFace struct {
	Owner int
	Side  NormalOffsetSweepSide
	Face  VerifiedNormalOffsetSweepFace
}

// NewVerifiedNormalOffsetSweepPair proves a bounded two-chart join with a
// single Cartesian separating plane. The exact seam certificate equates the
// original points and the oriented source normals at every matched station,
// hence the whole wall at every signed displacement. The shared source edge
// has constant separating coordinate and zero normal component on that axis.
// Each remaining point of each sweep is in its designated open halfspace,
// as proved by strict transverse derivative bounds at both displacement
// limits. These derivatives are affine in displacement. Strict separation
// therefore applies even to points at different displacement layers.
//
// The two original sweeps supply injectivity on their own charts. The join
// proof shares the exact operation budget over all its stages. MaxDepth and
// MaxCells bound subdivision separately on each original chart.
func NewVerifiedNormalOffsetSweepPair(ctx context.Context, a, b *VerifiedNormalOffsetSweep,
	join VerifiedNormalOffsetSweepJoin, o NormalOffsetCoverageOptions) (_ *VerifiedNormalOffsetSweepPair,
	r VerifiedNormalOffsetSweepJoinProof, err error) {
	if ctx == nil || a == nil || b == nil || a.orientation == 0 || b.orientation == 0 ||
		join.Edges[0] > SurfaceEdgeVMax || join.Edges[1] > SurfaceEdgeVMax ||
		join.Axis < 0 || join.Axis > 2 || (join.AInteriorSign != -1 && join.AInteriorSign != 1) ||
		(join.BStationRatio != nil && join.BStationRatio.Sign() <= 0) ||
		o.MaxDepth < 0 || o.MaxDepth > 64 || o.MaxCells < 0 || o.MaxCells > 1<<18 {
		return nil, r, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, r, err
	}
	limits, err := patchOptions(o.Patch)
	if err != nil {
		return nil, r, err
	}
	if a.lower.Cmp(b.lower) != 0 || a.upper.Cmp(b.upper) != 0 {
		return nil, r, ErrDomain
	}
	if len(a.source.net) > limits.MaxPoles || len(b.source.net) > limits.MaxPoles {
		return nil, r, ErrConvergence
	}

	r.Seam, err = CertifySurfaceNormalOffsetSeam(ctx, a.source, b.source, join.Edges[0], join.Edges[1],
		SurfaceNormalOffsetSeamOptions{ReverseB: join.ReverseB, BStationRatio: join.BStationRatio,
			MaxOperations: limits.MaxOperations, MaxCoefficientBits: limits.MaxCoefficientBits})
	r.ExactOperations = r.Seam.ExactOperations
	if err != nil {
		return nil, r, err
	}
	if !r.Seam.Complete {
		return nil, r, ErrDomain
	}
	if r.ExactOperations >= limits.MaxOperations {
		return nil, r, ErrConvergence
	}
	budget := &polynomialBudget{maxOperations: limits.MaxOperations - r.ExactOperations, maxBits: limits.MaxCoefficientBits}
	c, err := sweepSeparatorCoordinate(ctx, a.source, join.Edges[0], join.Axis, budget)
	r.ExactOperations += budget.operations
	if err != nil {
		return nil, r, err
	}

	sweeps := [2]*VerifiedNormalOffsetSweep{a, b}
	sign := join.AInteriorSign
	for owner, sweep := range sweeps {
		fixed, increasingInward := 0, 1
		edge := join.Edges[owner]
		if edge >= SurfaceEdgeVMin {
			fixed = 1
		}
		if edge == SurfaceEdgeUMax || edge == SurfaceEdgeVMax {
			increasingInward = -1
		}
		if r.ExactOperations >= limits.MaxOperations {
			return nil, r, ErrConvergence
		}
		o.Patch = limits
		o.Patch.MaxOperations -= r.ExactOperations
		proof, err := sweep.source.certifyNormalOffsetCells(ctx, sweep.upper, o,
			func(ctx context.Context, cell *SurfacePatch, upper *big.Rat, budget *polynomialBudget) (bool, error) {
				return sweepSeparatorCell(ctx, cell, sweep.lower, upper, join.Axis, fixed, sign*increasingInward, budget)
			})
		r.ExactOperations += proof.ExactOperations
		if err != nil {
			return nil, r, err
		}
		sign = -sign
	}
	wallA := VerifiedNormalOffsetSweepFace{a, NormalOffsetSweepSide(join.Edges[0])}
	wallB := VerifiedNormalOffsetSweepFace{b, NormalOffsetSweepSide(join.Edges[1])}
	// Reversing the seam station reverses B's (station, displacement)
	// wall frame before the two outward winding signs are compared.
	if wallA.ReverseWinding() == (wallB.ReverseWinding() != join.ReverseB) {
		return nil, r, ErrDomain
	}
	if err := ctx.Err(); err != nil {
		return nil, r, err
	}
	if join.BStationRatio != nil {
		join.BStationRatio = new(big.Rat).Set(join.BStationRatio)
	}
	r.Complete, r.Coordinate = true, new(big.Rat).Set(c)
	return &VerifiedNormalOffsetSweepPair{sweeps: sweeps, join: join, coordinate: c}, r, nil
}

// Join returns the retained original station map and the exact value of
// the selected separating coordinate. Rational values are caller-owned.
func (p *VerifiedNormalOffsetSweepPair) Join() (VerifiedNormalOffsetSweepJoin, *big.Rat) {
	if p == nil || p.coordinate == nil {
		return VerifiedNormalOffsetSweepJoin{}, nil
	}
	join := p.join
	if join.BStationRatio != nil {
		join.BStationRatio = new(big.Rat).Set(join.BStationRatio)
	}
	return join, new(big.Rat).Set(p.coordinate)
}

// Boundary returns the ten retained outer analytic faces in owner and side
// order. The two common-wall faces are interior to the joined image.
func (p *VerifiedNormalOffsetSweepPair) Boundary() []VerifiedNormalOffsetSweepBoundaryFace {
	if p == nil || p.coordinate == nil {
		return nil
	}
	result := make([]VerifiedNormalOffsetSweepBoundaryFace, 0, 10)
	for owner, sweep := range p.sweeps {
		for side := NormalOffsetSweepUMin; side <= NormalOffsetSweepUpper; side++ {
			if side != NormalOffsetSweepSide(p.join.Edges[owner]) {
				result = append(result, VerifiedNormalOffsetSweepBoundaryFace{
					Owner: owner, Side: side, Face: VerifiedNormalOffsetSweepFace{sweep, side},
				})
			}
		}
	}
	return result
}

func sweepSeparatorCoordinate(ctx context.Context, source *SurfacePatch, edge SurfaceEdge,
	axis int, budget *polynomialBudget) (*big.Rat, error) {
	jet, err := offsetEdgePowers(ctx, source, edge, false, nil, budget)
	if err != nil {
		return nil, err
	}
	zero := new(big.Rat)
	first, err := polynomialValue(jet[0][axis], zero, budget)
	if err != nil {
		return nil, err
	}
	weight, err := polynomialValue(jet[0][3], zero, budget)
	if err != nil {
		return nil, err
	}
	c := new(big.Rat).Quo(first, weight)
	if err := budget.check(c); err != nil {
		return nil, err
	}
	diff, err := offsetPolynomialDet(jet[0][axis], jet[0][3], rationalPolynomial{c}, rationalPolynomial{big.NewRat(1, 1)}, budget)
	if err != nil {
		return nil, err
	}
	if len(diff) != 0 {
		return nil, ErrDomain
	}
	normal, err := offsetNormalPowers(ctx, jet, budget)
	if err != nil {
		return nil, err
	}
	if len(normal[axis]) != 0 {
		return nil, ErrDomain
	}
	return c, nil
}

func sweepSeparatorCell(ctx context.Context, p *SurfacePatch, lower, upper *big.Rat,
	axis, fixed, sign int, budget *polynomialBudget) (bool, error) {
	if err := budget.check(lower); err != nil {
		return false, err
	}
	d, err := offsetCellJacobianBounds(ctx, p, budget)
	if err != nil || d == nil {
		return false, err
	}
	a := &d.arithmetic
	normalDerivative := d.normalDerivative(axis, fixed)
	for _, t := range []*big.Rat{lower, upper} {
		derivative := a.add(d.tangent[fixed][axis], a.mul(exactInterval{t, t}, normalDerivative))
		if a.err != nil {
			return false, a.err
		}
		lo, hi := derivative.lo.Sign(), derivative.hi.Sign()
		if sign < 0 {
			lo, hi = -hi, -lo
		}
		if hi < 0 {
			return false, ErrDomain
		}
		if lo <= 0 {
			return false, nil
		}
	}
	return true, nil
}
