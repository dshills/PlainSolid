package plaincadbridge

import (
	"encoding/base64"
	"math"
	"testing"
)

// TestBlindFloorsInsideAnObliqueCrossing cuts a bore along an axis and a
// bore oblique to it, both blind, with both floors inside the crossing
// (section 15 of specs/general-boolean-crossing). Each case was refused
// before for a reason of its own:
//
//   - the wall of the oblique bore touches itself at the two points where
//     the bores cross, and the chord that divides it runs from a corner
//     round to the seam, where no vertex was;
//   - the floor of the oblique bore has a hole that comes within 0.004 of
//     its rim, nearer than the arcs of the rim stand off their chords;
//   - a ruling that divides a wall ends at a vertex placed by a touching
//     hit, a hundred-millionth along the ruling's cylinder, and the STEP
//     reader took the edge for no ruling.
//
// The cut is the same in either order and as one cutMany, its volume is
// that of the first cut less the overlap of the second tool, and it is
// written to STEP and read back. In the third case the floor along the
// axis passes exactly through the two points where the bores cross; the
// other order of the cuts is still refused there, so only the cutMany is
// made, and its volume is the one the pinned OCCT build measures of the
// file to nine digits.
func TestBlindFloorsInsideAnObliqueCrossing(t *testing.T) {
	type q = map[string]any
	for _, c := range []struct {
		name           string
		origin, normal vector
		height         float64 // of the oblique tool
		entry, into    vector  // of the tool along the axis
		depth          float64
		removedByAxis  float64 // the volume the tool along the axis removes alone
		volume         float64 // of the cutMany, where the other orders are refused
	}{
		{"to the seam", vector{-4, 8, 40 - 56./3}, vector{3, 0, 4}, 70./3 + 2.5071, vector{10, 8, 80}, vector{0, 0, -1}, 40.5137, 4 * math.Pi * 40.5137, 0},
		{"a hole near the rim", vector{-4, 8, 29.5}, vector{4, 0, 3}, 17.5 + 1.0071, vector{10, 8, 0}, vector{0, 0, 1}, 41.0137, 4 * math.Pi * 41.0137, 0},
		{"a ruling from a touching hit", vector{24, 8, 50.5}, vector{-4, 0, -3}, 17.5 + 1, vector{10, 8, 80}, vector{0, 0, -1}, 40, 4 * math.Pi * 40, 24953.7016274},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			b := New()
			stock := box(t, b, vector{0, 0, 0}, vector{20, 16, 80})
			oblique := shape(t, b, q{"op": "circle", "origin": c.origin, "normal": c.normal, "radius": 2, "height": c.height})
			axial := shape(t, b, q{"op": "circle", "origin": c.entry, "normal": c.into, "radius": 2, "height": c.depth})
			many, err := b.cutMany(stock.Handle, []string{oblique.Handle, axial.Handle})
			if err != nil {
				t.Fatal(err)
			}
			cuts, want := []Shape{many}, c.volume
			if want == 0 {
				first := shape(t, b, q{"op": "cut", "a": stock.Handle, "b": axial.Handle})
				want = 20*16*80 - c.removedByAxis - overlapMeasure(t, b, first.Handle, oblique.Handle)
				second := shape(t, b, q{"op": "cut", "a": first.Handle, "b": oblique.Handle})
				other := shape(t, b, q{"op": "cut", "a": shape(t, b, q{"op": "cut", "a": stock.Handle, "b": oblique.Handle}).Handle, "b": axial.Handle})
				cuts = append(cuts, second, other)
			}
			for i, got := range cuts {
				if g := got.GeometryAssertions; !g.Valid || g.SolidCount != 1 || math.Abs(g.Volume-want) > 1e-8*want {
					t.Fatalf("cut %d: %+v, want volume %.12g", i, g, want)
				}
			}
			file, err := b.exportSTEP([]string{many.Handle})
			if err != nil {
				t.Fatal(err)
			}
			source, err := base64.StdEncoding.DecodeString(file.Base64)
			if err != nil {
				t.Fatal(err)
			}
			read, err := b.importSTEP(string(source))
			if err != nil {
				t.Fatal(err)
			}
			sameShape(t, read, many)
		})
	}
}
