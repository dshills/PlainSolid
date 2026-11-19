package trimvalidate

import "runtime"

// coordinateHullSeparated uses only exact comparisons of original Euclidean
// controls. Positive rational weights, proved before this predicate, put the
// entire curve in their convex hull. A STRICT coordinate interval gap proves
// disjointness without constructing any separating-axis dot products. Touching
// boxes deliberately fall through to the existing endpoint/contact proof.
// The caller has already charged its ordinary pair work unit; this changes
// neither work accounting nor subdivision/cancellation limits.
func coordinateHullSeparated(a, b curve) bool {
	ap, bp := a.points(), b.points()
	for axis := 0; axis < 2; axis++ {
		alo, ahi := ap[0][axis], ap[0][axis]
		blo, bhi := bp[0][axis], bp[0][axis]
		for _, p := range ap[1:] {
			if p[axis].Cmp(alo) < 0 {
				alo = p[axis]
			}
			if p[axis].Cmp(ahi) > 0 {
				ahi = p[axis]
			}
			runtime.KeepAlive(&p) // golang/go#82026: keeps p in the stack map. Remove once the Go fix is in use.
		}
		for _, p := range bp[1:] {
			if p[axis].Cmp(blo) < 0 {
				blo = p[axis]
			}
			if p[axis].Cmp(bhi) > 0 {
				bhi = p[axis]
			}
			runtime.KeepAlive(&p)
		}
		if ahi.Cmp(blo) < 0 || bhi.Cmp(alo) < 0 {
			return true
		}
	}
	return false
}
