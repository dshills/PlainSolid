package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func coincidenceFaceFixture(t *testing.T, swap, ru, rv bool) (Solid, Solid) {
	t.Helper()
	surface := func(mapped bool) geom.Surface {
		var p []geom.Vec3
		for i := 0; i < 2; i++ {
			for j := 0; j < 2; j++ {
				x, y := float64(i), float64(j)
				if mapped {
					if ru {
						x = 1 - x
					}
					if rv {
						y = 1 - y
					}
					if swap {
						x, y = y, x
					}
				}
				p = append(p, geom.Vec3{X: x, Y: y, Z: x * y})
			}
		}
		u, v := []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}
		if mapped {
			u = []float64{2, 2, 5, 5}
			v = []float64{-1, -1, 4, 4}
		}
		s, e := geom.NewBSplineSurface(1, 1, 2, 2, p, u, v, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	outer := []geom.Vec2{{X: .125, Y: .125}, {X: .875, Y: .125}, {X: .875, Y: .875}, {X: .125, Y: .875}}
	hole := []geom.Vec2{{X: .375, Y: .375}, {X: .375, Y: .625}, {X: .625, Y: .625}, {X: .625, Y: .375}}
	loop := func(points []geom.Vec2, base int) []Use {
		var r []Use
		for i, p := range points {
			r = append(r, Use{Edge: base + i, UVStart: p, UVEnd: points[(i+1)%len(points)]})
		}
		return r
	}
	fa := Face{Surface: surface(false), Outer: loop(outer, 10), Holes: [][]Use{loop(hole, 20)}}
	h := [][3]*big.Rat{{big.NewRat(1, 8), big.NewRat(1, 8), big.NewRat(1, 1)}, {big.NewRat(3, 8), big.NewRat(1, 16), big.NewRat(1, 1)}, {big.NewRat(5, 8), big.NewRat(1, 16), big.NewRat(1, 1)}, {big.NewRat(7, 8), big.NewRat(1, 8), big.NewRat(1, 1)}}
	q, e := NewExactBezierUV(h)
	if e != nil {
		t.Fatal(e)
	}
	fa.Outer[0].Curve = q
	mapPoint := func(x, y *big.Rat) (*big.Rat, *big.Rat) {
		x = new(big.Rat).Set(x)
		y = new(big.Rat).Set(y)
		if swap {
			x, y = y, x
		}
		if ru {
			x.Sub(big.NewRat(1, 1), x)
		}
		if rv {
			y.Sub(big.NewRat(1, 1), y)
		}
		return x.Add(big.NewRat(2, 1), x.Mul(x, big.NewRat(3, 1))), y.Add(big.NewRat(-1, 1), y.Mul(y, big.NewRat(5, 1)))
	}
	negative := (swap != ru) != rv
	mapLoop := func(l []Use) []Use {
		r := make([]Use, len(l))
		for i, u := range l {
			a, b := mapPoint(ratInset(u.UVStart.X), ratInset(u.UVStart.Y))
			c, d := mapPoint(ratInset(u.UVEnd.X), ratInset(u.UVEnd.Y))
			x, _ := a.Float64()
			y, _ := b.Float64()
			z, _ := c.Float64()
			w, _ := d.Float64()
			u.UVStart = geom.Vec2{X: x, Y: y}
			u.UVEnd = geom.Vec2{X: z, Y: w}
			u.Edge += 100
			if q, ok := u.Curve.(ExactBezierUV); ok {
				h := q.HomogeneousControls()
				for j := range h {
					h[j][0], h[j][1] = mapPoint(h[j][0], h[j][1])
				}
				mapped, e := NewExactBezierUV(h)
				if e != nil {
					t.Fatal(e)
				}
				u.Curve = mapped
			}
			if negative {
				u.UVStart, u.UVEnd = u.UVEnd, u.UVStart
				if q, ok := u.Curve.(ExactBezierUV); ok {
					u.Curve = q.reversed()
				}
				r[len(l)-1-i] = u
			} else {
				r[i] = u
			}
		}
		return append(r[1:], r[:1]...)
	}
	fb := Face{Surface: surface(true), Outer: mapLoop(fa.Outer), Holes: [][]Use{mapLoop(fa.Holes[0])}, Reversed: negative}
	return Solid{faces: []Face{fa}}, Solid{faces: []Face{fb}}
}
func TestSplineFaceCoincidenceMappedWholeTrims(t *testing.T) {
	for n := 0; n < 8; n++ {
		a, b := coincidenceFaceFixture(t, n&4 != 0, n&1 != 0, n&2 != 0)
		r, e := a.CertifySplineFaceCoincidence(context.Background(), 0, b, 0, SplineFaceCoincidenceOptions{})
		if e != nil || !r.Complete || len(r.Uses) != 8 || r.OppositeOrientation {
			t.Fatal(n, r, e)
		}
		for _, u := range r.Uses {
			la := append([][]Use{a.faces[0].Outer}, a.faces[0].Holes...)
			lb := append([][]Use{b.faces[0].Outer}, b.faces[0].Holes...)
			if u.EdgeA != la[u.LoopA][u.UseA].Edge || u.EdgeB != lb[u.LoopB][u.UseB].Edge {
				t.Fatal("source owners")
			}
		}
	}
}
func TestSplineFaceCoincidenceRejectsChangedTrim(t *testing.T) {
	a, b := coincidenceFaceFixture(t, true, true, false)
	for i, u := range b.faces[0].Outer {
		if q, ok := u.Curve.(ExactBezierUV); ok {
			h := q.HomogeneousControls()
			h[1][0].Add(h[1][0], big.NewRat(1, 64))
			b.faces[0].Outer[i].Curve, _ = NewExactBezierUV(h)
		}
	}
	r, e := a.CertifySplineFaceCoincidence(context.Background(), 0, b, 0, SplineFaceCoincidenceOptions{})
	if e == nil || r.Complete || len(r.Uses) != 0 {
		t.Fatal("same endpoints unequal interior", r, e)
	}
	a, b = coincidenceFaceFixture(t, false, false, false)
	b.faces[0].Holes = nil
	if r, e = a.CertifySplineFaceCoincidence(context.Background(), 0, b, 0, SplineFaceCoincidenceOptions{}); e == nil || r.Complete {
		t.Fatal("missing hole")
	}
	a, b = coincidenceFaceFixture(t, false, false, false)
	if r, e = a.CertifySplineFaceCoincidence(context.Background(), 0, b, 0, SplineFaceCoincidenceOptions{MaxOperations: 1}); e == nil || r.Complete || len(r.Uses) != 0 {
		t.Fatal("quota", r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = a.CertifySplineFaceCoincidence(ctx, 0, b, 0, SplineFaceCoincidenceOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestSplineFaceCoincidenceOriginalNativeFaces(t *testing.T) {
	a, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	b, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	r, e := a.CertifySplineFaceCoincidence(context.Background(), 4, b, 4, SplineFaceCoincidenceOptions{})
	if e != nil || !r.Complete || r.Faces != [2]int{4, 4} || len(r.Uses) != 4 {
		t.Fatal(r, e)
	}
	b.faces[4].Reversed = !b.faces[4].Reversed
	r, e = a.CertifySplineFaceCoincidence(context.Background(), 4, b, 4, SplineFaceCoincidenceOptions{})
	if e != nil || !r.OppositeOrientation {
		t.Fatal(r, e)
	}
	// A distinct original support cannot pass merely because its trim matches.
	if r, e = a.CertifySplineFaceCoincidence(context.Background(), 4, b, 5, SplineFaceCoincidenceOptions{}); e == nil || r.Complete {
		t.Fatal(r, e)
	}
}
