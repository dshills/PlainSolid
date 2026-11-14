package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep/placement"
	"plainsolid/geom"
)

// IntersectPlacedWrappedFrustumLineRational intersects a complete original
// capped frustum with an exact world occurrence line. Independently
// authored homothetic, noncircular rational profiles are supported. Every
// hit and overlap retains its original native face rather than a world copy.
func (s Solid) IntersectPlacedWrappedFrustumLineRational(ctx context.Context, pose placement.RationalRigid,
	origin, direction [3]*big.Rat, minT, maxT *big.Rat, o geom.ToroidLineOptions) (WrappedLoftLineIntersections, error) {
	var out WrappedLoftLineIntersections
	p, e := pose.TransformPoint(ctx, origin, true, o.Surface.MaxCoefficientBits)
	if e != nil {
		return out, e
	}
	d, e := pose.TransformVector(ctx, direction, true, o.Surface.MaxCoefficientBits)
	if e != nil {
		return out, e
	}
	w, proof, e := s.ValidateCanonicalWrappedFrustum(ctx, o.Surface)
	if e != nil {
		return out, e
	}
	parts := [3]geom.CylinderBoundaryPart{geom.CylinderLateral, geom.CylinderLowerCap, geom.CylinderUpperCap}
	ids := [3]int{-1, -1, -1}
	for i, f := range s.faces {
		switch x := f.Surface.(type) {
		case *geom.WrappedSurfacePatch:
			if x != w || ids[0] >= 0 {
				return out, geom.ErrConvergence
			}
			ids[0] = i
		case geom.PlaneSurface:
			z := ratInset(x.Frame().Origin().Z)
			switch {
			case z.Cmp(proof.LowerZ) == 0 && ids[1] < 0:
				ids[1] = i
			case z.Cmp(proof.UpperZ) == 0 && ids[2] < 0:
				ids[2] = i
			default:
				return out, geom.ErrConvergence
			}
		default:
			return out, geom.ErrConvergence
		}
	}
	for _, id := range ids {
		if id < 0 {
			return out, geom.ErrConvergence
		}
	}
	r, e := geom.IntersectWrappedFrustumLineRational(ctx, w, p, d, minT, maxT, o)
	if e != nil {
		return out, e
	}
	for _, h := range r.Hits {
		hit := WrappedLoftLineHit{Parameter: h.Parameter, Parts: h.Parts}
		for i, part := range parts {
			if h.Parts&part != 0 {
				hit.Faces = append(hit.Faces, ids[i])
			}
		}
		sort.Ints(hit.Faces)
		out.Hits = append(out.Hits, hit)
	}
	for _, h := range r.Overlaps {
		face := -1
		for i, part := range parts {
			if h.Part == part {
				face = ids[i]
				break
			}
		}
		if face < 0 {
			return WrappedLoftLineIntersections{}, geom.ErrConvergence
		}
		out.Overlaps = append(out.Overlaps, WrappedLoftLineOverlap{Start: h.Start, End: h.End, Face: face, Part: h.Part})
	}
	return out, nil
}
