package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// AlgebraicShellMassBounds encloses the unit-density volume and first
// moments in world coordinates. Inertia is about the world origin at unit
// density, with negative products off the diagonal. Centroid is provided
// only when the lower volume bound is positive.
type AlgebraicShellMassBounds struct {
	AlgebraicShellVolumeBounds
	FirstMoment, Centroid [3]geom.ExactParameterInterval
	Inertia               [3][3]geom.ExactParameterInterval
}

// BoundMassProperties uses the original oriented, trimmed spline supports
// and the flux form of the divergence theorem. Unclassified face cells
// contribute conservatively; no tessellation is used.
func (s AlgebraicShellBoolean) BoundMassProperties(ctx context.Context, o AlgebraicShellMeasureOptions) (out AlgebraicShellMassBounds, err error) {
	moment := [10][2]int{{-1, -1}, {0, -1}, {1, -1}, {2, -1}, {0, 0}, {1, 1}, {2, 2}, {0, 1}, {0, 2}, {1, 2}}
	var flux [len(moment)]geom.ExactParameterInterval
	for i := range flux {
		flux[i] = geom.ExactParameterInterval{Lower: new(big.Rat), Upper: new(big.Rat)}
	}
	var origin geom.Vec3
	counts, e := s.visitBooleanSurfaceCells(ctx, o, func(p *geom.SurfacePatch, at geom.Vec3, partial, reversed bool) error {
		origin = at
		for i, axes := range moment {
			lo, hi, e := booleanCellFlux(ctx, p, origin, partial, axes[0], axes[1], o.Shells.Embedding.Patches)
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
	worldOrigin := [3]*big.Rat{ratInset(origin.X), ratInset(origin.Y), ratInset(origin.Z)}
	for axis, x := range worldOrigin {
		a := flux[axis+1]
		out.FirstMoment[axis] = addBooleanMeasureIntervals(a, scaleBooleanMeasureInterval(out.Volume, x))
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
	var diagonal [3]geom.ExactParameterInterval
	for id, axes := range moment[4:] {
		i, j := axes[0], axes[1]
		x, y := worldOrigin[i], worldOrigin[j]
		// Move the certified local raw second moment to world coordinates:
		// Q_ij = Q'_ij + o_i M'_j + o_j M'_i + o_i o_j V.
		q := addBooleanMeasureIntervals(flux[4+id], scaleBooleanMeasureInterval(flux[1+i], y))
		q = addBooleanMeasureIntervals(q, scaleBooleanMeasureInterval(flux[1+j], x))
		q = addBooleanMeasureIntervals(q, scaleBooleanMeasureInterval(out.Volume, new(big.Rat).Mul(x, y)))
		if i == j {
			diagonal[i] = q
		} else {
			out.Inertia[i][j] = scaleBooleanMeasureInterval(q, big.NewRat(-1, 1))
			out.Inertia[j][i] = geom.ExactParameterInterval{Lower: new(big.Rat).Set(out.Inertia[i][j].Lower), Upper: new(big.Rat).Set(out.Inertia[i][j].Upper)}
		}
	}
	for i := range diagonal {
		out.Inertia[i][i] = addBooleanMeasureIntervals(diagonal[(i+1)%3], diagonal[(i+2)%3])
		if out.Inertia[i][i].Lower.Sign() < 0 {
			out.Inertia[i][i].Lower.SetInt64(0)
		}
	}
	return out, nil
}

func addBooleanMeasureIntervals(a, b geom.ExactParameterInterval) geom.ExactParameterInterval {
	return geom.ExactParameterInterval{Lower: new(big.Rat).Add(a.Lower, b.Lower), Upper: new(big.Rat).Add(a.Upper, b.Upper)}
}

func scaleBooleanMeasureInterval(a geom.ExactParameterInterval, s *big.Rat) geom.ExactParameterInterval {
	lo, hi := new(big.Rat).Mul(a.Lower, s), new(big.Rat).Mul(a.Upper, s)
	if s.Sign() < 0 {
		lo, hi = hi, lo
	}
	return geom.ExactParameterInterval{Lower: lo, Upper: hi}
}
