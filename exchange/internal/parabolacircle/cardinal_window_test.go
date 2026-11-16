package parabolacircle

import (
	"errors"
	"math/big"
	"slices"
	"testing"

	"plainsolid/geom"
)

func TestCardinalCircleWindowRetainsWholeOriginalClock(t *testing.T) {
	b := big.NewRat
	// This quarter has rational interior endpoints at the two 9–40–41
	// directions. Restrict it in the original half-angle clock.
	parent := [][4]*big.Rat{
		{b(41, 128), b(0, 1), b(2, 1), b(1, 1)},
		{b(41, 128), b(41, 128), b(2, 1), b(1, 1)},
		{b(0, 1), b(41, 64), b(4, 1), b(2, 1)},
	}
	first, last := b(1, 9), b(4, 5)
	model := geom.ExactBezierSpan{End: 1, Control: [][4]*big.Rat{
		circleWindowBlossom(parent, first, first),
		circleWindowBlossom(parent, first, last),
		circleWindowBlossom(parent, last, last),
	}}
	for order, want := range [][3]int64{{45, 5, 36}, {45, 9, 40}} {
		uv := geom.ExactBezierSpan{End: 1, Control: slices.Clone(model.Control)}
		for i := range uv.Control {
			uv.Control[i][2] = b(0, 1)
		}
		owners := [2]geom.ExactBezierSpan{uv, uv}
		window, err := originalCircleWindow(model, owners)
		if err != nil || ([3]int64{window.DomainEnd, window.First, window.Last}) != want {
			t.Fatal("changed exact quarter interval", order, window, err)
		}
		for i := range parent {
			row := parent[i]
			if order != 0 {
				row = parent[len(parent)-1-i]
			}
			for j, value := range row {
				if window.Model.Control[i][j].Cmp(value) != 0 {
					t.Fatal("changed finite-decimal original carrier", i, j)
				}
			}
		}

		// A homogeneous geometric progression keeps the curve image and
		// endpoint positions, but changes the interior parameter law.
		shifted := geom.ExactBezierSpan{End: 1, Control: make([][4]*big.Rat, 3)}
		for i, row := range uv.Control {
			for j, value := range row {
				shifted.Control[i][j] = new(big.Rat).Mul(value, b(1<<i, 1))
			}
		}
		if _, err := originalCircleWindow(model, [2]geom.ExactBezierSpan{uv, shifted}); !errors.Is(err, geom.ErrUnrepresentable) {
			t.Fatal("accepted a different original coedge clock", err)
		}
		slices.Reverse(model.Control)
	}
}

func TestOriginalVertexCircleWindowRetainsSubarcClock(t *testing.T) {
	b := big.NewRat
	anchor := geom.Vec3{X: 11. / 16, Y: 3. / 4, Z: 2}
	// Rotate the original radial vector (3,4)/16 by a quarter turn.
	parent := [][4]*big.Rat{
		{b(11, 16), b(3, 4), b(2, 1), b(1, 1)},
		{b(7, 16), b(15, 16), b(2, 1), b(1, 1)},
		{b(1, 2), b(11, 8), b(4, 1), b(2, 1)},
	}
	first, last := b(1, 7), b(2, 7)
	model := geom.ExactBezierSpan{End: 1, Control: [][4]*big.Rat{
		circleWindowBlossom(parent, first, first),
		circleWindowBlossom(parent, first, last),
		circleWindowBlossom(parent, last, last),
	}}
	uv := geom.ExactBezierSpan{End: 1, Control: slices.Clone(model.Control)}
	for i := range uv.Control {
		uv.Control[i][2] = b(0, 1)
	}
	owners := [2]geom.ExactBezierSpan{uv, uv}
	if _, err := originalCircleWindow(model, owners); !errors.Is(err, geom.ErrUnrepresentable) {
		t.Fatal("cardinal-only path accepted a different original clock", err)
	}
	w, err := originalCircleWindow(model, owners, anchor)
	if err != nil || [3]int64{w.DomainEnd, w.First, w.Last} != [3]int64{7, 1, 2} {
		t.Fatal("changed exact original-vertex interval", w, err)
	}
	for i, row := range parent {
		for j, x := range row {
			if w.Model.Control[i][j].Cmp(x) != 0 {
				t.Fatal("changed exact original-vertex carrier pole", i, j)
			}
		}
	}
	owners[1].Control = slices.Clone(uv.Control)
	owners[1].Control[1][0] = new(big.Rat).Add(uv.Control[1][0], b(1, 65536))
	if _, err := originalCircleWindow(model, owners, anchor); !errors.Is(err, geom.ErrUnrepresentable) {
		t.Fatal("accepted an unmatched original UV coedge", err)
	}
}
