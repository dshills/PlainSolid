package geom

import "context"

// ClipLineUnion returns the exact closed set A union B along a bounded raw
// line. Supports hidden by the strict interior of the other operand have no
// mask. At coincident constraints, masks describe source incidence; they do
// not assert an exposed manifold boundary (e.g. two abutting cap faces).
func (s CylinderPairCommon) ClipLineUnion(ctx context.Context, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions) (CylinderMaterialLine, error) {
	return s.clipLineSet(ctx, origin, direction, minT, maxT, o, false)
}

// ClipLineClosedDifference returns A minus the open interior of B. In
// particular A==B retains the complete common boundary. This closed-set
// operation is deliberately distinct from regularized solid difference,
// which could discard these lower-dimensional remnants. Points and boundary
// intervals are retained with exact source support masks.
func (s CylinderPairCommon) ClipLineClosedDifference(ctx context.Context, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions) (CylinderMaterialLine, error) {
	return s.clipLineSet(ctx, origin, direction, minT, maxT, o, true)
}

func (s CylinderPairCommon) clipLineSet(ctx context.Context, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions, difference bool) (CylinderMaterialLine, error) {
	classify := func(signs []int) (BorePointLocation, BoreBoundaryMask) {
		state := [2]BorePointLocation{BoreInside, BoreInside}
		masks := [2]BoreBoundaryMask{}
		for i := 0; i < 2; i++ {
			for j := 0; j < 3; j++ {
				k := 3*i + j
				if signs[k] > 0 {
					state[i] = BoreOutside
				}
				if signs[k] == 0 {
					masks[i] |= 1 << uint(k)
				}
			}
			if state[i] != BoreOutside && masks[i] != 0 {
				state[i] = BoreBoundary
			}
		}
		if difference {
			if state[0] == BoreOutside || state[1] == BoreInside {
				return BoreOutside, 0
			}
			mask := masks[0]
			if state[1] == BoreBoundary {
				mask |= masks[1]
			}
			return BoreBoundary, mask
		}
		if state[0] == BoreInside || state[1] == BoreInside {
			return BoreBoundary, 0
		}
		if state[0] == BoreOutside && state[1] == BoreOutside {
			return BoreOutside, 0
		}
		var mask BoreBoundaryMask
		for i := 0; i < 2; i++ {
			if state[i] == BoreBoundary {
				mask |= masks[i]
			}
		}
		return BoreBoundary, mask
	}
	r, e := s.intersectLine(ctx, origin, direction, minT, maxT, o, classify)
	if e != nil {
		return CylinderMaterialLine{}, e
	}
	return CylinderMaterialLine{Intervals: r.Intervals, Points: r.Hits}, nil
}
