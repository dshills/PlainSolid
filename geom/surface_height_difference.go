package geom

import (
	"math"
	"math/big"
)

// HeightSign is a certified scalar relation. Unresolved is deliberately distinct
// from exact zero/boundary; overlapping enclosures never establish equality.
type HeightSign int

const (
	HeightBelow      HeightSign = -1
	HeightBoundary   HeightSign = 0
	HeightAbove      HeightSign = 1
	HeightUnresolved HeightSign = 2
)

// SurfaceHeightDifferenceOptions bounds each XY inversion independently and
// the additional exact comparison work (defaults 1M operations, 16384 bits).
type SurfaceHeightDifferenceOptions struct {
	Inverse                           SurfaceXYInverseOptions
	MaxOperations, MaxCoefficientBits int
}

// SurfaceHeightDifference reports A-height minus B-height at query.X,query.Y.
// PointVsA/B classify query.Z minus each height. Outside flags prove absence of
// that projected point; its height/sign remain unresolved. Height ranges and
// Difference are outward. ExactOperations counts comparison work, while each
// Inverse result retains its independently bounded proof diagnostics.
type SurfaceHeightDifference struct {
	Relation, PointVsA, PointVsB HeightSign
	HeightA, HeightB, Difference CertifiedScalarRange
	OutsideA, OutsideB           bool
	InverseA, InverseB           SurfaceXYInverse
	ExactOperations              int
}

func heightRangeSign(r CertifiedScalarRange) HeightSign {
	if r.Sign > 0 || r.Min > 0 {
		return HeightAbove
	}
	if r.Sign < 0 || r.Max < 0 {
		return HeightBelow
	}
	if r.Min == 0 && r.Max == 0 {
		return HeightBoundary
	}
	return HeightUnresolved
}
func exactPatchPointAt(p *SurfacePatch, u, v float64, b *polynomialBudget) ([3]*big.Rat, error) {
	current := p
	for {
		t, axis, knot := patchSplitChoice(current)
		if !knot {
			break
		}
		left, right, e := current.split(t, axis, b)
		if e != nil {
			return [3]*big.Rat{}, e
		}
		parameter := u
		if !axis {
			parameter = v
		}
		if parameter < t {
			current = left
		} else {
			current = right
		}
	}
	point, _, e := exactSurfaceJet(current, new(big.Rat).SetFloat64(u), new(big.Rat).SetFloat64(v), b)
	return point, e
}
func commonXYDifference(a, c *SurfacePatch, b *polynomialBudget) (*SurfacePatch, bool, error) {
	if a.degreeU != c.degreeU || a.degreeV != c.degreeV || a.countU != c.countU || a.countV != c.countV || len(a.knotsU) != len(c.knotsU) || len(a.knotsV) != len(c.knotsV) {
		return nil, false, nil
	}
	for i, k := range a.knotsU {
		if k != c.knotsU[i] {
			return nil, false, nil
		}
	}
	for i, k := range a.knotsV {
		if k != c.knotsV[i] {
			return nil, false, nil
		}
	}
	scale := new(big.Rat).Quo(a.net[0][3], c.net[0][3])
	net := make([]exactHomogeneous, len(a.net))
	for i, h := range a.net {
		for _, axis := range []int{0, 1, 3} {
			value := new(big.Rat).Mul(scale, c.net[i][axis])
			if e := b.check(value); e != nil {
				return nil, false, e
			}
			if value.Cmp(h[axis]) != 0 {
				return nil, false, nil
			}
		}
		net[i] = h
		net[i][2] = new(big.Rat).Sub(h[2], new(big.Rat).Mul(scale, c.net[i][2]))
		if e := b.check(net[i][2]); e != nil {
			return nil, false, e
		}
	}
	p := *a
	p.net = net
	if e := p.computeBounds(b); e != nil {
		return nil, false, e
	}
	return &p, true, nil
}

