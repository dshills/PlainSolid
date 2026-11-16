package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestProjectiveGraphBoundaryContactAuthenticatesCompleteEdge(t *testing.T) {
	ctx := context.Background()
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	for _, reverse := range []bool{false, true} {
		b := projectiveBoundaryContactChart(t, reverse, false)
		r, err := CertifyProjectiveGraphBoundaryContact(ctx, a, b, [2]int{0, 1}, SurfaceAffineCurveOptions{})
		if err != nil || !r.Complete || !r.Edge || r.Point || r.Disjoint || r.ComparedSpans != 2 ||
			r.BClockRatio.Cmp(big.NewRat(3, 2)) != 0 || len(r.Owners[1].Pieces) != 2 {
			t.Fatal("complete common edge must retain both curvilinear clocks", reverse, r, err)
		}
		start, end, cut := big.NewRat(4, 1), big.NewRat(8, 1), big.NewRat(3, 4)
		if reverse {
			start, end, cut = end, start, big.NewRat(1, 4)
		}
		if r.Endpoints[0].OriginalUV[1][1].Cmp(start) != 0 || r.Endpoints[1].OriginalUV[1][1].Cmp(end) != 0 ||
			r.Owners[1].Pieces[0].End.Cmp(cut) != 0 {
			t.Fatal("original owner direction or real B knot station was lost", reverse, r.Endpoints, r.Owners[1])
		}
		for i, geo := range [2][3]*big.Rat{{big.NewRat(1, 1), new(big.Rat), new(big.Rat)}, {big.NewRat(1, 1), big.NewRat(1, 1), big.NewRat(1, 1)}} {
			for axis, x := range r.Endpoints[i].Point {
				if x.Cmp(geo[axis]) != 0 {
					t.Fatal("wrong original model point on shared edge", i, axis, x)
				}
			}
		}
	}
	b := projectiveBoundaryContactChart(t, false, true)
	r, err := CertifyProjectiveGraphBoundaryContact(ctx, a, b, [2]int{0, 1}, SurfaceAffineCurveOptions{})
	if !errors.Is(err, ErrConvergence) || r.Complete || r.Edge || r.Disjoint || !r.Owners[0].Complete || !r.Owners[1].Complete {
		t.Fatal("unequal traces may still meet and must retain original edge data", r, err)
	}
	// The unequal traces were independently authored to meet at y=1/2.
	// A's directed station is 1/2 and B's exact clock there is 3/5.
	x := evaluateProjectiveGraphBoundaryPiece(t, r.Owners[0], big.NewRat(1, 2))
	y := evaluateProjectiveGraphBoundaryPiece(t, r.Owners[1], big.NewRat(3, 5))
	if x[2].Cmp(y[2]) != 0 {
		t.Fatal("the unresolved case must contain a real source contact", x, y)
	}
}

func TestProjectiveGraphBoundaryContactSeparatesOnePointFromNoContact(t *testing.T) {
	ctx := context.Background()
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	for _, f := range []struct {
		x, y, z float64
		point   bool
	}{
		{1, 1, 0, true}, {1, 1, .25, false}, {2, 1, 0, false},
	} {
		b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{f.x + u, f.y + v, f.z} })
		r, err := CertifyProjectiveGraphBoundaryContact(ctx, a, b, [2]int{0, 1}, SurfaceAffineCurveOptions{})
		if err != nil || !r.Complete || r.Point != f.point || r.Disjoint == f.point || r.Edge {
			t.Fatal("wrong exact point/disjoint classification", f, r, err)
		}
		if f.point && (r.Endpoints[0].Point[0].Cmp(big.NewRat(1, 1)) != 0 || r.Endpoints[0].Point[1].Cmp(big.NewRat(1, 1)) != 0) {
			t.Fatal("common corner lost its model identity", r.Endpoints[0])
		}
	}
	r, err := CertifyProjectiveGraphBoundaryContact(ctx, a, a, [2]int{0, 1}, SurfaceAffineCurveOptions{})
	if !errors.Is(err, ErrDomain) || r.Complete {
		t.Fatal("positive rectangle belongs to the area proof", r, err)
	}
}

func projectiveBoundaryContactChart(t *testing.T, reverse, unequal bool) *SurfacePatch {
	t.Helper()
	p := &SurfacePatch{degreeU: 1, degreeV: 1, countU: 2, countV: 3,
		knotsU: []float64{2, 2, 3, 3}, knotsV: []float64{4, 4, 7, 8, 8}}
	for x := int64(1); x <= 2; x++ {
		for _, s := range [3]*big.Rat{new(big.Rat), big.NewRat(3, 4), big.NewRat(1, 1)} {
			if reverse {
				s = new(big.Rat).Sub(big.NewRat(1, 1), s)
			}
			w := new(big.Rat).Sub(big.NewRat(1, 1), new(big.Rat).Quo(s, big.NewRat(3, 1)))
			y := new(big.Rat).Mul(big.NewRat(2, 3), s)
			z := new(big.Rat).Mul(big.NewRat(x, 1), y)
			if unequal {
				shift := new(big.Rat).Sub(y, new(big.Rat).Quo(w, big.NewRat(2, 1)))
				z.Add(z, shift.Quo(shift, big.NewRat(1024, 1)))
			}
			p.net = append(p.net, exactHomogeneous{new(big.Rat).Mul(big.NewRat(x, 1), w), y, z, w})
		}
	}
	p.options, _ = patchOptions(SurfacePatchOptions{})
	if err := p.computeBounds(&polynomialBudget{maxOperations: p.options.MaxOperations, maxBits: p.options.MaxCoefficientBits}); err != nil {
		t.Fatal(err)
	}
	return p
}
