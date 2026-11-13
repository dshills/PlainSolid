package draft

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func hingeFixture(t *testing.T) (brep.Solid, []FaceHinge) {
	t.Helper()
	s, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 3, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	vs, es := s.Vertices(), s.Edges()
	var cs []FaceHinge
	for fi, f := range s.Faces() {
		x := vs[es[f.Boundary[0].Edge].Start].X
		fixed := true
		for _, u := range f.Boundary {
			if vs[es[u.Edge].Start].X != x || vs[es[u.Edge].End].X != x {
				fixed = false
			}
		}
		if !fixed {
			continue
		}
		for _, u := range f.Boundary {
			ed := es[u.Edge]
			a, b := vs[ed.Start], vs[ed.End]
			if a.Z != 0 || b.Z != 0 {
				continue
			}
			tan := .25
			if x == 4 {
				tan = -.125
			}
			if b.Y < a.Y {
				tan = -tan
			}
			cs = append(cs, FaceHinge{fi, u.Edge, tan})
		}
	}
	if len(cs) != 2 {
		t.Fatal(cs)
	}
	return s, cs
}
func TestHingeDraftIndependent(t *testing.T) {
	s, cs := hingeFixture(t)
	r, e := HingeDraft(context.Background(), s, cs, Options{})
	if e != nil {
		t.Fatal(e)
	}
	if r.ExactVolume.Cmp(big.NewRat(1136, 105)) != 0 {
		t.Fatal(r.ExactVolume)
	}
	m, e := r.Solid.MassProperties()
	if e != nil || math.Abs(m.Volume-1136.0/105) > 1e-10 {
		t.Fatal(m, e)
	}
	for _, h := range r.Hinges {
		if h.OutputFace < 0 || h.Edge < 0 {
			t.Fatal(h)
		}
		d := diff(h.Endpoints[1], h.Endpoints[0])
		for i := 0; i < 3; i++ {
			if dot(point(h.Rotation[i]), d).Cmp(d[i]) != 0 {
				t.Fatal("axis not fixed")
			}
			for j := 0; j < 3; j++ {
				want := int64(0)
				if i == j {
					want = 1
				}
				if dot(point(h.Rotation[i]), point(h.Rotation[j])).Cmp(big.NewRat(want, 1)) != 0 {
					t.Fatal("rotation not orthogonal")
				}
			}
		}
	}
}
func TestHingeDraftIdentityAndFailures(t *testing.T) {
	s, cs := hingeFixture(t)
	zero := append([]FaceHinge(nil), cs...)
	for i := range zero {
		zero[i].HalfAngleTangent = 0
	}
	a, e := HingeDraft(context.Background(), s, zero, Options{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := Apply(context.Background(), s, nil, Options{})
	if e != nil || !reflect.DeepEqual(a.Result, b) {
		t.Fatal("identity", e)
	}
	for _, bad := range [][]FaceHinge{{cs[0], cs[0]}, {{Face: cs[0].Face, Edge: cs[1].Edge}}, {{Face: cs[0].Face, Edge: cs[0].Edge, HalfAngleTangent: math.NaN()}}} {
		r, e := HingeDraft(context.Background(), s, bad, Options{})
		if !errors.Is(e, ErrInvalid) || len(r.Solid.Faces()) != 0 {
			t.Fatal(e)
		}
	}
	if _, e := HingeDraft(context.Background(), s, cs, Options{MaxWork: 1}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := HingeDraft(ctx, s, cs, Options{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	// A shear makes the selected Y axes length sqrt(2), outside exact rational rotation scope.
	ss, e := s.Transformed(geom.Affine{{1, 1, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e := HingeDraft(context.Background(), ss, cs, Options{}); !errors.Is(e, ErrUnsupported) {
		t.Fatal(e)
	}
}

func TestHingeDraftStoredAxisReversal(t *testing.T) {
	s, cs := hingeFixture(t)
	a, e := HingeDraft(context.Background(), s, cs, Options{})
	if e != nil {
		t.Fatal(e)
	}
	es, fs := s.Edges(), s.Faces()
	id := cs[0].Edge
	es[id].Start, es[id].End = es[id].End, es[id].Start
	for i := range fs {
		for j := range fs[i].Boundary {
			if fs[i].Boundary[j].Edge == id {
				fs[i].Boundary[j].Reversed = !fs[i].Boundary[j].Reversed
			}
		}
	}
	reversed, e := brep.NewSolid(s.Vertices(), es, fs)
	if e != nil {
		t.Fatal(e)
	}
	cs[0].HalfAngleTangent = -cs[0].HalfAngleTangent
	b, e := HingeDraft(context.Background(), reversed, cs, Options{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(a.Result, b.Result) || !reflect.DeepEqual(a.Hinges[0].Rotation, b.Hinges[0].Rotation) {
		t.Fatal("reversal changed result")
	}
	if a.Hinges[0].StartVertex != b.Hinges[0].EndVertex {
		t.Fatal("axis ancestry")
	}
}
