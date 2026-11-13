package geom

import (
	"context"
	"math/big"
)

// TubeEndpointEdgeOptions bounds exact proposed-endpoint verification. Zero
// limits select 1M operations and 16384 coefficient bits.
type TubeEndpointEdgeOptions struct{ MaxOperations, MaxCoefficientBits int }

// TubeEndpointEdgeCertificate proves that the tube's selected endpoint equals
// the exact rational point on the specified original face boundary. EdgeParameter
// is its original varying chart coordinate; NormalizedEdgeParameter is measured
// increasingly along that coordinate on the full original face side, not along
// a B-rep coedge orientation. All rational values are caller-owned.
type TubeEndpointEdgeCertificate struct {
	Proven                                 bool
	Parameters                             [4]*big.Rat
	Point                                  [3]*big.Rat
	EdgeParameter, NormalizedEdgeParameter *big.Rat
	ParameterBoundsA, ParameterBoundsB     UVBox
	PointBounds                            Box
	ExactOperations                        int
}

// CertifyEndpointOnEdge verifies a caller-supplied exact four-parameter endpoint
// proposal. surfaceIndex selects input A (0) or B (1); faceDomain is the ORIGINAL
// face rectangle and must contain that input's local tube rectangle. The chosen
// edge must fix the tube's free coordinate on that same chart. Strict uniform
// tubes keep dependent roots inside local bounds, so dependent-coordinate face
// sides and face corners are outside this slice and return ErrConvergence.
// upper chooses the upper free endpoint independently of oriented branch order.
// Exact A=B evaluation plus immutable tube uniqueness proves the connection;
// neither float proximity nor a caller-owned mutable certificate is trusted.
func (t VerifiedSurfaceTube) CertifyEndpointOnEdge(ctx context.Context, upper bool, surfaceIndex int, faceDomain ParameterRectangle, edge SurfaceEdge, hint [4]*big.Rat, o TubeEndpointEdgeOptions) (r TubeEndpointEdgeCertificate, err error) {
	if ctx == nil || t.a == nil || t.b == nil || t.axis < 0 || t.axis > 3 || t.tangentSign == 0 || surfaceIndex < 0 || surfaceIndex > 1 || !faceDomain.Valid() || edge > SurfaceEdgeVMax {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	selected := t.a
	if surfaceIndex == 1 {
		selected = t.b
	}
	local := selected.Domain()
	if local.UMin < faceDomain.UMin || local.UMax > faceDomain.UMax || local.VMin < faceDomain.VMin || local.VMax > faceDomain.VMax {
		return r, ErrDomain
	}
	fixed := 0
	if edge >= SurfaceEdgeVMin {
		fixed = 1
	}
	if 2*surfaceIndex+fixed != t.axis {
		return r, ErrConvergence
	}
	ranges := pairParameterRanges(SurfacePairParameterBox{t.a.Domain(), t.b.Domain()})
	endpoint := ranges[t.axis][0]
	if upper {
		endpoint = ranges[t.axis][1]
	}
	boundary := faceDomain.UMin
	switch edge {
	case SurfaceEdgeUMax:
		boundary = faceDomain.UMax
	case SurfaceEdgeVMin:
		boundary = faceDomain.VMin
	case SurfaceEdgeVMax:
		boundary = faceDomain.VMax
	}
	if endpoint != boundary {
		return r, ErrDomain
	}
	anchored := t.anchor != nil && t.anchor.free == endpoint
	for axis, x := range hint {
		if x == nil {
			return r, ErrInvalid
		}
		if e := budget.check(x); e != nil {
			return r, e
		}
		if anchored {
			if x.Cmp(t.anchor.parameters[axis]) != 0 {
				return r, ErrDomain
			}
			continue
		}
		if axis == t.axis {
			if x.Cmp(new(big.Rat).SetFloat64(endpoint)) != 0 {
				return r, ErrDomain
			}
		} else {
			if x.Cmp(new(big.Rat).SetFloat64(ranges[axis][0])) <= 0 || x.Cmp(new(big.Rat).SetFloat64(ranges[axis][1])) >= 0 {
				return r, ErrDomain
			}
		}
	}
	a, _, e := exactSurfaceJet(t.a, hint[0], hint[1], budget)
	if e != nil {
		return r, e
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	b, _, e := exactSurfaceJet(t.b, hint[2], hint[3], budget)
	if e != nil {
		return r, e
	}
	for axis := 0; axis < 3; axis++ {
		residual := new(big.Rat).Sub(a[axis], b[axis])
		if e := budget.check(residual); e != nil {
			return r, e
		}
		if residual.Sign() != 0 {
			return r, ErrConvergence
		}
	}
	varying := surfaceIndex*2 + 1 - fixed
	start, end := faceDomain.VMin, faceDomain.VMax
	if fixed == 1 {
		start, end = faceDomain.UMin, faceDomain.UMax
	}
	normalized := new(big.Rat).Quo(new(big.Rat).Sub(hint[varying], new(big.Rat).SetFloat64(start)), new(big.Rat).Sub(new(big.Rat).SetFloat64(end), new(big.Rat).SetFloat64(start)))
	if e := budget.check(normalized); e != nil {
		return r, e
	}
	if normalized.Sign() < 0 || normalized.Cmp(big.NewRat(1, 1)) > 0 {
		return r, ErrDomain
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	var bounds [4][2]float64
	for axis, x := range hint {
		r.Parameters[axis] = new(big.Rat).Set(x)
		bounds[axis] = [2]float64{rationalFloatBound(x, true), rationalFloatBound(x, false)}
	}
	r.ParameterBoundsA = UVBox{Min: Vec2{bounds[0][0], bounds[1][0]}, Max: Vec2{bounds[0][1], bounds[1][1]}}
	r.ParameterBoundsB = UVBox{Min: Vec2{bounds[2][0], bounds[3][0]}, Max: Vec2{bounds[2][1], bounds[3][1]}}
	for axis, x := range a {
		r.Point[axis] = new(big.Rat).Set(x)
		lo, hi := rationalFloatBound(x, true), rationalFloatBound(x, false)
		switch axis {
		case 0:
			r.PointBounds.Min.X, r.PointBounds.Max.X = lo, hi
		case 1:
			r.PointBounds.Min.Y, r.PointBounds.Max.Y = lo, hi
		case 2:
			r.PointBounds.Min.Z, r.PointBounds.Max.Z = lo, hi
		}
	}
	r.EdgeParameter = new(big.Rat).Set(hint[varying])
	r.NormalizedEdgeParameter = new(big.Rat).Set(normalized)
	r.Proven = true
	return r, nil
}
