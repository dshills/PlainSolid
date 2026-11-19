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
	b := New()
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
