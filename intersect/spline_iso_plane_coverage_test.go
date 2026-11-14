package intersect

import (
	"context"
	"errors"
	"plainsolid/geom"
	"testing"
)

func TestIsoPlaneCoverageGuards(t *testing.T) {
	k := []float64{0, 0, 1, 1}
	surface, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	p := Patch{Surface: surface, Max: geom.Vec2{X: 1, Y: 1}}
	ctx := context.Background()
	for _, o := range []IsoPlaneTubeCoverageOptions{{MaxOperations: -1}, {MaxCoefficientBits: 63}, {MaxTubes: 4097}, {MaxCells: -1}} {
		r, e := CertifyIsoPlaneSplineTubeCoverage(ctx, p, p, SplineSeedReport{}, o)
		if !errors.Is(e, geom.ErrInvalid) || r.Covered || len(r.Spans) != 0 {
			t.Fatal("invalid options", e)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := CertifyIsoPlaneSplineTubeCoverage(canceled, p, p, SplineSeedReport{}, IsoPlaneTubeCoverageOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	r, e := CertifyIsoPlaneSplineTubeCoverage(ctx, p, p, SplineSeedReport{}, IsoPlaneTubeCoverageOptions{})
	if e == nil || r.Covered {
		t.Fatal("coincident full supports", e)
	}
}
