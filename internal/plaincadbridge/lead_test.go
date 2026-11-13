package plaincadbridge

import (
	"math"
	"testing"
)

func TestNativeLeadPair(t *testing.T) {
	for _, hand := range []int{1, -1} {
		b := New()
		d := leadRequest{Major: 4, Minor: 3, Bore: 3.1, NutOuter: 6, Pitch: 2, Lead: 8,
			ScrewLength: 18, NutLength: 10, Clearance: .1, Slope: number(math.Tan(math.Pi / 12)), Starts: 4, Hand: hand}
		a, err := b.createLeadPart(d, false)
		if err != nil {
			t.Fatal("screw", hand, err)
		}
		c, err := b.createLeadPart(d, true)
		if err != nil {
			t.Fatal("nut", hand, err)
		}
		if err := b.checkLeadPair(b.shapes[a.Handle], b.shapes[c.Handle], d); err != nil {
			t.Fatal(hand, err)
		}
		if a.GeometryAssertions.Volume <= 680 || a.GeometryAssertions.Volume >= 780 ||
			c.GeometryAssertions.Volume <= 580 || c.GeometryAssertions.Volume >= 720 {
			t.Fatal(a.GeometryAssertions, c.GeometryAssertions)
		}
		near(t, a.Bounds.Max[2], 18)
		near(t, c.Bounds.Max[2], 10)
	}
}
