package geom

import (
	"context"
	"errors"
	"math/big"
	"math/bits"
)

// NormalOffsetSweepJoinedFaceArea retains a measured analytic boundary face
// together with its zero-based original chart index.
type NormalOffsetSweepJoinedFaceArea struct {
	Owner int
	NormalOffsetSweepFaceArea
}

// NormalOffsetSweepJoinedBoundaryArea bounds the retained original outer
// boundary. Complete requires the requested total enclosure width. Faces
// already proved are retained when a budget interrupts the measurement.
// A total is published only once all retained faces have bounds.
type NormalOffsetSweepJoinedBoundaryArea struct {
	Faces                         []NormalOffsetSweepJoinedFaceArea
	Bounds                        ExactParameterInterval
	Complete                      bool
	VisitedCells, ExactOperations int
}

// BoundaryAreaRational measures only the retained original boundary of the
// verified pair. The joining walls are excluded from the measured face
// lists. Each retained original face uses its own authenticated geometry.
func (p *VerifiedNormalOffsetSweepPair) BoundaryAreaRational(ctx context.Context,
	o NormalOffsetSweepAreaOptions) (NormalOffsetSweepJoinedBoundaryArea, error) {
	if p == nil || p.coordinate == nil {
		return NormalOffsetSweepJoinedBoundaryArea{}, ErrInvalid
	}
	return measureJoinedSweepBoundaryArea(ctx, p.sweeps[:], p.Boundary(), o)
}

// BoundaryAreaRational measures the retained original boundary of the chain
// under a common visit and exact-operation limit across its three charts.
func (c *VerifiedNormalOffsetSweepChain) BoundaryAreaRational(ctx context.Context,
	o NormalOffsetSweepAreaOptions) (NormalOffsetSweepJoinedBoundaryArea, error) {
	if c == nil || c.pairs[0] == nil || c.pairs[1] == nil {
		return NormalOffsetSweepJoinedBoundaryArea{}, ErrInvalid
	}
	return measureJoinedSweepBoundaryArea(ctx, c.sweeps[:], c.Boundary(), o)
}

func measureJoinedSweepBoundaryArea(ctx context.Context, sweeps []*VerifiedNormalOffsetSweep,
	boundary []VerifiedNormalOffsetSweepBoundaryFace, o NormalOffsetSweepAreaOptions) (r NormalOffsetSweepJoinedBoundaryArea, err error) {
	o, err = normalizeSweepBoundaryAreaOptions(ctx, o)
	if err != nil {
		return r, err
	}
	b := &polynomialBudget{maxOperations: o.Patch.MaxOperations, maxBits: o.Patch.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	if err := b.check(o.AbsoluteError); err != nil {
		return r, err
	}
	a := boundedRatArithmetic{b: b}
	ownerTolerance := a.quo(o.AbsoluteError, big.NewRat(int64(len(sweeps)), 1))
	if a.err != nil {
		return r, a.err
	}
	var sides [3]uint8
	for _, face := range boundary {
		sides[face.Owner] |= 1 << face.Side
	}
	total := offsetZeroInterval()
	for owner, sweep := range sweeps {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		localOptions := o
		localOptions.MaxCells -= r.VisitedCells
		if localOptions.MaxCells == 0 || b.operations >= b.maxOperations {
			return r, ErrConvergence
		}
		localOptions.AbsoluteError = ownerTolerance
		var local NormalOffsetSweepBoundaryArea
		err := sweep.measureRetainedBoundaryArea(ctx, localOptions, sides[owner], b, &local)
		r.VisitedCells += local.VisitedCells
		for _, face := range local.Faces {
			r.Faces = append(r.Faces, NormalOffsetSweepJoinedFaceArea{Owner: owner, NormalOffsetSweepFaceArea: face})
		}
		covered := len(local.Faces) == bits.OnesCount8(sides[owner]) && local.Bounds.Lower != nil
		if err != nil && (!errors.Is(err, ErrConvergence) || !covered) {
			return r, err
		}
		if !covered {
			return r, ErrConvergence
		}
		total = exactInterval{a.add(total.lo, local.Bounds.Lower), a.add(total.hi, local.Bounds.Upper)}
		if a.err != nil {
			return r, a.err
		}
	}
	r.Bounds = copySweepInterval(total)
	if err := ctx.Err(); err != nil {
		return r, err
	}
	width := a.sub(total.hi, total.lo)
	if a.err != nil {
		return r, a.err
	}
	if width.Cmp(o.AbsoluteError) > 0 {
		return r, ErrConvergence
	}
	r.Complete = true
	return r, nil
}
