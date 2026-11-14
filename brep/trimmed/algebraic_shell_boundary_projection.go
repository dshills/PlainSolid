package trimmed

import (
	"context"
	"errors"
	"plainsolid/geom"
)

// Whole original-face affinity is a sufficient projection selection proof;
// each subsequent tube sample and chord still performs its own exact proof.
func (s AlgebraicShellBoolean) selectBoundaryGraphProjection(ctx context.Context, owner [2]int, maxWork, bits int) (axes [2]int, used int, err error) {
	if ctx == nil || s.source == nil || owner[0] < 0 || owner[0] > 1 || maxWork < 1 {
		return axes, 0, geom.ErrConvergence
	}
	source := [2]Solid{s.source.a, s.source.b}[owner[0]]
	if owner[1] < 0 || owner[1] >= len(source.faces) {
		return axes, 0, geom.ErrInvalid
	}
	face := source.faces[owner[1]]
	p, e := exactFaceSupportPatch(face, geom.SurfacePatchOptions{MaxOperations: maxWork, MaxCoefficientBits: bits})
	if e != nil {
		return axes, 0, e
	}
	used += p.ExactOperations()
	domain, e := faceUVBounds(face)
	if e != nil {
		return axes, used, e
	}
	if used >= maxWork {
		return axes, used, geom.ErrConvergence
	}
	p, n, e := p.TrimWithWorkLimit(domain, maxWork-used)
	used += n
	if e != nil {
		return axes, used, e
	}
	for _, choice := range [][2]int{{0, 1}, {0, 2}, {1, 2}} {
		if used >= maxWork {
			return axes, used, geom.ErrConvergence
		}
		_, n, e := geom.NewAffineProjectedGraphWithWork(ctx, p, choice, geom.SurfacePatchOptions{MaxOperations: maxWork - used, MaxCoefficientBits: bits})
		used += n
		if e == nil {
			return choice, used, nil
		}
		if !errors.Is(e, geom.ErrConvergence) && !errors.Is(e, geom.ErrDegenerate) && !errors.Is(e, geom.ErrUnrepresentable) {
			return axes, used, e
		}
	}
	return axes, used, geom.ErrConvergence
}
