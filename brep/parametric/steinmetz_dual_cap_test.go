package parametric

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/collision"
	"plainsolid/geom"
)

func TestFiniteSteinmetzDualCaps(t *testing.T) {
	ctx := context.Background()
	for _, limits := range [][2][2]float64{{{-6, 2}, {-6, 3}}, {{-6, -2}, {-6, 3}}, {{2, 6}, {-6, -3}}, {{-6, 2}, {-6, 2}}} {
		a := geom.RawCylinder{Axis: geom.Vec3{X: 1}, Radius: 5, Min: limits[0][0], Max: limits[0][1]}
		b := geom.RawCylinder{Axis: geom.Vec3{Z: 1}, Radius: 5, Min: limits[1][0], Max: limits[1][1]}
		out, err := FiniteCylinderPairCommon(ctx, a, b, 1e-8)
		if err != nil || out.Construction != CylinderCommonPerpendicularDualCap {
			t.Fatal(limits, out.Construction, err)
		}
		for owner := range 2 {
			cap := limits[owner][1]
			ids := out.UpperCapFaces[owner]
			if limits[owner][1] >= 5 {
				cap = limits[owner][0]
				ids = out.LowerCapFaces[owner]
			}
			if len(ids) == 0 || len(out.SideFaces[owner]) == 0 {
				t.Fatal("lost support ownership", owner, limits)
			}
			for _, id := range ids {
				f := out.Solid.Faces()[id]
				p, e := f.Surface.Eval((f.U.Min+f.U.Max)/2, .5)
				if e != nil || [2]float64{p.X, p.Z}[owner] != cap {
					t.Fatal(p, cap, e)
				}
			}
		}
		p, err := out.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-6})
		if err != nil {
			t.Fatal(err)
		}
		// Integrate rectangular sections independently over model Y.
		volume := 0.
		const n = 20000
		for i := range n {
			y := -5 + 10*(float64(i)+.5)/n
			r := math.Sqrt(25 - y*y)
			w := func(c [2]float64) float64 { return max(0, min(r, c[1])-max(-r, c[0])) }
			volume += w(limits[0]) * w(limits[1]) * 10 / n
		}
		if math.Abs(p.Volume-volume) > 1e-4 {
			t.Fatal(limits, p.Volume, volume)
		}
		var buf bytes.Buffer
		if e := WriteJSON(&buf, out.Solid); e != nil {
			t.Fatal(e)
		}
		restored, e := ReadJSON(&buf, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = restored.Bounds(); e != nil {
			t.Fatal(e)
		}
		if limits == ([2][2]float64{{-6, 2}, {-6, 3}}) {
			m, e := restored.Tessellate(ctx, TessellationOptions{Deflection: .65, AnalyticBound: true})
			if e != nil || m.ValidateSolid() != nil || m.SignedVolume() <= 0 {
				t.Fatal("certified mesh", e)
			}
			report, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
			if e != nil || !report.Complete || len(report.Contacts) != 0 {
				t.Fatal(report, e)
			}
		}
		got, e := restored.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-6})
		if e != nil || p != got {
			t.Fatal(got, p, e)
		}
	}
}
