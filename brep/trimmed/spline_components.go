package trimmed

import (
	"context"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SplineComponentProof identifies one outward, embedded material shell and
// its outward-rounded conservative box. Exactly one geometry proof is set.
type SplineComponentProof struct {
	Shell     int
	Bounds    geom.Box
	Rectangle *SplineRectangleEmbedding
	Toroid    *geom.WrappedToroidEmbedding
	Cylinder  *geom.WrappedCylinderEmbedding
	Frustum   *geom.WrappedFrustumEmbedding
}

// ValidateDisjointSplineComponents proves each connected shell separately and
// requires pairwise disjoint conservative boxes. Rectangular spline shells
// and canonical wrapped toroids and cylinders are supported. This proof rejects
// nested shells, touching boxes and unrecognized trimmed domains.
func (s Solid) ValidateDisjointSplineComponents(ctx context.Context, o SplineRectangleEmbeddingOptions) ([]SplineComponentProof, error) {
	return s.validateSplineComponents(ctx, o, true)
}

func (s Solid) validateSplineComponents(ctx context.Context, o SplineRectangleEmbeddingOptions, requireDisjointBoxes bool) ([]SplineComponentProof, error) {
	if o.MaxFaces == 0 {
		o.MaxFaces = 64
	}
	if ctx == nil || o.MaxFaces < 1 || o.MaxFaces > 128 {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if len(s.faces) > o.MaxFaces {
		return nil, parametric.ErrBudget
	}
	topology := s.Topology()
	var result []SplineComponentProof
	for i, shell := range topology.Shells {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		out := SplineComponentProof{Shell: i}
		if len(shell.Faces) == 1 {
			f := s.faces[shell.Faces[0]]
			w, ok := f.Surface.(*geom.WrappedSurfacePatch)
			if !ok || f.Reversed || len(f.Outer) != 4 || len(f.Holes) != 0 ||
				len(shell.Vertices) != 1 || len(shell.Edges) != 2 {
				return nil, fmt.Errorf("%w: unsupported one-face spline material", geom.ErrConvergence)
			}
			toroid, proof, e := NewWrappedToroid(ctx, w, s.tolerance, o.Patches)
			if e != nil {
				return nil, e
			}
			// Check the same directed UV sides in order up to cyclic rotation,
			// and require the identical two-use pairing and exact model vertex.
			canonical := toroid.faces[0].Outer
			found := false
			for shift := range canonical {
				mapping := map[int]int{}
				match := true
				for j, u := range f.Outer {
					v := canonical[(j+shift)%4]
					if u.Curve != nil || u.UVStart != v.UVStart || u.UVEnd != v.UVEnd {
						match = false
						break
					}
					if old, ok := mapping[u.Edge]; ok && old != v.Edge {
						match = false
						break
					}
					mapping[u.Edge] = v.Edge
				}
				found = found || match && len(mapping) == 2
			}
			if !found || s.vertices[shell.Vertices[0]] != toroid.vertices[0] {
				return nil, geom.ErrConvergence
			}
			single, e := s.ShellContext(ctx, i)
			if e != nil {
				return nil, e
			}
			agreement, e := single.ValidateEdgeAgreement(ctx, o.EdgeAgreement)
			if e != nil || !agreement.Complete {
				if e == nil {
					e = geom.ErrConvergence
				}
				return nil, e
			}
			out.Toroid = &proof
			out.Bounds, _ = w.FundamentalPatch().Bounds()
		} else {
			single := s
			var e error
			if len(topology.Shells) > 1 {
				single, e = s.ShellContext(ctx, i)
				if e != nil {
					return nil, e
				}
			}
			if hasCanonicalWrappedCylinderShape(single) {
				w, proof, e := single.ValidateCanonicalWrappedFrustum(ctx, o.Patches)
				if e != nil {
					return nil, e
				}
				if proof.LowerRadius.Cmp(proof.UpperRadius) == 0 {
					out.Cylinder = &geom.WrappedCylinderEmbedding{CenterX: proof.CenterX, CenterY: proof.CenterY,
						Radius: proof.LowerRadius, LowerZ: proof.LowerZ, UpperZ: proof.UpperZ,
						QuarterParameters: proof.QuarterParameters, Circular: proof.Circular, Complete: proof.Complete,
						ExactOperations: proof.ExactOperations}
				} else {
					out.Frustum = &proof
				}
				out.Bounds, _ = w.FundamentalPatch().Bounds()
			} else {
				var faces []splineEmbeddingFace
				proof, e := single.validateSplineRectangleEmbedding(ctx, o, &faces)
				if e != nil {
					return nil, e
				}
				out.Rectangle = &proof
				out.Bounds = splineShellPatchBounds(faces)
			}
		}
		if requireDisjointBoxes {
			for _, p := range result {
				if _, overlap := p.Bounds.Intersect(out.Bounds); overlap {
					return nil, fmt.Errorf("%w: material components %d and %d have overlapping boxes", geom.ErrConvergence, p.Shell, out.Shell)
				}
			}
		}
		result = append(result, out)
	}
	return result, nil
}

func hasCanonicalWrappedCylinderShape(s Solid) bool {
	if len(s.faces) != 3 {
		return false
	}
	for _, f := range s.faces {
		if _, ok := f.Surface.(*geom.WrappedSurfacePatch); ok {
			return true
		}
	}
	return false
}

// CombineDisjointSplineComponents handles union, intersection and difference
// when all material shells are embedded, outward and pairwise separated by
// their conservative boxes. All retained surfaces, coedges and vertex IDs are
// copied without healing; history maps both original face and edge arrays.
func CombineDisjointSplineComponents(ctx context.Context, a, b Solid, op parametric.BooleanOp,
	o SplineRectangleEmbeddingOptions) (Solid, SplineShellCombineHistory, error) {
	var h SplineShellCombineHistory
	if op > parametric.Difference {
		return Solid{}, h, geom.ErrInvalid
	}
	ac, e := a.ValidateDisjointSplineComponents(ctx, o)
	if e != nil {
		return Solid{}, h, e
	}
	bc, e := b.ValidateDisjointSplineComponents(ctx, o)
	if e != nil {
		return Solid{}, h, e
	}
	for _, x := range ac {
		for _, y := range bc {
			if _, overlap := x.Bounds.Intersect(y.Bounds); overlap {
				return Solid{}, h, geom.ErrConvergence
			}
		}
	}
	h = SplineShellCombineHistory{
		FacesA: make([][]int, len(a.faces)), FacesB: make([][]int, len(b.faces)),
		EdgesA: make([][]int, len(a.edges)), EdgesB: make([][]int, len(b.edges)),
		Method: "disjoint-components",
	}
	if op == parametric.Intersection {
		return Solid{}, h, nil
	}
	for i := range h.FacesA {
		h.FacesA[i] = []int{i}
	}
	for i := range h.EdgesA {
		h.EdgesA[i] = []int{i}
	}
	if op == parametric.Difference || len(b.faces) == 0 {
		return a, h, nil
	}
	for i := range h.FacesB {
		h.FacesB[i] = []int{len(a.faces) + i}
	}
	for i := range h.EdgesB {
		h.EdgesB[i] = []int{len(a.edges) + i}
	}
	if len(a.faces) == 0 {
		return b, h, nil
	}
	out, e := AssembleShells(ctx, []PlacedSolid{{a, geom.Identity()}, {b, geom.Identity()}}, len(a.faces)+len(b.faces))
	if e != nil {
		return Solid{}, SplineShellCombineHistory{}, e
	}
	return out, h, nil
}
