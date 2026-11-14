package trimmed

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"plainsolid/geom"
)

func TestExactSupportRectangleShellEmbedding(t *testing.T) {
	ctx := context.Background()
	s := testWarpedSplinePrism(t, false, false)
	base := s.Faces()[1].Surface.(*geom.BSplineSurface)
	p, e := geom.NewSurfacePatch(base, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	recipe, e := p.ExportExactBezierRecipe()
	if e != nil {
		t.Fatal(e)
	}
	// Raise one interior control by 1/3, leaving all original model edges
	// unchanged. The resulting roof remains a positive projected graph.
	recipe.Controls[4][2] = "19/12"
	p, e = geom.ImportExactBezierSurfaceRecipe(ctx, recipe, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.ToBSplineSurface(); !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("fixture must require exact wall controls", e)
	}
	roof, e := geom.NewExactBezierSurface(p)
	if e != nil {
		t.Fatal(e)
	}
	parts := make([]SewFace, 6)
	for i, f := range s.Faces() {
		if i == 1 {
			parts[i], e = ExactSplineFaceRectangle(ctx, roof, geom.ParameterRectangle{}, f.Reversed, geom.SurfacePatchOptions{})
		} else {
			parts[i], e = RectangularSplineFace(f.Surface.(*geom.BSplineSurface), f.Reversed)
		}
		if e != nil {
			t.Fatal(i, e)
		}
	}
	s, _, e = Sew(ctx, parts, SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	check := func(s Solid) {
		t.Helper()
		r, e := s.ValidateSplineRectangleEmbedding(ctx, SplineRectangleEmbeddingOptions{})
		if e != nil || !r.Complete || len(r.Faces) != 6 || len(r.Pairs) != 15 || !r.Faces[1].ProvenInjective {
			t.Fatal("exact original support did not qualify as an embedded shell", r.Complete, e)
		}
	}
	check(s)
	var raw bytes.Buffer
	if e = WriteJSON(&raw, s); e != nil {
		t.Fatal(e)
	}
	res, e := ReadJSON(&raw, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	check(res)
	// A subrectangle with a nonrepresentable world corner must fail, even
	// though the selected UV bound is represented exactly.
	if f, e := ExactSplineFaceRectangle(ctx, roof, geom.ParameterRectangle{UMin: .5, UMax: 1, VMin: .5, VMax: 1}, false, geom.SurfacePatchOptions{}); !errors.Is(e, geom.ErrUnrepresentable) || len(f.Vertices) != 0 {
		t.Fatal("nonbinary vertex accepted without an exact endpoint layout", e)
	}
}
