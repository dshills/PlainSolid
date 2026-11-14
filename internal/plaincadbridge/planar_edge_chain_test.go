package plaincadbridge

import (
	"context"
	"math"
	"reflect"
	"testing"

	"plainsolid/brep"
	"plainsolid/csg/poly"
	"plainsolid/geom"
)

func TestSupportingJoinedEdgeChamfer(t *testing.T) {
	a, _ := brep.Box(geom.Vec3{X: -10, Y: -6}, geom.Vec3{X: 10, Y: 6, Z: 10})
	b, _ := brep.Box(geom.Vec3{X: 5, Y: -2}, geom.Vec3{X: 15, Y: 2, Z: 10})
	s, e := planarUnion(context.Background(), a, b)
	if e != nil {
		t.Fatal(e)
	}
	p, q := geom.Vec3{X: -10, Y: -6, Z: 10}, geom.Vec3{X: -10, Y: 6, Z: 10}
	if _, ok := supportingEdgeChain(s, p, q); !ok {
		t.Fatal("chain rejected")
	}
	var baseline brep.Solid
	for _, pair := range [][2]geom.Vec3{{p, q}, {q, p}} {
		out, e := chamferSupportingChain(s, pair[0], pair[1], geom.Vec3{Z: 1}, 1)
		if e != nil {
			t.Fatal(e)
		}
		m, e := out.MassProperties()
		if e != nil || math.Abs(m.Volume-2594) > 1e-8 {
			t.Fatal(m, e)
		}
		if len(baseline.Faces()) == 0 {
			baseline = out
		} else if !reflect.DeepEqual(out.Vertices(), baseline.Vertices()) || !reflect.DeepEqual(out.Edges(), baseline.Edges()) || !reflect.DeepEqual(out.Faces(), baseline.Faces()) {
			t.Fatal("reversed authored span changed topology order")
		}
	}
	if _, e := chamferSupportingChain(s, p, q, geom.Vec3{Z: 1}, 11); e == nil {
		t.Fatal("oversized accepted")
	}
	if _, ok := supportingEdgeChain(s, p, geom.Vec3{X: -10, Y: 2, Z: 10}); ok {
		t.Fatal("partial span")
	}
	if _, ok := supportingEdgeChain(s, p, geom.Vec3{X: -10, Y: 7, Z: 10}); ok {
		t.Fatal("gap")
	}

	notch, _ := brep.Box(geom.Vec3{X: -9.75, Y: -6, Z: 9.25}, geom.Vec3{X: -9.5, Y: 6, Z: 9.5})
	hollow, handled, e := poly.CombineAxisAligned(context.Background(), s, notch, poly.Difference, poly.Options{RequireExactConversion: true})
	if e != nil || !handled {
		t.Fatal(e)
	}
	if _, e := chamferSupportingChain(hollow, p, q, geom.Vec3{Z: 1}, 1); e == nil {
		t.Fatal("partially occupied collar accepted")
	}
	if _, e := chamferSupportingChain(s, p, q, geom.Vec3{Y: 1}, 1); e == nil {
		t.Fatal("wrong cap normal")
	}
}
