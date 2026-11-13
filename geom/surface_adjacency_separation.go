package geom

import (
	"context"
	"errors"
	"math/big"
)

// SurfaceAdjacencyOptions bounds the total search, including edge extraction,
// exact identity and supporting-plane certificates. Zero limits select 12
// candidate planes, depth24, 16384 visited cells, 8M operations and 16384 bits.
type SurfaceAdjacencyOptions struct{ MaxCandidates, MaxDepth, MaxCells, MaxOperations, MaxCoefficientBits int }

// SurfaceAdjacencyCertificate proves the two complete patch images meet exactly
// along the identified shared full edge: all other points occupy opposite open
// halfspaces. This includes disjoint interiors. ReverseB records whether edge B
// runs opposite edge A under normalized parameter alignment. PlanePoints defines
// the raw exact determinant plane, not a rounded unit normal. No assertion is
// made on error; no candidate found is unresolved, not proof of intersection.
type SurfaceAdjacencyCertificate struct {
	Complete, Separated, ReverseB             bool
	PlanePoints                               [3]Vec3
	A, B                                      SurfaceSupportingPlaneCertificate
	Candidates, VisitedCells, ExactOperations int
}

// CertifyAdjacentSurfaceSeparation searches a finite set of represented planes.
// Edge endpoints must be distinct and exactly representable. Candidates use
// additional exact edge samples and world-axis points; candidate choice is
// heuristic but acceptance is exact. Curved planar edges can pass; nonplanar
// edges and missed separating planes remain ErrConvergence. Side orientation is
// the increasing free parameter, independently of oriented B-rep conventions.
func CertifyAdjacentSurfaceSeparation(ctx context.Context, a, b *SurfacePatch, edgeA, edgeB SurfaceEdge, o SurfaceAdjacencyOptions) (r SurfaceAdjacencyCertificate, err error) {
	if ctx == nil || a == nil || b == nil || len(a.net) == 0 || len(b.net) == 0 || edgeA > SurfaceEdgeVMax || edgeB > SurfaceEdgeVMax {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxCandidates == 0 {
		o.MaxCandidates = 12
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 24
	}
	if o.MaxCells == 0 {
		o.MaxCells = 16384
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxCandidates < 1 || o.MaxDepth < 1 || o.MaxDepth > 128 || o.MaxCells < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	extract := func(p *SurfacePatch, edge SurfaceEdge) (*SurfaceIsoCurve, error) {
		d := p.Domain()
		axis, parameter := 0, d.UMin
		switch edge {
		case SurfaceEdgeUMax:
			parameter = d.UMax
		case SurfaceEdgeVMin:
			axis, parameter = 1, d.VMin
		case SurfaceEdgeVMax:
			axis, parameter = 1, d.VMax
		}
		remaining := o.MaxOperations - r.ExactOperations
		if remaining < 1 {
			return nil, ErrConvergence
		}
		c, e := p.IsoCurve(axis, parameter, SurfacePatchOptions{MaxPoles: max(p.options.MaxPoles, 2*max(p.countU, p.countV)), MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		if c != nil {
			r.ExactOperations += c.operations
		}
		return c, e
	}
	ca, e := extract(a, edgeA)
	if e != nil {
		return r, e
	}
	cb, e := extract(b, edgeB)
	if e != nil {
		return r, e
	}
	same := false
	for _, reverse := range []bool{false, true} {
		remaining := o.MaxOperations - r.ExactOperations
		if remaining < 1 {
			return r, ErrConvergence
		}
		comparison, e := CompareExactCurvesContext(ctx, ca, cb, ExactCurveComparisonOptions{NormalizeDomains: true, ReverseB: reverse, MaxSpans: o.MaxCells, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		r.ExactOperations += comparison.ExactOperations
		if e != nil {
			return r, e
		}
		if comparison.Equal {
			same = true
			r.ReverseB = reverse
			break
		}
	}
	if !same {
		return r, ErrConvergence
	}
	point := func(t float64) (Vec3, error) {
		remaining := o.MaxOperations - r.ExactOperations
		if remaining < 1 {
			return Vec3{}, ErrConvergence
		}
		budget := &polynomialBudget{maxOperations: remaining, maxBits: o.MaxCoefficientBits}
		exact, e := exactPatchPointAt(ca.patch, t, 0, budget)
		r.ExactOperations += budget.operations
		if e != nil {
			return Vec3{}, e
		}
		var p [3]float64
		for i := range p {
			f, e := exactRepresentableFloat(exact[i])
			if e != nil {
				return Vec3{}, e
			}
			p[i] = f
		}
		return Vec3{p[0], p[1], p[2]}, nil
	}
	lo, hi := ca.Domain()
	first, e := point(lo)
	if e != nil {
		if errors.Is(e, ErrUnrepresentable) {
			return r, ErrConvergence
		}
		return r, e
	}
	last, e := point(hi)
	if e != nil {
		if errors.Is(e, ErrUnrepresentable) {
			return r, ErrConvergence
		}
		return r, e
	}
	if first == last {
		return r, ErrConvergence
	}
	candidates := []Vec3{}
	// Sample parameters are rounded representatives; point conversion itself must
	// be exact. They propose planes and never establish curve planarity by sampling.
	for _, fraction := range []*big.Rat{big.NewRat(1, 2), big.NewRat(1, 4), big.NewRat(3, 4)} {
		mapped := parameterMap(ParameterSpan{Start: lo, End: hi}, fraction)
		t, _ := mapped.Float64()
		p, e := point(t)
		if e == nil {
			candidates = append(candidates, p)
		} else if !errors.Is(e, ErrUnrepresentable) {
			return r, e
		}
	}
	candidates = append(candidates, Vec3{}, Vec3{1, 0, 0}, Vec3{0, 1, 0}, Vec3{0, 0, 1}, Vec3{-1, 0, 0}, Vec3{0, -1, 0}, Vec3{0, 0, -1}, Vec3{1, 1, 1}, Vec3{1, -1, 1}, Vec3{-1, 1, 1})
	seen := map[Vec3]bool{}
	for _, third := range candidates {
		if third == first || third == last || seen[third] {
			continue
		}
		seen[third] = true
		if e := ctx.Err(); e != nil {
			return r, e
		}
		if r.Candidates >= o.MaxCandidates {
			return r, ErrConvergence
		}
		r.Candidates++
		plane := [3]Vec3{first, last, third}
		certify := func(p *SurfacePatch) (SurfaceSupportingPlaneCertificate, error) {
			operations, cells := o.MaxOperations-r.ExactOperations, o.MaxCells-r.VisitedCells
			if operations < 1 || cells < 1 {
				return SurfaceSupportingPlaneCertificate{}, ErrConvergence
			}
			cert, e := CertifySurfaceSupportingPlane(ctx, p, plane[0], plane[1], plane[2], SurfaceSupportingPlaneOptions{MaxDepth: o.MaxDepth, MaxCells: cells, MaxOperations: operations, MaxCoefficientBits: o.MaxCoefficientBits})
			r.ExactOperations += cert.ExactOperations
			r.VisitedCells += cert.VisitedCells
			return cert, e
		}
		ac, e := certify(a)
		if e != nil {
			if errors.Is(e, ErrConvergence) || errors.Is(e, ErrDegenerate) {
				continue
			}
			return r, e
		}
		if ac.Coincident || ac.SideMask != 1<<edgeA || ac.CornerMask != 0 {
			continue
		}
		bc, e := certify(b)
		if e != nil {
			if errors.Is(e, ErrConvergence) || errors.Is(e, ErrDegenerate) {
				continue
			}
			return r, e
		}
		if bc.Coincident || bc.SideMask != 1<<edgeB || bc.CornerMask != 0 || ac.HalfspaceSign != -bc.HalfspaceSign {
			continue
		}
		r.Complete, r.Separated = true, true
		r.PlanePoints = plane
		r.A, r.B = ac, bc
		return r, nil
	}
	return r, ErrConvergence
}
