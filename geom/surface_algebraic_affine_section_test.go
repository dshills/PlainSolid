package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestExactRootAffineMap(t *testing.T) {
	ctx := context.Background()
	root := importTestRoot(t, big.NewRat(-2, 1), true, "1", "2")
	mapped, ops, e := root.AffineMap(ctx, big.NewRat(1, 1), big.NewRat(-2, 1), ExactCurvePlaneEventOptions{})
	if e != nil || ops == 0 {
		t.Fatal(mapped, ops, e)
	}
	expected, e := ImportExactRoot(ctx, ExactRootRecipe{Version: 1, Coefficients: []string{"-7", "-2", "1"}, Lower: "-2", Upper: "-1"}, ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	order, e := mapped.Compare(ctx, expected, ExactCurvePlaneEventOptions{})
	if e != nil || !order.Proven || order.Order != 0 {
		t.Fatal(order, e)
	}
	constant, _, e := root.AffineMap(ctx, big.NewRat(1, 3), new(big.Rat), ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	classification, e := constant.Classify(ctx, ExactCurvePlaneEventOptions{})
	if e != nil || classification.Kind != ExactRootKindRational || classification.Rational.Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal(classification, e)
	}
	if _, _, e = root.AffineMap(ctx, big.NewRat(1, 1), big.NewRat(2, 1), ExactCurvePlaneEventOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
}

func TestAlgebraicAffineSectionOriginalUnitsAcrossKnots(t *testing.T) {
	ctx := context.Background()
	p, _, _, _ := algebraicSeamFixture(t)
	lower := importTestRoot(t, big.NewRat(-2, 1), true, "1", "3/2")
	upper := importTestRoot(t, big.NewRat(-5, 1), true, "2", "5/2")
	for _, reverse := range []bool{false, true} {
		origin := [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}
		direction := [2]*big.Rat{big.NewRat(1, 3), big.NewRat(-1, 3)}
		if reverse {
			origin = [2]*big.Rat{big.NewRat(1, 1), new(big.Rat)}
			direction = [2]*big.Rat{big.NewRat(-1, 3), big.NewRat(1, 3)}
		}
		r, e := p.BuildAffineSection(ctx, origin, direction, lower, upper, SurfaceCurveAgreementOptions{})
		if e != nil || !r.Complete || len(r.Cells) != 2 || len(r.Events) != 3 {
			t.Fatal(r, e)
		}
		lo, _ := r.Cells[0].EndpointRoot(0)
		hi, _ := r.Cells[1].EndpointRoot(1)
		for _, pair := range [][2]*ExactAlgebraicRoot{{lo, lower}, {hi, upper}} {
			order, e := pair[0].Compare(ctx, pair[1], ExactCurvePlaneEventOptions{})
			if e != nil || order.Order != 0 {
				t.Fatal(order, e)
			}
		}
		q, e := r.Cells[1].EvaluateRational(ctx, big.NewRat(2, 1), ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		uv := [2]*big.Rat{}
		for axis := 0; axis < 2; axis++ {
			uv[axis] = new(big.Rat).Add(origin[axis], new(big.Rat).Mul(big.NewRat(2, 1), direction[axis]))
		}
		expected, e := p.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for axis := 0; axis < 3; axis++ {
			if q.Point[axis].Cmp(expected.Point[axis]) != 0 {
				t.Fatal(q, expected)
			}
		}
	}
	origin := [2]*big.Rat{new(big.Rat), new(big.Rat)}
	direction := [2]*big.Rat{big.NewRat(1, 3), big.NewRat(1, 3)}
	outside := importTestRoot(t, big.NewRat(-4, 1), false, "4", "4")
	if _, e := p.BuildAffineSection(ctx, origin, direction, lower, outside, SurfaceCurveAgreementOptions{}); !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
	if _, e := p.BuildAffineSection(ctx, origin, direction, lower, upper, SurfaceCurveAgreementOptions{MaxSubdivisions: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := p.BuildAffineSection(cancelCtx, origin, direction, lower, upper, SurfaceCurveAgreementOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
