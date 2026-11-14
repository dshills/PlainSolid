package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

func (o *AlgebraicGraphOccurrence) pullbackLine(a *graphOccurrenceArithmetic, origin, direction [3]*big.Rat) (p, d [3]*big.Rat) {
	for i := 0; i < 3; i++ {
		p[i] = new(big.Rat).Set(o.inverse[i][3])
		d[i] = new(big.Rat)
		for j := 0; j < 3; j++ {
			p[i] = a.calc('+', p[i], a.calc('*', o.inverse[i][j], origin[j]))
			d[i] = a.calc('+', d[i], a.calc('*', o.inverse[i][j], direction[j]))
		}
	}
	return p, d
}
func (o *AlgebraicGraphOccurrence) imagePoint(a *graphOccurrenceArithmetic, p [3]*big.Rat) (r [3]*big.Rat) {
	for i := 0; i < 3; i++ {
		r[i] = new(big.Rat).Set(o.pose[i][3])
		for j := 0; j < 3; j++ {
			r[i] = a.calc('+', r[i], a.calc('*', o.pose[i][j], p[j]))
		}
	}
	return r
}

// IntersectNormalLineRational uses world p(t)=origin+t*scale*e_residual.
// Fiber preservation proves its pullback is an original normal line. Original t
// (including sign/scale), all native IDs and Before/After remain unchanged;
// rational witnesses are mapped exactly to world XYZ. Pose arithmetic, clipping
// and witness construction share the exact per-query work budget.
func (o *AlgebraicGraphOccurrence) IntersectNormalLineRational(ctx context.Context, origin [3]*big.Rat, scale *big.Rat, limits [2]*big.Rat, options geom.ExactCurvePlaneEventOptions) (out AlgebraicGraphFiberLine, err error) {
	out.Faces = [2]int{-1, -1}
	defer func() {
		if err != nil {
			out = AlgebraicGraphFiberLine{Faces: [2]int{-1, -1}, ExactOperations: out.ExactOperations}
		}
	}()
	if ctx == nil || o == nil || o.query == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	options, err = graphMaterialLimits(options)
	if err != nil {
		return out, err
	}
	a := graphOccurrenceArithmetic{ctx: ctx, options: options}
	direction := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	direction[o.residual] = scale
	p, d := o.pullbackLine(&a, origin, direction)
	out.ExactOperations = a.used
	if a.err != nil {
		return out, a.err
	}
	for _, axis := range o.query.axes {
		if d[axis].Sign() != 0 {
			return out, geom.ErrConvergence
		}
	}
	remaining := options
	remaining.MaxOperations -= a.used
	if remaining.MaxOperations < 1 {
		return out, geom.ErrConvergence
	}
	out, err = o.query.IntersectNormalLineRational(ctx, p, d[o.query.residual], limits, remaining)
	a.used += out.ExactOperations
	out.ExactOperations = a.used
	if err != nil {
		return out, err
	}
	for i := range out.Hits {
		out.Hits[i].Point = o.imagePoint(&a, out.Hits[i].Point)
	}
	out.ExactOperations = a.used
	if a.err != nil {
		return out, a.err
	}
	return out, nil
}

// BoundObliqueLineRational pulls a world line back without renormalizing t.
// Algebraic Root identities and parameter brackets therefore remain exact in
// caller units. All XYZ witness intervals are conservatively affine-transformed;
// isolated rational witnesses are transformed exactly. Original face/rim IDs,
// retained face overlaps and material transition directions remain unchanged.
// Source ambiguities (including unresolved shared-rim contacts) still reject.
func (o *AlgebraicGraphOccurrence) BoundObliqueLineRational(ctx context.Context, origin, direction [3]*big.Rat, limits [2]*big.Rat, options geom.ToroidLineOptions) (out AlgebraicGraphLineBounds, err error) {
	defer func() {
		if err != nil {
			out = AlgebraicGraphLineBounds{ExactOperations: out.ExactOperations}
		}
	}()
	if ctx == nil || o == nil || o.query == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if options.Surface.MaxOperations == 0 {
		options.Surface.MaxOperations = 64000000
	}
	exact, e := graphMaterialLimits(geom.ExactCurvePlaneEventOptions{MaxOperations: options.Surface.MaxOperations, MaxCoefficientBits: options.Surface.MaxCoefficientBits})
	if e != nil {
		return out, e
	}
	a := graphOccurrenceArithmetic{ctx: ctx, options: exact}
	p, d := o.pullbackLine(&a, origin, direction)
	out.ExactOperations = a.used
	if a.err != nil {
		return out, a.err
	}
	options.Surface.MaxOperations -= a.used
	if options.Surface.MaxOperations < 1 {
		return out, geom.ErrConvergence
	}
	out, err = o.query.BoundObliqueLineRational(ctx, p, d, limits, options)
	a.used += out.ExactOperations
	out.ExactOperations = a.used
	if err != nil {
		return out, err
	}
	for i := range out.Hits {
		h := &out.Hits[i]
		sourceBounds := h.PointBounds
		if h.Point[0] != nil {
			h.Point = o.imagePoint(&a, h.Point)
		}
		for row := 0; row < 3; row++ {
			lo, hi := new(big.Rat).Set(o.pose[row][3]), new(big.Rat).Set(o.pose[row][3])
			for column := 0; column < 3; column++ {
				x := a.calc('*', o.pose[row][column], sourceBounds[column].Lower)
				y := a.calc('*', o.pose[row][column], sourceBounds[column].Upper)
				if x.Cmp(y) > 0 {
					x, y = y, x
				}
				lo = a.calc('+', lo, x)
				hi = a.calc('+', hi, y)
			}
			h.PointBounds[row] = geom.ExactParameterInterval{Lower: lo, Upper: hi}
		}
	}
	out.ExactOperations = a.used
	if a.err != nil {
		return out, a.err
	}
	return out, nil
}
