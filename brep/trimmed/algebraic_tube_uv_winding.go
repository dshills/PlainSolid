package trimmed

import (
	"context"
	"errors"
	"math/big"
	"sort"

	"plainsolid/geom"
)

// closedTubeWinding chooses a horizontal test line between two proved curve
// points. It exhausts line/branch contacts, then reads the crossing direction
// at the exact rightmost event. For a simple loop this point enters or exits
// the bounded region immediately to its left, irrespective of other crossings.
func closedTubeWinding(ctx context.Context, chart int, support *geom.SurfacePatch,
	cut []AlgebraicTubeFaceCutPiece, intervals []SplineTubeClipInterval, leftSign int,
	o geom.ExactCurvePlaneEventOptions, projection ...[2]int) (winding, operations int, err error) {
	if len(cut) != len(intervals) || leftSign == 0 {
		return 0, 0, geom.ErrConvergence
	}
	exact := func() geom.ExactCurvePlaneEventOptions {
		p := o
		p.MaxOperations -= operations
		return p
	}
	compare := func(a, z *geom.ExactAlgebraicRoot) (geom.ExactCurveEventOrder, error) {
		p := exact()
		if p.MaxOperations < 1 {
			return geom.ExactCurveEventOrder{}, geom.ErrConvergence
		}
		q, e := a.Compare(ctx, z, p)
		operations += q.Operations
		if e == nil && !q.Proven {
			e = geom.ErrConvergence
		}
		return q, e
	}
	axis := 2*chart + 1
	a := intervals[0].WitnessPoint.Parameters[axis]
	var lower, upper *big.Rat
	for _, interval := range intervals[1:] {
		q, e := compare(a, interval.WitnessPoint.Parameters[axis])
		if e != nil {
			return 0, operations, e
		}
		switch q.Order {
		case -1:
			lower, upper = q.Intervals[0][1], q.Intervals[1][0]
		case 1:
			lower, upper = q.Intervals[1][1], q.Intervals[0][0]
		}
		if lower != nil {
			break
		}
	}
	if lower == nil || lower.Cmp(upper) >= 0 {
		return 0, operations, geom.ErrConvergence
	}
	width := new(big.Rat).Sub(upper, lower)
	for _, ratio := range [][2]int64{{1, 2}, {1, 3}, {2, 3}, {1, 5}, {2, 5}, {3, 5}, {4, 5}, {1, 7}, {2, 7}, {3, 7}, {4, 7}, {5, 7}, {6, 7}} {
		v := new(big.Rat).Add(lower, new(big.Rat).Mul(width, big.NewRat(ratio[0], ratio[1])))
		contacts, used, e := tubeRayCrossingsAxis(ctx, chart, support, cut, v, 0, exact(), projection...)
		operations += used
		if errors.Is(e, geom.ErrConvergence) {
			continue
		}
		if e != nil {
			return 0, operations, e
		}
		if len(contacts) == 0 {
			continue
		}
		rightmost := contacts[0]
		for _, h := range contacts[1:] {
			q, e := compare(rightmost.Parameters[axis-1], h.Parameters[axis-1])
			if e != nil {
				return 0, operations, e
			}
			if q.Order < 0 {
				rightmost = h
			}
		}
		return -leftSign * rightmost.SideSigns[1], operations, nil
	}
	return 0, operations, geom.ErrConvergence
}

// insideClosedTubeAtRoot classifies a known exact point from a disjoint
// curve at a fixed rational V. The target loop's entire ray is exhausted.
func insideClosedTubeAtRoot(ctx context.Context, chart int, support *geom.SurfacePatch,
	cut []AlgebraicTubeFaceCutPiece, u *geom.ExactAlgebraicRoot, v *big.Rat,
	o geom.ExactCurvePlaneEventOptions) (inside bool, operations int, err error) {
	return insideClosedTubeAtAxisRoot(ctx, chart, support, cut, u, v, 0, o)
}

