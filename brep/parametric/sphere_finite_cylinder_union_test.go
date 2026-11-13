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

func TestSphereFiniteCylinderUnion(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{Z: -1}, geom.Vec3{Y: 1})
	for _, c := range []struct {
		name      string
		r, lo, hi float64
		want      []*big.Rat
		shoulders int
	}{
		{"cylinder enclosed", 2, -2, 2, []*big.Rat{big.NewRat(500, 3)}, 0},
		{"sphere enclosed", 5, -6, 6, []*big.Rat{big.NewRat(300, 1)}, 0},
		{"two transverse rims", 3, -6, 6, []*big.Rat{big.NewRat(580, 3)}, 0},
		{"one transverse rim and exposed ends", 3, 4, 6, []*big.Rat{big.NewRat(180, 1)}, 0},
		{"two annular shoulders", 5, -4, 4, []*big.Rat{big.NewRat(628, 3)}, 2},
		{"disjoint above", 3, 6, 7, []*big.Rat{big.NewRat(500, 3), big.NewRat(9, 1)}, 0},
		{"disjoint below", 3, -7, -6, []*big.Rat{big.NewRat(9, 1), big.NewRat(500, 3)}, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			out, e := SphereFiniteCylinderUnion(ctx, f, 5, c.r, c.lo, c.hi, 1e-9)
			if e != nil || len(out.Solids) != len(c.want) || len(out.VolumePiCoefficients) != len(c.want) {
				t.Fatal(e, len(out.Solids), out.VolumePiCoefficients)
			}
			annularFaces := 0
			for i, s := range out.Solids {
				m, e := s.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
				want, _ := c.want[i].Float64()
				if e != nil || out.VolumePiCoefficients[i].Cmp(c.want[i]) != 0 || math.Abs(m.Volume-want*math.Pi) > 1e-7 {
					t.Fatal(i, m.Volume, out.VolumePiCoefficients[i], c.want[i], e)
				}
				for _, face := range s.Faces() {
					if _, ok := PolarPlaneFrame(face.Surface); ok && face.V.Min > 0 {
						annularFaces++
					}
				}
			}
			if annularFaces != 4*c.shoulders {
				t.Fatal("unexpected number of annular shoulder patches", annularFaces)
			}
			if c.shoulders != 0 {
				m, e := out.Solids[0].Tessellate(ctx, TessellationOptions{Deflection: .3})
				if e != nil || m.ValidateSolid() != nil || m.SignedVolume() <= 0 {
					t.Fatal("invalid display", e)
				}
				contacts, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
				if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
					t.Fatal("union boundary has contacts", contacts, e)
				}
			}
		})
	}
	for _, c := range []struct {
		r, lo, hi float64
		want      error
	}{
		{3, 5, 6, geom.ErrConvergence},
		{3, 5 + 1e-10, 6, geom.ErrConvergence},
		{2, -6, 6, geom.ErrUnrepresentable},
		{5, -2, 4, geom.ErrUnrepresentable},
	} {
		out, e := SphereFiniteCylinderUnion(ctx, f, 5, c.r, c.lo, c.hi, 1e-9)
		if !errors.Is(e, c.want) || len(out.Solids) != 0 {
			t.Fatal(c, e)
		}
	}
}
