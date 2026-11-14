package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestGraphAffineWholeMass(t *testing.T) {
	s := graphMaterialBooleanFixture(t, false, true)
	axes := [2]int{0, 2}
	ctx := context.Background()
	q, e := s.PrepareGraphMaterialQuery(ctx, AlgebraicGraphMaterialOptions{Projection: &axes})
	if e != nil {
		t.Fatal(e)
	}
	// Reflected, translated and genuinely fiber-mixing (old residual is Y).
	m := geom.Affine{{2, .25, .5, -1}, {0, -1, 1, 2}, {.5, 0, 1, -3}}
	o := AlgebraicGraphAffineMassOptions{Original: AlgebraicGraphWholeMassOptions{MomentOrder: 2, Boundary: AlgebraicGraphWindowAreaOptions{Refinement: AlgebraicGraphWindowRefineOptions{MaxCells: 341, MaxDepth: 4}}}}
	r, e := q.BoundAffineWholeMassProperties(ctx, m, o)
	if e != nil {
		t.Fatal(e)
	}
	if !r.HasFirst || !r.HasSecond || !r.HasCentroid || !r.HasCentroidal || r.AbsoluteDeterminant.Cmp(big.NewRat(13, 8)) != 0 || r.Placement != m {
		t.Fatal("missing affine mass proof", r)
	}
	rr := 25.0 / 4096
	v := 13.0 / 8 * math.Pi * rr * rr
	originalMean := [3]float64{1.5, -3 + rr/3, 4.5}
	variance := [3]float64{9.0/256 + rr/6, rr * rr / 18, rr / 6}
	var mean [3]float64
	var covariance [3][3]float64
	for i := 0; i < 3; i++ {
		mean[i] = m[i][3]
		for k := 0; k < 3; k++ {
			mean[i] += m[i][k] * originalMean[k]
		}
		for j := 0; j < 3; j++ {
			for k := 0; k < 3; k++ {
				covariance[i][j] += m[i][k] * m[j][k] * variance[k]
			}
		}
	}
	check := func(name string, bounds [2]*big.Rat, truth float64) {
		t.Helper()
		lo, _ := bounds[0].Float64()
		hi, _ := bounds[1].Float64()
		if lo > truth || hi < truth {
			t.Fatalf("%s: %g outside [%g,%g]", name, truth, lo, hi)
		}
	}
	check("volume", r.Volume, v)
	for i := 0; i < 3; i++ {
		check("centroid", r.Centroid[i], mean[i])
		check("first", r.FirstMoments[i], v*mean[i])
	}
	pairs := [6][2]int{{0, 0}, {0, 1}, {0, 2}, {1, 1}, {1, 2}, {2, 2}}
	trace := covariance[0][0] + covariance[1][1] + covariance[2][2]
	rawTrace := trace + mean[0]*mean[0] + mean[1]*mean[1] + mean[2]*mean[2]
	for k, p := range pairs {
		raw := covariance[p[0]][p[1]] + mean[p[0]]*mean[p[1]]
		check("raw second", r.RawSecondMoments[k], v*raw)
		central := -covariance[p[0]][p[1]]
		origin := -raw
		if p[0] == p[1] {
			central += trace
			origin += rawTrace
		}
		check("centroidal inertia", r.CentroidalInertia[k], v*central)
		check("origin inertia", r.OriginInertia[k], v*origin)
	}
	// Returned original evidence cannot mutate the image intervals.
	saved := new(big.Rat).Set(r.RawSecondMoments[0][1])
	r.Original.Inertia.RawSecondMoments[0][1].SetInt64(99)
	if r.RawSecondMoments[0][1].Cmp(saved) != 0 {
		t.Fatal("source/image alias")
	}
	basic := o
	basic.Original.MomentOrder = 0
	b, e := q.BoundAffineWholeMassProperties(ctx, m, basic)
	if e != nil || b.HasFirst || b.HasSecond || b.FirstMoments[0][0] != nil {
		t.Fatal("default requested unneeded moments", e)
	}
	check("basic volume", b.Volume, v)
	fail := o
	fail.Transform.MaxOperations = 1
	bad, e := q.BoundAffineWholeMassProperties(ctx, m, fail)
	if !errors.Is(e, geom.ErrConvergence) || bad.Volume[0] != nil || bad.HasFirst {
		t.Fatal("atomic transform budget", e)
	}
	late := o
	late.Transform.MaxOperations = r.TransformOperations - 1
	bad, e = q.BoundAffineWholeMassProperties(ctx, m, late)
	if !errors.Is(e, geom.ErrConvergence) || bad.Volume[0] != nil || bad.HasSecond || bad.Original.Moments != nil {
		t.Fatal("late propagation failure leaked partial proof", e)
	}
	singular := m
	singular[2] = singular[0]
	bad, e = q.BoundAffineWholeMassProperties(ctx, singular, o)
	if !errors.Is(e, geom.ErrConvergence) || bad.Volume[0] != nil {
		t.Fatal("singular image", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	bad, e = q.BoundAffineWholeMassProperties(cancelled, m, o)
	if !errors.Is(e, context.Canceled) || bad.Volume[0] != nil {
		t.Fatal("cancel", e)
	}
	t.Logf("determinant13/8, %d transform operations, %d original cells; V[%s,%s], analytic%.12g", r.TransformOperations, r.Original.Whole.Boundary.Partition.CellAttempts, r.Volume[0], r.Volume[1], v)
}
