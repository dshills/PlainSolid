package geom

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"slices"
	"testing"
)

func clippedPolynomialFixture(t *testing.T, axis int, ranges [2][2]float64) [3]*SurfacePatch {
	t.Helper()
	delta := arrangementGap(t, []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 4)}, []int{3, 2}, rationalPolynomial{big.NewRat(-1, 1)})
	physical, _, _ := polynomialPairGraphFixture(t, axis, delta)
	roofs := [2]*SurfacePatch{independentProjectedPolynomialRoof(physical), independentPolynomialArrangementRoof(t, axis, 1, 25.0/8)}
	xform := Affine{{2, 0, 0, 2}, {0, 3, 0, -1}, {0, 0, 1, 0}}
	for i, p := range roofs {
		lo, hi := ranges[i][0], ranges[i][1]
		d := p.Domain()
		switch {
		case i == 1:
			d.UMin, d.UMax = 5-3*hi, 5-3*lo
		case axis == 0:
			d.UMin, d.UMax = 4*lo-2, 4*hi-2
		default:
			d.VMin, d.VMax = 10-2*hi, 10-2*lo
		}
		sub, e := p.Trim(d)
		if e != nil {
			t.Fatal(e)
		}
		roofs[i], e = sub.Transformed(xform)
		if e != nil {
			t.Fatal(e)
		}
	}
	return [3]*SurfacePatch{originalPhysicalRectangleBase(t, false), roofs[0], roofs[1]}
}

func TestClippedSplineGraphPolynomialArrangement(t *testing.T) {
	ctx := context.Background()
	equal := func(x, y *big.Rat) bool { return x.Cmp(y) == 0 }
	for layout, ranges := range [][2][2]float64{{{0, .875}, {.125, 1}}, {{0, 1}, {.125, .875}}} {
		for axis := 0; axis < 2; axis++ {
			src := clippedPolynomialFixture(t, axis, ranges)
			for order, roofs := range [][2]*SurfacePatch{{src[1], src[2]}, {src[2], src[1]}} {
				r, e := DiscoverClippedSplineGraphPolynomialArrangement(ctx, src[0], roofs[0], roofs[1], axis, GraphPointContactOptions{})
				if e != nil || !r.Projection.Arrangement.Proven || !slices.Equal(r.Projection.Arrangement.ContactOrder, []int{3, 2}) ||
					len(r.Projection.Arrangement.CertifiedCells) != 6 {
					t.Fatal("complete contact graph on original independently bounded roofs", layout, axis, order, e)
				}
				physical := [2]ParameterRectangle{{UMin: 2.25, UMax: 3.75, VMin: -1, VMax: 2}, {UMin: 2, UMax: 4, VMin: -.625, VMax: 1.625}}[axis]
				expected := [3]ParameterRectangle{
					[2]ParameterRectangle{{UMin: 8, UMax: 10, VMin: -1.5, VMax: 1.5}, {UMin: 8.25, UMax: 9.75, VMin: -2, VMax: 2}}[axis],
					[2]ParameterRectangle{{UMin: -1.5, UMax: 1.5, VMin: 8, VMax: 10}, {UMin: -2, UMax: 2, VMin: 8.25, VMax: 9.75}}[axis],
					{UMin: 2.375, UMax: 4.625, VMin: -1, VMax: 1},
				}
				if order == 1 {
					expected[1], expected[2] = expected[2], expected[1]
				}
				if r.OriginalDomains != ([3]ParameterRectangle{src[0].Domain(), roofs[0].Domain(), roofs[1].Domain()}) ||
					r.RestrictedDomains != expected || r.Reference.Domain() != physical || r.Projection.Arrangement.Domain != physical {
					t.Fatal("absolute physical overlap and all three dyadic owner rectangles", layout, axis, order, r.RestrictedDomains, expected)
				}
				mask := [2][2][4]uint8{{{3, 1, 3, 2}, {2, 3, 1, 3}}, {{3, 2, 3, 2}, {2, 3, 2, 3}}}[layout][axis]
				if order == 1 {
					for i, side := range mask {
						mask[i] = (side&1)<<1 | (side&2)>>1
					}
				}
				if r.BoundaryRoofs != mask {
					t.Fatal("exact original footprint side roles", layout, axis, order, r.BoundaryRoofs, mask)
				}
				for i, owner := range r.Projection.Owners {
					if owner.Domain != r.RestrictedDomains[1+i] || r.BaseMinusRoof[i].StrictSign != -1 {
						t.Fatal("original owner trace and strict stock clearance", i)
					}
					for v, uv := range owner.Vertices {
						got, e := roofs[i].EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
						want := r.Projection.Arrangement.Vertices[v].Point
						if e != nil || !slices.EqualFunc(got.Point[:], want[:], equal) {
							t.Fatal("clipped pcurve vertices lift on the unmodified owner", i, v, e)
						}
					}
				}
				for i, v := range r.Projection.Arrangement.Vertices[4:] {
					want := [2][2]*big.Rat{{big.NewRat(5, 2), big.NewRat(7, 2)}, {big.NewRat(-1, 4), big.NewRat(5, 4)}}[axis][i]
					if v.Point[axis].Cmp(want) != 0 || v.Parameters[axis].Cmp(want) != 0 {
						t.Fatal("clipped contact retains its absolute station", i, v, want)
					}
				}
			}
		}
	}
}

