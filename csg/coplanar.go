package csg

import (
	"fmt"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
)

// clippingPlane keeps exact coefficients when an input face is exactly planar.
// Coefficients are scaled by their largest magnitude to bound the distance metric.
// Slightly warped faces retain the older represented supporting-plane fallback.
type clippingPlane struct {
	plane                plane
	normal               [3]*big.Rat
	offset, globalOffset *big.Rat
	metric               float64
	exact                bool
}

func coefficientDot(normal [3]*big.Rat, p geom.Vec3) *big.Rat {
	return new(big.Rat).Add(new(big.Rat).Add(product(normal[0], rational(p.X)), product(normal[1], rational(p.Y))), product(normal[2], rational(p.Z)))
}
func boundaryPlanes(s brep.Solid, support []plane, origin geom.Vec3) ([]clippingPlane, error) {
	vertices, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	result := make([]clippingPlane, len(faces))
	for f, face := range faces {
		if len(face.Holes) > 0 {
			return nil, fmt.Errorf("%w: convex operand has inner wires", ErrUnsupported)
		}
		points := make([]geom.Vec3, len(face.Boundary))
		for i, u := range face.Boundary {
			id := edges[u.Edge].Start
			if u.Reversed {
				id = edges[u.Edge].End
			}
			points[i] = vertices[id]
		}
		a, b, c := representedPoint(points[0]), representedPoint(points[1]), representedPoint(points[2])
		var x, y [3]*big.Rat
		for i := 0; i < 3; i++ {
			x[i] = difference(b[i], a[i])
			y[i] = difference(c[i], a[i])
		}
		normal := [3]*big.Rat{difference(product(x[1], y[2]), product(x[2], y[1])), difference(product(x[2], y[0]), product(x[0], y[2])), difference(product(x[0], y[1]), product(x[1], y[0]))}
		if coefficientDot(normal, support[f].n).Sign() < 0 {
			for i := range normal {
				normal[i] = new(big.Rat).Neg(normal[i])
			}
		}
		maximum := new(big.Rat)
		for _, n := range normal {
			absolute := new(big.Rat).Abs(n)
			if absolute.Cmp(maximum) > 0 {
				maximum = absolute
			}
		}
		if maximum.Sign() == 0 {
			return nil, fmt.Errorf("%w: degenerate source face plane", ErrUnsupported)
		}
		for i := range normal {
			normal[i] = new(big.Rat).Quo(normal[i], maximum)
		}
		axis := 0
		for i := 1; i < 3; i++ {
			if new(big.Rat).Abs(normal[i]).Cmp(new(big.Rat).Abs(normal[axis])) > 0 {
				axis = i
			}
		}
		orientation := normal[axis].Sign()
		for i, p := range points {
			q, r := points[(i+1)%len(points)], points[(i+2)%len(points)]
			if sectionOrient(representedPoint(p), representedPoint(q), representedPoint(r), axis).Sign()*orientation < 0 {
				return nil, fmt.Errorf("%w: boundary clipping requires convex individual face loops", ErrUnsupported)
			}
		}
		d := coefficientDot(normal, points[0])
		isExact := true
		for _, p := range points[1:] {
			if coefficientDot(normal, p).Cmp(d) != 0 {
				isExact = false
				break
			}
		}
		nx, _ := normal[0].Float64()
		ny, _ := normal[1].Float64()
		nz, _ := normal[2].Float64()
		result[f] = clippingPlane{plane: support[f], normal: normal, globalOffset: d, offset: difference(d, coefficientDot(normal, origin)), metric: math.Hypot(math.Hypot(nx, ny), nz), exact: isExact}
	}
	return result, nil
}
func (p clippingPlane) value(v geom.Vec3) *big.Rat {
	if !p.exact {
		return planeValue(p.plane, v)
	}
	return difference(coefficientDot(p.normal, v), p.offset)
}
func (p clippingPlane) limit(tolerance float64) *big.Rat {
	if !p.exact {
		return rational(tolerance)
	}
	return product(rational(tolerance), rational(p.metric))
}

// coplanarRelation is +1 for equal outward directions, -1 for opposite ones.
func coplanarRelation(a, b clippingPlane) int {
	if !a.exact || !b.exact {
		return 0
	}
	same, opposite := true, true
	for i := 0; i < 3; i++ {
		if a.normal[i].Cmp(b.normal[i]) != 0 {
			same = false
		}
		if a.normal[i].Cmp(new(big.Rat).Neg(b.normal[i])) != 0 {
			opposite = false
		}
	}
	if same && a.globalOffset.Cmp(b.globalOffset) == 0 {
		return 1
	}
	if opposite && a.globalOffset.Cmp(new(big.Rat).Neg(b.globalOffset)) == 0 {
		return -1
	}
	return 0
}
func exactContained(vertices []geom.Vec3, planes []clippingPlane) bool {
	for _, p := range planes {
		if !p.exact {
			return false
		}
		for _, v := range vertices {
			if coefficientDot(p.normal, v).Cmp(p.globalOffset) > 0 {
				return false
			}
		}
	}
	return true
}
func possibleContained(vertices []geom.Vec3, planes []plane, origin geom.Vec3, tolerance float64) bool {
	for _, v := range vertices {
		local := v.Sub(origin)
		for _, p := range planes {
			if planeValue(p, local).Cmp(rational(tolerance)) > 0 {
				return false
			}
		}
	}
	return true
}

func exactSupports(p clippingPlane, vertices []geom.Vec3) bool {
	if !p.exact {
		return false
	}
	for _, v := range vertices {
		if coefficientDot(p.normal, v).Cmp(p.globalOffset) > 0 {
			return false
		}
	}
	return true
}
