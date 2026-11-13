package support

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"testing"
)

func box(t *testing.T) brep.Solid {
	t.Helper()
	s, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 3, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestExactConvexSupports(t *testing.T) {
	s := box(t)
	a := geom.Identity()
	a[2][0] = .5
	s, e := s.Transformed(a)
	if e != nil {
		t.Fatal(e)
	}
	r, e := Certify(context.Background(), s, Options{Convex: true})
	if e != nil {
		t.Fatal(e)
	}
	if !r.Convex || len(r.Planes) != 6 {
		t.Fatal(r)
	}
	for _, p := range r.Planes {
		max := new(big.Rat)
		for _, x := range p.Normal {
			v := new(big.Rat).Abs(x)
			if v.Cmp(max) > 0 {
				max = v
			}
		}
		if max.Cmp(big.NewRat(1, 1)) != 0 {
			t.Fatal("normalization")
		}
		for _, v := range s.Vertices() {
			if dot(p.Normal, vec(v)).Cmp(p.Offset) > 0 {
				t.Fatal("not support")
			}
		}
	}
}
func TestConcaveFaceOrientationAndWitness(t *testing.T) {
	s, e := brep.Prism([]geom.Vec3{{X: 1, Y: 1}, {X: 1, Y: 2}, {Y: 2}, {}, {X: 2}, {X: 2, Y: 1}}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	r, e := Certify(context.Background(), s, Options{})
	if e != nil {
		t.Fatal(e)
	}
	if r.Planes[0].Normal[2].Sign() != -1 || r.Planes[1].Normal[2].Sign() != 1 {
		t.Fatal("reflex corner reversed normal")
	}
	_, e = Certify(context.Background(), s, Options{Convex: true})
	var v *Violation
	if !errors.Is(e, ErrNonconvex) || !errors.As(e, &v) || v.Residual.Sign() <= 0 {
		t.Fatal(e)
	}
}
func TestNonplanarityAndLimits(t *testing.T) {
	s := box(t)
	vs := s.Vertices()
	vs[0].Z = 1e-13
	warped, e := brep.NewSolid(vs, s.Edges(), s.Faces())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Certify(context.Background(), warped, Options{}); !errors.Is(e, ErrNonplanar) {
		t.Fatal(e)
	}
	for _, o := range []Options{{MaxFaces: 1}, {MaxVertices: 1}, {MaxWork: 1}, {MaxBits: 1}} {
		if _, e := Certify(context.Background(), s, o); !errors.Is(e, ErrLimit) {
			t.Fatal(e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Certify(ctx, s, Options{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
