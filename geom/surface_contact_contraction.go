package geom

import (
	"context"
	"math"
)

// contractSurfaceContactToTubes uses Krawczyk only as an all-roots enclosure,
// never as an existence claim. Inclusive containment covers boundary roots that
// strict separating-hull exclusion must retain. Tube proofs are established by
// the caller before this internal helper is used.
func contractSurfaceContactToTubes(ctx context.Context, a, b *SurfacePatch, box SurfacePairParameterBox, tubes []SurfacePairTubeRegion, maxOperations, maxBits int) (covered, excluded bool, contracted SurfacePairParameterBox, changed bool, operations int, err error) {
	contracted = box
	current := pairParameterRanges(box)
	contained := func(r [4][2]float64) bool {
		for _, tube := range tubes {
			target := pairParameterRanges(tube.Box)
			inside := true
			for axis := 0; axis < 4; axis++ {
				if r[axis][0] < target[axis][0] || r[axis][1] > target[axis][1] {
					inside = false
					break
				}
			}
			if inside {
				return true
			}
		}
		return false
	}
	if contained(current) {
		return true, false, box, false, 0, nil
	}
	var axes []int
	var used [4]bool
	for _, tube := range tubes {
		target := pairParameterRanges(tube.Box)
		overlaps := true
		for axis := 0; axis < 4; axis++ {
			if current[axis][1] < target[axis][0] || target[axis][1] < current[axis][0] {
				overlaps = false
				break
			}
		}
		if overlaps && !used[tube.FreeAxis] {
			axes = append(axes, tube.FreeAxis)
			used[tube.FreeAxis] = true
		}
	}
	if len(axes) == 0 {
		return
	}
	if a.countU != a.degreeU+1 || a.countV != a.degreeV+1 || b.countU != b.degreeU+1 || b.countV != b.degreeV+1 {
		return
	}
	for axis := 0; axis < 4; axis++ {
		if !used[axis] {
			axes = append(axes, axis)
		}
	}
	next, empty, work, e := weakSurfaceContactBounds(a, b, box, maxOperations, maxBits)
	operations += work
	if e != nil {
		err = e
		return
	}
	if empty {
		return false, true, box, false, operations, nil
	}
	if contained(next) {
		return true, false, box, false, operations, nil
	}
	// Exact common-chart boundary confinement can retain a shared endpoint
	// even when outward Krawczyk conversion leaves an adjacent one-ULP slab.
	if a.Domain() == b.Domain() {
		bound, applied, empty, work, e := commonXYBoundaryContactBounds(ctx, a, b, maxOperations-operations, maxBits)
		operations += work
		if e != nil {
			err = e
			return
		}
		if applied && empty {
			return false, true, box, false, operations, nil
		}
		if applied && contained(bound) {
			return true, false, box, false, operations, nil
		}
	}
	for _, axis := range axes {
		if e := ctx.Err(); e != nil {
			err = e
			return
		}
		remaining := maxOperations - operations
		if remaining < 1 {
			err = ErrConvergence
			return
		}
		certificate, e := CertifySurfacePairTube(a, b, SurfacePairTubeOptions{FreeAxis: axis, MaxOperations: remaining, MaxCoefficientBits: maxBits})
		operations += certificate.Test.ExactOperations
		if e != nil {
			err = e
			return
		}
		if certificate.ExcludesContact {
			return false, true, box, false, operations, nil
		}
		if !certificate.Test.CenterInvertible {
			continue
		}
		for j, dependent := range certificate.Test.ActiveAxes {
			bounds := certificate.Test.KrawczykBounds[j]
			next[dependent] = [2]float64{max(next[dependent][0], bounds.Min), min(next[dependent][1], bounds.Max)}
			if next[dependent][0] > next[dependent][1] {
				return false, true, box, false, operations, nil
			}
		}
		if contained(next) {
			return true, false, box, false, operations, nil
		}
	}
	useful := false
	for axis, pair := range next {
		lo, hi := pair[0], pair[1]
		if lo == hi {
			lo = max(current[axis][0], math.Nextafter(lo, math.Inf(-1)))
			hi = min(current[axis][1], math.Nextafter(hi, math.Inf(1)))
		}
		if lo >= hi {
			return false, false, box, false, operations, nil
		}
		next[axis] = [2]float64{lo, hi}
		if hi-lo < .9*(current[axis][1]-current[axis][0]) {
			useful = true
		}
	}
	if useful {
		return false, false, rangesParameterPair(next), true, operations, nil
	}
	return
}
