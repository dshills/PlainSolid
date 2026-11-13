package section

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"testing"
)

func TestExactSectionGraphAncestry(t *testing.T) {
	s, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewPlane(geom.Vec3{Z: 1}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	r, e := PlanarWithAncestry(context.Background(), s, p, 0)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Loops) != 1 || len(r.Loops[0].Points) != 4 || len(r.Corners) != 8 || len(r.Segments) != 8 {
		t.Fatal("section graph")
	}
	original := 0
	for _, c := range r.Corners {
		if c.Point[2].Cmp(big.NewRat(1, 1)) != 0 {
			t.Fatal("plane")
		}
		if c.Edge >= 0 {
			original++
			edge := s.Edges()[c.Edge]
			a, b := s.Vertices()[edge.Start], s.Vertices()[edge.End]
			want := big.NewRat(1, 3)
			if a.Z > b.Z {
				want = big.NewRat(2, 3)
			}
			if c.EdgeParameter.Cmp(want) != 0 {
				t.Fatal(c.EdgeParameter, want)
			}
		}
	}
	if original != 4 {
		t.Fatal("original edges")
	}
	in, out := make([]int, len(r.Corners)), make([]int, len(r.Corners))
	for _, segment := range r.Segments {
		if segment.Face != s.TriangleFaces()[segment.Triangle] {
			t.Fatal("face")
		}
		out[segment.Start]++
		in[segment.End]++
	}
	for i := range in {
		if in[i] != 1 || out[i] != 1 {
			t.Fatal("nonconforming graph")
		}
	}
}
func TestSectionGraphContactAndLimit(t *testing.T) {
	s, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	p, _ := geom.NewPlane(geom.Vec3{}, geom.Vec3{Z: 1})
	if _, e := PlanarWithAncestry(context.Background(), s, p, 0); !errors.Is(e, ErrContact) {
		t.Fatal(e)
	}
	p, _ = geom.NewPlane(geom.Vec3{Z: .5}, geom.Vec3{Z: 1})
	if _, e := PlanarWithAncestry(context.Background(), s, p, 1); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
}
