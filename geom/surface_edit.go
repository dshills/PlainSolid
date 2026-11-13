package geom

import "sort"

func refineHomogeneous(poles []homogeneous, knots []float64, degree int, t float64, count int) ([]homogeneous, []float64) {
	p := append([]homogeneous(nil), poles...)
	u := append([]float64(nil), knots...)
	for insertion := 0; insertion < count; insertion++ {
		first := sort.SearchFloat64s(u, t)
		k := sort.Search(len(u), func(i int) bool { return u[i] > t }) - 1
		s := k - first + 1
		q := make([]homogeneous, len(p)+1)
		for i := 0; i <= k-degree; i++ {
			q[i] = p[i]
		}
		for i := k - s; i < len(p); i++ {
			q[i+1] = p[i]
		}
		for i := k - degree + 1; i <= k-s; i++ {
			alpha := (t - u[i]) / (u[i+degree] - u[i])
			for j := 0; j < 4; j++ {
				q[i][j] = (1-alpha)*p[i-1][j] + alpha*p[i][j]
			}
		}
		v := make([]float64, len(u)+1)
		copy(v, u[:k+1])
		v[k+1] = t
		copy(v[k+2:], u[k+1:])
		p, u = q, v
	}
	return p, u
}
func splitHomogeneous(poles []homogeneous, knots []float64, degree int, t float64) ([]homogeneous, []homogeneous, []float64, []float64) {
	first := sort.SearchFloat64s(knots, t)
	last := sort.Search(len(knots), func(i int) bool { return knots[i] > t })
	p, u := refineHomogeneous(poles, knots, degree, t, degree+1-(last-first))
	first = sort.SearchFloat64s(u, t)
	return p[:first], p[first:], u[:first+degree+1], u[first:]
}
func surfaceFromHom(degreeU, degreeV, countU, countV int, h []homogeneous, knotsU, knotsV []float64) (*BSplineSurface, error) {
	p := make([]Vec3, len(h))
	w := make([]float64, len(h))
	for i, v := range h {
		point, e := projectHom(v)
		if e != nil {
			return nil, e
		}
		p[i], w[i] = point, v[3]
	}
	return NewBSplineSurface(degreeU, degreeV, countU, countV, p, knotsU, knotsV, w)
}

// SplitU returns independent tensor-product surfaces, preserving original
// parameters and rational geometry up to float64 arithmetic. At a discontinuous
// knot the left surface keeps the left limit and the right keeps the right limit.
func (s *BSplineSurface) SplitU(t float64) (*BSplineSurface, *BSplineSurface, error) {
	if s == nil || len(s.poles) == 0 {
		return nil, nil, ErrInvalid
	}
	a, b, _, _ := s.Domain()
	if !finite(t) || t <= a || t >= b {
		return nil, nil, ErrDomain
	}
	h := toHom(s.poles, s.weights)
	var lh, rh []homogeneous
	var lu, ru []float64
	leftCount, rightCount := 0, 0
	for v := 0; v < s.countV; v++ {
		column := make([]homogeneous, s.countU)
		for u := range column {
			column[u] = h[u*s.countV+v]
		}
		left, right, lk, rk := splitHomogeneous(column, s.knotsU, s.degreeU, t)
		if v == 0 {
			leftCount, rightCount = len(left), len(right)
			lh = make([]homogeneous, leftCount*s.countV)
			rh = make([]homogeneous, rightCount*s.countV)
			lu, ru = lk, rk
		}
		for u, p := range left {
			lh[u*s.countV+v] = p
		}
		for u, p := range right {
			rh[u*s.countV+v] = p
		}
	}
	left, e := surfaceFromHom(s.degreeU, s.degreeV, leftCount, s.countV, lh, lu, s.knotsV)
	if e != nil {
		return nil, nil, e
	}
	right, e := surfaceFromHom(s.degreeU, s.degreeV, rightCount, s.countV, rh, ru, s.knotsV)
	if e != nil {
		return nil, nil, e
	}
	return left, right, nil
}

// SplitV is the v-direction counterpart of SplitU.
func (s *BSplineSurface) SplitV(t float64) (*BSplineSurface, *BSplineSurface, error) {
	if s == nil || len(s.poles) == 0 {
		return nil, nil, ErrInvalid
	}
	_, _, a, b := s.Domain()
	if !finite(t) || t <= a || t >= b {
		return nil, nil, ErrDomain
	}
	h := toHom(s.poles, s.weights)
	var lh, rh []homogeneous
	var lv, rv []float64
	leftCount, rightCount := 0, 0
	for u := 0; u < s.countU; u++ {
		left, right, lk, rk := splitHomogeneous(h[u*s.countV:(u+1)*s.countV], s.knotsV, s.degreeV, t)
		if u == 0 {
			leftCount, rightCount = len(left), len(right)
			lh = make([]homogeneous, s.countU*leftCount)
			rh = make([]homogeneous, s.countU*rightCount)
			lv, rv = lk, rk
		}
		copy(lh[u*leftCount:], left)
		copy(rh[u*rightCount:], right)
	}
	left, e := surfaceFromHom(s.degreeU, s.degreeV, s.countU, leftCount, lh, s.knotsU, lv)
	if e != nil {
		return nil, nil, e
	}
	right, e := surfaceFromHom(s.degreeU, s.degreeV, s.countU, rightCount, rh, s.knotsU, rv)
	if e != nil {
		return nil, nil, e
	}
	return left, right, nil
}
