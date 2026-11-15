package heal

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
	"slices"
	"testing"
)

func shortSeamFixture(t *testing.T, top [][]geom.Vec3, a, b geom.Vec3) (brep.Solid, int) {
	t.Helper()
	f := slices.Delete(cubeFaces(), 1, 2)
	for _, loop := range top {
		f = append(f, Face{Outer: loop})
	}
	s, _, err := Sew(f, Options{})
	if err != nil {
		t.Fatal(err)
	}
	v := s.Vertices()
	for i, e := range s.Edges() {
		if v[e.Start] == a && v[e.End] == b {
			return s, i
		}
	}
	t.Fatal("missing target edge")
	return brep.Solid{}, -1
}

func TestCollapseShortEdgePreservesMaterialAndIndices(t *testing.T) {
	ctx := context.Background()
	a, b := geom.Vec3{Z: 1}, geom.Vec3{X: 1, Y: 1, Z: 1}
	c, d := geom.Vec3{X: 1, Z: 1}, geom.Vec3{Y: 1, Z: 1}
	p, q := geom.Vec3{X: 3. / 8, Y: .5, Z: 1}, geom.Vec3{X: 25. / 64, Y: .5, Z: 1}
	s, id := shortSeamFixture(t, [][]geom.Vec3{{a, c, b, q, p}, {a, p, q, b, d}}, p, q)
	for _, keepEnd := range []bool{false, true} {
		r, err := CollapseShortEdge(ctx, s, id, ShortEdgeOptions{MaxDistance: 1. / 64, KeepEnd: keepEnd})
		if err != nil {
			t.Fatal(err)
		}
		if r.DistanceSquared.Cmp(big.NewRat(1, 4096)) != 0 || r.EdgeMap[id] != -1 || !slices.Equal(r.AffectedFaces, []int{5, 6}) {
			t.Fatalf("unexpected repair certificate: %+v", r)
		}
		want := p
		if keepEnd {
			want = q
		}
		v := r.Solid.Vertices()
		if r.VertexMap[r.RemovedVertex] != r.VertexMap[r.KeptVertex] || v[r.VertexMap[r.KeptVertex]] != want {
			t.Fatal("wrong retained endpoint")
		}
		for i, point := range s.Vertices() {
			if i != r.RemovedVertex && v[r.VertexMap[i]] != point {
				t.Fatalf("vertex %d moved", i)
			}
		}
		edges := r.Solid.Edges()
		for i, old := range s.Edges() {
			if i != id && edges[r.EdgeMap[i]] != (brep.Edge{Start: r.VertexMap[old.Start], End: r.VertexMap[old.End]}) {
				t.Fatalf("source edge %d ancestry changed", i)
			}
		}
		if r.Solid.Topology() != (brep.Topology{Vertices: 9, Edges: 14, Faces: 7, Shells: 1, EulerCharacteristic: 2}) {
			t.Fatal(r.Solid.Topology())
		}
		v1, err := r.Solid.MassProperties()
		if err != nil {
			t.Fatal(err)
		}
		near(t, v1.Volume, 1, 1e-12)
		near(t, v1.SurfaceArea, 6, 1e-12)
	}
	for _, o := range []ShortEdgeOptions{{MaxDistance: math.Nextafter(1./64, 0)}, {MaxDistance: 1. / 64, Validation: collision.LocationOptions{MaxWork: 1}}} {
		r, err := CollapseShortEdge(ctx, s, id, o)
		if (!errors.Is(err, ErrUnsafeCollapse) && !errors.Is(err, ErrLimit)) || r.VertexMap != nil || len(r.Solid.Faces()) != 0 {
			t.Fatalf("admitted without a distance/work proof: %+v, %v", r, err)
		}
	}
}

func TestCollapseShortEdgeRejectsCreasesAndErasedFaces(t *testing.T) {
	ctx := context.Background()
	s, _, err := Sew(cubeFaces(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, keepEnd := range []bool{false, true} {
		if _, err := CollapseShortEdge(ctx, s, 0, ShortEdgeOptions{MaxDistance: 1, KeepEnd: keepEnd}); !errors.Is(err, ErrUnsafeCollapse) {
			t.Fatal("crease contraction accepted", err)
		}
	}
	a, b := geom.Vec3{Z: 1}, geom.Vec3{X: 1, Y: 1, Z: 1}
	c, d := geom.Vec3{X: 1, Z: 1}, geom.Vec3{Y: 1, Z: 1}
	p := geom.Vec3{X: 1. / 128, Y: 1. / 64, Z: 1}
	s, id := shortSeamFixture(t, [][]geom.Vec3{{a, c, p}, {c, b, p}, {b, d, p}, {d, a, p}}, a, p)
	if r, err := CollapseShortEdge(ctx, s, id, ShortEdgeOptions{MaxDistance: 1. / 32}); !errors.Is(err, ErrUnsafeCollapse) || len(r.AffectedFaces) != 0 {
		t.Fatal("erased face unexpectedly accepted", err)
	}
}
