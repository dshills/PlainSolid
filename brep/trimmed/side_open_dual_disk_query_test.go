package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestLocateSideOpenDualDisks(t *testing.T) {
	ctx := context.Background()
	frame, e := geom.NewFrame(geom.Vec3{X: -10, Y: 20, Z: 30}, geom.Vec3{Y: -1}, geom.Vec3{X: 1})
	if e != nil {
		t.Fatal(e)
	}
	// Deliberately put the right circle first, away from the box midpoint.
	discs := [2]ExtrusionCircle{{Center: geom.Vec2{X: 6.5, Y: 4}, Radius: 2.5}, {Center: geom.Vec2{X: 3.5, Y: 4}, Radius: 2.5}}
	const height = 6
	for _, depth := range []float64{3, height} {
		cut, e := CutSideOpenDualDisks(ctx, frame, geom.Vec2{X: -2, Y: 4}, geom.Vec2{X: 10, Y: 10}, discs, height, depth, 1e-10, trimvalidate.Options{})
		if e != nil {
			t.Fatal(e)
		}
		floor, farCap := parametric.Boundary, parametric.Boundary
		if depth == height {
			floor, farCap = parametric.Outside, parametric.Outside
		}
		for _, test := range []struct {
			name  string
			point geom.Vec3
			want  parametric.Location
		}{
			{"outside stock", geom.Vec3{X: 5, Y: 11, Z: 1}, parametric.Outside},
			{"material", geom.Vec3{X: 5, Y: 7, Z: 1}, parametric.Inside},
			{"entry opening", geom.Vec3{X: 5, Y: 5}, parametric.Outside},
			{"side opening", geom.Vec3{X: 5, Y: 4, Z: 1}, parametric.Outside},
			{"unopened side", geom.Vec3{X: 0, Y: 4, Z: 1}, parametric.Boundary},
			{"exposed arc", geom.Vec3{X: 3.5, Y: 6.5, Z: 1}, parametric.Boundary},
			{"hidden arc of individual cutter", geom.Vec3{X: 6, Y: 4, Z: 1}, parametric.Outside},
			{"common arc seam", geom.Vec3{X: 5, Y: 6, Z: 1}, parametric.Boundary},
			{"entry rim", geom.Vec3{X: 5, Y: 6}, parametric.Boundary},
			{"floor or through opening", geom.Vec3{X: 5, Y: 5, Z: depth}, floor},
			{"stock far cap or through opening", geom.Vec3{X: 5, Y: 5, Z: 6}, farCap},
		} {
			got, e := cut.Locate(ctx, frame.ToWorld(test.point))
			if e != nil || got != test.want {
				t.Fatalf("depth %g %s: %v, %v; want %v", depth, test.name, got, e, test.want)
			}
		}
		// World Z decreases as local Y increases; the exact offsets round
		// to the same float64 and must still land on opposite material sides.
		tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
		seam := ratInset(frame.ToWorld(geom.Vec3{X: 5, Y: 6, Z: 1}).Z)
		a := [3]*big.Rat{big.NewRat(-9, 1), big.NewRat(15, 1), new(big.Rat).Sub(seam, tiny)}
		b := [3]*big.Rat{a[0], a[1], new(big.Rat).Add(seam, tiny)}
		inside, aerr := cut.LocateRational(ctx, a)
		outside, berr := cut.LocateRational(ctx, b)
		if aerr != nil || berr != nil || inside != parametric.Inside || outside != parametric.Outside {
			t.Fatal("sub-float exact seam probes", inside, outside, aerr, berr)
		}
		integrated, e := cut.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil {
			t.Fatal(e)
		}
		formula, e := cut.Measures()
		if e != nil || math.Abs(formula.Volume-integrated.Volume) > 2e-7 || math.Abs(formula.Area-integrated.Area) > 2e-7 || formula.Centroid.Sub(integrated.Centroid).Length() > 2e-8 {
			t.Fatal("closed form vs chart integration in rotated frame", formula, integrated, e)
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if _, e := cut.Locate(cancelled, geom.Vec3{}); !errors.Is(e, context.Canceled) {
			t.Fatal("cancelled material query", e)
		}
	}
}
