package parametric

import (
	"fmt"
	"math"

	"plainsolid/geom"
)

// LoftSplines joins matched, closed, planar rational B-spline sections using
// straight generators. Both profiles must share degree, knot vector, weights,
// and winding, and their planes must be parallel. The caller must provide
// simple profiles star-shaped about their respective cap centers, with
// corresponding generators that do not cross. This construction checks
// sampled profile planarity and cap orientation and validates the output
// topology; it does not certify that a general loft is embedded.
func LoftSplines(lower, upper *geom.BSpline, lowerCenter, upperCenter, normal geom.Vec3, tolerance float64) (Solid, error) {
	if lower == nil || upper == nil || !lowerCenter.IsFinite() || !upperCenter.IsFinite() ||
		tolerance <= 0 || !finite(tolerance) {
		return Solid{}, geom.ErrInvalid
	}
	z, e := normal.Unit()
	if e != nil {
		return Solid{}, e
	}
	if upperCenter.Sub(lowerCenter).Dot(z) <= tolerance {
		return Solid{}, fmt.Errorf("parametric: upper cap must be above lower")
	}
	p, q := lower.Poles(), upper.Poles()
	w, x := lower.Weights(), upper.Weights()
	k, j := lower.Knots(), upper.Knots()
	if lower.Degree() != upper.Degree() || len(p) != len(q) || len(k) != len(j) || len(w) != len(x) {
		return Solid{}, fmt.Errorf("parametric: mismatched spline loft basis")
	}
	for i, v := range k {
		if v != j[i] {
			return Solid{}, fmt.Errorf("parametric: mismatched loft knots")
		}
	}
	for i, v := range w {
		if v != x[i] {
			return Solid{}, fmt.Errorf("parametric: mismatched loft weights")
		}
	}
	for i, v := range p {
		if math.Abs(v.Sub(lowerCenter).Dot(z)) > tolerance || math.Abs(q[i].Sub(upperCenter).Dot(z)) > tolerance {
			return Solid{}, fmt.Errorf("parametric: nonplanar loft controls")
		}
	}
	u0, u1 := lower.Domain()
	u := Interval{u0, u1}
	if !u.valid() {
		return Solid{}, geom.ErrDomain
	}
	a, e := lower.Eval(u0)
	if e != nil {
		return Solid{}, e
	}
	b, e := upper.Eval(u0)
	if e != nil {
		return Solid{}, e
	}
	for _, l := range []struct {
		c          *geom.BSpline
		center, at geom.Vec3
	}{{lower, lowerCenter, a}, {upper, upperCenter, b}} {
		last, e := l.c.Eval(u1)
		if e != nil || !near(l.at, last, tolerance) {
			return Solid{}, fmt.Errorf("parametric: loft profile is not closed")
		}
		for i := 0; i < 64; i++ {
			t := u.at((float64(i) + .5) / 64)
			p, e := l.c.Eval(t)
			if e != nil {
				return Solid{}, e
			}
			d, e := l.c.Derivative(t)
			if e != nil {
				return Solid{}, e
			}
			sign := p.Sub(l.center).Cross(d).Dot(z)
			if sign <= 0 || !finite(sign) {
				return Solid{}, fmt.Errorf("parametric: sampled loft profile not positive and star-shaped")
			}
		}
	}
	makeSurface := func(a, b []geom.Vec3) (*geom.BSplineSurface, error) {
		poles := make([]geom.Vec3, 2*len(a))
		weights := make([]float64, len(poles))
		for i := range a {
			poles[2*i], poles[2*i+1] = a[i], b[i]
			weights[2*i], weights[2*i+1] = w[i], w[i]
		}
		return geom.NewBSplineSurface(lower.Degree(), 1, len(a), 2, poles, k, []float64{0, 0, 1, 1}, weights)
	}
	flat := func(c geom.Vec3) []geom.Vec3 {
		x := make([]geom.Vec3, len(p))
		for i := range x {
			x[i] = c
		}
		return x
	}
	side, e := makeSurface(p, q)
	if e != nil {
		return Solid{}, e
	}
	bottom, e := makeSurface(flat(lowerCenter), p)
	if e != nil {
		return Solid{}, e
	}
	top, e := makeSurface(flat(upperCenter), q)
	if e != nil {
		return Solid{}, e
	}
	faces := []Face{
		{Surface: side, U: u, V: normalized, Boundary: [4]Use{{0, false}, {1, false}, {2, false}, {1, true}}},
		{Surface: bottom, U: u, V: normalized, Boundary: [4]Use{{3, false}, {4, false}, {0, true}, {4, true}}},
		{Surface: top, U: u, V: normalized, Boundary: [4]Use{{5, false}, {6, false}, {2, false}, {6, true}}, Reversed: true},
	}
	vertices := []geom.Vec3{a, b, lowerCenter, upperCenter}
	edges := []Edge{
		{0, 0, iso{faces[0], 0}, normalized}, {0, 1, iso{faces[0], 1}, normalized}, {1, 1, iso{faces[0], 2}, normalized},
		{Start: 2, End: 2}, {2, 0, iso{faces[1], 1}, normalized},
		{Start: 3, End: 3}, {3, 1, iso{faces[2], 1}, normalized},
	}
	return NewSolid(vertices, edges, faces, tolerance)
}
