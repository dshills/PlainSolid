package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestGraphAffineMeasuresSharedPartition(t *testing.T) {
	ctx := context.Background()
	s := graphMaterialBooleanFixture(t, false, true)
	axes := [2]int{0, 2}
	q, e := s.PrepareGraphMaterialQuery(ctx, AlgebraicGraphMaterialOptions{Projection: &axes})
	if e != nil {
		t.Fatal(e)
	}
	m := geom.Affine{{2, 0, .5, 1}, {.25, 3, 0, -2}, {0, 0, 1, 5}}
	o := AlgebraicGraphAffineMeasureOptions{Original: AlgebraicGraphWholeMassOptions{MomentOrder: 2, Boundary: AlgebraicGraphWindowAreaOptions{Refinement: AlgebraicGraphWindowRefineOptions{MaxCells: 341, MaxDepth: 4}}}, IncludeBoundary: true}
	r, e := q.BoundAffineWholeMeasures(ctx, m, o)
	if e != nil {
		t.Fatal(e)
	}
	if r.Boundary == nil || !r.Mass.HasCentroidal || r.OriginalCellAttempts <= 0 {
		t.Fatal("missing requested certificates")
	}
	massPartition := r.Mass.Original.Whole.Boundary.Partition
	areaPartition := r.Boundary.Original.Boundary.Partition
	if r.OriginalCellAttempts != massPartition.CellAttempts || r.OriginalCellAttempts != areaPartition.CellAttempts || massPartition.ExactOperations != areaPartition.ExactOperations || len(massPartition.Leaves) == 0 || &massPartition.Leaves[0] != &areaPartition.Leaves[0] {
		t.Fatal("not one private same-call partition")
	}
	standalone, e := q.BoundAffineWholeSurfaceArea(ctx, m, AlgebraicGraphAffineAreaOptions{Original: o.Original.Boundary})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(standalone.Owners, r.Boundary.Owners) || !reflect.DeepEqual(standalone.Total, r.Boundary.Total) || !reflect.DeepEqual(standalone.Original.Boundary.Partition.Leaves, massPartition.Leaves) || standalone.Original.Boundary.Partition.CellAttempts != r.OriginalCellAttempts {
		t.Fatal("shared path changed independent area/leaf proof")
	}
	rr := 25.0 / 4096
	v := 6 * math.Pi * rr * rr
	domainArea := 2 * math.Pi * rr
	sourceMean := [3]float64{1.5, -3 + rr/3, 4.5}
	variance := [3]float64{9.0/256 + rr/6, rr * rr / 18, rr / 6}
	var mean [3]float64
	var covariance [3][3]float64
	for i := 0; i < 3; i++ {
		mean[i] = m[i][3]
		for k := 0; k < 3; k++ {
			mean[i] += m[i][k] * sourceMean[k]
		}
		for j := 0; j < 3; j++ {
			for k := 0; k < 3; k++ {
				covariance[i][j] += m[i][k] * m[j][k] * variance[k]
			}
		}
	}
	contains := func(label string, b [2]*big.Rat, lo, hi float64) {
		t.Helper()
		a, _ := b[0].Float64()
		z, _ := b[1].Float64()
		if a > lo || z < hi {
			t.Fatalf("%s: [%g,%g] outside [%g,%g]", label, lo, hi, a, z)
		}
	}
	contains("volume", r.Mass.Volume, v, v)
	for i := 0; i < 3; i++ {
		contains("centroid", r.Mass.Centroid[i], mean[i], mean[i])
		contains("first", r.Mass.FirstMoments[i], v*mean[i], v*mean[i])
	}
	pairs := [6][2]int{{0, 0}, {0, 1}, {0, 2}, {1, 1}, {1, 2}, {2, 2}}
	trace := covariance[0][0] + covariance[1][1] + covariance[2][2]
	for k, p := range pairs {
		raw := v * (covariance[p[0]][p[1]] + mean[p[0]]*mean[p[1]])
		central := -covariance[p[0]][p[1]]
		if p[0] == p[1] {
			central += trace
		}
		contains("second", r.Mass.RawSecondMoments[k], raw, raw)
		contains("central inertia", r.Mass.CentroidalInertia[k], v*central, v*central)
	}
	// Independent disk-gradient enclosure from the analytic source paraboloids.
	base, delta := math.Sqrt(261)/8, 2*(5.0/64)*math.Sqrt(47.25)
	contains("plane area", r.Boundary.Owners[0].Area, domainArea*base, domainArea*base)
	contains("curved area", r.Boundary.Owners[1].Area, domainArea*(base-delta), domainArea*(base+delta))
	basicOptions := o
	basicOptions.IncludeBoundary = false
	basicOptions.Original.MomentOrder = 0
	basicOptions.MaxImageCells = -1 // Inactive image quotas do not enable work.
	basic, e := q.BoundAffineWholeMeasures(ctx, m, basicOptions)
	if e != nil || basic.Boundary != nil || basic.Mass.HasFirst || basic.Mass.HasSecond {
		t.Fatal("optional work became mandatory", e)
	}
	contains("basic volume", basic.Mass.Volume, v, v)
	badOptions := o
	badOptions.MaxImageCells = 1
	badOptions.Original.MomentOrder = 0
	bad, e := q.BoundAffineWholeMeasures(ctx, m, badOptions)
	if !errors.Is(e, geom.ErrConvergence) || bad.Boundary != nil || bad.Mass.Volume[0] != nil || bad.OriginalCellAttempts != 0 {
		t.Fatal("later area failure leaked mass", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	bad, e = q.BoundAffineWholeMeasures(cancelled, m, o)
	if !errors.Is(e, context.Canceled) || bad.Boundary != nil || bad.Mass.Volume[0] != nil {
		t.Fatal("cancel", e)
	}
	t.Logf("one shared partition%d attempts/%d leaves; %d transform operations, %d image owner cells", r.OriginalCellAttempts, len(massPartition.Leaves), r.Mass.TransformOperations, r.Boundary.ImageCells)
}
