package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/collision"
	"testing"
)

func assemblyOccurrence(t *testing.T, p *PreparedStoredMixedDisplay, x, y, z *big.Rat, rotate bool) *PlacedStoredMixedDisplay {
	t.Helper()
	r := big.NewRat
	m := [3][4]*big.Rat{{r(1, 1), r(0, 1), r(0, 1), x}, {r(0, 1), r(1, 1), r(0, 1), y}, {r(0, 1), r(0, 1), r(1, 1), z}}
	if rotate {
		m[0][0], m[0][2], m[2][0], m[2][2] = r(3, 5), r(4, 5), r(-4, 5), r(3, 5)
	}
	pose, e := placement.NewRationalRigid(m, 4096)
	if e != nil {
		t.Fatal(e)
	}
	out, e := p.Place(context.Background(), pose, placement.ClosedDisplayOptions{Validation: collision.LocationOptions{MaxCollisionPairs: 1000000}})
	if e != nil {
		t.Fatal(e)
	}
	return out
}
