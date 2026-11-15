package geom

import (
	"math"
	"math/big"
)

// LineSurfaceCrossing certifies the orientation of a regular line contact.
// Sign is sign((Su cross Sv) dot lineDirection) throughout the tested cell:
// +1 crosses toward the parametrically positive normal, -1 against it, and 0
// is unresolved. ScaledTripleProduct bounds W^4 times that product. A nonzero
// sign proves regularity and transversality but not existence by itself.
type LineSurfaceCrossing struct {
	Sign                int
	ProvenRegular       bool
	ScaledTripleProduct CertifiedScalarRange
	ExactOperations     int
}

// CertifyLineSurfaceCrossing uses exact homogeneous derivative bounds on a
// Bezier patch. Trim to a hit neighborhood first for sharper bounds; a point or
// boundary hit can use its positive-area certified uniqueness Cell. Orientation
// is parametric: reversing one surface parameter reverses Sign. There is no
// inferred solid/material orientation or tolerance-based grazing classification.
func CertifyLineSurfaceCrossing(line Line, p *SurfacePatch, o SurfaceTransversalityOptions) (result LineSurfaceCrossing, err error) {
	if !line.valid() {
		return result, ErrInvalid
	}
	return certifyRationalLineSurfaceCrossing(rationalLineVec3(line.direction), p, o)
}

func certifyRationalLineSurfaceCrossing(direction [3]*big.Rat, p *SurfacePatch, o SurfaceTransversalityOptions) (result LineSurfaceCrossing, err error) {
	if p == nil || len(p.net) == 0 {
		return result, ErrInvalid
	}
	if p.countU != p.degreeU+1 || p.countV != p.degreeV+1 {
		return result, ErrDomain
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return result, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { result.ExactOperations = b.operations }()
	normal, e := patchNormalInterval(p, b)
	if e != nil {
		return result, e
	}
	sum := exactInterval{new(big.Rat), new(big.Rat)}
	for axis := 0; axis < 3; axis++ {
		if e := b.check(direction[axis]); e != nil {
			return result, e
		}
		term, e := intervalMultiply(normal[axis], exactInterval{direction[axis], direction[axis]}, b)
		if e != nil {
			return result, e
		}
		sum, e = boundedExactInterval(new(big.Rat).Add(sum.lo, term.lo), new(big.Rat).Add(sum.hi, term.hi), b)
		if e != nil {
			return result, e
		}
	}
	result.ScaledTripleProduct = certifiedRange(sum)
	result.Sign = sum.sign()
	result.ProvenRegular = result.Sign != 0
	return result, nil
}

// Crossing certifies orientation over this hit's UV enclosure, using the supplied
// original surface patch. It does not trust the hit as an existence certificate:
// the returned claim is solely the derivative sign on that enclosure. Degenerate
// UV dimensions are expanded by at most one adjacent float inside Cell, retaining
// the exact boundary contact. A zero sign must not contribute a winding count.
func (h LineSplineSurfaceHit) Crossing(line Line, original *SurfacePatch, o SurfaceTransversalityOptions) (result LineSurfaceCrossing, err error) {
	if !line.valid() {
		return result, ErrInvalid
	}
	return h.RationalCrossing(rationalLineVec3(line.direction), original, o)
}

// RationalCrossing certifies the oriented derivative sign at the hit for an
// exact, nonzero direction. Its scale is preserved in the reported product.
func (h LineSplineSurfaceHit) RationalCrossing(direction [3]*big.Rat, original *SurfacePatch, o SurfaceTransversalityOptions) (result LineSurfaceCrossing, err error) {
	if original == nil || !h.Cell.Valid() || !validRationalLineDirection(direction) {
		return result, ErrInvalid
	}
	r := ParameterRectangle{max(h.Cell.UMin, h.UV.Min.X), min(h.Cell.UMax, h.UV.Max.X), max(h.Cell.VMin, h.UV.Min.Y), min(h.Cell.VMax, h.UV.Max.Y)}
	if !finite(r.UMin) || !finite(r.UMax) || !finite(r.VMin) || !finite(r.VMax) || r.UMin > r.UMax || r.VMin > r.VMax {
		return result, ErrInvalid
	}
	if r.UMin == r.UMax {
		r.UMin = max(h.Cell.UMin, math.Nextafter(r.UMin, math.Inf(-1)))
		r.UMax = min(h.Cell.UMax, math.Nextafter(r.UMax, math.Inf(1)))
	}
	if r.VMin == r.VMax {
		r.VMin = max(h.Cell.VMin, math.Nextafter(r.VMin, math.Inf(-1)))
		r.VMax = min(h.Cell.VMax, math.Nextafter(r.VMax, math.Inf(1)))
	}
	domain := original.Domain()
	if !r.Valid() || r.UMin < domain.UMin || r.UMax > domain.UMax || r.VMin < domain.VMin || r.VMax > domain.VMax {
		return result, ErrDomain
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return result, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	p, e := trimPatchBudget(original, r, budget)
	result.ExactOperations = budget.operations
	if e != nil {
		return result, e
	}
	o.MaxOperations -= budget.operations
	if o.MaxOperations < 1 {
		return result, ErrConvergence
	}
	result, e = certifyRationalLineSurfaceCrossing(direction, p, o)
	result.ExactOperations += budget.operations
	return result, e
}
