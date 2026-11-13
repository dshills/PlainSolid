package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestExtrudeMixedRectangleThroughPocket(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	p := RoundedRectangle{Max: geom.Vec2{X: 20, Y: 10}, Radii: [4]float64{1, 1}}
	rect := []ExtrusionRectangle{{Min: geom.Vec2{X: 10, Y: 5}, Max: geom.Vec2{X: 13, Y: 8}}}
	bore := []ExtrusionCircle{{Center: geom.Vec2{X: 5, Y: 5}, Radius: 1}}
	for _, h := range []float64{10, -10} {
		display, e := ExtrudeMixedRectangleCertified(ctx, f, p, rect, bore, nil, h, CertifiedRevolvePolygonOptions{Deflection: .1})
		if e != nil {
			t.Fatal(e)
		}
		s := display.Solid
		q := s.Topology()
		if q.InnerWires != 4 || q.EulerCharacteristic != -2 || len(q.Shells) != 1 || len(s.faces) != 16 {
			t.Fatal(q)
		}
		if display.Mesh.ValidateSolid() != nil || !display.Collision.Complete || len(display.Collision.Contacts) != 0 {
			t.Fatal(display.Collision)
		}
		if len(display.RimVertices) != 3 || len(display.RimVertices[1][0]) != 4 || len(display.RimVertices[1][1]) != 4 {
			t.Fatal("rectangular wire lost its exact corner samples")
		}
		m, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		want := 2000 - 20*(1-math.Pi/4) - 90 - 10*math.Pi
		if e != nil || math.Abs(m.Volume-want) > 1e-7 || math.Abs(m.Centroid.Z-h/2) > 1e-8 {
			t.Fatal(m, want, e)
		}
		var file bytes.Buffer
		if e := WriteJSON(&file, s); e != nil {
			t.Fatal(e)
		}
		reloaded, e := ReadJSONContext(ctx, &file, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		for _, test := range []struct {
			x, y float64
			want parametric.Location
		}{{5, 5, parametric.Outside}, {11, 6, parametric.Outside}, {10, 6, parametric.Boundary}, {15, 6, parametric.Inside}} {
			got, e := reloaded.Classify(geom.Vec3{X: test.x, Y: test.y, Z: h / 2}, 1e-9)
			if e != nil || got != test.want {
				t.Fatal(test, got, e)
			}
		}
	}
}

func TestExtrudeMixedRectangleRejectsInvalidPocket(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	p := RoundedRectangle{Max: geom.Vec2{X: 10, Y: 10}, Radii: [4]float64{1, 1}}
	a := ExtrusionRectangle{Min: geom.Vec2{X: 2, Y: 2}, Max: geom.Vec2{X: 4, Y: 4}}
	b := ExtrusionRectangle{Min: geom.Vec2{X: 4, Y: 3}, Max: geom.Vec2{X: 6, Y: 6}}
	c := ExtrusionRectangle{Min: geom.Vec2{X: .5, Y: .5}, Max: geom.Vec2{X: 2, Y: 2}}
	for _, rectangles := range [][]ExtrusionRectangle{{a, b}, {c}} {
		if s, e := ExtrudeMixedRectangle(ctx, f, p, rectangles, nil, nil, 2); !errors.Is(e, geom.ErrDomain) || len(s.faces) != 0 {
			t.Fatal("touching or outside rectangle accepted", e)
		}
	}
	circle := ExtrusionCircle{Center: geom.Vec2{X: 5, Y: 3}, Radius: 1}
	if s, e := ExtrudeMixedRectangle(ctx, f, p, []ExtrusionRectangle{a}, []ExtrusionCircle{circle}, nil, 2); !errors.Is(e, geom.ErrDomain) || len(s.faces) != 0 {
		t.Fatal("exact circle-rectangle tangency accepted", e)
	}
}

func TestExtrudeMixedRectangleWithBoss(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	p := RoundedRectangle{Max: geom.Vec2{X: 20, Y: 10}, Radii: [4]float64{1, 1}}
	rect := []ExtrusionRectangle{{Min: geom.Vec2{X: 10, Y: 5}, Max: geom.Vec2{X: 13, Y: 8}}}
	bore := []ExtrusionCircle{{Center: geom.Vec2{X: 5, Y: 5}, Radius: 1}}
	for _, upper := range []bool{false, true} {
		b := ExtrusionBoss{Footprint: ExtrusionRectangle{Min: geom.Vec2{X: 15, Y: 5}, Max: geom.Vec2{X: 17, Y: 7}}, Height: 3, Upper: upper}
		d, err := ExtrudeMixedRectangleWithBossCertified(ctx, f, p, rect, bore, nil, 10, b, CertifiedRevolvePolygonOptions{Deflection: .1})
		if err != nil {
			t.Fatal(upper, err)
		}
		s := d.Solid
		if d.Mesh.ValidateSolid() != nil || !d.Collision.Complete || len(d.Collision.Contacts) != 0 || len(d.OriginalFace) != len(d.Mesh.Triangles) {
			t.Fatal("boss display failed embedding proof")
		}
		if s.Topology().EulerCharacteristic != -2 || len(s.faces) != 21 {
			t.Fatal(s.Topology())
		}
		m, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		want := 2000 - 20*(1-math.Pi/4) - 90 - 10*math.Pi + 12
		if e != nil || math.Abs(m.Volume-want) > 1e-7 {
			t.Fatal(m, want, e)
		}
		z := -1.
		if upper {
			z = 11
		}
		for _, c := range []struct {
			x, y float64
			want parametric.Location
		}{{16, 6, parametric.Inside}, {14, 6, parametric.Outside}, {15, 6, parametric.Boundary}} {
			got, err := s.Classify(geom.Vec3{X: c.x, Y: c.y, Z: z}, 1e-9)
			if err != nil || got != c.want {
				t.Fatal(c, got, err)
			}
		}
		b.Footprint = rect[0]
		if s, err := ExtrudeMixedRectangleWithBoss(ctx, f, p, rect, bore, nil, 10, b); !errors.Is(err, geom.ErrDomain) || len(s.faces) != 0 {
			t.Fatal("boss over a through pocket", err)
		}
	}
}
