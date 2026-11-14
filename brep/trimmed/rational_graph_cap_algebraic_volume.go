package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// BoundVerticalPolynomialVolume certifies the retained volume by exact
// divergence flux. The two cap planes must be graphs over XY; every retained
// source chart must have identically zero XY-projected Jacobian. Polynomial
// cap boundaries and stock charts are required. Other shell families remain
// unresolved by this specialized measure, even if their topology is closed.
func (c AlgebraicGraphCapPair) BoundVerticalPolynomialVolume(ctx context.Context, width *big.Rat,
	o RationalGraphShellSectionOptions, roots geom.ExactCurvePlaneEventOptions) (geom.ExactParameterInterval, error) {
	var zero geom.ExactParameterInterval
	if ctx == nil || width == nil || width.Sign() <= 0 || roots.MaxOperations < 0 || roots.MaxSubdivisions < 0 || roots.MaxCoefficientBits < 0 {
		return zero, geom.ErrInvalid
	}
	if roots.MaxOperations == 0 {
		roots.MaxOperations = 2000000
	}
	if roots.MaxSubdivisions == 0 {
		roots.MaxSubdivisions = 8192
	}
	if roots.MaxCoefficientBits == 0 {
		roots.MaxCoefficientBits = 16384
	}
	if roots.MaxCoefficientBits < 64 || width.Num().BitLen() > roots.MaxCoefficientBits || width.Denom().BitLen() > roots.MaxCoefficientBits {
		return zero, geom.ErrInvalid
	}
	exact, e := c.ExactIndexedBoundary(ctx, o, roots)
	if e != nil {
		return zero, e
	}
	b := capClipBudget{ctx, roots.MaxOperations, roots.MaxSubdivisions, roots.MaxCoefficientBits}
	if e := c.proveZeroStockXYFlux(exact, o, &b); e != nil {
		return zero, e
	}
	n := 0
	for _, cap := range c.caps {
		for _, ring := range cap.wires {
			n += len(ring)
		}
	}
	if n < 1 {
		return zero, geom.ErrConvergence
	}
	part := new(big.Rat).Quo(width, big.NewRat(int64(n), 1))
	lo, hi := new(big.Rat), new(big.Rat)
	for _, cap := range c.caps {
		plane := cap.regions.plane
		if plane.dropped != 2 {
			return zero, geom.ErrUnrepresentable
		}
		// Chart order is physical orientation: a downward cap projects
		// in YX order, with the corresponding opposite physical flux.
		axes := orientedRationalGraphProjection(plane)
		affine := [3]*big.Rat{plane.offset, new(big.Rat).Neg(plane.normal[axes[0]]), new(big.Rat).Neg(plane.normal[axes[1]])}
		for _, ring := range cap.wires {
			for _, use := range ring {
				flux, e := use.Cell.BoundPolynomialProjectedAffineGreen(ctx, axes, affine, part, b.limits())
				if e != nil {
					return zero, e
				}
				if e := b.charge(flux.Operations, flux.Subdivisions); e != nil {
					return zero, e
				}
				if use.Reversed != (plane.orientation < 0) {
					lo.Sub(lo, flux.Interval.Upper)
					hi.Sub(hi, flux.Interval.Lower)
				} else {
					lo.Add(lo, flux.Interval.Lower)
					hi.Add(hi, flux.Interval.Upper)
				}
				if lo.Num().BitLen() > b.bits || lo.Denom().BitLen() > b.bits || hi.Num().BitLen() > b.bits || hi.Denom().BitLen() > b.bits {
					return zero, geom.ErrConvergence
				}
			}
		}
	}
	if lo.Sign() < 0 {
		lo.SetInt64(0)
	}
	if hi.Cmp(lo) < 0 || new(big.Rat).Sub(hi, lo).Cmp(width) > 0 {
		return zero, geom.ErrConvergence
	}
	return geom.ExactParameterInterval{Lower: lo, Upper: hi}, ctx.Err()
}

func (c AlgebraicGraphCapPair) proveZeroStockXYFlux(exact AlgebraicGraphShellBoundary, o RationalGraphShellSectionOptions, b *capClipBudget) error {
	for _, f := range exact.Faces {
		if f.StockFace < 0 {
			continue
		}
		face := c.caps[0].recipe.stock.faces[f.StockFace]
		patch, e := exactFaceSupportPatch(face, o.Spans.Faces.Patches)
		if e != nil {
			return e
		}
		uv, e := faceUVBounds(face)
		if e != nil {
			return e
		}
		patch, e = patch.Trim(uv)
		if e != nil {
			return e
		}
		zeroFlux, used, e := patch.ProjectedJacobianZero(b.ctx, [2]int{0, 1}, geom.SurfacePatchEvaluationOptions{MaxOperations: b.work, MaxCoefficientBits: b.bits})
		if e != nil {
			return e
		}
		if !zeroFlux {
			return geom.ErrUnrepresentable
		}
		if e := b.charge(used, 0); e != nil {
			return e
		}
	}
	return nil
}
