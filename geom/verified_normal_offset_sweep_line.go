package geom

import (
	"context"
	"math/big"
)

// NormalOffsetSweepLineOptions bounds exact work for a verified normal
// sweep line intersection. ProfileParameter optionally supplies an exact
// original transverse station for a curved perpendicular extrusion.
// With no station, original midpoint and endpoints are tried before
// isolating the normal-foot roots of an original profile of degree at
// most three.
type NormalOffsetSweepLineOptions struct {
	Patch            SurfacePatchOptions
	ProfileParameter *big.Rat
}

// NormalOffsetSweepLineEndpoint records a point of origin+Parameter*direction
// and the exact original (U,V,displacement) station. Faces lists all original
// analytic boundary faces incident to that point. A nil Station entry
// refers to the corresponding algebraic coordinate in the span.
type NormalOffsetSweepLineEndpoint struct {
	Parameter *big.Rat
	Point     [3]*big.Rat
	Station   [3]*big.Rat
	Faces     []NormalOffsetSweepSide
}

// NormalOffsetSweepLineSpan is the whole material interval along the
// queried line. Endpoints can coincide at an isolated boundary contact.
// BoundaryFaces lists the original faces containing the entire span.
type NormalOffsetSweepLineSpan struct {
	Start, End       NormalOffsetSweepLineEndpoint
	BoundaryFaces    []NormalOffsetSweepSide
	AlgebraicStation *NormalOffsetSweepLineAlgebraicStation `json:",omitempty"`
}

// NormalOffsetSweepLineAlgebraicStation records one isolated root of
// the original profile equation on normalized Bernstein [0,1] and its
// directed original raw parameter and signed displacement bounds. Exact
// material endpoints can accompany coarse station bounds when refinement
// uses its allotted work.
type NormalOffsetSweepLineAlgebraicStation struct {
	ProfileAxis        int
	OriginalProfile    ExactParameterInterval
	ProfileRoot        ExactRootRecipe
	ProfileParameter   ExactParameterInterval
	SignedDisplacement ExactParameterInterval
}

// NormalOffsetSweepLineIntersection returns one complete interval or a
// certified miss (Complete with a nil Span). Incomplete results carry
// no span.
type NormalOffsetSweepLineIntersection struct {
	Complete        bool
	Span            *NormalOffsetSweepLineSpan
	ExactOperations int
}

