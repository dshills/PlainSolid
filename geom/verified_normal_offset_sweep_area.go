package geom

import (
	"context"
	"math/big"
)

// NormalOffsetSweepAreaOptions bounds analytic boundary area measurement.
// AbsoluteError is the required nonnegative width of the total area
// enclosure. Nil requests 1e-8 square model units. Zero requests equal
// rational endpoints. Zero MaxCells allows 4095 visited original face
// cells. The default exact operations limit is four million.
type NormalOffsetSweepAreaOptions struct {
	AbsoluteError *big.Rat
	Patch         SurfacePatchOptions
	MaxCells      int
}

// NormalOffsetSweepFaceArea bounds the physical area of one original
// analytic boundary face. SquaredArea is present when its value was proved
// exactly, even if the corresponding area is irrational.
type NormalOffsetSweepFaceArea struct {
	Side        NormalOffsetSweepSide
	Bounds      ExactParameterInterval
	SquaredArea *big.Rat `json:",omitempty"`
}

// NormalOffsetSweepBoundaryArea records directed physical area intervals in
// original side order. Complete means the total width met the request.
// Previously proved faces survive resource refusal. Bounds is present only
// after every original face is covered and the total has been enclosed.
type NormalOffsetSweepBoundaryArea struct {
	Faces                         []NormalOffsetSweepFaceArea
	Bounds                        ExactParameterInterval
	Complete                      bool
	VisitedCells, ExactOperations int
}

// BoundaryAreaRational measures all six original boundary faces of a verified
// affine sweep directly from its exact source and displacement interval.
// The affine frame need not have a rational unit normal. Each face has an
// exact area square, and nonsquare roots are enclosed by directed rationals.
// Curved frames return ErrDomain.
func (s *VerifiedNormalOffsetSweep) BoundaryAreaRational(ctx context.Context,
	o NormalOffsetSweepAreaOptions) (r NormalOffsetSweepBoundaryArea, err error) {
	if ctx == nil || s == nil || s.source == nil || o.MaxCells < 0 || o.MaxCells > 1<<18 ||
		(o.AbsoluteError != nil && o.AbsoluteError.Sign() < 0) {
		return r, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	defaultTolerance := big.NewRat(1, 100_000_000)
	if o.AbsoluteError == nil {
		o.AbsoluteError = defaultTolerance
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4095
	}
	if o.Patch.MaxOperations == 0 {
		o.Patch.MaxOperations = 4_000_000
	}
	limits, err := patchOptions(o.Patch)
	if err != nil {
		return r, err
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	if len(s.source.net) > limits.MaxPoles {
		return r, ErrConvergence
	}
	for _, q := range [3]*big.Rat{o.AbsoluteError, s.lower, s.upper} {
		if err := b.check(q); err != nil {
			return r, err
		}
	}
	squares, affine, err := s.affineSweepAreaSquares(ctx, b)
	if err != nil {
		return r, err
	}
	if !affine {
		return r, ErrDomain
	}

	// Reserve less than one sixth of the requested total width for each
	// square root. An equal-endpoint request still receives a useful
	// enclosure in case the area is irrational.
	precisionGoal := o.AbsoluteError
	if precisionGoal.Sign() == 0 {
		precisionGoal = defaultTolerance
	}
	precision := max(0, precisionGoal.Denom().BitLen()-precisionGoal.Num().BitLen()) + 4
	a := boundedRatArithmetic{b: b}
	total := exactInterval{new(big.Rat), new(big.Rat)}
	for side, square := range squares {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		if r.VisitedCells >= o.MaxCells {
			return r, ErrConvergence
		}
		r.VisitedCells++
		// Limit the integer shift used by the directed square root.
		bits := min(precision, (b.maxBits-square.Num().BitLen())/2)
		lo, hi, err := rationalSqrtBounds(square, bits, b)
		if err != nil {
			return r, err
		}
		r.Faces = append(r.Faces, NormalOffsetSweepFaceArea{
			Side: NormalOffsetSweepSide(side), Bounds: ExactParameterInterval{Lower: lo, Upper: hi},
			SquaredArea: new(big.Rat).Set(square),
		})
		total = exactInterval{a.add(total.lo, lo), a.add(total.hi, hi)}
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

func (s *VerifiedNormalOffsetSweep) affineSweepAreaSquares(ctx context.Context,
	b *polynomialBudget) ([6]*big.Rat, bool, error) {
	var result [6]*big.Rat
	a := boundedRatArithmetic{b: b}
	u, v, affine, err := sweepHomogeneousAffineSpans(ctx, s.source, &a)
	if err != nil || !affine {
		return result, affine, err
	}
	weight := s.source.net[0][3]
	for i := range u {
		u[i], v[i] = a.quo(u[i], weight), a.quo(v[i], weight)
	}
	var cross [3]*big.Rat
	for i := range cross {
		j, k := (i+1)%3, (i+2)%3
		cross[i] = a.sub(a.mul(u[j], v[k]), a.mul(u[k], v[j]))
	}
	thickness := a.sub(s.upper, s.lower)
	thicknessSquare := a.mul(thickness, thickness)
	// Each side wall extrudes one full source edge along a unit normal
	// orthogonal to it. Both caps have the original area.
	uWall := a.mul(sweepLineDot(&a, v, v), thicknessSquare)
	vWall := a.mul(sweepLineDot(&a, u, u), thicknessSquare)
	cap := sweepLineDot(&a, cross, cross)
	if a.err != nil {
		return result, true, a.err
	}
	return [6]*big.Rat{uWall, uWall, vWall, vWall, cap, cap}, true, nil
}
