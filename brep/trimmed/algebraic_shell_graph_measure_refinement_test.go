//go:build research

package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestGraphWholeMeasureRefinement(t *testing.T) {
	slowtest.Skip(t)
	s := graphMaterialBooleanFixture(t, false, true)
	axes := [2]int{0, 2}
	q, e := s.PrepareGraphMaterialQuery(context.Background(), AlgebraicGraphMaterialOptions{Projection: &axes})
	if e != nil {
		t.Fatal(e)
	}
	schedule := []AlgebraicGraphWindowAreaOptions{{Refinement: AlgebraicGraphWindowRefineOptions{MaxCells: 85, MaxDepth: 3}}, {Refinement: AlgebraicGraphWindowRefineOptions{MaxCells: 341, MaxDepth: 4}}}
	r, e := q.CompareWholeMeasureRefinements(context.Background(), schedule)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Passes) != 2 || r.CellAttempts != r.Passes[0].Boundary.Partition.CellAttempts+r.Passes[1].Boundary.Partition.CellAttempts {
		t.Fatal("cumulative work")
	}
	width := func(a [2]*big.Rat) *big.Rat { return new(big.Rat).Sub(a[1], a[0]) }
	if width(r.RunningVolume[1]).Cmp(width(r.RunningVolume[0])) >= 0 {
		t.Fatal("volume width did not strictly decrease")
	}
	rr := 25.0 / 4096
	truth := math.Pi * rr * rr
	areas := [2]float64{2 * math.Pi * rr, math.Pi / 3 * (math.Pow(1+4*rr, 1.5) - 1)}
	for i := range r.Passes {
		lo, _ := r.RunningVolume[i][0].Float64()
		hi, _ := r.RunningVolume[i][1].Float64()
		if truth < lo || truth > hi {
			t.Fatal("independent volume", i)
		}
		for owner := 0; owner < 2; owner++ {
			lo, _ := r.RunningOwnerAreas[i][owner][0].Float64()
			hi, _ := r.RunningOwnerAreas[i][owner][1].Float64()
			if areas[owner] < lo || areas[owner] > hi {
				t.Fatal("independent owner area", owner)
			}
			if i > 0 && width(r.RunningOwnerAreas[i][owner]).Cmp(width(r.RunningOwnerAreas[i-1][owner])) > 0 {
				t.Fatal("area width increased")
			}
		}
	}
	if width(r.RunningOwnerAreas[1][0]).Cmp(width(r.RunningOwnerAreas[0][0])) >= 0 {
		t.Fatal("planar area did not tighten")
	}
	saved := new(big.Rat).Set(r.RunningVolume[0][0])
	r.Volume[0].SetInt64(99)
	if r.RunningVolume[0][0].Cmp(saved) != 0 {
		t.Fatal("aliased certificate history")
	}
	badSchedule := append([]AlgebraicGraphWindowAreaOptions(nil), schedule...)
	badSchedule[1].MaxSurfaceCells = 1
	bad, e := q.CompareWholeMeasureRefinements(context.Background(), badSchedule)
	if !errors.Is(e, geom.ErrConvergence) || len(bad.Passes) != 0 || bad.Volume[0] != nil {
		t.Fatal("partial success escaped", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	bad, e = q.CompareWholeMeasureRefinements(ctx, schedule)
	if !errors.Is(e, context.Canceled) || len(bad.Passes) != 0 {
		t.Fatal("cancel", e)
	}
	if _, e = q.CompareWholeMeasureRefinements(context.Background(), make([]AlgebraicGraphWindowAreaOptions, 5)); !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("unbounded passes", e)
	}
	t.Logf("volume widths%s -> %s, planar area widths%s -> %s, total attempts%d", width(r.RunningVolume[0]), width(r.RunningVolume[1]), width(r.RunningOwnerAreas[0][0]), width(r.RunningOwnerAreas[1][0]), r.CellAttempts)
}
