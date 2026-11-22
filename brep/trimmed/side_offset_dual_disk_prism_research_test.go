//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"testing"
)

func TestOffsetSideOpenDualDiskNative(t *testing.T) {
	ctx := context.Background()
	ds := [2]ExtrusionCircle{{Center: geom.Vec2{X: 12.5, Y: -7.5}, Radius: 65. / 8}, {Center: geom.Vec2{X: 22, Y: -7.5}, Radius: 87. / 8}}
	prim := func(x, r float64) float64 { return (x*math.Sqrt(r*r-x*x) + r*r*math.Asin(x/r)) / 2 }
	removed := -7.5*(239./8-75./8) + prim(2, 65./8) - prim(-25./8, 65./8) + prim(63./8, 87./8) - prim(-7.5, 87./8)
	arc := (65./8)*(math.Asin(2/(65./8))-math.Asin(-25./65)) + (87./8)*(math.Asin(63./87)-math.Asin(-60./87))
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
			cut, e := CutOffsetSideOpenDualDisks(ctx, f, geom.Vec2{}, geom.Vec2{X: 40, Y: 10}, discs, 6, depth, 1e-10, trimvalidate.Options{})
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
				t.Fatal("analytic moments", m, analytic, err)
			}
			wantV := 2400 - depth*removed
			wantA := 1400 + (arc-(239./8-75./8))*depth
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
