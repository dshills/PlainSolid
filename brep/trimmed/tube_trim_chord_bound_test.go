package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func TestCertifiedTubeTrimChordIndependent(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2,
		[]geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	// The wall extends past both sides of A. On z=0 the transverse
	// branch is y=29/64+x*(1-x)/16 for 0<=x<=1.
	b, e := geom.NewBSplineSurface(2, 1, 3, 2,
		[]geom.Vec3{{X: -.5, Y: 13.0 / 32, Z: -1}, {X: -.5, Y: 13.0 / 32, Z: 1},
			{X: .5, Y: 17.0 / 32, Z: -1}, {X: .5, Y: 17.0 / 32, Z: 1},
			{X: 1.5, Y: 13.0 / 32, Z: -1}, {X: 1.5, Y: 13.0 / 32, Z: 1}},
		[]float64{-.5, -.5, -.5, 1.5, 1.5, 1.5}, []float64{-1, -1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	da := geom.ParameterRectangle{UMax: 1, VMax: 1}
	db := geom.ParameterRectangle{UMin: -.5, UMax: 1.5, VMin: -1, VMax: 1}
	segments, e := intersect.RebuildTubeCurves(ctx, intersect.TubeRecipeData{
		A:     intersect.Patch{Surface: a, Max: geom.Vec2{X: 1, Y: 1}},
		B:     intersect.Patch{Surface: b, Min: geom.Vec2{X: -.5, Y: -1}, Max: geom.Vec2{X: 1.5, Y: 1}},
		Cells: []intersect.TubeRecipeCell{{DomainA: da, DomainB: db}},
	})
	if e != nil {
		t.Fatal(e)
	}

	start, end := geom.Vec3{Y: 29.0 / 64}, geom.Vec3{X: 1, Y: 29.0 / 64}
	for _, trim := range []intersect.TubeTrim{segments[0].TrimA(), segments[0].TrimB()} {
		bound, e := BoundTubeTrimChord(ctx, trim, 0, 1, start, end, TubeTrimChordOptions{})
		if e != nil {
			t.Fatal(e)
		}
		// The independently defined parabola attains sagitta 1/64 at x=.5.
		if bound.SquaredDistance.Cmp(big.NewRat(1, 4096)) < 0 {
			t.Fatal("false parabola bound", bound)
		}
		upper, _ := bound.SquaredDistance.Float64()
		for i := 0; i <= 100; i++ {
			x := float64(i) / 100
			sag := x * (1 - x) / 16
			if sag*sag > upper {
				t.Fatal("false interval enclosure")
			}
		}
		// Deliberately displaced chord: the certificate must include the
		// represented endpoints instead of assuming they lie on the section.
		shifted, e := BoundTubeTrimChord(ctx, trim, 0, 1, start.Add(geom.Vec3{Z: 1}), end.Add(geom.Vec3{Z: 1}), TubeTrimChordOptions{})
		if e != nil || shifted.SquaredDistance.Cmp(big.NewRat(1, 1)) < 0 {
			t.Fatal("rounded/displaced chord", e)
		}
		bad, e := BoundTubeTrimChord(ctx, trim, 0, 1, start, end, TubeTrimChordOptions{Interval: geom.SurfaceTubeIntervalOptions{MaxOperations: 1}})
		if e == nil || bad.SquaredDistance != nil {
			t.Fatal("budget false success", e)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	bad, e := BoundTubeTrimChord(canceled, segments[0].TrimA(), 0, 1, start, end, TubeTrimChordOptions{})
	if !errors.Is(e, context.Canceled) || bad.SquaredDistance != nil {
		t.Fatal("cancellation", e)
	}
}
