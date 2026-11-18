package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// The box clips a second positive roof lobe through its center. The first
// lobe creates a closed section inside the floor face; the second creates
// an open arc on that very face and an arc on the right wall. Their model
// sections never touch, and the exact common volume is 3/2 of a single lobe.
func TestCombineSplineShellsMixedOpenAndClosed(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{0, 0, 0}, [3]float64{.71875, 1, 1}, 0)
	b, _ := twoPeakSplineRoof(t)
	o := SplineShellCombineOptions{}
	o.Open.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Open.Trace.Coverage.Isolation.ParameterTolerance = .125
	o.Open.Trace.Coverage.Cell.MaxOperations = 16000000
	var e error
	o.Sections, e = mixedSplineSections(t, a, b, o.Open.Trace)
	if e != nil {
		t.Fatal(e)
	}
	s, h, e := CombineSplineShells(ctx, a, b, parametric.Intersection, o)
	if e != nil {
		t.Fatal(e)
	}
	if h.Method != "section-arrangement" || len(s.Topology().Shells) != 2 || s.Topology().EulerCharacteristic != 4 || len(h.FacesA[4]) != 2 || len(h.FacesA[1]) != 1 || len(h.FacesB[1]) != 2 {
		t.Fatal(s.Topology(), h)
	}
	m, e := s.refine(ctx, 2, 0, 0)
	want := .75 * math.Pi * math.Pow(5.0/32, 4)
	if e != nil || m.ValidateSolid() != nil || math.Abs(m.SignedVolume()-want) > want*.08 {
		t.Fatal(m.SignedVolume(), want, e)
	}
}

func TestDiscoverMixedSplineShellSections(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{0, 0, 0}, [3]float64{.71875, 1, 1}, 0)
	b, _ := twoPeakSplineRoof(t)
	var ap, bp []splineEmbeddingFace
	if _, e := a.validateSplineRectangleEmbedding(ctx, SplineRectangleEmbeddingOptions{}, &ap); e != nil {
		t.Fatal(e)
	}
	if _, e := b.validateSplineRectangleEmbedding(ctx, SplineRectangleEmbeddingOptions{}, &bp); e != nil {
		t.Fatal(e)
	}
	o := SplineShellOpenOptions{}
	o.Trace.PlaneFallback.UCuts = []float64{.15625, .1875, .375, .40625, .59375, .625, .8125, .84375}
	o.Trace.PlaneFallback.VCuts = []float64{.375, .40625, .59375, .625}
	sections, curved, e := discoverSplineShellSections(ctx, a, b, ap, bp, [][2]int{{4, 1}, {1, 1}}, o, true, true, false)
	if e != nil || !curved || len(sections) != 3 {
		t.Fatal("mixed plane discovery", curved, len(sections), e)
	}
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .125
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	const r = 5.0 / 32
	common := .75 * math.Pi * math.Pow(r, 4)
	box := 23.0 / 32
	// The two equal x spans and one y span have exact polynomial
	// mean heights from independent integrals of x² and y².
	roof := (7.0 / 8) * (1.0 / 2) * (1 + r*r - math.Pow(7.0/32, 2)/3 - 1.0/48)
	for _, tc := range []struct {
		name          string
		op            parametric.BooleanOp
		shells, euler int
		volume, tol   float64
	}{{"common", parametric.Intersection, 2, 4, common, common * .08},
		{"union", parametric.Union, 1, 0, box + roof - common, .004},
		{"cut", parametric.Difference, 1, 2, box - common, .004}} {
		t.Run(tc.name, func(t *testing.T) {
			s, h, e := CombineSplineShells(ctx, a, b, tc.op, SplineShellCombineOptions{Open: o})
			if e != nil || h.Method != "plane-section-arrangement" || len(s.Topology().Shells) != tc.shells || s.Topology().EulerCharacteristic != tc.euler {
				t.Fatal("automatic mixed arrangement", h.Method, s.Topology(), e)
			}
			if testing.Short() {
				return
			}
			m, e := s.refine(ctx, 2, 0, 0)
			if e != nil || m.ValidateSolid() != nil || math.Abs(m.SignedVolume()-tc.volume) > tc.tol {
				t.Fatal("mesh volume", m.SignedVolume(), tc.volume, e)
			}
		})
	}
}

