package intersect

import (
	"context"
	"math"
	"testing"

	"plainsolid/geom"
)

func mixedTubeFixture(t *testing.T) (Patch, Patch, SplineSeedReport) {
	t.Helper()
	k := []float64{0, 0, 1, 1}
	a, _ := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k, k, nil)
	b, _ := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Y: .5, Z: 1}, {Y: .5, Z: -1}, {X: 1, Y: .5, Z: 1}, {X: 1, Y: .5, Z: -1}}, k, k, nil)
	p := func(s *geom.BSplineSurface) Patch { return Patch{s, geom.Vec2{}, geom.Vec2{X: 1, Y: 1}} }
	pa, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	pb, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	full := geom.ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 1}
	x, y := full, full
	x.UMin, x.UMax = .25, .5
	y.UMin, y.UMax = .5, .75
	var seeds SplineSeedReport
	for i, cell := range []geom.SurfaceSurfaceCell{{DomainA: x, DomainB: full}, {DomainA: full, DomainB: y}} {
		qa, _ := pa.Trim(cell.DomainA)
		qb, _ := pb.Trim(cell.DomainB)
		c, e := geom.CertifySurfacePairTube(qa, qb, geom.SurfacePairTubeOptions{FreeAxis: i * 2})
		if e != nil || !c.ProvenBranch {
			t.Fatal(c, e)
		}
		seeds.Attempts = append(seeds.Attempts, SplineSeedCell{Cell: cell, Tube: &c})
	}
	return p(a), p(b), seeds
}

func TestJoinSplineTubeBranches(t *testing.T) {
	ctx := context.Background()
	a, b, seeds := mixedTubeFixture(t)
	r, e := JoinSplineTubeBranches(ctx, a, b, seeds, TubeBranchOptions{})
	if e != nil || r.Truncated || len(r.Branches) != 1 {
		t.Fatal(r, e)
	}
	x := r.Branches[0]
	if x.Closed || x.StartUnresolved || x.EndUnresolved || len(x.Cells) != 2 ||
		x.Cells[0].FreeAxis == x.Cells[1].FreeAxis {
		t.Fatal(x)
	}
	segments, e := SplineBranchCurves(ctx, a, b, seeds, x, TubeCurveOptions{})
	if e != nil {
		t.Fatal(e)
	}
	as, bs := a.Surface.(*geom.BSplineSurface), b.Surface.(*geom.BSplineSurface)
	if !segments[0].TrimA().CertifiesPairedTube(segments[0].TrimB(), as, bs) ||
		segments[0].TrimA().CertifiesPairedTube(segments[1].TrimB(), as, bs) ||
		segments[0].TrimA().CertifiesPairedTube(segments[0].TrimB(), bs, as) {
		t.Fatal("paired tube support or segment identity")
	}
	for _, chart := range []func(TubeSegment) TubeTrim{TubeSegment.TrimA, TubeSegment.TrimB} {
		x, e := chart(segments[0]).EvalUV(1)
		y, f := chart(segments[1]).EvalUV(0)
		if e != nil || f != nil || x != y || math.Abs(x.X-.5) > 1e-12 {
			t.Fatal(x, y, e, f)
		}
	}
	seeds.Attempts = append(seeds.Attempts, seeds.Attempts[1])
	r, e = JoinSplineTubeBranches(ctx, a, b, seeds, TubeBranchOptions{})
	if e != nil || len(r.Branches) != 3 {
		t.Fatal(r, e)
	}
	for _, x := range r.Branches {
		if len(x.Cells) != 1 || !x.StartUnresolved && !x.EndUnresolved {
			t.Fatal(x)
		}
	}
	r, e = JoinSplineTubeBranches(ctx, a, b, seeds, TubeBranchOptions{MaxComparisons: 1})
	if e != nil || !r.Truncated || len(r.Branches) != 3 {
		t.Fatal(r, e)
	}
}

func TestJoinClosedSplineTubeBranch(t *testing.T) {
	k1, k2 := []float64{0, 0, 1, 1}, []float64{0, 0, 0, 1, 1, 1}
	a, _ := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k1, k1, nil)
	q := [3]float64{.25, -.25, .25}
	const r = 5.0 / 16
	var p []geom.Vec3
	for i := range 3 {
		for j := range 3 {
			p = append(p, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: q[i] + q[j] - r*r})
		}
	}
	b, _ := geom.NewBSplineSurface(2, 2, 3, 3, p, k2, k2, nil)
	pa, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	pb, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	patch := func(s *geom.BSplineSurface) Patch { return Patch{s, geom.Vec2{}, geom.Vec2{X: 1, Y: 1}} }
	var seeds SplineSeedReport
	var add func(int, float64, float64, float64, int)
	add = func(axis int, lo, hi, sign float64, depth int) {
		t.Helper()
		coord := func(x float64) float64 { return .5 + sign*math.Sqrt(r*r-(x-.5)*(x-.5)) }
		y0, y1 := min(coord(lo), coord(hi)), max(coord(lo), coord(hi))
		if lo <= .5 && hi >= .5 {
			y0, y1 = min(y0, .5+sign*r), max(y1, .5+sign*r)
		}
		pad := (hi - lo) / 2
		pad = min(pad, .03125)
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
				t.Fatal("no local proof", d, g)
			}
			m := (lo + hi) / 2
			add(axis, lo, m, sign, depth+1)
			add(axis, m, hi, sign, depth+1)
			return
		}
		seeds.Attempts = append(seeds.Attempts, SplineSeedCell{Cell: geom.SurfaceSurfaceCell{DomainA: d, DomainB: g}, Tube: &c})
	}
	for _, sign := range []float64{-1, 1} {
		add(0, .5-3.0/16, .5+3.0/16, sign, 0)
		add(1, .5-4.0/16, .5+4.0/16, sign, 0)
	}
	ctx := context.Background()
	report, e := JoinSplineTubeBranches(ctx, patch(a), patch(b), seeds, TubeBranchOptions{})
	if e != nil || report.Truncated || len(report.Branches) != 1 || !report.Branches[0].Closed ||
		len(report.Branches[0].Cells) != len(seeds.Attempts) {
		t.Fatal(report, e)
	}
	segments, e := SplineBranchCurves(ctx, patch(a), patch(b), seeds, report.Branches[0], TubeCurveOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i, x := range segments {
		y := segments[(i+1)%len(segments)]
		v, e := x.TrimA().EvalUV(1)
		w, f := y.TrimA().EvalUV(0)
		if e != nil || f != nil || v != w {
			t.Fatal(v, w, e, f)
		}
		v, e = x.TrimB().EvalUV(.5)
		if e != nil || math.Abs(math.Hypot(v.X-.5, v.Y-.5)-r) > 1e-8 {
			t.Fatal(v, e)
		}
	}
}
