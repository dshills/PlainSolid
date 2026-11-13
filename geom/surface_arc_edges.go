package geom

import "math/big"

// SurfaceEdge identifies one boundary of a parameter rectangle.
type SurfaceEdge uint8

const (
	SurfaceEdgeUMin SurfaceEdge = iota
	SurfaceEdgeUMax
	SurfaceEdgeVMin
	SurfaceEdgeVMax
)

// UVBox is a closed parameter enclosure; degenerate point/edge boxes are valid.
type UVBox struct{ Min, Max Vec2 }

// SurfacePlaneEndpoint brackets one exact boundary root. Bounds encloses its
// spatial point. CornerIndex is -1 except at corners ordered (umin,vmin),
// (umax,vmin), (umax,vmax), (umin,vmax). Shared-cell endpoints are not stitched.
type SurfacePlaneEndpoint struct {
	Edge                      SurfaceEdge
	Parameters                UVBox
	Bounds                    Box
	Multiplicity, CornerIndex int
	identity                  *surfaceRootIdentity
}

func (p *SurfacePatch) edgeIndices(edge SurfaceEdge) []int {
	count := p.countV
	if edge >= SurfaceEdgeVMin {
		count = p.countU
	}
	indices := make([]int, count)
	for i := range indices {
		switch edge {
		case SurfaceEdgeUMin:
			indices[i] = i
		case SurfaceEdgeUMax:
			indices[i] = (p.countU-1)*p.countV + i
		case SurfaceEdgeVMin:
			indices[i] = i * p.countV
		case SurfaceEdgeVMax:
			indices[i] = i*p.countV + p.countV - 1
		}
	}
	return indices
}
func splitExactBezier(net []exactHomogeneous, t *big.Rat, b *polynomialBudget) ([]exactHomogeneous, []exactHomogeneous, error) {
	degree := len(net) - 1
	work := append([]exactHomogeneous(nil), net...)
	left, right := make([]exactHomogeneous, len(net)), make([]exactHomogeneous, len(net))
	left[0], right[degree] = work[0], work[degree]
	complement := new(big.Rat).Sub(big.NewRat(1, 1), t)
	for level := 1; level <= degree; level++ {
		for i := 0; i <= degree-level; i++ {
			var h exactHomogeneous
			for axis := 0; axis < 4; axis++ {
				value := new(big.Rat).Mul(complement, work[i][axis])
				value.Add(value, new(big.Rat).Mul(t, work[i+1][axis]))
				if e := b.check(value); e != nil {
					return nil, nil, e
				}
				h[axis] = value
			}
			work[i] = h
		}
		left[level], right[degree-level] = work[0], work[degree-level]
	}
	return left, right, nil
}
func restrictExactBezier(net []exactHomogeneous, a, c *big.Rat, b *polynomialBudget) ([]exactHomogeneous, error) {
	if a.Cmp(c) == 0 {
		if a.Sign() == 0 {
			return net[:1], nil
		}
		if a.Cmp(big.NewRat(1, 1)) == 0 {
			return net[len(net)-1:], nil
		}
		left, _, e := splitExactBezier(net, a, b)
		if e != nil {
			return nil, e
		}
		return left[len(left)-1:], nil
	}
	result := net
	if a.Sign() != 0 {
		_, right, e := splitExactBezier(result, a, b)
		if e != nil {
			return nil, e
		}
		result = right
	}
	if c.Cmp(big.NewRat(1, 1)) != 0 {
		relative := new(big.Rat).Quo(new(big.Rat).Sub(c, a), new(big.Rat).Sub(big.NewRat(1, 1), a))
		left, _, e := splitExactBezier(result, relative, b)
		if e != nil {
			return nil, e
		}
		result = left
	}
	return result, nil
}
func edgeCorner(edge SurfaceEdge, atEnd bool) int {
	switch edge {
	case SurfaceEdgeUMin:
		if atEnd {
			return 3
		}
		return 0
	case SurfaceEdgeUMax:
		if atEnd {
			return 2
		}
		return 1
	case SurfaceEdgeVMin:
		if atEnd {
			return 1
		}
		return 0
	default:
		if atEnd {
			return 2
		}
		return 3
	}
}
func (p *SurfacePatch) endpoint(edge SurfaceEdge, a, c *big.Rat, multiplicity int, parameterTolerance, boxTolerance float64, b *polynomialBudget) (SurfacePlaneEndpoint, bool, error) {
	domain := p.Domain()
	start, end := domain.VMin, domain.VMax
	if edge >= SurfaceEdgeVMin {
		start, end = domain.UMin, domain.UMax
	}
	span := ParameterSpan{Start: start, End: end}
	lower, upper := parameterMap(span, a), parameterMap(span, c)
	lo, hi := rationalFloatBound(lower, true), rationalFloatBound(upper, false)
	if hi-lo > parameterTolerance {
		return SurfacePlaneEndpoint{}, false, nil
	}
	indices := p.edgeIndices(edge)
	net := make([]exactHomogeneous, len(indices))
	for i, index := range indices {
		net[i] = p.net[index]
	}
	restricted, e := restrictExactBezier(net, a, c, b)
	if e != nil {
		return SurfacePlaneEndpoint{}, false, e
	}
	temporary := SurfacePatch{net: restricted}
	if e := temporary.computeBounds(b); e != nil {
		return SurfacePlaneEndpoint{}, false, e
	}
	size := temporary.bounds.Size()
	if size.X > boxTolerance || size.Y > boxTolerance || size.Z > boxTolerance {
		return SurfacePlaneEndpoint{}, false, nil
	}
	uv := UVBox{}
	switch edge {
	case SurfaceEdgeUMin:
		uv = UVBox{Vec2{domain.UMin, lo}, Vec2{domain.UMin, hi}}
	case SurfaceEdgeUMax:
		uv = UVBox{Vec2{domain.UMax, lo}, Vec2{domain.UMax, hi}}
	case SurfaceEdgeVMin:
		uv = UVBox{Vec2{lo, domain.VMin}, Vec2{hi, domain.VMin}}
	case SurfaceEdgeVMax:
		uv = UVBox{Vec2{lo, domain.VMax}, Vec2{hi, domain.VMax}}
	}
	corner := -1
	if a.Cmp(c) == 0 {
		if a.Sign() == 0 {
			corner = edgeCorner(edge, false)
		} else if a.Cmp(big.NewRat(1, 1)) == 0 {
			corner = edgeCorner(edge, true)
		}
	}
	return SurfacePlaneEndpoint{Edge: edge, Parameters: uv, Bounds: temporary.bounds, Multiplicity: multiplicity, CornerIndex: corner, identity: &surfaceRootIdentity{axis: int(edge) / 2, fixed: new(big.Rat).SetFloat64(func() float64 {
		if edge == SurfaceEdgeUMin {
			return domain.UMin
		}
		if edge == SurfaceEdgeUMax {
			return domain.UMax
		}
		if edge == SurfaceEdgeVMin {
			return domain.VMin
		}
		return domain.VMax
	}()), lo: lower, hi: upper, start: start, end: end}}, true, nil
}
func cornerEntersPatch(p *SurfacePatch, data surfaceArcData, corner, fixedAxis int) (bool, bool) {
	u, v := 0, 0
	if corner == 1 || corner == 2 {
		u = p.countU - 1
	}
	if corner == 2 || corner == 3 {
		v = p.countV - 1
	}
	du, dv := new(big.Rat), new(big.Rat)
	if u == 0 {
		du.Sub(data.fields[4][p.countV+v], data.fields[4][v])
	} else {
		du.Sub(data.fields[4][u*p.countV+v], data.fields[4][(u-1)*p.countV+v])
	}
	if v == 0 {
		dv.Sub(data.fields[4][u*p.countV+1], data.fields[4][u*p.countV])
	} else {
		dv.Sub(data.fields[4][u*p.countV+v], data.fields[4][u*p.countV+v-1])
	}
	fixed, free := du.Sign(), dv.Sign()
	dependentSide, freeSide := 1, 1
	if u != 0 {
		dependentSide = -1
	}
	if v != 0 {
		freeSide = -1
	}
	if fixedAxis == 1 {
		fixed, free = free, fixed
		dependentSide, freeSide = freeSide, dependentSide
	}
	if fixed == 0 || free == 0 {
		return false, false
	}
	return -free*fixed*freeSide == dependentSide, true
}
