package geom

// SurfaceTransversalityOptions limits exact rational interval work. Zero fields
// select 1M operations and 16384 coefficient bits.
type SurfaceTransversalityOptions struct{ MaxOperations, MaxCoefficientBits int }

// SurfaceTransversality bounds scaled normal vectors and their cross product.
// NormalA/B are cross products of homogeneous derivative numerators, equal to
// W^4 times the actual parameter-surface normals (strictly positive weights).
// CrossNormals is their cross product. These are not unit normal bounds.
// ProvenTransverse means a strictly signed cross-product component proves both
// patches regular and their normals nonparallel at every parameter pair. It
// does not prove that the patches intersect. False means unresolved, not tangent.
type SurfaceTransversality struct {
	NormalA, NormalB, CrossNormals [3]CertifiedScalarRange
	ProvenTransverse               bool
	SeparatingComponent            int
	ExactOperations                int
}

func intervalCross(a, c [3]exactInterval, b *polynomialBudget) (out [3]exactInterval, err error) {
	for axis := 0; axis < 3; axis++ {
		j, k := (axis+1)%3, (axis+2)%3
		p, e := intervalMultiply(a[j], c[k], b)
		if e != nil {
			return out, e
		}
		q, e := intervalMultiply(a[k], c[j], b)
		if e != nil {
			return out, e
		}
		out[axis], e = intervalSubtract(p, q, b)
		if e != nil {
			return out, e
		}
	}
	return out, nil
}
func patchNormalInterval(p *SurfacePatch, b *polynomialBudget) (normal [3]exactInterval, err error) {
	// Reuse exact centered homogeneous field and derivative hull construction.
	plane, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	data, e := p.arcData(plane, b)
	if e != nil {
		return normal, e
	}
	var du, dv [3]exactInterval
	for axis := 0; axis < 3; axis++ {
		for direction := 0; direction < 2; direction++ {
			derivative := data.du
			if direction == 1 {
				derivative = data.dv
			}
			first, e := intervalMultiply(derivative[axis], data.base[3], b)
			if e != nil {
				return normal, e
			}
			second, e := intervalMultiply(data.base[axis], derivative[3], b)
			if e != nil {
				return normal, e
			}
			v, e := intervalSubtract(first, second, b)
			if e != nil {
				return normal, e
			}
			if direction == 0 {
				du[axis] = v
			} else {
				dv[axis] = v
			}
		}
	}
	return intervalCross(du, dv, b)
}

// CertifySurfaceTransversality classifies two single Bezier rectangles using
// exact rational derivative hulls. Split internal knot lines first (for example
// via the knot-aligned domains from BoundSurfaceSurface); non-Bezier patches
// return ErrDomain. Refinement may resolve an inconclusive interval result.
func CertifySurfaceTransversality(a, b *SurfacePatch, options SurfaceTransversalityOptions) (result SurfaceTransversality, err error) {
	result.SeparatingComponent = -1
	if a == nil || b == nil || len(a.net) == 0 || len(b.net) == 0 {
		return result, ErrInvalid
	}
	for _, p := range []*SurfacePatch{a, b} {
		if p.countU != p.degreeU+1 || p.countV != p.degreeV+1 {
			return result, ErrDomain
		}
	}
	o := options
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
	na, e := patchNormalInterval(a, budget)
	if e != nil {
		return result, e
	}
	nb, e := patchNormalInterval(b, budget)
	if e != nil {
		return result, e
	}
	cross, e := intervalCross(na, nb, budget)
	if e != nil {
		return result, e
	}
	for axis := 0; axis < 3; axis++ {
		result.NormalA[axis] = certifiedRange(na[axis])
		result.NormalB[axis] = certifiedRange(nb[axis])
		result.CrossNormals[axis] = certifiedRange(cross[axis])
		if cross[axis].sign() != 0 && !result.ProvenTransverse {
			result.ProvenTransverse = true
			result.SeparatingComponent = axis
		}
	}
	return result, nil
}
