package intersect

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestTraceSplinePair(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	patch := func(p []geom.Vec3) Patch {
		t.Helper()
		s, e := geom.NewBSplineSurface(1, 1, 2, 2, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return Patch{s, geom.Vec2{}, geom.Vec2{X: 1, Y: 1}}
	}
	a := patch([]geom.Vec3{{X: .25}, {X: .25, Y: 1}, {X: .75}, {X: .75, Y: 1}})
	wall := func(z float64) Patch {
		return patch([]geom.Vec3{{Y: .5, Z: z - 1}, {Y: .5, Z: z + 1},
			{X: 1, Y: .5, Z: z - 1}, {X: 1, Y: .5, Z: z + 1}})
	}
	r, e := TraceSplinePair(ctx, a, wall(0), SplinePairTraceOptions{})
	if e != nil || !r.Complete || len(r.Curves) != 1 || len(r.Curves[0]) != 1 || !r.Coverage.CoversAllContacts {
		t.Fatal(r, e)
	}
	x, y := r.Curves[0][0].TrimA().EvalUV(.5)
	if y != nil || x != (geom.Vec2{X: .5, Y: .5}) {
		t.Fatal(x, y)
	}
	segment := r.Curves[0][0]
	for end := range 2 {
		terminal, e := DiscoverRationalSplineTubeTerminal(ctx, segment, end, a, wall(0), geom.TubeEndpointEdgeOptions{})
		if e != nil || !terminal.Proof.Proven || terminal.Operand != 0 ||
			terminal.Proof.Parameters[1].Cmp(big.NewRat(1, 2)) != 0 ||
			terminal.Proof.Point[1].Cmp(big.NewRat(1, 2)) != 0 {
			t.Fatal(end, terminal, e)
		}
	}
	first, e := segment.EvaluatePair(0)
	if e != nil {
		t.Fatal(e)
	}
	last, e := segment.EvaluatePair(1)
	if e != nil {
		t.Fatal(e)
	}
	line, e := segment.CertifyAffineLine(ctx, first, last, geom.TubeAffineLineOptions{})
	if e != nil || !line.Proven {
		t.Fatal(line, e)
	}
	q, e := line.Curve.ExactPoint(.5)
	if e != nil || q != (geom.Vec3{X: .5, Y: .5}) {
		t.Fatal(q, e)
	}
	if _, e := segment.CertifyAffineLine(ctx, last, first, geom.TubeAffineLineOptions{}); !errors.Is(e, geom.ErrDomain) {
		t.Fatal(e)
	}
	r, e = TraceSplinePair(ctx, a, wall(3), SplinePairTraceOptions{})
	if e != nil || !r.Complete || len(r.Curves) != 0 {
		t.Fatal(r, e)
	}
	r, e = TraceSplinePair(ctx, a, a, SplinePairTraceOptions{})
	if !errors.Is(e, ErrUnresolved) || r.Complete || len(r.Coverage.Isolation.Coincident) != 1 {
		t.Fatal(r, e)
	}
}

func TestCertifySuppliedSplineBranches(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	a, _ := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{X: .25}, {X: .25, Y: 1}, {X: .75}, {X: .75, Y: 1}}, k, k, nil)
	b, _ := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Y: .5, Z: -1}, {Y: .5, Z: 1}, {X: 1, Y: .5, Z: -1}, {X: 1, Y: .5, Z: 1}}, k, k, nil)
	patch := func(s *geom.BSplineSurface) Patch { return Patch{s, geom.Vec2{}, geom.Vec2{X: 1, Y: 1}} }
	d := geom.ParameterRectangle{UMax: 1, VMax: 1}
	pa, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	pb, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	c, e := geom.CertifySurfacePairTube(pa, pb, geom.SurfacePairTubeOptions{})
	if e != nil || !c.ProvenBranch {
		t.Fatal(c, e)
	}
	seeds := SplineSeedReport{Attempts: []SplineSeedCell{{
		Cell: geom.SurfaceSurfaceCell{DomainA: d, DomainB: d},
		Tube: &c,
	}}}
	r, e := CertifySplinePairBranches(ctx, patch(a), patch(b), seeds, SplinePairTraceOptions{})
	if e != nil || !r.Complete || len(r.Curves) != 1 || !r.Coverage.CoversAllContacts {
		t.Fatal(r, e)
	}
	seeds.Attempts[0].Cell.DomainB.VMin = .75
	r, e = CertifySplinePairBranches(ctx, patch(a), patch(b), seeds, SplinePairTraceOptions{})
	if e == nil || r.Complete {
		t.Fatal(r, e)
	}
}