// ClassifySurfaceHeightDifference proves projected injectivity and inverts XY
// on each patch. Exact known UVs give exact height and point-Z comparisons.
// Otherwise height enclosures can prove strict order. For identical XY control
// maps with proportional positive weights and identical bases, exact homogeneous
// height subtraction additionally proves cancellation/boundary without rounded
// pole subtraction. Other unresolved algebraic equalities stay unresolved.
func ClassifySurfaceHeightDifference(a, c *SurfacePatch, query Vec3, o SurfaceHeightDifferenceOptions) (result SurfaceHeightDifference, err error) {
	result.Relation, result.PointVsA, result.PointVsB = HeightUnresolved, HeightUnresolved, HeightUnresolved
	if a == nil || c == nil || !query.IsFinite() {
		return result, ErrInvalid
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
	defer func() { result.ExactOperations = budget.operations }()
	result.InverseA, err = InvertSurfaceXY(a, query.X, query.Y, o.Inverse)
	if err != nil {
		return result, err
	}
	result.OutsideA = result.InverseA.Excluded
	result.InverseB, err = InvertSurfaceXY(c, query.X, query.Y, o.Inverse)
	if err != nil {
		return result, err
	}
	result.OutsideB = result.InverseB.Excluded
	var exactA, exactB *big.Rat
	classify := func(p *SurfacePatch, inverse SurfaceXYInverse) (CertifiedScalarRange, HeightSign, *big.Rat, error) {
		if inverse.Excluded {
			return CertifiedScalarRange{}, HeightUnresolved, nil, nil
		}
		height := inverse.Height
		var exact *big.Rat
		if inverse.UV.Min == inverse.UV.Max {
			point, e := exactPatchPointAt(p, inverse.UV.Min.X, inverse.UV.Min.Y, budget)
			if e != nil {
				return height, HeightUnresolved, nil, e
			}
			exact = point[2]
			height = certifiedRange(exactInterval{exact, exact})
		}
		lo, hi := new(big.Rat).SetFloat64(height.Min), new(big.Rat).SetFloat64(height.Max)
		if exact != nil {
			lo, hi = exact, exact
		}
		z := new(big.Rat).SetFloat64(query.Z)
		delta, e := intervalSubtract(exactInterval{z, z}, exactInterval{lo, hi}, budget)
		if e != nil {
			return height, HeightUnresolved, exact, e
		}
		return height, heightRangeSign(certifiedRange(delta)), exact, nil
	}
	result.HeightA, result.PointVsA, exactA, err = classify(a, result.InverseA)
	if err != nil {
		return result, err
	}
	result.HeightB, result.PointVsB, exactB, err = classify(c, result.InverseB)
	if err != nil {
		return result, err
	}
	if result.OutsideA || result.OutsideB {
		return result, nil
	}
	if exactA != nil && exactB != nil {
		delta := new(big.Rat).Sub(exactA, exactB)
		if e := budget.check(delta); e != nil {
			return result, e
		}
		result.Difference = certifiedRange(exactInterval{delta, delta})
		result.Relation = heightRangeSign(result.Difference)
		return result, nil
	}
	difference, e := intervalSubtract(exactInterval{new(big.Rat).SetFloat64(result.HeightA.Min), new(big.Rat).SetFloat64(result.HeightA.Max)}, exactInterval{new(big.Rat).SetFloat64(result.HeightB.Min), new(big.Rat).SetFloat64(result.HeightB.Max)}, budget)
	if e != nil {
		return result, e
	}
	result.Difference = certifiedRange(difference)
	result.Relation = heightRangeSign(result.Difference)
	if result.Relation != HeightUnresolved {
		return result, nil
	}
	diff, common, e := commonXYDifference(a, c, budget)
	if e != nil {
		return result, e
	}
	if !common {
		return result, nil
	}
	uv := result.InverseA.UV
	if uv.Min == uv.Max {
		point, e := exactPatchPointAt(diff, uv.Min.X, uv.Min.Y, budget)
		if e != nil {
			return result, e
		}
		result.Difference = certifiedRange(exactInterval{point[2], point[2]})
		result.Relation = heightRangeSign(result.Difference)
		return result, nil
	}
	d := a.Domain()
	r := ParameterRectangle{uv.Min.X, uv.Max.X, uv.Min.Y, uv.Max.Y}
	if r.UMin == r.UMax {
		r.UMin = max(d.UMin, math.Nextafter(r.UMin, math.Inf(-1)))
		r.UMax = min(d.UMax, math.Nextafter(r.UMax, math.Inf(1)))
	}
	if r.VMin == r.VMax {
		r.VMin = max(d.VMin, math.Nextafter(r.VMin, math.Inf(-1)))
		r.VMax = min(d.VMax, math.Nextafter(r.VMax, math.Inf(1)))
	}
	patch, e := trimPatchBudget(diff, r, budget)
	if e != nil {
		return result, e
	}
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	bound, e := patch.planeBounds(plane, budget)
	if e != nil {
		return result, e
	}
	result.Difference = CertifiedScalarRange{Min: bound.Min, Max: bound.Max}
	switch bound.Relation {
	case PlanePatchAbove:
		result.Difference.Sign = 1
		result.Relation = HeightAbove
	case PlanePatchBelow:
		result.Difference.Sign = -1
		result.Relation = HeightBelow
	case PlanePatchCoplanar:
		result.Relation = HeightBoundary
	}
	return result, nil
}
