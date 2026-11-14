package placement

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/mesh"
	"reflect"
	"testing"
)

func inverseDisplayPose(t *testing.T, p RationalRigid) RationalRigid {
	t.Helper()
	m, e := p.Matrix()
	if e != nil {
		t.Fatal(e)
	}
	var inverse [3][4]*big.Rat
	for i := 0; i < 3; i++ {
		inverse[i][3] = new(big.Rat)
		for j := 0; j < 3; j++ {
			inverse[i][j] = new(big.Rat).Set(m[j][i])
			inverse[i][3].Sub(inverse[i][3], new(big.Rat).Mul(m[j][i], m[j][3]))
		}
	}
	q, e := NewRationalRigid(inverse, 4096)
	if e != nil {
		t.Fatal(e)
	}
	return q
}
func TestRepeatedClosedDisplayUsesOriginal(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	m, _ := mesh.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 4})
	inner, _ := mesh.Box(geom.Vec3{X: 1, Y: 1, Z: 1}, geom.Vec3{X: 3, Y: 3, Z: 3})
	m = displayJoin(m, inner, true)
	prepared, e := PrepareClosedDisplay(ctx, m, nil, ClosedDisplayOptions{})
	if e != nil {
		t.Fatal(e)
	}
	pose := displayTestPose(t, r(1, 10))
	first, e := prepared.Place(ctx, pose, ClosedDisplayOptions{})
	if e != nil {
		t.Fatal(e)
	}
	inverse := inverseDisplayPose(t, pose)
	restored, e := first.Then(ctx, inverse, ClosedDisplayOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(restored.Shells()) != 2 || restored.Shells()[1].Parent != 0 || restored.Shells()[1].Depth != 1 || !reflect.DeepEqual(restored.Mesh(), m) || restored.RoundingUpper().Sign() != 0 {
		t.Fatal("repeated rounding accumulated")
	}
	naive, e := inverse.RoundMesh(ctx, first.Mesh(), MeshRoundingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if reflect.DeepEqual(naive.Mesh, m) || new(big.Rat).Add(first.RoundingUpper(), naive.RoundingUpper).Sign() <= 0 {
		t.Fatal("fixture lacks actual sequential rounding")
	}
	old, _ := first.InheritedDistanceUpper(r(1, 100))
	old.Add(old, naive.RoundingUpper)
	fresh, _ := restored.InheritedDistanceUpper(r(1, 100))
	if fresh.Cmp(r(1, 100)) != 0 || fresh.Cmp(old) >= 0 {
		t.Fatal("strictly tighter inherited bound")
	}
	next, e := NewRationalRigid([3][4]*big.Rat{{r(0, 1), r(-1, 1), r(0, 1), r(-7, 10)}, {r(1, 1), r(0, 1), r(0, 1), r(1, 3)}, {r(0, 1), r(0, 1), r(1, 1), r(1, 5)}}, 4096)
	if e != nil {
		t.Fatal(e)
	}
	combined, e := pose.Then(ctx, next, 4096)
	if e != nil {
		t.Fatal(e)
	}
	point := [3]*big.Rat{r(7, 3), r(-1, 7), r(9, 11)}
	a, _ := pose.TransformPoint(ctx, point, false, 4096)
	b, _ := next.TransformPoint(ctx, a, false, 4096)
	c, _ := combined.TransformPoint(ctx, point, false, 4096)
	for j := 0; j < 3; j++ {
		if b[j].Cmp(c[j]) != 0 {
			t.Fatal("composition order")
		}
	}
	chain, e := first.Then(ctx, next, ClosedDisplayOptions{})
	if e != nil {
		t.Fatal(e)
	}
	direct, e := prepared.Place(ctx, combined, ClosedDisplayOptions{})
	if e != nil || !reflect.DeepEqual(chain.Mesh(), direct.Mesh()) || chain.RoundingUpper().Cmp(direct.RoundingUpper()) != 0 {
		t.Fatal("one final rounding", e)
	}
	huge := displayTestPose(t, new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 100)))
	if q, e := pose.Then(ctx, huge, 64); e == nil {
		t.Fatal("composition quota", q)
	}
	if q, e := first.Then(ctx, huge, ClosedDisplayOptions{}); e == nil || q != nil {
		t.Fatal("final collapse")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := first.Then(canceled, inverse, ClosedDisplayOptions{}); e == nil || q != nil {
		t.Fatal("cancel")
	}
}
