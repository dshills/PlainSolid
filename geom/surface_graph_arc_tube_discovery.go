package geom

import (
	"context"
	"errors"
	"math/big"
)

// DiscoverCommonXYGraphArcTubes turns one certified zero-height-difference arc
// into independently verified original-support tube seeds. The exact common-XY
// identity and source-arc identity are rechecked; no approximate pole differences
// or UV correspondences are trusted. UV equality supplies paired contacts even
// when support degrees/weights differ. Complete covers this requested arc slab,
// not the entire surface pair: callers must separately prove pair coverage and
// branch endpoint completeness. Unsupported original knot-crossing tube boxes
// and simultaneous outer-boundary ends remain Pending, never silently clipped.
func DiscoverCommonXYGraphArcTubes(ctx context.Context, arc SurfacePlaneArc, a, c *SurfacePatch, start, end float64, o SurfaceArcTubeDiscoveryOptions) (r SurfaceArcTubeDiscovery, err error) {
	if ctx == nil || a == nil || c == nil || arc.source == nil || len(a.net) == 0 || len(c.net) == 0 || !finite(start) || !finite(end) || start >= end {
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
	fixed := arc.Certificate.FixedParameterAxis
	if fixed < 0 || fixed > 1 {
		return r, ErrInvalid
	}
	free := 1 - fixed
	r.OriginalA, r.OriginalB, r.ArcDomain = a.Domain(), c.Domain(), arc.source.Domain()
	ranges := [2][2]float64{{r.ArcDomain.UMin, r.ArcDomain.UMax}, {r.ArcDomain.VMin, r.ArcDomain.VMax}}
	if start < ranges[free][0] || end > ranges[free][1] {
		return r, ErrDomain
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		r.ExactOperations = b.operations
		if err != nil && !r.Complete && len(r.Pending) == 0 {
			r.Pending = append(r.Pending, ParameterSpan{Start: start, End: end})
		}
	}()
	plane := exactPlaneCoefficients(arc.plane)
	if plane[0].Sign() != 0 || plane[1].Sign() != 0 || plane[2].Sign() == 0 || plane[3].Sign() != 0 {
		return r, ErrInvalid
	}
	difference, e := orderingDifference(ctx, a, c, o.MaxIntervals, b)
	if e != nil {
		return r, e
	}
	restricted, e := trimPatchBudget(difference, r.ArcDomain, b)
	if e != nil {
		return r, e
	}
	left := o.MaxOperations - b.operations
	if left < 1 {
		return r, ErrConvergence
	}
	identity, e := CompareExactSurfaces(ctx, restricted, arc.source, ExactSurfaceComparisonOptions{MaxCells: o.MaxIntervals, MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
	b.operations += identity.ExactOperations
	if e != nil {
		return r, e
	}
	if !identity.Complete || !identity.Equal {
		return r, ErrInvalid
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
	domains := pairParameterRanges(SurfacePairParameterBox{a.Domain(), c.Domain()})
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
		left = o.MaxOperations - b.operations
		if left < 1 {
			pending(n)
			return r, ErrConvergence
		}
		enclosure, e := arc.EncloseInterval(ctx, n.lo, n.hi, SurfaceArcIntervalOptions{MaxIterations: o.EnclosureIterations, MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
		b.operations += enclosure.ExactOperations
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
			base := [4][2]float64{{enclosure.Parameters.Min.X, enclosure.Parameters.Max.X}, {enclosure.Parameters.Min.Y, enclosure.Parameters.Max.Y}, {enclosure.Parameters.Min.X, enclosure.Parameters.Max.X}, {enclosure.Parameters.Min.Y, enclosure.Parameters.Max.Y}}
			for _, candidateFree := range []int{free, free + 2} {
				for _, scale := range []float64{1, .5, 2} {
					if e := ctx.Err(); e != nil {
						pending(n)
						return r, e
					}
					proposal := base
					proposal[candidateFree] = [2]float64{n.lo, n.hi}
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
					// Restrict padded proposals to the original knot cells containing
					// this exact trace cell, never a polynomial continuation across a crease.
					for owner, source := range []*SurfacePatch{a, c} {
						for axis, knots := range [][]float64{source.knotsU, source.knotsV} {
							center := (r.ArcDomain.UMin + r.ArcDomain.UMax) / 2
							if axis == 1 {
								center = (r.ArcDomain.VMin + r.ArcDomain.VMax) / 2
							}
							for k := 0; k+1 < len(knots); k++ {
								if knots[k] < center && center < knots[k+1] {
									index := 2*owner + axis
									proposal[index][0] = max(proposal[index][0], knots[k])
									proposal[index][1] = min(proposal[index][1], knots[k+1])
									break
								}
							}
						}
					}
					if proposal[candidateFree][0] != n.lo || proposal[candidateFree][1] != n.hi {
						continue
					}
					box := rangesParameterPair(proposal)
					if !box.A.Valid() || !box.B.Valid() {
						continue
					}
					pa, e := trimPatchBudget(a, box.A, b)
					if e != nil {
						pending(n)
						return r, e
					}
					pc, e := trimPatchBudget(c, box.B, b)
					if e != nil {
						pending(n)
						return r, e
					}
					left = o.MaxOperations - b.operations
					if left < 1 {
						pending(n)
						return r, ErrConvergence
					}
					r.Attempts++
					proof, e := CertifySurfacePairTube(pa, pc, SurfacePairTubeOptions{FreeAxis: candidateFree, MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
					b.operations += proof.Test.ExactOperations
					if errors.Is(e, ErrDomain) || errors.Is(e, ErrDegenerate) {
						continue
					}
					if e != nil {
						pending(n)
						return r, e
					}
					var boundary *SurfaceTubeBoundaryOptions
					var anchored VerifiedSurfaceTube
					if !proof.ProvenBranch {
						for endpoint := 0; endpoint < 2; endpoint++ {
							left = o.MaxOperations - b.operations
							if left < 1 {
								return r, ErrConvergence
							}
							exact, ee := arc.ExactEndpoint(endpoint, ArcExactEndpointOptions{MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
							b.operations += exact.ExactOperations
							if ee != nil {
								continue
							}
							anchor := [4]*big.Rat{exact.Parameters[0], exact.Parameters[1], exact.Parameters[0], exact.Parameters[1]}
							value, ee := RepresentedExactScalar(anchor[candidateFree])
							if ee != nil {
								continue
							}
							atUpper := value == proposal[candidateFree][1]
							if !atUpper && value != proposal[candidateFree][0] {
								continue
							}
							represented := true
							for _, q := range anchor {
								if _, ee := RepresentedExactScalar(q); ee != nil {
									represented = false
								}
							}
							if !represented {
								continue
							}
							left = o.MaxOperations - b.operations
							if left < 1 {
								return r, ErrConvergence
							}
							option := SurfaceTubeBoundaryOptions{SurfacePairTubeOptions: SurfacePairTubeOptions{FreeAxis: candidateFree, MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits}, AtUpper: atUpper, Anchor: anchor}
							used := 0
							anchored, ee = newBoundaryAnchoredSurfaceTube(ctx, pa, pc, option, &used)
							b.operations += used
							r.Attempts++
							if ee == nil {
								boundary = &option
								break
							}
						}
					}
					if proof.ProvenBranch || boundary != nil {
						r.Tubes = append(r.Tubes, SurfaceArcDiscoveredTube{FreeAxis: candidateFree, Span: ParameterSpan{Start: n.lo, End: n.hi, Index: len(r.Tubes)}, Box: box, Proof: VerifiedSurfaceTube{a: pa, b: pc, axis: candidateFree, tangentSign: proof.FreeTangentSign}})
						if boundary != nil {
							last := len(r.Tubes) - 1
							r.Tubes[last].Proof = anchored
							r.Tubes[last].Boundary = boundary
						}
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
	r.Complete = len(r.Pending) == 0
	if !r.Complete {
		return r, ErrConvergence
	}
	return r, nil
}
