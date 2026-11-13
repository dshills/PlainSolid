package parametric

import (
	"bytes"
	"math/big"

	"plainsolid/geom"
)

// AxialSteppedCylinderMeridian recognizes the complete native boundary of two
// axial cylindrical sections with unequal radii and one annular shoulder.
// Its six returned vertices are ordered from the axis along the low cap,
// both walls and shoulder, then the high cap back to the axis. Recognition
// requires the original revolution supports and the entire canonical topology;
// coincident sample points, edited seams or detached pieces do not qualify.
func AxialSteppedCylinderMeridian(s Solid) (geom.Frame, [6]geom.Vec2, bool) {
	var frame geom.Frame
	var profile [6]geom.Vec2
	if len(s.faces) != 5 {
		return geom.Frame{}, profile, false
	}
	for i, f := range s.faces {
		p, ok := f.Surface.(revolvedSide)
		if !ok || f.U != circleDomain || f.V != normalized || f.Reversed || i != 0 && (p.frame != frame || p.a != profile[i]) {
			return geom.Frame{}, [6]geom.Vec2{}, false
		}
		if i == 0 {
			frame, profile[0] = p.frame, p.a
		}
		profile[i+1] = p.b
	}
	if profile[0] != (geom.Vec2{}) || profile[5].X != 0 || profile[1].Y != 0 || profile[1].X <= 0 || profile[3].X <= 0 ||
		profile[1].X != profile[2].X || profile[2].Y <= 0 || profile[2].Y != profile[3].Y || profile[2].X == profile[3].X ||
		profile[3].X != profile[4].X || profile[4].Y <= profile[3].Y || profile[4].Y != profile[5].Y {
		return geom.Frame{}, [6]geom.Vec2{}, false
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	axis, scale := coordinateCylinderAxis(frame.ZAxis())
	_, xScale := coordinateCylinderAxis(frame.XAxis())
	if scale != 1 || xScale != 1 {
		return geom.Frame{}, [6]geom.Vec2{}, false
	}
	o := frame.Origin()
	for _, z := range []float64{profile[2].Y, profile[4].Y} {
		for i, d := range [3]float64{axis.X, axis.Y, axis.Z} {
			if _, err := geom.RepresentedExactScalar(new(big.Rat).Add(q([3]float64{o.X, o.Y, o.Z}[i]), new(big.Rat).Mul(q(d), q(z)))); err != nil {
				return geom.Frame{}, [6]geom.Vec2{}, false
			}
		}
	}
	if _, err := geom.RepresentedExactScalar(new(big.Rat).Sub(q(profile[4].Y), q(profile[2].Y))); err != nil {
		return geom.Frame{}, [6]geom.Vec2{}, false
	}
	canonical, err := Revolve(frame, profile[:])
	if err != nil {
		return geom.Frame{}, [6]geom.Vec2{}, false
	}
	var actual, expected bytes.Buffer
	if WriteJSON(&actual, s) != nil || WriteJSON(&expected, canonical) != nil || !bytes.Equal(actual.Bytes(), expected.Bytes()) {
		return geom.Frame{}, [6]geom.Vec2{}, false
	}
	return frame, profile, true
}
