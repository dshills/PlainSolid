package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"testing"
)

func TestUnequalCenterSideOpenDualDiskNative(t *testing.T) {
	ctx := context.Background()
	ds := [2]ExtrusionCircle{{Center: geom.Vec2{X: 7, Y: -3}, Radius: 5}, {Center: geom.Vec2{X: 30, Y: -20}, Radius: 29}}
	removed := 216.5*math.Pi - 409
	arc := 17 * math.Pi
	for _, sign := range []float64{1, -1} {
		for _, depth := range []float64{3, 6} {
			f, e := geom.NewFrame(geom.Vec3{X: 10, Y: 20, Z: 30}, geom.Vec3{X: 1}, geom.Vec3{Z: sign})
			if e != nil {
				t.Fatal(e)
			}
			discs := ds
			if sign < 0 {
				discs[0], discs[1] = discs[1], discs[0]
			}
			cut, e := CutUnequalCenterSideOpenDualDisks(ctx, f, geom.Vec2{}, geom.Vec2{X: 60, Y: 10}, discs, [2]*big.Rat{big.NewRat(10, 1), big.NewRat(1, 1)}, 6, depth, 1e-10, trimvalidate.Options{})
			if e != nil {
				t.Fatal(sign, depth, e)
			}
			if len(cut.WallFaces[0]) != 1 || len(cut.WallFaces[1]) != 1 || (cut.FloorFace < 0) != (depth == 6) {
				t.Fatal(cut.WallFaces, cut.FloorFace)
			}
			for owner, faces := range cut.WallFaces {
				for _, id := range faces {
					surface := cut.Solid.Faces()[id].Surface
					p, e := surface.Eval(.5, .5)
					if e != nil {
						t.Fatal(e)
					}
					local := f.ToLocal(p)
					d := discs[owner]
					if math.Abs((local.X-d.Center.X)*(local.X-d.Center.X)+(local.Y-d.Center.Y)*(local.Y-d.Center.Y)-d.Radius*d.Radius) > 1e-10 {
						t.Fatal("owner", owner, local)
					}
				}
			}
			m, e := cut.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-7})
			if e != nil {
				t.Fatal(e)
			}
			analytic, err := cut.Measures()
			if err != nil || math.Abs(m.Volume-analytic.Volume) > 1e-4 || math.Abs(m.Area-analytic.Area) > 1e-4 || m.Centroid.Sub(analytic.Centroid).Length() > 1e-6 {
				t.Fatal("unequal-center moments", m, analytic, err)
			}
			wantV := 3600 - depth*removed
			wantA := 2040 + (arc-48)*depth
			if depth == 6 {
				wantA -= 2 * removed
			}
			if math.Abs(m.Volume-wantV) > 1e-4 || math.Abs(m.Area-wantA) > 1e-4 {
				t.Fatalf("mass %+v want %g %g", m, wantV, wantA)
			}
			if sign == 1 && depth == 3 {
				var stream bytes.Buffer
				if e = WriteJSON(&stream, cut.Solid); e != nil {
					t.Fatal(e)
				}
				back, e := ReadJSONContext(ctx, &stream, JSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
				m, e = back.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-7})
				if e != nil || math.Abs(m.Volume-wantV) > 1e-4 || math.Abs(m.Area-wantA) > 1e-4 {
					t.Fatal("reload", m, e)
				}
			}
		}
	}
}

func TestUnequalCenterSideOpenDualDiskNativeOwnership(t *testing.T) {
	ctx := context.Background()
	f, e := geom.NewFrame(geom.Vec3{}, geom.Vec3{Y: -1}, geom.Vec3{Z: -1})
	if e != nil {
		t.Fatal(e)
	}
	ds := [2]ExtrusionCircle{{Center: geom.Vec2{X: 7, Y: -3}, Radius: 5}, {Center: geom.Vec2{X: 30, Y: -20}, Radius: 29}}
	join := [2]*big.Rat{big.NewRat(10, 1), big.NewRat(1, 1)}
	cut, e := CutUnequalCenterSideOpenDualDisks(ctx, f, geom.Vec2{}, geom.Vec2{X: 60, Y: 10}, ds, join, 6, 3, 1e-10, trimvalidate.Options{})
	if e != nil {
		t.Fatal(e)
	}
	join[0].SetInt64(0)
	var stream bytes.Buffer
	if e = WriteJSON(&stream, cut.Solid); e != nil {
		t.Fatal(e)
	}
	back, e := ReadJSONContext(ctx, &stream, JSONLimits{})
	if e != nil || len(back.Faces()) != len(cut.Solid.Faces()) {
		t.Fatal(e)
	}
	for _, faces := range cut.WallFaces {
		for _, id := range faces {
			a, e := cut.Solid.Faces()[id].Surface.Eval(.5, .5)
			if e != nil {
				t.Fatal(e)
			}
			b, e := back.Faces()[id].Surface.Eval(.5, .5)
			if e != nil || a != b {
				t.Fatal("stored support changed", a, b, e)
			}
		}
	}
	bad, e := CutUnequalCenterSideOpenDualDisks(ctx, f, geom.Vec2{}, geom.Vec2{X: 60, Y: 10}, ds, join, 6, 3, 1e-10, trimvalidate.Options{})
	if e == nil || len(bad.Solid.Faces()) != 0 {
		t.Fatal("bad join published", e)
	}
}
