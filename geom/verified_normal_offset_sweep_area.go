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

// BoundaryAreaRational measures all six original boundary faces directly
// from the exact source and displacement interval. Affine frames have exact
// area squares even when the unit normal is irrational. A perpendicular
// circular extrusion also supports directed area bounds when its original
// rational quadratic arc has tangent of the angle at most 1/2 and both
// shifted radii are positive. Other frames return ErrDomain.
func (s *VerifiedNormalOffsetSweep) BoundaryAreaRational(ctx context.Context,
	o NormalOffsetSweepAreaOptions) (r NormalOffsetSweepBoundaryArea, err error) {
	if s == nil || s.source == nil {
		return r, ErrInvalid
	}
	o, err = normalizeSweepBoundaryAreaOptions(ctx, o)
	if err != nil {
		return r, err
	}
	b := &polynomialBudget{maxOperations: o.Patch.MaxOperations, maxBits: o.Patch.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	err = s.measureRetainedBoundaryArea(ctx, o, 0b111111, b, &r)
	return r, err
}

func normalizeSweepBoundaryAreaOptions(ctx context.Context, o NormalOffsetSweepAreaOptions) (NormalOffsetSweepAreaOptions, error) {
	if ctx == nil || o.MaxCells < 0 || o.MaxCells > 1<<18 ||
		(o.AbsoluteError != nil && o.AbsoluteError.Sign() < 0) {
		return o, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return o, err
	}
	if o.AbsoluteError == nil {
		o.AbsoluteError = big.NewRat(1, 100_000_000)
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4095
	}
	if o.Patch.MaxOperations == 0 {
		o.Patch.MaxOperations = 4_000_000
	}
	patch, err := patchOptions(o.Patch)
	o.Patch = patch
	return o, err
}

func (s *VerifiedNormalOffsetSweep) measureRetainedBoundaryArea(ctx context.Context, o NormalOffsetSweepAreaOptions,
	retained uint8, b *polynomialBudget, r *NormalOffsetSweepBoundaryArea) error {
	if len(s.source.net) > o.Patch.MaxPoles {
		return ErrConvergence
	}
	for _, q := range [3]*big.Rat{o.AbsoluteError, s.lower, s.upper} {
		if err := b.check(q); err != nil {
			return err
		}
	}
	squares, affine, err := s.affineSweepAreaSquares(ctx, retained, b)
	if err != nil {
		return err
	}
	if !affine {
		return s.circularSweepBoundaryArea(ctx, o, retained, b, r)
	}

	// Reserve less than one sixth of the requested total width for each
	// square root. An equal-endpoint request still receives a useful
	// enclosure in case the area is irrational.
	precisionGoal := o.AbsoluteError
	if precisionGoal.Sign() == 0 {
		precisionGoal = big.NewRat(1, 100_000_000)
	}
	precision := max(0, precisionGoal.Denom().BitLen()-precisionGoal.Num().BitLen()) + 4
	a := boundedRatArithmetic{b: b}
	total := exactInterval{new(big.Rat), new(big.Rat)}
	for side, square := range squares {
		if retained&(1<<side) == 0 {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if r.VisitedCells >= o.MaxCells {
			return ErrConvergence
		}
		r.VisitedCells++
		// Limit the integer shift used by the directed square root.
		bits := min(precision, (b.maxBits-square.Num().BitLen())/2)
		lo, hi, err := rationalSqrtBounds(square, bits, b)
		if err != nil {
			return err
		}
		r.Faces = append(r.Faces, NormalOffsetSweepFaceArea{
			Side: NormalOffsetSweepSide(side), Bounds: ExactParameterInterval{Lower: lo, Upper: hi},
			SquaredArea: new(big.Rat).Set(square),
		})
		total = exactInterval{a.add(total.lo, lo), a.add(total.hi, hi)}
		if a.err != nil {
			return a.err
		}
	}
	r.Bounds = copySweepInterval(total)
	if err := ctx.Err(); err != nil {
		return err
	}
	width := a.sub(total.hi, total.lo)
	if a.err != nil {
		return a.err
	}
	if width.Cmp(o.AbsoluteError) > 0 {
		return ErrConvergence
	}
	r.Complete = true
	return nil
}

func (s *VerifiedNormalOffsetSweep) affineSweepAreaSquares(ctx context.Context,
	retained uint8, b *polynomialBudget) ([6]*big.Rat, bool, error) {
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
	if retained&(3<<NormalOffsetSweepLower) != 0 {
		for i := range cross {
			j, k := (i+1)%3, (i+2)%3
			cross[i] = a.sub(a.mul(u[j], v[k]), a.mul(u[k], v[j]))
		}
	}
	thickness := a.sub(s.upper, s.lower)
	thicknessSquare := a.mul(thickness, thickness)
	// Each side wall extrudes one full source edge along a unit normal
	// orthogonal to it. Both caps have the original area.
	if retained&(3<<NormalOffsetSweepUMin) != 0 {
		uWall := a.mul(sweepLineDot(&a, v, v), thicknessSquare)
		result[NormalOffsetSweepUMin], result[NormalOffsetSweepUMax] = uWall, uWall
	}
	if retained&(3<<NormalOffsetSweepVMin) != 0 {
		vWall := a.mul(sweepLineDot(&a, u, u), thicknessSquare)
		result[NormalOffsetSweepVMin], result[NormalOffsetSweepVMax] = vWall, vWall
	}
	if retained&(3<<NormalOffsetSweepLower) != 0 {
		cap := sweepLineDot(&a, cross, cross)
		result[NormalOffsetSweepLower], result[NormalOffsetSweepUpper] = cap, cap
	}
	if a.err != nil {
		return result, true, a.err
	}
	return result, true, nil
}
