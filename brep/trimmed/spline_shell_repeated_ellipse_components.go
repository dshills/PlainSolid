package trimmed

import (
	"context"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// FuseExternallyTangentSplineShellEllipseComponents keeps two outward native
// shells for a proved exterior closed double contact. Each component retains
// its original supports, trims and topology, with independent identities
// along the geometric contact. The method establishes disjoint interiors
// before assembling the result.
func FuseExternallyTangentSplineShellEllipseComponents(ctx context.Context, a, b Solid,
	o SplineShellRepeatedEllipseOptions) (Solid, SplineShellCombineHistory, SplineShellRepeatedEllipse, error) {
	_, _, proof, err := CombineExternallyTangentSplineShellEllipse(ctx, a, b, parametric.Intersection, o)
	if err != nil {
		return Solid{}, SplineShellCombineHistory{}, proof, err
	}
	out, h, err := assembleTangentEllipseBoundaries(ctx, a, b, false)
	if err != nil {
		return Solid{}, SplineShellCombineHistory{}, SplineShellRepeatedEllipse{ExactOperations: proof.ExactOperations}, err
	}
	return out, h, proof, nil
}

// CutInternallyTangentSplineShellEllipseCavity retains the outward stock
// and inward cutter as two native boundary shells. The cutter must be
// contained by the stock and touch it only on the proved double ellipse.
func CutInternallyTangentSplineShellEllipseCavity(ctx context.Context, stock, cutter Solid,
	o SplineShellRepeatedEllipseOptions) (Solid, SplineShellCombineHistory, SplineShellRepeatedEllipse, error) {
	_, _, proof, err := CombineInternallyTangentSplineShellEllipse(ctx, stock, cutter, parametric.Intersection, o)
	if err != nil {
		return Solid{}, SplineShellCombineHistory{}, proof, err
	}
	if proof.Enclosing != 0 {
		return Solid{}, SplineShellCombineHistory{}, SplineShellRepeatedEllipse{ExactOperations: proof.ExactOperations}, geom.ErrConvergence
	}
	out, h, err := assembleTangentEllipseBoundaries(ctx, stock, cutter, true)
	if err != nil {
		return Solid{}, SplineShellCombineHistory{}, SplineShellRepeatedEllipse{ExactOperations: proof.ExactOperations}, err
	}
	return out, h, proof, nil
}

func assembleTangentEllipseBoundaries(ctx context.Context, a, b Solid, cavity bool) (Solid, SplineShellCombineHistory, error) {
	if cavity {
		var err error
		b, err = reverseMaterialShell(ctx, b)
		if err != nil {
			return Solid{}, SplineShellCombineHistory{}, err
		}
	}
	out, err := AssembleShells(ctx, []PlacedSolid{{a, geom.Identity()}, {b, geom.Identity()}}, len(a.faces)+len(b.faces))
	if err != nil {
		return Solid{}, SplineShellCombineHistory{}, err
	}
	h := SplineShellCombineHistory{
		Method: "repeated-ellipse-tangent-components",
		FacesA: make([][]int, len(a.faces)), FacesB: make([][]int, len(b.faces)),
		EdgesA: make([][]int, len(a.edges)), EdgesB: make([][]int, len(b.edges)),
	}
	for i := range h.FacesA {
		h.FacesA[i] = []int{i}
	}
	for i := range h.FacesB {
		h.FacesB[i] = []int{len(a.faces) + i}
	}
	for i := range h.EdgesA {
		h.EdgesA[i] = []int{i}
	}
	for i := range h.EdgesB {
		h.EdgesB[i] = []int{len(a.edges) + i}
	}
	if cavity {
		h.Method = "contained-repeated-ellipse-cavity"
		for i := range b.faces {
			h.ReversedFaces = append(h.ReversedFaces, len(a.faces)+i)
		}
	}
	return out, h, nil
}
