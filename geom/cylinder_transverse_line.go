package geom

import "context"

// TransverseCylinderMaterialLine separates the regularized filled set from
// original finite support incidence. Material intervals with Boundary==0 are
// interior; nonzero masks identify exposed supports along their interiors.
// Sources includes canceled/hidden incidences, even outside retained material.
// It is not a second material classification. Original unnormalized line units
// and immutable exact root identities are preserved in both reports.
type TransverseCylinderMaterialLine struct {
	Material CylinderMaterialLine
	Sources  BoreLineBoundary
}

// ClipLine partitions by exact radial quadratic and axial linear roots, sharing
// the regularized point classifier. Coincident cap and generator lines are
// supported. Two independent bounded isolation passes use the supplied options;
// each underlying engine's comparisons/refinements have its documented bounded
// work policy. Any failure/cancellation returns an entirely empty report.
func (s TransverseCylinderSetQuery) ClipLine(ctx context.Context, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions) (TransverseCylinderMaterialLine, error) {
	var zero TransverseCylinderMaterialLine
	convert := func(v []int) CylinderSetPoint {
		return s.classifySigns([2][3]int{{v[0], v[1], v[2]}, {v[3], v[4], v[5]}})
	}
	material, e := s.pair.intersectLine(ctx, origin, direction, minT, maxT, o, func(v []int) (BorePointLocation, BoreBoundaryMask) {
		r := convert(v)
		if r.Location == BoreOutside {
			return BoreOutside, 0
		}
		return BoreBoundary, r.Boundary
	})
	if e != nil {
		return zero, e
	}
	sources, e := s.pair.intersectLine(ctx, origin, direction, minT, maxT, o, func(v []int) (BorePointLocation, BoreBoundaryMask) {
		r := convert(v)
		if r.Incidence == 0 {
			return BoreOutside, 0
		}
		return BoreBoundary, r.Incidence
	})
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return TransverseCylinderMaterialLine{Material: CylinderMaterialLine{Intervals: material.Intervals, Points: material.Hits}, Sources: sources}, nil
}
