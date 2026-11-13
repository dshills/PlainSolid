package csg

import (
	"math"
	"plainsolid/geom"
	"testing"
)

func TestNearDependentPlaneTriple(t *testing.T) {
	epsilon := 1e-14
	p := plane{n: geom.Vec3{X: 1}, d: 1}
	q := plane{n: geom.Vec3{X: 1, Y: epsilon}, d: 1 + epsilon}
	r := plane{n: geom.Vec3{Z: 1}, d: 2}
	point, exists, e := exactIntersection(p, q, r, []plane{p, q, r}, 1e-12)
	if e != nil || !exists {
		t.Fatalf("near-dependent triple lost: %v, %v", exists, e)
	}
	near(t, point.X, 1)
	near(t, point.Y, (q.d-p.d)/epsilon)
	near(t, point.Z, 2)
	if _, exists, e = exactIntersection(p, p, r, nil, 1e-12); e != nil || exists {
		t.Fatal("exactly dependent triple must not yield a point", exists, e)
	}
	if _, exists, e = exactIntersection(p, q, r, []plane{{n: geom.Vec3{X: 1}, d: 0}}, 1e-12); e != nil || exists {
		t.Fatal("outside candidate accepted", exists, e)
	}
}

func TestPlaneMergeRespectsModelDistance(t *testing.T) {
	p := plane{n: geom.Vec3{Y: 1}, d: 0}
	q := plane{n: geom.Vec3{X: 1e-13, Y: 1}, d: 0}
	if equivalentPlanes(p, q, []geom.Vec3{{X: 1e8}}, nil, geom.Vec3{}, 1e-9) {
		t.Fatal("angularly close planes differ by 1e-5 over the model and must not merge")
	}
	if !equivalentPlanes(p, q, []geom.Vec3{{X: 1}}, nil, geom.Vec3{}, 1e-9) {
		t.Fatal("distance-equivalent planes should merge")
	}
	if !equivalentPlanes(p, plane{n: p.n, d: 100}, nil, nil, geom.Vec3{}, 1e-9) {
		t.Fatal("exact parallel planes may discard their weaker constraint")
	}
}

func TestNearParallelLongBoxesFailOrPreserveClip(t *testing.T) {
	// A tiny rotation still removes measurable volume across a long lever arm.
	// Returning the uncut box because the normals look alike is incorrect.
	length := 1e7
	angle := 1e-13
	a := box(t, geom.Vec3{X: -length, Y: -1, Z: -1}, geom.Vec3{X: length, Y: 1, Z: 1})
	rotation, e := geom.Rotation(geom.Vec3{Z: 1}, angle)
	if e != nil {
		t.Fatal(e)
	}
	b, e := a.Transformed(rotation)
	if e != nil {
		t.Fatal(e)
	}
	result, e := Intersect(a, b, 1e-9)
	if e != nil {
		t.Logf("ill-conditioned geometry rejected explicitly: %v", e)
		return
	} // An explicit reconstruction error is permitted for this ill-conditioned case.
	volume := result.Tessellate().SignedVolume()
	// To first order, the two long-side triangles remove angle*length^2
	// in area. The O(angle^2*length) term is below float64 resolution here.
	expected := 8*length - 2*angle*length*length
	if math.Abs(volume-expected) > 0.1 {
		t.Fatalf("near-parallel clipping silently lost: volume %g expected %g or explicit error", volume, expected)
	}
}
