//go:build research

package trimmed

import (
	"context"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func TestTrimmedCanonicalToroidAxialAdapter(t *testing.T) {
	ctx := context.Background()
	recipe, e := independentWrappedToroid(t).ExportRecipe()
	if e != nil {
		t.Fatal(e)
	}
	d := recipe.Surface.Data()
	weights := []float64{1, 1, 2, 2, 4, 2, 2, 1, 1}
	for i := 0; i < 9; i++ {
		for j := 0; j < 9; j++ {
			d.Weights[i*9+j] = weights[i] * weights[j]
		}
	}
	source, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	patch, e := geom.NewSurfacePatch(source, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, _, e := geom.NewWrappedSurfacePatch(ctx, patch, recipe.Options)
	if e != nil {
		t.Fatal(e)
	}
	s, _, e := NewWrappedToroid(ctx, w, 1e-9, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	roots, e := s.IntersectCanonicalToroidAxial(ctx, 2, 0, geom.ToroidAxialOptions{AbsoluteTolerance: 1e-10})
	if e != nil || len(roots) != 2 || roots[1].MinZ.RatString() != "1/2" {
		t.Fatal(roots, e)
	}
	split, _, e := s.SplitSplineEdgeExact(ctx, 0, .5, edgecheck.Options{})
	if e != nil {
		t.Fatal(e)
	}
	if roots, e := split.IntersectCanonicalToroidAxial(ctx, 2, 0, geom.ToroidAxialOptions{AbsoluteTolerance: 1e-10}); e == nil || roots != nil {
		t.Fatal("edited seam partition should fail narrow adapter", e)
	}
}
