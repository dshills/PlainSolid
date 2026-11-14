package placement

import (
	"context"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/geom"
	"strconv"
	"strings"
	"testing"
)

func TestEccentricCylinderTorusCavityWorldDistance(t *testing.T) {
	r := big.NewRat
	ctx := context.Background()
	source, _ := geom.NewEccentricCylinderTorusCavityMaterial(5, 2, 10, -3, 4, 1, 0)
	pose, e := NewRationalRigid([3][4]*big.Rat{{r(3, 5), r(0, 1), r(4, 5), r(1, 10)}, {r(0, 1), r(1, 1), r(0, 1), r(-2, 5)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 8)}}, 4096)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := source.BoundBoundaryDistance(ctx, [3]*big.Rat{r(2, 1), r(1, 1), r(1, 1)}, geom.TorusCylinderDistanceOptions{SquaredTolerance: r(1, 100), Arithmetic: geom.SurfacePatchOptions{MaxOperations: 1}}); e == nil {
		t.Fatal("shared budget")
	}
	for _, p := range [][3]*big.Rat{{r(9, 1), r(0, 1), r(0, 1)}, {r(2, 1), r(1, 1), r(1, 1)}, {r(11, 1), r(0, 1), r(5, 1)}, {r(8, 1), r(0, 1), r(0, 1)}} {
		o := geom.TorusCylinderDistanceOptions{SquaredTolerance: r(1, 100)}
		local, e := source.BoundBoundaryDistance(ctx, p, o)
		if e != nil {
			t.Fatal(e)
		}
		if p[0].Cmp(r(9, 1)) == 0 && local.ExactOwners != (geom.TorusCylinderWall|geom.TorusCylinderTorus) {
			t.Fatal("distinct feet tie", local.ExactOwners)
		}
		world, _ := pose.TransformPoint(ctx, p, false, 4096)
		got, e := BoundEccentricCylinderTorusCavityDistance(ctx, source, pose, world, o)
		if e != nil || got.Base.Lower.Cmp(local.Lower) != 0 || got.Base.Upper.Cmp(local.Upper) != 0 || got.Base.PossibleOwners != local.PossibleOwners {
			t.Fatal(got, e)
		}
		if got.Witnesses[0].Origin[0].Cmp(r(7, 10)) != 0 {
			t.Fatal("world")
		}
		if oracle := os.Getenv("PLAINSOLID_OCCT_ECCENTRIC_CAVITY_DISTANCE"); oracle != "" {
			args := []string{}
			for _, q := range p {
				x, _ := q.Float64()
				args = append(args, strconv.FormatFloat(x, 'g', -1, 64))
			}
			raw, e := exec.Command(oracle, args...).CombinedOutput()
			if e != nil {
				t.Fatal(e, string(raw))
			}
			v, e := strconv.ParseFloat(strings.TrimSpace(string(raw)), 64)
			lo, _ := local.Lower.Float64()
			hi, _ := local.Upper.Float64()
			if e != nil || v < lo-1e-7 || v > hi+1e-7 {
				t.Fatal(v, lo, hi, e)
			}
		}
	}
}
