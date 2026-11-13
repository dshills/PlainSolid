package geom

import "math/big"

// SurfaceTubeEndpointOptions selects a parameter-domain endpoint on each tube.
// End selectors are 0=lower, 1=upper. Proof limits apply independently to tube
// and section tests; exact extraction uses each patch's trim limits.
type SurfaceTubeEndpointOptions struct {
	// ExactParameterHint optionally proposes all four rational coordinates. It
	// is never trusted: both fixed endpoints, box membership and zero residual
	// are verified exactly. Wrong hints leave equality unresolved.
	ExactParameterHint                             [4]*big.Rat
	LeftFreeAxis, RightFreeAxis, LeftEnd, RightEnd int
	MaxOperations, MaxCoefficientBits              int
}

// SurfaceTubeEndpointJunction concerns endpoint equality, not interior overlap.
// Connected requires exact rational endpoint solutions, from singleton
// Krawczyk images or an exactly verified rational Newton proposal. Separated follows
// from disjoint certified parameter boxes or disjoint spatial boxes. Neither
// flag means unresolved; overlapping enclosures never prove equality.
type SurfaceTubeEndpointJunction struct {
	Connected, Separated            bool
	Left, Right                     SurfaceTubeCertificate
	LeftEndpoint, RightEndpoint     SurfaceSectionCertificate
	ParametersLeft, ParametersRight [4]CertifiedScalarRange
	ExactOperations                 int
}

