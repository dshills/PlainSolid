package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestLineSplineSurfaceExactRationalOriginAndScale(t *testing.T) {
	ctx := context.Background()
	patch := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	direction := [3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(-2, 7)}
	for _, v := range []*big.Rat{big.NewRat(3, 5), new(big.Rat)} {
		origin := [3]*big.Rat{big.NewRat(1, 3), v, big.NewRat(1, 3)}
		o := LineSplineSurfaceOptions{MaxParameter: 2}
		r, e := IntersectRationalLineSplineSurface(ctx, origin, direction, patch, o)
		if e != nil || !r.Complete || len(r.Hits) != 1 {
			t.Fatal("unproved contact for original exact coordinates", r, e)
		}
		h := r.Hits[0]
		brackets := func(lo, hi float64, p *big.Rat) bool {
			return new(big.Rat).SetFloat64(lo).Cmp(p) <= 0 && new(big.Rat).SetFloat64(hi).Cmp(p) >= 0
		}
		if !brackets(h.UV.Min.X, h.UV.Max.X, origin[0]) || !brackets(h.UV.Min.Y, h.UV.Max.Y, v) ||
			!brackets(h.ParameterMin, h.ParameterMax, big.NewRat(7, 6)) {
			t.Fatal("nonbinary contact enclosure must preserve caller's parameter scale", h)
		}
		crossing, e := h.RationalCrossing(direction, patch, SurfaceTransversalityOptions{})
		if e != nil || !crossing.ProvenRegular || crossing.Sign != -1 {
			t.Fatal("exact direction crossing", crossing, e)
		}
		o.MaxCoefficientBits = 64
		origin[0] = new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 128))
		r, e = IntersectRationalLineSplineSurface(ctx, origin, direction, patch, o)
		if !errors.Is(e, ErrConvergence) || r.Complete || len(r.Hits) != 0 || len(r.Pending) != 1 {
			t.Fatal("oversize exact inputs must retain unresolved cells", r, e)
		}
	}
}

func TestLineSplineSurfaceRegularAndMissing(t *testing.T) {
	patch := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	line, _ := NewLine(Vec3{.3, .4, -1}, Vec3{Z: 1})
	r, e := IntersectLineSplineSurface(line, patch, LineSplineSurfaceOptions{MinParameter: 0, MaxParameter: 2})
	if e != nil || !r.Complete || len(r.Hits) != 1 {
		t.Fatal(r, e)
	}
	h := r.Hits[0]
	if h.ParameterMin > 1 || h.ParameterMax < 1 || h.UV.Min.X > .3 || h.UV.Max.X < .3 || !h.Bounds.Contains(Vec3{.3, .4, 0}) {
		t.Fatal(h)
	}
	line, _ = NewLine(Vec3{2, 2, -1}, Vec3{Z: 1})
	r, e = IntersectLineSplineSurface(line, patch, LineSplineSurfaceOptions{MinParameter: 0, MaxParameter: 2})
	if e != nil || !r.Complete || len(r.Hits) != 0 {
		t.Fatal(r, e)
	}
}
func TestLineSplineSurfaceRationalCylinder(t *testing.T) {
	patch, _ := NewSurfacePatch(quarterCylinder(t), SurfacePatchOptions{})
	line, _ := NewLine(Vec3{0, 1, .73}, Vec3{X: 1})
	r, e := IntersectLineSplineSurface(line, patch, LineSplineSurfaceOptions{MinParameter: 1, MaxParameter: 2, MaxDepth: 18})
	if e != nil || !r.Complete || len(r.Hits) != 1 {
		t.Fatal(r, e)
	}
}
func TestLineSplineSurfaceTangentCoincidentAndLimits(t *testing.T) {
	patch, _ := NewSurfacePatch(quadraticContactSurface(t, 0), SurfacePatchOptions{})
	line, _ := NewLine(Vec3{0, .5, 0}, Vec3{X: 1})
	r, e := IntersectLineSplineSurface(line, patch, LineSplineSurfaceOptions{MinParameter: 0, MaxParameter: 1, MaxDepth: 6})
	if !errors.Is(e, ErrConvergence) || r.Complete || len(r.Pending) == 0 {
		t.Fatal(r, e)
	}
	patch = arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	r, e = IntersectLineSplineSurface(line, patch, LineSplineSurfaceOptions{MinParameter: 0, MaxParameter: 1, MaxDepth: 4})
	if !errors.Is(e, ErrConvergence) || r.Complete || len(r.Pending) == 0 {
		t.Fatal(r, e)
	}
	r, e = IntersectLineSplineSurface(line, patch, LineSplineSurfaceOptions{MinParameter: 0, MaxParameter: 1, MaxCells: 1})
	if !errors.Is(e, ErrConvergence) || len(r.Pending) == 0 {
		t.Fatal(r, e)
	}
}

func TestLineSplineSurfaceInternalBoundaryRescue(t *testing.T) {
	patch, _ := NewSurfacePatch(quarterCylinder(t), SurfacePatchOptions{})
	line, _ := NewLine(Vec3{0, 1, 1}, Vec3{X: 1})
	r, e := IntersectLineSplineSurface(line, patch, LineSplineSurfaceOptions{MinParameter: 1, MaxParameter: 2, MaxDepth: 12})
	if e != nil || !r.Complete || len(r.Hits) != 1 || r.Hits[0].UV.Min.Y > .5 || r.Hits[0].UV.Max.Y < .5 {
		t.Fatal(r, e)
	}
}

func TestLineSplineSurfaceOriginalEdgesAndCorners(t *testing.T) {
	patch := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	for _, uv := range []Vec2{{0, .3}, {1, .7}, {.2, 0}, {.8, 1}, {0, 0}, {1, 1}} {
		line, _ := NewLine(Vec3{uv.X, uv.Y, -1}, Vec3{Z: 1})
		r, e := IntersectLineSplineSurface(line, patch, LineSplineSurfaceOptions{MinParameter: 0, MaxParameter: 2})
		if e != nil || !r.Complete || len(r.Hits) != 1 || !r.Hits[0].Bounds.Contains(Vec3{uv.X, uv.Y, 0}) {
			t.Fatal(uv, r, e)
		}
	}
}
func TestLineSplineSurfaceCurvedOuterEdge(t *testing.T) {
	patch, _ := NewSurfacePatch(quarterCylinder(t), SurfacePatchOptions{})
	line, _ := NewLine(Vec3{0, 1, 0}, Vec3{X: 1})
	r, e := IntersectLineSplineSurface(line, patch, LineSplineSurfaceOptions{MinParameter: 1, MaxParameter: 2, MaxDepth: 16})
	if e != nil || !r.Complete || len(r.Hits) != 1 {
		t.Fatal(r, e)
	}
}
