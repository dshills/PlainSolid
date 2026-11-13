package parametric

import "plainsolid/geom"

// IsoCurveParameters describes a built-in edge evaluator as a straight
// parameter path on its support. The curve parameter runs from zero to one.
// A placement applied to the model curve is also applied to the support.
func IsoCurveParameters(c geom.Curve) (surface geom.Surface, start, end geom.Vec2, ok bool) {
	base, placement := CurveMapping(c)
	x, ok := base.(iso)
	if !ok {
		return nil, start, end, false
	}
	for i, dst := range []*geom.Vec2{&start, &end} {
		u, v := sideUV(x.side, float64(i))
		*dst = geom.Vec2{X: x.face.U.at(u), Y: x.face.V.at(v)}
	}
	surface = x.face.Surface
	if placement != geom.Identity() {
		surface = MapSurface(surface, placement)
	}
	return surface, start, end, true
}
