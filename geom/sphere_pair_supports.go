package geom

// ExposedSphereSupports identifies original spheres with a nonempty exposed
// boundary patch in the regularized set. Coincident union/intersection keeps
// both owners for the same support. It does not assert manifold topology.
func (s SpherePairMaterial) ExposedSphereSupports() (BoreBoundaryMask, error) {
	if s.radius2[0] == nil {
		return 0, ErrInvalid
	}
	switch s.mode {
	case 1:
		return SpherePairFirst, nil
	case 2:
		return SpherePairSecond, nil
	case 3:
		return 0, nil
	default:
		return SpherePairFirst | SpherePairSecond, nil
	}
}

// HasTransverseBoundaryIntersection proves a genuine intersection circle of
// the two original sphere supports, independent of the selected set operation.
func (s SpherePairMaterial) HasTransverseBoundaryIntersection() (bool, error) {
	if s.radius2[0] == nil {
		return false, ErrInvalid
	}
	return s.transverse, nil
}
