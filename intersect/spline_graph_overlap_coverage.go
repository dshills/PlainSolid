package intersect

import (
	"context"

	"plainsolid/geom"
)

// certifyOverlapCell first proves that every common model point is in both
// exact projected-overlap rectangles. Restricting an original candidate to
// those rectangles therefore excludes no contact, including its boundary.
// Degenerate rectangle intersections remain unresolved, never empty by fiat.
func certifyOverlapCell(ctx context.Context, a, b *geom.SurfacePatch, cell geom.SurfacePairParameterBox, tubes []geom.SurfacePairTubeRegion, o geom.SurfaceTubeUnionCoverageOptions) (geom.SurfaceTubeUnionCoverage, error) {
	var out geom.SurfaceTubeUnionCoverage
	clip, e := geom.ClipAffineXYGraphOverlap(ctx, a, b, geom.SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return out, e
	}
	if clip.Disjoint {
		out.Covered = true
		return out, nil
	}
	intersect := func(x, y geom.ParameterRectangle) (geom.ParameterRectangle, bool) {
		r := geom.ParameterRectangle{UMin: max(x.UMin, y.UMin), UMax: min(x.UMax, y.UMax), VMin: max(x.VMin, y.VMin), VMax: min(x.VMax, y.VMax)}
		return r, r.UMin > r.UMax || r.VMin > r.VMax
	}
	ca, emptyA := intersect(cell.A, clip.A)
	cb, emptyB := intersect(cell.B, clip.B)
	if emptyA || emptyB {
		out.Covered = true
		return out, nil
	}
	if !ca.Valid() || !cb.Valid() {
		return out, geom.ErrConvergence
	}
	a, e = a.Trim(clip.A)
	if e != nil {
		return out, e
	}
	b, e = b.Trim(clip.B)
	if e != nil {
		return out, e
	}
	// The common-domain prover must independently recover each original tube;
	// never crop a tube or widen its accepted box to fit the overlap.
	for _, t := range tubes {
		ta, _ := intersect(t.Box.A, clip.A)
		tb, _ := intersect(t.Box.B, clip.B)
		if ta != t.Box.A || tb != t.Box.B {
			return out, geom.ErrConvergence
		}
	}
	return geom.CertifyAffineXYSurfacePairCellInTubes(ctx, a, b, geom.SurfacePairParameterBox{A: ca, B: cb}, tubes, o)
}

func certifyProjectedOverlapCell(ctx context.Context, a, b *geom.SurfacePatch, cell geom.SurfacePairParameterBox,
	tubes []geom.SurfacePairTubeRegion, axes [2]int, o geom.SurfaceTubeUnionCoverageOptions) (geom.SurfaceTubeUnionCoverage, error) {
	if axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] {
		return geom.SurfaceTubeUnionCoverage{}, geom.ErrInvalid
	}
	if axes != ([2]int{0, 1}) {
		var m geom.Affine
		m[0][axes[0]], m[1][axes[1]], m[2][3-axes[0]-axes[1]] = 1, 1, 1
		var e error
		a, e = a.Transformed(m)
		if e != nil {
			return geom.SurfaceTubeUnionCoverage{}, e
		}
		b, e = b.Transformed(m)
		if e != nil {
			return geom.SurfaceTubeUnionCoverage{}, e
		}
	}
	// A permutation preserves equality of the original XYZ supports. Tube
	// domains and candidate chart coordinates are unchanged, and the proof
	// revalidates each tube after that exact coordinate change.
	return certifyOverlapCell(ctx, a, b, cell, tubes, o)
}
