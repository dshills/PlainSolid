package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func commonXYArcFixture(t *testing.T) (*SurfacePatch, *SurfacePatch, SurfacePlaneArc) {
	t.Helper()
	var pa, pb []Vec3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			u, v := float64(i)/2, float64(j)/2
			u2, v2 := float64(i*(i-1))/2, float64(j*(j-1))/2
			z := u2/4 + v2/8
			pa = append(pa, Vec3{u, v, z})
			pb = append(pb, Vec3{u, v, z + v - u2})
		}
	}
	knots := []float64{0, 0, 0, 1, 1, 1}
	sa, _ := NewBSplineSurface(2, 2, 3, 3, pa, knots, knots, nil)
	sb, _ := NewBSplineSurface(2, 2, 3, 3, pb, knots, knots, nil)
	a, _ := NewSurfacePatch(sa, SurfacePatchOptions{})
	b, _ := NewSurfacePatch(sb, SurfacePatchOptions{})
	// Reauthor B exactly in cubic Bernstein form, preserving rational poles
	// such as 1/3 instead of silently rounding a degree-elevated float export.
	cubicKnots := []float64{0, 0, 0, 0, 1, 1, 1, 1}
	cubic, _ := NewBSplineSurface(3, 3, 4, 4, make([]Vec3, 16), cubicKnots, cubicKnots, nil)
	b, _ = NewSurfacePatch(cubic, SurfacePatchOptions{})
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			u, v := big.NewRat(int64(i), 3), big.NewRat(int64(j), 3)
			u2, v2 := big.NewRat(int64(i*(i-1)), 6), big.NewRat(int64(j*(j-1)), 6)
			z := new(big.Rat).Add(new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(-3, 4), u2), new(big.Rat).Mul(big.NewRat(1, 8), v2)), v)
			b.net[i*4+j] = exactHomogeneous{u, v, z, big.NewRat(1, 1)}
		}
	}
	if e := b.computeBounds(&polynomialBudget{maxOperations: 100000, maxBits: 16384}); e != nil {
		t.Fatal(e)
	}
	var e error
	b, e = b.InsertUKnot(.75, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	difference, e := NewGeneralSurfaceHeightDifferencePatch(context.Background(), a, b, GeneralSurfaceHeightOptions{})
	if e != nil {
		t.Fatal(e)
	}
	local, e := difference.Trim(ParameterRectangle{.25, .5, 0, .5})
	if e != nil {
		t.Fatal(e)
	}
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	arcs, e := CertifySurfacePlaneArcs(local, plane, SurfacePlaneArcOptions{})
	if e != nil || len(arcs.Arcs) != 1 {
		t.Fatal(arcs, e)
	}
	return a, b, arcs.Arcs[0]
}
func TestDiscoverCommonXYCurvedArcTubes(t *testing.T) {
	ctx := context.Background()
	a, b, arc := commonXYArcFixture(t)
	start, end := .25, .5
	if arc.Certificate.FixedParameterAxis == 0 {
		start, end = .0625, .25
	}
	r, e := DiscoverCommonXYGraphArcTubes(ctx, arc, a, b, start, end, SurfaceArcTubeDiscoveryOptions{Padding: .1})
	if e != nil || !r.Complete || len(r.Tubes) == 0 || len(r.Pending) > 0 {
		t.Fatal(r, e)
	}
	regions := make([]SurfacePairTubeRegion, len(r.Tubes))
	for i, tube := range r.Tubes {
		regions[i] = SurfacePairTubeRegion{Box: tube.Box, FreeAxis: tube.FreeAxis}
	}
	coverage, ce := CertifySurfacePairCellInTubesContext(ctx, a, b, SurfacePairParameterBox{A: arc.source.Domain(), B: arc.source.Domain()}, regions, SurfaceTubeUnionCoverageOptions{MaxOperations: 8000000})
	if ce != nil || !coverage.Covered {
		t.Fatal("independent full arc-domain pair coverage", coverage, ce)
	}
	for _, tube := range r.Tubes {
		parameter := (tube.Span.Start + tube.Span.End) / 2
		u, v := parameter, parameter*parameter
		if tube.FreeAxis%2 == 1 {
			v = parameter
			u = math.Sqrt(v)
		}
		point, e := tube.Proof.Evaluate(SurfaceTubeEvaluationOptions{Parameter: parameter})
		if e != nil || !point.Complete || !point.Bounds.Contains(Vec3{u, v, u*u/4 + v*v/8}) {
			t.Fatal(point, e)
		}
		for _, upper := range []bool{false, true} {
			x := tube.Span.Start
			if upper {
				x = tube.Span.End
			}
			if x != start && x != end {
				continue
			}
			q := new(big.Rat).SetFloat64(x)
			y := new(big.Rat).Mul(q, q)
			if tube.FreeAxis%2 == 1 {
				y = new(big.Rat).SetFloat64(x)
				root := .25
				if x == end {
					root = .5
				}
				q = new(big.Rat).SetFloat64(root)
			}
			endpoint, e := tube.Proof.CertifyEndpoint(ctx, upper, [4]*big.Rat{q, y, new(big.Rat).Set(q), new(big.Rat).Set(y)}, TubeEndpointOptions{})
			if e != nil || !endpoint.Proven {
				t.Fatal(endpoint, e)
			}
		}
	}
}
func TestDiscoverCommonXYArcRejectsWrongSources(t *testing.T) {
	ctx := context.Background()
	a, b, arc := commonXYArcFixture(t)
	wrong, e := b.Transformed(Translation(Vec3{Z: .125}))
	if e != nil {
		t.Fatal(e)
	}
	r, e := DiscoverCommonXYGraphArcTubes(ctx, arc, a, wrong, .25, .5, SurfaceArcTubeDiscoveryOptions{})
	if !errors.Is(e, ErrInvalid) || r.Complete {
		t.Fatal(r, e)
	}
	_, e = DiscoverCommonXYGraphArcTubes(ctx, arc, a, b, .25, .5, SurfaceArcTubeDiscoveryOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = DiscoverCommonXYGraphArcTubes(canceled, arc, a, b, .25, .5, SurfaceArcTubeDiscoveryOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
