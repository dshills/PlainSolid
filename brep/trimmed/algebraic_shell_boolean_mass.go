package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// AlgebraicShellMassBounds encloses the unit-density volume and first
// moments in world coordinates. Centroid is provided only when the
// lower volume bound is positive.
type AlgebraicShellMassBounds struct {
	AlgebraicShellVolumeBounds
	FirstMoment, Centroid [3]geom.ExactParameterInterval
}

// BoundMassProperties uses the original oriented, trimmed spline supports
// and the flux form of the divergence theorem. Unclassified face cells
// contribute conservatively; no tessellation is used. Inertia is not part
// of this report.
func (s AlgebraicShellBoolean) BoundMassProperties(ctx context.Context, o AlgebraicShellMeasureOptions) (out AlgebraicShellMassBounds, err error) {
	var flux [4]geom.ExactParameterInterval
	for i := range flux {
		flux[i] = geom.ExactParameterInterval{Lower: new(big.Rat), Upper: new(big.Rat)}
	}
	var origin geom.Vec3
	counts, e := s.visitBooleanSurfaceCells(ctx, o, func(p *geom.SurfacePatch, at geom.Vec3, partial, reversed bool) error {
		origin = at
		for i := range flux {
			lo, hi, e := booleanCellFlux(ctx, p, origin, partial, i-1, o.Shells.Embedding.Patches)
			if e != nil {
				return e
			}
			if reversed {
				lo, hi = hi.Neg(hi), lo.Neg(lo)
			}
			flux[i].Lower.Add(flux[i].Lower, lo)
			flux[i].Upper.Add(flux[i].Upper, hi)
		}
		return nil
	})
	out.VisitedCells, out.AmbiguousCells = counts.VisitedCells, counts.AmbiguousCells
	if e != nil {
		return out, e
	}
	out.Volume = flux[0]
	for axis, c := range [3]float64{origin.X, origin.Y, origin.Z} {
		x := ratInset(c)
		lo, hi := new(big.Rat).Mul(x, out.Volume.Lower), new(big.Rat).Mul(x, out.Volume.Upper)
		if x.Sign() < 0 {
			lo, hi = hi, lo
		}
		a := flux[axis+1]
		out.FirstMoment[axis] = geom.ExactParameterInterval{Lower: lo.Add(lo, a.Lower), Upper: hi.Add(hi, a.Upper)}
		if out.Volume.Lower.Sign() > 0 {
			// Center from the tighter local moment before translating.
			var ratio geom.ExactParameterInterval
			for _, n := range [2]*big.Rat{a.Lower, a.Upper} {
				for _, d := range [2]*big.Rat{out.Volume.Lower, out.Volume.Upper} {
					v := new(big.Rat).Quo(n, d)
					if ratio.Lower == nil || v.Cmp(ratio.Lower) < 0 {
						ratio.Lower = v
					}
					if ratio.Upper == nil || v.Cmp(ratio.Upper) > 0 {
						ratio.Upper = v
					}
				}
			}
			out.Centroid[axis] = geom.ExactParameterInterval{Lower: new(big.Rat).Add(ratio.Lower, x), Upper: new(big.Rat).Add(ratio.Upper, x)}
		}
	}
	return out, nil
}
