package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"slices"
	"testing"
)

func periodicRefinedCurveJet(t *testing.T, c *PeriodicBSpline, x *big.Rat) (point, derivative [3]*big.Rat) {
	t.Helper()
	// Extract a short exact restriction with x as its left endpoint.
	// Its homogeneous endpoint and endpoint derivative determine
	// the original real-coordinate jet without float evaluation.
	end := new(big.Rat).Add(x, big.NewRat(1, 2048))
	pieces, _, err := c.ExactBezierSpans(context.Background(), x, end, ExactBezierSpanOptions{})
	if err != nil || len(pieces) == 0 {
		t.Fatal("exact source restriction", err)
	}
	s := pieces[0]
	a, b := s.Control[0], s.Control[1]
	dt := new(big.Rat).Sub(s.End, s.Start)
	scale := new(big.Rat).Quo(big.NewRat(int64(c.Degree()), 1), dt)
	w2 := new(big.Rat).Mul(a[3], a[3])
	for i := range point {
		point[i] = new(big.Rat).Quo(a[i], a[3])
		d := new(big.Rat).Sub(new(big.Rat).Mul(b[i], a[3]), new(big.Rat).Mul(a[i], b[3]))
		derivative[i] = d.Mul(d.Quo(d, w2), scale)
	}
	return point, derivative
}

func TestPeriodicCurveExactDyadicRefinement(t *testing.T) {
	ctx := context.Background()
	poles := []Vec3{{1, 0, 0}, {3, 2, 1}, {2, -1, 3}, {-1, 1, 2}, {-2, 0, 1}}
	for _, tc := range []struct {
		name           string
		degree, levels int
		poles          []Vec3
		weights        []float64
	}{
		{"cubic polynomial", 3, 2, poles, []float64{1, 1, 1, 1, 1}},
		{"linear rational", 1, 1, poles[:4], []float64{1, 3, 1, 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := NewPeriodicBSplineWithExactWeights(tc.degree, tc.poles, tc.weights, -.25, 1.75)
			if err != nil {
				t.Fatal(err)
			}
			refined, operations, err := c.RefineUniformKnots(ctx, tc.levels, SurfacePatchOptions{})
			if err != nil {
				t.Fatal(err)
			}
			a, b := c.Domain()
			x, y := refined.Domain()
			if x != a || y != b || refined.Period() != c.Period() || refined.Degree() != tc.degree ||
				len(refined.Poles()) != len(tc.poles)<<tc.levels {
				t.Fatal("refinement changes physical clock or degree")
			}
			reversed, err := refined.Reversed()
			if err != nil {
				t.Fatal(err)
			}
			for cycle := int64(-2); cycle <= 2; cycle++ {
				for _, offset := range []*big.Rat{big.NewRat(0, 1), big.NewRat(7, int64(16*len(tc.poles))), big.NewRat(31, 81)} {
					u := new(big.Rat).Add(big.NewRat(7*cycle-1, 4), offset)
					p, d := periodicRefinedCurveJet(t, c, u)
					p2, d2 := periodicRefinedCurveJet(t, refined, u)
					v := new(big.Rat).Sub(big.NewRat(5, 4), u) // 2*start+period-u
					pr, _ := periodicRefinedCurveJet(t, reversed, v)
					for axis := range p {
						if p[axis].Cmp(p2[axis]) != 0 || d[axis].Cmp(d2[axis]) != 0 || p[axis].Cmp(pr[axis]) != 0 {
							t.Fatal("geometry, first jet or reversed exact weights changed", u, p, p2, d, d2)
						}
					}
				}
			}
			if got, _, err := c.RefineUniformKnots(ctx, tc.levels, SurfacePatchOptions{MaxOperations: operations - 1}); !errors.Is(err, ErrConvergence) || got != nil {
				t.Fatal("short exact budget returned a curve", err)
			}
			if !slices.Equal(c.Poles(), tc.poles) || !slices.Equal(c.Weights(), tc.weights) {
				t.Fatal("refinement mutated the source")
			}
		})
	}
}

