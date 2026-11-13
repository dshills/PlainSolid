package parametric

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/geom"
)

func TestCompoundBoundaryAndPlacement(t *testing.T) {
	s, e := Sphere(geom.Vec3{}, 1)
	if e != nil {
		t.Fatal(e)
	}
	c, e := NewCompound([]Component{
		{ID: "left", Solid: s, Transform: geom.Translation(geom.Vec3{X: -3})},
		{ID: "right", Solid: s, Transform: geom.Translation(geom.Vec3{X: 3}).Mul(geom.Scaling(geom.Vec3{X: -1, Y: 1, Z: 1}))},
	})
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	p, e := c.Boundary(ctx, 2)
	if e != nil {
		t.Fatal(e)
	}
	g, e := p.MassProperties(ctx, IntegrationOptions{})
	if e != nil || math.Abs(g.Volume-8*math.Pi/3) > 1e-9 || g.Centroid.Length() > 1e-9 {
		t.Fatal(g, e)
	}
	box, e := c.Bounds()
	if e != nil || !box.Contains(geom.Vec3{X: -4}) || !box.Contains(geom.Vec3{X: 4}) {
		t.Fatal(box, e)
	}
	if _, e = c.Boundary(ctx, 1); !errors.Is(e, ErrBudget) {
		t.Fatal(e)
	}
}