func mixedSplineSections(t *testing.T, a, b Solid, o intersect.SplinePairTraceOptions) ([]SplineShellSection, error) {
	t.Helper()
	ctx := context.Background()
	var ap, bp []splineEmbeddingFace
	if _, e := a.validateSplineRectangleEmbedding(ctx, SplineRectangleEmbeddingOptions{}, &ap); e != nil {
		return nil, e
	}
	if _, e := b.validateSplineRectangleEmbedding(ctx, SplineRectangleEmbeddingOptions{}, &bp); e != nil {
		return nil, e
	}
	p, _ := intersectSplineFace(a.faces[4], ap[4].patch.Domain())
	q, _ := intersectSplineFace(b.faces[1], bp[1].patch.Domain())
	const cx, cy, radius = 23.0 / 32, .5, 5.0 / 32
	const ymin, ymax = cy - radius, cy + radius
	var seeds intersect.SplineSeedReport
	add := func(face int, da, db geom.ParameterRectangle, axis int) bool {
		t.Helper()
		x, e := ap[face].patch.Trim(da)
		if e != nil {
			t.Fatal(e)
		}
		y, e := bp[1].patch.Trim(db)
		if e != nil {
			t.Fatal(e)
		}
		cert, e := geom.CertifySurfacePairTube(x, y, geom.SurfacePairTubeOptions{FreeAxis: axis})
		if e != nil {
			t.Fatal("wall/roof tube", da, db, axis, cert, e)
		}
		if !cert.ProvenBranch {
			return false
		}
		seeds.Attempts = append(seeds.Attempts, intersect.SplineSeedCell{Cell: geom.SurfaceSurfaceCell{DomainA: da, DomainB: db}, Tube: &cert})
		return true
	}
	var circle func(int, float64, float64, float64, float64, int)
	circle = func(axis int, lo, hi, sign, center float64, depth int) {
		m, n := center, cy
		if axis == 1 {
			m, n = n, m
		}
		coord := func(x float64) float64 { return n + sign*math.Sqrt(radius*radius-(x-m)*(x-m)) }
		y0, y1 := min(coord(lo), coord(hi)), max(coord(lo), coord(hi))
		if lo <= m && hi >= m {
			y0, y1 = min(y0, n+sign*radius), max(y1, n+sign*radius)
		}
		pad := min((hi-lo)/2, .015625)
		da := geom.ParameterRectangle{UMin: lo, UMax: hi, VMin: y0 - pad, VMax: y1 + pad}
		db := da
		db.UMin, db.UMax = lo-pad, hi+pad
		if axis == 1 {
			da = geom.ParameterRectangle{UMin: da.VMin, UMax: da.VMax, VMin: da.UMin, VMax: da.UMax}
			db = geom.ParameterRectangle{UMin: db.VMin, UMax: db.VMax, VMin: db.UMin, VMax: db.UMax}
		}
		if add(4, da, db, axis) {
			return
		}
		if depth == 8 {
			t.Fatal("no circle tube", da, db)
		}
		mid := lo/2 + hi/2
		circle(axis, lo, mid, sign, center, depth+1)
		circle(axis, mid, hi, sign, center, depth+1)
	}
	const first = 9.0 / 32
	for _, sign := range []float64{-1, 1} {
		circle(0, first-3.0/32, first+3.0/32, sign, first, 0)
		circle(1, cy-4.0/32, cy+4.0/32, sign, first, 0)
	}
	for _, sign := range []float64{-1, 1} {
		circle(0, cx-3.0/32, cx, sign, cx, 0)
	}
	circle(1, cy-4.0/32, cy+4.0/32, -1, cx, 0)
	g, e := intersect.JoinSplineTubeBranches(ctx, p, q, seeds, o.Branches)
	if e != nil || len(g.Branches) != 2 || g.Branches[0].Closed == g.Branches[1].Closed {
		t.Fatal("floor/roof branch graph", g, e)
	}
	var sections []SplineShellSection
	for i, branch := range g.Branches {
		s := SplineShellSection{FaceA: 4, FaceB: 1, Branch: i, Closed: branch.Closed}
		if i == 0 {
			s.TubeCells = seeds
		}
		if !branch.Closed {
			curves, e := intersect.SplineBranchCurves(ctx, p, q, seeds, branch, o.Branches.Curves)
			if e != nil {
				t.Fatal(e)
			}
			start, e := curves[0].EvaluatePair(0)
			if e != nil {
				t.Fatal(e)
			}
			end := func(y float64) SplineShellOpenEndpoint {
				return SplineShellOpenEndpoint{Operand: 0, Edge: SplineTubeEndpoint{Side: geom.SurfaceEdgeUMax,
					Hint: [4]*big.Rat{ratInset(cx), ratInset(y), ratInset(cx), ratInset(y)}}}
			}
			s.Start, s.End = end(ymin), end(ymax)
			if start.A.Y > cy {
				s.Start, s.End = s.End, s.Start
			}
		}
		sections = append(sections, s)
	}
	// On the right wall the exact model curve is x=23/32,
	// z=(5/32)^2-(y-1/2)^2. Use z at the terminals and y through
	// the central turn. All tube spans and terminal hints are dyadic.
	const left, right = cy - 3.0/32, cy + 3.0/32
	const top = radius * radius
	const joinZ = top - (3.0/32)*(3.0/32)
	seeds = intersect.SplineSeedReport{}
	// Reduce the size of each uniform z slab; a broad parameter-dependent
	// enclosure is unnecessary for this independently authored fixture.
	for _, ends := range [][2]float64{{ymin, cy - .125}, {cy - .125, left}, {right, cy + .125}, {cy + .125, ymax}} {
		y0, y1 := ends[0], ends[1]
		z0, z1 := top-(y0-cy)*(y0-cy), top-(y1-cy)*(y1-cy)
		pad := 1.0 / 64
		if !add(1, geom.ParameterRectangle{UMin: y0 - pad, UMax: y1 + pad, VMin: min(z0, z1), VMax: max(z0, z1)},
			geom.ParameterRectangle{UMin: cx - pad, UMax: cx + pad, VMin: y0 - pad, VMax: y1 + pad}, 1) {
			t.Fatal("wall arc z tube", ends)
		}
	}
	for _, ends := range [][2]float64{{left, cy}, {cy, right}} {
		pad := 1.0 / 64
		if !add(1, geom.ParameterRectangle{UMin: ends[0], UMax: ends[1], VMin: joinZ / 4, VMax: top + pad},
			geom.ParameterRectangle{UMin: cx - pad, UMax: cx + pad, VMin: ends[0] - pad, VMax: ends[1] + pad}, 0) {
			t.Fatal("wall arc y tube", ends)
		}
	}
	p, _ = intersectSplineFace(a.faces[1], ap[1].patch.Domain())
	g, e = intersect.JoinSplineTubeBranches(ctx, p, q, seeds, o.Branches)
	if e != nil || len(g.Branches) != 1 || g.Branches[0].Closed || len(g.Branches[0].Cells) != 6 {
		t.Fatal("wall/roof graph", g, e)
	}
	branch := g.Branches[0]
	ends := [2]SplineShellOpenEndpoint{}
	for i, y := range [2]float64{ymin, ymax} {
		if branch.Cells[0].Cell >= 2 {
			y = 2*cy - y
		}
		ends[i] = SplineShellOpenEndpoint{Operand: 0, Edge: SplineTubeEndpoint{Side: geom.SurfaceEdgeVMin,
			Hint: [4]*big.Rat{ratInset(y), ratInset(0), ratInset(cx), ratInset(y)}}}
	}
	sections = append(sections, SplineShellSection{FaceA: 1, FaceB: 1, TubeCells: seeds, Start: ends[0], End: ends[1]})
	return sections, nil
}
