package plaincadbridge

import (
	"math"
	"testing"
)

func TestCircleDisplayRetainsProductionSTLVolume(t *testing.T) {
	b := New()
	s := circle(t, b, vector{}, vector{0, -1, 0}, 10, 10)
	o := request{LinearDeflection: optionalNumber{value: .5, set: true}, AngularDeflection: optionalNumber{value: .2, set: true}}
	m, err := b.shapes[s.Handle].tessellate(o)
	if err != nil {
		t.Fatal(err)
	}
	p, err := m.MassProperties()
	if err != nil || math.Abs(p.Volume/s.GeometryAssertions.Volume-1) >= .005 {
		t.Fatal(p.Volume, s.GeometryAssertions.Volume, err)
	}
}
