package placement

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestTorusCylinderOccurrence(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	pose, e := NewRationalRigid([3][4]*big.Rat{{r(3, 13), r(-12, 13), r(4, 13), r(1, 7)}, {r(36, 65), r(5, 13), r(48, 65), r(-2, 9)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 11)}}, 4096)
	if e != nil {
		t.Fatal(e)
	}
	for _, op := range []geom.TorusCylinderOperation{geom.TorusCylinderCommon, geom.TorusCylinderDifference, geom.TorusCylinderUnion} {
		source, _ := geom.NewTorusCylinderMaterial(5, 2, 5, -3, 4, op)
		occ, e := PlaceTorusCylinder(ctx, source, pose)
		if e != nil {
			t.Fatal(e)
		}
		local := [3]*big.Rat{r(5, 1), r(0, 1), r(2, 1)}
		world, _ := pose.TransformPoint(ctx, local, false, 4096)
		l, m, e := occ.ClassifyRational(ctx, world, geom.SurfacePatchOptions{})
		if e != nil || l != geom.BoreBoundary || m != 3 {
			t.Fatal(l, m, e)
		}
		n, e := occ.BoundaryNormalsRational(ctx, world, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		want := [3]*big.Rat{r(400, 13), r(960, 13), r(60, 1)}
		for i := range want {
			if n.Directions[0][i].Cmp(want[i]) != 0 {
				t.Fatal("independent world torus direction", n)
			}
		}
		sourceN, _ := source.BoundaryNormalsRational(ctx, local, geom.SurfacePatchOptions{})
		for i := 0; i < 2; i++ {
			back, e := pose.TransformVector(ctx, n.Directions[i], true, 4096)
			if e != nil {
				t.Fatal(e)
			}
			for j := range back {
				if back[j].Cmp(sourceN.Directions[i][j]) != 0 {
					t.Fatal("inverse", n)
				}
			}
		}
		p := [3]*big.Rat{r(0, 1), r(0, 1), r(0, 1)}
		d := [3]*big.Rat{r(1, 1), r(0, 1), r(1, 10)}
		wp, _ := pose.TransformPoint(ctx, p, false, 4096)
		wd, _ := pose.TransformVector(ctx, d, false, 4096)
		options := geom.ToroidLineOptions{ParameterTolerance: 1e-10}
		a, e := occ.ClipRationalLine(ctx, wp, wd, r(-8, 1), r(8, 1), options)
		if e != nil {
			t.Fatal(e)
		}
		b, e := source.ClipRationalLine(ctx, p, d, r(-8, 1), r(8, 1), options)
		if e != nil || !reflect.DeepEqual(a, b) {
			t.Fatal("line t/ownership", e)
		}
		n.Directions[0][0].SetInt64(99)
		again, e := occ.BoundaryNormalsRational(ctx, world, geom.SurfacePatchOptions{})
		if e != nil || again.Directions[0][0].Cmp(want[0]) != 0 {
			t.Fatal("mutation", e)
		}
		c, cancel := context.WithCancel(ctx)
		cancel()
		if v, e := occ.BoundaryNormalsRational(c, world, geom.SurfacePatchOptions{}); e == nil || !reflect.DeepEqual(v, geom.TorusCylinderNormals{}) {
			t.Fatal("cancel")
		}
	}
	if _, e := PlaceTorusCylinder(ctx, geom.TorusCylinderMaterial{}, pose); e == nil {
		t.Fatal("invalid source")
	}
	source, _ := geom.NewTorusCylinderMaterial(5, 2, 5, -3, 4, geom.TorusCylinderUnion)
	if _, e := PlaceTorusCylinder(ctx, source, RationalRigid{}); e == nil {
		t.Fatal("invalid pose")
	}
}
