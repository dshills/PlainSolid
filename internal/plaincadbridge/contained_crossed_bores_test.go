package plaincadbridge

import (
	"math"
	"testing"
)

func TestContainedCrossedBores(t *testing.T) {
	b := New()
	stock := box(t, b, vector{0, 0, 0}, vector{20, 24, 18})
	x := circle(t, b, vector{0, 9, 7}, vector{1, 0, 0}, 3, 20)
	short := circle(t, b, vector{8, 9, 18}, vector{0, 0, -1}, 2, 13)
	blind := circle(t, b, vector{8, 9, 18}, vector{0, 0, -1}, 3, 15)
	deep := circle(t, b, vector{8, 9, 18}, vector{0, 0, -1}, 3, 16)
	through := circle(t, b, vector{8, 9, 0}, vector{0, 0, 1}, 3, 18)
	target, err := b.cutMany(stock.Handle, []string{blind.Handle, x.Handle, short.Handle, blind.Handle, x.Handle})
	if err != nil {
		t.Fatal(err)
	}
	near(t, target.GeometryAssertions.Volume, 20.*24*18-35*9*math.Pi+144)
	near(t, overlapMeasure(t, b, target.Handle, short.Handle), 0)
	near(t, overlapMeasure(t, b, target.Handle, deep.Handle), 9*math.Pi)
	deeper := shape(t, b, map[string]any{"op": "cut", "a": target.Handle, "b": deep.Handle})
	near(t, deeper.GeometryAssertions.Volume, target.GeometryAssertions.Volume-9*math.Pi)
	open, err := b.cutMany(deeper.Handle, []string{short.Handle, through.Handle, deep.Handle})
	if err != nil {
		t.Fatal(err)
	}
	near(t, open.GeometryAssertions.Volume, 20.*24*18-38*9*math.Pi+144)
	baseOpen, err := b.cutMany(stock.Handle, []string{x.Handle, through.Handle})
	if err != nil {
		t.Fatal(err)
	}
	first, err := b.exportSTEP([]string{open.Handle})
	if err != nil {
		t.Fatal(err)
	}
	other, err := b.exportSTEP([]string{baseOpen.Handle})
	if err != nil {
		t.Fatal(err)
	}
	sameSTEP(t, b, first, other)
	if _, err := b.cutMany(deeper.Handle, []string{short.Handle, blind.Handle}); err == nil {
		t.Fatal("redundant set did not reject")
	}
}
