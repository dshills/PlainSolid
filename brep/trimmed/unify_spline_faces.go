//go:build research

package trimmed

import (
	"context"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// UnifySplineFacesOptions controls exact same-domain merging.
type UnifySplineFacesOptions struct {
	Patches     geom.SurfacePatchOptions
	Injectivity geom.SurfaceInjectivityOptions
	Edges       edgecheck.Options
}

// UnifySplineFacesHistory maps original entities to their output IDs.
// Removed internal edges/vertices map to -1. The retained support and
// every exterior model edge evaluator are shared with the input.
type UnifySplineFacesHistory struct {
	Faces, Edges, Vertices []int
	RemovedEdges           []int
	Chart                  geom.SurfaceInjectivity
}

// UnifyAdjacentSplineFaces merges two hole-free straight-trim rectangles
// on the same spline chart and placement. If supports differ, the first
// support must extend over the union and exactly match the second
// restricted chart. One full side must be shared, possibly split into
// several coedges. The operation proves injectivity of
// the entire union chart and verifies continuous model edge agreement.
// Exterior subdivisions remain and can still meet neighboring faces.
func (s Solid) UnifyAdjacentSplineFaces(ctx context.Context, first, second int, o UnifySplineFacesOptions) (Solid, UnifySplineFacesHistory, error) {
	var h UnifySplineFacesHistory
	if ctx == nil || first < 0 || second < 0 || first == second || first >= len(s.faces) || second >= len(s.faces) {
		return Solid{}, h, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, h, e
	}
	first, second = min(first, second), max(first, second)
	a, b := s.faces[first], s.faces[second]
	baseA, mapA := parametric.SurfaceMapping(a.Surface)
	baseB, mapB := parametric.SurfaceMapping(b.Surface)
	spline, ok := baseA.(*geom.BSplineSurface)
	other, otherOK := baseB.(*geom.BSplineSurface)
	if !ok || !otherOK || mapA != mapB || a.Reversed != b.Reversed {
		return Solid{}, h, geom.ErrConvergence
	}
	ad, e := faceUVBounds(a)
	if e != nil {
		return Solid{}, h, e
	}
	bd, e := faceUVBounds(b)
	if e != nil {
		return Solid{}, h, e
	}
	if !straightRectangleLoop(a, ad) || !straightRectangleLoop(b, bd) {
		return Solid{}, h, geom.ErrConvergence
	}
	horizontal := ad.VMin == bd.VMin && ad.VMax == bd.VMax && (ad.UMax == bd.UMin || bd.UMax == ad.UMin)
	vertical := ad.UMin == bd.UMin && ad.UMax == bd.UMax && (ad.VMax == bd.VMin || bd.VMax == ad.VMin)
	if horizontal == vertical {
		return Solid{}, h, geom.ErrConvergence
	}
	domain := geom.ParameterRectangle{UMin: min(ad.UMin, bd.UMin), UMax: max(ad.UMax, bd.UMax),
		VMin: min(ad.VMin, bd.VMin), VMax: max(ad.VMax, bd.VMax)}
	patch, e := geom.NewSurfacePatch(spline, o.Patches)
	if e == nil {
		patch, e = patch.Trim(domain)
	}
	if e == nil && spline != other {
		otherPatch, nextErr := geom.NewSurfacePatch(other, o.Patches)
		if nextErr == nil {
			otherPatch, nextErr = otherPatch.Trim(bd)
		}
		onSecond, compareErr := patch.Trim(bd)
		if nextErr != nil {
			e = nextErr
		} else if compareErr != nil {
			e = compareErr
		} else {
			same, compareErr := geom.CompareExactSurfaces(ctx, onSecond, otherPatch, geom.ExactSurfaceComparisonOptions{
				MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits,
			})
			if compareErr != nil {
				e = compareErr
			} else if !same.Complete || !same.Equal {
				e = geom.ErrConvergence
			}
		}
	}
	if e == nil && mapA != geom.Identity() {
		patch, e = patch.Transformed(mapA)
	}
	if e != nil {
		return Solid{}, h, e
	}
	h.Chart, e = geom.CertifySurfaceInjectivity(patch, o.Injectivity)
	if e != nil || !h.Chart.ProvenInjective {
		if e == nil {
			e = geom.ErrConvergence
		}
		return Solid{}, h, e
	}
	fixed := max(ad.UMin, bd.UMin)
	if vertical {
		fixed = max(ad.VMin, bd.VMin)
	}
	isInterior := func(u Use) bool {
		if horizontal {
			return u.UVStart.X == fixed && u.UVEnd.X == fixed
		}
		return u.UVStart.Y == fixed && u.UVEnd.Y == fixed
	}
	joined := Face{Surface: a.Surface, Reversed: a.Reversed}
	var removed = map[int]Use{}
	outer := map[geom.Vec2]Use{}
	for i, f := range []Face{a, b} {
		for _, u := range f.Outer {
			if isInterior(u) {
				if i == 0 {
					removed[u.Edge] = u
				} else {
					p, ok := removed[u.Edge]
					if !ok || p.UVStart != u.UVEnd || p.UVEnd != u.UVStart || p.Reversed == u.Reversed {
						return Solid{}, UnifySplineFacesHistory{}, geom.ErrConvergence
					}
					h.RemovedEdges = append(h.RemovedEdges, u.Edge)
				}
				continue
			}
			if _, duplicate := outer[u.UVStart]; duplicate {
				return Solid{}, UnifySplineFacesHistory{}, geom.ErrConvergence
			}
			outer[u.UVStart] = u
		}
	}
	if len(removed) == 0 || len(h.RemovedEdges) != len(removed) {
		return Solid{}, UnifySplineFacesHistory{}, geom.ErrConvergence
	}
	at := geom.Vec2{X: domain.UMin, Y: domain.VMin}
	for range len(outer) {
		u, ok := outer[at]
		if !ok {
			return Solid{}, UnifySplineFacesHistory{}, geom.ErrConvergence
		}
		delete(outer, at)
		joined.Outer = append(joined.Outer, u)
		at = u.UVEnd
	}
	if !straightRectangleLoop(joined, domain) {
		return Solid{}, UnifySplineFacesHistory{}, geom.ErrConvergence
	}
	uvProof, e := ValidateFaceTrimLoops(ctx, joined, trimvalidate.Options{})
	if e != nil || uvProof.Status != trimvalidate.Certified {
		if e == nil {
			e = geom.ErrConvergence
		}
		return Solid{}, UnifySplineFacesHistory{}, e
	}
	h.Vertices, h.Edges, h.Faces = deletedEntityMap(len(s.vertices)), deletedEntityMap(len(s.edges)), deletedEntityMap(len(s.faces))
	vertexUsed := make([]bool, len(s.vertices))
	for i, edge := range s.edges {
		if _, remove := removed[i]; !remove {
			vertexUsed[edge.Start], vertexUsed[edge.End] = true, true
		}
	}
	var vertices []geom.Vec3
	for i, used := range vertexUsed {
		if used {
			h.Vertices[i] = len(vertices)
			vertices = append(vertices, s.vertices[i])
		}
	}
	var edges []parametric.Edge
	for i, edge := range s.edges {
		if _, remove := removed[i]; remove {
			continue
		}
		h.Edges[i] = len(edges)
		edge.Start, edge.End = h.Vertices[edge.Start], h.Vertices[edge.End]
		edges = append(edges, edge)
	}
	var faces []Face
	for i, f := range s.Faces() {
		if i == second {
			continue
		}
		if i == first {
			f = joined
			h.Faces[second] = len(faces)
		}
		h.Faces[i] = len(faces)
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for j := range loop {
				loop[j].Edge = h.Edges[loop[j].Edge]
			}
		}
		faces = append(faces, f)
	}
	result, e := NewSolidContext(ctx, vertices, edges, faces, s.tolerance)
	if e != nil {
		return Solid{}, UnifySplineFacesHistory{}, e
	}
	proof, e := result.ValidateEdgeAgreement(ctx, o.Edges)
	if e != nil || !proof.Complete || result.Topology().EulerCharacteristic != s.Topology().EulerCharacteristic {
		if e == nil {
			e = geom.ErrConvergence
		}
		return Solid{}, UnifySplineFacesHistory{}, e
	}
	for _, use := range proof.Uses {
		if use.Report.SquaredUpperBound == nil || use.Report.SquaredUpperBound.Sign() != 0 {
			return Solid{}, UnifySplineFacesHistory{}, geom.ErrConvergence
		}
	}
	return result, h, nil
}

func deletedEntityMap(n int) []int {
	m := make([]int, n)
	for i := range m {
		m[i] = -1
	}
	return m
}

func straightRectangleLoop(f Face, d geom.ParameterRectangle) bool {
	if !d.Valid() || len(f.Holes) != 0 || len(f.Outer) < 4 {
		return false
	}
	corners := insetCorners(d)
	for start, u := range f.Outer {
		if u.UVStart != corners[0] {
			continue
		}
		side := 0
		at := u.UVStart
		for j := range f.Outer {
			use := f.Outer[(start+j)%len(f.Outer)]
			if side >= 4 || use.Curve != nil || use.UVStart != at {
				return false
			}
			a, b := corners[side], corners[(side+1)%4]
			p, q := use.UVStart, use.UVEnd
			if a.X == b.X {
				if p.X != a.X || q.X != a.X || !directedSidePiece(a.Y, b.Y, p.Y, q.Y) {
					return false
				}
			} else if p.Y != a.Y || q.Y != a.Y || !directedSidePiece(a.X, b.X, p.X, q.X) {
				return false
			}
			at = q
			if q == b {
				side++
			}
		}
		return side == 4
	}
	return false
}
