package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"plainsolid/intersect"
	"testing"
)

func graphMaterialBooleanFixture(t *testing.T, nested, placed bool) AlgebraicShellBoolean {
	t.Helper()
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{-.125, -.125, 0}, [3]float64{1.125, 1.125, 1}, 0)
	var b Solid
	var hints intersect.SplineSeedReport
	o := AlgebraicShellSectionOptions{Trace: nestedTubeSectionOptions().Trace}
	o.Trace.Coverage.OverlappingAffineXYFallback = true
	if nested {
		b = nestedQuarticRoofSolid(t)
		x, e := algebraicShellTracePatch(a.faces[4], geom.ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 1})
		if e != nil {
			t.Fatal(e)
		}
		y, e := algebraicShellTracePatch(b.faces[1], geom.ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 1})
		if e != nil {
			t.Fatal(e)
		}
		hints, e = intersect.DiscoverRadialGraphPairTubes(ctx, x, y, geom.SurfacePatchOptions{}, intersect.SplinePlanePairDiscoveryOptions{MaxTubes: 128})
		if e != nil {
			t.Fatal(e)
		}
	} else {
		a, _ = makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
		a = exactPlaneOwnerForRegions(t, a)
		b = disjointLoopRoof(t)
		domain, e := faceUVBounds(b.faces[1])
		if e != nil {
			t.Fatal(e)
		}
		for i, center := range []float64{5.0 / 16, 11.0 / 16} {
			span := domain
			if i == 0 {
				span.UMax = .5
			} else {
				span.UMin = .5
			}
			x, e := algebraicShellTracePatch(a.faces[4], span)
			if e != nil {
				t.Fatal(e)
			}
			y, e := algebraicShellTracePatch(b.faces[1], span)
			if e != nil {
				t.Fatal(e)
			}
			opts := o.Trace.GraphFallback
			opts.UCuts = []float64{center - 1.0/16, center - 3.0/64, center + 3.0/64, center + 1.0/16}
			opts.VCuts = []float64{.5 - 1.0/16, .5 - 3.0/64, .5 + 3.0/64, .5 + 1.0/16}
			opts.Plane.Arcs.ResolveSubdivisionCorners = true
			h, e := intersect.DiscoverCommonXYGraphPairTubes(ctx, x, y, geom.SurfacePatchOptions{}, opts)
			if e != nil {
				t.Fatal(e)
			}
			hints.Attempts = append(hints.Attempts, h.Attempts...)
		}
	}
	if placed {
		a = placeAnnulusTestSolid(t, a, 2, geom.Vec3{X: 2, Y: -3, Z: 4})
		b = placeAnnulusTestSolid(t, b, 2, geom.Vec3{X: 2, Y: -3, Z: 4})
		o.Trace.GraphProjectionAxes = [2]int{0, 2}
		for i := range hints.Attempts {
			hints.Attempts[i].Sample = nil
			hints.Attempts[i].Section = nil
		}
	}
	o.Seeds = []AlgebraicShellSectionSeed{{Faces: [2]int{4, 1}, Tubes: hints}}
	section, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if e != nil || !section.Complete {
		t.Fatal("query source section", e)
	}
	measure := AlgebraicShellBooleanOptions{}
	measure.Imprint.Exact.MaxOperations = 512000000
	result, e := section.CombineAlgebraic(ctx, parametric.Intersection, measure)
	if e != nil {
		t.Fatal("query source Boolean", e)
	}
	return result
}
func queryPoint(x, y, z *big.Rat) [3]*big.Rat { return [3]*big.Rat{x, y, z} }
func nestedQueryHeight(x, y *big.Rat) *big.Rat {
	a := new(big.Rat).Sub(x, big.NewRat(1, 2))
	b := new(big.Rat).Sub(y, big.NewRat(1, 2))
	q := new(big.Rat).Add(new(big.Rat).Mul(a, a), new(big.Rat).Mul(b, b))
	return new(big.Rat).Mul(big.NewRat(-9, 16), new(big.Rat).Mul(new(big.Rat).Sub(q, big.NewRat(25, 1024)), new(big.Rat).Sub(q, big.NewRat(25, 256))))
}
func TestGraphMaterialNestedRational(t *testing.T) {
	slowtest.Skip(t)
	if testing.Short() {
		t.Skip("complete nested original source")
	}
	s := graphMaterialBooleanFixture(t, true, false)
	s.Faces = nil
	s.Edges = nil
	s.Vertices = nil
	q, e := s.PrepareGraphMaterialQuery(context.Background(), AlgebraicGraphMaterialOptions{})
	if e != nil {
		t.Fatal(e)
	}
	x, y := big.NewRat(2, 3), big.NewRat(7, 13)
	height := nestedQueryHeight(x, y)
	half := new(big.Rat).Quo(height, big.NewRat(2, 1))
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 200))
	cases := []struct {
		point [3]*big.Rat
		want  parametric.Location
	}{
		{queryPoint(x, y, half), parametric.Inside}, {queryPoint(x, y, new(big.Rat)), parametric.Boundary}, {queryPoint(x, y, height), parametric.Boundary},
		{queryPoint(x, y, tiny), parametric.Inside}, {queryPoint(x, y, new(big.Rat).Neg(tiny)), parametric.Outside},
		{queryPoint(big.NewRat(1, 2), big.NewRat(1, 2), new(big.Rat)), parametric.Outside},
		{queryPoint(big.NewRat(9, 10), big.NewRat(1, 2), new(big.Rat)), parametric.Outside},
	}
	for i, c := range cases {
		got, e := q.LocateRational(context.Background(), c.point, geom.ExactCurvePlaneEventOptions{})
		if e != nil || !got.Complete || got.Location != c.want {
			t.Fatal("rational material", i, got, e)
		}
		if i < 3 {
			if got.Faces != [2]int{0, 1} || got.Lower.Sign() != 0 || got.Upper.Cmp(height) != 0 {
				t.Fatal("exact native fiber", got)
			}
		}
	}
	rims := map[int]bool{}
	for _, x := range []*big.Rat{big.NewRat(13, 16), big.NewRat(21, 32)} {
		got, e := q.LocateRational(context.Background(), queryPoint(x, big.NewRat(1, 2), new(big.Rat)), geom.ExactCurvePlaneEventOptions{})
		if e != nil || got.Location != parametric.Boundary || len(got.RimEdges) < 1 || len(got.RimEdges) > 2 || len(got.BoundaryFaces) != 2 {
			t.Fatal("retained rim", got, e)
		}
		for _, edge := range got.RimEdges {
			if edge < 0 || edge >= len(s.index) {
				t.Fatal("native rim index")
			}
			rims[s.parts[0].Section[s.index[edge].SectionPiece].Branch] = true
		}
	}
	if len(rims) != 2 {
		t.Fatal("lost inner or outer native rim")
	}
	checkGraphQueryFailures(t, s, q, queryPoint(x, y, half))
}
func TestGraphMaterialPlacedDisjointRational(t *testing.T) {
	slowtest.Skip(t)
	s := graphMaterialBooleanFixture(t, false, true)
	axes := [2]int{0, 2}
	q, e := s.PrepareGraphMaterialQuery(context.Background(), AlgebraicGraphMaterialOptions{Projection: &axes})
	if e != nil {
		t.Fatal(e)
	}
	owners := map[[2]int]bool{}
	for _, x := range []*big.Rat{big.NewRat(5, 16), big.NewRat(11, 16)} {
		p := queryPoint(new(big.Rat).Sub(big.NewRat(2, 1), x), new(big.Rat).Add(big.NewRat(-3, 1), big.NewRat(25, 8192)), big.NewRat(9, 2))
		got, e := q.LocateRational(context.Background(), p, geom.ExactCurvePlaneEventOptions{})
		if e != nil || !got.Complete || got.Location != parametric.Inside {
			t.Fatal("placed exact component", got, e)
		}
		owners[got.Faces] = true
	}
	if len(owners) != 2 {
		t.Fatal("distinct retained native fragments")
	}
	if _, ok := q.faces[0].Surface.(*geom.ExactBezierSurface); !ok {
		t.Fatal("prepared original exact support lost")
	}
	inside := queryPoint(big.NewRat(27, 16), new(big.Rat).Add(big.NewRat(-3, 1), big.NewRat(25, 8192)), big.NewRat(9, 2))
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() {
			r, e := q.LocateRational(context.Background(), inside, geom.ExactCurvePlaneEventOptions{})
			if e == nil && (!r.Complete || r.Location != parametric.Inside) {
				e = errors.New("concurrent exact material mismatch")
			}
			if e == nil {
				r.Upper.SetInt64(0)
			}
			results <- e
		}()
	}
	for i := 0; i < 8; i++ {
		if e := <-results; e != nil {
			t.Fatal(e)
		}
	}
	repeated, e := q.LocateRational(context.Background(), inside, geom.ExactCurvePlaneEventOptions{})
	if e != nil || repeated.Upper.Cmp(new(big.Rat).Add(big.NewRat(-3, 1), big.NewRat(25, 4096))) != 0 {
		t.Fatal("caller result mutated prepared source", repeated, e)
	}
	checkGraphQueryFailures(t, s, q, inside)
	t.Log("prepared operations", q.PreparationOperations(), "query operations", repeated.ExactOperations)

	got, e := q.LocateRational(context.Background(), queryPoint(big.NewRat(3, 2), big.NewRat(-3, 1), big.NewRat(9, 2)), geom.ExactCurvePlaneEventOptions{})
	if e != nil || got.Location != parametric.Outside {
		t.Fatal("buried plane between disjoint lenses", got, e)
	}
}
func checkGraphQueryFailures(t *testing.T, s AlgebraicShellBoolean, q *AlgebraicGraphMaterialQuery, p [3]*big.Rat) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, test := range []struct {
		ctx  context.Context
		o    geom.ExactCurvePlaneEventOptions
		want error
	}{{ctx, geom.ExactCurvePlaneEventOptions{}, context.Canceled}, {context.Background(), geom.ExactCurvePlaneEventOptions{MaxOperations: 1}, geom.ErrConvergence}} {
		got, e := q.LocateRational(test.ctx, p, test.o)
		if !errors.Is(e, test.want) || got.Complete || got.Lower != nil || len(got.RimEdges) != 0 || got.Faces != [2]int{-1, -1} {
			t.Fatal("non-atomic query failure", got, e)
		}
	}
	failed, e := s.PrepareGraphMaterialQuery(context.Background(), AlgebraicGraphMaterialOptions{Exact: geom.ExactCurvePlaneEventOptions{MaxOperations: 1}})
	if !errors.Is(e, geom.ErrConvergence) || failed != nil {
		t.Fatal("preparation arithmetic budget", e)
	}
	failed, e = s.PrepareGraphMaterialQuery(context.Background(), AlgebraicGraphMaterialOptions{MaxLoops: 1})
	if !errors.Is(e, geom.ErrConvergence) || failed != nil {
		t.Fatal("preparation loop budget", e)
	}
}
func TestGraphMaterialQueryGuards(t *testing.T) {
	var q *AlgebraicGraphMaterialQuery
	got, e := q.LocateRational(context.Background(), [3]*big.Rat{}, geom.ExactCurvePlaneEventOptions{})
	if !errors.Is(e, geom.ErrInvalid) || got.Complete {
		t.Fatal(e)
	}
	failed, e := (AlgebraicShellBoolean{}).PrepareGraphMaterialQuery(context.Background(), AlgebraicGraphMaterialOptions{})
	if !errors.Is(e, geom.ErrInvalid) || failed != nil {
		t.Fatal(e)
	}
}
