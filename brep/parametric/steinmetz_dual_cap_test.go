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

func TestFiniteSteinmetzFourCapWindows(t *testing.T) {
	ctx := context.Background()
	for _, caps := range [][2][2]float64{{{-1, 2}, {-3, 4}}, {{1, 2}, {-4, -3}}, {{-1, 2}, {-3, 6}}} {
		a := geom.RawCylinder{Axis: geom.Vec3{X: 1}, Radius: 5, Min: caps[0][0], Max: caps[0][1]}
		b := geom.RawCylinder{Axis: geom.Vec3{Z: 1}, Radius: 5, Min: caps[1][0], Max: caps[1][1]}
		out, e := FiniteCylinderPairCommon(ctx, a, b, 1e-8)
		if e != nil || out.Construction != CylinderCommonPerpendicularCapWindows {
			t.Fatal(caps, out.Construction, e)
		}
		faces := out.Solid.Faces()
		owned := make(map[int]bool)
		for axis := range 2 {
			for _, id := range out.SideFaces[axis] {
				if owned[id] {
					t.Fatal("face has multiple owners", id)
				}
				owned[id] = true
			}
			for side, ids := range [2][]int{out.LowerCapFaces[axis], out.UpperCapFaces[axis]} {
				cap := caps[axis][side]
				// The opposing tail can exclude the Y sections at which a
				// radial boundary is active, but each interior cap meets y=0.
				if (math.Abs(cap) < 5) != (len(ids) > 0) {
					t.Fatal(caps, axis, side, ids)
				}
				for _, id := range ids {
					if owned[id] {
						t.Fatal("face has multiple owners", id)
					}
					owned[id] = true
					f := faces[id]
					p, e := f.Surface.Eval((f.U.Min+f.U.Max)/2, .5)
					if e != nil || [2]float64{p.X, p.Z}[axis] != cap {
						t.Fatal(p, cap, e)
					}
				}
			}
		}
		if len(owned) != len(faces) {
			t.Fatal("unowned faces")
		}
		m, e := out.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil {
			t.Fatal(e)
		}
		volume := 0.
		const n = 20000
		for i := range n {
			y := -5 + 10*(float64(i)+.5)/n
			r := math.Sqrt(25 - y*y)
			width := func(w [2]float64) float64 { return max(0, min(r, w[1])-max(-r, w[0])) }
			volume += width(caps[0]) * width(caps[1]) * 10 / n
		}
		if math.Abs(m.Volume-volume) > 1e-4 {
			t.Fatal(caps, m.Volume, volume)
		}
		var native bytes.Buffer
		if e := WriteJSON(&native, out.Solid); e != nil {
			t.Fatal(e)
		}
		s, e := ReadJSON(&native, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		if caps == ([2][2]float64{{-1, 2}, {-3, 4}}) {
			mesh, e := s.Tessellate(ctx, TessellationOptions{Deflection: .7, AnalyticBound: true})
			if e != nil || mesh.ValidateSolid() != nil || mesh.SignedVolume() <= 0 {
				t.Fatal("four-cap certified mesh", e)
			}
			report, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
			if e != nil || !report.Complete || len(report.Contacts) != 0 {
				t.Fatal(report, e)
			}
		}
		p, e := s.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil || p != m {
			t.Fatal(p, m, e)
		}
	}
}

func TestFiniteSteinmetzUnequalCapWindows(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		radii [2]float64
		caps  [2][2]float64
	}{
		{[2]float64{3, 5}, [2][2]float64{{-2, 4.5}, {-2, 1}}},
		{[2]float64{5, 3}, [2][2]float64{{.5, 2}, {-4.5, 3.5}}},
		{[2]float64{3, 5}, [2][2]float64{{4, 4.5}, {-2, 1}}},
	} {
		a := geom.RawCylinder{Axis: geom.Vec3{X: 1}, Radius: tc.radii[0], Min: tc.caps[0][0], Max: tc.caps[0][1]}
		b := geom.RawCylinder{Axis: geom.Vec3{Z: 1}, Radius: tc.radii[1], Min: tc.caps[1][0], Max: tc.caps[1][1]}
		out, e := FiniteCylinderPairCommon(ctx, a, b, 1e-8)
		if e != nil || out.Construction != CylinderCommonPerpendicularCapWindows {
			t.Fatal(tc, out.Construction, e)
		}
		faces := out.Solid.Faces()
		for owner, ids := range out.SideFaces {
			for _, id := range ids {
				f := faces[id]
				p, e := f.Surface.Eval((f.U.Min+f.U.Max)/2, .5)
				radial := [2]float64{math.Hypot(p.Y, p.Z), math.Hypot(p.X, p.Y)}[owner]
				if e != nil || math.Abs(radial-tc.radii[owner]) > 1e-10 {
					t.Fatal("cylinder wall ownership", tc, owner, p, e)
				}
			}
		}
		p, e := out.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil {
			t.Fatal(e)
		}
		const n = 100000
		volume := 0.
		r := min(a.Radius, b.Radius)
		for i := range n {
			y := -r + 2*r*(float64(i)+.5)/n
			w := func(axis int) float64 {
				d := math.Sqrt(tc.radii[1-axis]*tc.radii[1-axis] - y*y)
				c := tc.caps[axis]
				return max(0, min(d, c[1])-max(-d, c[0]))
			}
			volume += w(0) * w(1) * 2 * r / n
		}
		if math.Abs(p.Volume-volume) > 1e-4 {
			t.Fatal(tc, p.Volume, volume)
		}
		var buf bytes.Buffer
		if e := WriteJSON(&buf, out.Solid); e != nil {
			t.Fatal(e)
		}
		s, e := ReadJSON(&buf, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		m, e := s.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil || m != p {
			t.Fatal(m, p, e)
		}
	}
}
