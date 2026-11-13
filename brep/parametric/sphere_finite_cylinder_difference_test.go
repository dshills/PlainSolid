package parametric

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestSphereFiniteCylinderDifferenceComponents(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.NewFrame(geom.Vec3{X: -1, Y: 3, Z: 2}, geom.Vec3{Y: 1}, geom.Vec3{X: 1})
	for _, c := range []struct {
		name      string
		r, lo, hi float64
		want      []*big.Rat
	}{
		{"enclosed cylindrical cavity", 3, -2, 2, []*big.Rat{big.NewRat(392, 3)}},
		{"blind bore", 3, 2, 6, []*big.Rat{big.NewRat(144, 1)}},
		{"through bore", 3, -6, 6, []*big.Rat{big.NewRat(256, 3)}},
		{"two disconnected caps", 5, -4, 4, []*big.Rat{big.NewRat(14, 3), big.NewRat(14, 3)}},
		{"whole sphere removed", 5, -6, 6, nil},
		{"touch at pole", 3, 5, 6, []*big.Rat{big.NewRat(500, 3)}},
		{"cylinder reaches only one transverse rim", 3, -6, 2, []*big.Rat{big.NewRat(324, 3)}},
	} {
		t.Run(c.name, func(t *testing.T) {
			r, e := SphereFiniteCylinderDifference(ctx, f, 5, c.r, c.lo, c.hi, 1e-9)
			if e != nil || len(r.Solids) != len(c.want) || len(r.VolumePiCoefficients) != len(c.want) {
				t.Fatal(e, len(r.Solids), r.VolumePiCoefficients, c.want)
			}
			for i, expected := range c.want {
				p, e := r.Solids[i].MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
				want, _ := expected.Float64()
				if e != nil || r.VolumePiCoefficients[i].Cmp(expected) != 0 || math.Abs(p.Volume-want*math.Pi) > 1e-7 {
					t.Fatal(i, p.Volume, r.VolumePiCoefficients[i], want, e)
				}
			}
		})
	}
	if r, e := SphereFiniteCylinderDifference(ctx, f, 5, 2, -6, 6, 1e-9); !errors.Is(e, geom.ErrUnrepresentable) || len(r.Solids) != 0 {
		t.Fatal("unexpected partial model or error", e)
	}
	if r, e := SphereFiniteCylinderDifference(ctx, f, 5, 3, -4, 2, 1e-9); !errors.Is(e, geom.ErrConvergence) || len(r.Solids) != 0 {
		t.Fatal("unexpected pinched model or error", e)
	}
}