// CertifySurfaceTubeEndpointJunction checks two selected endpoints from the same
// ordered pair of original surfaces. It supports different free axes without
// confusing overlap with equality. Exact rational singleton section images are
// a deliberately limited equality domain, supplemented by exactly verified
// rational Newton proposals. Other equalities remain unresolved.
// A parameter-box separation proves distinct paired-chart endpoints, even when
// a globally self-overlapping surface could place them at the same spatial point.
func CertifySurfaceTubeEndpointJunction(a, c *SurfacePatch, leftA, leftC, rightA, rightC ParameterRectangle, o SurfaceTubeEndpointOptions) (result SurfaceTubeEndpointJunction, err error) {
	if a == nil || c == nil || o.LeftFreeAxis < 0 || o.LeftFreeAxis > 3 || o.RightFreeAxis < 0 || o.RightFreeAxis > 3 || o.LeftEnd < 0 || o.LeftEnd > 1 || o.RightEnd < 0 || o.RightEnd > 1 {
		return result, ErrInvalid
	}
	la, e := a.Trim(leftA)
	if e != nil {
		return result, e
	}
	lc, e := c.Trim(leftC)
	if e != nil {
		return result, e
	}
	ra, e := a.Trim(rightA)
	if e != nil {
		return result, e
	}
	rc, e := c.Trim(rightC)
	if e != nil {
		return result, e
	}
	options := func(axis int) SurfacePairTubeOptions {
		return SurfacePairTubeOptions{FreeAxis: axis, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	}
	result.Left, e = CertifySurfacePairTube(la, lc, options(o.LeftFreeAxis))
	result.ExactOperations += result.Left.Test.ExactOperations
	if e != nil {
		return result, e
	}
	result.Right, e = CertifySurfacePairTube(ra, rc, options(o.RightFreeAxis))
	result.ExactOperations += result.Right.Test.ExactOperations
	if e != nil {
		return result, e
	}
	if !result.Left.ProvenBranch || !result.Right.ProvenBranch {
		return result, nil
	}
	ranges := func(x, y ParameterRectangle) [4][2]float64 {
		return [4][2]float64{{x.UMin, x.UMax}, {x.VMin, x.VMax}, {y.UMin, y.UMax}, {y.VMin, y.VMax}}
	}
	l, r := ranges(leftA, leftC), ranges(rightA, rightC)
	lv, rv := l[o.LeftFreeAxis][o.LeftEnd], r[o.RightFreeAxis][o.RightEnd]
	section := func(p, q *SurfacePatch, axis int, value float64) (SurfaceSectionCertificate, error) {
		return CertifySurfacePairSection(p, q, SurfacePairIntervalOptions{UseFixedParameter: true, FixedAxis: axis, FixedParameter: value, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	}
	result.LeftEndpoint, e = section(la, lc, o.LeftFreeAxis, lv)
	result.ExactOperations += result.LeftEndpoint.ExactOperations
	if e != nil {
		return result, e
	}
	result.RightEndpoint, e = section(ra, rc, o.RightFreeAxis, rv)
	result.ExactOperations += result.RightEndpoint.ExactOperations
	if e != nil {
		return result, e
	}
	enclose := func(cert SurfaceSectionCertificate, axis int, value float64, domain [4][2]float64) [4]CertifiedScalarRange {
		var out [4]CertifiedScalarRange
		for i, d := range domain {
			out[i] = CertifiedScalarRange{Min: d[0], Max: d[1]}
		}
		out[axis] = CertifiedScalarRange{Min: value, Max: value}
		if cert.CenterInvertible {
			for j, i := range cert.ActiveAxes {
				out[i].Min = max(out[i].Min, cert.KrawczykBounds[j].Min)
				out[i].Max = min(out[i].Max, cert.KrawczykBounds[j].Max)
			}
		}
		return out
	}
	result.ParametersLeft = enclose(result.LeftEndpoint, o.LeftFreeAxis, lv, l)
	result.ParametersRight = enclose(result.RightEndpoint, o.RightFreeAxis, rv, r)
	equalSingleton := true
	for i := 0; i < 4; i++ {
		x, y := result.ParametersLeft[i], result.ParametersRight[i]
		if x.Max < y.Min || y.Max < x.Min {
			result.Separated = true
			return result, nil
		}
		if x.Min != x.Max || y.Min != y.Max || x.Min != y.Min {
			equalSingleton = false
		}
	}
	// Singleton float intervals alone are exact: outward rounding can collapse
	// to a singleton only for an exactly representable rational bound. Tube
	// existence ensures the endpoint root lies in every section enclosure.
	if equalSingleton {
		result.Connected = true
		return result, nil
	}
	// An exact rational Newton proposal is accepted only after exact residual
	// evaluation and membership in both boxes, never merely because it is close.
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	if budget.maxOperations == 0 {
		budget.maxOperations = 1000000
	}
	if budget.maxBits == 0 {
		budget.maxBits = 16384
	}
	// Try a supplied rational hint, then the exact midpoint of the intersected
	// endpoint enclosures. Neither proposal is accepted without exact validation.
	hints := [][4]*big.Rat{}
	supplied := false
	for _, x := range o.ExactParameterHint {
		if x != nil {
			supplied = true
		}
	}
	if supplied {
		for _, x := range o.ExactParameterHint {
			if x == nil {
				return result, ErrInvalid
			}
		}
		hints = append(hints, o.ExactParameterHint)
	}
	var middle [4]*big.Rat
	for axis := 0; axis < 4; axis++ {
		lo := max(result.ParametersLeft[axis].Min, result.ParametersRight[axis].Min)
		hi := min(result.ParametersLeft[axis].Max, result.ParametersRight[axis].Max)
		middle[axis] = new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).SetFloat64(lo), new(big.Rat).SetFloat64(hi)), big.NewRat(2, 1))
	}
	hints = append(hints, middle)
	for _, hint := range hints {
		verified, e := verifyEndpointHint(la, lc, l, r, o.LeftFreeAxis, lv, o.RightFreeAxis, rv, hint, budget)
		if e != nil {
			result.ExactOperations += budget.operations
			return result, e
		}
		if verified {
			for axis, x := range hint {
				rangeValue := certifiedRange(exactInterval{x, x})
				result.ParametersLeft[axis], result.ParametersRight[axis] = rangeValue, rangeValue
			}
			result.Connected = true
			result.ExactOperations += budget.operations
			return result, nil
		}
	}
	rationalEqual, e := rationalEndpointEquality(la, lc, l, r, o.LeftFreeAxis, lv, o.RightFreeAxis, rv, budget)
	result.ExactOperations += budget.operations
	if e != nil {
		return result, e
	}
	if rationalEqual {
		result.Connected = true
		return result, nil
	}
	// For nonsingleton parameter enclosures, spatial hull separation remains a
	// valid additional disproof. A zero-width parameter box is expanded within
	// the source tube solely to obtain a positive-area patch hull.
	spatial := func(p *SurfacePatch, uv [2]CertifiedScalarRange) (Box, error) {
		d := p.Domain()
		loU, hiU, loV, hiV := uv[0].Min, uv[0].Max, uv[1].Min, uv[1].Max
		if loU == hiU {
			loU, hiU = d.UMin, d.UMax
		}
		if loV == hiV {
			loV, hiV = d.VMin, d.VMax
		}
		q, e := p.Trim(ParameterRectangle{loU, hiU, loV, hiV})
		if e != nil {
			return Box{}, e
		}
		return q.bounds, nil
	}
	for _, pair := range []struct {
		p, q   *SurfacePatch
		offset int
	}{{la, ra, 0}, {lc, rc, 2}} {
		x, e := spatial(pair.p, [2]CertifiedScalarRange{result.ParametersLeft[pair.offset], result.ParametersLeft[pair.offset+1]})
		if e != nil {
			return result, e
		}
		y, e := spatial(pair.q, [2]CertifiedScalarRange{result.ParametersRight[pair.offset], result.ParametersRight[pair.offset+1]})
		if e != nil {
			return result, e
		}
		if _, overlap := x.Intersect(y); !overlap {
			result.Separated = true
			return result, nil
		}
	}
	return result, nil
}

