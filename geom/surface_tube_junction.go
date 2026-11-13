package geom

// SurfaceTubeJunctionOptions chooses the common free axis. Arithmetic limits
// apply independently to each of the two tube proofs and the overlap section.
// Zero limits select the defaults of SurfacePairTubeOptions.
type SurfaceTubeJunctionOptions struct{ FreeAxis, MaxOperations, MaxCoefficientBits int }

// SurfaceTubeJunction distinguishes a proved shared branch endpoint from a
// proved absence of a shared endpoint; otherwise the junction is unresolved.
// Connected requires both uniform tube certificates and a unique section root
// strictly inside the common dependent box. Excluded concerns a common root,
// not the existence of either individual tube branch. ExactOperations counts
// proof arithmetic; exact trimming uses each patch's separate operation limits.
type SurfaceTubeJunction struct {
	Connected, Excluded bool
	Parameter           float64
	Left, Right         SurfaceTubeCertificate
	Section             SurfaceSectionCertificate
	ExactOperations     int
}

// CertifySurfaceTubeJunction proves a junction for two boxes from the SAME
// underlying ordered surface pair. Pass the original surfaces as a,c and four
// parameter rectangles describing the left/right tubes. This prevents silently
// identifying geometrically different surfaces that share parameter ranges.
// The free intervals must abut exactly; all three dependent domains are
// intersected. Positive-width common dependent domains are required for strict
// Krawczyk inclusion. Boundary-only overlap remains unresolved. Inputs must be
// knot-aligned Bezier subrectangles; Trim performs exact rational subdivision.
func CertifySurfaceTubeJunction(a, c *SurfacePatch, leftA, leftC, rightA, rightC ParameterRectangle, o SurfaceTubeJunctionOptions) (result SurfaceTubeJunction, err error) {
	if a == nil || c == nil || o.FreeAxis < 0 || o.FreeAxis > 3 {
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
	ranges := func(x, y ParameterRectangle) [4][2]float64 {
		return [4][2]float64{{x.UMin, x.UMax}, {x.VMin, x.VMax}, {y.UMin, y.UMax}, {y.VMin, y.VMax}}
	}
	l, r := ranges(leftA, leftC), ranges(rightA, rightC)
	axis := o.FreeAxis
	if l[axis][1] == r[axis][0] {
		result.Parameter = l[axis][1]
	} else if r[axis][1] == l[axis][0] {
		result.Parameter = r[axis][1]
	} else {
		return result, ErrDomain
	}
	options := SurfacePairTubeOptions{FreeAxis: axis, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	result.Left, e = CertifySurfacePairTube(la, lc, options)
	result.ExactOperations += result.Left.Test.ExactOperations
	if e != nil {
		return result, e
	}
	result.Right, e = CertifySurfacePairTube(ra, rc, options)
	result.ExactOperations += result.Right.Test.ExactOperations
	if e != nil {
		return result, e
	}
	common := l
	for i := 0; i < 4; i++ {
		if i == axis {
			continue
		}
		common[i] = [2]float64{max(l[i][0], r[i][0]), min(l[i][1], r[i][1])}
		if common[i][0] > common[i][1] {
			result.Excluded = true
			return result, nil
		}
		if common[i][0] == common[i][1] {
			return result, nil
		}
	}
	// Retain the left free interval for patch construction; section evaluation
	// fixes it exactly to the shared endpoint, so the other free extent is unused.
	da := ParameterRectangle{common[0][0], common[0][1], common[1][0], common[1][1]}
	dc := ParameterRectangle{common[2][0], common[2][1], common[3][0], common[3][1]}
	pa, e := a.Trim(da)
	if e != nil {
		return result, e
	}
	pc, e := c.Trim(dc)
	if e != nil {
		return result, e
	}
	result.Section, e = CertifySurfacePairSection(pa, pc, SurfacePairIntervalOptions{UseFixedParameter: true, FixedAxis: axis, FixedParameter: result.Parameter, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	result.ExactOperations += result.Section.ExactOperations
	if e != nil {
		return result, e
	}
	result.Excluded = result.Section.ExcludesContact
	result.Connected = result.Left.ProvenBranch && result.Right.ProvenBranch && result.Section.ProvenUnique
	return result, nil
}
