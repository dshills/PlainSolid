package trimmed

import (
	"context"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// WrappedPairProof recovers the two original supports only after exact material
// topology validation. First/Second are larger/smaller for shoulder, outer/inner
// for toroidal-void. FaceMap maps reconstructed face IDs to original face IDs.
type WrappedPairProof struct {
	First, Second *geom.WrappedSurfacePatch
	FaceMap       []int
	Complete      bool
}

func (s Solid) ValidateCanonicalWrappedShoulder(ctx context.Context, o geom.SurfacePatchOptions) (WrappedPairProof, error) {
	fail := func() (WrappedPairProof, error) { return WrappedPairProof{}, geom.ErrConvergence }
	if ctx == nil {
		return WrappedPairProof{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return WrappedPairProof{}, e
	}
	var ws []*geom.WrappedSurfacePatch
	for _, f := range s.faces {
		if w, ok := f.Surface.(*geom.WrappedSurfacePatch); ok {
			if f.Reversed {
				return fail()
			}
			ws = append(ws, w)
		}
	}
	if len(ws) != 2 || len(s.faces) != 5 {
		return fail()
	}
	a, pa, e := NewWrappedCylinder(ctx, ws[0], s.tolerance, o)
	if e != nil {
		return WrappedPairProof{}, e
	}
	b, pb, e := NewWrappedCylinder(ctx, ws[1], s.tolerance, o)
	if e != nil {
		return WrappedPairProof{}, e
	}
	if pa.Radius.Cmp(pb.Radius) == 0 {
		return fail()
	}
	if pa.UpperZ.Cmp(pb.LowerZ) != 0 && pb.UpperZ.Cmp(pa.LowerZ) != 0 {
		return fail()
	}
	reference, _, e := CombineAxialWrappedCylinders(ctx, a, b, parametric.Union, o)
	if e != nil {
		return WrappedPairProof{}, e
	}
	// Axial union may allocate an equivalent restricted support. Rebind only
	// after proving identical represented parameterization over the whole chart.
	reference.faces = reference.Faces()
	for i, f := range reference.faces {
		w, ok := f.Surface.(*geom.WrappedSurfacePatch)
		if !ok {
			continue
		}
		matched := false
		for _, source := range ws {
			same, err := geom.CompareExactSurfaces(ctx, w.FundamentalPatch(), source.FundamentalPatch(), geom.ExactSurfaceComparisonOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
			if err != nil {
				return WrappedPairProof{}, err
			}
			if same.Complete && same.Equal {
				reference.faces[i].Surface = source
				matched = true
				break
			}
		}
		if !matched {
			return fail()
		}
	}
	mapping, e := matchCanonicalWrappedMaterial(ctx, s, reference, o)
	if e != nil {
		return WrappedPairProof{}, e
	}
	if pa.Radius.Cmp(pb.Radius) < 0 {
		ws[0], ws[1] = ws[1], ws[0]
	}
	return WrappedPairProof{First: ws[0], Second: ws[1], FaceMap: mapping, Complete: true}, nil
}
func (s Solid) ValidateCanonicalWrappedToroidalVoid(ctx context.Context, o geom.SurfacePatchOptions) (WrappedPairProof, error) {
	fail := func() (WrappedPairProof, error) { return WrappedPairProof{}, geom.ErrConvergence }
	if ctx == nil {
		return WrappedPairProof{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return WrappedPairProof{}, e
	}
	var outer, inner *geom.WrappedSurfacePatch
	for _, f := range s.faces {
		w, ok := f.Surface.(*geom.WrappedSurfacePatch)
		if !ok {
			return fail()
		}
		if f.Reversed {
			if inner != nil {
				return fail()
			}
			inner = w
		} else {
			if outer != nil {
				return fail()
			}
			outer = w
		}
	}
	if outer == nil || inner == nil || len(s.faces) != 2 {
		return fail()
	}
	a, pa, e := NewWrappedToroid(ctx, outer, s.tolerance, o)
	if e != nil {
		return WrappedPairProof{}, e
	}
	b, pb, e := NewWrappedToroid(ctx, inner, s.tolerance, o)
	if e != nil {
		return WrappedPairProof{}, e
	}
	if !pa.CircularProfiles || !pb.CircularProfiles || pa.MajorRadius.Cmp(pb.MajorRadius) != 0 || pa.RadialMinorRadius.Cmp(pb.RadialMinorRadius) <= 0 || pa.AxialMinorRadius.Cmp(pb.AxialMinorRadius) <= 0 {
		return fail()
	}
	for k := 0; k < 3; k++ {
		if pa.Center[k].Cmp(pb.Center[k]) != 0 {
			return fail()
		}
	}
	reference, _, e := CombineNestedSplineComponents(ctx, a, b, parametric.Difference, SplineShellBooleanOptions{Embedding: SplineRectangleEmbeddingOptions{Patches: o}})
	if e != nil {
		return WrappedPairProof{}, e
	}
	mapping, e := matchCanonicalWrappedMaterial(ctx, s, reference, o)
	if e != nil {
		return WrappedPairProof{}, e
	}
	return WrappedPairProof{First: outer, Second: inner, FaceMap: mapping, Complete: true}, nil
}
