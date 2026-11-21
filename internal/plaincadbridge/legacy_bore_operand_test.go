package plaincadbridge

import (
	"math"
	"strings"
	"testing"
)

// TestLegacyBoreBodyCutAcross cuts a blind bore across a bore that the
// legacy bore engine built, with its floor inside that bore. No legacy
// path takes the pair, and the general Boolean takes the legacy body as
// its stock cut again (section 13 of specs/general-boolean-crossing):
// the wall as the legacy engine divided it has a ruling where the two
// bores cross, and the cut of that solid was refused. The overlap and
// the cut are those of a default bridge, where the first bore is a
// result of the general Boolean.
func TestLegacyBoreBodyCutAcross(t *testing.T) {
	type q = map[string]any
	for _, depth := range []float64{6, 7.2} {
		var volume, overlap [2]float64
		for i, open := range []func() *Bridge{New, newLegacy} {
			b := open()
			stock := box(t, b, vector{0, 0, 0}, vector{20, 16, 12})
			x := circle(t, b, vector{0, 8, 6}, vector{1, 0, 0}, 2, 20)
			z := circle(t, b, vector{10, 8, 12}, vector{0, 0, -1}, 2, depth)
			first := shape(t, b, q{"op": "cut", "a": stock.Handle, "b": x.Handle})
			if legacy := b.shapes[first.Handle].bore != nil; legacy != (i == 1) {
				t.Fatalf("bridge %d: the first bore is a legacy body: %v", i, legacy)
			}
			overlap[i] = overlapMeasure(t, b, first.Handle, z.Handle)
			volume[i] = shape(t, b, q{"op": "cut", "a": first.Handle, "b": z.Handle}).GeometryAssertions.Volume
		}
		if math.Abs(volume[1]-volume[0]) > 1e-9*volume[0] || math.Abs(overlap[1]-overlap[0]) > 1e-9*overlap[0] {
			t.Fatalf("depth %g: volume %.12g and overlap %.12g on the legacy body, want %.12g and %.12g", depth, volume[1], overlap[1], volume[0], overlap[0])
		}
	}
}

// TestRefusalOfTheGeneralBooleanStands asks for two bores that touch
// along a line, which no path cuts. No legacy family takes two crossing
// bores, so the reason given is that of the general Boolean and not the
// last legacy path's, which is that it has no such family.
func TestRefusalOfTheGeneralBooleanStands(t *testing.T) {
	b := New()
	stock := box(t, b, vector{0, 0, 0}, vector{20, 16, 12})
	x := circle(t, b, vector{0, 8, 6}, vector{1, 0, 0}, 2, 20)
	z := circle(t, b, vector{10, 8, 0}, vector{0, 0, 1}, 2, 4)
	_, err := b.cutMany(stock.Handle, []string{z.Handle, x.Handle})
	if err == nil || !strings.Contains(err.Error(), "touch") {
		t.Fatalf("got %v, want the refusal of the general Boolean, that the bores touch", err)
	}
}
