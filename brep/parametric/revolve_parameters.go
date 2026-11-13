package parametric

import "plainsolid/geom"

// RevolutionSideParameters exposes the immutable analytic meridian recipe of
// an unplaced revolution side. Other evaluators, including mapped surfaces,
// return false; callers must not infer a revolution from sampled evaluations.
func RevolutionSideParameters(surface geom.Surface) (geom.Frame, geom.Vec2, geom.Vec2, bool) {
	s, ok := surface.(revolvedSide)
	if !ok {
		return geom.Frame{}, geom.Vec2{}, geom.Vec2{}, false
	}
	return s.frame, s.a, s.b, true
}

// IsRevolutionGenerator reports an exact linear meridian iso-curve from the
// native revolution constructor. Circular rims and unknown curves return false.
func IsRevolutionGenerator(curve geom.Curve) bool {
	c, ok := curve.(iso)
	if !ok || (c.side != 1 && c.side != 3) {
		return false
	}
	_, ok = c.face.Surface.(revolvedSide)
	return ok
}
