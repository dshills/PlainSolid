package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestRationalGraphSplineShellThroughHoleRegions(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	s := makeSquareRationalGraphTube(t)
	domain := geom.ParameterRectangle{UMax: 1, VMax: 1}
	k := []float64{0, 0, 1, 1}
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
	checkSquareRationalGraphTubeSection(t, s, r, k, domain, p, z)
}

func makeSquareRationalGraphTube(t *testing.T) Solid {
	return makeWarpedSquareRationalGraphTube(t, 0)
}

func makeWarpedSquareRationalGraphTube(t *testing.T, warp float64) Solid {
	t.Helper()
	ctx := context.Background()
	var surfaces []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	domain := geom.ParameterRectangle{UMax: 1, VMax: 1}
	k := []float64{0, 0, 1, 1}
	rect := func(p, u, v geom.Vec3) {
		t.Helper()
		degree, count, knots := 1, 2, k
		controls := []geom.Vec3{p, p.Add(v), p.Add(u), p.Add(u).Add(v)}
		if warp != 0 {
			degree, count, knots = 2, 3, []float64{0, 0, 0, 1, 1, 1}
			controls = nil
			// Apply the exact shear Y <- Y + warp*X² to every
			// chart by converting the quadratic to Bernstein controls.
			// The tube fixture never has both U and V changing X.
			for i := 0; i < count; i++ {
				for j := 0; j < count; j++ {
					a, b := float64(i)/2, float64(j)/2
					q := p.Add(u.Scale(a)).Add(v.Scale(b))
					qx2 := p.X*p.X + 2*p.X*(u.X*a+v.X*b) + u.X*u.X*math.Floor(a) + v.X*v.X*math.Floor(b)
					q.Y += warp * qx2
					controls = append(controls, q)
				}
			}
		}
		s, e := geom.NewBSplineSurface(degree, degree, count, count, controls, knots, knots, nil)
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
	return s
}

func checkSquareRationalGraphTubeSection(t *testing.T, s Solid, r RationalGraphShellSection, k []float64, domain geom.ParameterRectangle, p [3]geom.Vec3, z float64) {
	t.Helper()
	ctx := context.Background()
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
	bandPoint := func(z float64) [3]geom.Vec3 {
		return [3]geom.Vec3{{Z: z}, {X: 1, Z: z}, {Y: 1, Z: z}}
	}
	// Exercise both cap wires in both planes on a constant square annulus.
	upper, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{
		{Z: .75}, {X: 4, Z: .75}, {Y: 4, Z: .75}, {X: 4, Y: 4, Z: .75},
	}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	band, e := s.ClipParallelRationalGraphSplineShell(ctx, [2][3]geom.Vec3{bandPoint(.5), bandPoint(.75)},
		[2]geom.Surface{support, upper}, [2]geom.ParameterRectangle{domain, domain}, RationalGraphParallelBandOptions{})
	if e != nil || len(band.CapFaces[0]) != 1 || len(band.CapFaces[1]) != 1 || len(band.CapEdges[0]) != 16 || len(band.CapEdges[1]) != 16 {
		t.Fatal("square tube band loses cap loops", e, band.CapFaces)
	}
	mp, e := band.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(mp.Volume-2) > 1e-8 {
		t.Fatal("independent square tube band volume", e, mp.Volume)
	}
	got := map[int]bool{}
	for _, row := range append(append([][]int(nil), band.CapEdges[:]...), band.SourceEdges...) {
		for _, id := range row {
			if got[id] {
				t.Fatal("ambiguous two-plane edge ancestry", id)
			}
			got[id] = true
		}
	}
	if len(got) != len(band.Solid.edges) {
		t.Fatal("incomplete two-plane edge ancestry")
	}
}
