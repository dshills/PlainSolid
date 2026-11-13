package collision

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"testing"
)

func TestAssemblyNestedVoidAncestry(t *testing.T) {
	outer := dbox(t, geom.Vec3{}, geom.Vec3{X: 6, Y: 6, Z: 6})
	hole := dbox(t, geom.Vec3{X: 1, Y: 1, Z: 1}, geom.Vec3{X: 5, Y: 5, Z: 5}).Reversed()
	island := dbox(t, geom.Vec3{X: 2, Y: 2, Z: 2}, geom.Vec3{X: 3, Y: 3, Z: 3})
	r, e := ValidateAssembly(context.Background(), []brep.Solid{island, hole, outer}, AssemblyOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if r.SignedVolume.Cmp(big.NewRat(153, 1)) != 0 || len(r.Shells) != 3 {
		t.Fatal(r)
	}
	for i, w := range []struct{ depth, parent int }{{2, 1}, {1, 2}, {0, -1}} {
		s := r.Shells[i]
		if s.Depth != w.depth || s.Parent != w.parent || len(s.Faces) != 6 {
			t.Fatal(i, s)
		}
		for _, f := range s.Faces {
			if f.Solid != i {
				t.Fatal("ancestry")
			}
		}
	}
}
func TestAssemblyRejectContactsAndOrientation(t *testing.T) {
	a := dbox(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 2})
	for _, b := range []brep.Solid{a, dbox(t, geom.Vec3{X: 1, Y: 1, Z: 1}, geom.Vec3{X: 3, Y: 3, Z: 3}), dbox(t, geom.Vec3{X: 2}, geom.Vec3{X: 3, Y: 1, Z: 1}), dbox(t, geom.Vec3{X: 2, Y: 2, Z: 2}, geom.Vec3{X: 3, Y: 3, Z: 3}), dbox(t, geom.Vec3{X: .5, Y: .5, Z: .5}, geom.Vec3{X: 1, Y: 1, Z: 1})} {
		r, e := ValidateAssembly(context.Background(), []brep.Solid{a, b}, AssemblyOptions{})
		if e == nil || len(r.Shells) != 0 {
			t.Fatal("accepted invalid assembly")
		}
	}
	b := dbox(t, geom.Vec3{X: 3}, geom.Vec3{X: 4, Y: 1, Z: 1})
	r, e := ValidateAssembly(context.Background(), []brep.Solid{a, b}, AssemblyOptions{})
	if e != nil || len(r.Shells) != 2 || r.Shells[1].Parent != -1 {
		t.Fatal(e, r)
	}
}
func TestAssemblyLimitsAndOwnedReports(t *testing.T) {
	s := projectionBox(t)
	if _, e := ValidateAssembly(context.Background(), []brep.Solid{s, s}, AssemblyOptions{MaxSolids: 1}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
	if _, e := ValidateAssembly(context.Background(), []brep.Solid{s}, AssemblyOptions{Validation: LocationOptions{MaxTriangles: 1}}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
	p, e := NewPointLocator(context.Background(), s, LocationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a := p.Shells()
	a[0].SignedVolume.SetInt64(0)
	a[0].Triangles[0] = 99
	b := p.Shells()
	if b[0].SignedVolume.Cmp(big.NewRat(1, 1)) != 0 || b[0].Triangles[0] != 0 {
		t.Fatal("report aliases locator")
	}
	r, e := ValidateAssembly(context.Background(), nil, AssemblyOptions{})
	if e != nil || len(r.Shells) != 0 || r.SignedVolume.Sign() != 0 {
		t.Fatal(e)
	}
}
