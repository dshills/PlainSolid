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

func TestTopFilletedBottomChamferedBox(t *testing.T) {
	for _, c := range []float64{.5, 1} {
		testTopFilletedBottomChamferedBox(t, c)
	}
}
func testTopFilletedBottomChamferedBox(t *testing.T, c float64) {
	ctx := context.Background()
	const w, d, h, r = 20., 10., 10., 1.
	top := 2*(w+d)*r*r*(1-math.Pi/4) - 4*r*r*r*(5./3-math.Pi/2)
	bottom := (w+d)*c*c - 4*c*c*c/3
	volume := w*d*h - top - bottom
	area := (w-2*c)*(d-2*c) + (w-2*r)*(d-2*r) + 2*(w+d)*(h-r-c) + math.Pi*r*(w+d-4*r) + 8*r*r + 2*math.Sqrt2*c*(w+d-2*c)
	momentTop := (h-r)*top + (w+d)*r*r*r/3 - r*r*r*r/3
	momentBottom := (w+d)*c*c*c/3 - c*c*c*c/3
	center := geom.Vec3{X: w / 2, Y: d / 2, Z: (w*d*h*h/2 - momentTop - momentBottom) / volume}
	for _, axis := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		f, _ := geom.FrameFromNormal(geom.Vec3{X: 3, Y: 5, Z: 7}, axis)
		s, e := TopFilletedBottomChamferedBox(ctx, f, w, d, h, r, c, 1e-8)
		if e != nil {
			t.Fatal(e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(p.Volume-volume) > 1e-7 || math.Abs(p.Area-area) > 1e-7 || p.Centroid.Sub(f.ToWorld(center)).Length() > 1e-8 {
			t.Fatal(p, volume, area, center)
		}
		m, e := s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 1000000})
		if e != nil {
			t.Fatal(e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		var data bytes.Buffer
		if e = WriteJSON(&data, s); e != nil {
			t.Fatal(e)
		}
		loaded, e := ReadJSON(&data, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		p, e = loaded.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(p.Volume-volume) > 1e-7 {
			t.Fatal(p, e)
		}
		data.Reset()
		if e = exchange.WriteSTL(&data, m); e != nil {
			t.Fatal(e)
		}
		restored, e := exchange.ReadSTL(&data, exchange.Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if e = restored.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
	}
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	for _, c := range []float64{0, -1, 5, 9, math.NaN()} {
		if _, e := TopFilletedBottomChamferedBox(ctx, f, w, d, h, r, c, 1e-8); e == nil {
			t.Fatal("accepted setback", c)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := TopFilletedBottomChamferedBox(cancelled, f, w, d, h, r, .5, 1e-8); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
