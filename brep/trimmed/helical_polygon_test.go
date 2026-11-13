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

func TestSweepHelicalPolygon(t *testing.T) {
	ctx := context.Background()
	f, e := geom.NewFrame(geom.Vec3{X: 3, Y: -2, Z: 4}, geom.Vec3{Y: 1}, geom.Vec3{X: 1})
	if e != nil {
		t.Fatal(e)
	}
	p := []geom.Vec2{{X: 2, Y: -.25}, {X: 3}, {X: 2, Y: .25}}
	const start, end = -.25, 1.75
	for _, hand := range []geom.HelixHandedness{geom.RightHanded, geom.LeftHanded} {
		s, e := SweepHelicalPolygon(ctx, f, p, 1, start, end, hand)
		if e != nil || s.Topology().EulerCharacteristic != 2 || len(s.faces) != 5 {
			t.Fatal(hand, s.Topology(), e)
		}
		// Jacobian magnitude is 2πr: the lead contributes an axial
		// shear and cancels from the determinant. Meridian area=1/4,
		// centroid radius=7/3, and the interval covers two full turns.
		want := 7 * math.Pi / 3
		a, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-6})
		if e != nil || math.Abs(a.Volume-want) > 1e-7*want || math.Abs(a.Centroid.X-3.75) > 1e-7 {
			t.Fatal(hand, a.Volume, a.Centroid, want, e)
		}
		var b bytes.Buffer
		if e := WriteJSON(&b, s); e != nil {
			t.Fatal(e)
		}
		back, e := ReadJSONContext(ctx, &b, JSONLimits{})
		if e != nil || back.Topology().EulerCharacteristic != 2 {
			t.Fatal(back.Topology(), e)
		}
		box, e := back.Bounds()
		if e != nil || box.Min.X > 2.5 || box.Max.X < 5 || box.Min.X < 2.49 || box.Max.X > 5.01 {
			t.Fatal("helical axial bounds", box, e)
		}
		m, e := back.Tessellate(ctx, parametric.TessellationOptions{Deflection: .08, MaxTriangles: 150000, AnalyticBound: true})
		if e != nil || m.ValidateSolid() != nil || math.Abs(m.SignedVolume()-want) > .025*want {
			t.Fatal("helical display", len(m.Triangles), m.SignedVolume(), e)
		}
	}
	if s, e := SweepHelicalPolygon(ctx, f, p, .5, 0, 2, geom.RightHanded); !errors.Is(e, geom.ErrDomain) || len(s.faces) != 0 {
		t.Fatal("neighboring turns may overlap", e)
	}
}

func TestHelicalCylinderCommon(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	p := []geom.Vec2{{X: 2, Y: -.25}, {X: 3}, {X: 2, Y: .25}}
	for _, hand := range []geom.HelixHandedness{geom.RightHanded, geom.LeftHanded} {
		s, e := HelicalCylinderCommon(ctx, f, p, 1, 0, 2, hand, 0, 2.5, -.5, 3)
		if e != nil || s.Topology().EulerCharacteristic != 2 || len(s.faces) != 6 {
			t.Fatal(hand, s.Topology(), e)
		}
		m, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-6})
		// 4π * ∫[2,2.5] r*(3-r)/2 dr = 5π/3.
		if want := 5 * math.Pi / 3; e != nil || math.Abs(m.Volume-want) > 1e-7 {
			t.Fatal(hand, m.Volume, want, e)
		}
	}
	if s, e := HelicalCylinderCommon(ctx, f, p, 1, 0, 2, geom.RightHanded, 3, 4, -.5, 3); e != nil || len(s.faces) != 0 {
		t.Fatal("zero-width radial contact is empty after regularization", e)
	}
	if s, e := HelicalCylinderCommon(ctx, f, p, 1, 0, 2, geom.RightHanded, 0, 2.5, 0, 1); !errors.Is(e, geom.ErrConvergence) || len(s.faces) != 0 {
		t.Fatal("unsupported cap crossing must be unresolved", e)
	}
}

