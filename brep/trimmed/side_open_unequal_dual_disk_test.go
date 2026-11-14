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

func TestUnequalSideOpenDualDiskNative(t *testing.T) {
	ctx := context.Background()
	frame, e := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	discs := [2]ExtrusionCircle{{Center: geom.Vec2{X: 20}, Radius: 13}, {Center: geom.Vec2{X: 34}, Radius: 15}}
	angleA := math.Pi/2 + math.Asin(5./13)
	angleB := math.Pi/2 + math.Asin(9./15)
	removed := 84 + (169*angleA+225*angleB)/2
	arcLength := 13*angleA + 15*angleB
	for _, swap := range []bool{false, true} {
		for _, depth := range []float64{3, 6} {
			ds := discs
			if swap {
				ds[0], ds[1] = ds[1], ds[0]
			}
			section, e := CertifyUnequalSideOpenDualDiskSection(ctx, geom.Vec2{}, geom.Vec2{X: 60, Y: 22}, ds, trimvalidate.Options{})
			if e != nil || section.Join[0].Cmp(big.NewRat(25, 1)) != 0 || section.Join[1].Cmp(big.NewRat(12, 1)) != 0 {
				t.Fatal(section, e)
			}
			cut, e := CutUnequalSideOpenDualDisks(ctx, frame, geom.Vec2{}, geom.Vec2{X: 60, Y: 22}, ds, 6, depth, 1e-10, trimvalidate.Options{})
			if e != nil {
				t.Fatal("native", swap, depth, e)
			}
			if len(cut.WallFaces[0]) != 1 || len(cut.WallFaces[1]) != 1 || (cut.FloorFace >= 0) != (depth < 6) {
				t.Fatal(cut)
			}
			for owner, ids := range cut.WallFaces {
				patch, e := exactFaceSupportPatch(cut.Solid.Faces()[ids[0]], geom.SurfacePatchOptions{})
				if e != nil {
					t.Fatal(e)
				}
				point, e := patch.EvaluateRational(ctx, big.NewRat(1, 3), big.NewRat(1, 2), geom.SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				dx := new(big.Rat).Sub(point.Point[0], ratInset(ds[owner].Center.X))
				dy := new(big.Rat).Sub(point.Point[1], ratInset(ds[owner].Center.Y))
				residual := new(big.Rat).Add(new(big.Rat).Mul(dx, dx), new(big.Rat).Mul(dy, dy))
				radius := ratInset(ds[owner].Radius)
				if residual.Cmp(new(big.Rat).Mul(radius, radius)) != 0 {
					t.Fatal("original wall owner", owner, point)
				}
			}
			measures, e := cut.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-7})
			if e != nil {
				t.Fatal(e)
			}
			wantV := 7920 - depth*removed
			wantA := 3624 + (arcLength-42)*depth
			if depth == 6 {
				wantA -= 2 * removed
			}
			analytic, e := cut.Measures()
			if e != nil || math.Abs(analytic.Volume-measures.Volume) > 1e-4 || math.Abs(analytic.Area-measures.Area) > 1e-4 || analytic.Centroid.Sub(measures.Centroid).Length() > 1e-6 {
				t.Fatal("asymmetric analytic moments", analytic, measures, e)
			}
			if math.Abs(measures.Volume-wantV) > 1e-4 || math.Abs(measures.Area-wantA) > 1e-4 {
				t.Fatal(measures, wantV, wantA)
			}
			if !swap && depth == 3 {
				var stream bytes.Buffer
				if e := WriteJSON(&stream, cut.Solid); e != nil {
					t.Fatal(e)
				}
				restored, e := ReadJSONContext(ctx, &stream, JSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
				if len(restored.Faces()) != len(cut.Solid.Faces()) {
					t.Fatal("reload topology")
				}
			}
		}
	}
	// Legacy entry point cannot expose equal-radius Measures on this geometry.
	if _, e := CutSideOpenDualDisks(ctx, frame, geom.Vec2{}, geom.Vec2{X: 60, Y: 22}, discs, 6, 3, 1e-10, trimvalidate.Options{}); e == nil {
		t.Fatal("legacy broadened")
	}
	for _, bad := range [][2]ExtrusionCircle{{{Center: geom.Vec2{X: 20}, Radius: 13}, {Center: geom.Vec2{X: 48}, Radius: 15}}, {{Center: geom.Vec2{X: 20}, Radius: 13}, {Center: geom.Vec2{X: 22}, Radius: 15}}, {{Center: geom.Vec2{X: 20}, Radius: 13}, {Center: geom.Vec2{X: 33}, Radius: 15}}} {
		r, e := CertifyUnequalSideOpenDualDiskSection(ctx, geom.Vec2{}, geom.Vec2{X: 80, Y: 22}, bad, trimvalidate.Options{})
		if e == nil || len(r.Outer) != 0 {
			t.Fatal("unqualified accepted", r, e)
		}
	}
}
