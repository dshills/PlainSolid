package geom

import (
	"context"
	"testing"
)

func TestExactTensorSurfaceRecipe(t *testing.T) {
	ctx := context.Background()
	p := affineTriangleSupport(t, true, 0)
	r, e := p.ExportExactTensorRecipe()
	if e != nil {
		t.Fatal(e)
	}
	q, e := ImportExactTensorSurfaceRecipe(ctx, r, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	same, e := CompareExactSurfaces(ctx, p, q, ExactSurfaceComparisonOptions{})
	if e != nil || !same.Equal {
		t.Fatal(same, e)
	}
	for _, uv := range [][2]float64{{.25, .5}, {.1, .7}} {
		a, b, e := p.Partials(uv[0], uv[1])
		if e != nil {
			t.Fatal(e)
		}
		c, d, e := q.Partials(uv[0], uv[1])
		if e != nil || a != c || b != d {
			t.Fatal("reloaded derivatives")
		}
	}
	for _, mutate := range []func(*ExactTensorSurfaceRecipe){func(r *ExactTensorSurfaceRecipe) { r.KnotsU[0] = .125 }, func(r *ExactTensorSurfaceRecipe) { r.Controls[0][3] = "0" }, func(r *ExactTensorSurfaceRecipe) { r.Controls[0][0] = "1e100000" }, func(r *ExactTensorSurfaceRecipe) { r.CountU = 10000000 }} {
		bad, _ := p.ExportExactTensorRecipe()
		mutate(&bad)
		if got, e := ImportExactTensorSurfaceRecipe(ctx, bad, SurfacePatchOptions{}); e == nil || got != nil {
			t.Fatal("invalid recipe")
		}
	}
	if got, e := ImportExactTensorSurfaceRecipe(ctx, r, SurfacePatchOptions{MaxOperations: 1}); e == nil || got != nil {
		t.Fatal("budget")
	}
}
