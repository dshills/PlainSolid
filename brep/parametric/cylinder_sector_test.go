package parametric

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/geom"
)

func TestCylinderSector(t *testing.T) {
	f, e := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{Y: 1}, geom.Vec3{X: 1})
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	for _, theta := range []float64{math.Pi / 2, 1.5 * math.Pi} {
		const r, h = 2., 3.
		s, e := CylinderSector(f, r, h, theta)
		if e != nil || len(s.Faces()) != 5 || len(s.Edges()) != 11 || len(s.Vertices()) != 6 {
			t.Fatal(s, e)
		}
		var native bytes.Buffer
		if e = WriteJSON(&native, s); e != nil {
			t.Fatal(e)
		}
		s, e = ReadJSON(&native, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		p, e := s.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-8})
		volume := r * r * theta * h / 2
		area := r*r*theta + r*h*theta + 2*r*h
		x := 4 * r * math.Sin(theta/2) / (3 * theta)
		centroid := f.ToWorld(geom.Vec3{X: x * math.Cos(theta/2), Y: x * math.Sin(theta/2), Z: h / 2})
		if e != nil || math.Abs(p.Volume-volume) > 1e-7 || math.Abs(p.Area-area) > 1e-7 || p.Centroid.Distance(centroid) > 1e-7 {
			t.Fatal(p, volume, area, centroid, e)
		}
		m, e := s.Tessellate(ctx, TessellationOptions{Deflection: .03})
		if e != nil || m.ValidateSolid() != nil || math.Abs(m.SignedVolume()-volume) > .5 {
			t.Fatal(m.SignedVolume(), volume, e)
		}
	}
	for _, a := range []float64{0, 2 * math.Pi, 1e-15} {
		if _, e := CylinderSector(f, 2, 3, a); e == nil {
			t.Fatal("accepted invalid sector angle", a)
		}
	}
}

func TestConicalSector(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{Y: 1}, geom.Vec3{X: 1})
	const r, s, h, a = 3., 1., 4., 1.5 * math.Pi
	x, e := ConicalSector(f, r, s, h, a)
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e = WriteJSON(&raw, x); e != nil {
		t.Fatal(e)
	}
	x, e = ReadJSON(&raw, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := x.MassProperties(ctx, IntegrationOptions{})
	v := a * h * (r*r + r*s + s*s) / 6
	area := a*(r*r+s*s+(r+s)*math.Hypot(r-s, h))/2 + h*(r+s)
	if e != nil || math.Abs(p.Volume-v) > 1e-7 || math.Abs(p.Area-area) > 1e-7 {
		t.Fatal(p, v, area, e)
	}
	m, e := x.Tessellate(ctx, TessellationOptions{Deflection: .04})
	if e != nil || m.ValidateSolid() != nil || math.Abs(m.SignedVolume()-v) > 1 {
		t.Fatal(m.SignedVolume(), e)
	}
}
