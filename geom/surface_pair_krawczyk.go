package geom

import "math/big"

func scalarBernsteinValue(values []*big.Rat, t *big.Rat, b *polynomialBudget) (*big.Rat, error) {
	work := append([]*big.Rat(nil), values...)
	complement := new(big.Rat).Sub(big.NewRat(1, 1), t)
	for n := len(work) - 1; n > 0; n-- {
		for i := 0; i < n; i++ {
			v := new(big.Rat).Add(new(big.Rat).Mul(complement, work[i]), new(big.Rat).Mul(t, work[i+1]))
			if e := b.check(v); e != nil {
				return nil, e
			}
			work[i] = v
		}
	}
	return work[0], nil
}
func tensorScalarValue(values []*big.Rat, nu, nv int, u, v *big.Rat, b *polynomialBudget) (*big.Rat, error) {
	rows := make([]*big.Rat, nu)
	for i := range rows {
		x, e := scalarBernsteinValue(values[i*nv:(i+1)*nv], v, b)
		if e != nil {
			return nil, e
		}
		rows[i] = x
	}
	return scalarBernsteinValue(rows, u, b)
}
func exactSurfaceJet(p *SurfacePatch, u, v *big.Rat, b *polynomialBudget) (point [3]*big.Rat, derivative [3][2]*big.Rat, err error) {
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	data, e := p.arcData(plane, b)
	if e != nil {
		return point, derivative, e
	}
	domain := p.Domain()
	normalize := func(x *big.Rat, lo, hi float64) *big.Rat {
		return new(big.Rat).Quo(new(big.Rat).Sub(x, new(big.Rat).SetFloat64(lo)), new(big.Rat).Sub(new(big.Rat).SetFloat64(hi), new(big.Rat).SetFloat64(lo)))
	}
	un, vn := normalize(u, domain.UMin, domain.UMax), normalize(v, domain.VMin, domain.VMax)
	var h, hu, hv [4]*big.Rat
	for field := 0; field < 4; field++ {
		values := data.fields[field]
		h[field], e = tensorScalarValue(values, p.countU, p.countV, un, vn, b)
		if e != nil {
			return point, derivative, e
		}
		du, e := scalarPartial(values, p.countU, p.countV, true, domain, b)
		if e != nil {
			return point, derivative, e
		}
		dv, e := scalarPartial(values, p.countU, p.countV, false, domain, b)
		if e != nil {
			return point, derivative, e
		}
		hu[field], e = tensorScalarValue(du, p.countU-1, p.countV, un, vn, b)
		if e != nil {
			return point, derivative, e
		}
		hv[field], e = tensorScalarValue(dv, p.countU, p.countV-1, un, vn, b)
		if e != nil {
			return point, derivative, e
		}
	}
	denominator := new(big.Rat).Mul(h[3], h[3])
	for axis := 0; axis < 3; axis++ {
		point[axis] = new(big.Rat).Add(new(big.Rat).Quo(h[axis], h[3]), new(big.Rat).SetFloat64(coordinate(p.bounds.Min, axis)))
		if e := b.check(point[axis]); e != nil {
			return point, derivative, e
		}
		for direction, d := range [2][4]*big.Rat{hu, hv} {
			value := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(d[axis], h[3]), new(big.Rat).Mul(h[axis], d[3])), denominator)
			if e := b.check(value); e != nil {
				return point, derivative, e
			}
			derivative[axis][direction] = value
		}
	}
	return point, derivative, nil
}
func inverseExact3(matrix [3][3]*big.Rat, b *polynomialBudget) (inverse [3][3]*big.Rat, ok bool, err error) {
	var augmented [3][6]*big.Rat
	for i := 0; i < 3; i++ {
		for j := 0; j < 6; j++ {
			augmented[i][j] = new(big.Rat)
			if j < 3 {
				augmented[i][j].Set(matrix[i][j])
			} else if j == i+3 {
				augmented[i][j].SetInt64(1)
			}
		}
	}
	for column := 0; column < 3; column++ {
		pivot := column
		for pivot < 3 && augmented[pivot][column].Sign() == 0 {
			pivot++
		}
		if pivot == 3 {
			return inverse, false, nil
		}
		augmented[column], augmented[pivot] = augmented[pivot], augmented[column]
		scale := new(big.Rat).Set(augmented[column][column])
		for j := 0; j < 6; j++ {
			augmented[column][j] = new(big.Rat).Quo(augmented[column][j], scale)
			if e := b.check(augmented[column][j]); e != nil {
				return inverse, false, e
			}
		}
		for i := 0; i < 3; i++ {
			if i == column {
				continue
			}
			factor := new(big.Rat).Set(augmented[i][column])
			for j := 0; j < 6; j++ {
				augmented[i][j] = new(big.Rat).Sub(augmented[i][j], new(big.Rat).Mul(factor, augmented[column][j]))
				if e := b.check(augmented[i][j]); e != nil {
					return inverse, false, e
				}
			}
		}
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			inverse[i][j] = augmented[i][j+3]
		}
	}
	return inverse, true, nil
}

