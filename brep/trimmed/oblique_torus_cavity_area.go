//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// CertifiedObliqueTorusCavityArea keeps the four analytic support measures
// and their actual native face owners. Torus area is aggregated over16 charts.
type CertifiedObliqueTorusCavityArea struct {
	geom.TorusCylinderArea
	SupportFaces [4][]int
}

// BoundObliqueTorusCavityArea authenticates the actual immutable stored native
// geometry/topology against an independently rebuilt exact source before using
// analytic measures. Canonical identity deliberately rejects remapped entities.
// No sampled geometry, stored proof flag, or caller-authored face label is used.
// Source construction has the bounded per-chart work of ObliqueTorusCavity;
// area arithmetic is a separate single ledger. Existing JSON remains unchanged.
func BoundObliqueTorusCavityArea(ctx context.Context, s Solid, R, r, c, lower, upper float64, pose [3][4]*big.Rat, o geom.TorusCylinderAreaOptions) (CertifiedObliqueTorusCavityArea, error) {
	var zero CertifiedObliqueTorusCavityArea
	q, e := bindObliqueTorusCavitySource(ctx, s, R, r, c, lower, upper, pose, o.Arithmetic)
	if e != nil {
		return zero, e
	}
	area, e := q.BoundArea(ctx, o)
	if e != nil {
		return zero, e
	}
	out := CertifiedObliqueTorusCavityArea{TorusCylinderArea: area}
	out.SupportFaces[1] = []int{0}
	out.SupportFaces[2] = []int{1}
	out.SupportFaces[3] = []int{2}
	for i := 3; i < 19; i++ {
		out.SupportFaces[0] = append(out.SupportFaces[0], i)
	}
	return out, nil
}

// Shared stored-identity gate; public helpers never trust mutable face labels.
func bindObliqueTorusCavitySource(ctx context.Context, s Solid, R, r, c, lower, upper float64, pose [3][4]*big.Rat, o geom.SurfacePatchOptions) (geom.ObliqueCylinderTorusCavityMaterial, error) {
	var zero geom.ObliqueCylinderTorusCavityMaterial
	if len(s.faces) != 19 || len(s.vertices) > 100 || len(s.edges) > 100 {
		return zero, geom.ErrDomain
	}
	if _, ok := s.faces[0].Surface.(geom.CylinderSurface); !ok {
		return zero, geom.ErrDomain
	}
	for _, f := range s.faces[1:3] {
		if _, ok := parametric.PolarPlaneFrame(f.Surface); !ok {
			return zero, geom.ErrDomain
		}
	}
	for _, f := range s.faces[3:] {
		surface, ok := f.Surface.(*geom.ExactBezierSurface)
		if !ok {
			return zero, geom.ErrDomain
		}
		recipe, e := surface.ExactPatch().ExportExactBezierRecipe()
		if e != nil || recipe.DegreeU != 2 || recipe.DegreeV != 2 || len(recipe.Controls) != 9 {
			return zero, geom.ErrDomain
		}
	}
	expected, q, e := ObliqueTorusCavity(ctx, R, r, c, lower, upper, pose, s.tolerance, o)
	if e != nil {
		return zero, e
	}
	var a, b bytes.Buffer
	if e = WriteJSON(&a, s); e != nil {
		return zero, e
	}
	if e = WriteJSON(&b, expected); e != nil {
		return zero, e
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		return zero, geom.ErrDomain
	}
	return q, nil
}
