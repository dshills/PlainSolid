//go:build research

package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestGraphRefinedObliqueWholeVolume(t *testing.T) {
	slowtest.Skip(t)
	s := graphMaterialBooleanFixture(t, false, true)
	axes := [2]int{0, 2}
	ctx := context.Background()
	q, e := s.PrepareGraphMaterialQuery(ctx, AlgebraicGraphMaterialOptions{Projection: &axes})
	if e != nil {
		t.Fatal(e)
	}
	m := geom.Identity()
	m[0][0] = 2
	m[0][2] = .25
	m[1][1] = -3
	m[1][0] = .5
	o := AlgebraicGraphConvexRefineOptions{Whole: AlgebraicGraphWindowAreaOptions{Refinement: AlgebraicGraphWindowRefineOptions{MaxCells: 341, MaxDepth: 4}}}
	wide, e := q.BoundAffineConvexWholeVolume(ctx, m, axes, o.Convex)
	if e != nil {
		t.Fatal(e)
	}
	r, e := q.BoundRefinedAffineConvexWholeVolume(ctx, m, axes, o)
	if e != nil {
		t.Fatal(e)
	}
	if !r.Complete || r.OriginalWhole == nil || !r.OriginalWhole.Boundary.Partition.HasBounds || r.Volume[0].Sign() <= 0 || r.Volume[1].Cmp(wide.Volume[1]) >= 0 {
		t.Fatal("no certified tightening", r.Volume, e)
	}
	truth := 6 * math.Pi * math.Pow(5.0/64, 4)
	lo, _ := r.Volume[0].Float64()
	hi, _ := r.Volume[1].Float64()
	if lo > truth || hi < truth {
		t.Fatal("independent determinant-scaled truth", lo, truth, hi)
	}
	for i := 0; i < 2; i++ {
		if r.Volume[i].Cmp(new(big.Rat).Mul(big.NewRat(6, 1), r.OriginalWhole.Volume[i])) != 0 {
			t.Fatal("exact volume scaling", i)
		}
	}
	if !reflect.DeepEqual(r.Overlap, wide.Overlap) || !reflect.DeepEqual(r.NativeFaces, wide.NativeFaces) || r.ProjectedArea.Cmp(big.NewRat(3, 4)) != 0 {
		t.Fatal("changed projected exclusion/ownership")
	}
	if r.OriginalWhole.SupportWindow.Min[0].Cmp(big.NewRat(9, 8)) != 0 {
		t.Fatal("original partition misrepresented as world image")
	}
	// Independent returned enclosures share no mutable rationals.
	saved := new(big.Rat).Set(r.Volume[1])
	r.OriginalWhole.Volume[1].SetInt64(99)
	if r.Volume[1].Cmp(saved) != 0 {
		t.Fatal("partition aliases image enclosure")
	}
	// Structural fallback is distinct from resource failure. This deliberately
	// incomplete local descriptor only tests eligibility, never material proof.
	nonrectangle := *q
	nonrectangle.faces[0] = cloneFace(q.faces[0])
	nonrectangle.faces[0].Outer = nonrectangle.faces[0].Outer[:3]
	if nonrectangle.graphWholePartitionEligible() {
		t.Fatal("nonrectangle classified as supported")
	}
	for _, badOpts := range []AlgebraicGraphConvexRefineOptions{{Convex: AlgebraicGraphConvexOverlapOptions{Exact: geom.ExactCurvePlaneEventOptions{MaxOperations: 256}}}, {Whole: AlgebraicGraphWindowAreaOptions{MaxSurfaceCells: 1}}} {
		bad, e := q.BoundRefinedAffineConvexWholeVolume(ctx, m, axes, badOpts)
		if !errors.Is(e, geom.ErrConvergence) || bad.Complete || bad.OriginalWhole != nil || bad.Volume[0] != nil {
			t.Fatal("budget failure became wide fallback", e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	bad, e := q.BoundRefinedAffineConvexWholeVolume(cancelled, m, axes, o)
	if !errors.Is(e, context.Canceled) || bad.Complete {
		t.Fatal("cancel", e)
	}
	t.Logf("original partition%d attempts: image V[%s,%s] vs wide[0,%s], analytic%.12g", r.OriginalWhole.Boundary.Partition.CellAttempts, r.Volume[0], r.Volume[1], wide.Volume[1], truth)
}
