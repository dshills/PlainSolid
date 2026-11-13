package trimmed

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func transverseSplineCaps(t *testing.T) (Solid, Solid, intersect.SplineSeedReport) {
	t.Helper()
	ctx := context.Background()
	const radius = 5.0 / 16
	build := func(x0, x1, floor float64, dome bool) (Solid, *geom.BSplineSurface) {
		t.Helper()
		var surfaces []*geom.BSplineSurface
		var domains []geom.ParameterRectangle
		var reversed []bool
		mk := func(nu, nv int, f func(i, j int) geom.Vec3, rev bool) {
			t.Helper()
			var poles []geom.Vec3
			for i := range nu {
				for j := range nv {
					poles = append(poles, f(i, j))
				}
			}
			knots := func(n int) []float64 {
				k := make([]float64, 2*n)
				for i := n; i < len(k); i++ {
					k[i] = x1
				}
				for i := 0; i < n; i++ {
					k[i] = x0
				}
				return k
			}
			s, e := geom.NewBSplineSurface(nu-1, nv-1, nu, nv, poles, knots(nu), knots(nv), nil)
			if e != nil {
				t.Fatal(e)
			}
			surfaces, reversed = append(surfaces, s), append(reversed, rev)
			domains = append(domains, geom.ParameterRectangle{})
		}
		z := func(i, j int) float64 {
			if !dome {
				return 1
			}
			q := (x0 - .5) * (x0 - .5)
			v := [3]float64{q, -q, q}
			return radius*radius - v[i] - v[j]
		}
		n := 2
		if dome {
			n = 3
		}
		at := func(i, n int) float64 { return x0 + (x1-x0)*float64(i)/float64(n-1) }
		mk(2, 2, func(i, j int) geom.Vec3 { return geom.Vec3{X: at(i, 2), Y: at(j, 2), Z: floor} }, true)
		mk(n, n, func(i, j int) geom.Vec3 { return geom.Vec3{X: at(i, n), Y: at(j, n), Z: z(i, j)} }, false)
		for _, edge := range []int{0, n - 1} {
			mk(n, 2, func(i, j int) geom.Vec3 {
				return geom.Vec3{X: at(edge, n), Y: at(i, n), Z: []float64{floor, z(edge, i)}[j]}
			}, edge == 0)
		}
		for _, edge := range []int{0, n - 1} {
			mk(n, 2, func(i, j int) geom.Vec3 {
				return geom.Vec3{X: at(i, n), Y: at(edge, n), Z: []float64{floor, z(i, edge)}[j]}
			}, edge != 0)
		}
		s, _, e := SewSplineRectangles(ctx, surfaces, domains, reversed, SewOptions{Tolerance: 1e-8})
		if e != nil {
			t.Fatal(e)
		}
		if dome {
			return s, surfaces[1]
		}
		return s, surfaces[0]
	}
	a, aFace := build(0, 1, 0, false)
	b, bFace := build(.125, .875, -.5, true)
	pa, _ := geom.NewSurfacePatch(aFace, geom.SurfacePatchOptions{})
	pb, _ := geom.NewSurfacePatch(bFace, geom.SurfacePatchOptions{})
	var hints intersect.SplineSeedReport
	var add func(int, float64, float64, float64, int)
	add = func(axis int, lo, hi, sign float64, depth int) {
		coord := func(x float64) float64 { return .5 + sign*math.Sqrt(radius*radius-(x-.5)*(x-.5)) }
		y0, y1 := min(coord(lo), coord(hi)), max(coord(lo), coord(hi))
		if lo <= .5 && hi >= .5 {
			y0, y1 = min(y0, .5+sign*radius), max(y1, .5+sign*radius)
		}
		pad := min((hi-lo)/2, .03125)
		d := geom.ParameterRectangle{UMin: lo, UMax: hi, VMin: y0 - pad, VMax: y1 + pad}
		g := d
		g.UMin, g.UMax = lo-pad, hi+pad
		if axis == 1 {
			d.UMin, d.UMax, d.VMin, d.VMax = d.VMin, d.VMax, d.UMin, d.UMax
			g.UMin, g.UMax, g.VMin, g.VMax = g.VMin, g.VMax, g.UMin, g.UMax
		}
		x, e := pa.Trim(d)
		if e != nil {
			t.Fatal(e)
		}
		y, e := pb.Trim(g)
		if e != nil {
			t.Fatal(e)
		}
		c, e := geom.CertifySurfacePairTube(x, y, geom.SurfacePairTubeOptions{FreeAxis: axis})
		if e != nil {
			t.Fatal(e)
		}
		if !c.ProvenBranch {
			if depth == 8 {
				t.Fatal("no tube", d, g)
			}
			m := (lo + hi) / 2
			add(axis, lo, m, sign, depth+1)
			add(axis, m, hi, sign, depth+1)
			return
		}
		hints.Attempts = append(hints.Attempts, intersect.SplineSeedCell{Cell: geom.SurfaceSurfaceCell{DomainA: d, DomainB: g}, Tube: &c})
	}
	for _, sign := range []float64{-1, 1} {
		add(0, .5-3.0/16, .5+3.0/16, sign, 0)
		add(1, .5-4.0/16, .5+4.0/16, sign, 0)
	}
	return a, b, hints
}

