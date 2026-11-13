package geom

import (
	"context"
	"math/big"
)

// TubeEndpointOptions bounds exact endpoint verification. Zero limits select
// 1M operations and 16384 coefficient bits for an individual certificate.
type TubeEndpointOptions = TubeEndpointEdgeOptions

// TubeEndpointCertificate proves the exact point at the selected free-coordinate
// endpoint of an immutable verified tube. Parameters and Point are caller-owned
// rationals; boxes are outward enclosures, not equality witnesses themselves.
// This certificate has no face-boundary or coedge-orientation requirement.
type TubeEndpointCertificate struct {
	Proven                             bool
	Parameters                         [4]*big.Rat
	Point                              [3]*big.Rat
	ParameterBoundsA, ParameterBoundsB UVBox
	PointBounds                        Box
	ExactOperations                    int
}

// CertifyEndpoint verifies a rational parameter proposal using exact A=B
// evaluation, selected free endpoint equality and strict dependent containment.
// Immutable tube uniqueness then identifies the proposed point as its endpoint.
// upper selects the upper free value, independently of branch orientation.
func (t VerifiedSurfaceTube) CertifyEndpoint(ctx context.Context, upper bool, hint [4]*big.Rat, o TubeEndpointOptions) (TubeEndpointCertificate, error) {
	if t.a == nil || t.b == nil || t.axis < 0 || t.axis > 3 {
		return TubeEndpointCertificate{}, ErrInvalid
	}
	surface := t.a
	if t.axis >= 2 {
		surface = t.b
	}
	edge := SurfaceEdgeUMin
	if t.axis%2 == 1 {
		edge = SurfaceEdgeVMin
	}
	if upper {
		edge++
	}
	// The local free side is used solely to reuse exact endpoint verification;
	// no assertion about a surrounding model face is made by this wrapper.
	verified, e := t.CertifyEndpointOnEdge(ctx, upper, t.axis/2, surface.Domain(), edge, hint, o)
	return TubeEndpointCertificate{Proven: verified.Proven, Parameters: verified.Parameters, Point: verified.Point, ParameterBoundsA: verified.ParameterBoundsA, ParameterBoundsB: verified.ParameterBoundsB, PointBounds: verified.PointBounds, ExactOperations: verified.ExactOperations}, e
}

// TubeEndpointComparison compares independently verified endpoints, even when
// their tubes refer to different ordered surface pairs. Complete&&Equal proves
// the same exact model point; Complete&&!Equal proves distinct points. Mutable
// public endpoint certificates are never accepted as proof input. On error the
// comparison is incomplete, though a returned Left/Right proof may be valid.
type TubeEndpointComparison struct {
	Complete, Equal bool
	Left, Right     TubeEndpointCertificate
	ExactOperations int
}

// CompareVerifiedTubeEndpoints first certifies both supplied parameter proposals
// against their respective immutable tubes, then compares all three rational
// model coordinates exactly. No rounded spatial endpoint merging occurs.
// Options apply cumulatively; zero limits select 2M operations and 16384 bits.
func CompareVerifiedTubeEndpoints(ctx context.Context, left, right VerifiedSurfaceTube, leftUpper, rightUpper bool, leftHint, rightHint [4]*big.Rat, o TubeEndpointOptions) (r TubeEndpointComparison, err error) {
	if ctx == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	r.Left, err = left.CertifyEndpoint(ctx, leftUpper, leftHint, o)
	r.ExactOperations = r.Left.ExactOperations
	if err != nil {
		return r, err
	}
	remaining := o.MaxOperations - r.ExactOperations
	if remaining < 1 {
		return r, ErrConvergence
	}
	r.Right, err = right.CertifyEndpoint(ctx, rightUpper, rightHint, TubeEndpointOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	r.ExactOperations += r.Right.ExactOperations
	if err != nil {
		return r, err
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations - r.ExactOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations += budget.operations }()
	same := true
	for axis := 0; axis < 3; axis++ {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		difference := new(big.Rat).Sub(r.Left.Point[axis], r.Right.Point[axis])
		if e := budget.check(difference); e != nil {
			return r, e
		}
		same = same && difference.Sign() == 0
	}
	r.Complete, r.Equal = true, same
	return r, nil
}
