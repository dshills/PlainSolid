package intersect

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func affineTrimUV(x, y int64) [2]*big.Rat { return [2]*big.Rat{big.NewRat(x, 1), big.NewRat(y, 1)} }
func affineTrimSquare() [][2]*big.Rat {
	return [][2]*big.Rat{affineTrimUV(0, 0), affineTrimUV(1, 0), affineTrimUV(1, 1), affineTrimUV(0, 1)}
}
func TestConvexAffineOriginalTrimContact(t *testing.T) {
	ctx := context.Background()
	d := geom.ParameterRectangle{UMax: 1, VMax: 1}
	embed := func(x, y float64) geom.Vec3 { return geom.Vec3{X: x, Y: y, Z: x + 2*y} }
	a := affineContactPatch(t, embed, d, true)
	b := affineContactPatch(t, func(u, v float64) geom.Vec3 { return embed(2*u, v) }, d, false)
	triangle := [][2]*big.Rat{affineTrimUV(0, 0), affineTrimUV(1, 0), affineTrimUV(0, 1)}
	clip := affineTrimSquare()
	clip[0][0] = big.NewRat(1, 6)
	clip[3][0] = big.NewRat(1, 6)
	loops := [2][][2]*big.Rat{triangle, clip}
	q, e := IntersectConvexAffinePatches(ctx, a, b, loops, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	s := q.Contact()
	kind, _ := s.Kind()
	area, squared, orientation, e := s.CoplanarArea()
	if kind != AffinePatchContactArea || e != nil || area.Cmp(big.NewRat(2, 9)) != 0 || squared.Cmp(big.NewRat(8, 27)) != 0 || orientation != 1 {
		t.Fatal(kind, area, squared, orientation, e)
	}
	vertices, _ := s.Vertices()
	if len(vertices) != 3 {
		t.Fatal(vertices)
	}
	seen := false
	for _, v := range vertices {
		if v.A[0].Cmp(big.NewRat(1, 3)) == 0 && v.A[1].Cmp(big.NewRat(2, 3)) == 0 {
			seen = true
			if v.B[0].Cmp(big.NewRat(1, 6)) != 0 {
				t.Fatal(v)
			}
		}
	}
	if !seen {
		t.Fatal("missing exact rational trim junction")
	}
	sides := q.EdgeTrimSides()
	if len(sides) != 3 {
		t.Fatal(sides)
	}
	for _, owners := range sides {
		if len(owners[0])+len(owners[1]) == 0 {
			t.Fatal("unowned exterior")
		}
	}
	vs := q.VertexTrimSides()
	vs[0][0] = append(vs[0][0], 99)
	sides[0][0] = append(sides[0][0], 99)
	for _, sides := range q.VertexTrimSides() {
		for _, owner := range sides {
			for _, side := range owner {
				if side == 99 {
					t.Fatal("mutable ancestry")
				}
			}
		}
	}
	for _, cap := range []int{s.ExactOperations(), s.ExactOperations() - 1} {
		out, e := IntersectConvexAffinePatches(ctx, a, b, loops, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{MaxOperations: cap})
		if cap == s.ExactOperations() {
			if e != nil || out == nil {
				t.Fatal(e)
			}
		} else if e == nil || out != nil {
			t.Fatal("non-atomic budget")
		}
	}
	reversed := append([][2]*big.Rat(nil), clip...)
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	r, e := IntersectConvexAffinePatches(ctx, a, b, [2][][2]*big.Rat{triangle, reversed}, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	ra, _, _, _ := r.Contact().CoplanarArea()
	if ra.Cmp(area) != 0 {
		t.Fatal(ra)
	}
	clip[0][0].SetInt64(99)
	again, _ := s.Vertices()
	for _, v := range again {
		if v.A[0].Cmp(big.NewRat(99, 1)) == 0 || v.B[0].Cmp(big.NewRat(99, 1)) == 0 {
			t.Fatal("retained mutable input")
		}
	}
}
func TestConvexAffineTrimTransverseAndLimits(t *testing.T) {
	ctx := context.Background()
	d := geom.ParameterRectangle{UMax: 1, VMax: 1}
	a := affineContactPatch(t, func(u, v float64) geom.Vec3 { return geom.Vec3{X: u, Y: v} }, d, false)
	b := affineContactPatch(t, func(u, v float64) geom.Vec3 { return geom.Vec3{X: .5, Y: u, Z: v - .5} }, d, false)
	triangle := [][2]*big.Rat{affineTrimUV(0, 0), affineTrimUV(1, 0), affineTrimUV(0, 1)}
	q, e := IntersectConvexAffinePatches(ctx, a, b, [2][][2]*big.Rat{triangle, affineTrimSquare()}, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	kind, _ := q.Contact().Kind()
	v, _ := q.Contact().Vertices()
	if kind != AffinePatchContactSegment || len(v) != 2 {
		t.Fatal(kind, v)
	}
	for _, p := range v {
		if p.A[0].Cmp(big.NewRat(1, 2)) != 0 || p.A[1].Sign() < 0 || p.A[1].Cmp(big.NewRat(1, 2)) > 0 {
			t.Fatal(p)
		}
	}
	bad := [][][2]*big.Rat{
		{affineTrimUV(0, 0), affineTrimUV(1, 1), affineTrimUV(0, 1), affineTrimUV(1, 0)},
		{affineTrimUV(0, 0), affineTrimUV(1, 0), {big.NewRat(1, 4), big.NewRat(1, 4)}, affineTrimUV(0, 1)},
		{affineTrimUV(0, 0), affineTrimUV(1, 0), affineTrimUV(1, 0), affineTrimUV(0, 1)},
		{affineTrimUV(0, 0), affineTrimUV(2, 0), affineTrimUV(0, 1)},
		{{nil, big.NewRat(0, 1)}, affineTrimUV(1, 0), affineTrimUV(0, 1)},
	}
	for _, loop := range bad {
		out, e := IntersectConvexAffinePatches(ctx, a, b, [2][][2]*big.Rat{loop, affineTrimSquare()}, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
		if e == nil || out != nil {
			t.Fatal("invalid loop accepted", loop)
		}
	}
	for _, o := range []AffinePolygonContactOptions{{MaxOperations: 1}, {MaxEdges: 3}, {MaxCoefficientBits: 63}} {
		out, e := IntersectConvexAffinePatches(ctx, a, b, [2][][2]*big.Rat{triangle, affineTrimSquare()}, geom.SurfacePatchOptions{}, o)
		if e == nil || out != nil {
			t.Fatal("limits", o)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	out, e := IntersectConvexAffinePatches(canceled, a, b, [2][][2]*big.Rat{triangle, affineTrimSquare()}, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
	if e != context.Canceled || out != nil {
		t.Fatal(out, e)
	}
}

func TestConvexAffineTrimDegenerateFootprints(t *testing.T) {
	d := geom.ParameterRectangle{UMax: 1, VMax: 1}
	a := affineContactPatch(t, func(u, v float64) geom.Vec3 { return geom.Vec3{X: u, Y: v} }, d, false)
	triangle := [][2]*big.Rat{affineTrimUV(0, 0), affineTrimUV(1, 0), affineTrimUV(0, 1)}
	for _, tc := range []struct {
		loop [][2]*big.Rat
		kind AffinePatchContactKind
	}{
		{[][2]*big.Rat{affineTrimUV(1, 0), affineTrimUV(1, 1), affineTrimUV(0, 1)}, AffinePatchContactSegment},
		{[][2]*big.Rat{affineTrimUV(1, 0), affineTrimUV(1, 1), {big.NewRat(1, 2), big.NewRat(1, 1)}}, AffinePatchContactPoint},
		{[][2]*big.Rat{{big.NewRat(3, 4), big.NewRat(3, 4)}, affineTrimUV(1, 1), {big.NewRat(3, 4), big.NewRat(1, 1)}}, AffinePatchContactEmpty},
	} {
		out, e := IntersectConvexAffinePatches(context.Background(), a, a, [2][][2]*big.Rat{triangle, tc.loop}, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
		if e != nil {
			t.Fatal(e)
		}
		kind, _ := out.Contact().Kind()
		area, squared, _, e := out.Contact().CoplanarArea()
		if kind != tc.kind || e != nil || area.Sign() != 0 || squared.Sign() != 0 {
			t.Fatal(kind, area, squared, e)
		}
	}
}
