package placement_test

import (
	"context"
	"plainsolid/brep"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestExactCoordinatePlacement(t *testing.T) {
	s, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 3, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	p, e := placement.New([3]int{-2, 1, 3}, geom.Vec3{X: 8, Y: -4, Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	m, e := p.ApplyPlanar(context.Background(), s, 100)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(m.Edges(), s.Edges()) || !reflect.DeepEqual(m.Faces(), s.Faces()) {
		t.Fatal("ancestry")
	}
	inv, e := p.Inverse()
	if e != nil {
		t.Fatal(e)
	}
	back, e := inv.ApplyPlanar(context.Background(), m, 100)
	if e != nil || !reflect.DeepEqual(back.Vertices(), s.Vertices()) {
		t.Fatal("inverse", e)
	}
	if e = m.Tessellate().ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if m.Tessellate().SignedVolume() != 24 {
		t.Fatal("mass")
	}
}
func TestPlacementRejects(t *testing.T) {
	for _, axes := range [][3]int{{-1, 2, 3}, {1, 1, 3}, {4, 2, 3}} {
		if _, e := placement.New(axes, geom.Vec3{}); e == nil {
			t.Fatal("invalid frame")
		}
	}
	s, _ := brep.Box(geom.Vec3{X: .1}, geom.Vec3{X: 1, Y: 1, Z: 1})
	p, _ := placement.New([3]int{1, 2, 3}, geom.Vec3{X: .2})
	if _, e := p.ApplyPlanar(context.Background(), s, 100); e == nil {
		t.Fatal("rounded coordinate")
	}
	if _, e := p.ApplyPlanar(context.Background(), s, 1); e == nil {
		t.Fatal("budget")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := p.ApplyPlanar(ctx, s, 100); e == nil {
		t.Fatal("cancel")
	}
}
