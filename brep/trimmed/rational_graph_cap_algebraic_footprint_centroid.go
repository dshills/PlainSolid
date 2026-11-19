//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// AlgebraicGraphFootprintCentroid reports the projected area's centroid
// embedded in the local XY plane or in that same plane after placement.
type AlgebraicGraphFootprintCentroid struct {
	Moments                       AlgebraicGraphFootprintMoments
	Coordinates                   [3]geom.ExactParameterInterval
	ExactOperations, Subdivisions int
}

// BoundFootprintCentroid encloses each coordinate of the original
// footprint centroid with the requested width. Its local Z is zero.
func (m AlgebraicGraphVerticalMaterial) BoundFootprintCentroid(ctx context.Context, width *big.Rat,
	opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphFootprintCentroid, error) {
	q := big.NewRat
	return m.boundFootprintCentroid(ctx, width, [3][4]*big.Rat{
		{q(1, 1), q(0, 1), q(0, 1), q(0, 1)}, {q(0, 1), q(1, 1), q(0, 1), q(0, 1)}, {q(0, 1), q(0, 1), q(1, 1), q(0, 1)},
	}, opts)
}

// BoundFootprintCentroid places the original XY footprint centroid in
// world space. This is the centroid of an orthogonal projection along the
// placed stock axis; it is independent of the original caps' heights.
func (o AlgebraicGraphStockOccurrence) BoundFootprintCentroid(ctx context.Context, width *big.Rat,
	opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphFootprintCentroid, error) {
	p, e := o.pose.Matrix()
	if e != nil {
		return AlgebraicGraphFootprintCentroid{}, e
	}
	return o.model.boundFootprintCentroid(ctx, width, p, opts)
}

func (m AlgebraicGraphVerticalMaterial) boundFootprintCentroid(ctx context.Context, width *big.Rat,
	p [3][4]*big.Rat, opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphFootprintCentroid, error) {
	var zero AlgebraicGraphFootprintCentroid
	b, e := m.extremaBudget(ctx, opts)
	if e != nil {
		return zero, e
	}
	if width == nil || width.Sign() <= 0 {
		return zero, geom.ErrInvalid
	}
	widthOK := func(w *big.Rat) bool {
		return max(w.Num().BitLen(), w.Denom().BitLen()) <= b.bits
	}
	if !widthOK(width) {
		return zero, geom.ErrConvergence
	}
	maxWork, maxSplits := b.work, b.subdiv
	w := new(big.Rat).Set(width)
	for attempts := 0; attempts < 64; attempts++ {
		if !widthOK(w) {
			return zero, geom.ErrConvergence
		}
		v, e := m.BoundFootprintMoments(ctx, w, b.limits())
		if e != nil {
			return zero, e
		}
		if e = b.charge(v.ExactOperations, v.Subdivisions); e != nil {
			return zero, e
		}
		if v.Area.Lower.Sign() > 0 {
			var centroid [2]geom.ExactParameterInterval
			for i, n := range v.First {
				var lo, hi *big.Rat
				for _, a := range []*big.Rat{n.Lower, n.Upper} {
					for _, z := range []*big.Rat{v.Area.Lower, v.Area.Upper} {
						r := new(big.Rat).Quo(a, z)
						if lo == nil || r.Cmp(lo) < 0 {
							lo = r
						}
						if hi == nil || r.Cmp(hi) > 0 {
							hi = r
						}
					}
				}
				centroid[i] = geom.ExactParameterInterval{Lower: lo, Upper: hi}
				if !graphStockMomentFits(centroid[i], b.bits) {
					return zero, geom.ErrConvergence
				}
			}
			out := AlgebraicGraphFootprintCentroid{Moments: v}
			converged := true
			for i, axis := range p {
				lo, hi := new(big.Rat).Set(axis[3]), new(big.Rat).Set(axis[3])
				for j, c := range centroid {
					a, z := c.Lower, c.Upper
					if axis[j].Sign() < 0 {
						a, z = z, a
					}
					lo.Add(lo, new(big.Rat).Mul(axis[j], a))
					hi.Add(hi, new(big.Rat).Mul(axis[j], z))
				}
				out.Coordinates[i] = geom.ExactParameterInterval{Lower: lo, Upper: hi}
				if !graphStockMomentFits(out.Coordinates[i], b.bits) {
					return zero, geom.ErrConvergence
				}
				converged = converged && new(big.Rat).Sub(hi, lo).Cmp(width) <= 0
			}
			if e = b.charge(48, 0); e != nil {
				return zero, e
			}
			if converged {
				out.ExactOperations, out.Subdivisions = maxWork-b.work, maxSplits-b.subdiv
				return out, nil
			}
		} else if v.Area.Upper.Sign() <= 0 {
			return zero, geom.ErrConvergence
		}
		if e := b.charge(1, 0); e != nil {
			return zero, e
		}
		w.Quo(w, big.NewRat(2, 1))
	}
	return zero, geom.ErrConvergence
}
