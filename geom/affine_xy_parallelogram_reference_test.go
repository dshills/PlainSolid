package geom

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"slices"
	"testing"
)

func obliquePolynomialFixture(t *testing.T, axis int) (*SurfacePatch, *SurfacePatch, *SurfacePatch) {
	t.Helper()
	var poles []Vec3
	// Source base parameters (10-2v, 2-4u) are exchanged and reflected.
	for _, v := range []float64{1, 0} {
		for _, u := range []float64{1, 0} {
			poles = append(poles, Vec3{X: 2 + 2*u + v, Y: -1 + u + 3*v, Z: 1.0/8 + u/4 + 3*v/16})
		}
	}
	s, e := NewBSplineSurface(1, 1, 2, 2, poles, []float64{8, 8, 10, 10}, []float64{-2, -2, 2, 2}, nil)
	if e != nil {
		t.Fatal(e)
	}
	base, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	delta := arrangementGap(t, []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 4)}, []int{3, 2}, rationalPolynomial{big.NewRat(-1, 1)})
	physical, _, _ := polynomialPairGraphFixture(t, axis, delta)
	m := Affine{{2, 1, 0, 2}, {1, 3, 0, -1}, {0, 0, 1, 0}}
	a, e := independentProjectedPolynomialRoof(physical).Transformed(m)
	if e != nil {
		t.Fatal(e)
	}
	b, e := independentPolynomialArrangementRoof(t, axis, 1, 25.0/8).Transformed(m)
	if e != nil {
		t.Fatal(e)
	}
	return base, a, b
}

