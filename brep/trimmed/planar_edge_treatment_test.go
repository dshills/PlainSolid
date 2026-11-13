package trimmed

import (
	"context"
	"errors"
	"math"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/csg/poly"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestJoinedPlanarStockRetainedEdgeTreatment(t *testing.T) {
	ctx := context.Background()
	a, _ := brep.Box(geom.Vec3{X: -10, Y: -6}, geom.Vec3{X: 10, Y: 6, Z: 12})
	b, _ := brep.Box(geom.Vec3{X: 5, Y: -2}, geom.Vec3{X: 15, Y: 2, Z: 12})
	base, e := poly.Combine(ctx, a, b, poly.Union, poly.Options{})
	if e != nil {
		t.Fatal(e)
	}
	for _, bevel := range []bool{false, true} {
		start, end := geom.Vec3{X: -10, Y: 6, Z: 12}, geom.Vec3{X: 10, Y: 6, Z: 12}
		length := 20.
		if bevel {
			start, end = geom.Vec3{X: -10, Y: -6}, geom.Vec3{X: -10, Y: 6}
			length = 12
		}
		s, e := TreatPlanarOrthogonalEdge(ctx, base, start, end, 1, bevel, 1e-9)
		if e != nil {
			t.Fatal(bevel, e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if e != nil {
			t.Fatal(e)
		}
		removed := length * (1 - math.Pi/4)
		if bevel {
			removed = length / 2
		}
		if math.Abs(p.Volume-(3120-removed)) > 1e-7 {
			t.Fatal(p.Volume, removed)
		}
		m, e := s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxTriangles: 100000, MaxSubdivisions: 128})
		if e != nil {
			t.Fatal(e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
	}
}

func TestLocalPlanarEdgeTreatmentRejectsAndOwns(t *testing.T) {
	ctx := context.Background()
	a, _ := brep.Box(geom.Vec3{X: -10, Y: -6}, geom.Vec3{X: 10, Y: 6, Z: 12})
	b, _ := brep.Box(geom.Vec3{X: 5, Y: -2}, geom.Vec3{X: 15, Y: 2, Z: 12})
	base, e := poly.Combine(ctx, a, b, poly.Union, poly.Options{})
	if e != nil {
		t.Fatal(e)
	}
	original := base.Faces()
	start, end := geom.Vec3{X: -10, Y: 6, Z: 12}, geom.Vec3{X: 10, Y: 6, Z: 12}
	if _, e = TreatPlanarOrthogonalEdge(ctx, base, start, end, 1, false, 1e-9); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(original, base.Faces()) {
		t.Fatal("mutated source")
	}
	for _, size := range []float64{4, 5, math.NaN()} {
		if _, e = TreatPlanarOrthogonalEdge(ctx, base, start, end, size, false, 1e-9); e == nil {
			t.Fatal("unsafe size", size)
		}
	}
	if _, e = TreatPlanarOrthogonalEdge(ctx, base, geom.Vec3{X: 10, Y: 2}, geom.Vec3{X: 10, Y: 2, Z: 12}, 1, false, 1e-9); e == nil {
		t.Fatal("concave edge accepted")
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = TreatPlanarOrthogonalEdge(cancelCtx, base, start, end, 1, false, 1e-9); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