func insideClosedTubeAtAxisRoot(ctx context.Context, chart int, support *geom.SurfacePatch,
	cut []AlgebraicTubeFaceCutPiece, u *geom.ExactAlgebraicRoot, v *big.Rat, vary int,
	o geom.ExactCurvePlaneEventOptions, projection ...[2]int) (inside bool, operations int, err error) {
	contacts, used, e := tubeRayCrossingsAxis(ctx, chart, support, cut, v, vary, o, projection...)
	operations += used
	if e != nil {
		return false, operations, e
	}
	count := 0
	for _, h := range contacts {
		p := o
		p.MaxOperations -= operations
		if p.MaxOperations < 1 {
			return false, operations, geom.ErrConvergence
		}
		q, e := u.Compare(ctx, h.Parameters[2*chart+vary], p)
		operations += q.Operations
		if e != nil {
			return false, operations, e
		}
		if !q.Proven || q.Order == 0 {
			return false, operations, geom.ErrConvergence
		}
		if q.Order < 0 {
			count++
		}
	}
	return count%2 == 1, operations, nil
}

type tubeLoopParentCandidate struct {
	index, depth int
}

func sortedTubeParents(contains [][]bool, child int) []tubeLoopParentCandidate {
	var p []tubeLoopParentCandidate
	for i, row := range contains {
		if i == child || !row[child] {
			continue
		}
		depth := 0
		for _, c := range contains {
			if c[i] {
				depth++
			}
		}
		p = append(p, tubeLoopParentCandidate{i, depth})
	}
	sort.Slice(p, func(i, j int) bool { return p[i].depth > p[j].depth })
	return p
}

// tubeRayCrossings isolates all intersections with a rational horizontal
// chart line and requires every retained root to cross regularly. At an
// exact common junction, duplicate contacts retain one crossing, and
// inconsistent one-sided signs fail closed.
func tubeRayCrossings(ctx context.Context, chart int, support *geom.SurfacePatch,
	cut []AlgebraicTubeFaceCutPiece, v *big.Rat, o geom.ExactCurvePlaneEventOptions) (out []geom.TubeTrimContact, operations int, err error) {
	return tubeRayCrossingsAxis(ctx, chart, support, cut, v, 0, o)
}

