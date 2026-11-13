package geom

import "math/big"

// SurfacePairTubeOptions chooses the free coordinate among (u,v,s,t). Its whole
// closed domain in the supplied patch is the tube interval; trim the patch first
// for a smaller interval. Other coordinates use their complete patch domains.
// Zero arithmetic limits select 1M operations and 16384 coefficient bits.
type SurfacePairTubeOptions struct{ FreeAxis, MaxOperations, MaxCoefficientBits int }

// SurfaceTubeCertificate is a uniform parameterized Krawczyk certificate.
// ProvenBranch proves exactly one contact for EVERY free parameter in its
// closed interval, with dependent parameters strictly inside their box. Uniform
// contraction and smooth rational geometry imply a continuous (indeed smooth)
// parameter branch, including one-sided limits at interval endpoints. It makes
// no global spatial injectivity or outside-box completeness claim.
// Test contains uniform Krawczyk bounds, full-range center residual bounds and
// contraction diagnostics. ExcludesContact means the entire tube is disjoint.
// Inconclusive tests or a singular center preconditioner remain unresolved.
type SurfaceTubeCertificate struct {
	// FreeTangentSign is (-1)^FreeAxis times the active 3x3 Jacobian
	// determinant sign, for columns [Au,Av,-Bs,-Bt]. Zero unless ProvenBranch;
	// uniform contraction certifies this cofactor never vanishes on the box.
	FreeTangentSign               int
	FreeAxis                      int
	FreeParameter                 CertifiedScalarRange
	Test                          SurfaceSectionCertificate
	ProvenBranch, ExcludesContact bool
}

// CertifySurfacePairTube applies one center preconditioner uniformly over a
// four-parameter box. The center residual retains the free parameter interval;
// only dependent parameters are fixed at their exact rational midpoints.
// Strict uniform inclusion AND contraction are required for branch existence.
func CertifySurfacePairTube(a, c *SurfacePatch, o SurfacePairTubeOptions) (result SurfaceTubeCertificate, err error) {
	if a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 || o.FreeAxis < 0 || o.FreeAxis > 3 {
		return result, ErrInvalid
	}
	da, dc := a.Domain(), c.Domain()
	ranges := [4][2]float64{{da.UMin, da.UMax}, {da.VMin, da.VMax}, {dc.UMin, dc.UMax}, {dc.VMin, dc.VMax}}
	r := ranges[o.FreeAxis]
	result.FreeAxis = o.FreeAxis
	result.FreeParameter = certifiedRange(exactInterval{new(big.Rat).SetFloat64(r[0]), new(big.Rat).SetFloat64(r[1])})
	options := SurfacePairIntervalOptions{UseFixedParameter: true, FixedAxis: o.FreeAxis, FixedParameter: r[0], MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	result.Test, err = certifySurfacePairBox(a, c, options, true)
	result.ProvenBranch = result.Test.ProvenUnique
	if result.ProvenBranch {
		result.FreeTangentSign = result.Test.JacobianDeterminantSign
		if o.FreeAxis%2 != 0 {
			result.FreeTangentSign = -result.FreeTangentSign
		}
	}
	result.ExcludesContact = result.Test.ExcludesContact
	return result, err
}
func centerLinePositions(p *SurfacePatch, freeAxis int, u, v *big.Rat, b *polynomialBudget) (position [3]exactInterval, err error) {
	if freeAxis < 0 {
		point, _, e := exactSurfaceJet(p, u, v, b)
		if e != nil {
			return position, e
		}
		for axis := 0; axis < 3; axis++ {
			position[axis] = exactInterval{point[axis], point[axis]}
		}
		return position, nil
	}
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	data, e := p.arcData(plane, b)
	if e != nil {
		return position, e
	}
	d := p.Domain()
	fixedAxis := 1 - freeAxis
	x, lo, hi := u, d.UMin, d.UMax
	if fixedAxis == 1 {
		x, lo, hi = v, d.VMin, d.VMax
	}
	normalized := new(big.Rat).Quo(new(big.Rat).Sub(x, new(big.Rat).SetFloat64(lo)), new(big.Rat).Sub(new(big.Rat).SetFloat64(hi), new(big.Rat).SetFloat64(lo)))
	var fields [4]exactInterval
	for field := 0; field < 4; field++ {
		fields[field], e = restrictedScalarRange(data.fields[field], p.countU, p.countV, fixedAxis, normalized, b)
		if e != nil {
			return position, e
		}
	}
	for axis := 0; axis < 3; axis++ {
		value, e := intervalDivide(fields[axis], fields[3], b)
		if e != nil {
			return position, e
		}
		center := new(big.Rat).SetFloat64(coordinate(p.bounds.Min, axis))
		position[axis], e = boundedExactInterval(new(big.Rat).Add(value.lo, center), new(big.Rat).Add(value.hi, center), b)
		if e != nil {
			return position, e
		}
	}
	return position, nil
}
