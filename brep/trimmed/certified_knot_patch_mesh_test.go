package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestCertifiedNonuniformWrappedKnotGrid(t *testing.T) {
	ctx := context.Background()
	recipe, e := independentWrappedToroid(t).ExportRecipe()
	if e != nil {
		t.Fatal(e)
	}
	d := recipe.Surface.Data()
	d.KnotsU = []float64{0, 0, 0, .5, .5, 1.5, 1.5, 3, 3, 4, 4, 4}
	d.KnotsV = []float64{-2, -2, -2, -1, -1, .5, .5, 1, 1, 2, 2, 2}
	source, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	patch, e := geom.NewSurfacePatch(source, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	// Position seams are exact; the nonuniform reparameterization deliberately
	// does not claim matching first derivatives across the global seam.
	wrapped, _, e := geom.NewWrappedSurfacePatch(ctx, patch, geom.WrappedSurfaceOptions{WrapU: true, WrapV: true})
	if e != nil {
		t.Fatal(e)
	}
	r, e := TessellateWrappedKnotPatchCertified(ctx, wrapped, CertifiedPatchMeshOptions{Deflection: .5, MaxSubdivisions: 16})
	if e != nil || !r.BothDirections {
		t.Fatal("local rational certificates", r.Subdivisions, e)
	}
	for _, tc := range []struct{ parameters, cuts []float64 }{{r.UParameters, []float64{0, .5, 1.5, 3, 4}}, {r.VParameters, []float64{-2, -1, .5, 1, 2}}} {
		for _, cut := range tc.cuts {
			found := false
			for _, p := range tc.parameters {
				found = found || p == cut
			}
			if !found {
				t.Fatal("lost true knot", cut)
			}
		}
	}
	nu, nv := len(r.UParameters)-1, len(r.VParameters)-1
	if len(r.Mesh.Vertices) != nu*nv || len(r.Mesh.Triangles) != 2*nu*nv {
		t.Fatal("seam grid dimensions")
	}
	if e := r.Mesh.ValidateSolid(); e != nil {
		t.Fatal("exact seam welding", e)
	}
	for _, bound := range r.SquaredErrors {
		if bound.Cmp(big.NewRat(1, 4)) > 0 {
			t.Fatal("false chord tolerance")
		}
	}
	if r.Mesh.SignedVolume() <= 0 {
		t.Fatal("orientation")
	}
	denied, e := TessellateWrappedKnotPatchCertified(ctx, wrapped, CertifiedPatchMeshOptions{Deflection: .001, MaxTriangleChecks: 1})
	if !errors.Is(e, geom.ErrConvergence) || len(denied.Mesh.Vertices) != 0 {
		t.Fatal("partial grid budget", e)
	}
}
