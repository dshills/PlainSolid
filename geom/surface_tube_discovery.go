package geom

import "math"

// SurfaceTubeDiscoveryOptions describes one fixed-section seed. DependentBox
// encloses the three unknown parameters and is reverified, never trusted.
// InitialFreeWidth=0 selects one eighth of the original free domain. Attempts
// halve free width and try three nested dependent boxes. Zero limits select
// 16 attempts, 2M cumulative proof/trim operations and 16384 coefficient bits.
type SurfaceTubeDiscoveryOptions struct {
	FreeAxis                                       int
	Parameter, InitialFreeWidth                    float64
	DependentBox                                   SurfacePairParameterBox
	MaxAttempts, MaxOperations, MaxCoefficientBits int
}

// SurfaceTubeDiscovery reports a locally discovered uniform branch tube.
// Found says neither that this tube is maximal nor that other branches are
// covered. Seed proves the initial fixed-section contact. Attempted counts
// uniform tube tests; failure retains seed diagnostics without a tube claim.
type SurfaceTubeDiscovery struct {
	Found                      bool
	Box                        SurfacePairParameterBox
	Tube                       SurfaceTubeCertificate
	Seed                       SurfaceSectionCertificate
	Attempted, ExactOperations int
}

// DiscoverSurfacePairTube revalidates a section, surrounds its root enclosure
// with positive dependent margins and searches dyadically shrinking free spans.
// It does not convert a numerical residual or unchecked seed into existence.
// The free value must lie strictly inside the original domain. Knot-containing
// candidate rectangles may fail with ErrDomain; trim to Bezier pieces first.
func DiscoverSurfacePairTube(a, b *SurfacePatch, o SurfaceTubeDiscoveryOptions) (result SurfaceTubeDiscovery, err error) {
	if a == nil || b == nil || len(a.net) == 0 || len(b.net) == 0 || o.FreeAxis < 0 || o.FreeAxis > 3 || !finite(o.Parameter) || !finite(o.InitialFreeWidth) || o.InitialFreeWidth < 0 || !o.DependentBox.A.Valid() || !o.DependentBox.B.Valid() {
		return result, ErrInvalid
	}
	if o.MaxAttempts == 0 {
		o.MaxAttempts = 16
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxAttempts < 1 || o.MaxAttempts > 256 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return result, ErrInvalid
	}
	domain := pairParameterRanges(SurfacePairParameterBox{a.Domain(), b.Domain()})
	seedRanges := pairParameterRanges(o.DependentBox)
	for axis, r := range seedRanges {
		if r[0] < domain[axis][0] || r[1] > domain[axis][1] {
			return result, ErrDomain
		}
	}
	free := o.FreeAxis
	freeMin, freeMax := domain[free][0], domain[free][1]
	if o.Parameter <= freeMin || o.Parameter >= freeMax || o.Parameter < seedRanges[free][0] || o.Parameter > seedRanges[free][1] {
		return result, ErrDomain
	}
	trim := func(box SurfacePairParameterBox) (*SurfacePatch, *SurfacePatch, error) {
		remaining := o.MaxOperations - result.ExactOperations
		if remaining < 1 {
			return nil, nil, ErrConvergence
		}
		budget := &polynomialBudget{maxOperations: remaining, maxBits: o.MaxCoefficientBits}
		pa, e := trimPatchBudget(a, box.A, budget)
		if e != nil {
			result.ExactOperations += budget.operations
			return nil, nil, e
		}
		pb, e := trimPatchBudget(b, box.B, budget)
		result.ExactOperations += budget.operations
		return pa, pb, e
	}
	pa, pb, e := trim(o.DependentBox)
	if e != nil {
		return result, e
	}
	remaining := o.MaxOperations - result.ExactOperations
	if remaining < 1 {
		return result, ErrConvergence
	}
	result.Seed, e = CertifySurfacePairSection(pa, pb, SurfacePairIntervalOptions{UseFixedParameter: true, FixedAxis: free, FixedParameter: o.Parameter, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	result.ExactOperations += result.Seed.ExactOperations
	if e != nil {
		return result, e
	}
	if !result.Seed.ProvenUnique {
		return result, ErrConvergence
	}
	e = searchSurfacePairTube(o, freeMin, freeMax, seedRanges, &result, trim)
	return result, e
}

func searchSurfacePairTube(o SurfaceTubeDiscoveryOptions, freeMin, freeMax float64,
	seedRanges [4][2]float64, result *SurfaceTubeDiscovery, trim func(SurfacePairParameterBox) (*SurfacePatch, *SurfacePatch, error)) error {
	roots := seedRanges
	for i, axis := range result.Seed.ActiveAxes {
		roots[axis] = [2]float64{max(seedRanges[axis][0], result.Seed.KrawczykBounds[i].Min), min(seedRanges[axis][1], result.Seed.KrawczykBounds[i].Max)}
	}
	width := o.InitialFreeWidth
	if width == 0 {
		width = (freeMax - freeMin) / 8
	}
	if !finite(width) || width <= 0 {
		return ErrInvalid
	}
	free := o.FreeAxis
	for attempt := 0; attempt < o.MaxAttempts; attempt++ {
		// Three dependent margin choices per free span avoid conflating too-wide
		// Jacobian bounds with too-tight root containment. All keep the proved root.
		level, choice := attempt/3, attempt%3
		half := math.Ldexp(width, -level-1)
		lo, hi := max(freeMin, o.Parameter-half), min(freeMax, o.Parameter+half)
		if lo >= o.Parameter || hi <= o.Parameter {
			return ErrConvergence
		}
		candidate := seedRanges
		candidate[free] = [2]float64{lo, hi}
		if choice != 0 {
			fraction := .5
			if choice == 2 {
				fraction = .25
			}
			for axis := 0; axis < 4; axis++ {
				if axis == free {
					continue
				}
				r, s := roots[axis], seedRanges[axis]
				candidate[axis] = [2]float64{max(s[0], r[0]-(r[0]-s[0])*fraction), min(s[1], r[1]+(s[1]-r[1])*fraction)}
				if candidate[axis][0] >= r[0] {
					candidate[axis][0] = max(s[0], math.Nextafter(r[0], math.Inf(-1)))
				}
				if candidate[axis][1] <= r[1] {
					candidate[axis][1] = min(s[1], math.Nextafter(r[1], math.Inf(1)))
				}
			}
		}
		box := rangesParameterPair(candidate)
		pa, pb, e := trim(box)
		if e != nil {
			return e
		}
		remaining := o.MaxOperations - result.ExactOperations
		if remaining < 1 {
			return ErrConvergence
		}
		tube, e := CertifySurfacePairTube(pa, pb, SurfacePairTubeOptions{FreeAxis: free, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		result.Attempted++
		result.ExactOperations += tube.Test.ExactOperations
		if e != nil {
			return e
		}
		if tube.ProvenBranch {
			result.Found, result.Box, result.Tube = true, box, tube
			return nil
		}
	}
	return ErrConvergence
}