// SurfaceSectionCertificate is a local three-variable contact certificate for
// one fixed parameter. KrawczykBounds are outward enclosures of the Krawczyk
// image, ordered by ActiveAxes. Every contact in the supplied section lies in
// that image. ProvenUnique requires strict interior inclusion and a strict
// infinity-norm contraction bound <1, proving existence and uniqueness in the
// section. ExcludesContact proves no contact; neither flag means unresolved.
// A singular center Jacobian is unresolved and leaves KrawczykBounds zero.
type SurfaceSectionCertificate struct {
	// JacobianDeterminantSign is exact at the section center, with active
	// columns in ActiveAxes order. Global nonvanishing requires ProvenUnique.
	JacobianDeterminantSign                         int
	ActiveAxes                                      [3]int
	KrawczykBounds                                  [3]CertifiedScalarRange
	CenterResidual                                  [3]CertifiedScalarRange
	ContractionBound                                float64
	CenterInvertible, ProvenUnique, ExcludesContact bool
	ExactOperations                                 int
}

// CertifySurfacePairSection performs one exact-rational Krawczyk test on two
// Bezier rectangles. UseFixedParameter must be true. It uses an exact center
// Jacobian inverse and interval Jacobian bounds; no numerical seed, heuristic
// root acceptance or existence claim from overlapping hulls is used. Refining
// unresolved cells is the caller's responsibility. Contacts on section-domain
// boundaries may remain unresolved because inclusion must be strict.
func CertifySurfacePairSection(a, c *SurfacePatch, options SurfacePairIntervalOptions) (result SurfaceSectionCertificate, err error) {
	return certifySurfacePairBox(a, c, options, false)
}