func TestPeriodicSurfaceExactDyadicRefinement(t *testing.T) {
	ctx := context.Background()
	ds := BSplineSurfaceData{DegreeU: 2, DegreeV: 1, CountU: 4, CountV: 4}
	for u := range ds.CountU {
		for v := range ds.CountV {
			ds.Poles = append(ds.Poles, Vec3{float64(u), float64(v), float64((u*u+v*u+v)%7) / 8})
			ds.Weights = append(ds.Weights, float64(1+2*(v%2)))
		}
	}
	for _, wrapV := range []bool{false, true} {
		var vAxis *PeriodicSplineAxis
		vLevels := 0
		data := ds
		if wrapV {
			vAxis = &PeriodicSplineAxis{Start: -2, Period: 3}
			vLevels = 1
		} else {
			// Preserve the original one-sided law at a discontinuous
			// ordinary V knot while bisecting the separate periodic U axis.
			data.KnotsV = []float64{0, 0, .5, .5, 1, 1}
		}
		s, err := NewPeriodicBSplineSurface(data, &PeriodicSplineAxis{Start: 1, Period: 3}, vAxis)
		if err != nil {
			t.Fatal(err)
		}
		r, _, err := s.RefineUniformKnots(ctx, 2, vLevels, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		a, b, c, d := s.Domain()
		aa, bb, cc, dd := r.Domain()
		rd, _, _ := r.Data()
		if [4]float64{a, b, c, d} != ([4]float64{aa, bb, cc, dd}) || rd.CountU != 16 ||
			rd.CountV != 4<<vLevels || !slices.Equal(rd.KnotsV, data.KnotsV) {
			t.Fatal("period or ordinary knot vector changed")
		}
		huge := new(big.Int).Lsh(big.NewInt(1), 100)
		deepCycle := new(big.Rat).Add(new(big.Rat).Mul(new(big.Rat).SetInt(huge), big.NewRat(-3, 1)), big.NewRat(13, 7))
		for _, u := range []*big.Rat{big.NewRat(1, 1), big.NewRat(7, 4), big.NewRat(9, 5), deepCycle} {
			for _, v := range []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 2), big.NewRat(5, 7)} {
				want, err := s.EvaluateRational(ctx, u, v, SurfacePatchOptions{})
				if err != nil {
					t.Fatal(err)
				}
				got, err := r.EvaluateRational(ctx, u, v, SurfacePatchOptions{})
				if err != nil {
					t.Fatal(err)
				}
				for axis := range want.Point {
					if want.Point[axis].Cmp(got.Point[axis]) != 0 || want.Du[axis].Cmp(got.Du[axis]) != 0 ||
						want.Dv[axis].Cmp(got.Dv[axis]) != 0 || want.U.Cycle.Cmp(got.U.Cycle) != 0 {
						t.Fatal("exact periodic tensor point, jet or cycle changed", u, v, axis)
					}
				}
			}
		}
		if !wrapV {
			if out, _, err := s.RefineUniformKnots(ctx, 0, 1, SurfacePatchOptions{}); !errors.Is(err, ErrDomain) || out != nil {
				t.Fatal("ordinary knot axis admitted periodic refinement", err)
			}
		}
	}
}

func TestPeriodicRefinementRefusesLossAndOverflow(t *testing.T) {
	ctx := context.Background()
	poles := []Vec3{{0, 0, 0}, {1, 1, 0}, {2, 0, 1}, {1, -1, 3}}
	nonbinary, err := NewPeriodicBSplineWithExactWeights(1, poles, []float64{1, 2, 1, 2}, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result, _, err := nonbinary.RefineUniformKnots(ctx, 1, SurfacePatchOptions{}); !errors.Is(err, ErrUnrepresentable) || result != nil {
		t.Fatal("nonbinary refined pole was rounded", err)
	}
	c, err := NewPeriodicBSpline(1, poles, nil, math.Ldexp(1, 52), 16)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		levels int
		limit  SurfacePatchOptions
		err    error
	}{
		{4, SurfacePatchOptions{}, ErrUnrepresentable},
		{1, SurfacePatchOptions{MaxPoles: 4}, ErrConvergence},
		{math.MaxInt, SurfacePatchOptions{}, ErrConvergence},
		{-1, SurfacePatchOptions{}, ErrInvalid},
	} {
		if result, _, err := c.RefineUniformKnots(ctx, tc.levels, tc.limit); !errors.Is(err, tc.err) || result != nil {
			t.Fatal("unsafe refinement returned a candidate", tc.levels, err)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if result, _, err := c.RefineUniformKnots(cancelled, 1, SurfacePatchOptions{}); !errors.Is(err, context.Canceled) || result != nil {
		t.Fatal("cancelled refinement returned a curve", err)
	}
}
