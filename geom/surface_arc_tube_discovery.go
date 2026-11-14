package geom

import (
	"context"
	"errors"
	"math/big"
)

// SurfaceArcTubeDiscoveryOptions bounds proposal and verification work. Zero
// limits select depth10, 256 intervals, 8 enclosure iterations, 8M operations
// and 16384 coefficient bits. Padding>0 is a base parameter-space margin; zero
// chooses margins from free-interval width and enclosure width. Three bounded
// margin scales are attempted before bisection.
type SurfaceArcTubeDiscoveryOptions struct {
	// AffinePlaneFreeAxes also proposes plane-chart free axes after an exact
	// axis-aligned affine projection identity. Default preserves the original axis.
	AffinePlaneFreeAxes                                                            bool
	Padding                                                                        float64
	MaxDepth, MaxIntervals, EnclosureIterations, MaxOperations, MaxCoefficientBits int
}

// SurfaceArcDiscoveredTube retains a private uniform proof, its original chart
// boxes and the free subinterval that generated it. Dependent boxes can extend
// beyond the source arc cell, so this is a valid branch proposal, not a claim
// that the entire tube image lies inside that cell.
type SurfaceArcDiscoveredTube struct {
	// ExactSourceSpan is populated by rational iso discovery; Span is then only an enclosure.
	ExactSourceSpan [2]*big.Rat
	// Boundary is an exact represented anchor proposal, independently re-proved on rebuild.
	Boundary *SurfaceTubeBoundaryOptions
	FreeAxis int
	Span     ParameterSpan
	Box      SurfacePairParameterBox
	Proof    VerifiedSurfaceTube
}

// SurfaceArcTubeDiscovery covers contacts of one certified plane arc over the
// requested represented free interval. Complete means all such contacts are
// contained in proved tube boxes or absent from B's domain by exact bounds.
// It is NOT global intersection/branch enumeration. Pending retains intervals
// whose coverage is unproved. OriginalA/B and ArcDomain report the checked
// chart rectangles; changing caller-owned diagnostics does not alter proofs.
type SurfaceArcTubeDiscovery struct {
	Tubes                                       []SurfaceArcDiscoveredTube
	Pending, Excluded                           []ParameterSpan
	OriginalA, OriginalB, ArcDomain             ParameterRectangle
	Complete                                    bool
	VisitedIntervals, Attempts, ExactOperations int
}

