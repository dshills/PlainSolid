package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestTopChamferedBoxRetainedBore(t *testing.T) {
	ctx := context.Background()
	for _, axis := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		f, e := geom.FrameFromNormal(geom.Vec3{X: 3, Y: 5, Z: 7}, axis)
		if e != nil {
			t.Fatal(e)
		}
		for _, radius := range []float64{0, 2} {
			s, e := TopChamferedBox(ctx, f, 20, 20, 10, 1, geom.Vec2{X: 10, Y: 10}, radius, 1e-8)
			if e != nil {
				t.Fatal(axis, radius, e)
			}
			p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			want := 4000 - (40 - 4./3) - math.Pi*radius*radius*10
			area := 400 + 324 + 720 + 76*math.Sqrt2 - 2*math.Pi*radius*radius + 20*math.Pi*radius
			if math.Abs(p.Volume-want) > 1e-7 || math.Abs(p.Area-area) > 1e-7 {
				t.Fatalf("mass: %+v wantV %g wantA %g", p, want, area)
			}
			m, e := s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .05, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 1000000})
			if e != nil {
				t.Fatal("mesh", axis, radius, e)
			}
			if e = m.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
		}
	}
}

func TestTopChamferedBoxMomentsAndDomain(t *testing.T) {
	ctx := context.Background()
	frame, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	const w, d, h, c, r = 24., 18., 12., 2., 1.
	center := geom.Vec2{X: 8, Y: 6}
	s, e := TopChamferedBox(ctx, frame, w, d, h, c, center, r, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	removed := (w+d)*c*c - 4*c*c*c/3
	removedZ := (h-c)*removed + 2*(w+d)*c*c*c/3 - c*c*c*c
	bore := math.Pi * r * r * h
	v := w*d*h - removed - bore
	want := geom.Vec3{X: ((w*d*h-removed)*w/2 - bore*center.X) / v, Y: ((w*d*h-removed)*d/2 - bore*center.Y) / v, Z: (w*d*h*h/2 - removedZ - bore*h/2) / v}
	if p.Centroid.Sub(want).Length() > 1e-8 {
		t.Fatalf("centroid %+v want %+v", p.Centroid, want)
	}
	var buf bytes.Buffer
	if e = WriteJSON(&buf, s); e != nil {
		t.Fatal(e)
	}
	loaded, e := ReadJSON(&buf, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	q, e := loaded.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(q.Volume-v) > 1e-7 {
		t.Fatal(q, e)
	}
	for _, test := range []struct {
		setback, radius float64
		center          geom.Vec2
	}{{0, r, center}, {h, r, center}, {c, -1, center}, {c, r, geom.Vec2{X: c + r, Y: 6}}, {c, r, geom.Vec2{X: w - c - r, Y: 6}}, {c, r, geom.Vec2{X: 8, Y: c + r}}, {c, r, geom.Vec2{X: 8, Y: d - c - r}}} {
		if _, e := TopChamferedBox(ctx, frame, w, d, h, test.setback, test.center, test.radius, 1e-8); e == nil {
			t.Fatal("accepted invalid or tangent recipe", test)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = TopChamferedBox(cancelled, frame, w, d, h, c, center, r, 1e-8); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e = s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .01, MaxTriangles: 1}); !errors.Is(e, parametric.ErrBudget) {
		t.Fatal("mesh budget", e)
	}
}
