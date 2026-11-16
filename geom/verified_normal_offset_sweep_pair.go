package geom

import (
	"context"
	"math/big"
)

// VerifiedNormalOffsetSweepJoin describes a join of two original boundary
// edges with equal signed displacement limits. ReverseB and BStationRatio
// specify the normalized free-parameter map as in SurfaceNormalOffsetSeamOptions.
// Axis selects a Cartesian separating coordinate when Covector is omitted.
// An explicit Covector selects an exact linear projection instead. Nil
// components count as zero, and an explicit zero covector is invalid.
// AInteriorSign is -1 or +1 depending on which open halfspace contains
// the rest of A.
type VerifiedNormalOffsetSweepJoin struct {
	Edges         [2]SurfaceEdge
	ReverseB      bool
	BStationRatio *big.Rat
	Axis          int
	AInteriorSign int
	Covector      *[3]*big.Rat `json:",omitempty"`
}

// VerifiedNormalOffsetSweepJoinProof records the exact original seam and
// the two full-displacement halfspace tests. Coordinate is the constant
// of the plane in the selected coordinate or the original covector scale.
// It is published only when the join is complete.
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
// single exact separating plane. The seam certificate equates the original
// points and the oriented source normals at every matched station, hence the
// whole wall at every signed displacement. The shared source edge has
// constant projection onto the specified plane covector, and the source
// normal is orthogonal to that covector along the edge.
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
	oblique := join.Covector != nil
	if ctx == nil || a == nil || b == nil || a.orientation == 0 || b.orientation == 0 ||
		join.Edges[0] > SurfaceEdgeVMax || join.Edges[1] > SurfaceEdgeVMax ||
		(!oblique && (join.Axis < 0 || join.Axis > 2)) || (oblique && !nonzeroSweepSeparatorCovector(*join.Covector)) ||
		(join.AInteriorSign != -1 && join.AInteriorSign != 1) ||
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
	c, err := sweepSeparatorCoordinate(ctx, a.source, join.Edges[0], join.Axis, join.Covector, budget)
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
				return sweepSeparatorCell(ctx, cell, sweep.lower, upper, join.Axis, join.Covector, fixed, sign*increasingInward, budget)
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
	join = join.copyExact()
	r.Complete, r.Coordinate = true, new(big.Rat).Set(c)
	return &VerifiedNormalOffsetSweepPair{sweeps: sweeps, join: join, coordinate: c}, r, nil
}

// Join returns the retained original station map, exact separating
// projection and constant of the plane. Rational values are caller-owned.
func (p *VerifiedNormalOffsetSweepPair) Join() (VerifiedNormalOffsetSweepJoin, *big.Rat) {
	if p == nil || p.coordinate == nil {
		return VerifiedNormalOffsetSweepJoin{}, nil
	}
	return p.join.copyExact(), new(big.Rat).Set(p.coordinate)
}

func (join VerifiedNormalOffsetSweepJoin) copyExact() VerifiedNormalOffsetSweepJoin {
	if join.BStationRatio != nil {
		join.BStationRatio = new(big.Rat).Set(join.BStationRatio)
	}
	if join.Covector != nil {
		covector := *join.Covector
		for i, q := range covector {
			if q != nil {
				covector[i] = new(big.Rat).Set(q)
			}
		}
		join.Covector = &covector
	}
	return join
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
	axis int, covector *[3]*big.Rat, budget *polynomialBudget) (*big.Rat, error) {
	oblique := covector != nil
	if oblique {
		if err := checkSweepSeparatorCovector(*covector, budget); err != nil {
			return nil, err
		}
	}
	jet, err := offsetEdgePowers(ctx, source, edge, false, nil, budget)
	if err != nil {
		return nil, err
	}
	zero := new(big.Rat)
	var projection rationalPolynomial
	if oblique {
		projection, err = sweepSeparatorPolynomial([3]rationalPolynomial(jet[0][:3]), *covector, budget)
		if err != nil {
			return nil, err
		}
	} else {
		projection = jet[0][axis]
	}
	first, err := polynomialValue(projection, zero, budget)
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
	diff, err := offsetPolynomialDet(projection, jet[0][3], rationalPolynomial{c}, rationalPolynomial{big.NewRat(1, 1)}, budget)
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
	if oblique {
		projection, err = sweepSeparatorPolynomial(normal, *covector, budget)
		if err != nil {
			return nil, err
		}
	} else {
		projection = normal[axis]
	}
	if len(projection) != 0 {
		return nil, ErrDomain
	}
	return c, nil
}

func sweepSeparatorCell(ctx context.Context, p *SurfacePatch, lower, upper *big.Rat,
	axis int, covector *[3]*big.Rat, fixed, sign int, budget *polynomialBudget) (bool, error) {
	if err := budget.check(lower); err != nil {
		return false, err
	}
	d, err := offsetCellJacobianBounds(ctx, p, budget)
	if err != nil || d == nil {
		return false, err
	}
	a := &d.arithmetic
	var tangent, normalDerivative exactInterval
	if covector == nil {
		tangent, normalDerivative = d.tangent[fixed][axis], d.normalDerivative(axis, fixed)
	} else {
		tangent, normalDerivative = sweepSeparatorProjectedDifferential(d, fixed, *covector)
	}
	for _, t := range []*big.Rat{lower, upper} {
		derivative := a.add(tangent, a.mul(exactInterval{t, t}, normalDerivative))
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
