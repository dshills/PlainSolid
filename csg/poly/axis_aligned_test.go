package poly

import (
	"context"
	"errors"
	"math"
	"plainsolid/brep"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestPlainCADAxisAlignedJoinThenCut(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for axis := 0; axis < 3; axis++ {
		for _, sign := range []float64{-1, 1} {
			makeBox := func(radius, height float64) brep.Solid {
				lo := [3]float64{-radius, -radius, -radius}
				hi := [3]float64{radius, radius, radius}
				lo[axis] = math.Min(0, sign*height)
				hi[axis] = math.Max(0, sign*height)
				return box(t, lo[0]+3, lo[1]+5, lo[2]+7, hi[0]+3, hi[1]+5, hi[2]+7)
			}
			a, b := makeBox(5, 10), makeBox(2.5, 12)
			av, bv := a.Vertices(), b.Vertices()
			joined, e := Combine(ctx, a, b, Union, Options{})
			if e != nil {
				t.Fatal(e)
			}
			if math.Abs(volume(t, joined)-1050) > 1e-9 {
				t.Fatal("join volume")
			}
			// Keep the generic union's rounded diagonal subdivisions to reproduce the
			// original browser failure at the subsequent exact subtraction.
			var sources []Source
			if _, e = combine(ctx, joined, b, Difference, Options{}, &sources); !errors.Is(e, ErrPrecision) {
				t.Fatal("regression no longer exercises precision fallback", e)
			}
			cut, e := Combine(ctx, joined, b, Difference, Options{})
			if e != nil {
				t.Fatal(e)
			}
			if math.Abs(volume(t, cut)-750) > 1e-9 {
				t.Fatal("cut volume")
			}
			if e = ValidateEmbeddedBoundary(ctx, cut, Options{}); e != nil {
				t.Fatal(e)
			}
			allowed := [3]map[float64]bool{{}, {}, {}}
			for _, v := range append(joined.Vertices(), b.Vertices()...) {
				for i, x := range []float64{v.X, v.Y, v.Z} {
					allowed[i][x] = true
				}
			}
			for _, v := range cut.Vertices() {
				for i, x := range []float64{v.X, v.Y, v.Z} {
					if !allowed[i][x] {
						t.Fatal("invented coordinate", v)
					}
				}
			}
			if !reflect.DeepEqual(av, a.Vertices()) || !reflect.DeepEqual(bv, b.Vertices()) {
				t.Fatal("mutated inputs")
			}
		}
	}
}
func TestAxisAlignedExactCellsAndCavities(t *testing.T) {
	ctx := context.Background()
	a := box(t, 0, 0, 0, 10, 10, 10)
	inner := box(t, 2, 2, 2, 8, 8, 8)
	cavity, handled, e := CombineAxisAligned(ctx, a, inner, Difference, Options{RequireExactConversion: true})
	if !handled || e != nil {
		t.Fatal(handled, e)
	}
	if math.Abs(volume(t, cavity)-784) > 1e-9 {
		t.Fatal(volume(t, cavity))
	}
	restored, handled, e := CombineAxisAligned(ctx, cavity, inner, Union, Options{})
	if !handled || e != nil || math.Abs(volume(t, restored)-1000) > 1e-9 {
		t.Fatal(handled, e)
	}
	for _, tc := range []struct {
		op   Operation
		want float64
	}{{Union, 1500}, {Intersection, 500}, {Difference, 500}} {
		r, ok, e := CombineAxisAligned(ctx, a, box(t, 5, 0, 0, 15, 10, 10), tc.op, Options{})
		if !ok || e != nil || math.Abs(volume(t, r)-tc.want) > 1e-9 {
			t.Fatal(tc, ok, e)
		}
	}
	empty, ok, e := CombineAxisAligned(ctx, a, a, Difference, Options{})
	if !ok || e != nil || len(empty.Faces()) != 0 {
		t.Fatal(ok, e)
	}
	if _, _, e = CombineAxisAligned(ctx, a, box(t, 10, 10, 0, 20, 20, 10), Union, Options{}); e == nil {
		t.Fatal("accepted edge-only nonmanifold contact")
	}
	// Non-axis surfaces are explicitly outside this path, not approximated.
	skew, e := brep.PrismWithHoles([]geom.Vec3{{}, {X: 4}, {X: 1, Y: 3}}, nil, geom.Vec3{Z: 5})
	if e != nil {
		t.Fatal(e)
	}
	if _, handled, e = CombineAxisAligned(ctx, skew, a, Union, Options{}); handled || e != nil {
		t.Fatal("skew domain", handled, e)
	}
}
func TestAxisAlignedBudgetsAndCumulativeRetry(t *testing.T) {
	ctx := context.Background()
	a := box(t, -5, -5, 0, 5, 5, 10)
	b := box(t, -2.5, -2.5, 0, 2.5, 2.5, 12)
	for _, o := range []Options{{MaxWork: 1}, {MaxFragments: 1}, {MaxDepth: 1}, {MaxVertices: 1}, {MaxOutputTriangles: 1}, {MaxBits: 1}, {MaxInputTriangles: 1}, {MaxCollisionPairs: 1}} {
		if _, _, e := CombineAxisAligned(ctx, a, b, Union, o); !errors.Is(e, ErrLimit) {
			t.Fatal(o, e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, e := CombineAxisAligned(cancelled, a, b, Union, Options{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	joined, e := Combine(ctx, a, b, Union, Options{})
	if e != nil {
		t.Fatal(e)
	}
	var sources []Source
	var usage [2]int
	if _, e = combineMeasured(ctx, joined, b, Difference, Options{}, &sources, nil, &usage); !errors.Is(e, ErrPrecision) {
		t.Fatal(e)
	}
	if _, e = Combine(ctx, joined, b, Difference, Options{MaxWork: usage[0]}); !errors.Is(e, ErrLimit) {
		t.Fatal("retry reset work budget", usage, e)
	}
	if _, _, e = combineAxisAlignedBudget(ctx, a, b, Union, Options{MaxFragments: 10}, [2]int{0, 10}); !errors.Is(e, ErrLimit) {
		t.Fatal("retry reset fragment budget", e)
	}
}