func TestCombineTransverseSplineShells(t *testing.T) {
	ctx := context.Background()
	a, b, _ := transverseSplineCaps(t)
	o := SplineShellTransverseOptions{}
	o.Trace.PlaneFallback.Enabled = true
	o.Trace.PlaneFallback.UCuts = []float64{.25, .3125, .6875, .75}
	o.Trace.PlaneFallback.VCuts = o.Trace.PlaneFallback.UCuts
	o.Trace.PlaneFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .25
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	s, h, e := CombineTransverseSplineShells(ctx, a, b, 0, 1, parametric.Intersection, o)
	if e != nil || !h.Trace.Complete || len(h.Trace.Coverage.BoundaryCovered) == 0 || len(s.faces) != 2 || len(h.SectionEdges) == 0 || s.Topology().EulerCharacteristic != 2 {
		t.Fatal(len(s.faces), h.Trace.Issues, len(h.Trace.Coverage.Isolation.Candidates), len(h.Trace.Coverage.Unresolved), e)
	}
	if testing.Short() {
		return
	}
	var data bytes.Buffer
	if e := WriteJSON(&data, s); e != nil {
		t.Fatal(e)
	}
	restored, e := ReadJSONContext(ctx, &data, JSONLimits{})
	if e != nil || restored.Topology().EulerCharacteristic != 2 || len(restored.faces) != 2 {
		t.Fatal(restored.Topology(), e)
	}
	m, e := s.refine(ctx, 1, 0, 0)
	want := math.Pi * math.Pow(5.0/16, 4) / 2
	if e != nil || m.ValidateSolid() != nil || math.Abs(m.SignedVolume()-want) > .004 {
		t.Fatal(m.SignedVolume(), want, e)
	}
}

func TestCombineTransverseSplineShellsAdditionalOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("closed transverse shell assembly")
	}
	ctx := context.Background()
	a, b, hints := transverseSplineCaps(t)
	o := SplineShellTransverseOptions{TubeCells: hints}
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .25
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	intersection := math.Pi * math.Pow(5.0/16, 4) / 2
	volumeB := .75 * .75 * (.5 + math.Pow(5.0/16, 2) - 2*math.Pow(.375, 2)/3)
	for _, tc := range []struct {
		op     parametric.BooleanOp
		volume float64
	}{{parametric.Union, 1 + volumeB - intersection}, {parametric.Difference, 1 - intersection}} {
		s, h, e := CombineTransverseSplineShells(ctx, a, b, 0, 1, tc.op, o)
		if e != nil || !h.Trace.Complete || len(h.SectionEdges) == 0 || len(s.Topology().Shells) != 1 || s.Topology().EulerCharacteristic != 2 {
			t.Fatal(tc, h.Trace.Issues, e)
		}
		m, e := s.refine(ctx, 1, 0, 0)
		if e != nil || m.ValidateSolid() != nil || math.Abs(m.SignedVolume()-tc.volume) > .025 {
			t.Fatal(tc, m.SignedVolume(), e)
		}
	}
}

