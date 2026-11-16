package cuspidal

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestExactQuarticWallPCurve(t *testing.T) {
	s := testCubicWallSpan(3)
	net, err := exactQuarticWallPCurve(s, edgecheck.Options{}, testWallDecimal)
	if err != nil || len(net) != 5 || net[0] != s.Control[0] || net[4] != s.Control[3] {
		t.Fatalf("exact degree elevation lost clamped endpoint identity: %v %v", net, err)
	}
	// A degree-four difference vanishing at five distinct rational values
	// is identically zero. Evaluate each basis independently.
	for i := range int64(5) {
		u := big.NewRat(i, 4)
		for axis := range net[0] {
			want := evalWallBernstein(s.Control, axis, u)
			got := evalWallBernstein(net, axis, u)
			if got.Cmp(want) != 0 {
				t.Fatalf("coordinate %d at %v changed from %v to %v", axis, u, want, got)
			}
		}
	}

	for _, opts := range []edgecheck.Options{{MaxDegree: 3}, {MaxBits: 2}} {
		net, err := exactQuarticWallPCurve(s, opts, testWallDecimal)
		if net != nil || !errors.Is(err, geom.ErrConvergence) {
			t.Fatalf("degree/bit limit %+v returned a curve: %v %v", opts, net, err)
		}
	}
	s.Control[1][0] = big.NewRat(1, 7)
	if net, err := exactQuarticWallPCurve(s, edgecheck.Options{}, testWallDecimal); net != nil || err != nil {
		t.Fatalf("nonterminating quartic was accepted: %v %v", net, err)
	}
	for _, mutate := range []func(*geom.ExactBezierSpan){
		func(s *geom.ExactBezierSpan) { s.Control[0][0] = big.NewRat(1, 3) },
		func(s *geom.ExactBezierSpan) { s.Control[1][3] = big.NewRat(2, 1) },
		func(s *geom.ExactBezierSpan) { s.Control = append(s.Control, s.Control[3]) },
		func(s *geom.ExactBezierSpan) { s.End = 2 },
	} {
		s := testCubicWallSpan(3)
		mutate(&s)
		if net, err := exactQuarticWallPCurve(s, edgecheck.Options{}, testWallDecimal); net != nil || err != nil {
			t.Fatalf("unsupported input was changed: %v %v", net, err)
		}
	}
}

func TestElevateWallPCurvesKeepsOwnerAndClock(t *testing.T) {
	ctx := context.Background()
	affine := &edgecheck.Chart{}
	alreadyExact := testCubicWallSpan(4)
	b := trimmed.ExactBezierBoundary{
		Faces: []trimmed.ExactBezierBoundaryFace{
			{Loops: [][]trimmed.ExactBezierBoundaryUse{{{Edge: 5, UV: testCubicWallSpan(3)}}}},
			{Affine: affine, Loops: [][]trimmed.ExactBezierBoundaryUse{{
				{Edge: 5, Reversed: true, UV: testCubicWallSpan(3)},
				{Edge: 9, UV: alreadyExact},
			}}},
		},
	}
	if err := ElevateWallPCurves(ctx, b, edgecheck.Options{}, testWallDecimal); err != nil {
		t.Fatal(err)
	}
	roof, wall := b.Faces[0].Loops[0][0], b.Faces[1].Loops[0][0]
	if len(roof.UV.Control) != 4 || len(wall.UV.Control) != 5 || roof.Edge != wall.Edge ||
		!wall.Reversed || b.Faces[1].Affine != affine || wall.UV.Start != 0 || wall.UV.End != 1 {
		t.Fatal("degree elevation altered the support, competing owner, use or edge clock")
	}
	if got := b.Faces[1].Loops[0][1].UV.Control; len(got) != 4 || &got[0] != &alreadyExact.Control[0] {
		t.Fatal("degree elevation changed an already representable curve")
	}
}

func testCubicWallSpan(denom int64) geom.ExactBezierSpan {
	return geom.ExactBezierSpan{Start: 0, End: 1, Control: [][4]*big.Rat{
		{new(big.Rat), big.NewRat(2, 1), new(big.Rat), big.NewRat(1, 1)},
		{big.NewRat(1, denom), big.NewRat(3, 1), new(big.Rat), big.NewRat(1, 1)},
		{big.NewRat(2, denom), big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 1)},
		{big.NewRat(1, 1), big.NewRat(2, 1), new(big.Rat), big.NewRat(1, 1)},
	}}
}

func testWallDecimal(q *big.Rat) (string, error) {
	s := q.FloatString(24)
	r, ok := new(big.Rat).SetString(s)
	if !ok || r.Cmp(q) != 0 {
		return "", geom.ErrUnrepresentable
	}
	return s, nil
}

func evalWallBernstein(net [][4]*big.Rat, axis int, u *big.Rat) *big.Rat {
	total := new(big.Rat)
	n := int64(len(net) - 1)
	v := new(big.Rat).Sub(big.NewRat(1, 1), u)
	for i, row := range net {
		term := new(big.Rat).Mul(row[axis], new(big.Rat).SetInt(new(big.Int).Binomial(n, int64(i))))
		for range i {
			term.Mul(term, u)
		}
		for range len(net) - 1 - i {
			term.Mul(term, v)
		}
		total.Add(total, term)
	}
	return total
}