func certifySurfacePairBox(a, c *SurfacePatch, options SurfacePairIntervalOptions, tube bool) (result SurfaceSectionCertificate, err error) {
	o := options
	if !o.UseFixedParameter || o.FixedAxis < 0 || o.FixedAxis > 3 || !finite(o.FixedParameter) || a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 {
		return result, ErrInvalid
	}
	for _, p := range []*SurfacePatch{a, c} {
		if p.countU != p.degreeU+1 || p.countV != p.degreeV+1 {
			return result, ErrDomain
		}
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
	da, dc := a.Domain(), c.Domain()
	ranges := [4][2]float64{{da.UMin, da.UMax}, {da.VMin, da.VMax}, {dc.UMin, dc.UMax}, {dc.VMin, dc.VMax}}
	if o.FixedParameter < ranges[o.FixedAxis][0] || o.FixedParameter > ranges[o.FixedAxis][1] {
		return result, ErrDomain
	}
	var center [4]*big.Rat
	active := 0
	for axis, r := range ranges {
		center[axis] = new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).SetFloat64(r[0]), new(big.Rat).SetFloat64(r[1])), big.NewRat(2, 1))
		if axis != o.FixedAxis {
			result.ActiveAxes[active] = axis
			active++
		}
	}
	if !tube {
		center[o.FixedAxis] = new(big.Rat).SetFloat64(o.FixedParameter)
	}
	ap, ad, e := exactSurfaceJet(a, center[0], center[1], b)
	if e != nil {
		return result, e
	}
	cp, cd, e := exactSurfaceJet(c, center[2], center[3], b)
	if e != nil {
		return result, e
	}
	var residual [3]*big.Rat
	var matrix [3][3]*big.Rat
	for world := 0; world < 3; world++ {
		residual[world] = new(big.Rat).Sub(ap[world], cp[world])
		result.CenterResidual[world] = certifiedRange(exactInterval{residual[world], residual[world]})
		for j, axis := range result.ActiveAxes {
			if axis < 2 {
				matrix[world][j] = ad[world][axis]
			} else {
				matrix[world][j] = new(big.Rat).Neg(cd[world][axis-2])
			}
		}
	}
	determinant := new(big.Rat)
	for column := 0; column < 3; column++ {
		j, k := (column+1)%3, (column+2)%3
		minor := new(big.Rat).Sub(new(big.Rat).Mul(matrix[1][j], matrix[2][k]), new(big.Rat).Mul(matrix[1][k], matrix[2][j]))
		determinant.Add(determinant, new(big.Rat).Mul(matrix[0][column], minor))
	}
	if e := b.check(determinant); e != nil {
		return result, e
	}
	result.JacobianDeterminantSign = determinant.Sign()
	inverse, ok, e := inverseExact3(matrix, b)
	if e != nil {
		return result, e
	}
	if !ok {
		return result, nil
	}
	result.CenterInvertible = true
	aa, ca := -1, -1
	if !tube {
		if o.FixedAxis < 2 {
			aa = o.FixedAxis
		} else {
			ca = o.FixedAxis - 2
		}
	}
	_, adi, e := surfaceDifferentialIntervals(a, aa, o.FixedParameter, b)
	if e != nil {
		return result, e
	}
	_, cdi, e := surfaceDifferentialIntervals(c, ca, o.FixedParameter, b)
	if e != nil {
		return result, e
	}
	var jacobian [3][3]exactInterval
	for world := 0; world < 3; world++ {
		for j, axis := range result.ActiveAxes {
			if axis < 2 {
				jacobian[world][j] = adi[world][axis]
			} else {
				v := cdi[world][axis-2]
				jacobian[world][j] = exactInterval{new(big.Rat).Neg(v.hi), new(big.Rat).Neg(v.lo)}
			}
		}
	}
	centerIntervals := [3]exactInterval{}
	for world := 0; world < 3; world++ {
		centerIntervals[world] = exactInterval{residual[world], residual[world]}
	}
	if tube {
		freeA, freeC := -1, -1
		if o.FixedAxis < 2 {
			freeA = o.FixedAxis
		} else {
			freeC = o.FixedAxis - 2
		}
		ap, e := centerLinePositions(a, freeA, center[0], center[1], b)
		if e != nil {
			return result, e
		}
		cp, e := centerLinePositions(c, freeC, center[2], center[3], b)
		if e != nil {
			return result, e
		}
		for world := 0; world < 3; world++ {
			centerIntervals[world], e = intervalSubtract(ap[world], cp[world], b)
			if e != nil {
				return result, e
			}
			result.CenterResidual[world] = certifiedRange(centerIntervals[world])
		}
	}
	strict := true
	maxNorm := new(big.Rat)
	for i, axis := range result.ActiveAxes {
		image := exactInterval{new(big.Rat).Set(center[axis]), new(big.Rat).Set(center[axis])}
		for world := 0; world < 3; world++ {
			term, e := intervalMultiply(exactInterval{inverse[i][world], inverse[i][world]}, centerIntervals[world], b)
			if e != nil {
				return result, e
			}
			image, e = intervalSubtract(image, term, b)
			if e != nil {
				return result, e
			}
		}
		rowNorm := new(big.Rat)
		for j, columnAxis := range result.ActiveAxes {
			entry := exactInterval{new(big.Rat), new(big.Rat)}
			if i == j {
				entry.lo.SetInt64(1)
				entry.hi.SetInt64(1)
			}
			for world := 0; world < 3; world++ {
				term, e := intervalMultiply(exactInterval{inverse[i][world], inverse[i][world]}, jacobian[world][j], b)
				if e != nil {
					return result, e
				}
				entry, e = intervalSubtract(entry, term, b)
				if e != nil {
					return result, e
				}
			}
			magnitude := new(big.Rat).Abs(entry.lo)
			upper := new(big.Rat).Abs(entry.hi)
			if upper.Cmp(magnitude) > 0 {
				magnitude = upper
			}
			rowNorm.Add(rowNorm, magnitude)
			offset := exactInterval{new(big.Rat).Sub(new(big.Rat).SetFloat64(ranges[columnAxis][0]), center[columnAxis]), new(big.Rat).Sub(new(big.Rat).SetFloat64(ranges[columnAxis][1]), center[columnAxis])}
			term, e := intervalMultiply(entry, offset, b)
			if e != nil {
				return result, e
			}
			image, e = boundedExactInterval(new(big.Rat).Add(image.lo, term.lo), new(big.Rat).Add(image.hi, term.hi), b)
			if e != nil {
				return result, e
			}
		}
		if rowNorm.Cmp(maxNorm) > 0 {
			maxNorm.Set(rowNorm)
		}
		lo, hi := new(big.Rat).SetFloat64(ranges[axis][0]), new(big.Rat).SetFloat64(ranges[axis][1])
		if image.lo.Cmp(lo) <= 0 || image.hi.Cmp(hi) >= 0 {
			strict = false
		}
		if image.hi.Cmp(lo) < 0 || image.lo.Cmp(hi) > 0 {
			result.ExcludesContact = true
		}
		result.KrawczykBounds[i] = certifiedRange(image)
	}
	result.ContractionBound = rationalFloatBound(maxNorm, false)
	result.ProvenUnique = strict && maxNorm.Cmp(big.NewRat(1, 1)) < 0
	return result, nil
}
