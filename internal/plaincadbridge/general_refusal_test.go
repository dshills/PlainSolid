package plaincadbridge

import (
	"strings"
	"testing"
)

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