func rationalEndpointEquality(a, c *SurfacePatch, left, right [4][2]float64, axis int, value float64, rightAxis int, rightValue float64, b *polynomialBudget) (bool, error) {
	var parameters [4]*big.Rat
	var active [3]int
	j := 0
	for i, r := range left {
		parameters[i] = new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).SetFloat64(r[0]), new(big.Rat).SetFloat64(r[1])), big.NewRat(2, 1))
		if i != axis {
			active[j] = i
			j++
		}
	}
	parameters[axis] = new(big.Rat).SetFloat64(value)
	ap, ad, e := exactSurfaceJet(a, parameters[0], parameters[1], b)
	if e != nil {
		return false, e
	}
	cp, cd, e := exactSurfaceJet(c, parameters[2], parameters[3], b)
	if e != nil {
		return false, e
	}
	var matrix [3][3]*big.Rat
	var residual [3]*big.Rat
	for world := 0; world < 3; world++ {
		residual[world] = new(big.Rat).Sub(ap[world], cp[world])
		for j, i := range active {
			if i < 2 {
				matrix[world][j] = ad[world][i]
			} else {
				matrix[world][j] = new(big.Rat).Neg(cd[world][i-2])
			}
		}
	}
	inverse, ok, e := inverseExact3(matrix, b)
	if e != nil || !ok {
		return false, e
	}
	for i, coordinate := range active {
		correction := new(big.Rat)
		for k := 0; k < 3; k++ {
			correction.Add(correction, new(big.Rat).Mul(inverse[i][k], residual[k]))
		}
		parameters[coordinate] = new(big.Rat).Sub(parameters[coordinate], correction)
		if e := b.check(parameters[coordinate]); e != nil {
			return false, e
		}
	}
	if parameters[rightAxis].Cmp(new(big.Rat).SetFloat64(rightValue)) != 0 {
		return false, nil
	}
	for i, p := range parameters {
		for _, box := range [][4][2]float64{left, right} {
			if p.Cmp(new(big.Rat).SetFloat64(box[i][0])) < 0 || p.Cmp(new(big.Rat).SetFloat64(box[i][1])) > 0 {
				return false, nil
			}
		}
	}
	ap, _, e = exactSurfaceJet(a, parameters[0], parameters[1], b)
	if e != nil {
		return false, e
	}
	cp, _, e = exactSurfaceJet(c, parameters[2], parameters[3], b)
	if e != nil {
		return false, e
	}
	for world := 0; world < 3; world++ {
		if ap[world].Cmp(cp[world]) != 0 {
			return false, nil
		}
	}
	return true, nil
}

func verifyEndpointHint(a, c *SurfacePatch, left, right [4][2]float64, leftAxis int, leftValue float64, rightAxis int, rightValue float64, hint [4]*big.Rat, b *polynomialBudget) (bool, error) {
	for _, x := range hint {
		if x == nil {
			return false, ErrInvalid
		}
		if e := b.check(x); e != nil {
			return false, e
		}
	}
	if hint[leftAxis].Cmp(new(big.Rat).SetFloat64(leftValue)) != 0 || hint[rightAxis].Cmp(new(big.Rat).SetFloat64(rightValue)) != 0 {
		return false, nil
	}
	for axis, x := range hint {
		for _, domain := range [][4][2]float64{left, right} {
			if x.Cmp(new(big.Rat).SetFloat64(domain[axis][0])) < 0 || x.Cmp(new(big.Rat).SetFloat64(domain[axis][1])) > 0 {
				return false, nil
			}
		}
	}
	ap, _, e := exactSurfaceJet(a, hint[0], hint[1], b)
	if e != nil {
		return false, e
	}
	cp, _, e := exactSurfaceJet(c, hint[2], hint[3], b)
	if e != nil {
		return false, e
	}
	for axis := 0; axis < 3; axis++ {
		if ap[axis].Cmp(cp[axis]) != 0 {
			return false, nil
		}
	}
	return true, nil
}
