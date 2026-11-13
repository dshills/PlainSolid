package solid

import (
	"errors"
	"math"
	"testing"

	"plainsolid/brep"
	"plainsolid/geom"
)

func TestOrthogonalBRep(t *testing.T) {
	outer := []geom.Vec3{{}, {X: 4}, {X: 4, Y: 2}, {X: 3, Y: 2}, {X: 3, Y: 4}, {Y: 4}}
	holes := [][]geom.Vec3{{{X: 1, Y: 1}, {X: 2, Y: 1}, {X: 2, Y: 2}, {X: 1, Y: 2}}}
	b, e := brep.PrismWithHoles(outer, holes, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	s, e := FromOrthogonal(b, Options{})
	if e != nil || s.Volume() != 26 || s.Contains(geom.Vec3{X: 1.5, Y: 1.5, Z: 1}) || s.Contains(geom.Vec3{X: 3.5, Y: 3.5, Z: 1}) {
		t.Fatalf("invalid solid %g %v", s.Volume(), e)
	}
	tool := must(t, box(0, 0, 0, 4, 4, 1))
	r, e := s.Difference(tool)
	if e != nil || r.Volume() != 13 {
		t.Fatal(r.Volume(), e)
	}
	out, e := r.BRep(Options{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := out.MassProperties()
	if e != nil || math.Abs(p.Volume-13) > 1e-10 || out.Topology().EulerCharacteristic != 0 {
		t.Fatalf("result topology %+v properties %+v %v", out.Topology(), p, e)
	}
	again, e := FromOrthogonal(out, Options{})
	if e != nil || again.Volume() != 13 {
		t.Fatal(again.Volume(), e)
	}

	if _, e = FromOrthogonal(b.Reversed(), Options{}); e == nil {
		t.Fatal("inward boundary accepted")
	}
	if _, e = FromOrthogonal(b, Options{MaxCells: 1}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
}