func TestAffineXYParallelogramReference(t *testing.T) {
	ctx := context.Background()
	base, a, b := obliquePolynomialFixture(t, 0)
	ref, e := NewAffineXYParallelogramReference(ctx, base, SurfacePatchOptions{})
	if e != nil || ref.Domain() != (ParameterRectangle{UMax: 1, VMax: 1}) {
		t.Fatal("positively oriented physical parallelogram reference", e)
	}
	baseOnRef, e := AlignAffineXYCharts(ctx, ref, base, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m, e := baseOnRef.MapCoefficients()
	want := [2][3]*big.Rat{{new(big.Rat), big.NewRat(-2, 1), big.NewRat(10, 1)}, {big.NewRat(-4, 1), new(big.Rat), big.NewRat(2, 1)}}
	if e != nil {
		t.Fatal(e)
	}
	for i, row := range m {
		if !slices.EqualFunc(row[:], want[i][:], func(a, b *big.Rat) bool { return a.Cmp(b) == 0 }) {
			t.Fatal("exact original exchanged and reflected base parameters", row, want[i])
		}
	}
	for _, roof := range []*SurfacePatch{a, b} {
		h, e := AlignAffineXYCharts(ctx, baseOnRef.TargetOnSource(), roof, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		height, e := h.HeightRange(ctx, GeneralSurfaceHeightOptions{})
		if e != nil || height.StrictSign != -1 {
			t.Fatal("each actual roof is strictly above the original sloping base", height.StrictSign, e)
		}
	}
	p, e := ref.EvaluateRational(ctx, big.NewRat(1, 4), big.NewRat(3, 8), SurfacePatchEvaluationOptions{})
	if e != nil || p.Point[0].Cmp(big.NewRat(23, 8)) != 0 || p.Point[1].Cmp(big.NewRat(3, 8)) != 0 || p.Point[2].Sign() != 0 {
		t.Fatal("reference keeps absolute physical XY and an independent height", p.Point, e)
	}
	for _, limit := range []int{ref.ExactOperations() - 1, ref.ExactOperations()} {
		got, e := NewAffineXYParallelogramReference(ctx, base, SurfacePatchOptions{MaxOperations: limit})
		if (e == nil && got != nil) != (limit == ref.ExactOperations()) {
			t.Fatal("one cumulative reference construction budget", limit, e)
		}
	}
	if r, e := NewAffineXYRectangleReference(ctx, base, SurfacePatchOptions{}); r != nil || !errors.Is(e, ErrDomain) {
		t.Fatal("physical rectangle proof still refuses the oblique footprint", e)
	}
}

func TestIntegrateAffineProjectedSplineGraphPolynomialVolumes(t *testing.T) {
	ctx := context.Background()
	// The original height antiderivatives in the unit station chart.
	// Their physical areas and volumes multiply by det([[2,1],[1,3]])=5.
	wantVolumes := [2][]*big.Rat{
		{big.NewRat(28550399214829, 2063645886382080), big.NewRat(590037023506439, 6190937659146240)},
		{big.NewRat(4909741, 2063645886382080), big.NewRat(1, 31488737280), big.NewRat(160842989, 2063645886382080)},
	}
	for axis := 0; axis < 2; axis++ {
		base, a, b := obliquePolynomialFixture(t, axis)
		ref, e := NewAffineXYParallelogramReference(ctx, base, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for order, roofs := range [][2]*SurfacePatch{{a, b}, {b, a}} {
			got, e := IntegrateAffineProjectedSplineGraphPolynomialVolumes(ctx, ref, roofs[0], roofs[1], axis, GraphPointContactOptions{})
			regions := [2][]int{{0, 4}, {1, 2, 3}}[order]
			if e != nil || got.AreaScale.Cmp(big.NewRat(5, 1)) != 0 || !got.Projection.Arrangement.Proven ||
				len(got.Projection.Arrangement.CertifiedCells) != 6 || !slices.Equal(got.Regions, regions) {
				t.Fatal("exact physical chart, determinant and positive layers", axis, order, got.Regions, e)
			}
			s := new(big.Rat)
			for i, vol := range wantVolumes[order] {
				want := new(big.Rat).Mul(vol, big.NewRat(5, 1))
				if got.Volumes[i].Cmp(want) != 0 {
					t.Fatal("independently integrated physical component volume", axis, order, i, got.Volumes[i], want)
				}
				s.Add(s, want)
			}
			if got.Total.Cmp(s) != 0 {
				t.Fatal("exact physical volume total", got.Total, s)
			}
			for _, limit := range []int{got.ExactOperations - 1, got.ExactOperations} {
				q, e := IntegrateAffineProjectedSplineGraphPolynomialVolumes(ctx, ref, roofs[0], roofs[1], axis, GraphPointContactOptions{MaxOperations: limit})
				if limit == got.ExactOperations {
					if e != nil || !reflect.DeepEqual(q, got) {
						t.Fatal("exact spent work is sufficient", e)
					}
				} else if !errors.Is(e, ErrConvergence) ||
					!reflect.DeepEqual(q, AffineProjectedSplineGraphPolynomialVolumes{ProjectedSplineGraphPolynomialVolumes: ProjectedSplineGraphPolynomialVolumes{ExactOperations: q.ExactOperations}}) {
					t.Fatal("one-short work publishes no chart, determinant or volume", e)
				}
			}
		}
		if got, e := IntegrateProjectedSplineGraphPolynomialVolumes(ctx, ref, a, b, axis, GraphPointContactOptions{}); got.Projection.Arrangement.Proven || !errors.Is(e, ErrConvergence) {
			t.Fatal("identity volume gate remains strict", axis, e)
		}
	}
}

func TestIntegrateAffineProjectedSplineGraphPolynomialVolumesReflectedReference(t *testing.T) {
	ctx := context.Background()
	base, a, b := obliquePolynomialFixture(t, 0)
	// The original sloping base chart exchanges both axes and reverses
	// physical orientation. Its UV area is eight and its XY area is five.
	got, e := IntegrateAffineProjectedSplineGraphPolynomialVolumes(ctx, base, a, b, 1, GraphPointContactOptions{})
	want := new(big.Rat).Add(big.NewRat(28550399214829, 412729177276416), big.NewRat(590037023506439, 1238187531829248))
	if e != nil || got.AreaScale.Cmp(big.NewRat(5, 8)) != 0 || got.Total.Cmp(want) != 0 ||
		got.Projection.Arrangement.Graph.ProjectionOrientation != -1 {
		t.Fatal("absolute physical Jacobian on the original reflected sloping reference", got.AreaScale, got.Total, want, e)
	}
}

func TestIntegrateAffineProjectedSplineGraphPolynomialVolumesRefusals(t *testing.T) {
	ctx := context.Background()
	base, a, b := obliquePolynomialFixture(t, 0)
	ref, e := NewAffineXYParallelogramReference(ctx, base, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	shift := Identity()
	shift[0][3] = .25
	shifted, e := b.Transformed(shift)
	if e != nil {
		t.Fatal(e)
	}
	if got, e := IntegrateAffineProjectedSplineGraphPolynomialVolumes(ctx, ref, a, shifted, 0, GraphPointContactOptions{}); got.Projection.Arrangement.Proven || !errors.Is(e, ErrDomain) {
		t.Fatal("shared oblique physical footprint must match exactly", e)
	}
	collapse := Identity()
	collapse[1] = [4]float64{}
	singular, e := ref.Transformed(collapse)
	if e != nil {
		t.Fatal(e)
	}
	if got, e := IntegrateAffineProjectedSplineGraphPolynomialVolumes(ctx, singular, a, b, 0, GraphPointContactOptions{}); got.AreaScale != nil || !errors.Is(e, ErrDegenerate) {
		t.Fatal("zero physical area cannot produce a volume", e)
	}
}