func tubeRayCrossingsAxis(ctx context.Context, chart int, support *geom.SurfacePatch,
	cut []AlgebraicTubeFaceCutPiece, level *big.Rat, vary int, o geom.ExactCurvePlaneEventOptions, projection ...[2]int) (out []geom.TubeTrimContact, operations int, err error) {
	if o.MaxOperations < 1 {
		return nil, 0, geom.ErrConvergence
	}
	d := support.Domain()
	a, z := ratInset(d.UMin), ratInset(d.UMax)
	if vary == 1 {
		a, z = ratInset(d.VMin), ratInset(d.VMax)
	}
	m := new(big.Rat).Quo(new(big.Rat).Add(a, z), big.NewRat(2, 1))
	h := [3][3]*big.Rat{{a, level, big.NewRat(1, 1)}, {m, level, big.NewRat(1, 1)}, {z, level, big.NewRat(1, 1)}}
	if vary == 1 {
		for i := range h {
			h[i][0], h[i][1] = h[i][1], h[i][0]
		}
	}
	uv, e := exactQuadraticTrimParameterCurve(ctx, h,
		geom.SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return nil, operations, e
	}
	operations += uv.ExactOperations()
	compare := func(a, z *geom.ExactAlgebraicRoot) (int, error) {
		p := o
		p.MaxOperations -= operations
		if p.MaxOperations < 1 {
			return 0, geom.ErrConvergence
		}
		q, e := a.Compare(ctx, z, p)
		operations += q.Operations
		if e == nil && !q.Proven {
			e = geom.ErrConvergence
		}
		return q.Order, e
	}
	for _, piece := range cut {
		if e := ctx.Err(); e != nil {
			return nil, operations, e
		}
		trim := piece.Segment.TrimA()
		if chart == 1 {
			trim = piece.Segment.TrimB()
		}
		box, e := trim.BoundsUV()
		if e != nil {
			return nil, operations, e
		}
		loBound, hiBound := box.Min.Y, box.Max.Y
		if vary == 1 {
			loBound, hiBound = box.Min.X, box.Max.X
		}
		if level.Cmp(ratInset(loBound)) < 0 || level.Cmp(ratInset(hiBound)) > 0 {
			continue
		}
		left := o.MaxOperations - operations
		if left < 1 {
			return nil, operations, geom.ErrConvergence
		}
		options := geom.TubeTrimIntersectionOptions{
			SurfaceCurveAgreementOptions: geom.SurfaceCurveAgreementOptions{
				SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits},
				MaxSubdivisions:     o.MaxSubdivisions}}
		var hits geom.TubeTrimIntersection
		if len(projection) == 1 && projection[0] != [2]int{0, 1} {
			q, e2 := piece.Segment.IntersectQuadraticTrimProjected(ctx, chart, uv, projection[0], options)
			hits, e = q.TubeTrimIntersection, e2
		} else {
			hits, e = piece.Segment.IntersectQuadraticTrim(ctx, chart, uv, options)
		}
		operations += hits.ExactOperations
		if e != nil {
			return nil, operations, e
		}
		if !hits.Complete {
			return nil, operations, geom.ErrConvergence
		}
		free, _, rev, e := piece.Segment.ExactFreeDomain()
		if e != nil {
			return nil, operations, e
		}
		lo, hi := piece.FreeStart, piece.FreeEnd
		if rev {
			lo, hi = hi, lo
		}
		for _, h := range hits.Hits {
			a, e := compare(h.Parameters[free], lo)
			if e != nil {
				return nil, operations, e
			}
			z, e := compare(h.Parameters[free], hi)
			if e != nil {
				return nil, operations, e
			}
			if a < 0 || z > 0 {
				continue
			}
			if !h.Transverse || h.SideSigns[0] == 0 || h.SideSigns[0] != -h.SideSigns[1] {
				return nil, operations, geom.ErrConvergence
			}
			duplicate := false
			for _, other := range out {
				q, e := compare(h.Parameters[2*chart+vary], other.Parameters[2*chart+vary])
				if e != nil {
					return nil, operations, e
				}
				if q == 0 {
					if h.SideSigns != other.SideSigns {
						return nil, operations, geom.ErrConvergence
					}
					duplicate = true
					break
				}
			}
			if !duplicate {
				out = append(out, h)
			}
		}
	}
	return out, operations, nil
}

func classifyHoleAgainstClosedTube(ctx context.Context, chart int, support *geom.SurfacePatch,
	cut []AlgebraicTubeFaceCutPiece, uv [2]*big.Rat, o geom.ExactCurvePlaneEventOptions, projection ...[2]int) (inside bool, operations int, err error) {
	contacts, used, e := tubeRayCrossingsAxis(ctx, chart, support, cut, uv[1], 0, o, projection...)
	operations += used
	if e != nil {
		return false, operations, e
	}
	o.MaxOperations -= operations
	if o.MaxOperations < 1 {
		return false, operations, geom.ErrConvergence
	}
	u, used, e := geom.NewExactRationalRoot(ctx, uv[0], o)
	operations += used
	if e != nil {
		return false, operations, e
	}
	o.MaxOperations -= used
	count := 0
	for _, h := range contacts {
		if o.MaxOperations < 1 {
			return false, operations, geom.ErrConvergence
		}
		q, e := u.Compare(ctx, h.Parameters[2*chart], o)
		operations += q.Operations
		o.MaxOperations -= q.Operations
		if e != nil {
			return false, operations, e
		}
		if !q.Proven || q.Order == 0 {
			return false, operations, geom.ErrConvergence
		}
		if q.Order < 0 {
			count++
		}
	}
	return count%2 == 1, operations, nil
}
