package parametric

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/geom"
)

func TestLoftEllipticStations(t *testing.T) {
	ctx := context.Background()
	f, e := geom.FrameFromNormal(geom.Vec3{X: 3, Y: -2, Z: 4}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	s, e := LoftEllipticStations(f, []EllipticStation{
		{Height: -1, Radii: geom.Vec2{X: 2, Y: 3}},
		{Height: 1, Radii: geom.Vec2{X: 4, Y: 3}},
		{Height: 3, Radii: geom.Vec2{X: 2, Y: 3}},
	}, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.MassProperties(ctx, IntegrationOptions{})
	// On each half, the station radii have mean 3+1/6 by
	// integrating the endpoint tangent contributions to cubic Hermite.
	if e != nil || math.Abs(p.Volume-38*math.Pi) > 1e-8 ||
		p.Centroid.Distance(geom.Vec3{X: 3, Y: -2, Z: 5}) > 1e-9 {
		t.Fatal(p, e)
	}
	faces := s.Faces()
	a, b := faces[0].Surface, faces[4].Surface
	for _, u := range []float64{0, .4, math.Pi / 2} {
		p, _ := a.Eval(u, 1)
		q, _ := b.Eval(u, 0)
		_, ad, _ := a.Partials(u, 1)
		_, bd, _ := b.Partials(u, 0)
		if p.Distance(q) > 1e-12 || ad.Distance(bd) > 1e-12 {
			t.Fatal(p, q, ad, bd)
		}
	}
	var stream bytes.Buffer
	if e = WriteJSON(&stream, s); e != nil {
		t.Fatal(e)
	}
	s, e = ReadJSON(&stream, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	m, e := s.Tessellate(ctx, TessellationOptions{Deflection: .02, AnalyticBound: true})
	if e != nil || m.ValidateSolid() != nil {
		t.Fatal(e)
	}
	box, e := s.Bounds()
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range m.Vertices {
		if !box.Contains(p) {
			t.Fatal(box, p)
		}
	}
}
