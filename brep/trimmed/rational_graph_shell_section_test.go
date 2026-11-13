package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestRationalGraphSplineShellAffineExactTerminals(t *testing.T) {
	ctx := context.Background()
	s, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	// z=(1+x)/3 crosses four stock edges at points that cannot be
	// represented in binary64. A section does not need to alter the solid.
	q := [3]geom.Vec3{{X: -1}, {X: 2, Z: 1}, {X: -1, Y: 1}}
	r, e := s.SectionRationalGraphSplineShell(ctx, q, RationalGraphShellSectionOptions{VerifyModelEdges: true, Regions: &RationalGraphShellRegionOptions{}})
	if e != nil || !r.Complete || len(r.Wires) != 1 || len(r.Wires[0]) != 4 {
		t.Fatal("exact rational box section", r, e)
	}
	if len(r.Regions.Material) != 1 || len(r.Regions.Material[0].Holes) != 0 || r.Regions.Rings[0].Orientation != 1 {
		t.Fatal("exact nonbinary section does not prove the projected material", r.Regions)
	}
	seen := map[[2]int64]bool{}
	for _, b := range r.Branches {
		p, ok := b.Pieces[0].(ExactQuadraticUV)
		if !ok {
			t.Fatal("missing exact chart line", b)
		}
		exact, display := p.Endpoints()
		for i, end := range b.Ends {
			x, y, z := end.Point[0], end.Point[1], end.Point[2]
			if new(big.Rat).Mul(z, big.NewRat(3, 1)).Cmp(new(big.Rat).Add(x, big.NewRat(1, 1))) != 0 {
				t.Fatal("rounded exact endpoint", end)
			}
			seen[[2]int64{x.Num().Int64(), y.Num().Int64()}] = true
			u, v := ratInset(display[i].X), ratInset(display[i].Y)
			if exact[i][0].Cmp(u) == 0 && exact[i][1].Cmp(v) == 0 {
				t.Fatal("expected one chart coordinate to retain nonbinary data", exact[i], display[i])
			}
		}
	}
	if len(seen) != 4 {
		t.Fatal("missing corner columns", seen)
	}
	limited := RationalGraphShellSectionOptions{VerifyModelEdges: true, EdgeContacts: geom.ComposedPlaneContactsOptions{EventOptions: geom.ExactCurvePlaneEventOptions{MaxOperations: 1}}}
	if bad, e := s.SectionRationalGraphSplineShell(ctx, q, limited); e == nil || bad.Complete || len(bad.Branches) != 0 {
		t.Fatal("exhausted independent model-edge verification returned a wire", bad, e)
	}
	for _, c := range []struct {
		h    float64
		pass bool
	}{{2, true}, {1, false}} {
		p := [3]geom.Vec3{{Z: c.h}, {X: 1, Z: c.h}, {Y: 1, Z: c.h}}
		r, e := s.SectionRationalGraphSplineShell(ctx, p, RationalGraphShellSectionOptions{})
		if len(r.Branches) != 0 || len(r.Wires) != 0 || r.Complete != c.pass || (e == nil) != c.pass {
			t.Fatal("disjoint or unsupported coincident plane", c, r, e)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := s.SectionRationalGraphSplineShell(canceled, q, RationalGraphShellSectionOptions{}); !errors.Is(e, context.Canceled) || r.Complete {
		t.Fatal("canceled section published a result", r, e)
	}
}

func TestRationalGraphSplineShellThroughHoleRegions(t *testing.T) {
	ctx := context.Background()
	var surfaces []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	domain := geom.ParameterRectangle{UMax: 1, VMax: 1}
	k := []float64{0, 0, 1, 1}
	rect := func(p, u, v geom.Vec3) {
		t.Helper()
		s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{p, p.Add(v), p.Add(u), p.Add(u).Add(v)}, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		surfaces = append(surfaces, s)
		domains = append(domains, domain)
	}
	occupied := func(x, y int) bool { return x >= 0 && x < 3 && y >= 0 && y < 3 && (x != 1 || y != 1) }
	ex, ey, ez := geom.Vec3{X: 1}, geom.Vec3{Y: 1}, geom.Vec3{Z: 1}
	for x := range 3 {
		for y := range 3 {
			if !occupied(x, y) {
				continue
			}
			p := geom.Vec3{X: float64(x), Y: float64(y)}
			rect(p, ey, ex)
			rect(p.Add(ez), ex, ey)
			if !occupied(x-1, y) {
				rect(p, ez, ey)
			}
			if !occupied(x+1, y) {
				rect(p.Add(ex), ey, ez)
			}
			if !occupied(x, y-1) {
				rect(p, ex, ez)
			}
			if !occupied(x, y+1) {
				rect(p.Add(ey), ez, ex)
			}
		}
	}
	s, _, e := SewSplineRectangles(ctx, surfaces, domains, make([]bool, len(surfaces)), SewOptions{Tolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	z := .5
	p := [3]geom.Vec3{{Z: z}, {X: 1, Z: z}, {Y: 1, Z: z}}
	r, e := s.SectionRationalGraphSplineShell(ctx, p, RationalGraphShellSectionOptions{Regions: &RationalGraphShellRegionOptions{}})
	if e != nil || !r.Complete || len(r.Wires) != 2 || len(r.Regions.Material) != 1 {
		t.Fatal("section of the square tube", r, e)
	}
	m := r.Regions.Material[0]
	if len(m.Holes) != 1 || len(r.Wires[m.Outer]) != 12 || len(r.Wires[m.Holes[0]]) != 4 || r.Regions.Rings[m.Holes[0]].Parent != m.Outer {
		t.Fatal("square-tube void was not retained as a hole", r.Regions)
	}
	a, e := r.Regions.BoundArea(ctx, big.NewRat(1, 1<<20))
	if e != nil || a.ModelScaleSquared.Cmp(big.NewRat(1, 1)) != 0 || a.Projection.Lower.Cmp(big.NewRat(8, 1)) != 0 || a.Projection.Upper.Cmp(a.Projection.Lower) != 0 {
		t.Fatal("exact square-ring area", a, e)
	}
	for _, tt := range []struct {
		x, y float64
		want trimvalidate.Location
	}{{.5, 1.5, trimvalidate.Inside}, {1.5, 1.5, trimvalidate.Outside}, {-1, 0, trimvalidate.Outside}, {1, 1.5, trimvalidate.Boundary}} {
		loc, proof, e := r.Regions.LocateRational(ctx, [3]*big.Rat{ratInset(tt.x), ratInset(tt.y), big.NewRat(1, 2)})
		if e != nil || proof.Status != trimvalidate.Certified || loc != tt.want {
			t.Fatal("square-tube section material query", tt, loc, proof, e)
		}
	}
	if loc, _, e := r.Regions.LocateRational(ctx, [3]*big.Rat{big.NewRat(1, 2), big.NewRat(3, 2), big.NewRat(1, 3)}); !errors.Is(e, geom.ErrDomain) || loc != trimvalidate.Unknown {
		t.Fatal("section query accepted an off-plane point", loc, e)
	}
	// The cap chart swaps model axes, so its native normal points down.
	// Return a correctly reversed upward face with the same physical hole.
	support, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{
		{Z: z}, {X: 4, Z: z}, {Y: 4, Z: z}, {X: 4, Y: 4, Z: z},
	}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	r.Regions.Material[0].Holes[0] = -1 // diagnostic mutation cannot alter proof
	caps, e := r.Regions.BuildCaps(ctx, support, domain, RationalGraphSectionCapOptions{})
	if e != nil || len(caps) != 1 {
		t.Fatal("square-tube cap", caps, e)
	}
	cap := caps[0]
	if !cap.Boundary.Face.Reversed || len(cap.Boundary.Face.Outer) != 12 || len(cap.Boundary.Face.Holes) != 1 || len(cap.Boundary.Face.Holes[0]) != 4 || len(cap.Edges) != 16 {
		t.Fatal("cap chart orientation or hole ownership changed", cap)
	}
	for i, x := range cap.Edges {
		if !x.Reversed || r.Branches[x.Branch].Face != x.Face || x.Piece != 0 {
			t.Fatal("cap edge ownership", i, x)
		}
	}
	if loc, report, e := LocateRationalInFaceTrim(ctx, cap.Boundary.Face, [2]*big.Rat{big.NewRat(3, 8), big.NewRat(3, 8)}, trimvalidate.Options{}); e != nil || report.Status != trimvalidate.Certified || loc != trimvalidate.Outside {
		t.Fatal("cap trim filled the void", loc, report, e)
	}
	split, e := s.SplitRationalGraphSplineShell(ctx, p, support, domain, RationalGraphShellSplitOptions{})
	if e != nil {
		t.Fatal("closed square-tube halves", e)
	}
	for side, half := range split.Halves {
		fs := half.Solid.Faces()
		if len(half.CapFaces) != 1 || len(half.CapEdges[0]) != 16 || len(half.Solid.Topology().Shells) != 1 ||
			len(fs[half.CapFaces[0]].Holes) != 1 || fs[half.CapFaces[0]].Reversed != (side == 0) {
			t.Fatal("clipped tube lost topology or cap normal", side)
		}
		mp, e := half.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(mp.Volume-4) > 1e-8 {
			t.Fatal("independent square tube half volume", side, mp, e)
		}
		owned := map[int]bool{}
		for _, row := range append(append([][]int(nil), half.CapEdges...), half.SourceEdges...) {
			for _, id := range row {
				if owned[id] {
					t.Fatal("ambiguous split model-edge ancestry", side, id)
				}
				owned[id] = true
			}
		}
		if len(owned) != len(half.Solid.Edges()) {
			t.Fatal("incomplete split model-edge ancestry", side)
		}
	}
}
