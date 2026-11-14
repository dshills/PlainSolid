package trimmed

import (
	"math"

	"plainsolid/geom"
)

// CrossedObliqueAxisCoefficients describes the normalized oblique axis,
// projection onto an X cap, and both intersecting cylinder bisectors.
type CrossedObliqueAxisCoefficients struct {
	Sin, Cos, Slope, Stretch, Positive, Negative float64
}

var crossedObliqueAxisRatios = [...]geom.Vec2{
	{X: 4, Y: 3}, {X: 3, Y: 4}, {X: 12, Y: 5}, {X: 5, Y: 12},
}

// CrossedObliqueCoefficients returns the exact geometric formulas evaluated
// on a supported represented Pythagorean ratio.
func CrossedObliqueCoefficients(ratio geom.Vec2) (CrossedObliqueAxisCoefficients, bool) {
	h := 5.
	switch ratio {
	case crossedObliqueAxisRatios[0], crossedObliqueAxisRatios[1]:
	case crossedObliqueAxisRatios[2], crossedObliqueAxisRatios[3]:
		h = 13
	default:
		return CrossedObliqueAxisCoefficients{}, false
	}
	return CrossedObliqueAxisCoefficients{
		Sin: ratio.X / h, Cos: ratio.Y / h, Slope: ratio.Y / ratio.X, Stretch: h / ratio.X,
		Positive: (h + ratio.Y) / ratio.X, Negative: -(h - ratio.Y) / ratio.X,
	}, true
}

// CrossedObliqueAxisRatio identifies the supported represented unit direction
// from its positive components. The caller proves the lines and placement.
func CrossedObliqueAxisRatio(ax, az float64) (geom.Vec2, bool) {
	for _, ratio := range crossedObliqueAxisRatios {
		c, _ := CrossedObliqueCoefficients(ratio)
		if math.Abs(ax-c.Sin) <= 8*0x1p-52 && math.Abs(az-c.Cos) <= 8*0x1p-52 {
			return ratio, true
		}
	}
	return geom.Vec2{}, false
}
