package plaincadbridge

import (
	"context"
	"plainsolid/brep"
	"plainsolid/csg/poly"
	"plainsolid/geom"
	"testing"
)

func TestConnectedCoplanarNativeFaceQuery(t *testing.T) {
	a, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 20, Y: 10, Z: 8})
	c, _ := brep.Box(geom.Vec3{X: 1, Y: 1, Z: 1}, geom.Vec3{X: 19, Y: 9, Z: 9})
	shell, e := poly.Combine(context.Background(), a, c, poly.Difference, poly.Options{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := matchingPlanarFace(shell, geom.Vec3{Z: 8}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	near(t, p.Area, 56)
	p, e = matchingPlanarFace(shell, geom.Vec3{}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	near(t, p.Area, 200)
}
func TestDisconnectedCoplanarNativeFacesStayAmbiguous(t *testing.T) {
	a, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 20, Y: 10, Z: 8})
	c, _ := brep.Box(geom.Vec3{X: 9, Y: -1, Z: -1}, geom.Vec3{X: 11, Y: 11, Z: 9})
	split, e := poly.Combine(context.Background(), a, c, poly.Difference, poly.Options{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = matchingPlanarFace(split, geom.Vec3{Z: 8}, geom.Vec3{Z: 1}); e == nil {
		t.Fatal("accepted two disjoint cap regions")
	}
}
