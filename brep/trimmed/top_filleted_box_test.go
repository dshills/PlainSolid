package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/exchange"
	"plainsolid/geom"
	"testing"
)

func TestTopFilletedBoxPerimeter(t *testing.T) {
	ctx := context.Background()
	const w, d, h, r = 12., 8., 10., 1.
	removed := 2*(w+d)*r*r*(1-math.Pi/4) - 4*r*r*r*(5./3-math.Pi/2)
	v := w*d*h - removed
	area := w*d + (w-2*r)*(d-2*r) + 2*(w+d)*(h-r) + math.Pi*r*(w+d-4*r) + 8*r*r
	removedZ := (h-r)*removed + (w+d)*r*r*r/3 - r*r*r*r/3
	center := geom.Vec3{X: w / 2, Y: d / 2, Z: (w*d*h*h/2 - removedZ) / v}
	for _, axis := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		frame, e := geom.FrameFromNormal(geom.Vec3{X: 3, Y: 5, Z: 7}, axis)
		if e != nil {
			t.Fatal(e)
		}
		s, e := TopFilletedBox(ctx, frame, w, d, h, r, 1e-8)
		if e != nil {
			t.Fatal(e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(p.Volume-v) > 1e-7 || math.Abs(p.Area-area) > 1e-7 || p.Centroid.Sub(frame.ToWorld(center)).Length() > 1e-8 {
			t.Fatal(p, v, area, center)
		}
		m, e := s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 1000000})
		if e != nil {
			t.Fatal("mesh", e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		var stl bytes.Buffer
		if e = exchange.WriteSTL(&stl, m); e != nil {
			t.Fatal(e)
		}
		loadedMesh, e := exchange.ReadSTL(&stl, exchange.Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if e = loadedMesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		var native bytes.Buffer
		if e = WriteJSON(&native, s); e != nil {
			t.Fatal(e)
		}
		loaded, e := ReadJSON(&native, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		q, e := loaded.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(q.Volume-v) > 1e-7 {
			t.Fatal(q, e)
		}
	}
}
func TestTopFilletedBoxDomain(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	for _, r := range []float64{0, -1, 4, math.Inf(1), math.NaN()} {
		if _, e := TopFilletedBox(ctx, f, 12, 8, 10, r, 1e-8); e == nil {
			t.Fatal("accepted radius", r)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := TopFilletedBox(cancelled, f, 12, 8, 10, 1, 1e-8); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	s, e := TopFilletedBox(ctx, f, 12, 8, 10, 1, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .01, MaxTriangles: 1}); !errors.Is(e, parametric.ErrBudget) {
		t.Fatal(e)
	}
}
