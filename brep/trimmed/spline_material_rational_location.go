//go:build research

package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// LocateRationalSplineMaterialPoint classifies an exact model-space point
// against valid oriented spline material, including inward cavity shells.
// It proves original face membership or counts certified oriented crossings
// from the exact point. Inconclusive cases return an error.
func (s Solid) LocateRationalSplineMaterialPoint(ctx context.Context, point [3]*big.Rat, o SplineShellBooleanOptions) (parametric.Location, error) {
	bits := o.Ray.Line.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	if bits < 64 {
		return parametric.Outside, geom.ErrInvalid
	}
	for _, q := range point {
		if q == nil {
			return parametric.Outside, geom.ErrInvalid
		}
		if q.Num().BitLen() > bits || q.Denom().BitLen() > bits {
			return parametric.Outside, geom.ErrConvergence
		}
	}
	bounds, e := s.splineMaterialPointBounds(ctx, o)
	if e != nil || len(s.faces) == 0 {
		return parametric.Outside, e
	}
	lo, hi := affineMaterialXYZ(bounds.Min), affineMaterialXYZ(bounds.Max)
	for i, q := range point {
		if q.Cmp(lo[i]) < 0 || q.Cmp(hi[i]) > 0 {
			return parametric.Outside, nil
		}
	}
	boundary, e := s.locateSplineMaterialBoundary(ctx, point, o)
	if e != nil {
		return parametric.Outside, e
	}
	if boundary {
		return parametric.Boundary, nil
	}
	if o.MaxRays == 0 {
		o.MaxRays = len(splineWitnessDirections)
	}
	for _, d := range splineWitnessDirections[:o.MaxRays] {
		if e := ctx.Err(); e != nil {
			return parametric.Outside, e
		}
		direction := affineMaterialXYZ(d)
		end := rationalSplineWitnessFarParameter(point, direction, lo, hi)
		if !finite(end) {
			continue
		}
		o.Ray.Line.MinParameter, o.Ray.Line.MaxParameter = 0, end
		r, e := s.SplineRayRational(ctx, point, direction, o.Ray)
		if e != nil && !errors.Is(e, geom.ErrConvergence) {
			return parametric.Outside, e
		}
		if inside, known := splineMaterialRayWinding(r, end); known {
			if inside {
				return parametric.Inside, nil
			}
			return parametric.Outside, nil
		}
	}
	return parametric.Outside, fmt.Errorf("%w: no transverse rational spline material ray", geom.ErrConvergence)
}

func rationalSplineWitnessFarParameter(point, direction, lo, hi [3]*big.Rat) float64 {
	best := math.Inf(1)
	unit := big.NewRat(1, 1)
	for i, d := range direction {
		if d.Sign() == 0 {
			continue
		}
		end := hi[i]
		if d.Sign() < 0 {
			end = lo[i]
		}
		exit := new(big.Rat).Quo(new(big.Rat).Sub(end, point[i]), d)
		margin := exit
		if margin.Cmp(unit) < 0 {
			margin = unit
		}
		if r, e := geom.RationalOutwardRange(new(big.Rat).Add(exit, margin)); e == nil {
			best = min(best, r.Max)
		}
	}
	return best
}
