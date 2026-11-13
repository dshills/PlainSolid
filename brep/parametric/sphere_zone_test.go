package parametric

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/geom"
)

func TestSphereZone(t *testing.T) {
	ctx := context.Background()
	f, e := geom.NewFrame(geom.Vec3{X: 7, Y: 1, Z: -3}, geom.Vec3{X: 1}, geom.Vec3{Y: 1})
	if e != nil {
		t.Fatal(e)
	}
	const r, lo, hi = 3., -math.Pi / 6, math.Pi / 4
	s, e := SphereZone(f, r, lo, hi, 1e-10)
	if e != nil || len(s.Faces()) != 3 || len(s.Edges()) != 7 {
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
	a, b := r*math.Sin(lo), r*math.Sin(hi)
	wantV := math.Pi * (r*r*(b-a) - (b*b*b-a*a*a)/3)
	wantA := math.Pi * (2*r*(b-a) + 2*r*r - a*a - b*b)
	wantZ := math.Pi * (r*r*(b*b-a*a)/2 - (math.Pow(b, 4)-math.Pow(a, 4))/4) / wantV
	p, e := s.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil || math.Abs(p.Volume-wantV) > 1e-7 || math.Abs(p.Area-wantA) > 1e-7 ||
		p.Centroid.Distance(f.ToWorld(geom.Vec3{Z: wantZ})) > 1e-8 {
		t.Fatal(p, wantV, wantA, wantZ, e)
	}
	m, e := s.Tessellate(ctx, TessellationOptions{Deflection: .015})
	if e != nil || m.ValidateSolid() != nil || math.Abs(m.SignedVolume()-wantV) > .7 {
		t.Fatal(m.SignedVolume(), wantV, e)
	}
	for _, x := range [][2]float64{{hi, lo}, {-math.Pi / 2, hi}, {0, 1e-14}} {
		if _, e = SphereZone(f, r, x[0], x[1], 1e-10); e == nil {
			t.Fatal("accepted invalid zone", x)
		}
	}
}
