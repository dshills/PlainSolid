package loft

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func subdivisionRings() ([]geom.Vec3, []geom.Vec3) {
	return []geom.Vec3{{}, {X: 1}, {X: 3}, {X: 3, Y: 2}, {Y: 2}}, []geom.Vec3{{Z: 2}, {X: 3, Z: 2}, {X: 4.5, Z: 2}, {X: 6, Z: 2}, {X: 6, Y: 2, Z: 2}, {Y: 2, Z: 2}}
}
func TestSubdividedExactOwnership(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		a, b := subdivisionRings()
		if reverse {
			for _, r := range [][]geom.Vec3{a, b} {
				for i, j := 1, len(r)-1; i < j; i, j = i+1, j-1 {
					r[i], r[j] = r[j], r[i]
				}
			}
		}
		r, e := PlanarSubdivided(context.Background(), a, b, Options{})
		if e != nil {
			t.Fatal(e)
		}
		if r.ExactVolume.Cmp(big.NewRat(18, 1)) != 0 || len(r.Solid.Vertices()) != 8 {
			t.Fatal(r.ExactVolume)
		}
		for k, ring := range [][]geom.Vec3{a, b} {
			for i, p := range ring {
				v := r.Vertices[k][i]
				edge := r.Solid.Edges()[v.Edge]
				start, end := r.Solid.Vertices()[edge.Start], r.Solid.Vertices()[edge.End]
				if v.Reversed {
					start, end = end, start
				}
				exact := rat(start)
				delta := diff(rat(end), exact)
				for axis := 0; axis < 3; axis++ {
					got := new(big.Rat).Add(exact[axis], mul(delta[axis], v.Parameter))
					if got.Cmp(rat(p)[axis]) != 0 {
						t.Fatal("source point mismatch", k, i)
					}
				}
			}
		}
		if !reverse {
			if r.Segments[0][0].End.Cmp(big.NewRat(1, 3)) != 0 || r.Segments[1][0].End.Cmp(big.NewRat(1, 2)) != 0 || r.Segments[1][1].End.Cmp(big.NewRat(3, 4)) != 0 {
				t.Fatal("intervals")
			}
			if r.Vertices[0][1].NativeVertex != -1 {
				t.Fatal("invented breakpoint")
			}
		}
	}
}
func TestSubdividedRejects(t *testing.T) {
	a, b := subdivisionRings()
	a[1].X = 4
	if _, e := PlanarSubdivided(context.Background(), a, b, Options{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	a, b = subdivisionRings()
	if _, e := PlanarSubdivided(context.Background(), a, b, Options{MaxVertices: 5}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
	a = append(a[1:], a[0])
	if _, e := PlanarSubdivided(context.Background(), a, b, Options{}); !errors.Is(e, ErrUnsupported) {
		t.Fatal(e)
	}
}
