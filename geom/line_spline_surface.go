package geom

import (
	"context"
	"math/big"
)

// LineSplineSurfaceOptions bounds line/surface contact isolation. MinParameter
// and MaxParameter are required finite signed line distances. Zero work limits
// select depth 16, 4096 cells, 4M exact operations and 16384 coefficient bits.
// The patch's own temporary-pole limit applies to subdivision.
type LineSplineSurfaceOptions struct {
	MinParameter, MaxParameter                            float64
	MaxDepth, MaxCells, MaxOperations, MaxCoefficientBits int
}

// LineSplineSurfaceHit certifies exactly one regular contact inside Cell and
// LineRange. UV and Parameter bracket that root using an exact Krawczyk image.
// Bounds is a spatial enclosure, not a point-on-surface approximation.
type LineSplineSurfaceHit struct {
	Cell                       ParameterRectangle
	LineRange                  ParameterSpan
	UV                         UVBox
	ParameterMin, ParameterMax float64
	Bounds                     Box
}

// LineSplineSurfacePending retains contact that is not classified. Reason does
// not assert tangency or coincidence; either can cause a singular section test.
type LineSplineSurfacePending struct {
	Cell      ParameterRectangle
	LineRange ParameterSpan
	Depth     int
	Reason    string
}

// LineSplineSurfaceResult is exhaustive only when Complete. Tangent, coincident,
// boundary-only and unresolved contacts remain in Pending; they are never
// discarded by a lack of sign changes. Isolated hits have disjoint cell interiors.
type LineSplineSurfaceResult struct {
	Hits                          []LineSplineSurfaceHit
	Pending                       []LineSplineSurfacePending
	VisitedCells, ExactOperations int
	Complete                      bool
}

func validRationalLineDirection(d [3]*big.Rat) bool {
	return d[0] != nil && d[1] != nil && d[2] != nil && (d[0].Sign() != 0 || d[1].Sign() != 0 || d[2].Sign() != 0)
}

func exactLineSectionPatch(origin, direction [3]*big.Rat, lo, hi float64, options SurfacePatchOptions, b *polynomialBudget) (*SurfacePatch, error) {
	axis := 0
	for k := range direction {
		if e := b.check(origin[k]); e != nil {
			return nil, e
		}
		if e := b.check(direction[k]); e != nil {
			return nil, e
		}
		if new(big.Rat).Abs(direction[k]).Cmp(new(big.Rat).Abs(direction[axis])) < 0 {
			axis = k
		}
	}
	p := &SurfacePatch{degreeU: 1, degreeV: 1, countU: 2, countV: 2, knotsU: []float64{-1, -1, 1, 1}, knotsV: []float64{lo, lo, hi, hi}, options: options, net: make([]exactHomogeneous, 4)}
	for i, s := range []int64{-1, 1} {
		for j, t := range []float64{lo, hi} {
			var h exactHomogeneous
			h[3] = big.NewRat(1, 1)
			for k := 0; k < 3; k++ {
				h[k] = new(big.Rat).Add(origin[k], new(big.Rat).Mul(new(big.Rat).SetFloat64(t), direction[k]))
				if k == axis {
					h[k].Add(h[k], big.NewRat(s, 1))
				}
				if e := b.check(h[k]); e != nil {
					return nil, e
				}
			}
			p.net[i*2+j] = h
		}
	}
	if e := p.computeBounds(b); e != nil {
		return nil, e
	}
	return p, nil
}

// IntersectLineSplineSurface isolates bounded straight-line contacts using an
// exact affine ruled patch whose fixed section is precisely origin+t*direction.
// No rounded endpoint chord substitutes for the original line. Exact residual
// hulls exclude cells; strict Krawczyk inclusion plus contraction proves regular
// isolated hits. Root existence is not inferred from hull overlap. Contacts on
// Original surface-edge roots are proved by exact homogeneous polynomial GCD
// when a strict contraction certificate proves cell uniqueness. Other
// internal split boundaries are retried in overlapping neighborhoods with exact
// uniqueness-based deduplication. Original domain boundaries may remain unresolved, as do tangencies and
// coincident curves. ErrConvergence preserves every remaining parameter box.
func IntersectLineSplineSurface(line Line, patch *SurfacePatch, o LineSplineSurfaceOptions) (result LineSplineSurfaceResult, err error) {
	return IntersectLineSplineSurfaceContext(context.Background(), line, patch, o)
}

