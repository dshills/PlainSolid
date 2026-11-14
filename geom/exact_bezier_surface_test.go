package geom

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"
)

func nonbinaryCylinderSurface(t *testing.T) *SurfacePatch {
	t.Helper()
	arc := [][3]*big.Rat{{big.NewRat(7, 1), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(7, 1), big.NewRat(26, 3), big.NewRat(1, 1)}, {big.NewRat(65, 3), big.NewRat(52, 3), big.NewRat(13, 9)}}
	var h [][4]*big.Rat
	for _, p := range arc {
		for _, z := range []int64{0, 3} {
			h = append(h, [4]*big.Rat{p[0], p[1], new(big.Rat).Mul(big.NewRat(z, 1), p[2]), p[2]})
		}
	}
	patch, e := NewExactHomogeneousBezierSurface(context.Background(), 2, 1, h, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	h[1][0].SetInt64(99)
	return patch
}
func TestExactBezierSurfaceNonbinaryPersistence(t *testing.T) {
	ctx := context.Background()
	p := nonbinaryCylinderSurface(t)
	if _, e := p.ToBSplineSurface(); !errors.Is(e, ErrUnrepresentable) {
		t.Fatal("rounded native pole", e)
	}
	recipe, e := p.ExportExactBezierRecipe()
	if e != nil {
		t.Fatal(e)
	}
	if recipe.Controls[2][1] != "26/3" {
		t.Fatal(recipe.Controls)
	}
	bytes, e := json.Marshal(recipe)
	if e != nil {
		t.Fatal(e)
	}
	var decoded ExactBezierSurfaceRecipe
	if e = json.Unmarshal(bytes, &decoded); e != nil {
		t.Fatal(e)
	}
	q, e := ImportExactBezierSurfaceRecipe(ctx, decoded, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, uv := range [][2]*big.Rat{{new(big.Rat), new(big.Rat)}, {big.NewRat(1, 3), big.NewRat(2, 7)}, {big.NewRat(1, 1), big.NewRat(1, 1)}} {
		a, e := p.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		b, e := q.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for i := 0; i < 3; i++ {
			if a.Point[i].Cmp(b.Point[i]) != 0 || a.Du[i].Cmp(b.Du[i]) != 0 || a.Dv[i].Cmp(b.Dv[i]) != 0 {
				t.Fatal(a, b)
			}
		}
		x := new(big.Rat).Sub(a.Point[0], big.NewRat(20, 1))
		radius := new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(a.Point[1], a.Point[1]))
		if radius.Cmp(big.NewRat(169, 1)) != 0 {
			t.Fatal("circle equation", a)
		}
		if new(big.Rat).Add(new(big.Rat).Mul(x, a.Du[0]), new(big.Rat).Mul(a.Point[1], a.Du[1])).Sign() != 0 || a.Dv[2].Cmp(big.NewRat(3, 1)) != 0 {
			t.Fatal("derivative identity", a)
		}
	}
	// Persist original nonunit parameter units independently of geometric controls.
	mapped, e := p.Reparameterize(ParameterRectangle{2, 5, -1, 1}, false, false, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := mapped.ExportExactBezierRecipe()
	if e != nil {
		t.Fatal(e)
	}
	back, e := ImportExactBezierSurfaceRecipe(ctx, r, SurfacePatchOptions{})
	if e != nil || back.Domain() != mapped.Domain() {
		t.Fatal(back, e)
	}
	a, e := mapped.EvaluateRational(ctx, big.NewRat(3, 1), new(big.Rat), SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := back.EvaluateRational(ctx, big.NewRat(3, 1), new(big.Rat), SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 3; i++ {
		if a.Point[i].Cmp(b.Point[i]) != 0 || a.Du[i].Cmp(b.Du[i]) != 0 {
			t.Fatal(a, b)
		}
	}
	recipe.Controls[2][1] = "999"
	fresh, _ := p.ExportExactBezierRecipe()
	if fresh.Controls[2][1] != "26/3" {
		t.Fatal("recipe aliases")
	}
}
func TestExactBezierSurfaceRecipeRejects(t *testing.T) {
	ctx := context.Background()
	p := nonbinaryCylinderSurface(t)
	recipe, _ := p.ExportExactBezierRecipe()
	for _, bad := range []string{"1e999999", "1/0", "01", "2/2", "-0", "nan", strings.Repeat("9", 100)} {
		r := recipe
		r.Controls = append([][4]string(nil), recipe.Controls...)
		r.Controls[0][0] = bad
		if q, e := ImportExactBezierSurfaceRecipe(ctx, r, SurfacePatchOptions{MaxCoefficientBits: 64}); e == nil || q != nil {
			t.Fatal(bad, q, e)
		}
	}
	for _, o := range []SurfacePatchOptions{{MaxPoles: 4}, {MaxOperations: 1}} {
		if q, e := ImportExactBezierSurfaceRecipe(ctx, recipe, o); e == nil || q != nil {
			t.Fatal(q, e)
		}
	}
	for _, degree := range []int{-1, 0, 65, 1000000} {
		r := recipe
		r.DegreeU = degree
		if q, e := ImportExactBezierSurfaceRecipe(ctx, r, SurfacePatchOptions{}); e == nil || q != nil {
			t.Fatal(q, e)
		}
	}
	r := recipe
	r.Controls = append([][4]string(nil), recipe.Controls...)
	r.Controls[2][3] = "0"
	if q, e := ImportExactBezierSurfaceRecipe(ctx, r, SurfacePatchOptions{}); e == nil || q != nil {
		t.Fatal(q, e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := ImportExactBezierSurfaceRecipe(cancelled, recipe, SurfacePatchOptions{}); !errors.Is(e, context.Canceled) || q != nil {
		t.Fatal(q, e)
	}
}
