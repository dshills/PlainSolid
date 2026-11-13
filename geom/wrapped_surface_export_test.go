package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestWrappedSurfaceLosslessRecipeAndPlacement(t *testing.T) {
	ctx := context.Background()
	w, _, e := NewWrappedSurfacePatch(ctx, wrappedCylinderPatch(t), WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	recipe, e := w.ExportRecipe()
	if e != nil || !recipe.Options.WrapU || recipe.Options.WrapV || recipe.Options.ContinuityOrder != 1 {
		t.Fatal(recipe, e)
	}
	p, e := NewSurfacePatch(recipe.Surface, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	rebuilt, seam, e := NewWrappedSurfacePatch(ctx, p, recipe.Options)
	if e != nil || !seam.FirstPartialsMatched[0] {
		t.Fatal(seam, e)
	}
	a, e := w.EvaluateRational(ctx, big.NewRat(1, 3), big.NewRat(2, 5), SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := rebuilt.EvaluateRational(ctx, big.NewRat(1, 3), big.NewRat(2, 5), SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 3; i++ {
		if a.Point[i].Cmp(b.Point[i]) != 0 {
			t.Fatal(a, b)
		}
	}
	placement := Identity()
	placement[0][1] = 2
	placement[2][3] = .25
	moved, seam, e := w.Transformed(ctx, placement, WrappedSurfaceOptions{})
	if e != nil || !seam.FirstPartialsMatched[0] {
		t.Fatal(seam, e)
	}
	b, e = moved.EvaluateRational(ctx, big.NewRat(1, 3), big.NewRat(2, 5), SurfacePatchEvaluationOptions{})
	if e != nil || b.Point[0].Cmp(new(big.Rat).Add(a.Point[0], new(big.Rat).Mul(big.NewRat(2, 1), a.Point[1]))) != 0 {
		t.Fatal(b, e)
	}
	tiny, _, e := w.Transformed(ctx, Translation(Vec3{X: math.Ldexp(1, -80)}), WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = tiny.ExportRecipe(); !errors.Is(e, ErrUnrepresentable) {
		t.Fatal("rounded recipe", e)
	}
	collapsed, proof, e := w.Transformed(ctx, Scaling(Vec3{}), WrappedSurfaceOptions{})
	if e != nil || !proof.FirstPartialsMatched[0] {
		t.Fatal(proof, e)
	}
	zero, e := collapsed.Eval(9, .5)
	if e != nil || zero != (Vec3{}) {
		t.Fatal(zero, e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, _, e = w.Transformed(canceled, placement, WrappedSurfaceOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

// The dyadic approximation weight has exactly representable tensor products,
// making seam identities exact without claiming an ideal analytic torus.
func wrappedTorusPatch(t *testing.T) *WrappedSurfacePatch {
	t.Helper()
	circle := []Vec2{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	var poles []Vec3
	var weights []float64
	for i, u := range circle {
		wu := 1.
		if i%2 == 1 {
			wu = math.Round(math.Sqrt(.5)*(1<<24)) / (1 << 24)
		}
		for j, v := range circle {
			wv := 1.
			if j%2 == 1 {
				wv = math.Round(math.Sqrt(.5)*(1<<24)) / (1 << 24)
			}
			poles = append(poles, Vec3{(3 + v.X) * u.X, (3 + v.X) * u.Y, v.Y})
			weights = append(weights, wu*wv)
		}
	}
	knots := []float64{0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4}
	s, e := NewBSplineSurface(2, 2, 9, 9, poles, knots, knots, weights)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, seam, e := NewWrappedSurfacePatch(context.Background(), p, WrappedSurfaceOptions{WrapU: true, WrapV: true, ContinuityOrder: 1})
	if e != nil || seam.FirstPartialsMatched != ([2]bool{true, true}) {
		t.Fatal(seam, e)
	}
	return w
}

func TestWrappedRationalTensorTorusRecipe(t *testing.T) {
	w := wrappedTorusPatch(t)
	recipe, e := w.ExportRecipe()
	if e != nil || !recipe.Options.WrapV {
		t.Fatal(recipe, e)
	}
	for _, uv := range []Vec2{{.25, .75}, {-3.75, 8.75}, {4, 4}} {
		point, e := w.Eval(uv.X, uv.Y)
		if e != nil {
			t.Fatal(e)
		}
		radial := math.Hypot(point.X, point.Y) - 3
		if math.Abs(radial*radial+point.Z*point.Z-1) > 1e-6 {
			t.Fatal(point)
		}
	}
}