func TestTransverseSplineContourArrangementRejectsOverlappingCharts(t *testing.T) {
	ctx := context.Background()
	a, b, hints := transverseSplineCaps(t)
	// Two separated instances give a controlled arrangement fixture with
	// independently verified sections. The public API still requires a
	// single embedded shell per operand; this isolates fragment sewing.
	shift := geom.Identity()
	shift[0][3] = 2
	pa, e := a.Transformed(shift)
	if e != nil {
		t.Fatal(e)
	}
	pb, e := b.Transformed(shift)
	if e != nil {
		t.Fatal(e)
	}
	certify := func(x, y Solid, m geom.Affine) []intersect.TubeSegment {
		t.Helper()
		makePatch := func(s Solid, i int) intersect.Patch {
			base, matrix := parametric.SurfaceMapping(s.faces[i].Surface)
			sp := base.(*geom.BSplineSurface)
			u, v, x, y := sp.Domain()
			if matrix != geom.Identity() {
				p, e := geom.NewSurfacePatch(sp, geom.SurfacePatchOptions{})
				if e != nil {
					t.Fatal(e)
				}
				p, e = p.Transformed(m)
				if e != nil {
					t.Fatal(e)
				}
				sp, e = p.ToBSplineSurface()
				if e != nil {
					t.Fatal(e)
				}
			}
			return intersect.Patch{Surface: sp, Min: geom.Vec2{X: u, Y: x}, Max: geom.Vec2{X: v, Y: y}}
		}
		p, q := makePatch(x, 0), makePatch(y, 1)
		r, e := intersect.JoinSplineTubeBranches(ctx, p, q, hints, intersect.TubeBranchOptions{})
		if e != nil || len(r.Branches) != 1 || !r.Branches[0].Closed {
			t.Fatal(r, e)
		}
		c, e := intersect.SplineBranchCurves(ctx, p, q, hints, r.Branches[0], intersect.TubeCurveOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return c
	}
	x, y := certify(a, b, geom.Identity()), certify(pa, pb, shift)
	// Their model points are distinct, but both sections occupy the same
	// parameter rectangle. The multi-contour path must not accept these as
	// disjoint in one face chart just because they are separated in space.
	r, e := transverseRings(ctx, [][]intersect.TubeSegment{x, y},
		geom.ParameterRectangle{UMax: 1, VMax: 1}, geom.ParameterRectangle{UMin: .125, UMax: .875, VMin: .125, VMax: .875})
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e := transverseRingNesting(ctx, r, 0); e == nil {
		t.Fatal("accepted intersecting contour bounds in both charts")
	}
}

func twoPeakSplineRoof(t *testing.T) (Solid, *geom.BSplineSurface) {
	t.Helper()
	ctx := context.Background()
	const r = 5.0 / 32
	const height = -1
	// Author a roof from two C0 quadratic graph pieces meeting at x=.5.
	// Their intersections with the box floor are disjoint circles.
	ux := []float64{.0625, .0625, .0625, .5, .5, .9375, .9375, .9375}
	vy := []float64{.25, .25, .25, .75, .75, .75}
	xs := []float64{.0625, .28125, .5, .71875, .9375}
	ys := []float64{.25, .5, .75}
	var z [5][3]float64
	for i := range xs {
		x := (.21875) * (.21875)
		if i%2 == 1 {
			x = -x
		}
		for j := range ys {
			y := .0625
			if j == 1 {
				y = -y
			}
			z[i][j] = r*r - x - y
		}
	}
	var surfaces []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reversed []bool
	add := func(du, dv, nu, nv int, ku, kv []float64, poles []geom.Vec3, flip bool) {
		t.Helper()
		s, e := geom.NewBSplineSurface(du, dv, nu, nv, poles, ku, kv, nil)
		if e != nil {
			t.Fatal(e)
		}
		surfaces = append(surfaces, s)
		domains = append(domains, geom.ParameterRectangle{})
		reversed = append(reversed, flip)
	}
	ku := []float64{.0625, .0625, .9375, .9375}
	kv := []float64{.25, .25, .75, .75}
	linear := []float64{0, 0, 1, 1}
	add(1, 1, 2, 2, ku, kv, []geom.Vec3{{X: xs[0], Y: ys[0], Z: height}, {X: xs[0], Y: ys[2], Z: height},
		{X: xs[4], Y: ys[0], Z: height}, {X: xs[4], Y: ys[2], Z: height}}, true)
	var roof []geom.Vec3
	for i, x := range xs {
		for j, y := range ys {
			roof = append(roof, geom.Vec3{X: x, Y: y, Z: z[i][j]})
		}
	}
	add(2, 2, 5, 3, ux, vy, roof, false)
	for _, i := range []int{0, 4} {
		var p []geom.Vec3
		for j, y := range ys {
			p = append(p, geom.Vec3{X: xs[i], Y: y, Z: height}, geom.Vec3{X: xs[i], Y: y, Z: z[i][j]})
		}
		add(2, 1, 3, 2, vy, linear, p, i == 0)
	}
	for _, j := range []int{0, 2} {
		var p []geom.Vec3
		for i, x := range xs {
			p = append(p, geom.Vec3{X: x, Y: ys[j], Z: height}, geom.Vec3{X: x, Y: ys[j], Z: z[i][j]})
		}
		add(2, 1, 5, 2, ux, linear, p, j != 0)
	}
	b, _, e := SewSplineRectangles(ctx, surfaces, domains, reversed, SewOptions{Tolerance: 1e-8})
	if e != nil {
		t.Fatal(e)
	}
	return b, surfaces[1]
}

func TestTransverseSplineTwoClosedContours(t *testing.T) {
	ctx := context.Background()
	const r = 5.0 / 32
	centers := []float64{.28125, .71875}
	a, _, _ := transverseSplineCaps(t)
	b, roof := twoPeakSplineRoof(t)
	pa, _ := geom.NewSurfacePatch(a.faces[0].Surface.(*geom.BSplineSurface), geom.SurfacePatchOptions{})
	pb, _ := geom.NewSurfacePatch(roof, geom.SurfacePatchOptions{})
	var hints intersect.SplineSeedReport
	for _, cx := range centers {
		var addTube func(int, float64, float64, float64, int)
		addTube = func(axis int, lo, hi, sign float64, depth int) {
			m, n := cx, .5
			if axis == 1 {
				m, n = n, m
			}
			coord := func(x float64) float64 { return n + sign*math.Sqrt(r*r-(x-m)*(x-m)) }
			y0, y1 := min(coord(lo), coord(hi)), max(coord(lo), coord(hi))
			if lo <= m && hi >= m {
				y0, y1 = min(y0, n+sign*r), max(y1, n+sign*r)
			}
			pad := min((hi-lo)/2, .015625)
			d := geom.ParameterRectangle{UMin: lo, UMax: hi, VMin: y0 - pad, VMax: y1 + pad}
			g := d
			g.UMin, g.UMax = lo-pad, hi+pad
			if axis == 1 {
				d = geom.ParameterRectangle{UMin: d.VMin, UMax: d.VMax, VMin: d.UMin, VMax: d.UMax}
				g = geom.ParameterRectangle{UMin: g.VMin, UMax: g.VMax, VMin: g.UMin, VMax: g.UMax}
			}
			x, e := pa.Trim(d)
			if e != nil {
				t.Fatal(e)
			}
			y, e := pb.Trim(g)
			if e != nil {
				t.Fatal(e)
			}
			c, e := geom.CertifySurfacePairTube(x, y, geom.SurfacePairTubeOptions{FreeAxis: axis})
			if e != nil {
				t.Fatal(e)
			}
			if !c.ProvenBranch {
				if depth == 8 {
					t.Fatal("no tube", d, g)
				}
				mid := (lo + hi) / 2
				addTube(axis, lo, mid, sign, depth+1)
				addTube(axis, mid, hi, sign, depth+1)
				return
			}
			hints.Attempts = append(hints.Attempts, intersect.SplineSeedCell{Cell: geom.SurfaceSurfaceCell{DomainA: d, DomainB: g}, Tube: &c})
		}
		for _, sign := range []float64{-1, 1} {
			addTube(0, cx-3.0/32, cx+3.0/32, sign, 0)
			addTube(1, .5-4.0/32, .5+4.0/32, sign, 0)
		}
	}
	o := SplineShellTransverseOptions{TubeCells: hints}
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .125
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	s, h, e := CombineTransverseSplineShells(ctx, a, b, 0, 1, parametric.Intersection, o)
	if e != nil || !h.Trace.Complete || len(h.Trace.Curves) != 2 || len(s.Topology().Shells) != 2 || len(s.faces) != 4 {
		t.Fatal(len(s.faces), h.Trace.Issues, len(h.Trace.Coverage.Isolation.Candidates), h.Trace.Coverage.Unresolved, e)
	}
	if len(h.FacesA[0]) != 2 || len(h.FacesB[1]) != 2 || len(h.SectionEdges) < 6 {
		t.Fatal(h.FacesA, h.FacesB, h.SectionEdges)
	}
	if testing.Short() {
		return
	}
	for _, op := range []parametric.BooleanOp{parametric.Union, parametric.Difference} {
		s, h, e := CombineTransverseSplineShells(ctx, a, b, 0, 1, op, o)
		if e != nil || !h.Trace.Complete || len(s.Topology().Shells) != 1 {
			t.Fatal(op, h.Trace.Issues, e)
		}
		if len(h.FacesA[0]) != 1 || len(s.faces[h.FacesA[0][0]].Holes) != 2 {
			t.Fatal(op, h.FacesA)
		}
		wantEuler := 2
		if op == parametric.Union {
			// Joining the two solids at two disjoint disks produces a handle.
			wantEuler = 0
		}
		if top := s.Topology(); top.EulerCharacteristic != wantEuler {
			t.Fatal(op, top)
		}
	}
}

func TestTransverseSplineNestedArrangement(t *testing.T) {
	ctx := context.Background()
	// Build nested contours independently, on two surfaces sharing the
	// same identity XY projection. This exercises winding classification
	// without requiring full shell fixtures for both source supports.
	a, b, hints := transverseSplineCaps(t)
	patch := func(s Solid, i int) intersect.Patch {
		p := s.faces[i].Surface.(*geom.BSplineSurface)
		u, v, x, y := p.Domain()
		return intersect.Patch{Surface: p, Min: geom.Vec2{X: u, Y: x}, Max: geom.Vec2{X: v, Y: y}}
	}
	x, y := patch(a, 0), patch(b, 1)
	get := func(x, y intersect.Patch, hints intersect.SplineSeedReport) []intersect.TubeSegment {
		t.Helper()
		g, e := intersect.JoinSplineTubeBranches(ctx, x, y, hints, intersect.TubeBranchOptions{})
		if e != nil || len(g.Branches) != 1 || !g.Branches[0].Closed {
			t.Fatal(g, e)
		}
		c, e := intersect.SplineBranchCurves(ctx, x, y, hints, g.Branches[0], intersect.TubeCurveOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return c
	}
	outer := get(x, y, hints)
	transform := geom.Identity()
	transform[0][0], transform[1][1], transform[2][2] = .5, .5, .25
	transform[0][3], transform[1][3] = .25, .25
	scale := func(d geom.ParameterRectangle) geom.ParameterRectangle {
		return geom.ParameterRectangle{UMin: .25 + d.UMin/2, UMax: .25 + d.UMax/2, VMin: .25 + d.VMin/2, VMax: .25 + d.VMax/2}
	}
	rescale := func(p intersect.Patch) intersect.Patch {
		t.Helper()
		a, e := geom.NewSurfacePatch(p.Surface.(*geom.BSplineSurface), geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		a, e = a.Transformed(transform)
		if e != nil {
			t.Fatal(e)
		}
		d := scale(a.Domain())
		a, e = a.Reparameterize(d, false, false, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		b, e := a.ToBSplineSurface()
		if e != nil {
			t.Fatal(e)
		}
		return intersect.Patch{Surface: b, Min: geom.Vec2{X: d.UMin, Y: d.VMin}, Max: geom.Vec2{X: d.UMax, Y: d.VMax}}
	}
	x, y = rescale(x), rescale(y)
	for i, c := range hints.Attempts {
		d, e := scale(c.Cell.DomainA), scale(c.Cell.DomainB)
		hints.Attempts[i].Cell.DomainA, hints.Attempts[i].Cell.DomainB = d, e
		p := *c.Tube
		p.FreeParameter.Min = .25 + p.FreeParameter.Min/2
		p.FreeParameter.Max = .25 + p.FreeParameter.Max/2
		hints.Attempts[i].Tube = &p
	}
	inner := get(x, y, hints)
	r, e := transverseRings(ctx, [][]intersect.TubeSegment{outer, inner}, geom.ParameterRectangle{UMax: 1, VMax: 1},
		geom.ParameterRectangle{UMin: .125, UMax: .875, VMin: .125, VMax: .875})
	if e != nil {
		t.Fatal(e)
	}
	for chart := range 2 {
		parent, depth, e := transverseRingNesting(ctx, r, chart)
		if e != nil || parent[0] != -1 || parent[1] != 0 || depth[0] != 0 || depth[1] != 1 {
			t.Fatal(chart, parent, depth, e)
		}
	}
}