func TestClippedSplineGraphPolynomialArrangementTwoAxes(t *testing.T) {
	ctx := context.Background()
	s := clippedPolynomialFixture(t, 0, [2][2]float64{{0, .875}, {.125, 1}})
	for i, domain := range [2]ParameterRectangle{
		{UMin: -2, UMax: 1.5, VMin: 8.25, VMax: 10}, {UMin: 2, UMax: 4.625, VMin: -.75, VMax: 1},
	} {
		q, e := s[i+1].Trim(domain)
		if e != nil {
			t.Fatal(e)
		}
		s[i+1] = q
	}
	p, e := DiscoverClippedSplineGraphPolynomialArrangement(ctx, s[0], s[1], s[2], 0, GraphPointContactOptions{})
	if e != nil || p.Reference.Domain() != (ParameterRectangle{UMin: 2.25, UMax: 3.75, VMin: -.625, VMax: 1.625}) ||
		p.BoundaryRoofs != ([4]uint8{2, 1, 1, 2}) || len(p.Projection.Arrangement.Regions) != 5 {
		t.Fatal("each axis has sides from both independently clipped originals", p.BoundaryRoofs, e)
	}
	for i := range p.Projection.Arrangement.Edges {
		for _, t0 := range []*big.Rat{big.NewRat(1, 3), big.NewRat(2, 3)} {
			var points [2][3]*big.Rat
			for owner, roof := range s[1:] {
				uv := testGraphBezierValue(p.Projection.Owners[owner].Edges[i].Controls, t0)
				point, e := roof.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				points[owner] = point.Point
			}
			for j, x := range points[0] {
				if x.Cmp(points[1][j]) != 0 {
					t.Fatal("owner-specific clipped edge traces agree exactly on the originals", i, t0, points)
				}
			}
		}
	}
}

func TestClippedSplineGraphPolynomialArrangementRefusals(t *testing.T) {
	ctx := context.Background()
	src := clippedPolynomialFixture(t, 0, [2][2]float64{{0, .875}, {.125, 1}})
	proof, e := DiscoverClippedSplineGraphPolynomialArrangement(ctx, src[0], src[1], src[2], 0, GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	atLimit, e := DiscoverClippedSplineGraphPolynomialArrangement(ctx, src[0], src[1], src[2], 0,
		GraphPointContactOptions{MaxOperations: proof.ExactOperations})
	if e != nil || atLimit.ExactOperations != proof.ExactOperations || atLimit.RestrictedDomains != proof.RestrictedDomains ||
		!reflect.DeepEqual(atLimit.Projection, proof.Projection) {
		t.Fatal("one cumulative exact clipping, height and contact budget", e)
	}
	shortBase, e := src[0].Trim(ParameterRectangle{UMin: 8, UMax: 10, VMin: -1.5, VMax: 2})
	if e != nil {
		t.Fatal(e)
	}
	above := *src[0]
	above.net = make([]exactHomogeneous, len(src[0].net))
	for i, h := range src[0].net {
		above.net[i] = h
		// Physical free x=(2-V)/4. This slope is below the roofs on the
		// overlap but crosses the original A stock to its left.
		v := i % src[0].countV
		above.net[i][2] = new(big.Rat).Mul(h[3], big.NewRat(int64(-6+10*v), 1))
	}
	if e := above.computeBounds(&polynomialBudget{maxOperations: 1000, maxBits: 1024}); e != nil {
		t.Fatal(e)
	}
	// A's dyadic physical cut at X=15/4 maps to 7/12 in this roof.
	s, e := NewBSplineSurface(1, 1, 2, 2, []Vec3{{X: 2, Y: -1, Z: 3}, {X: 2, Y: 2, Z: 3}, {X: 5, Y: -1, Z: 3}, {X: 5, Y: 2, Z: 3}},
		[]float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	unrepresented, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	shift := Identity()
	shift[0][3] = 2
	gap, e := src[2].Transformed(shift)
	if e != nil {
		t.Fatal(e)
	}
	shift[0][3] = 1.5
	touch, e := src[2].Transformed(shift)
	if e != nil {
		t.Fatal(e)
	}
	tooNarrow := clippedPolynomialFixture(t, 0, [2][2]float64{{0, .875}, {.375, .625}})
	for _, tc := range []struct {
		base, b *SurfacePatch
		o       GraphPointContactOptions
		err     error
	}{
		{src[0], src[2], GraphPointContactOptions{MaxOperations: proof.ExactOperations - 1}, ErrConvergence},
		{shortBase, src[2], GraphPointContactOptions{}, ErrDomain},
		{src[0], unrepresented, GraphPointContactOptions{}, ErrUnrepresentable},
		{&above, src[2], GraphPointContactOptions{}, ErrConvergence},
		{src[0], gap, GraphPointContactOptions{}, ErrDomain},
		{src[0], touch, GraphPointContactOptions{}, ErrConvergence},
		{src[0], tooNarrow[2], GraphPointContactOptions{}, ErrConvergence},
	} {
		r, e := DiscoverClippedSplineGraphPolynomialArrangement(ctx, tc.base, src[1], tc.b, 0, tc.o)
		if !errors.Is(e, tc.err) || !reflect.DeepEqual(r, ClippedSplineGraphPolynomialArrangement{ExactOperations: r.ExactOperations}) {
			t.Fatal("coverage, stock height and contact failures publish no partial ledger", tc.err, e)
		}
	}
}
