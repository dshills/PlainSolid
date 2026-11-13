package collision

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"sync"
	"testing"
)

func projectionBox(t *testing.T) brep.Solid {
	t.Helper()
	s, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestPointProjectionSigned(t *testing.T) {
	s := projectionBox(t)
	p, e := NewPointProjector(context.Background(), s, ProjectionOptions{Signed: true})
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		p   geom.Vec3
		d   float64
		loc Location
	}{{geom.Vec3{X: .5, Y: .5, Z: 2}, 1, Outside}, {geom.Vec3{X: .5, Y: .5, Z: .5}, -.5, Inside}, {geom.Vec3{X: 1, Y: .5, Z: .5}, 0, OnBoundary}, {geom.Vec3{X: 2, Y: 2, Z: 2}, math.Sqrt(3), Outside}} {
		r, e := p.Project(context.Background(), tc.p)
		if e != nil {
			t.Fatal(e)
		}
		if !r.HasLocation || r.Location.Location != tc.loc || r.SignedDistance != tc.d || r.Face != s.TriangleFaces()[r.Triangle] {
			t.Fatal(r)
		}
		delta := minus(exactTriangle([3]geom.Vec3{tc.p})[0], r.ExactPoint)
		if dot(delta, delta).Cmp(r.SquaredDistance) != 0 {
			t.Fatal("witness")
		}
	}
	// Center is tied on six faces; exact winner must be first triangle.
	r, e := p.Project(context.Background(), geom.Vec3{X: .5, Y: .5, Z: .5})
	if e != nil || r.Triangle != 0 {
		t.Fatal(e, r.Triangle)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := p.Project(context.Background(), geom.Vec3{Z: -.25})
			if e != nil || r.SquaredDistance.Cmp(big.NewRat(1, 16)) != 0 {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
}
func TestProjectionHolesAndConcavity(t *testing.T) {
	outer := []geom.Vec3{{}, {X: 4}, {X: 4, Y: 4}, {Y: 4}}
	hole := []geom.Vec3{{X: 1, Y: 1}, {X: 1, Y: 3}, {X: 3, Y: 3}, {X: 3, Y: 1}}
	s, e := brep.PrismWithHoles(outer, [][]geom.Vec3{hole}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	r, e := ProjectBoundary(context.Background(), s, geom.Vec3{X: 2, Y: 2, Z: .5}, ProjectionOptions{Signed: true})
	if e != nil || r.SquaredDistance.Cmp(big.NewRat(1, 1)) != 0 || r.Location.Location != Outside {
		t.Fatal(e, r)
	}
	s, e = brep.Prism([]geom.Vec3{{}, {X: 2}, {X: 2, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 2}, {Y: 2}}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	r, e = ProjectBoundary(context.Background(), s, geom.Vec3{X: 1.5, Y: 1.5, Z: .5}, ProjectionOptions{})
	if e != nil || r.SquaredDistance.Cmp(big.NewRat(1, 4)) != 0 {
		t.Fatal(e, r)
	}
}
func TestProjectionNearAndFailures(t *testing.T) {
	s := projectionBox(t)
	gap := math.Ldexp(1, -50)
	r, e := ProjectBoundary(context.Background(), s, geom.Vec3{X: -gap, Y: .5, Z: .5}, ProjectionOptions{})
	if e != nil || r.Distance != gap {
		t.Fatal(e, r)
	}
	if _, e = ProjectBoundary(context.Background(), s, geom.Vec3{}, ProjectionOptions{MaxWork: 1}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
	if _, e = ProjectBoundary(context.Background(), brep.Solid{}, geom.Vec3{}, ProjectionOptions{}); !errors.Is(e, ErrEmpty) {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = ProjectBoundary(ctx, s, geom.Vec3{}, ProjectionOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestProjectionAffineRationalWitness(t *testing.T) {
	a := geom.Identity()
	a[2][0] = .5
	s, e := projectionBox(t).Transformed(a)
	if e != nil {
		t.Fatal(e)
	}
	r, e := ProjectBoundary(context.Background(), s, geom.Vec3{X: .5, Y: .5, Z: 2}, ProjectionOptions{Signed: true})
	if e != nil {
		t.Fatal(e)
	}
	want := [3]*big.Rat{big.NewRat(4, 5), big.NewRat(1, 2), big.NewRat(7, 5)}
	for i := range want {
		if r.ExactPoint[i].Cmp(want[i]) != 0 {
			t.Fatal(r.ExactPoint)
		}
	}
	if r.SquaredDistance.Cmp(big.NewRat(9, 20)) != 0 {
		t.Fatal(r.SquaredDistance)
	}
}
func FuzzProjectionBox(f *testing.F) {
	f.Add(int16(10), int16(20), int16(-4))
	f.Fuzz(func(t *testing.T, ix, iy, iz int16) {
		s := projectionBox(t)
		v := geom.Vec3{X: float64(ix%64) / 16, Y: float64(iy%64) / 16, Z: float64(iz%64) / 16}
		r, e := ProjectBoundary(context.Background(), s, v, ProjectionOptions{})
		if e != nil {
			t.Fatal(e)
		}
		coords := []float64{v.X, v.Y, v.Z}
		squared := 0.
		inside := true
		nearest := math.Inf(1)
		for _, x := range coords {
			d := 0.
			if x < 0 {
				d = -x
				inside = false
			} else if x > 1 {
				d = x - 1
				inside = false
			}
			squared += d * d
			nearest = math.Min(nearest, math.Min(x, 1-x))
		}
		if inside {
			squared = nearest * nearest
		}
		if r.SquaredDistance.Cmp(new(big.Rat).SetFloat64(squared)) != 0 {
			t.Fatal(v, r.SquaredDistance, squared)
		}
	})
}
