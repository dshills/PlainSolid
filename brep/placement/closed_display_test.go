package placement

import (
	"context"
	"math"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
	"reflect"
	"testing"
)

func displayTestPose(t *testing.T, translation *big.Rat) RationalRigid {
	t.Helper()
	r := big.NewRat
	m := [3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), translation}, {r(0, 1), r(1, 1), r(0, 1), r(-2, 5)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 8)}}
	p, e := NewRationalRigid(m, 4096)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func displayJoin(a, b mesh.Mesh, reverse bool) mesh.Mesh {
	base := len(a.Vertices)
	a.Vertices = append(a.Vertices, b.Vertices...)
	for _, tr := range b.Triangles {
		if reverse {
			tr[1], tr[2] = tr[2], tr[1]
		}
		for i := range tr {
			tr[i] += base
		}
		a.Triangles = append(a.Triangles, tr)
	}
	return a
}
func TestClosedDisplayPlacement(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	outer, _ := mesh.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 4})
	inner, _ := mesh.Box(geom.Vec3{X: 1, Y: 1, Z: 1}, geom.Vec3{X: 3, Y: 3, Z: 3})
	m := displayJoin(outer, inner, true)
	ids := make([]int, len(m.Triangles))
	for i := range ids {
		ids[i] = 17
		if i >= 12 {
			ids[i] = 42
		}
	}
	source, e := PrepareClosedDisplay(ctx, m, ids, ClosedDisplayOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m.Vertices[0] = geom.Vec3{X: 999}
	ids[0] = 999
	pose := displayTestPose(t, r(1, 10))
	placed, e := source.Place(ctx, pose, ClosedDisplayOptions{})
	if e != nil {
		t.Fatal(e)
	}
	sh := placed.Shells()
	if len(sh) != 2 || sh[0].Parent != -1 || sh[1].Parent != 0 || sh[1].Depth != 1 || sh[0].SignedVolume.Sign() != 1 || sh[1].SignedVolume.Sign() != -1 {
		t.Fatal("nesting", sh)
	}
	if placed.Faces()[0] != 17 || placed.Faces()[12] != 42 || !reflect.DeepEqual(source.Mesh().Triangles, placed.Mesh().Triangles) {
		t.Fatal("ancestry")
	}
	if math.Abs(placed.Mesh().SignedVolume()-56) > 1e-12 {
		t.Fatal("independent assembly volume")
	}
	original := source.Mesh()
	image := placed.Mesh()
	round := placed.RoundingUpper()
	if round.Sign() <= 0 {
		t.Fatal("nonbinary rounding")
	}
	for i, v := range original.Vertices {
		p, e := pose.TransformPoint(ctx, [3]*big.Rat{new(big.Rat).SetFloat64(v.X), new(big.Rat).SetFloat64(v.Y), new(big.Rat).SetFloat64(v.Z)}, false, 4096)
		if e != nil {
			t.Fatal(e)
		}
		f := image.Vertices[i]
		d := new(big.Rat)
		for j, x := range []float64{f.X, f.Y, f.Z} {
			expected, _ := p[j].Float64()
			if x != expected {
				t.Fatal("changed rounded display coordinate")
			}
			delta := new(big.Rat).Sub(p[j], new(big.Rat).SetFloat64(x))
			d.Add(d, delta.Abs(delta))
		}
		if d.Cmp(round) > 0 {
			t.Fatal("rounding enclosure")
		}
	}
	image.Vertices[0] = geom.Vec3{X: 999}
	round.SetInt64(999)
	sh[0].SignedVolume.SetInt64(999)
	if placed.Mesh().Vertices[0].X == 999 || placed.RoundingUpper().Cmp(r(999, 1)) == 0 || placed.Shells()[0].SignedVolume.Cmp(r(999, 1)) == 0 {
		t.Fatal("mutable snapshot")
	}
	// Original octahedron lies between concentric radii 1/sqrt(3) and 1. Since
	// 1/sqrt(3)>1/2, radial correspondence proves a two-sided sphere error <=1/2.
	oct := mesh.Mesh{Vertices: []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {Y: 1}, {X: -1}, {Y: -1}}}
	for i := 0; i < 4; i++ {
		a, b := 2+i, 2+(i+1)%4
		oct.Triangles = append(oct.Triangles, [3]int{0, a, b}, [3]int{1, b, a})
	}
	sphere, e := PrepareClosedDisplay(ctx, oct, nil, ClosedDisplayOptions{})
	if e != nil {
		t.Fatal(e)
	}
	moved, e := sphere.Place(ctx, pose, ClosedDisplayOptions{})
	if e != nil {
		t.Fatal(e)
	}
	bound, e := moved.InheritedDistanceUpper(r(1, 2))
	if e != nil || bound.Cmp(new(big.Rat).Add(r(1, 2), moved.RoundingUpper())) != 0 {
		t.Fatal("conditional inherited proof", e)
	}
	// Independent sphere point (3/5,0,4/5) and its octahedral radial foot.
	exact, e := pose.TransformPoint(ctx, [3]*big.Rat{r(3, 5), r(0, 1), r(4, 5)}, false, 4096)
	if e != nil {
		t.Fatal(e)
	}
	verts := moved.Mesh().Vertices
	squared := new(big.Rat)
	for j := 0; j < 3; j++ {
		u := []float64{verts[2].X, verts[2].Y, verts[2].Z}[j]
		v := []float64{verts[0].X, verts[0].Y, verts[0].Z}[j]
		foot := new(big.Rat).Add(new(big.Rat).Mul(r(3, 7), new(big.Rat).SetFloat64(u)), new(big.Rat).Mul(r(4, 7), new(big.Rat).SetFloat64(v)))
		delta := new(big.Rat).Sub(exact[j], foot)
		squared.Add(squared, new(big.Rat).Mul(delta, delta))
	}
	if squared.Cmp(new(big.Rat).Mul(bound, bound)) > 0 {
		t.Fatal("analytic sphere witness")
	}
}
func TestClosedDisplayPlacementRejects(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	m, _ := mesh.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	source, e := PrepareClosedDisplay(ctx, m, nil, ClosedDisplayOptions{})
	if e != nil {
		t.Fatal(e)
	}
	collapse := displayTestPose(t, new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 60)))
	if got, e := source.Place(ctx, collapse, ClosedDisplayOptions{}); e == nil || got != nil {
		t.Fatal("rounded collapse accepted")
	}
	// Two valid separate shells acquire exact shared display vertices after
	// translation; vertex positions are not welded and contact must be rejected.
	gap := math.Ldexp(1, -40)
	b, _ := mesh.Box(geom.Vec3{X: 1 + gap}, geom.Vec3{X: 2 + gap, Y: 1, Z: 1})
	pair := displayJoin(m, b, false)
	separate, e := PrepareClosedDisplay(ctx, pair, nil, ClosedDisplayOptions{})
	if e != nil {
		t.Fatal(e)
	}
	translate, e := NewRationalRigid([3][4]*big.Rat{{r(1, 1), r(0, 1), r(0, 1), r(1<<20, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(0, 1), r(0, 1), r(1, 1), r(0, 1)}}, 4096)
	if e != nil {
		t.Fatal(e)
	}
	if got, e := separate.Place(ctx, translate, ClosedDisplayOptions{}); e == nil || got != nil {
		t.Fatal("rounded inter-component contact accepted")
	}
	pose := displayTestPose(t, r(1, 10))
	for _, o := range []ClosedDisplayOptions{{Rounding: MeshRoundingOptions{MaxVertices: 7}}, {Rounding: MeshRoundingOptions{MaxTriangles: 11}}, {Validation: collision.LocationOptions{MaxCollisionPairs: 1}}, {Validation: collision.LocationOptions{MaxWork: 1}}} {
		if got, e := source.Place(ctx, pose, o); e == nil || got != nil {
			t.Fatal("atomic quota")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got, e := source.Place(canceled, pose, ClosedDisplayOptions{}); e == nil || got != nil {
		t.Fatal("cancel")
	}
	inner, _ := mesh.Box(geom.Vec3{X: .25, Y: .25, Z: .25}, geom.Vec3{X: .75, Y: .75, Z: .75})
	wrong := displayJoin(m, inner, false)
	if got, e := PrepareClosedDisplay(ctx, wrong, nil, ClosedDisplayOptions{}); e == nil || got != nil {
		t.Fatal("positive nested shell")
	}
}
