package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestHelicalWholeRadialClearance(t *testing.T) {
	ctx := context.Background()
	f, _ := NewFrame(Vec3{}, Vec3{1, 0, 0}, Vec3{0, 0, 1})
	triangle := []Vec2{{2, 0}, {3, .5}, {2, 1}}
	a, _ := NewHelicalPolygon(f, triangle, 2, RightHanded, -1, 2)
	c, _ := NewHelicalPolygon(f, triangle, 2, RightHanded, 0, 3)
	r, e := CompareHelicalRadialBand(ctx, a, c, [2]*big.Rat{big.NewRat(1, 1), big.NewRat(4, 1)}, big.NewRat(1, 3), HelicalClearanceBandOptions{})
	if e != nil || !r.Complete || !r.HasBand || r.Minimum.Cmp(big.NewRat(-2, 3)) != 0 || r.Maximum.Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal(r, e)
	}
	// Independent gap: 1/3 - (3-radius), zero at8/3.
	found := false
	for _, event := range r.Events {
		if event.Radius.Cmp(big.NewRat(8, 3)) == 0 {
			found = event.Clearance.Sign() == 0
		}
		want := new(big.Rat).Sub(event.Radius, big.NewRat(8, 3))
		if event.Clearance.Cmp(want) != 0 {
			t.Fatal(event, want)
		}
	}
	if !found {
		t.Fatal("missing rational contact")
	}
	if r.Cells[0].InteriorSign != -1 || r.Cells[len(r.Cells)-1].InteriorSign != 1 {
		t.Fatal(r.Cells)
	}
	touching, e := CompareHelicalRadialBand(ctx, a, c, [2]*big.Rat{big.NewRat(2, 1), big.NewRat(3, 1)}, big.NewRat(0, 1), HelicalClearanceBandOptions{})
	if e != nil || touching.Maximum.Sign() != 0 || touching.Minimum.Cmp(big.NewRat(-1, 1)) != 0 {
		t.Fatal(touching, e)
	}
	// Opposing slanted strips switch directed-gap dominance at an interior radius.
	stripA, _ := NewHelicalPolygon(f, []Vec2{{2, 0}, {3, 0}, {3, 1}, {2, 1}}, 2, RightHanded, 0, 1)
	stripB, _ := NewHelicalPolygon(f, []Vec2{{2, -1}, {3, 1}, {3, 2}, {2, 0}}, 2, RightHanded, 0, 1)
	switched, e := CompareHelicalRadialBand(ctx, stripA, stripB, [2]*big.Rat{big.NewRat(2, 1), big.NewRat(3, 1)}, big.NewRat(0, 1), HelicalClearanceBandOptions{})
	if e != nil || switched.Minimum.Cmp(big.NewRat(-1, 1)) != 0 || switched.Maximum.Sign() != 0 {
		t.Fatal(switched, e)
	}
	hasSwitch := false
	for _, event := range switched.Events {
		if event.Radius.Cmp(big.NewRat(5, 2)) == 0 && event.Clearance.Cmp(big.NewRat(-1, 1)) == 0 {
			hasSwitch = true
		}
	}
	if !hasSwitch {
		t.Fatal("missing switch")
	}

	diamond, _ := NewHelicalPolygon(f, []Vec2{{2, .5}, {2.5, 0}, {3, .5}, {2.5, 1}}, 2, RightHanded, 0, 1)
	multi, e := CompareHelicalRadialBand(ctx, diamond, diamond, [2]*big.Rat{big.NewRat(2, 1), big.NewRat(3, 1)}, big.NewRat(1, 4), HelicalClearanceBandOptions{})
	if e != nil || multi.Minimum.Cmp(big.NewRat(-3, 4)) != 0 || multi.Maximum.Cmp(big.NewRat(1, 4)) != 0 {
		t.Fatal(multi, e)
	}
	zeroCount := 0
	for _, event := range multi.Events {
		if event.Clearance.Sign() == 0 {
			zeroCount++
			if event.Radius.Cmp(big.NewRat(17, 8)) != 0 && event.Radius.Cmp(big.NewRat(23, 8)) != 0 {
				t.Fatal(event)
			}
		}
	}
	if zeroCount != 2 {
		t.Fatal("missing two contact radii", multi)
	}
	if bad, e := CompareHelicalRadialBand(ctx, a, c, [2]*big.Rat{big.NewRat(2, 1), big.NewRat(3, 1)}, big.NewRat(1, 3), HelicalClearanceBandOptions{MaxCells: 1}); e == nil || bad.Complete || len(bad.Cells) != 0 {
		t.Fatal("cell cap", bad, e)
	}
}
