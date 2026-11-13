package intersect

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/geom"
)

func TestFindSplineSeeds(t *testing.T) {
	makePatch := func(f func(x, y float64) geom.Vec3) Patch {
		t.Helper()
		p := []geom.Vec3{f(0, 0), f(0, 1), f(1, 0), f(1, 1)}
		k := []float64{0, 0, 1, 1}
		s, e := geom.NewBSplineSurface(1, 1, 2, 2, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return Patch{s, geom.Vec2{}, geom.Vec2{X: 1, Y: 1}}
	}
	a := makePatch(func(x, y float64) geom.Vec3 { return geom.Vec3{X: x, Y: y} })
	b := makePatch(func(x, y float64) geom.Vec3 { return geom.Vec3{X: .4, Y: x, Z: y - .6} })
	r, e := FindSplineSeeds(context.Background(), a, b, SplineSeedOptions{Sections: true, Covering: geom.SurfaceSurfaceIsolationOptions{ParameterTolerance: .25}})
	if e != nil || r.Truncated || !r.Covering.Complete || r.Unattempted != 0 {
		t.Fatal(r, e)
	}
	n, proofs := 0, 0
	for _, c := range r.Attempts {
		if !c.Transversality.ProvenTransverse || c.Sample == nil {
			t.Fatal(c)
		}
		p := c.Sample.Point
		if math.Abs(p.X-.4) > 1e-8 || math.Abs(p.Z) > 1e-8 || c.Sample.Residual > 1e-8 {
			t.Fatal(c)
		}
		n++
		if c.Section != nil {
			proofs++
			if !c.Section.Certificate.ProvenUnique || c.Section.FixedAxis != 1 && c.Section.FixedAxis != 2 {
				t.Fatal(c.Section)
			}
		}
	}
	if n < 4 || proofs < 4 {
		t.Fatal(n, proofs)
	}
	r, e = FindSplineSeeds(context.Background(), a, a, SplineSeedOptions{})
	if e != nil || !r.Covering.Complete || len(r.Covering.Coincident) != 1 || len(r.Attempts) != 0 {
		t.Fatal(r, e)
	}
	r, e = FindSplineSeeds(context.Background(), a, b, SplineSeedOptions{MaxCells: 1})
	if e != nil || !r.Truncated || r.Unattempted == 0 || len(r.Attempts) != 1 {
		t.Fatal(r, e)
	}
	r, e = FindSplineSeeds(context.Background(), a, b, SplineSeedOptions{Covering: geom.SurfaceSurfaceIsolationOptions{MaxPairs: 1}})
	if !errors.Is(e, geom.ErrConvergence) || !r.Truncated || len(r.Covering.Pending) == 0 {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = FindSplineSeeds(ctx, a, b, SplineSeedOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestSplineSeedTubeProvesBranch(t *testing.T) {
	k := []float64{0, 0, 1, 1}
	// The first patch runs along x in [.25,.75], while the second
	// retains strict interior room in its corresponding coordinate.
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{X: .25}, {X: .25, Y: 1}, {X: .75}, {X: .75, Y: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	b, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Y: .4, Z: -.5}, {Y: .4, Z: .5}, {X: 1, Y: .4, Z: -.5}, {X: 1, Y: .4, Z: .5}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	p := func(s *geom.BSplineSurface) Patch { return Patch{s, geom.Vec2{}, geom.Vec2{X: 1, Y: 1}} }
	r, e := FindSplineSeeds(context.Background(), p(a), p(b), SplineSeedOptions{Sections: true, BranchTubes: true,
		Covering: geom.SurfaceSurfaceIsolationOptions{ParameterTolerance: 1}})
	if e != nil || len(r.Attempts) != 1 || r.Attempts[0].Tube == nil || !r.Attempts[0].Tube.ProvenBranch {
		t.Fatal(r, e)
	}
	x := r.Attempts[0].Tube
	if x.FreeAxis != 0 || x.FreeParameter.Min != 0 || x.FreeParameter.Max != 1 {
		t.Fatal(x)
	}
}

func TestJoinSplineTubes(t *testing.T) {
	k := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{X: .25}, {X: .25, Y: 1}, {X: .75}, {X: .75, Y: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	b, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Y: .4, Z: -.5}, {Y: .4, Z: .5}, {X: 1, Y: .4, Z: -.5}, {X: 1, Y: .4, Z: .5}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	p := func(s *geom.BSplineSurface) Patch { return Patch{s, geom.Vec2{}, geom.Vec2{X: 1, Y: 1}} }
	ar, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	br, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	da, db := ar.Domain(), br.Domain()
	var seeds SplineSeedReport
	for i := range 3 {
		d := da
		d.UMin, d.UMax = float64(i)/3, float64(i+1)/3
		q, e := ar.Trim(d)
		if e != nil {
			t.Fatal(e)
		}
		c, e := geom.CertifySurfacePairTube(q, br, geom.SurfacePairTubeOptions{})
		if e != nil || !c.ProvenBranch {
			t.Fatal(c, e)
		}
		seeds.Attempts = append(seeds.Attempts, SplineSeedCell{Cell: geom.SurfaceSurfaceCell{DomainA: d, DomainB: db}, Tube: &c})
	}
	r, e := JoinSplineTubes(context.Background(), p(a), p(b), seeds, TubeChainOptions{})
	if e != nil || len(r.Chains) != 1 || len(r.Chains[0].Cells) != 3 || len(r.Junctions) != 2 || !r.Junctions[0].Certificate.Connected {
		t.Fatal(r, e)
	}
	curves, e := TubeCurves(context.Background(), p(a), p(b), seeds, r.Chains[0], TubeCurveOptions{})
	if e != nil || len(curves) != 3 {
		t.Fatal(curves, e)
	}
	for i, x := range curves {
		t0 := .37
		q, e := x.Evaluate(t0)
		if e != nil || !q.Complete {
			t.Fatal(q, e)
		}
		f, e := x.TrimA().EvalUV(t0)
		if e != nil {
			t.Fatal(e)
		}
		g, e := x.TrimB().EvalUV(t0)
		if e != nil || math.Abs(g.X-(.25+.5*f.X)) > 1e-10 || math.Abs(g.Y-.5) > 1e-10 {
			t.Fatal(f, g, e)
		}
		deriv, e := x.TrimB().DerivativeUV(t0)
		if e != nil || math.Abs(deriv.X-1.0/6) > 1e-10 || math.Abs(deriv.Y) > 1e-10 {
			t.Fatal(deriv, e)
		}
		if i+1 < len(curves) {
			a, _ := x.TrimA().EvalUV(1)
			b, _ := curves[i+1].TrimA().EvalUV(0)
			c, _ := x.TrimB().EvalUV(1)
			d, _ := curves[i+1].TrimB().EvalUV(0)
			if a != b || c != d {
				t.Fatal(a, b, c, d)
			}
		}
	}
	r, e = JoinSplineTubes(context.Background(), p(a), p(b), seeds, TubeChainOptions{MaxJunctions: 1})
	if e != nil || !r.Truncated || len(r.Chains) != 3 {
		t.Fatal(r, e)
	}
	seeds.Attempts = append(seeds.Attempts, seeds.Attempts[1])
	r, e = JoinSplineTubes(context.Background(), p(a), p(b), seeds, TubeChainOptions{})
	if e != nil || len(r.Chains) != 4 {
		t.Fatal(r, e)
	}
}

func TestFindSplineTubeOverlaps(t *testing.T) {
	k := []float64{0, 0, 1, 1}
	a, _ := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k, k, nil)
	b, _ := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Y: .4, Z: -.5}, {Y: .4, Z: .5}, {X: 1, Y: .4, Z: -.5}, {X: 1, Y: .4, Z: .5}}, k, k, nil)
	p := func(s *geom.BSplineSurface) Patch { return Patch{s, geom.Vec2{}, geom.Vec2{X: 1, Y: 1}} }
	ap, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	bp, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	d := geom.ParameterRectangle{UMin: .25, UMax: .75, VMin: 0, VMax: 1}
	var r SplineSeedReport
	for i, axis := range []int{0, 2} {
		x, y := ap, bp
		if i == 0 {
			x, _ = ap.Trim(d)
		} else {
			y, _ = bp.Trim(d)
		}
		c, e := geom.CertifySurfacePairTube(x, y, geom.SurfacePairTubeOptions{FreeAxis: axis})
		if e != nil || !c.ProvenBranch {
			t.Fatal(c, e)
		}
		r.Attempts = append(r.Attempts, SplineSeedCell{Tube: &c, Cell: geom.SurfaceSurfaceCell{DomainA: x.Domain(), DomainB: y.Domain()}})
	}
	m, truncated, e := FindSplineTubeOverlaps(context.Background(), p(a), p(b), r, TubeChainOptions{})
	if e != nil || truncated || len(m) != 1 || !m[0].Certificate.SameLocalBranch {
		t.Fatal(m, truncated, e)
	}
}