// IntersectLineSplineSurfaceContext cooperatively cancels between exact
// contact cells and retains all unprocessed candidates in Pending.
func IntersectLineSplineSurfaceContext(ctx context.Context, line Line, patch *SurfacePatch, o LineSplineSurfaceOptions) (result LineSplineSurfaceResult, err error) {
	if !line.valid() {
		return result, ErrInvalid
	}
	return IntersectRationalLineSplineSurface(ctx, rationalLineVec3(line.origin), rationalLineVec3(line.direction), patch, o)
}

// IntersectRationalLineSplineSurface isolates contacts of the exact line
// origin+t*direction. The supplied direction defines the parameter scale.
// Contact enclosures use outward float64 bounds and unresolved cells are retained.
func IntersectRationalLineSplineSurface(ctx context.Context, origin, direction [3]*big.Rat, patch *SurfacePatch, o LineSplineSurfaceOptions) (result LineSplineSurfaceResult, err error) {
	if ctx == nil || origin[0] == nil || origin[1] == nil || origin[2] == nil || !validRationalLineDirection(direction) ||
		patch == nil || len(patch.net) == 0 || !finite(o.MinParameter) || !finite(o.MaxParameter) || o.MinParameter >= o.MaxParameter || !finite(o.MaxParameter-o.MinParameter) {
		return result, ErrInvalid
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 16
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 4000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxDepth < 1 || o.MaxDepth > 128 || o.MaxCells < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return result, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		result.Pending = append(result.Pending, LineSplineSurfacePending{patch.Domain(), ParameterSpan{Start: o.MinParameter, End: o.MaxParameter}, 0, "canceled"})
		return result, e
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	linePatch, e := exactLineSectionPatch(origin, direction, o.MinParameter, o.MaxParameter, patch.options, budget)
	result.ExactOperations = budget.operations
	if e != nil {
		result.Pending = append(result.Pending, LineSplineSurfacePending{patch.Domain(), ParameterSpan{Start: o.MinParameter, End: o.MaxParameter}, 0, "line section construction limit"})
		return result, e
	}
	type node struct {
		p, l  *SurfacePatch
		depth int
	}
	stack := []node{{patch, linePatch, 0}}
	pending := func(n node, reason string) {
		d := n.l.Domain()
		result.Pending = append(result.Pending, LineSplineSurfacePending{n.p.Domain(), ParameterSpan{Start: d.VMin, End: d.VMax}, n.depth, reason})
	}
	fail := func(n *node) {
		if n != nil {
			pending(*n, "work limit")
		}
		for _, x := range stack {
			pending(x, "work limit")
		}
	}
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			for _, n := range stack {
				pending(n, "canceled")
			}
			return result, e
		}
		if result.VisitedCells >= o.MaxCells || result.ExactOperations >= o.MaxOperations {
			fail(nil)
			return result, ErrConvergence
		}
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		result.VisitedCells++
		uk, hu := internalPatchKnot(n.p.knotsU, n.p.degreeU, n.p.countU)
		vk, hv := internalPatchKnot(n.p.knotsV, n.p.degreeV, n.p.countV)
		if !hu && !hv {
			opts := SurfacePairIntervalOptions{UseFixedParameter: true, FixedAxis: 2, FixedParameter: 0, MaxOperations: o.MaxOperations - result.ExactOperations, MaxCoefficientBits: o.MaxCoefficientBits}
			bounds, e := BoundSurfacePairDifferentials(n.p, n.l, opts)
			result.ExactOperations += bounds.ExactOperations
			if e != nil {
				fail(&n)
				return result, e
			}
			if bounds.ExcludesContact {
				continue
			}
			if result.ExactOperations >= o.MaxOperations {
				fail(&n)
				return result, ErrConvergence
			}
			opts.MaxOperations = o.MaxOperations - result.ExactOperations
			certificate, e := CertifySurfacePairSection(n.p, n.l, opts)
			result.ExactOperations += certificate.ExactOperations
			if e != nil {
				fail(&n)
				return result, e
			}
			if certificate.ExcludesContact {
				continue
			}
			boundaryExists := false
			if !certificate.ProvenUnique && certificate.CenterInvertible && certificate.ContractionBound < 1 {
				remaining := o.MaxOperations - result.ExactOperations
				if remaining < 1 {
					fail(&n)
					return result, ErrConvergence
				}
				edgeBudget := &polynomialBudget{maxOperations: remaining, maxBits: o.MaxCoefficientBits}
				ld := n.l.Domain()
				boundaryExists, e = proveLineEdgeRoot(n.p, origin, direction, patch.Domain(), ld.VMin, ld.VMax, edgeBudget)
				result.ExactOperations += edgeBudget.operations
				if e != nil {
					fail(&n)
					return result, e
				}
			}
			if certificate.ProvenUnique || boundaryExists {
				r := certificate.KrawczykBounds
				spatial, ok := n.p.bounds.Intersect(n.l.bounds)
				if !ok {
					return result, ErrInvalid
				}
				d := n.l.Domain()
				pd := n.p.Domain()
				hit := LineSplineSurfaceHit{pd, ParameterSpan{Start: d.VMin, End: d.VMax}, UVBox{Vec2{max(pd.UMin, r[0].Min), max(pd.VMin, r[1].Min)}, Vec2{min(pd.UMax, r[0].Max), min(pd.VMax, r[1].Max)}}, max(d.VMin, r[2].Min), min(d.VMax, r[2].Max), spatial}
				same, ambiguous := false, false
				for _, previous := range result.Hits {
					if lineHitRootInside(hit, previous) || lineHitRootInside(previous, hit) {
						same = true
					} else if lineHitCellsOverlap(hit, previous) {
						ambiguous = true
					}
				}
				if !same {
					if ambiguous {
						pending(n, "unresolved contact identity across boundary")
					} else {
						result.Hits = append(result.Hits, hit)
					}
				}
				continue
			}
		}
		if n.depth >= o.MaxDepth {
			pending(n, "unresolved boundary, tangent, coincident or singular contact")
			continue
		}
		d, ld := n.p.Domain(), n.l.Domain()
		splitLine := false
		u, t := true, uk
		if !hu {
			if hv {
				u, t = false, vk
			} else {
				wu, wv, wl := d.UMax-d.UMin, d.VMax-d.VMin, ld.VMax-ld.VMin
				if wl > max(wu, wv) {
					splitLine = true
					u, t = false, ld.VMin+wl/2
				} else if wu >= wv {
					t = d.UMin + wu/2
				} else {
					u, t = false, d.VMin+wv/2
				}
			}
		}
		source := n.p
		if splitLine {
			source = n.l
		}
		domain := source.Domain()
		if (u && (t == domain.UMin || t == domain.UMax)) || (!u && (t == domain.VMin || t == domain.VMax)) {
			pending(n, "parameter resolution exhausted")
			continue
		}
		budget = &polynomialBudget{maxOperations: o.MaxOperations - result.ExactOperations, maxBits: o.MaxCoefficientBits}
		if budget.maxOperations < 1 {
			fail(&n)
			return result, ErrConvergence
		}
		left, right, e := source.split(t, u, budget)
		result.ExactOperations += budget.operations
		if e != nil {
			fail(&n)
			return result, e
		}
		if splitLine {
			stack = append(stack, node{n.p, right, n.depth + 1}, node{n.p, left, n.depth + 1})
		} else {
			stack = append(stack, node{right, n.l, n.depth + 1}, node{left, n.l, n.depth + 1})
		}
	}
	if len(result.Pending) > 0 {
		if e := rescueLineSurfaceBoundaries(&result, patch, linePatch, o); e != nil {
			return result, e
		}
	}
	if len(result.Pending) > 0 {
		return result, ErrConvergence
	}
	result.Complete = true
	return result, nil
}
