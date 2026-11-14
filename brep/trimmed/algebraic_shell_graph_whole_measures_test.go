package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestGraphWholeMeasuresTwoLobes(t *testing.T) {
	s := graphMaterialBooleanFixture(t, false, true)
	axes := [2]int{0, 2}
	q, e := s.PrepareGraphMaterialQuery(context.Background(), AlgebraicGraphMaterialOptions{Projection: &axes})
	if e != nil {
		t.Fatal(e)
	}
	o := AlgebraicGraphWindowAreaOptions{Refinement: AlgebraicGraphWindowRefineOptions{MaxCells: 341, MaxDepth: 4}}
	r, e := q.BoundWholeMeasures(context.Background(), o)
	if e != nil {
		t.Fatal(e)
	}
	if !r.Boundary.Partition.HasBounds || r.Boundary.Partition.Complete || r.Boundary.Partition.ProjectionAxes != axes || r.Boundary.Partition.CellAttempts > o.Refinement.MaxCells {
		t.Fatal("whole shared certificate", r.Boundary.Partition)
	}
	if r.SupportWindow.Min[0].Cmp(big.NewRat(9, 8)) != 0 || r.SupportWindow.Max[0].Cmp(big.NewRat(15, 8)) != 0 || r.SupportWindow.Min[1].Cmp(big.NewRat(17, 4)) != 0 || r.SupportWindow.Max[1].Cmp(big.NewRat(19, 4)) != 0 {
		t.Fatal("original complete support overlap", r.SupportWindow)
	}
	rr := 25.0 / 4096
	exactVolume := math.Pi * rr * rr
	exactArea := [2]float64{2 * math.Pi * rr, math.Pi / 3 * (math.Pow(1+4*rr, 1.5) - 1)}
	lo, _ := r.Volume[0].Float64()
	hi, _ := r.Volume[1].Float64()
	if lo > exactVolume || hi < exactVolume || lo <= 0 {
		t.Fatal("whole independent volume", lo, hi, exactVolume)
	}
	for end := 0; end < 2; end++ {
		if r.Volume[end].Cmp(r.Boundary.Partition.Volume[end]) != 0 {
			t.Fatal("different volume partition")
		}
	}
	for owner := 0; owner < 2; owner++ {
		if len(r.Boundary.Owners[owner].NativeFaces) != 2 {
			t.Fatal("missing lobe owner")
		}
		lo, _ := r.Boundary.Owners[owner].Area[0].Float64()
		hi, _ := r.Boundary.Owners[owner].Area[1].Float64()
		if exactArea[owner] < lo || exactArea[owner] > hi {
			t.Fatal("whole independent boundary area", owner, lo, hi, exactArea[owner])
		}
	}
	capUpper := new(big.Rat).Add(r.Boundary.Partition.MaterialArea, r.Boundary.Partition.UnresolvedArea)
	if r.Boundary.Owners[0].Area[0].Cmp(r.Boundary.Partition.MaterialArea) != 0 || r.Boundary.Owners[0].Area[1].Cmp(capUpper) != 0 {
		t.Fatal("horizontal area did not use shared partition")
	}
	if r.Boundary.Partition.ProjectedArea.Cmp(big.NewRat(3, 8)) != 0 {
		t.Fatal("support area")
	}
	expected := new(big.Rat).Set(r.Boundary.Partition.Volume[0])
	r.Volume[0].SetInt64(99)
	if r.Boundary.Partition.Volume[0].Cmp(expected) != 0 {
		t.Fatal("result alias")
	}
	r.SupportWindow.Min[0].SetInt64(99)
	window, e := q.exactCommonGraphRectangle(16384)
	if e != nil || window.Min[0].Cmp(big.NewRat(9, 8)) != 0 {
		t.Fatal("immutable preparation")
	}
	badOptions := o
	badOptions.MaxSurfaceCells = 1
	bad, e := q.BoundWholeMeasures(context.Background(), badOptions)
	if !errors.Is(e, geom.ErrConvergence) || bad.Volume[0] != nil || bad.Boundary.Partition.HasBounds {
		t.Fatal("atomic whole budget", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	bad, e = q.BoundWholeMeasures(ctx, o)
	if !errors.Is(e, context.Canceled) || bad.Volume[0] != nil {
		t.Fatal("cancel", e)
	}
	broken := *q
	broken.faces[0].Outer = broken.faces[0].Outer[:3]
	bad, e = broken.BoundWholeMeasures(context.Background(), o)
	if !errors.Is(e, geom.ErrConvergence) || bad.Volume[0] != nil {
		t.Fatal("original nonrectangle accepted", e)
	}
	t.Logf("one partition: attempts%d, leaves%d, area support3/8, V[%s,%s], total area[%s,%s]", r.Boundary.Partition.CellAttempts, len(r.Boundary.Partition.Leaves), expected, r.Volume[1], r.Boundary.Total[0], r.Boundary.Total[1])
}
