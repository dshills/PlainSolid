package scene

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestPickWrappedMaterialsExactCapsAndTie(t *testing.T) {
	ctx := context.Background()
	makeCylinder := func(z float64) trimmed.Solid {
		t.Helper()
		w, err := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{Z: z}, 2, 4, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		s, _, err := trimmed.NewWrappedCylinder(ctx, w, 1e-9, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	camera, err := Orthographic(geom.Vec3{Z: 10}, geom.Vec3{}, geom.Vec3{Y: 1}, 8, 8, 1, 11)
	if err != nil {
		t.Fatal(err)
	}
	base, elevated := makeCylinder(0), makeCylinder(2)
	items := []WrappedPickItem{{ID: "far", Solid: base}, {ID: "second", Solid: elevated}, {ID: "first", Solid: base, Translation: geom.Vec3{Z: 2}}}
	got, hit, err := PickWrappedMaterials(ctx, camera, geom.Vec2{}, items, geom.ToroidLineOptions{})
	want := big.NewRat(3, 1)
	if err != nil || !hit || got.ID != "first" || got.Part != geom.CylinderUpperCap ||
		got.Distance.Min.Cmp(want) != 0 || got.Distance.Max.Cmp(want) != 0 {
		t.Fatalf("nearest exact cap: %+v, %t, %v", got, hit, err)
	}
	_, _, err = PickWrappedMaterials(ctx, camera, geom.Vec2{},
		append(items, WrappedPickItem{ID: "unsupported"}), geom.ToroidLineOptions{})
	if !errors.Is(err, geom.ErrConvergence) {
		t.Fatalf("unsupported candidate was not rejected: %v", err)
	}
}

func TestPickWrappedMaterialsThroughLoftBore(t *testing.T) {
	ctx := context.Background()
	body := func(r float64) trimmed.Solid {
		s, _, e := trimmed.LoftWrappedCircularStations(ctx, geom.Vec2{},
			[]trimmed.WrappedLoftStation{{Z: 0, Radius: r}, {Z: 4, Radius: r + 1}}, 1e-9, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	tube, _, err := trimmed.CombineCoaxialWrappedStationLofts(ctx, body(2), body(1), parametric.Difference, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	camera, err := Orthographic(geom.Vec3{Z: 8}, geom.Vec3{}, geom.Vec3{Y: 1}, 8, 8, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	items := []WrappedPickItem{{ID: "bore", Solid: tube}}
	if _, hit, err := PickWrappedMaterials(ctx, camera, geom.Vec2{}, items, geom.ToroidLineOptions{}); err != nil || hit {
		t.Fatal("center ray should pass through the bore", hit, err)
	}
	result, hit, err := PickWrappedMaterials(ctx, camera, geom.Vec2{X: .625}, items, geom.ToroidLineOptions{})
	if err != nil || !hit || result.Part != geom.CylinderUpperCap || result.Distance.Min.Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatal("expected exact upper annular cap hit", result, hit, err)
	}
}

func TestPickWrappedMaterialsToroidAndHole(t *testing.T) {
	ctx := context.Background()
	w, err := geom.NewCanonicalWrappedCircularToroid(ctx, geom.Vec3{}, 2, .5, .5, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	torus, _, err := trimmed.NewWrappedToroid(ctx, w, 1e-9, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	items := []WrappedPickItem{{ID: "ring", Solid: torus}}
	side, err := Orthographic(geom.Vec3{X: 6}, geom.Vec3{}, geom.Vec3{Z: 1}, 6, 6, 1, 12)
	if err != nil {
		t.Fatal(err)
	}
	got, hit, err := PickWrappedMaterials(ctx, side, geom.Vec2{}, items, geom.ToroidLineOptions{})
	want := big.NewRat(5, 2)
	if err != nil || !hit || got.ID != "ring" || got.Part != 0 ||
		got.Distance.Min.Cmp(want) > 0 || got.Distance.Max.Cmp(want) < 0 {
		t.Fatalf("first outer toroid crossing: %+v, %t, %v", got, hit, err)
	}
	top, err := Orthographic(geom.Vec3{Z: 6}, geom.Vec3{}, geom.Vec3{Y: 1}, 6, 6, 1, 12)
	if err != nil {
		t.Fatal(err)
	}
	if _, hit, err := PickWrappedMaterials(ctx, top, geom.Vec2{}, items, geom.ToroidLineOptions{}); err != nil || hit {
		t.Fatalf("center ray should pass through the toroid hole: %t, %v", hit, err)
	}
}

func TestPickWrappedMaterialsLoftCoplanarCap(t *testing.T) {
	ctx := context.Background()
	s, h, err := trimmed.LoftWrappedCircularStations(ctx, geom.Vec2{}, []trimmed.WrappedLoftStation{
		{Z: 0, Radius: 2}, {Z: 3, Radius: 1}, {Z: 5, Radius: 2},
	}, 1e-9, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// The center ray lies in the upper cap plane at y=1. The entrance
	// into that disk at x=sqrt(3) is both a rim hit and an overlap start.
	camera, err := Orthographic(geom.Vec3{X: 5, Y: 1, Z: 5}, geom.Vec3{Y: 1, Z: 5}, geom.Vec3{Z: 1}, 4, 4, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	got, hit, err := PickWrappedMaterials(ctx, camera, geom.Vec2{}, []WrappedPickItem{{ID: "loft", Solid: s}}, geom.ToroidLineOptions{})
	two := big.NewRat(2, 1)
	three := big.NewRat(3, 1)
	if err != nil || !hit || !got.Coincident || got.Distance.Min.Cmp(two) <= 0 || got.Distance.Max.Cmp(three) >= 0 ||
		got.Part != geom.CylinderLateral|geom.CylinderUpperCap || len(got.Faces) != 2 ||
		got.Faces[0] != h.WallFaces[1] || got.Faces[1] != h.CapFaces[1] {
		t.Fatal(got, hit, err)
	}
}

func TestPickWrappedCylinderFilletFace(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 3, 5, o)
	if e != nil {
		t.Fatal(e)
	}
	stock, _, e := trimmed.NewWrappedCylinder(ctx, w, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	body, h, e := trimmed.FilletWrappedCylinderRims(ctx, stock, .5, 1, o)
	if e != nil {
		t.Fatal(e)
	}
	c, e := Orthographic(geom.Vec3{X: 8, Z: .25}, geom.Vec3{Z: .25}, geom.Vec3{Z: 1}, 8, 8, 1, 16)
	if e != nil {
		t.Fatal(e)
	}
	got, hit, e := PickWrappedMaterials(ctx, c, geom.Vec2{}, []WrappedPickItem{{ID: "blend", Solid: body}}, geom.ToroidLineOptions{})
	if e != nil || !hit || got.Part != geom.CylinderLowerBlend || len(got.Faces) != 1 || got.Faces[0] != h.Blends[0] {
		t.Fatal(got, hit, e)
	}
}
