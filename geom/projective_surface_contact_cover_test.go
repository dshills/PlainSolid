package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func projectiveTinyLinearHeightCharts(t *testing.T) (*SurfacePatch, *SurfacePatch) {
	t.Helper()
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	b := projectiveOverlapTestChart(t, false, false)
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	for i, h := range b.net {
		change := new(big.Rat).Sub(h[0], new(big.Rat).Quo(h[3], big.NewRat(2, 1)))
		b.net[i][2] = new(big.Rat).Add(h[2], change.Mul(change, tiny))
	}
	budget := &polynomialBudget{maxOperations: b.options.MaxOperations, maxBits: b.options.MaxCoefficientBits}
	if err := b.computeBounds(budget); err != nil {
		t.Fatal(err)
	}
	return a, b
}

func TestProjectiveSurfaceContactCoverPreservesOriginalArcPieces(t *testing.T) {
	ctx := context.Background()
	a, b := projectiveTinyLinearHeightCharts(t)
	opt := ProjectiveSurfaceContactCoverOptions{}
	r, err := BoundProjectiveSurfaceContacts(ctx, a, b, [2]int{0, 1}, opt)
	if err != nil || !r.Complete || len(r.Isolations) != 4 || len(r.IncompleteCells) != 0 {
		t.Fatal("could not cover exact contact across original nonbinary knot", r, err)
	}
	for index, c := range r.Isolations {
		want := 0
		if index >= 2 {
			want = 1
		}
		if len(c.Arcs) != want || len(c.Tangencies)+len(c.Coplanar) != 0 || !c.Complete {
			t.Fatal("wrong local original contact family", index, c)
		}
		if want == 0 {
			continue
		}
		for i := range 2 {
			end, err := c.Arcs[0].ExactEndpoint(i, ArcExactEndpointOptions{})
			if err != nil || end.Parameters[0].Cmp(big.NewRat(1, 3)) != 0 || end.Point[0].Cmp(big.NewRat(1, 2)) != 0 || end.Point[2].Sign() != 0 {
				t.Fatal("exact owner arc endpoint was replaced by a rounded witness", index, end, err)
			}
		}
	}
	opt.MaxExactOperations = r.ExactOperations - 1
	short, err := BoundProjectiveSurfaceContacts(ctx, a, b, [2]int{0, 1}, opt)
	if !errors.Is(err, ErrConvergence) || short.Complete || !short.Atlas.Complete || len(short.IncompleteCells) == 0 ||
		short.Atlas.Cells[short.IncompleteCells[len(short.IncompleteCells)-1]].Common.V.Upper.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("bounded contact cover discarded an original pending cell", short, err)
	}
}
