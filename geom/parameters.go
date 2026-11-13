package geom

import "slices"

// Parameters returns the immutable frame and semiaxes.
func (e Ellipse) Parameters() (Frame, float64, float64) { return e.frame, e.rx, e.ry }
func (s PlaneSurface) Frame() Frame                     { return s.frame }
func (s CylinderSurface) Parameters() (Frame, float64)  { return s.frame, s.radius }

// Parameters returns the frame and radial growth per unit axial distance.
func (s ConeSurface) Parameters() (Frame, float64)           { return s.frame, s.slope }
func (s SphereSurface) Parameters() (Frame, float64)         { return s.frame, s.radius }
func (s TorusSurface) Parameters() (Frame, float64, float64) { return s.frame, s.major, s.minor }

// BSplineSurfaceData is a copy of a tensor product control net and its knots.
// Control points and weights are indexed as u*CountV+v.
type BSplineSurfaceData struct {
	DegreeU, DegreeV, CountU, CountV int
	Poles                            []Vec3
	KnotsU, KnotsV, Weights          []float64
}

func (s *BSplineSurface) Data() BSplineSurfaceData {
	if s == nil {
		return BSplineSurfaceData{}
	}
	return BSplineSurfaceData{s.degreeU, s.degreeV, s.countU, s.countV, append([]Vec3(nil), s.poles...), append([]float64(nil), s.knotsU...), append([]float64(nil), s.knotsV...), append([]float64(nil), s.weights...)}
}

// SameSplineSurface compares represented tensor bases, poles and weights.
// It does not infer geometric equality for different parameterizations.
func SameSplineSurface(a, b *BSplineSurface) bool {
	if a == nil || b == nil || len(a.poles) == 0 || len(b.poles) == 0 {
		return false
	}
	return a == b || a.degreeU == b.degreeU && a.degreeV == b.degreeV && a.countU == b.countU && a.countV == b.countV &&
		slices.Equal(a.poles, b.poles) && slices.Equal(a.knotsU, b.knotsU) && slices.Equal(a.knotsV, b.knotsV) &&
		slices.Equal(a.weights, b.weights)
}
