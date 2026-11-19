//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// WrappedFacePointLocation classifies a point of the original canonical
// cylinder support against this face, rather than merely against one chart.
// SuppliedUV is the caller's occurrence. FaceUV is the unique period-equivalent
// occurrence in the face hull when MatchedLift; nil otherwise. PeriodShift is
// the exact integer shift from SuppliedUV to FaceUV, in original chart periods.
// Point remains exact; no display representative decides membership.
type WrappedFacePointLocation struct {
	Complete, MatchedLift bool
	Location              trimvalidate.Location
	SuppliedUV, FaceUV    [2]*big.Rat
	PeriodShift           [2]*big.Int
	Point                 [3]*big.Rat
	ExactOperations       int
}

// LocateCylinderRationalUV evaluates the original support at an exact rational
// UV witness and classifies its model point against the retained quadratic
// loops, including holes and period-shifted occurrences. A separate global
// canonical cylinder embedding certificate excludes distant chart aliases;
// local face injectivity alone would not justify an Outside model claim.
// V outside the original support and noncanonical supports remain unsupported.
// Limits default to8M operations,16384 bits,4096 poles. Failure is atomic.
func (r *WrappedQuadraticFace) LocateCylinderRationalUV(ctx context.Context, uv [2]*big.Rat, o geom.SurfacePatchOptions) (WrappedFacePointLocation, error) {
	var zero WrappedFacePointLocation
	if ctx == nil || r == nil || len(r.loops) == 0 {
		return zero, geom.ErrInvalid
	}
	w, ok := r.native.Face.Surface.(*geom.WrappedSurfacePatch)
	if !ok {
		return zero, geom.ErrConvergence
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxPoles == 0 {
		o.MaxPoles = 4096
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxPoles < 4 {
		return zero, geom.ErrInvalid
	}
	used := 0
	check := func(q *big.Rat) error {
		if q == nil {
			return geom.ErrInvalid
		}
		used++
		if used > o.MaxOperations || q.Num().BitLen() > o.MaxCoefficientBits || q.Denom().BitLen() > o.MaxCoefficientBits {
			return geom.ErrConvergence
		}
		return ctx.Err()
	}
	for _, q := range uv {
		if e := check(q); e != nil {
			return zero, e
		}
	}
	if used >= o.MaxOperations {
		return zero, geom.ErrConvergence
	}
	limits := o
	limits.MaxOperations -= used
	embedding, e := geom.CertifyWrappedCylinderEmbedding(ctx, w, limits)
	if e != nil {
		return zero, e
	}
	if !embedding.Complete {
		return zero, geom.ErrConvergence
	}
	used += embedding.ExactOperations
	if used >= o.MaxOperations {
		return zero, geom.ErrConvergence
	}
	jet, e := w.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations - used, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return zero, e
	}
	used += jet.ExactOperations
	result := WrappedFacePointLocation{SuppliedUV: [2]*big.Rat{new(big.Rat).Set(uv[0]), new(big.Rat).Set(uv[1])}, Point: jet.Point, Location: trimvalidate.Outside}
	var hull [2][2]*big.Rat
	for _, loop := range r.loops {
		for _, q := range loop {
			for _, h := range q.control {
				for axis := 0; axis < 2; axis++ {
					value := new(big.Rat).Quo(h[axis], h[2])
					if e := check(value); e != nil {
						return zero, e
					}
					if hull[axis][0] == nil || value.Cmp(hull[axis][0]) < 0 {
						hull[axis][0] = value
					}
					if hull[axis][1] == nil || value.Cmp(hull[axis][1]) > 0 {
						hull[axis][1] = value
					}
				}
			}
		}
	}
	period := w.Periods()[0]
	if period == nil || new(big.Rat).Sub(hull[0][1], hull[0][0]).Cmp(period) >= 0 {
		return zero, geom.ErrConvergence
	}
	quotient := new(big.Rat).Quo(new(big.Rat).Sub(hull[0][0], uv[0]), period)
	if e := check(quotient); e != nil {
		return zero, e
	}
	k, rem := new(big.Int), new(big.Int)
	k.QuoRem(quotient.Num(), quotient.Denom(), rem)
	if rem.Sign() > 0 {
		k.Add(k, big.NewInt(1))
	}
	u := new(big.Rat).Add(uv[0], new(big.Rat).Mul(new(big.Rat).SetInt(k), period))
	if e := check(u); e != nil {
		return zero, e
	}
	if u.Cmp(hull[0][1]) <= 0 && uv[1].Cmp(hull[1][0]) >= 0 && uv[1].Cmp(hull[1][1]) <= 0 {
		result.MatchedLift = true
		result.FaceUV = [2]*big.Rat{u, new(big.Rat).Set(uv[1])}
		result.PeriodShift = [2]*big.Int{k, new(big.Int)}
		loops := make([]trimvalidate.Loop, len(r.loops))
		for li, loop := range r.loops {
			for _, q := range loop {
				segment, e := trimvalidate.ExactQuadratic(q.HomogeneousControls())
				if e != nil {
					return zero, e
				}
				loops[li] = append(loops[li], segment)
			}
		}
		if used >= o.MaxOperations {
			return zero, geom.ErrConvergence
		}
		location, report, e := trimvalidate.LocateRationalInLoops(ctx, loops[0], loops[1:], result.FaceUV, trimvalidate.Options{MaxEdges: 32, MaxWork: o.MaxOperations - used, MaxBits: o.MaxCoefficientBits})
		if e != nil {
			return zero, e
		}
		used += report.Work
		if report.Status != trimvalidate.Certified || location == trimvalidate.Unknown {
			return zero, geom.ErrConvergence
		}
		result.Location = location
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	result.ExactOperations = used
	result.Complete = true
	return result, nil
}

// ClassifyCylinderRationalPoint independently corroborates the supplied model
// point against the exact original UV witness before returning membership.
// A mismatching witness is ErrDomain, never a rounded point-on-face claim.
func (r *WrappedQuadraticFace) ClassifyCylinderRationalPoint(ctx context.Context, point [3]*big.Rat, uv [2]*big.Rat, o geom.SurfacePatchOptions) (WrappedFacePointLocation, error) {
	var zero WrappedFacePointLocation
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations <= 3 || o.MaxCoefficientBits < 64 {
		return zero, geom.ErrInvalid
	}
	for _, q := range point {
		if q == nil {
			return zero, geom.ErrInvalid
		}
		if q.Num().BitLen() > o.MaxCoefficientBits || q.Denom().BitLen() > o.MaxCoefficientBits {
			return zero, geom.ErrConvergence
		}
	}
	o.MaxOperations -= 3
	result, e := r.LocateCylinderRationalUV(ctx, uv, o)
	if e != nil {
		return zero, e
	}
	for i, q := range point {
		if q.Cmp(result.Point[i]) != 0 {
			return zero, geom.ErrDomain
		}
	}
	result.ExactOperations += 3
	return result, nil
}
