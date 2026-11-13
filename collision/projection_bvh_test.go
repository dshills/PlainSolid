package collision

import (
	"context"
	"plainsolid/brep"
	"plainsolid/geom"
	"testing"
)

func bruteProjection(items []distanceItem, p geom.Vec3) (closest, int) {
	x := exactTriangle([3]geom.Vec3{p})[0]
	var best closest
	winner := -1
	for i, item := range items {
		var c closest
		t := exactTriangle(item.t)
		if q, ok := faceProjection(x, t); ok {
			c.consider(x, q)
		}
		for j := range t {
			c.consider(x, projection(x, t[j], t[(j+1)%3]))
		}
		if best.squared == nil || c.squared.Cmp(best.squared) < 0 {
			best = c
			winner = i
		}
	}
	return best, winner
}
func TestProjectionBVHSeparatedComponents(t *testing.T) {
	var parts []brep.Solid
	for i := 0; i < 100; i++ {
		x := float64(i * 10)
		parts = append(parts, dbox(t, geom.Vec3{X: x}, geom.Vec3{X: x + 1, Y: 1, Z: 1}))
	}
	s := locationJoin(t, parts...)
	p, e := NewPointProjector(context.Background(), s, ProjectionOptions{MaxWork: 100})
	if e != nil {
		t.Fatal(e)
	}
	query := geom.Vec3{X: -1, Y: .5, Z: .5}
	r, e := p.Project(context.Background(), query)
	if e != nil {
		t.Fatal(e)
	}
	want, id := bruteProjection(p.items, query)
	if r.Triangle != id || r.SquaredDistance.Cmp(want.squared) != 0 || !same(r.ExactPoint, want.b) {
		t.Fatal("flat mismatch")
	}
	if r.Work >= 100 || r.NarrowPhase >= 10 {
		t.Fatal("poor pruning", r.Work, r.NarrowPhase)
	}
	t.Logf("1200 triangles: %d node bounds, %d triangle predicates", r.Work, r.NarrowPhase)
}
func TestProjectionBVHAllWitnessTies(t *testing.T) {
	s := projectionBox(t)
	p, e := NewPointProjector(context.Background(), s, ProjectionOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for ix := -2; ix <= 4; ix++ {
		for iy := -2; iy <= 4; iy++ {
			for iz := -2; iz <= 4; iz++ {
				q := geom.Vec3{X: float64(ix) / 2, Y: float64(iy) / 2, Z: float64(iz) / 2}
				r, e := p.Project(context.Background(), q)
				if e != nil {
					t.Fatal(e)
				}
				want, id := bruteProjection(p.items, q)
				if r.Triangle != id || r.SquaredDistance.Cmp(want.squared) != 0 || !same(r.ExactPoint, want.b) {
					t.Fatal(q, r.Triangle, id)
				}
			}
		}
	}
}
