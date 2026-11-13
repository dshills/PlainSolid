package geom

// PermutedModelAxes returns an immutable surface whose model components are the
// selected permutation of original XYZ (0=X,1=Y,2=Z). Poles/weights/knots are
// copied without arithmetic or normalization; chart parameters are unchanged.
func (s *BSplineSurface) PermutedModelAxes(order [3]int) (*BSplineSurface, error) {
	if s == nil || len(s.poles) == 0 {
		return nil, ErrInvalid
	}
	seen := [3]bool{}
	for _, axis := range order {
		if axis < 0 || axis > 2 || seen[axis] {
			return nil, ErrInvalid
		}
		seen[axis] = true
	}
	copy := *s
	copy.poles = make([]Vec3, len(s.poles))
	copy.knotsU = append([]float64(nil), s.knotsU...)
	copy.knotsV = append([]float64(nil), s.knotsV...)
	copy.weights = append([]float64(nil), s.weights...)
	for i, p := range s.poles {
		copy.poles[i] = Vec3{coordinate(p, order[0]), coordinate(p, order[1]), coordinate(p, order[2])}
	}
	return &copy, nil
}
