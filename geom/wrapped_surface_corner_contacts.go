package geom

import (
	"context"
	"math/big"
)

// wrappedCornerContacts resolves only the fundamental tracer diagnostic for
// isolated corner points with no same-chart incident arc. Such a point adds no
// branch; exact quotient UV identity to an existing arc proves it is represented.
func wrappedCornerContacts(ctx context.Context, w *WrappedSurfacePatch, f SurfacePlaneTrace, maxComparisons int) (bool, int, error) {
	if !f.Isolation.Complete || len(f.Unresolved) == 0 {
		return false, 0, nil
	}
	for _, issue := range f.Unresolved {
		if issue.Arc != -1 || issue.Endpoint != -1 || issue.Reason != "corner-only contact has no certified incident arc" {
			return false, 0, nil
		}
	}
	d := w.Domain()
	limits := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	singleton := func(id *surfaceRootIdentity) (uv [2]*big.Rat, ok bool) {
		if id == nil || id.lo.Cmp(id.hi) != 0 {
			return uv, false
		}
		uv[id.axis] = id.fixed
		uv[1-id.axis] = id.lo
		return uv, true
	}
	count := 0
	for _, point := range f.Isolation.CornerContacts {
		a, ok := singleton(point.identity)
		if !ok {
			return false, count, nil
		}
		found := false
		for _, arc := range f.Isolation.Arcs {
			for _, endpoint := range arc.Endpoints {
				if e := ctx.Err(); e != nil {
					return false, count, e
				}
				count++
				if count > maxComparisons {
					return false, count, ErrConvergence
				}
				b, ok := singleton(endpoint.identity)
				if !ok {
					continue
				}
				same := true
				for axis := 0; axis < 2; axis++ {
					if a[axis].Cmp(b[axis]) == 0 {
						continue
					}
					if w.period[axis] == nil {
						same = false
						break
					}
					lo, hi := new(big.Rat).SetFloat64(limits[axis][0]), new(big.Rat).SetFloat64(limits[axis][1])
					if !((a[axis].Cmp(lo) == 0 && b[axis].Cmp(hi) == 0) || (a[axis].Cmp(hi) == 0 && b[axis].Cmp(lo) == 0)) {
						same = false
						break
					}
				}
				if same {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false, count, nil
		}
	}
	return true, count, nil
}
