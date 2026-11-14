package scene

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/geom"
)

// SideOpenDualDiskBoundary is implemented by the native equal, unequal and
// external-radical cut results. The query must retain exact original line units.
type SideOpenDualDiskBoundary interface {
	IntersectLineBoundary(context.Context, [3]*big.Rat, [3]*big.Rat, *big.Rat, *big.Rat, geom.ToroidLineOptions) (geom.BoxDualCircleBoundaryLine, error)
}

// SideOpenDualDiskPickItem identifies one exact part occurrence. Translation
// is applied in exact represented coordinates to the finite camera ray.
type SideOpenDualDiskPickItem struct {
	ID          string
	Boundary    SideOpenDualDiskBoundary
	Translation geom.Vec3
}

// SideOpenDualDiskPick reports the original exposed supports at the first
// hit, including simultaneous cylindrical or stock and floor incidences.
// Distance is measured from the represented camera near plane.
type SideOpenDualDiskPick struct {
	ID         string
	Parameter  *geom.ExactAlgebraicRoot
	Distance   geom.ExactParameterInterval
	Point      [3]geom.ExactParameterInterval
	Support    geom.BoxDualCircleSupport
	Coincident bool
}

// PickSideOpenDualDiskCuts orders the exact first contacts over all visible
// occurrences. Equal algebraic depths use lexicographic ID order. An
// unproved contact or depth ordering fails atomically, with no mesh fallback.
func PickSideOpenDualDiskCuts(ctx context.Context, camera Camera, screen geom.Vec2, items []SideOpenDualDiskPickItem, o geom.ToroidLineOptions) (SideOpenDualDiskPick, bool, error) {
	var zero SideOpenDualDiskPick
	if ctx == nil || len(items) > 1024 || o.ParameterTolerance < 0 || !finite(o.ParameterTolerance) {
		return zero, false, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, false, e
	}
	ray, e := camera.Ray(screen)
	if e != nil {
		return zero, false, e
	}
	if o.ParameterTolerance == 0 {
		o.ParameterTolerance = 1e-10
	}
	limits := geom.ExactCurvePlaneEventOptions{MaxOperations: o.Surface.MaxOperations, MaxCoefficientBits: o.Surface.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	a := [3]*big.Rat{rat(ray.Origin.X), rat(ray.Origin.Y), rat(ray.Origin.Z)}
	d := [3]*big.Rat{rat(ray.Direction.X), rat(ray.Direction.Y), rat(ray.Direction.Z)}
	ends := [2]*big.Rat{new(big.Rat), rat(ray.Length)}
	ids := map[string]bool{}
	best := zero
	for _, item := range items {
		if item.ID == "" || ids[item.ID] || item.Boundary == nil || !item.Translation.IsFinite() {
			return zero, false, geom.ErrInvalid
		}
		ids[item.ID] = true
		x := [3]*big.Rat{new(big.Rat).Sub(a[0], rat(item.Translation.X)), new(big.Rat).Sub(a[1], rat(item.Translation.Y)), new(big.Rat).Sub(a[2], rat(item.Translation.Z))}
		hits, err := item.Boundary.IntersectLineBoundary(ctx, x, d, ends[0], ends[1], o)
		if err != nil {
			return zero, false, fmt.Errorf("scene: side opening %q: %w", item.ID, err)
		}
		var candidate SideOpenDualDiskPick
		consider := func(p *geom.ExactAlgebraicRoot, support geom.BoxDualCircleSupport, coincident bool) error {
			if p == nil || support == 0 {
				return geom.ErrInvalid
			}
			if candidate.Parameter == nil {
				candidate = SideOpenDualDiskPick{ID: item.ID, Parameter: p, Support: support, Coincident: coincident}
				return nil
			}
			order, e := p.Compare(ctx, candidate.Parameter, limits)
			if e != nil {
				return e
			}
			if !order.Proven {
				return geom.ErrConvergence
			}
			if order.Order < 0 {
				candidate.Parameter, candidate.Support, candidate.Coincident = p, support, coincident
			} else if order.Order == 0 {
				candidate.Support |= support
				candidate.Coincident = candidate.Coincident || coincident
			}
			return nil
		}
		// Each sequence is independently ordered. Only its first hit or
		// first interval start can be the first contact for this occurrence.
		if len(hits.Hits) > 0 {
			h := hits.Hits[0]
			if e := consider(h.Parameter, h.Support, false); e != nil {
				return zero, false, e
			}
		}
		if len(hits.Intervals) > 0 {
			h := hits.Intervals[0]
			if e := consider(h.Start, h.StartSupport|h.Support, true); e != nil {
				return zero, false, e
			}
		}
		if candidate.Parameter == nil {
			continue
		}
		if best.Parameter == nil {
			best = candidate
			continue
		}
		order, e := candidate.Parameter.Compare(ctx, best.Parameter, limits)
		if e != nil {
			return zero, false, e
		}
		if !order.Proven {
			return zero, false, geom.ErrConvergence
		}
		if order.Order < 0 || order.Order == 0 && candidate.ID < best.ID {
			best = candidate
		}
	}
	if e := ctx.Err(); e != nil {
		return zero, false, e
	}
	if best.Parameter == nil {
		return zero, false, nil
	}
	best.Distance = best.Parameter.Interval()
	for i := range best.Point {
		lo := new(big.Rat).Add(a[i], new(big.Rat).Mul(d[i], best.Distance.Lower))
		hi := new(big.Rat).Add(a[i], new(big.Rat).Mul(d[i], best.Distance.Upper))
		if lo.Cmp(hi) > 0 {
			lo, hi = hi, lo
		}
		best.Point[i] = geom.ExactParameterInterval{Lower: lo, Upper: hi}
	}
	return best, true, nil
}
