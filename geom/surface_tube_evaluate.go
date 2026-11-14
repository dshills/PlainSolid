package geom

import "math"

// SurfaceTubeEvaluationOptions chooses a section and bounds enclosure refinement.
// Parameter is the exact binary rational value of the supplied float. Zero
// tolerances select 1e-8 in UV and space, MaxIterations=24. Proof limits apply
// separately to the tube and each section test; patch trimming retains its own
// limits. The entire tube is revalidated, so mutable public certificates are
// never trusted as provenance.
type SurfaceTubeEvaluationOptions struct {
	FreeAxis                                         int
	Parameter, ParameterTolerance, BoxTolerance      float64
	MaxIterations, MaxOperations, MaxCoefficientBits int
}

// SurfaceTubeEvaluation encloses the unique common point in both parameter
// charts. Bounds contains the exact shared spatial point; Point is a convenient
// representative and need not lie exactly on either surface. Residual encloses
// S-T over the returned chart boxes. Complete means both requested UV and spatial
// widths are met. Enclosed means these boxes contain the root, even if a
// resource failure interrupted refinement before the requested widths.
type SurfaceTubeEvaluation struct {
	ParametersA, ParametersB    UVBox
	Bounds                      Box
	Point                       Vec3
	Residual                    [3]CertifiedScalarRange
	Complete, Enclosed          bool
	Iterations, ExactOperations int
}

// EvaluateSurfacePairTube revalidates a uniform tube, then repeatedly contracts
// its dependent section box using exact Krawczyk images. The selected parameter
// remains fixed exactly; it is never widened into a thin free-coordinate slice.
// A successful tube proof guarantees existence throughout refinement. Every
// retained dependent box contains that root, even if later inclusion tests are
// inconclusive. Bounds come from exact homogeneous hulls and outward rounding.
func EvaluateSurfacePairTube(a, c *SurfacePatch, o SurfaceTubeEvaluationOptions) (result SurfaceTubeEvaluation, err error) {
	return evaluateSurfacePairTube(a, c, o, false)
}

// VerifiedSurfaceTube retains an immutable branch proof and its original
// bounded patch pair. Repeated section evaluation does not re-prove the
// uniform tube. No caller-owned certificate is used as proof provenance.
type VerifiedSurfaceTube struct {
	a, b              *SurfacePatch
	axis, tangentSign int
	anchor            *surfaceTubeBoundaryAnchor
}

// NewVerifiedSurfaceTube proves a regular uniform branch in the two boxes.
func NewVerifiedSurfaceTube(a, b *SurfacePatch, o SurfacePairTubeOptions) (VerifiedSurfaceTube, error) {
	tube, _, e := NewVerifiedSurfaceTubeWithCertificate(a, b, o)
	return tube, e
}

// FreeTangentSign is the certified canonical tangent sign in the tube's
// free coordinate. The zero value of VerifiedSurfaceTube has no proof.
func (t VerifiedSurfaceTube) FreeTangentSign() int { return t.tangentSign }

// Evaluate isolates one exact root enclosure of the verified branch.
// FreeAxis in o is ignored; the original proof fixes that coordinate.
func (t VerifiedSurfaceTube) Evaluate(o SurfaceTubeEvaluationOptions) (SurfaceTubeEvaluation, error) {
	o.FreeAxis = t.axis
	if t.anchor != nil && o.Parameter == t.anchor.free {
		return t.anchor.evaluation(o)
	}
	return evaluateSurfacePairTube(t.a, t.b, o, true)
}

