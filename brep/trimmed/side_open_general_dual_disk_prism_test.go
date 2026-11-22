//go:build research

package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestGeneralSideOpenDiskNative(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	ds := [2]ExtrusionCircle{{Center: geom.Vec2{X: 20}, Radius: 5}, {Center: geom.Vec2{X: 24.5}, Radius: 8.5}}
	for _, sign := range []float64{1, -1} {
		f, e := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: sign})
		if e != nil {
			t.Fatal(e)
		}
		for _, depth := range []float64{3, 6} {
			for _, swap := range []bool{false, true} {
				discs := ds
				if swap {
					discs[0], discs[1] = discs[1], discs[0]
				}
				cut, e := CutGeneralSideOpenDualDisks(ctx, f, geom.Vec2{}, geom.Vec2{X: 50, Y: 12}, discs, 6, depth, 1e-10, trimvalidate.Options{})
				if e != nil {
					t.Fatal(sign, depth, swap, e)
				}
				measures, e := cut.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-7})
				if e != nil {
					t.Fatal(e)
				}
				analytic, e := cut.Measures()
				if e != nil || math.Abs(measures.Volume-analytic.Volume) > 1e-4 || math.Abs(measures.Area-analytic.Area) > 1e-4 || measures.Centroid.Sub(analytic.Centroid).Length() > 1e-6 {
					t.Fatal(measures, analytic, e)
				}
				thetaL, thetaR := math.Pi/2-math.Asin(.6), math.Pi/2+math.Asin(15./17)
				removed := 9 + (25*thetaL+72.25*thetaR)/2
				wantV := 3600 - depth*removed
				wantA := 1944 + (5*thetaL+8.5*thetaR-18)*depth
				if depth == 6 {
					wantA -= 2 * removed
				}
				if math.Abs(analytic.Volume-wantV) > 1e-9 || math.Abs(analytic.Area-wantA) > 1e-9 {
					t.Fatal(analytic, wantV, wantA)
				}
				at, e := cut.Locate(ctx, f.ToWorld(geom.Vec3{X: 17, Y: 4, Z: 1}))
				if e != nil || at != parametric.Boundary {
					t.Fatal(at, e)
				}
				if sign == 1 && depth == 3 && !swap {
					var stream bytes.Buffer
					if e := WriteJSON(&stream, cut.Solid); e != nil {
						t.Fatal(e)
					}
					back, e := ReadJSONContext(ctx, &stream, JSONLimits{})
					if e != nil || len(back.Faces()) != len(cut.Solid.Faces()) {
						t.Fatal(e)
					}
				}
			}
		}
	}
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	// Exact section is valid, but its required whole-arc pole 26/3 is not
	// representable in the stored native surface. Never silently snap it.
	bad := [2]ExtrusionCircle{{Center: geom.Vec2{X: 20}, Radius: 13}, {Center: geom.Vec2{X: 24}, Radius: 15}}
	if r, e := CutGeneralSideOpenDualDisks(ctx, f, geom.Vec2{}, geom.Vec2{X: 60, Y: 22}, bad, 6, 3, 1e-10, trimvalidate.Options{}); !errors.Is(e, geom.ErrUnrepresentable) || len(r.Solid.Faces()) != 0 {
		t.Fatal("inexact pole accepted", e)
	}
}
