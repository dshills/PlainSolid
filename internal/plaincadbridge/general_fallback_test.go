package plaincadbridge

import (
	"encoding/base64"
	"errors"
	"math"
	"testing"
)

// TestOrGeneral states the rule of the fallback: the general Boolean is
// asked only where the legacy path refuses as unsupported, and where it
// fails too the legacy refusal is the answer.
func TestOrGeneral(t *testing.T) {
	unsupported, invalid := &failureError{"UNSUPPORTED", "outside the family"}, &failureError{"INVALID", "bad input"}
	plain, broken := errors.New("legacy failed"), errors.New("general failed")
	for _, c := range []struct {
		name       string
		legacy     error
		general    error
		want       string
		wantErr    error
		wantCalled bool
	}{
		{"legacy answers", nil, nil, "legacy", nil, false},
		{"legacy fails otherwise", plain, nil, "legacy", plain, false},
		{"legacy rejects the input", invalid, nil, "legacy", invalid, false},
		{"legacy declines, general answers", unsupported, nil, "general", nil, true},
		{"both decline", unsupported, broken, "legacy", unsupported, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			called := false
			got, err := orGeneral("legacy", c.legacy, func() (string, error) {
				called = true
				return "general", c.general
			})
			if got != c.want || err != c.wantErr || called != c.wantCalled {
				t.Fatalf("answer %q, error %v, general asked %v", got, err, called)
			}
		})
	}
}

// TestGeneralFallback takes a box with a plain bore, which a legacy path
// cuts, through a cut and a fuse that no legacy path takes, and then
// through what a part needs afterwards.
func TestGeneralFallback(t *testing.T) {
	b := newLegacy()
	type q = map[string]any
	stock := box(t, b, vector{}, vector{20, 10, 10})
	tool := circle(t, b, vector{5, 5, 0}, vector{0, 0, 1}, 2, 10)
	bored := shape(t, b, q{"op": "cut", "a": stock.Handle, "b": tool.Handle})
	if b.shapes[bored.Handle].bore == nil {
		t.Fatal("the plain bore is no longer cut by its legacy path")
	}
	near(t, bored.GeometryAssertions.Volume, 2000-40*math.Pi)

	// A slot across the top, clear of the bore: 4 by 10 by 4.
	slot := box(t, b, vector{12, -1, 6}, vector{16, 11, 11})
	slotted := declined(t, b, "", "cut", bored.Handle, slot.Handle)
	near(t, slotted.GeometryAssertions.Volume, 2000-40*math.Pi-160)
	// The bore trades two disks for its wall. The slot trades its strip of
	// the top for its floor, adds two walls and opens two ends.
	near(t, slotted.GeometryAssertions.SurfaceArea, 1000+(40-8)*math.Pi+80-32)

	// A boss on the top of the slotted part, clear of the bore and of the
	// slot, of radius 1 and height 3. The legacy paths answer first here
	// too, and take no result of the general Boolean as stock.
	boss := circle(t, b, vector{9.5, 5, 10}, vector{0, 0, 1}, 1, 3)
	bossed := declined(t, b, "", "fuse", slotted.Handle, boss.Handle)
	near(t, bossed.GeometryAssertions.Volume, slotted.GeometryAssertions.Volume+3*math.Pi)
	near(t, bossed.GeometryAssertions.SurfaceArea, slotted.GeometryAssertions.SurfaceArea+6*math.Pi)

	for _, part := range []Shape{slotted, bossed} {
		// A rigid motion keeps the measures and moves the bounds.
		moved := shape(t, b, q{"op": "transform", "handle": part.Handle, "matrix": [][]float64{{1, 0, 0, 7}, {0, 1, 0, 0}, {0, 0, 1, 0}}})
		near(t, moved.GeometryAssertions.Volume, part.GeometryAssertions.Volume)
		near(t, moved.Bounds.Min[0], part.Bounds.Min[0]+7)
		near(t, moved.Bounds.Max[0], part.Bounds.Max[0]+7)

		// The mesh is closed and holds the volume to the chord error.
		m, err := b.shapes[part.Handle].tessellate(request{})
		if err != nil {
			t.Fatal(err)
		}
		if err := m.ValidateSolid(); err != nil {
			t.Fatal(err)
		}
		if got := m.SignedVolume(); math.Abs(got/part.GeometryAssertions.Volume-1) > .01 {
			t.Fatalf("mesh volume %.9g of a part of %.9g", got, part.GeometryAssertions.Volume)
		}

		// STEP export writes the boundary and reads it back to the same
		// measures, and importStep reads the file on either bridge.
		exported, err := b.exportSTEP([]string{part.Handle})
		if err != nil {
			t.Fatal(err)
		}
		near(t, exported.After[0].Volume, part.GeometryAssertions.Volume)
		near(t, exported.After[0].SurfaceArea, part.GeometryAssertions.SurfaceArea)
		file, err := base64.StdEncoding.DecodeString(exported.Base64)
		if err != nil {
			t.Fatal(err)
		}
		back := shape(t, b, q{"op": "importStep", "source": string(file)})
		sameShape(t, back, part)
		sameShape(t, shape(t, newGeneral(), q{"op": "importStep", "source": string(file)}), part)
		// The part that came back is a result like the one that left: it
		// exports again.
		if _, err := b.exportSTEP([]string{back.Handle}); err != nil {
			t.Fatal(err)
		}

		// A further bore through the part, clear of what is there.
		second := circle(t, b, vector{10, 2, 0}, vector{0, 0, 1}, 1, 10)
		again := shape(t, b, q{"op": "cut", "a": part.Handle, "b": second.Handle})
		near(t, again.GeometryAssertions.Volume, part.GeometryAssertions.Volume-10*math.Pi)
	}
}