func evaluateSurfacePairTube(a, c *SurfacePatch, o SurfaceTubeEvaluationOptions, verified bool) (result SurfaceTubeEvaluation, err error) {
	if a == nil || c == nil || o.FreeAxis < 0 || o.FreeAxis > 3 || !finite(o.Parameter) {
		return result, ErrInvalid
	}
	if o.ParameterTolerance == 0 {
		o.ParameterTolerance = 1e-8
	}
	if o.BoxTolerance == 0 {
		o.BoxTolerance = 1e-8
	}
	if o.MaxIterations == 0 {
		o.MaxIterations = 24
	}
	if !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 || !finite(o.BoxTolerance) || o.BoxTolerance <= 0 || o.MaxIterations < 1 || o.MaxIterations > 256 {
		return result, ErrInvalid
	}
	if !verified {
		tube, e := CertifySurfacePairTube(a, c, SurfacePairTubeOptions{FreeAxis: o.FreeAxis, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
		result.ExactOperations += tube.Test.ExactOperations
		if e != nil {
			return result, e
		}
		if !tube.ProvenBranch {
			return result, ErrConvergence
		}
	}
	da, dc := a.Domain(), c.Domain()
	ranges := [4][2]float64{{da.UMin, da.UMax}, {da.VMin, da.VMax}, {dc.UMin, dc.UMax}, {dc.VMin, dc.VMax}}
	if o.Parameter < ranges[o.FreeAxis][0] || o.Parameter > ranges[o.FreeAxis][1] {
		return result, ErrDomain
	}
	current := ranges
	for iteration := 0; iteration < o.MaxIterations; iteration++ {
		result.Iterations = iteration + 1
		pa, e := a.Trim(ParameterRectangle{current[0][0], current[0][1], current[1][0], current[1][1]})
		if e != nil {
			return result, e
		}
		pc, e := c.Trim(ParameterRectangle{current[2][0], current[2][1], current[3][0], current[3][1]})
		if e != nil {
			return result, e
		}
		section, e := CertifySurfacePairSection(pa, pc, SurfacePairIntervalOptions{UseFixedParameter: true, FixedAxis: o.FreeAxis, FixedParameter: o.Parameter, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
		result.ExactOperations += section.ExactOperations
		if e != nil {
			return result, e
		}
		if !section.CenterInvertible || section.ExcludesContact {
			return result, ErrConvergence
		}
		next := current
		changed := false
		for j, axis := range section.ActiveAxes {
			bound := section.KrawczykBounds[j]
			lo, hi := max(current[axis][0], bound.Min), min(current[axis][1], bound.Max)
			if lo > hi {
				return result, ErrConvergence
			}
			if lo == hi {
				// Patch domains need positive width. An adjacent representable enclosure
				// keeps the exact point while permitting another certified section test.
				lo = max(current[axis][0], math.Nextafter(lo, math.Inf(-1)))
				hi = min(current[axis][1], math.Nextafter(hi, math.Inf(1)))
				if lo == hi {
					return result, ErrConvergence
				}
			}
			next[axis] = [2]float64{lo, hi}
			changed = changed || next[axis] != current[axis]
		}
		current = next
		pa, e = a.Trim(ParameterRectangle{current[0][0], current[0][1], current[1][0], current[1][1]})
		if e != nil {
			return result, e
		}
		pc, e = c.Trim(ParameterRectangle{current[2][0], current[2][1], current[3][0], current[3][1]})
		if e != nil {
			return result, e
		}
		budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
		if budget.maxOperations == 0 {
			budget.maxOperations = 1000000
		}
		if budget.maxBits == 0 {
			budget.maxBits = 16384
		}
		aa, ca := -1, -1
		if o.FreeAxis < 2 {
			aa = o.FreeAxis
		} else {
			ca = o.FreeAxis - 2
		}
		ap, _, e := surfaceDifferentialIntervals(pa, aa, o.Parameter, budget)
		if e != nil {
			result.ExactOperations += budget.operations
			return result, e
		}
		cp, _, e := surfaceDifferentialIntervals(pc, ca, o.Parameter, budget)
		if e != nil {
			result.ExactOperations += budget.operations
			return result, e
		}
		var bounds Box
		for axis := 0; axis < 3; axis++ {
			lo := max(rationalFloatBound(ap[axis].lo, true), rationalFloatBound(cp[axis].lo, true), coordinate(pa.bounds.Min, axis), coordinate(pc.bounds.Min, axis))
			hi := min(rationalFloatBound(ap[axis].hi, false), rationalFloatBound(cp[axis].hi, false), coordinate(pa.bounds.Max, axis), coordinate(pc.bounds.Max, axis))
			if lo > hi {
				return result, ErrConvergence
			}
			switch axis {
			case 0:
				bounds.Min.X, bounds.Max.X = lo, hi
			case 1:
				bounds.Min.Y, bounds.Max.Y = lo, hi
			case 2:
				bounds.Min.Z, bounds.Max.Z = lo, hi
			}
			r, e := intervalSubtract(ap[axis], cp[axis], budget)
			if e != nil {
				result.ExactOperations += budget.operations
				return result, e
			}
			result.Residual[axis] = certifiedRange(r)
		}
		result.ExactOperations += budget.operations
		uv := current
		uv[o.FreeAxis] = [2]float64{o.Parameter, o.Parameter}
		result.ParametersA = UVBox{Vec2{uv[0][0], uv[1][0]}, Vec2{uv[0][1], uv[1][1]}}
		result.ParametersB = UVBox{Vec2{uv[2][0], uv[3][0]}, Vec2{uv[2][1], uv[3][1]}}
		result.Bounds = bounds
		result.Point = boxRepresentative(bounds)
		result.Enclosed = true
		small := true
		for axis, r := range uv {
			if axis != o.FreeAxis && r[1]-r[0] > o.ParameterTolerance {
				small = false
			}
		}
		size := bounds.Size()
		if small && size.X <= o.BoxTolerance && size.Y <= o.BoxTolerance && size.Z <= o.BoxTolerance {
			result.Complete = true
			return result, nil
		}
		if !changed {
			return result, ErrConvergence
		}
	}
	return result, ErrConvergence
}
