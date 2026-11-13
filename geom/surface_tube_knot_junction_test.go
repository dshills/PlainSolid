package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestSurfaceKnotJunctionTrueCrease(t *testing.T) {
	ctx := context.Background()
	ku := []float64{0, 0, .5, 1, 1}
	kv := []float64{0, 0, 2, 2}
	var ap, bp []Vec3
	for i, u := range []float64{0, .5, 1} {
		f := []float64{0, .5, 1.5}[i]
		for _, v := range []float64{0, 2} {
			ap = append(ap, Vec3{u, v, 0})
			bp = append(bp, Vec3{u, v, v - f})
		}
	}
	sa, e := NewBSplineSurface(1, 1, 3, 2, ap, ku, kv, nil)
	if e != nil {
		t.Fatal(e)
	}
	sb, e := NewBSplineSurface(1, 1, 3, 2, bp, ku, kv, nil)
	if e != nil {
		t.Fatal(e)
	}
	a, _ := NewSurfacePatch(sa, SurfacePatchOptions{})
	b, _ := NewSurfacePatch(sb, SurfacePatchOptions{})
	options := SurfaceKnotJunctionOptions{SurfacePairTubeOptions: SurfacePairTubeOptions{FreeAxis: 0}, Left: SurfacePairParameterBox{A: ParameterRectangle{.375, .5, 0, 1}, B: ParameterRectangle{.25, .5, 0, 1}}, Right: SurfacePairParameterBox{A: ParameterRectangle{.5, .625, 0, 1}, B: ParameterRectangle{.5, .75, 0, 1}}, Anchor: [4]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2), big.NewRat(1, 2), big.NewRat(1, 2)}}
	j, e := CertifySurfaceKnotJunction(ctx, a, b, options)
	if e != nil {
		t.Fatal(e)
	}
	left, right, e := j.Tubes()
	if e != nil {
		t.Fatal(e)
	}
	for _, tube := range []VerifiedSurfaceTube{left, right} {
		endpoint, e := tube.EvaluateRationalGraph(ctx, big.NewRat(1, 2), TubeAlgebraicGraphEndpointOptions{})
		if e != nil || !endpoint.Proven {
			t.Fatal("anchor graph", endpoint, e)
		}
		bound, e := tube.BindAffineGraphEvent(ctx, endpoint.ModelEvent, TubeAlgebraicGraphEndpointOptions{})
		if e != nil || bound == nil {
			t.Fatal("anchor event", e)
		}
	}
	interior, e := left.EvaluateRationalGraph(ctx, big.NewRat(7, 16), TubeAlgebraicGraphEndpointOptions{})
	if e != nil || !interior.Proven {
		t.Fatal("anchored interior", interior, e)
	}
	trim, e := NewExactHomogeneousBezier(ctx, [][4]*big.Rat{{big.NewRat(1, 4), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 2), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(3, 4), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, tube := range []VerifiedSurfaceTube{left, right} {
		contacts, e := tube.IntersectQuadraticTrim(ctx, a, b, 0, trim, TubeTrimIntersectionOptions{})
		if e != nil || !contacts.Complete || len(contacts.Hits) == 0 {
			t.Fatal("anchored trim", contacts, e)
		}
	}
	// Sharing only the free wall does not authorize a false nearby UV contact.
	fake := [4]*ExactAlgebraicRoot{}
	for i, q := range options.Anchor {
		if i == 1 {
			q = big.NewRat(500001, 1000000)
		}
		fake[i], _, e = NewExactRationalRoot(ctx, q, ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
	}
	equal, e := left.matchesAnchorRoots(ctx, fake, &polynomialBudget{maxOperations: 100000, maxBits: 16384}, 1024)
	if e != nil || equal {
		t.Fatal("nearby wall", equal, e)
	}
	for side, tube := range []VerifiedSurfaceTube{left, right} {
		lo, hi := .375, .5
		if side == 1 {
			lo, hi = .5, .625
		}
		envelope, e := tube.EncloseInterval(ctx, lo, hi, SurfaceTubeIntervalOptions{MaxIterations: 4})
		if !envelope.HasBounds {
			t.Fatal("anchored bounds", envelope, e)
		}
		for i := 0; i <= 16; i++ {
			u := lo + (hi-lo)*float64(i)/16
			v := u
			if side == 1 {
				v = 2*u - .5
			}
			if u < envelope.ParametersA.Min.X || u > envelope.ParametersA.Max.X || v < envelope.ParametersA.Min.Y || v > envelope.ParametersA.Max.Y || u < envelope.ParametersB.Min.X || u > envelope.ParametersB.Max.X || v < envelope.ParametersB.Min.Y || v > envelope.ParametersB.Max.Y {
				t.Fatal("lost anchored branch", u, v, envelope)
			}
		}
	}
	dl, e := left.ParameterDerivativeSigns(ctx, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	dr, e := right.ParameterDerivativeSigns(ctx, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if dl.Ranges[1].Min != 1 || dl.Ranges[1].Max != 1 || dr.Ranges[1].Min != 2 || dr.Ranges[1].Max != 2 {
		t.Fatal("lost crease", dl, dr)
	}
	q, p, e := j.Anchor()
	if e != nil || p[0].Cmp(big.NewRat(1, 2)) != 0 || p[2].Sign() != 0 {
		t.Fatal(q, p, e)
	}
	q[0].SetInt64(0)
	again, _, _ := j.Anchor()
	if again[0].Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal("mutable")
	}
	for side, box := range []SurfacePairParameterBox{options.Left, options.Right} {
		anchor := SurfaceTubeBoundaryOptions{SurfacePairTubeOptions: options.SurfacePairTubeOptions, AtUpper: side == 0, Anchor: options.Anchor}
		coverage, e := CertifySurfacePairCellInTubesContext(ctx, a, b, box, []SurfacePairTubeRegion{{Box: box, FreeAxis: 0, Boundary: &anchor}}, SurfaceTubeUnionCoverageOptions{})
		if e != nil || !coverage.Covered {
			t.Fatal("independent one-sided coverage", coverage, e)
		}
	}
	wrong := options
	wrong.Anchor[1] = big.NewRat(1, 3)
	if _, e := CertifySurfaceKnotJunction(ctx, a, b, wrong); e == nil {
		t.Fatal("wrong anchor")
	}
	limited := options
	limited.MaxOperations = 1
	if _, e := CertifySurfaceKnotJunction(ctx, a, b, limited); e == nil {
		t.Fatal("budget")
	}
}
