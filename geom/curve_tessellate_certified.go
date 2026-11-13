package geom

import (
	"context"
	"math"
	"math/big"
)

// CertifiedTessellationOptions specifies a bound on pointwise distance from
// the exact rational curve to parameter-linear interpolation of the returned
// float endpoints. Zero values select deflection 1e-4, 8192 segments, depth32,
// 8M operations, 16384 coefficient bits. Floating endpoint error is included.
type CertifiedTessellationOptions struct {
	Deflection                                               float64
	MaxSegments, MaxDepth, MaxOperations, MaxCoefficientBits int
}

// CertifiedCurveSegment is ordered in original parameter space. ErrorUpper
// bounds distance to its rounded chord at every parameter in Domain. It also
// gives a symmetric Hausdorff bound, via the same parameter correspondence.
type CertifiedCurveSegment struct {
	Domain     ParameterSpan
	Start, End Vec3
	ErrorUpper float64
}

// CertifiedCurveTessellation contains an ordered prefix of certified segments.
// Only Complete asserts coverage of the whole curve. On any error, unprocessed
// suffixes are omitted, never silently bridged by a chord.
type CertifiedCurveTessellation struct {
	Segments        []CertifiedCurveSegment
	Complete        bool
	ExactOperations int
}

// TessellateCurveCertified handles continuous clamped positive-weight rational
// curves. Interior knot boundaries are always retained, including C0 corners.
func TessellateCurveCertified(c *BSpline, o CertifiedTessellationOptions) (CertifiedCurveTessellation, error) {
	return TessellateCurveCertifiedContext(context.Background(), c, o)
}

// TessellateCurveCertifiedContext supports cooperative cancellation.
func TessellateCurveCertifiedContext(ctx context.Context, c *BSpline, o CertifiedTessellationOptions) (CertifiedCurveTessellation, error) {
	if ctx == nil {
		return CertifiedCurveTessellation{}, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return CertifiedCurveTessellation{}, e
	}
	operations, bits := o.MaxOperations, o.MaxCoefficientBits
	if operations == 0 {
		operations = 8000000
	}
	if bits == 0 {
		bits = 16384
	}
	p, e := curveDistancePatch(c, operations, bits)
	if e != nil {
		return CertifiedCurveTessellation{}, e
	}
	if operations <= p.operations {
		return CertifiedCurveTessellation{ExactOperations: p.operations}, ErrConvergence
	}
	o.MaxOperations = operations - p.operations
	r, e := tessellateExactCurve(ctx, p, o)
	r.ExactOperations += p.operations
	return r, e
}

// TessellateCertified preserves the exact extracted iso-curve geometry.
func (c *SurfaceIsoCurve) TessellateCertified(o CertifiedTessellationOptions) (CertifiedCurveTessellation, error) {
	return c.TessellateCertifiedContext(context.Background(), o)
}

// TessellateCertifiedContext supports cancellation between bounded cell steps.
func (c *SurfaceIsoCurve) TessellateCertifiedContext(ctx context.Context, o CertifiedTessellationOptions) (CertifiedCurveTessellation, error) {
	if ctx == nil || c == nil || c.patch == nil {
		return CertifiedCurveTessellation{}, ErrInvalid
	}
	return tessellateExactCurve(ctx, c.patch, o)
}

func roundedHomogeneousPoint(h exactHomogeneous, b *polynomialBudget) (Vec3, float64, error) {
	var values [3]float64
	squared := new(big.Rat)
	for axis := 0; axis < 3; axis++ {
		exact := new(big.Rat).Quo(h[axis], h[3])
		f, _ := exact.Float64()
		if !finite(f) {
			return Vec3{}, 0, ErrConvergence
		}
		values[axis] = f
		delta := new(big.Rat).Sub(exact, new(big.Rat).SetFloat64(f))
		squared.Add(squared, new(big.Rat).Mul(delta, delta))
		if e := b.check(squared); e != nil {
			return Vec3{}, 0, e
		}
	}
	return Vec3{values[0], values[1], values[2]}, outwardSqrtRat(squared, false), nil
}
func tessellateExactCurve(ctx context.Context, p *SurfacePatch, o CertifiedTessellationOptions) (r CertifiedCurveTessellation, err error) {
	if o.Deflection == 0 {
		o.Deflection = 1e-4
	}
	if o.MaxSegments == 0 {
		o.MaxSegments = 8192
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 32
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if !finite(o.Deflection) || o.Deflection <= 0 || o.MaxSegments < 1 || o.MaxDepth < 1 || o.MaxDepth > 128 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	type cell struct {
		p     *SurfacePatch
		depth int
	}
	stack := []cell{{p, 0}}
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		p := current.p
		d := p.Domain()
		split, knot := internalPatchKnot(p.knotsU, p.degreeU, p.countU)
		if !knot {
			derivatives, e := curveDerivativeCell(p, b)
			if e != nil {
				return r, e
			}
			start, roundStart, e := roundedHomogeneousPoint(p.net[0], b)
			if e != nil {
				return r, e
			}
			end, roundEnd, e := roundedHomogeneousPoint(p.net[(p.countU-1)*p.countV], b)
			if e != nil {
				return r, e
			}
			errorBound := math.Inf(1)
			if finite(derivatives.ChordDeviationUpper) {
				sum := new(big.Rat).Add(new(big.Rat).SetFloat64(derivatives.ChordDeviationUpper), new(big.Rat).SetFloat64(max(roundStart, roundEnd)))
				if e := b.check(sum); e != nil {
					return r, e
				}
				errorBound = rationalFloatBound(sum, false)
			}
			if errorBound <= o.Deflection {
				r.Segments = append(r.Segments, CertifiedCurveSegment{ParameterSpan{Start: d.UMin, End: d.UMax, Index: len(r.Segments)}, start, end, errorBound})
				continue
			}
			if current.depth >= o.MaxDepth {
				return r, ErrConvergence
			}
			split = d.UMin + (d.UMax-d.UMin)/2
			if split <= d.UMin || split >= d.UMax {
				return r, ErrConvergence
			}
		}
		if len(r.Segments)+len(stack)+2 > o.MaxSegments {
			return r, ErrConvergence
		}
		left, right, e := p.split(split, true, b)
		if e != nil {
			return r, e
		}
		depth := current.depth + 1
		if knot {
			depth = 0
		}
		stack = append(stack, cell{right, depth}, cell{left, depth})
	}
	r.Complete = true
	return r, nil
}