// IntersectRationalLine clips the entire exact line against the original
// verified sweep. It supports affine frames with rational unit normals
// and ruling-parallel lines through verified perpendicular extrusions
// with a proved rational or isolated algebraic transverse source station.
// A line outside the
// extrusion's transverse hull is a certified miss. Unsupported frames
// and line directions return ErrDomain. A ruling query with neither
// an exact transverse witness nor an exclusion returns ErrConvergence.
func (s *VerifiedNormalOffsetSweep) IntersectRationalLine(ctx context.Context, origin, direction [3]*big.Rat,
	o NormalOffsetSweepLineOptions) (r NormalOffsetSweepLineIntersection, err error) {
	if ctx == nil || s == nil || s.source == nil {
		return r, ErrInvalid
	}
	nonzero := false
	for i, q := range origin {
		if q == nil || direction[i] == nil {
			return r, ErrInvalid
		}
		nonzero = nonzero || direction[i].Sign() != 0
	}
	if !nonzero {
		return r, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return r, err
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
	for _, vector := range [2][3]*big.Rat{origin, direction} {
		for _, q := range vector {
			if err := b.check(q); err != nil {
				return r, err
			}
		}
	}
	for _, q := range [3]*big.Rat{s.lower, s.upper, o.ProfileParameter} {
		if q != nil {
			if err := b.check(q); err != nil {
				return r, err
			}
		}
	}
	work := sweepLineWork{
		sweep: s, origin: origin, direction: direction, budget: b,
		original: normalSweepParameterIntervals(s.source.Domain(), exactInterval{s.lower, s.upper}),
	}
	span, affine, err := work.intersectAffine(ctx)
	if err != nil {
		return r, err
	}
	if !affine {
		span, err = work.intersectExtrusion(ctx, o.ProfileParameter)
		if err != nil {
			return r, err
		}
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	r.Complete, r.Span = true, span
	return r, nil
}

type sweepLineWork struct {
	sweep             *VerifiedNormalOffsetSweep
	origin, direction [3]*big.Rat
	original          [3]exactInterval
	budget            *polynomialBudget
}

func (w *sweepLineWork) intersectAffine(ctx context.Context) (*NormalOffsetSweepLineSpan, bool, error) {
	a := boundedRatArithmetic{b: w.budget}
	source := w.sweep.source
	u, v, affine, err := sweepHomogeneousAffineSpans(ctx, source, &a)
	if err != nil || !affine {
		return nil, false, err
	}
	n, rational, err := w.unitNormal(ctx, u, v)
	if err != nil || !rational {
		return nil, false, err
	}
	h := source.net[0]
	thickness := a.sub(w.original[2].hi, w.original[2].lo)
	var frame [3][3]*big.Rat
	var offset [3]*big.Rat
	for i := range frame {
		frame[i] = [3]*big.Rat{a.quo(u[i], h[3]), a.quo(v[i], h[3]), a.mul(thickness, n[i])}
		base := a.add(a.quo(h[i], h[3]), a.mul(w.original[2].lo, n[i]))
		offset[i] = a.sub(w.origin[i], base)
	}
	if a.err != nil {
		return nil, false, a.err
	}
	inverse, invertible, err := inverseExact3(frame, w.budget)
	if err != nil {
		return nil, false, err
	}
	if !invertible {
		return nil, false, ErrConvergence
	}
	var at, rate [3]*big.Rat
	for i, row := range inverse {
		at[i], rate[i] = sweepLineDot(&a, offset, row), sweepLineDot(&a, w.direction, row)
	}
	if a.err != nil {
		return nil, false, a.err
	}
	span, err := w.clipUnitCube(at, rate)
	return span, true, err
}

func (w *sweepLineWork) clipUnitCube(at, rate [3]*big.Rat) (*NormalOffsetSweepLineSpan, error) {
	a := boundedRatArithmetic{b: w.budget}
	zero, one := new(big.Rat), big.NewRat(1, 1)
	var line exactInterval
	for i, q := range at {
		if rate[i].Sign() == 0 {
			if q.Sign() < 0 || q.Cmp(one) > 0 {
				return nil, nil
			}
			continue
		}
		lo := a.quo(a.sub(zero, q), rate[i])
		hi := a.quo(a.sub(one, q), rate[i])
		if a.err != nil {
			return nil, a.err
		}
		if lo.Cmp(hi) > 0 {
			lo, hi = hi, lo
		}
		if line.lo == nil || lo.Cmp(line.lo) > 0 {
			line.lo = lo
		}
		if line.hi == nil || hi.Cmp(line.hi) < 0 {
			line.hi = hi
		}
	}
	if line.lo == nil || line.hi == nil {
		return nil, ErrConvergence
	}
	if line.lo.Cmp(line.hi) > 0 {
		return nil, nil
	}
	return w.span(line, at, rate)
}

func (w *sweepLineWork) span(line exactInterval, at, rate [3]*big.Rat) (*NormalOffsetSweepLineSpan, error) {
	a := boundedRatArithmetic{b: w.budget}
	one := big.NewRat(1, 1)
	var endpoints [2]NormalOffsetSweepLineEndpoint
	for i, q := range [2]*big.Rat{line.lo, line.hi} {
		end := &endpoints[i]
		end.Parameter = new(big.Rat).Set(q)
		for axis, original := range w.original {
			parameter := a.add(at[axis], a.mul(q, rate[axis]))
			width := a.sub(original.hi, original.lo)
			end.Station[axis] = a.add(original.lo, a.mul(parameter, width))
			end.Point[axis] = a.add(w.origin[axis], a.mul(q, w.direction[axis]))
			if a.err != nil {
				return nil, a.err
			}
			if parameter.Sign() < 0 || parameter.Cmp(one) > 0 {
				return nil, ErrConvergence
			}
			switch {
			case parameter.Sign() == 0:
				end.Faces = append(end.Faces, NormalOffsetSweepSide(axis*2))
			case parameter.Cmp(one) == 0:
				end.Faces = append(end.Faces, NormalOffsetSweepSide(axis*2+1))
			}
		}
	}
	result := &NormalOffsetSweepLineSpan{Start: endpoints[0], End: endpoints[1]}
	for _, a := range result.Start.Faces {
		for _, b := range result.End.Faces {
			if a == b {
				result.BoundaryFaces = append(result.BoundaryFaces, a)
			}
		}
	}
	return result, nil
}

func sweepLineDot(a *boundedRatArithmetic, x, y [3]*big.Rat) *big.Rat {
	return a.add(a.add(a.mul(x[0], y[0]), a.mul(x[1], y[1])), a.mul(x[2], y[2]))
}

func (w *sweepLineWork) unitNormal(ctx context.Context, u, v [3]*big.Rat) (normal [3]*big.Rat, rational bool, err error) {
	remaining := w.budget.maxOperations - w.budget.operations
	if remaining <= 0 {
		return normal, false, ErrConvergence
	}
	zero := new(big.Rat)
	jet := RationalSurfacePatchJet{Point: [3]*big.Rat{zero, zero, zero}, Du: u, Dv: v}
	box, err := jet.NormalOffsetRational(ctx, big.NewRat(1, 1), SurfaceNormalOffsetEvaluationOptions{
		MaxOperations: remaining, MaxCoefficientBits: w.budget.maxBits,
	})
	w.budget.operations += box.ExactOperations
	if err != nil {
		return normal, false, err
	}
	for i, lo := range box.Lower {
		if lo.Cmp(box.Upper[i]) != 0 {
			return normal, false, nil
		}
		normal[i] = lo
	}
	return normal, true, nil
}