// TestGeneralFirst holds the default bridge to the requests that the
// general Boolean answers first (L3): a cut of cylinders from planar
// stock, and any request on a result of it. Such a result keeps the stock
// and the cylinders it was cut from. A bore that the general Boolean does
// not cut from it, here one of another radius whose axis passes the
// first's, is cut by the legacy bore engine from that history, also after
// the result has been mirrored. Other stock stays with its legacy path.
func TestGeneralFirst(t *testing.T) {
	type q = map[string]any
	b := New()
	stock := box(t, b, vector{0, 0, 0}, vector{20, 24, 18})
	x := circle(t, b, vector{0, 12, 9}, vector{1, 0, 0}, 5, 20)
	bored := shape(t, b, q{"op": "cut", "a": stock.Handle, "b": x.Handle})
	near(t, bored.GeometryAssertions.Volume, 8640-500*math.Pi)
	history := func(s Shape) int {
		t.Helper()
		body := b.shapes[s.Handle].analytic
		if body == nil || !body.general || body.bores == nil {
			t.Fatalf("%s is not a result of the general Boolean with a bore history", s.Handle)
		}
		return len(body.bores.cylinders)
	}
	if history(bored) != 1 {
		t.Fatal("the history of one bore does not hold one cylinder")
	}

	// A second bore clear of the first: the general Boolean again, as a
	// cut and as a group, and the history grows.
	clear := circle(t, b, vector{16, 4, 18}, vector{0, 0, -1}, 1, 3)
	for _, request := range []q{
		{"op": "cut", "a": bored.Handle, "b": clear.Handle},
		{"op": "cutMany", "a": stock.Handle, "tools": []string{clear.Handle, x.Handle}},
	} {
		twice := shape(t, b, request)
		near(t, twice.GeometryAssertions.Volume, 8640-503*math.Pi)
		if history(twice) != 2 {
			t.Fatal("the history of two bores does not hold two cylinders")
		}
	}

	// A bore of radius two whose axis passes one unit from the first's.
	// The volume is the one TestSkewCrossedBoreScopeAndSTEP holds the
	// legacy path to.
	const crossed = 6963.3892873778959
	mirror := [][]float64{{-1, 0, 0, 20}, {0, 1, 0, 0}, {0, 0, 1, 0}}
	moved := shape(t, b, q{"op": "transform", "handle": bored.Handle, "matrix": mirror})
	if history(moved) != 1 {
		t.Fatal("the mirrored result lost its history")
	}
	for _, c := range []struct {
		name string
		from Shape
		at   float64
	}{{"as cut", bored, 8}, {"mirrored", moved, 12}} {
		askew := circle(t, b, vector{c.at, 13, 18}, vector{0, 0, -1}, 2, 18)
		removed := overlapMeasure(t, b, c.from.Handle, askew.Handle)
		out := shape(t, b, q{"op": "cut", "a": c.from.Handle, "b": askew.Handle})
		if math.Abs(out.GeometryAssertions.Volume-crossed) > 1e-5 || out.Bounds != stock.Bounds {
			t.Fatalf("%s: %+v, want volume %.10g", c.name, out, crossed)
		}
		near(t, out.GeometryAssertions.Volume, c.from.GeometryAssertions.Volume-removed)
		if b.shapes[out.Handle].bore == nil {
			t.Fatalf("%s: the bore askew was not cut by the legacy bore engine", c.name)
		}
	}

	// A coaxial cut of a cylinder is not a plain bore: its legacy path
	// answers, and the tube keeps the recipe that a thread needs.
	rod := circle(t, b, vector{40, 0, 0}, vector{0, 0, 1}, 5, 10)
	drill := circle(t, b, vector{40, 0, -1}, vector{0, 0, 1}, 2, 12)
	tube := shape(t, b, q{"op": "cut", "a": rod.Handle, "b": drill.Handle})
	if body := b.shapes[tube.Handle].analytic; body == nil || body.general || body.sector == nil {
		t.Fatal("the coaxial cut of a cylinder was not answered by its legacy path")
	}

	// On a bridge whose legacy paths answer first, the plain bore is a
	// legacy bore body.
	old := newLegacy()
	legacyBored := shape(t, old, q{"op": "cut", "a": box(t, old, vector{0, 0, 0}, vector{20, 24, 18}).Handle, "b": circle(t, old, vector{0, 12, 9}, vector{1, 0, 0}, 5, 20).Handle})
	if old.shapes[legacyBored.Handle].bore == nil {
		t.Fatal("the legacy-first bridge did not cut the plain bore with a legacy path")
	}
	sameShape(t, bored, legacyBored)
}
