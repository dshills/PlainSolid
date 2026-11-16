package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestSplineFaceCoincidenceProjectiveOriginalOuterAndHole(t *testing.T) {
	ctx := context.Background()
	var solids [2]Solid
	for owner := range solids {
		var h [][4]*big.Rat
		for u := int64(0); u <= 1; u++ {
			for v := int64(0); v <= 1; v++ {
				x, y := big.NewRat(u, 1), big.NewRat(v, 1)
				dx, dy := big.NewRat(1, 1), big.NewRat(1, 1)
				if owner == 1 {
					x, dx, dy = big.NewRat(1-u, 1), big.NewRat(1+2*u, 1), big.NewRat(1+2*v, 3)
				}
				h = append(h, [4]*big.Rat{new(big.Rat).Mul(x, dy), new(big.Rat).Mul(y, dx),
					new(big.Rat).Mul(x, y), new(big.Rat).Mul(dx, dy)})
			}
		}
		p, err := geom.NewExactHomogeneousBezierSurface(ctx, 1, 1, h, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		surface, err := geom.NewExactBezierSurface(p)
		if err != nil {
			t.Fatal(err)
		}
		solids[owner] = Solid{faces: []Face{{Surface: surface, Reversed: owner == 1}}}
	}
	loop := func(low, high geom.Vec2, clockwise bool, offset int) [2][]Use {
		points := [4]geom.Vec2{low, {X: high.X, Y: low.Y}, high, {X: low.X, Y: high.Y}}
		if clockwise {
			points[1], points[3] = points[3], points[1]
		}
		var all [2][]Use
		for i, p := range points {
			q := points[(i+1)%len(points)]
			all[0] = append(all[0], Use{Edge: offset + i, UVStart: p, UVEnd: q})
			// Author B's homogeneous cubic from the exact endpoint
			// projective weights, using B's native reversed wire order.
			point := func(v geom.Vec2) [3]*big.Rat {
				u, v2 := ratInset(v.X), ratInset(v.Y)
				dx := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Mul(big.NewRat(2, 1), u))
				dy := new(big.Rat).Sub(big.NewRat(3, 1), new(big.Rat).Mul(big.NewRat(2, 1), v2))
				x := new(big.Rat).Sub(big.NewRat(1, 1), new(big.Rat).Quo(new(big.Rat).Mul(big.NewRat(3, 1), u), dx))
				y := new(big.Rat).Quo(v2, dy)
				w := dx
				if p.X == q.X {
					w = dy
				}
				return [3]*big.Rat{new(big.Rat).Mul(w, x), new(big.Rat).Mul(w, y), w}
			}
			h0, h1 := point(q), point(p)
			h := make([][3]*big.Rat, 4)
			for j := range h {
				for k := range h0 {
					h[j][k] = new(big.Rat).Add(new(big.Rat).Mul(h0[k], big.NewRat(int64(3-j), 3)), new(big.Rat).Mul(h1[k], big.NewRat(int64(j), 3)))
				}
			}
			c, err := NewExactBezierUV(h)
			if err != nil {
				t.Fatal(err)
			}
			all[1] = append(all[1], Use{Edge: offset + 100 + i, UVStart: c.ends[0], UVEnd: c.ends[1], Curve: c})
		}
		all[1][0], all[1][3] = all[1][3], all[1][0]
		all[1][1], all[1][2] = all[1][2], all[1][1]
		all[1] = append(all[1][1:], all[1][0])
		return all
	}
	outer := loop(geom.Vec2{}, geom.Vec2{X: 1, Y: 1}, false, 10)
	hole := loop(geom.Vec2{X: .25, Y: .5}, geom.Vec2{X: .5, Y: .75}, true, 20)
	for i := range solids {
		solids[i].faces[0].Outer = outer[i]
		solids[i].faces[0].Holes = [][]Use{hole[i]}
	}
	axes := [2]int{0, 1}
	o := SplineFaceCoincidenceOptions{ProjectiveAxes: &axes}
	r, err := solids[0].CertifySplineFaceCoincidence(ctx, 0, solids[1], 0, o)
	if err != nil || !r.Complete || r.ProjectiveMap == nil || r.OppositeOrientation || len(r.Uses) != 8 ||
		r.ProjectiveMap.Orientation != -1 || r.ProjectiveMap.BStationRatios[0].Cmp(big.NewRat(3, 1)) != 0 ||
		r.ProjectiveMap.BStationRatios[1].Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal("supported whole projective face failed to prove both original wires", r, err)
	}
	for _, x := range r.Uses {
		if x.EdgeB != x.EdgeA+100 || x.LoopA != x.LoopB || !x.Reversed || x.RangeB[0].Cmp(big.NewRat(1, 1)) != 0 {
			t.Fatal("incorrect coedge owner or direction", x)
		}
	}
	short := o
	short.MaxOperations = r.ExactOperations - 1
	if r, err := solids[0].CertifySplineFaceCoincidence(ctx, 0, solids[1], 0, short); err == nil || r.Complete || len(r.Uses) != 0 {
		t.Fatal("partial original wire proof must not be published on short work", r, err)
	}
	r, err = solids[0].CertifySplineFaceCoincidence(ctx, 0, solids[1], 0, SplineFaceCoincidenceOptions{})
	if !errors.Is(err, geom.ErrConvergence) || r.Complete {
		t.Fatal("affine chart path must not claim a projective face", r, err)
	}
	use := &solids[1].faces[0].Holes[0][0]
	c := use.Curve.(ExactBezierUV)
	control := c.HomogeneousControls()
	axis := 0
	if use.UVStart.X != use.UVEnd.X {
		axis = 1
	}
	control[1][axis].Add(control[1][axis], new(big.Rat).Quo(control[1][2], big.NewRat(100, 1)))
	use.Curve, err = NewExactBezierUV(control)
	if err != nil {
		t.Fatal(err)
	}
	r, err = solids[0].CertifySplineFaceCoincidence(ctx, 0, solids[1], 0, o)
	if err == nil || r.Complete || r.ProjectiveMap != nil || len(r.Uses) != 0 {
		t.Fatal("matching original vertices did not justify altered hole geometry", r, err)
	}
}
