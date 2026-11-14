package parametric

import (
	"bytes"
	"context"
	"math"
	"plainsolid/geom"
	"testing"
)

func TestCardinalQuarterCylinder(t *testing.T) {
	for _, axes := range [][2]geom.Vec3{{{X: 1}, {Z: 1}}, {{Y: -1}, {X: 1}}, {{Z: 1}, {Y: -1}}} {
		f, _ := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, axes[0], axes[1])
		s, e := CardinalQuarterCylinder(f, 2, 3)
		if e != nil {
			t.Fatal(e)
		}
		if len(s.Faces()) != 5 || len(s.Edges()) != 11 || len(s.Vertices()) != 6 {
			t.Fatal("topology")
		}
		wall := s.Faces()[0].Surface
		for _, tt := range []float64{0, .25, .5, 1} {
			p, e := wall.Eval(tt, .5)
			if e != nil {
				t.Fatal(e)
			}
			want := f.ToWorld(geom.Vec3{X: 2 * (1 - tt*tt) / (1 + tt*tt), Y: 4 * tt / (1 + tt*tt), Z: 1.5})
			if p.Distance(want) > 1e-14 {
				t.Fatal(p, want)
			}
		}
		// The end radial plane is exactly cardinal, including at its corners.
		p, e := s.Faces()[4].Surface.Eval(2, 3)
		if e != nil || p != f.ToWorld(geom.Vec3{Y: 2, Z: 3}) {
			t.Fatal(p, e)
		}
		var b bytes.Buffer
		if e = WriteJSON(&b, s); e != nil {
			t.Fatal(e)
		}
		raw := append([]byte(nil), b.Bytes()...)
		s, e = ReadJSON(&b, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		b.Reset()
		if e = WriteJSON(&b, s); e != nil || !bytes.Equal(raw, b.Bytes()) {
			t.Fatal("unstable reload", e)
		}
		m, e := s.MassProperties(context.Background(), IntegrationOptions{RelativeTolerance: 1e-9})
		wantV := 3 * math.Pi
		wantA := 5*math.Pi + 12
		wantC := f.ToWorld(geom.Vec3{X: 8 / (3 * math.Pi), Y: 8 / (3 * math.Pi), Z: 1.5})
		if e != nil || math.Abs(m.Volume-wantV) > 1e-7 || math.Abs(m.Area-wantA) > 1e-7 || m.Centroid.Distance(wantC) > 1e-7 {
			t.Fatal(m, e)
		}
		mesh, e := s.Tessellate(context.Background(), TessellationOptions{Deflection: .04})
		if e != nil {
			t.Fatal(e)
		}
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
	}
}
func TestCardinalQuarterCylinderReject(t *testing.T) {
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1, Y: 1}, geom.Vec3{Z: 1})
	if _, e := CardinalQuarterCylinder(f, 2, 3); e == nil {
		t.Fatal("noncardinal")
	}
	f, _ = geom.NewFrame(geom.Vec3{X: math.Exp2(53)}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if _, e := CardinalQuarterCylinder(f, 1, 3); e == nil {
		t.Fatal("rounded pole")
	}
	for _, v := range []float64{0, -1, math.Inf(1), math.NaN()} {
		if _, e := CardinalQuarterCylinder(f, v, 3); e == nil {
			t.Fatal("radius")
		}
	}
}
