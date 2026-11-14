package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/collision"
	"reflect"
	"testing"
)

func TestRepeatedStoredMixedDisplayUsesOriginalProof(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	p := prepareAssemblySource(t, false, .04)
	first := assemblyOccurrence(t, p, r(1, 10), r(-2, 5), r(3, 8), true)
	matrix, e := first.Display().Pose().Matrix()
	if e != nil {
		t.Fatal(e)
	}
	var inverse [3][4]*big.Rat
	for i := 0; i < 3; i++ {
		inverse[i][3] = new(big.Rat)
		for j := 0; j < 3; j++ {
			inverse[i][j] = new(big.Rat).Set(matrix[j][i])
			inverse[i][3].Sub(inverse[i][3], new(big.Rat).Mul(matrix[j][i], matrix[j][3]))
		}
	}
	pose, e := placement.NewRationalRigid(inverse, 4096)
	if e != nil {
		t.Fatal(e)
	}
	world, e := first.Then(ctx, pose, placement.ClosedDisplayOptions{Validation: collision.LocationOptions{MaxCollisionPairs: 1000000}})
	if e != nil {
		t.Fatal(e)
	}
	if world.Display().RoundingUpper().Sign() != 0 || world.SquaredDistance().Cmp(p.SourceSquaredDistance()) != 0 || world.SquaredDistance().Cmp(first.SquaredDistance()) >= 0 {
		t.Fatal("source-derived bound was accumulated")
	}
	if !reflect.DeepEqual(world.Ancestry(), first.Ancestry()) || !reflect.DeepEqual(world.Display().Mesh(), p.display.Mesh()) {
		t.Fatal("original model indexing")
	}
	if _, e := world.Source().RationalVertices(ctx); e != nil {
		t.Fatal("lost source", e)
	}
	if denied, e := first.Then(ctx, pose, placement.ClosedDisplayOptions{Validation: collision.LocationOptions{MaxCollisionPairs: 1}}); e == nil || denied != nil {
		t.Fatal("fresh final proof skipped")
	}
}
