package trimmed

import (
	"context"
	"plainsolid/geom"
)

// WrappedBoreProof recovers canonical outward/inner supports and original face
// IDs after checking every loop, model edge and vertex against a newly proved
// bore construction. Kind is tube, blind-bore or cavity.
type WrappedBoreProof struct {
	Outer, Inner           *geom.WrappedSurfacePatch
	Kind                   string
	OuterFaces, InnerFaces [3]int
	Complete               bool
}

// ValidateCanonicalWrappedBore permits topology reindexing but requires the
// complete canonical rim partition and exact cap pcurves. It never infers
// material closure from two wrapped supports alone.
func (s Solid) ValidateCanonicalWrappedBore(ctx context.Context, o geom.SurfacePatchOptions) (WrappedBoreProof, error) {
	fail := func() (WrappedBoreProof, error) { return WrappedBoreProof{}, geom.ErrConvergence }
	if ctx == nil {
		return WrappedBoreProof{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return WrappedBoreProof{}, e
	}
	var p WrappedBoreProof
	for _, f := range s.faces {
		if w, ok := f.Surface.(*geom.WrappedSurfacePatch); ok {
			if f.Reversed {
				if p.Inner != nil {
					return fail()
				}
				p.Inner = w
			} else {
				if p.Outer != nil {
					return fail()
				}
				p.Outer = w
			}
		}
	}
	if p.Outer == nil || p.Inner == nil {
		return fail()
	}
	reference, h, e := NewWrappedCylinderBore(ctx, p.Outer, p.Inner, s.tolerance, o)
	if e != nil {
		return WrappedBoreProof{}, e
	}
	if len(s.faces) != len(reference.faces) || len(s.edges) != len(reference.edges) || len(s.vertices) != len(reference.vertices) || len(s.Topology().Shells) != len(reference.Topology().Shells) {
		return fail()
	}
	switch len(s.faces) {
	case 4:
		p.Kind = "tube"
	case 5:
		p.Kind = "blind-bore"
	case 6:
		p.Kind = "cavity"
	default:
		return fail()
	}
	mapping, e := matchCanonicalWrappedMaterial(ctx, s, reference, o)
	if e != nil {
		return WrappedBoreProof{}, e
	}
	for i := 0; i < 3; i++ {
		p.OuterFaces[i] = mapping[h.OuterFaces[i]]
		p.InnerFaces[i] = mapping[h.InnerFaces[i]]
	}
	p.Complete = true
	return p, nil
}

func matchCanonicalWrappedMaterial(ctx context.Context, s, reference Solid, o geom.SurfacePatchOptions) ([]int, error) {
	fail := func() ([]int, error) { return nil, geom.ErrConvergence }
	if len(s.faces) != len(reference.faces) || len(s.edges) != len(reference.edges) || len(s.vertices) != len(reference.vertices) || len(s.Topology().Shells) != len(reference.Topology().Shells) {
		return fail()
	}
	mapping := make([]int, len(reference.faces))
	used := make([]bool, len(s.faces))
	for i, want := range reference.faces {
		found := -1
		for j, have := range s.faces {
			if !used[j] && want.Surface == have.Surface && want.Reversed == have.Reversed && len(want.Outer) == len(have.Outer) && len(want.Holes) == len(have.Holes) {
				if found >= 0 {
					return fail()
				}
				found = j
			}
		}
		if found < 0 {
			return fail()
		}
		mapping[i] = found
		used[found] = true
	}
	// Expose each hole as an additional directed contour to the exact shared
	// edge/vertex matcher. Original surfaces and curves remain authoritative.
	originalFlat, referenceFlat := s, reference
	originalFlat.faces = nil
	referenceFlat.faces = nil
	ids := []int{}
	for i, j := range mapping {
		a, b := s.faces[j], reference.faces[i]
		loopsA, loopsB := append([][]Use{a.Outer}, a.Holes...), append([][]Use{b.Outer}, b.Holes...)
		for k := range loopsA {
			if len(loopsA[k]) != len(loopsB[k]) {
				return fail()
			}
			aa, bb := a, b
			aa.Outer, bb.Outer = loopsA[k], loopsB[k]
			aa.Holes, bb.Holes = nil, nil
			ids = append(ids, len(originalFlat.faces))
			originalFlat.faces = append(originalFlat.faces, aa)
			referenceFlat.faces = append(referenceFlat.faces, bb)
		}
	}
	if _, e := matchWrappedStationLoft(ctx, originalFlat, referenceFlat, ids, o); e != nil {
		return nil, e
	}
	return mapping, nil
}
