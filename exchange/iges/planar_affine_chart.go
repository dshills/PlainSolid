package iges

import "math/big"

type igesPlanarFaceGeometry struct {
	origin, normal igesPlanarVec
	chart          *igesPlanarChart
}

type igesPlanarChart struct {
	u, v igesPlanarVec
}

func (g igesPlanarGraph) planarModelSegment(id int) (s [2]igesPlanarVec, err error) {
	if g.records[id].kind == 126 {
		return g.planarSplineSegment(id)
	}
	r, err := g.take(id, 110)
	if err != nil {
		return s, err
	}
	if len(r.values) < 7 || noPointers(r.values[7:]) != nil {
		return s, igesPlanarFailure(id, "invalid model line")
	}
	for i := range s {
		s[i], err = igesPlanarCoordinates(r.values[1+3*i : 4+3*i])
		if err != nil {
			return s, err
		}
	}
	return s, nil
}

func (g igesPlanarGraph) planarSplineSegment(id int) (s [2]igesPlanarVec, err error) {
	r, err := g.take(id, 126)
	if err != nil {
		return s, err
	}
	if err := compositeSourceNumbers(126, r.values); err != nil {
		return s, igesPlanarFailure(id, "linear spline has inexact source coefficients")
	}
	en, err := decode(126, r.values, 2)
	if err != nil {
		return s, err
	}
	pts, err := tetraLinear(en)
	if err != nil || pts[0] == pts[1] {
		return s, igesPlanarFailure(id, "expected one directed linear spline on the original unit interval")
	}
	return [2]igesPlanarVec{igesPlanarFromVec(pts[0]), igesPlanarFromVec(pts[1])}, nil
}

func (g igesPlanarGraph) planarFaceGeometry(id int) (s igesPlanarFaceGeometry, err error) {
	if g.records[id].kind == 128 {
		return g.planarAffineSplineChart(id)
	}
	plane, err := g.take(id, 190)
	if err != nil {
		return s, err
	}
	p, err := topologyInts(plane, 4)
	if err != nil {
		return s, err
	}
	s.origin, err = g.datum(p[1], 116)
	if err != nil {
		return s, err
	}
	s.normal, err = g.datum(p[2], 123)
	if err != nil {
		return s, err
	}
	x, err := g.datum(p[3], 123)
	if err != nil {
		return s, err
	}
	if s.normal.dot(s.normal).Sign() == 0 || x.dot(x).Sign() == 0 || s.normal.dot(x).Sign() != 0 {
		return s, igesPlanarFailure(id, "invalid exact plane axes")
	}
	return s, nil
}

func (g igesPlanarGraph) planarAffineSplineChart(id int) (s igesPlanarFaceGeometry, err error) {
	r, err := g.take(id, 128)
	if err != nil {
		return s, err
	}
	if err := compositeSourceNumbers(110, r.values); err != nil {
		return s, igesPlanarFailure(id, "affine chart has inexact source coefficients")
	}
	en, err := decode(128, r.values, 4)
	if err != nil {
		return s, err
	}
	a := en.Surface
	if a == nil || !a.Polynomial || a.UStart != 0 || a.UEnd != 1 || a.VStart != 0 || a.VEnd != 1 ||
		a.ClosedU || a.ClosedV || a.PeriodicU || a.PeriodicV {
		return s, igesPlanarFailure(id, "expected open polynomial chart on the original unit rectangle")
	}
	d := a.Geometry.Data()
	unit := func(k []float64) bool {
		return len(k) == 4 && k[0] == 0 && k[1] == 0 && k[2] == 1 && k[3] == 1
	}
	if d.DegreeU != 1 || d.DegreeV != 1 || d.CountU != 2 || d.CountV != 2 || !unit(d.KnotsU) || !unit(d.KnotsV) {
		return s, igesPlanarFailure(id, "expected one original affine spline cell")
	}
	for _, w := range d.Weights {
		if w != 1 {
			return s, igesPlanarFailure(id, "varying affine chart weights")
		}
	}
	s.origin = igesPlanarFromVec(d.Poles[0])
	u := igesPlanarFromVec(d.Poles[2]).sub(s.origin)
	v := igesPlanarFromVec(d.Poles[1]).sub(s.origin)
	if !igesPlanarFromVec(d.Poles[3]).sub(igesPlanarFromVec(d.Poles[2])).equal(v) {
		return s, igesPlanarFailure(id, "original chart is not affine")
	}
	s.normal = u.cross(v)
	if s.normal.dot(s.normal).Sign() == 0 {
		return s, igesPlanarFailure(id, "degenerate original affine chart")
	}
	s.chart = &igesPlanarChart{u, v}
	return s, nil
}

func (s igesPlanarFaceGeometry) lift(p igesPlanarVec) (q igesPlanarVec, ok bool) {
	one := big.NewRat(1, 1)
	if p[2].Sign() != 0 || p[0].Sign() < 0 || p[1].Sign() < 0 || p[0].Cmp(one) > 0 || p[1].Cmp(one) > 0 {
		return q, false
	}
	for i := range q {
		q[i] = new(big.Rat).Add(s.origin[i], new(big.Rat).Mul(p[0], s.chart.u[i]))
		q[i].Add(q[i], new(big.Rat).Mul(p[1], s.chart.v[i]))
	}
	return q, true
}
