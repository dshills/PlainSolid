package brep

import (
	"plainsolid/geom"
	"testing"
)

func TestHoledProjectionPreservesCrossLoopCollinearity(t *testing.T) {
	// Exact binary-rational coordinates from a two-start helical bore UV chart.
	// Division by the span5 perturbed cross-loop collinearity and selected a
	// triangle whose three ORIGINAL vertices were exactly collinear.
	outer := []geom.Vec3{{}, {X: 1}, {X: 1, Y: 5}, {Y: 5}}
	holes := [][]geom.Vec3{{{X: .625, Y: 1.5}, {X: .625, Y: 3.5}, {X: .875, Y: 3.25}, {X: .875, Y: 1.25}}, {{X: .125, Y: 2}, {X: .125, Y: 4}, {X: .375, Y: 3.75}, {X: .375, Y: 1.75}}}
	for _, offset := range []float64{0, 1048576} {
		for axis := 0; axis < 3; axis++ {
			mapPoint := func(p geom.Vec3) geom.Vec3 {
				p.X += offset
				p.Y -= offset
				switch axis {
				case 0:
					return geom.Vec3{X: p.Z, Y: p.X, Z: p.Y}
				case 1:
					return geom.Vec3{X: p.Y, Y: p.Z, Z: p.X}
				default:
					return p
				}
			}
			out := make([]geom.Vec3, len(outer))
			for i, p := range outer {
				out[i] = mapPoint(p)
			}
			in := make([][]geom.Vec3, len(holes))
			for i, ring := range holes {
				for _, p := range ring {
					in[i] = append(in[i], mapPoint(p))
				}
			}
			direction := mapPoint(geom.Vec3{Z: 1}).Sub(mapPoint(geom.Vec3{}))
			s, e := PrismWithHoles(out, in, direction)
			if e != nil {
				t.Fatal(offset, axis, e)
			}
			m := s.Tessellate()
			if e := m.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
			if m.SignedVolume() != 4 {
				t.Fatal("exact analytic prism volume", offset, axis, m.SignedVolume())
			}
		}
	}
}
