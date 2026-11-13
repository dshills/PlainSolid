package geom

// SurfaceTubeOverlapOptions chooses the independently parameterized tubes and
// one section used to witness their overlap. WitnessAxis must equal LeftFreeAxis
// or RightFreeAxis. WitnessParameter is exact as a binary float. Proof arithmetic
// limits apply independently to each tube and to the common section.
type SurfaceTubeOverlapOptions struct {
	LeftFreeAxis, RightFreeAxis, WitnessAxis int
	WitnessParameter                         float64
	MaxOperations, MaxCoefficientBits        int
}

// SurfaceTubeOverlap proves local branch identity, not endpoint equality or a
// global one-to-one correspondence of whole tubes. SameLocalBranch means both
// tubes are certified and share a regular contact strictly inside both 4D boxes.
// They consequently describe the same smooth intersection branch locally.
// Excluded means the common boxes/selected witness section have no contact; it
// does not exclude a different overlap elsewhere. Neither flag is unresolved.
// Operations count proof arithmetic; exact trimming has separate patch limits.
type SurfaceTubeOverlap struct {
	SameLocalBranch, Excluded bool
	Left, Right               SurfaceTubeCertificate
	Witness                   SurfaceSectionCertificate
	WitnessAxis               int
	WitnessParameter          float64
	ExactOperations           int
}

// CertifySurfaceTubeOverlap uses shared original patches and four trim domains
// to avoid asserting identity between unrelated surfaces. A regular common root
// is certified by strict section Krawczyk inclusion inside the intersection of
// the dependent boxes, while the fixed witness parameter must lie strictly
// inside both free and dependent extents on its axis. This is an overlap test;
// a witness at a tube end remains unresolved, not a proved endpoint junction.
func CertifySurfaceTubeOverlap(a, c *SurfacePatch, leftA, leftC, rightA, rightC ParameterRectangle, o SurfaceTubeOverlapOptions) (result SurfaceTubeOverlap, err error) {
	if a == nil || c == nil || o.LeftFreeAxis < 0 || o.LeftFreeAxis > 3 || o.RightFreeAxis < 0 || o.RightFreeAxis > 3 || (o.WitnessAxis != o.LeftFreeAxis && o.WitnessAxis != o.RightFreeAxis) || !finite(o.WitnessParameter) {
		return result, ErrInvalid
	}
	result.WitnessAxis, result.WitnessParameter = o.WitnessAxis, o.WitnessParameter
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
	tubeOptions := func(axis int) SurfacePairTubeOptions {
		return SurfacePairTubeOptions{FreeAxis: axis, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	}
	result.Left, e = CertifySurfacePairTube(la, lc, tubeOptions(o.LeftFreeAxis))
	result.ExactOperations += result.Left.Test.ExactOperations
	if e != nil {
		return result, e
	}
	result.Right, e = CertifySurfacePairTube(ra, rc, tubeOptions(o.RightFreeAxis))
	result.ExactOperations += result.Right.Test.ExactOperations
	if e != nil {
		return result, e
	}
	ranges := func(x, y ParameterRectangle) [4][2]float64 {
		return [4][2]float64{{x.UMin, x.UMax}, {x.VMin, x.VMax}, {y.UMin, y.UMax}, {y.VMin, y.VMax}}
	}
	l, r := ranges(leftA, leftC), ranges(rightA, rightC)
	var common [4][2]float64
	degenerate := false
	for axis := 0; axis < 4; axis++ {
		common[axis] = [2]float64{max(l[axis][0], r[axis][0]), min(l[axis][1], r[axis][1])}
		if common[axis][0] > common[axis][1] {
			result.Excluded = true
			return result, nil
		}
		degenerate = degenerate || common[axis][0] == common[axis][1]
	}
	if degenerate {
		return result, nil
	}
	witnessRange := common[o.WitnessAxis]
	if o.WitnessParameter < witnessRange[0] || o.WitnessParameter > witnessRange[1] {
		return result, ErrDomain
	}
	if o.WitnessParameter == witnessRange[0] || o.WitnessParameter == witnessRange[1] {
		return result, nil
	}
	pa, e := a.Trim(ParameterRectangle{common[0][0], common[0][1], common[1][0], common[1][1]})
	if e != nil {
		return result, e
	}
	pc, e := c.Trim(ParameterRectangle{common[2][0], common[2][1], common[3][0], common[3][1]})
	if e != nil {
		return result, e
	}
	result.Witness, e = CertifySurfacePairSection(pa, pc, SurfacePairIntervalOptions{UseFixedParameter: true, FixedAxis: o.WitnessAxis, FixedParameter: o.WitnessParameter, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	result.ExactOperations += result.Witness.ExactOperations
	if e != nil {
		return result, e
	}
	result.Excluded = result.Witness.ExcludesContact
	result.SameLocalBranch = result.Left.ProvenBranch && result.Right.ProvenBranch && result.Witness.ProvenUnique
	return result, nil
}
