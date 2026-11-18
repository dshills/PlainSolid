package trimmed

import (
	"context"
	"errors"
	"math"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestGraphWholeMassSharedPartition(t *testing.T) {
	slowtest.Skip(t)
	s := graphMaterialBooleanFixture(t, false, true)
	axes := [2]int{0, 2}
	q, e := s.PrepareGraphMaterialQuery(context.Background(), AlgebraicGraphMaterialOptions{Projection: &axes})
	if e != nil {
		t.Fatal(e)
	}
	o := AlgebraicGraphWholeMassOptions{Boundary: AlgebraicGraphWindowAreaOptions{Refinement: AlgebraicGraphWindowRefineOptions{MaxCells: 341, MaxDepth: 4}}}
	basic, e := q.BoundWholeMassProperties(context.Background(), o)
	if e != nil || basic.Moments != nil || basic.Inertia != nil {
		t.Fatal("default work changed", e)
	}
	o.MomentOrder = 2
	r, e := q.BoundWholeMassProperties(context.Background(), o)
	if e != nil {
		t.Fatal(e)
	}
	if r.Moments == nil || r.Inertia == nil || !r.Moments.HasCentroid || !r.Inertia.HasCentroidal {
		t.Fatal("optional whole moments missing")
	}
	partition := r.Whole.Boundary.Partition
	if partition.CellAttempts != basic.Whole.Boundary.Partition.CellAttempts || !reflect.DeepEqual(partition.Leaves, basic.Whole.Boundary.Partition.Leaves) || !reflect.DeepEqual(r.Whole.Boundary.Owners, basic.Whole.Boundary.Owners) {
		t.Fatal("moment path changed material/area partition")
	}
	if r.Moments.Volume.Bounds.CellAttempts != partition.CellAttempts || r.Inertia.Moments.Volume.Bounds.CellAttempts != partition.CellAttempts || r.Moments.Volume.Bounds.UnresolvedArea.Cmp(partition.UnresolvedArea) != 0 || partition.UnresolvedArea.Sign() <= 0 {
		t.Fatal("shared unresolved evidence")
	}
	for i := 0; i < 2; i++ {
		if r.Whole.Volume[i].Cmp(r.Moments.Volume.Bounds.Volume[i]) != 0 || r.Whole.Volume[i].Cmp(partition.Volume[i]) != 0 {
			t.Fatal("inconsistent tightened whole volume")
		}
	}
	if r.Whole.Volume[0].Cmp(basic.Whole.Volume[0]) < 0 || r.Whole.Volume[1].Cmp(basic.Whole.Volume[1]) > 0 {
		t.Fatal("polynomial integration loosened volume")
	}
	rr := 25.0 / 4096
	v := math.Pi * rr * rr
	meanY := -3 + rr/3
	centroid := [3]float64{1.5, meanY, 4.5}
	raw := [6]float64{v * ((math.Pow(27.0/16, 2)+math.Pow(21.0/16, 2))/2 + rr/6), v * 1.5 * meanY, v * 1.5 * 4.5, v * (9 - 2*rr + rr*rr/6), v * 4.5 * meanY, v * (4.5*4.5 + rr/6)}
	central := [6]float64{v * (rr*rr/18 + rr/6), 0, 0, v * (9.0/256 + rr/3), 0, v * (9.0/256 + rr/6 + rr*rr/18)}
	lo, _ := r.Whole.Volume[0].Float64()
	hi, _ := r.Whole.Volume[1].Float64()
	if v < lo || v > hi {
		t.Fatal("whole mass")
	}
	for i, c := range centroid {
		lo, _ := r.Moments.Centroid[i][0].Float64()
		hi, _ := r.Moments.Centroid[i][1].Float64()
		if c < lo || c > hi {
			t.Fatal("whole centroid", i)
		}
		lo, _ = r.Moments.FirstMoments[i][0].Float64()
		hi, _ = r.Moments.FirstMoments[i][1].Float64()
		if v*c < lo || v*c > hi {
			t.Fatal("whole first moment", i)
		}
	}
	for i := 0; i < 6; i++ {
		lo, _ := r.Inertia.RawSecondMoments[i][0].Float64()
		hi, _ := r.Inertia.RawSecondMoments[i][1].Float64()
		if raw[i] < lo || raw[i] > hi {
			t.Fatal("whole raw tensor", i)
		}
		lo, _ = r.Inertia.CentroidalInertia[i][0].Float64()
		hi, _ = r.Inertia.CentroidalInertia[i][1].Float64()
		if central[i] < lo || central[i] > hi {
			t.Fatal("whole central tensor", i)
		}
	}
	o.MomentOrder = 1
	first, e := q.BoundWholeMassProperties(context.Background(), o)
	if e != nil || first.Inertia != nil || first.Moments == nil || !reflect.DeepEqual(first.Moments.FirstMoments, r.Moments.FirstMoments) {
		t.Fatal("first-only path", e)
	}
	o.MomentOrder = 2
	o.SecondCellOperations = 1
	bad, e := q.BoundWholeMassProperties(context.Background(), o)
	if !errors.Is(e, geom.ErrConvergence) || bad.Whole.Volume[0] != nil || bad.Moments != nil || bad.Inertia != nil {
		t.Fatal("atomic optional failure", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	bad, e = q.BoundWholeMassProperties(ctx, o)
	if !errors.Is(e, context.Canceled) || bad.Whole.Volume[0] != nil {
		t.Fatal("cancel", e)
	}
	t.Logf("one partition%d attempts/%d leaves; exact cells%d; whole V%s..%s; unresolved area%s", partition.CellAttempts, len(partition.Leaves), len(r.Inertia.ExactCells), r.Whole.Volume[0], r.Whole.Volume[1], partition.UnresolvedArea)
}