func TestGrooveCylinderWithHelicalPolygon(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.NewFrame(geom.Vec3{X: 3, Y: 4, Z: 1}, geom.Vec3{X: 1}, geom.Vec3{Y: 1})
	p := []geom.Vec2{{X: 1.5, Y: .5}, {X: 2, Y: .375}, {X: 2, Y: .625}}
	for _, hand := range []geom.HelixHandedness{geom.RightHanded, geom.LeftHanded} {
		s, e := GrooveCylinderWithHelicalPolygon(ctx, f, 2, 3, p, 1, .125, .625, hand)
		if e != nil || s.Topology().EulerCharacteristic != 2 || s.Topology().InnerWires != 1 || len(s.faces) != 7 {
			t.Fatal(hand, s.Topology(), e)
		}
		v, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-6})
		// Triangle area=1/16, radial centroid=11/6, angle=π.
		want := (12 - 11.0/96) * math.Pi
		if e != nil || math.Abs(v.Volume-want) > want*1e-7 {
			t.Fatal(hand, v.Volume, want, e)
		}
		var file bytes.Buffer
		if e := WriteJSON(&file, s); e != nil {
			t.Fatal(e)
		}
		back, e := ReadJSONContext(ctx, &file, JSONLimits{})
		if e != nil || back.Topology().EulerCharacteristic != 2 {
			t.Fatal(hand, back.Topology(), e)
		}
	}
	if s, e := GrooveCylinderWithHelicalPolygon(ctx, f, 2, 3, p, 1, .75, 1.125, geom.RightHanded); !errors.Is(e, geom.ErrConvergence) || len(s.faces) != 0 {
		t.Fatal("unresolved periodic crossing accepted", e)
	}
}

func TestGrooveCylinderWithHelicalSeam(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.NewFrame(geom.Vec3{X: 3, Y: 4, Z: 1}, geom.Vec3{X: 1}, geom.Vec3{Y: 1})
	p := []geom.Vec2{{X: 1.5, Y: .5}, {X: 2, Y: .375}, {X: 2, Y: .625}}
	for _, hand := range []geom.HelixHandedness{geom.RightHanded, geom.LeftHanded} {
		s, e := GrooveCylinderWithHelicalSeam(ctx, f, 2, 3, p, 1, .125, 2.125, hand)
		if e != nil || s.Topology().EulerCharacteristic != 2 || s.Topology().InnerWires != 1 || len(s.faces) != 7 {
			t.Fatal(hand, s.Topology(), e)
		}
		v, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-6})
		want := (12 - 11.0/24) * math.Pi
		if e != nil || math.Abs(v.Volume-want) > want*1e-7 {
			t.Fatal(hand, v.Volume, want, e)
		}
		var file bytes.Buffer
		if e := WriteJSON(&file, s); e != nil {
			t.Fatal(e)
		}
		back, e := ReadJSONContext(ctx, &file, JSONLimits{})
		if e != nil || back.Topology().EulerCharacteristic != 2 {
			t.Fatal(hand, back.Topology(), e)
		}
		m, e := back.Tessellate(ctx, parametric.TessellationOptions{Deflection: .1, MaxTriangles: 500000, AnalyticBound: true})
		if e != nil || m.ValidateSolid() != nil || math.Abs(m.SignedVolume()-want) > .02*want {
			t.Fatal("analytic helical seam display", hand, len(m.Triangles), m.SignedVolume(), e)
		}
	}
}

func TestGrooveCylinderWithHelicalStarts(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	p := []geom.Vec2{{X: 1.5, Y: .5}, {X: 2, Y: .375}, {X: 2, Y: .625}}
	for _, hand := range []geom.HelixHandedness{geom.RightHanded, geom.LeftHanded} {
		s, e := GrooveCylinderWithHelicalStarts(ctx, f, 2, 4, p, 1, .125, 2.125, hand, 2)
		if e != nil || s.Topology().EulerCharacteristic != 2 || s.Topology().InnerWires != 2 || len(s.faces) != 11 {
			t.Fatal(hand, s.Topology(), e)
		}
		v, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-6})
		want := (16 - 11.0/12) * math.Pi
		if e != nil || math.Abs(v.Volume-want) > want*1e-7 {
			t.Fatal(hand, v.Volume, want, e)
		}
	}
	if s, e := GrooveCylinderWithHelicalStarts(ctx, f, 2, 4, p, 1, .125, 2.125, geom.RightHanded, 4); !errors.Is(e, geom.ErrDomain) || len(s.faces) != 0 {
		t.Fatal("touching starts must reject", e)
	}
}
