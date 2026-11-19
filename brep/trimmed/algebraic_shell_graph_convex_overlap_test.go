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

func TestGraphObliqueWholeVolume(t *testing.T) {
	slowtest.Skip(t)
	s := graphMaterialBooleanFixture(t, false, true)
	axes := [2]int{0, 2}
	q, e := s.PrepareGraphMaterialQuery(context.Background(), AlgebraicGraphMaterialOptions{Projection: &axes})
	if e != nil {
		t.Fatal(e)
	}
	base, e := q.BoundConvexWholeVolume(context.Background(), AlgebraicGraphConvexOverlapOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m := geom.Identity()
	m[0][0] = 2
	m[0][2] = .25
	m[1][1] = -1
	m[1][0] = .5
	r, e := q.BoundAffineConvexWholeVolume(context.Background(), m, axes, AlgebraicGraphConvexOverlapOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if !r.Complete || !r.HasProjectedArea || r.ProjectedArea.Cmp(big.NewRat(3, 4)) != 0 || len(r.Overlap) != 4 || r.Placement != m || !reflect.DeepEqual(r.NativeFaces, base.NativeFaces) {
		t.Fatal("exact oblique overlap", r)
	}
	want := map[string]bool{"53/16,17/4": true, "77/16,17/4": true, "79/16,19/4": true, "55/16,19/4": true}
	for _, p := range r.Overlap {
		key := p[0].RatString() + "," + p[1].RatString()
		if !want[key] {
			t.Fatal("original-domain image vertex", key)
		}
		delete(want, key)
	}
	if len(want) != 0 {
		t.Fatal("missing corners", want)
	}
	windowArea := new(big.Rat).Mul(new(big.Rat).Sub(r.Window.Max[0], r.Window.Min[0]), new(big.Rat).Sub(r.Window.Max[1], r.Window.Min[1]))
	if windowArea.Cmp(big.NewRat(13, 16)) != 0 || windowArea.Cmp(r.ProjectedArea) <= 0 {
		t.Fatal("bounding window promoted to polygon", windowArea)
	}
	truth := 2 * math.Pi * math.Pow(5.0/64, 4)
	hi, _ := r.Volume[1].Float64()
	if r.Volume[0].Sign() != 0 || hi < truth || r.Volume[1].Cmp(new(big.Rat).Mul(big.NewRat(2, 1), base.Volume[1])) != 0 {
		t.Fatal("independent determinant-scaled volume", r.Volume)
	}

	// Reverse the projected orientation and change the selected world axes.
	odd := geom.Affine{{0, 0, 1, 0}, {2, 0, .25, 0}, {0, -3, 0, 0}}
	reflected, e := q.BoundAffineConvexWholeVolume(context.Background(), odd, [2]int{0, 1}, AlgebraicGraphConvexOverlapOptions{})
	if e != nil || reflected.ResidualAxis != 2 || reflected.Volume[1].Cmp(new(big.Rat).Mul(big.NewRat(6, 1), base.Volume[1])) != 0 {
		t.Fatal("odd projection", e)
	}
	area2 := new(big.Rat)
	for i, p := range reflected.Overlap {
		z := reflected.Overlap[(i+1)%len(reflected.Overlap)]
		area2.Add(area2, new(big.Rat).Sub(new(big.Rat).Mul(p[0], z[1]), new(big.Rat).Mul(p[1], z[0])))
	}
	if area2.Cmp(big.NewRat(3, 2)) != 0 {
		t.Fatal("mapped polygon is not oriented CCW", area2)
	}
	// Returned ownership and polygon values cannot change prepared source proof.
	r.NativeFaces[0][0] = -1
	r.Original[0][0][0].SetInt64(999)
	r.Overlap[0][0].SetInt64(999)
	again, e := q.BoundAffineConvexWholeVolume(context.Background(), m, axes, AlgebraicGraphConvexOverlapOptions{})
	if e != nil || again.NativeFaces[0][0] < 0 || again.ProjectedArea.Cmp(big.NewRat(3, 4)) != 0 {
		t.Fatal("caller mutation", e)
	}
	for _, badMap := range []geom.Affine{{{1, 1, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}, {{0, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}, {{1, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 1, 0}}} {
		bad, e := q.BoundAffineConvexWholeVolume(context.Background(), badMap, axes, AlgebraicGraphConvexOverlapOptions{})
		if !errors.Is(e, geom.ErrConvergence) || bad.Complete || bad.ProjectedArea != nil {
			t.Fatal("invalid fiber map accepted", e)
		}
	}
	for _, o := range []AlgebraicGraphConvexOverlapOptions{{MaxVertices: 3}, {Exact: geom.ExactCurvePlaneEventOptions{MaxOperations: 1}}, {Exact: geom.ExactCurvePlaneEventOptions{MaxOperations: base.ExactOperations + 3}}} {
		bad, e := q.BoundAffineConvexWholeVolume(context.Background(), m, axes, o)
		if !errors.Is(e, geom.ErrConvergence) || bad.Complete || bad.ProjectedArea != nil {
			t.Fatal("atomic cumulative budget", e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	bad, e := q.BoundAffineConvexWholeVolume(ctx, m, axes, AlgebraicGraphConvexOverlapOptions{})
	if !errors.Is(e, context.Canceled) || bad.Complete {
		t.Fatal("cancel", e)
	}
	t.Logf("exact projected area%s versus bounding rectangle%s; V[0,%s], truth%.12g; %d operations", again.ProjectedArea, windowArea, again.Volume[1], truth, again.ExactOperations)
}
