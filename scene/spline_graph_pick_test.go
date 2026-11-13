package scene

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestPickSplineGraphEnvelope(t *testing.T) {
	ctx := context.Background()
	graph := func(z, slope float64) *geom.BSplineSurface {
		t.Helper()
		k := []float64{0, 0, 1, 1}
		p := []geom.Vec3{{Z: z}, {Y: 1, Z: z}, {X: 1, Z: z + slope}, {X: 1, Y: 1, Z: z + slope}}
		s, e := geom.NewBSplineSurface(1, 1, 2, 2, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	body, e := trimmed.NewSharedBaseSplineGraphMaterial(ctx, graph(0, 0), graph(2, 1), graph(2.5, 0), intersect.SplinePlaneOptions{})
	if e != nil {
		t.Fatal(e)
	}
	items := []SplineGraphPickItem{
		{ID: "far", Graph: body, Operation: parametric.Union},
		{ID: "near", Graph: body, Operation: parametric.Union, Translation: geom.Vec3{Z: 1}},
	}
	p := geom.Vec3{X: .5, Y: .5}
	camera, e := Orthographic(geom.Vec3{X: .5, Y: .5, Z: 8}, p, geom.Vec3{Y: 1}, 2, 2, 1, 10)
	if e != nil {
		t.Fatal(e)
	}
	h, hit, e := PickSplineGraphEnvelopes(ctx, camera, geom.Vec2{}, items, geom.ToroidLineOptions{})
	if e != nil || !hit || h.ID != "near" || len(h.TopFaces) != 2 || h.Parameter.Min.Cmp(big.NewRat(7, 2)) != 0 {
		t.Fatal(h, hit, e)
	}
	ambiguous := []SplineGraphPickItem{{ID: "duplicate", Graph: body}, {ID: "duplicate", Graph: body}}
	if _, hit, e := PickSplineGraphEnvelopes(ctx, camera, geom.Vec2{}, ambiguous, geom.ToroidLineOptions{}); !errors.Is(e, geom.ErrInvalid) || hit {
		t.Fatal("duplicate ID", hit, e)
	}

	camera, e = Orthographic(geom.Vec3{X: 3, Y: .5, Z: 1}, geom.Vec3{Y: .5, Z: 1}, geom.Vec3{Z: 1}, 2, 2, 1, 8)
	if e != nil {
		t.Fatal(e)
	}
	h, hit, e = PickSplineGraphEnvelopes(ctx, camera, geom.Vec2{}, items[:1], geom.ToroidLineOptions{})
	if e != nil || !hit || len(h.Sides) != 1 || h.Sides[0] != 1 || h.Parameter.Min.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal(h, hit, e)
	}
	camera, e = Orthographic(geom.Vec3{X: 3, Z: 1}, geom.Vec3{Z: 1}, geom.Vec3{Z: 1}, 2, 2, 1, 8)
	if e != nil {
		t.Fatal(e)
	}
	h, hit, e = PickSplineGraphEnvelopes(ctx, camera, geom.Vec2{}, items[:1], geom.ToroidLineOptions{})
	if e != nil || !hit || !h.Coincident || len(h.Sides) != 2 || h.Parameter.Min.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal(h, hit, e)
	}
}
