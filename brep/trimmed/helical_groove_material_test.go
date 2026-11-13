package trimmed

import (
	"context"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestHelicalGrooveAxialProbe(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	p := []geom.Vec2{{X: 1.5, Y: .5}, {X: 2, Y: .375}, {X: 2, Y: .625}}
	part, e := BuildHelicalGrooveMaterial(ctx, HelicalGrooveRecipe{Frame: f, OuterRadius: 2, Height: 4,
		Profile: p, Lead: 1, Start: .25, End: 2.25, Hand: geom.RightHanded, Starts: 2})
	if e != nil || part.Solid().Topology().InnerWires != 2 {
		t.Fatal(part.Solid().Topology(), e)
	}
	p[0] = geom.Vec2{} // immutable recipe is already captured
	for _, c := range []struct {
		radius, z *big.Rat
		ray       [2]*big.Rat
		want      parametric.Location
		start     int
	}{
		{big.NewRat(7, 4), big.NewRat(2, 1), [2]*big.Rat{big.NewRat(-1, 1), big.NewRat(0, 1)}, parametric.Outside, 0},
		{big.NewRat(7, 4), big.NewRat(9, 4), [2]*big.Rat{big.NewRat(-1, 1), big.NewRat(0, 1)}, parametric.Inside, -1},
		{big.NewRat(7, 4), big.NewRat(5, 2), [2]*big.Rat{big.NewRat(-1, 1), big.NewRat(0, 1)}, parametric.Outside, 1},
		{big.NewRat(7, 4), big.NewRat(3, 4), [2]*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1)}, parametric.Boundary, 0},
		{big.NewRat(3, 2), big.NewRat(2, 1), [2]*big.Rat{big.NewRat(-1, 1), big.NewRat(0, 1)}, parametric.Boundary, 0},
		{big.NewRat(2, 1), big.NewRat(9, 4), [2]*big.Rat{big.NewRat(-1, 1), big.NewRat(0, 1)}, parametric.Boundary, -1},
		{big.NewRat(2, 1), big.NewRat(2, 1), [2]*big.Rat{big.NewRat(-1, 1), big.NewRat(0, 1)}, parametric.Outside, 0},
	} {
		q, r, e := part.ClassifyAxialProbe(ctx, c.radius, c.z, c.ray, geom.HelicalStartsLineOptions{})
		if e != nil || !r.Complete || q != c.want {
			t.Fatal(c.radius, c.z, q, c.want, e)
		}
		if c.start >= 0 {
			found := false
			for _, x := range r.Intervals {
				if x.Source.Start == c.start && c.z.Cmp(x.Bounds[0][0]) >= 0 && c.z.Cmp(x.Bounds[1][1]) <= 0 {
					found = true
				}
			}
			if !found {
				t.Fatal("missing source start", c.start)
			}
		}
	}
}