// DiscoverSurfacePlaneArcTubes verifies arc.source matches originalA exactly and
// originalB is an affine patch on exactly the arc's plane. A bounded adaptive
// search projects conservative arc enclosures into B's exact affine chart and
// proposes padded uniform tubes. All proposals receive independent Krawczyk
// proofs. No approximate plane equality, roots or tube proofs are trusted.
func DiscoverSurfacePlaneArcTubes(ctx context.Context, arc SurfacePlaneArc, a, b *SurfacePatch, start, end float64, o SurfaceArcTubeDiscoveryOptions) (r SurfaceArcTubeDiscovery, err error) {
	if ctx == nil || arc.source == nil || a == nil || b == nil || len(a.net) == 0 || len(b.net) == 0 || !finite(start) || !finite(end) || start >= end {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 10
	}
	if o.MaxIntervals == 0 {
		o.MaxIntervals = 256
	}
	if o.EnclosureIterations == 0 {
		o.EnclosureIterations = 8
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if !finite(o.Padding) || o.Padding < 0 || o.MaxDepth < 1 || o.MaxDepth > 128 || o.MaxIntervals < 1 || o.EnclosureIterations < 1 || o.EnclosureIterations > 256 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	r.OriginalA, r.OriginalB, r.ArcDomain = a.Domain(), b.Domain(), arc.source.Domain()
	fixed := arc.Certificate.FixedParameterAxis
	if fixed < 0 || fixed > 1 {
		return r, ErrInvalid
	}
	free := 1 - fixed
	freeMin, freeMax := r.ArcDomain.UMin, r.ArcDomain.UMax
	if free == 1 {
		freeMin, freeMax = r.ArcDomain.VMin, r.ArcDomain.VMax
	}
	if start < freeMin || end > freeMax {
		return r, ErrDomain
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		r.ExactOperations = budget.operations
		if err != nil && !r.Complete && len(r.Pending) == 0 {
			r.Pending = append(r.Pending, ParameterSpan{Start: start, End: end})
		}
	}()
	restricted, e := trimPatchBudget(a, r.ArcDomain, budget)
	if e != nil {
		return r, e
	}
	remaining := func() (int, error) {
		left := o.MaxOperations - budget.operations
		if left < 1 {
			return 0, ErrConvergence
		}
		return left, nil
	}
	left, e := remaining()
	if e != nil {
		return r, e
	}
	identity, e := CompareExactSurfaces(ctx, restricted, arc.source, ExactSurfaceComparisonOptions{MaxCells: o.MaxIntervals, MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
	budget.operations += identity.ExactOperations
	if e != nil {
		return r, e
	}
	if !identity.Complete || !identity.Equal {
		return r, ErrInvalid
	}
	left, e = remaining()
	if e != nil {
		return r, e
	}
	chart, e := NewAffineSurfaceChart(ctx, b, AffineSurfaceChartOptions{MaxCells: o.MaxIntervals, MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return r, e
	}
	budget.operations += chart.operations
	// Compare the four exact homogeneous plane coefficients, including offset.
	planeA := exactPlaneCoefficients(arc.plane)
	var planeB [4]*big.Rat
	planeB[3] = new(big.Rat)
	for axis := 0; axis < 3; axis++ {
		j, k := (axis+1)%3, (axis+2)%3
		planeB[axis] = new(big.Rat).Sub(new(big.Rat).Mul(chart.u[j], chart.v[k]), new(big.Rat).Mul(chart.u[k], chart.v[j]))
		planeB[3].Sub(planeB[3], new(big.Rat).Mul(planeB[axis], chart.origin[axis]))
	}
	pivot := -1
	for axis := 0; axis < 3; axis++ {
		if planeA[axis].Sign() != 0 {
			pivot = axis
			break
		}
	}
	if pivot < 0 {
		return r, ErrInvalid
	}
	for axis := 0; axis < 4; axis++ {
		difference := new(big.Rat).Sub(new(big.Rat).Mul(planeA[axis], planeB[pivot]), new(big.Rat).Mul(planeB[axis], planeA[pivot]))
		if e := budget.check(difference); e != nil {
			return r, e
		}
		if difference.Sign() != 0 {
			return r, ErrInvalid
		}
	}
	if planeB[pivot].Sign() == 0 {
		return r, ErrInvalid
	}
	var projection *SurfaceAffineProjection
	if o.AffinePlaneFreeAxes {
		left, e := remaining()
		if e != nil {
			return r, e
		}
		mapping, e := ClipSurfaceAffineProjection(ctx, a, chart, SurfacePatchOptions{MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
		budget.operations += mapping.ExactOperations
		if e != nil {
			return r, e
		}
		projection = &mapping
	}
	type node struct {
		lo, hi float64
		depth  int
	}
	stack := []node{{start, end, 0}}
	pending := func(n node) {
		r.Pending = append(r.Pending, ParameterSpan{Start: n.lo, End: n.hi})
		for _, x := range stack {
			r.Pending = append(r.Pending, ParameterSpan{Start: x.lo, End: x.hi})
		}
	}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if e := ctx.Err(); e != nil {
			pending(n)
			return r, e
		}
		if r.VisitedIntervals >= o.MaxIntervals {
			pending(n)
			return r, ErrConvergence
		}
		r.VisitedIntervals++
		left, e := remaining()
		if e != nil {
			pending(n)
			return r, e
		}
		enclosure, e := arc.EncloseInterval(ctx, n.lo, n.hi, SurfaceArcIntervalOptions{MaxIterations: o.EnclosureIterations, MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
		budget.operations += enclosure.ExactOperations
		if e != nil && !errors.Is(e, ErrConvergence) {
			pending(n)
			return r, e
		}
		if enclosure.Excluded {
			r.Excluded = append(r.Excluded, ParameterSpan{Start: n.lo, End: n.hi})
			continue
		}
		success := false
		if enclosure.HasBounds {
			// Coordinate-axis projection inverse is affine even for off-plane box
			// corners; its hull encloses every exact on-plane point in this model box.
			var uvMin, uvMax [2]*big.Rat
			for corner := 0; corner < 8; corner++ {
				p := enclosure.Bounds.Min
				if corner&1 != 0 {
					p.X = enclosure.Bounds.Max.X
				}
				if corner&2 != 0 {
					p.Y = enclosure.Bounds.Max.Y
				}
				if corner&4 != 0 {
					p.Z = enclosure.Bounds.Max.Z
				}
				if !p.IsFinite() {
					pending(n)
					return r, ErrConvergence
				}
				left, e := remaining()
				if e != nil {
					pending(n)
					return r, e
				}
				localChart := *chart
				localChart.options.MaxOperations = left
				inverse, e := localChart.Invert(p)
				budget.operations += inverse.ExactOperations
				if e != nil {
					pending(n)
					return r, e
				}
				for axis, value := range []*big.Rat{inverse.U, inverse.V} {
					if uvMin[axis] == nil || value.Cmp(uvMin[axis]) < 0 {
						uvMin[axis] = new(big.Rat).Set(value)
					}
					if uvMax[axis] == nil || value.Cmp(uvMax[axis]) > 0 {
						uvMax[axis] = new(big.Rat).Set(value)
					}
				}
			}
			domainB := r.OriginalB
			if uvMax[0].Cmp(new(big.Rat).SetFloat64(domainB.UMin)) < 0 || uvMin[0].Cmp(new(big.Rat).SetFloat64(domainB.UMax)) > 0 || uvMax[1].Cmp(new(big.Rat).SetFloat64(domainB.VMin)) < 0 || uvMin[1].Cmp(new(big.Rat).SetFloat64(domainB.VMax)) > 0 {
				r.Excluded = append(r.Excluded, ParameterSpan{Start: n.lo, End: n.hi})
				continue
			}
			base := [4][2]float64{{enclosure.Parameters.Min.X, enclosure.Parameters.Max.X}, {enclosure.Parameters.Min.Y, enclosure.Parameters.Max.Y}, {rationalFloatBound(uvMin[0], true), rationalFloatBound(uvMax[0], false)}, {rationalFloatBound(uvMin[1], true), rationalFloatBound(uvMax[1], false)}}
			domains := pairParameterRanges(SurfacePairParameterBox{a.Domain(), b.Domain()})
			base[free] = [2]float64{n.lo, n.hi}
			freeCandidates := []int{free}
			if projection != nil {
				for axis, source := range projection.Axis {
					if source == free {
						freeCandidates = append(freeCandidates, axis+2)
					}
				}
			}
			for _, candidateFree := range freeCandidates {
				candidateBase := base
				if candidateFree >= 2 {
					axis := candidateFree - 2
					lo := new(big.Rat).Add(projection.Offset[axis], new(big.Rat).Mul(projection.Scale[axis], new(big.Rat).SetFloat64(n.lo)))
					hi := new(big.Rat).Add(projection.Offset[axis], new(big.Rat).Mul(projection.Scale[axis], new(big.Rat).SetFloat64(n.hi)))
					if lo.Cmp(hi) > 0 {
						lo, hi = hi, lo
					}
					l, e := exactRepresentableFloat(lo)
					if e != nil {
						continue
					}
					h, e := exactRepresentableFloat(hi)
					if e != nil {
						continue
					}
					candidateBase[candidateFree] = [2]float64{l, h}
				}
				for _, scale := range []float64{1, .5, 2} {
					if e := ctx.Err(); e != nil {
						pending(n)
						return r, e
					}
					proposal := candidateBase
					for axis := 0; axis < 4; axis++ {
						if axis == candidateFree {
							continue
						}
						padding := o.Padding
						if padding == 0 {
							padding = min((domains[axis][1]-domains[axis][0])/16, max((base[axis][1]-base[axis][0])/4, (n.hi-n.lo)/4))
						}
						proposal[axis] = [2]float64{max(domains[axis][0], base[axis][0]-scale*padding), min(domains[axis][1], base[axis][1]+scale*padding)}
					}
					box := rangesParameterPair(proposal)
					if !box.A.Valid() || !box.B.Valid() {
						continue
					}
					pa, e := trimPatchBudget(a, box.A, budget)
					if e != nil {
						pending(n)
						return r, e
					}
					pb, e := trimPatchBudget(b, box.B, budget)
					if e != nil {
						pending(n)
						return r, e
					}
					left, e := remaining()
					if e != nil {
						pending(n)
						return r, e
					}
					r.Attempts++
					proof, e := CertifySurfacePairTube(pa, pb, SurfacePairTubeOptions{FreeAxis: candidateFree, MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
					budget.operations += proof.Test.ExactOperations
					if errors.Is(e, ErrDomain) || errors.Is(e, ErrDegenerate) {
						continue
					}
					if e != nil {
						pending(n)
						return r, e
					}
					if proof.ProvenBranch {
						r.Tubes = append(r.Tubes, SurfaceArcDiscoveredTube{FreeAxis: candidateFree, Span: ParameterSpan{Start: n.lo, End: n.hi, Index: len(r.Tubes)}, Box: box, Proof: VerifiedSurfaceTube{a: pa, b: pb, axis: candidateFree, tangentSign: proof.FreeTangentSign}})
						success = true
						break
					}
				}
				if success {
					break
				}
			}
		}
		if success {
			continue
		}
		mid := n.lo/2 + n.hi/2
		if n.depth >= o.MaxDepth || mid <= n.lo || mid >= n.hi {
			r.Pending = append(r.Pending, ParameterSpan{Start: n.lo, End: n.hi})
			continue
		}
		stack = append(stack, node{mid, n.hi, n.depth + 1}, node{n.lo, mid, n.depth + 1})
	}
	if len(r.Pending) > 0 {
		return r, ErrConvergence
	}
	r.Complete = true
	return r, nil
}
