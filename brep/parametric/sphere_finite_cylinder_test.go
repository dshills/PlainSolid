package parametric

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/collision"
	"plainsolid/geom"
)

func TestSphereFiniteCylinderCommonAxialCases(t *testing.T) {
	ctx := context.Background()
	frame, _ := geom.NewFrame(geom.Vec3{X: 3, Y: -2, Z: 1}, geom.Vec3{Y: 1}, geom.Vec3{X: -1})
	for _, c := range []struct {
		name       string
		r, lo, hi  float64
		v          *big.Rat
		s, c, l, h int
	}{
		{"both radial seams", 3, -6, 6, big.NewRat(244, 3), 8, 4, 0, 0},
		{"one radial seam", 3, -6, 2, big.NewRat(176, 3), 4, 4, 0, 4},
		{"exact rim cap", 3, -4, 4, big.NewRat(72, 1), 0, 4, 4, 4},
		{"spherical zone", 5, -4, 4, big.NewRat(472, 3), 4, 0, 4, 4},
		{"point touch", 3, 5, 6, new(big.Rat), 0, 0, 0, 0},
		{"irrational rim outside clip", 2, -1, 1, big.NewRat(8, 1), 0, 4, 4, 4},
	} {
		t.Run(c.name, func(t *testing.T) {
			r, e := SphereFiniteCylinderCommon(ctx, frame, 5, c.r, c.lo, c.hi, 1e-9)
			if e != nil {
				t.Fatal(e)
			}
			if r.VolumePiCoefficient.Cmp(c.v) != 0 || len(r.SphericalFaces) != c.s || len(r.CylindricalFaces) != c.c ||
				len(r.LowerCap) != c.l || len(r.UpperCap) != c.h {
				t.Fatal(r.VolumePiCoefficient, c.v, len(r.SphericalFaces), len(r.CylindricalFaces), len(r.LowerCap), len(r.UpperCap))
			}
			if c.v.Sign() == 0 {
				if len(r.Solid.Faces()) != 0 {
					t.Fatal("regularized touch returned boundary")
				}
				return
			}
			m, e := r.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
			want, _ := c.v.Float64()
			if e != nil || math.Abs(m.Volume-want*math.Pi) > 1e-7 {
				t.Fatal(m.Volume, want*math.Pi, e)
			}
		})
	}
}

func TestSphereFiniteCylinderCommonRejectsUnrepresentedRims(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	for _, c := range []struct{ r, lo, hi float64 }{{2, -6, 6}, {5, -2, 4}} {
		if out, e := SphereFiniteCylinderCommon(ctx, f, 5, c.r, c.lo, c.hi, 1e-9); !errors.Is(e, geom.ErrUnrepresentable) || len(out.Solid.Faces()) != 0 {
			t.Fatal("unrepresented seam or cap accepted", c, e)
		}
	}
	r, e := SphereFiniteCylinderCommon(ctx, f, 5, 3, -6, 2, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	m, e := r.Solid.Tessellate(ctx, TessellationOptions{Deflection: .3})
	if e != nil || m.ValidateSolid() != nil || m.SignedVolume() <= 0 {
		t.Fatal("invalid display", e)
	}
	contacts, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
	if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
		t.Fatal("closed common boundary has contacts", contacts, e)
	}
}
