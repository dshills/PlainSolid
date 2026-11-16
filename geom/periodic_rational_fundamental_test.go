package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestPeriodicExactNonuniformFundamentalKeepsPhysicalMap(t *testing.T) {
	ctx := context.Background()
	original := periodicDistanceSurface(t)
	s, err := original.ExactCyclicSurface(ctx, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	station := big.NewRat(1, 7)
	s, err = s.InsertPeriodicKnot(ctx, 0, station, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	s, err = s.RephasePeriodicKnot(ctx, 0, station, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	f, err := s.ExactFundamentalPatch(ctx, PeriodicSurfaceBezierOptions{})
	if err != nil || f.Wrapped == nil || f.Seam.PositionMatched != ([2]bool{true, false}) || f.Seam.FirstPartialsMatched != ([2]bool{}) {
		t.Fatal("only original periodic position closure is expected", f, err)
	}
	axis := f.Coordinates
	uKnots, uSpans := axis[0].Breaks()
	vKnots, vSpans := axis[1].Breaks()
	if len(uSpans) != 4 || len(vSpans) != 2 || uKnots[0].Cmp(station) != 0 || uKnots[4].Cmp(big.NewRat(8, 7)) != 0 ||
		vKnots[0].Cmp(big.NewRat(-2, 1)) != 0 || vKnots[1].Cmp(big.NewRat(1, 4)) != 0 || vKnots[2].Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("lost a true source station or the original topology of the axes", uKnots, vKnots)
	}
	uKnots[0].SetInt64(20)
	if x, err := axis[0].ToPhysical(new(big.Rat)); err != nil || x.Cmp(station) != 0 {
		t.Fatal("mapped source coordinates are not owned", x, err)
	}
	const samples = 4
	patch := f.Wrapped.FundamentalPatch()
	for i := range uSpans {
		for j := range vSpans {
			for k := 1; k <= samples; k++ {
				wu, wv := big.NewRat(int64(5*i+k), 5), big.NewRat(int64(7*j+k), 7)
				u, err := axis[0].ToPhysical(wu)
				if err != nil {
					t.Fatal(err)
				}
				v, err := axis[1].ToPhysical(wv)
				if err != nil {
					t.Fatal(err)
				}
				for dim, physical := range []*big.Rat{u, v} {
					working := []*big.Rat{wu, wv}[dim]
					if got, err := axis[dim].ToWorking(physical); err != nil || got.Cmp(working) != 0 {
						t.Fatal("working inverse rounded physical parameters", got, working, err)
					}
				}
				want, err := s.EvaluateRational(ctx, u, v, SurfacePatchOptions{})
				if err != nil {
					t.Fatal(err)
				}
				got, err := patch.EvaluateRational(ctx, wu, wv, SurfacePatchEvaluationOptions{})
				if err != nil || want.U.Span != uSpans[i] || want.V.Span != vSpans[j] {
					t.Fatal("wrong source knot owner in the reassembled patch", got, want, err)
				}
				for dim, d := range [2]ExactParameterInterval{want.U.Domain, want.V.Domain} {
					a, b := [][3]*big.Rat{got.Du, got.Dv}[dim], [][3]*big.Rat{want.Du, want.Dv}[dim]
					width := new(big.Rat).Sub(d.Upper, d.Lower)
					for component := range got.Point {
						if got.Point[component].Cmp(want.Point[component]) != 0 || new(big.Rat).Quo(a[component], width).Cmp(b[component]) != 0 {
							t.Fatal("working patch changed source value or physical partial", i, j, k)
						}
					}
				}
			}
		}
	}
	if got, err := axis[0].ToWorking(big.NewRat(8, 7)); err != nil || got.Cmp(big.NewRat(4, 1)) != 0 {
		t.Fatal("upper seam mapping changed endpoint", got, err)
	}
	plane, err := NewPlane(Vec3{Z: 2.5}, Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	trace, err := TraceWrappedSurfacePlaneContext(ctx, f.Wrapped, plane, SurfaceTraceOptions{AllowC0SeamContinuation: true})
	if err != nil || !trace.Complete || len(trace.Contours) != 1 || !trace.Contours[0].Closed || len(trace.Contours[0].Arcs) != 4 || len(trace.SeamJoins) != 1 {
		t.Fatal("section does not retain all four source spans across the new seam", trace, err)
	}
	join := trace.SeamJoins[0]
	if join.Axis != 0 || !join.RootCertificate.C0Continuation {
		t.Fatal("missing one-sided continuation at the new seam", join)
	}
	lo, err := axis[1].ToPhysical(join.RootCertificate.Lower)
	if err != nil {
		t.Fatal(err)
	}
	hi, err := axis[1].ToPhysical(join.RootCertificate.Upper)
	if err != nil {
		t.Fatal(err)
	}
	assertRationalParameterBracket(t, ExactParameterInterval{Lower: lo, Upper: hi}, big.NewRat(-1, 8))
	if p, err := s.ExactFundamentalPatch(ctx, PeriodicSurfaceBezierOptions{MaxCells: 7}); p.Wrapped != nil || !errors.Is(err, ErrConvergence) {
		t.Fatal("partial fundamental patch published with a short grid budget", err)
	}
	if p, err := s.ExactFundamentalPatch(ctx, PeriodicSurfaceBezierOptions{SurfacePatchOptions: SurfacePatchOptions{MaxOperations: f.ExactOperations - 1}}); p.Wrapped != nil || !errors.Is(err, ErrConvergence) {
		t.Fatal("partial fundamental patch published with a short work budget", err)
	}
}

func TestPeriodicExactFundamentalRefusesOrdinaryJump(t *testing.T) {
	ctx := context.Background()
	data := BSplineSurfaceData{DegreeU: 1, DegreeV: 1, CountU: 3, CountV: 4, KnotsV: []float64{0, 0, .5, .5, 1, 1}}
	for u := range data.CountU {
		for v := range data.CountV {
			data.Poles = append(data.Poles, Vec3{float64(u), float64(u + v), float64(v)})
		}
	}
	base, err := NewPeriodicBSplineSurface(data, &PeriodicSplineAxis{Period: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s, err := base.ExactCyclicSurface(ctx, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if p, err := s.ExactFundamentalPatch(ctx, PeriodicSurfaceBezierOptions{}); p.Wrapped != nil || !errors.Is(err, ErrDomain) {
		t.Fatal("ordinary discontinuity was incorrectly welded into a single patch", p, err)
	}
}
