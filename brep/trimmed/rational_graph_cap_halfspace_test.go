package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestRationalGraphCapHalfspace(t *testing.T) {
	ctx := context.Background()
	stock, _ := polynomialClipStock(t)
	z := 3. / 16
	section, e := stock.SectionRationalGraphSplineShell(ctx, [3]geom.Vec3{{Z: z}, {X: 1, Z: z}, {Y: 1, Z: z}}, RationalGraphShellSectionOptions{Regions: &RationalGraphShellRegionOptions{}})
	if e != nil {
		t.Fatal(e)
	}
	for _, keep := range []int{-1, 1} {
		cutter, e := polynomialPlaneFromPoints([3]geom.Vec3{{X: .5}, {X: .5, Y: 1}, {X: .5, Z: 1}})
		if e != nil {
			t.Fatal(e)
		}
		clipped, cuts, e := section.Regions.intersectCapHalfspace(ctx, cutter, keep, RationalGraphShellRegionOptions{})
		if e != nil || cuts[0].Point[0].Cmp(big.NewRat(1, 2)) != 0 || cuts[1].Point[0].Cmp(big.NewRat(1, 2)) != 0 {
			t.Fatal("missing two certified junctions", cuts, e)
		}
		faces := map[int]*big.Rat{2: new(big.Rat), 4: big.NewRat(11, 16)}
		for _, cut := range cuts {
			if y := faces[cut.Source.Face]; y == nil || cut.Point[1].Cmp(y) != 0 || cut.Point[2].Cmp(big.NewRat(3, 16)) != 0 || cut.Parameter.Cmp(big.NewRat(1, 2)) != 0 {
				t.Fatal("exact stock-cap junction", cut)
			}
		}
		for _, x := range []int64{1, 2, 3} {
			want := trimvalidate.Outside
			if x == 2 {
				want = trimvalidate.Boundary
			} else if int(x-2) == keep {
				want = trimvalidate.Inside
			}
			p := [3]*big.Rat{big.NewRat(x, 4), big.NewRat(1, 8), big.NewRat(3, 16)}
			loc, report, e := clipped.LocateRational(ctx, p)
			if e != nil || loc != want || report.Status != trimvalidate.Certified {
				t.Fatal("trim class across new chord", x, keep, loc, report, e)
			}
		}
		if out, cuts, e := section.Regions.intersectCapHalfspace(ctx, cutter, keep, RationalGraphShellRegionOptions{Topology: trimvalidate.Options{MaxWork: 1}}); !errors.Is(e, geom.ErrConvergence) || out != nil || cuts[0].Parameter != nil {
			t.Fatal("exhausted proof must be atomic", out, cuts, e)
		}
	}
	// y=3/4 only touches the exact original section vertex x=0.
	tangent, _ := polynomialPlaneFromPoints([3]geom.Vec3{{Y: .75}, {Y: .75, Z: 1}, {X: 1, Y: .75}})
	if out, cuts, e := section.Regions.intersectCapHalfspace(ctx, tangent, -1, RationalGraphShellRegionOptions{}); !errors.Is(e, geom.ErrConvergence) || out != nil || cuts[0].Parameter != nil {
		t.Fatal("vertex touch cannot certify two regular interior crossings", out, cuts, e)
	}
}
