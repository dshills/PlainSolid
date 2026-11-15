package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// AffineShellCapContactOptions bounds affine chart recovery, halfspace checks
// and contact clipping cumulatively (64M default, 128M maximum). Embedding and
// exact patch recovery retain their separate existing proof limits.
type AffineShellCapContactOptions struct {
	Embedding     SplineRectangleEmbeddingOptions
	Patches       geom.SurfacePatchOptions
	MaxOperations int
}

// AffineShellCapContact binds a positive-area coplanar interface to two complete
// original convex affine shells in strictly opposite supporting halfspaces.
// Their regularized three-dimensional Common is empty; Contact retains its original
// chart-coordinate area and boundary ownership instead of inventing a solid.
type AffineShellCapContact struct {
	sources [2]Solid
	charts  [2][]splineEmbeddingFace
	affine  [2][]*geom.AffineSurfaceChart
	faces   [2]int
	contact *intersect.AffinePatchContact
	work    int
	plane   affineMaterialPlane
	sides   [2]int
}

func (r *AffineShellCapContact) Contact() *intersect.AffinePatchContact {
	if r == nil {
		return nil
	}
	return r.contact
}
func (r *AffineShellCapContact) OriginalFaces() [2]int {
	if r == nil {
		return [2]int{-1, -1}
	}
	return r.faces
}

// SupportingPlane returns exact N·P=offset and the occupied side of each shell.
func (r *AffineShellCapContact) SupportingPlane() ([3]*big.Rat, *big.Rat, [2]int) {
	if r == nil {
		return [3]*big.Rat{}, nil, [2]int{}
	}
	n := [3]*big.Rat{}
	for k := range n {
		n[k] = new(big.Rat).Set(r.plane.n[k])
	}
	return n, new(big.Rat).Set(r.plane.d), r.sides
}
func (r *AffineShellCapContact) ExactCommonVolume() *big.Rat {
	if r == nil {
		return nil
	}
	return new(big.Rat)
}
func (r *AffineShellCapContact) ExactOperations() int {
	if r == nil {
		return 0
	}
	return r.work
}
func affineCapPlane(b *nativePrismAffineBudget, s Solid, ch splineEmbeddingFace, reverse bool) affineMaterialPlane {
	p, q, t := affineMaterialXYZ(s.vertices[ch.vertex[0]]), affineMaterialXYZ(s.vertices[ch.vertex[1]]), affineMaterialXYZ(s.vertices[ch.vertex[3]])
	plane := affineMaterialPlane{d: new(big.Rat)}
	for k := 0; k < 3; k++ {
		j, l := (k+1)%3, (k+2)%3
		plane.n[k] = b.op('-', b.op('*', b.op('-', q[j], p[j]), b.op('-', t[l], p[l])), b.op('*', b.op('-', q[l], p[l]), b.op('-', t[j], p[j])))
		if reverse {
			plane.n[k].Neg(plane.n[k])
		}
		plane.d = b.op('+', plane.d, b.op('*', plane.n[k], p[k]))
	}
	return plane
}
func CertifyAffineShellCapContact(ctx context.Context, a, c Solid, faces [2]int, o AffineShellCapContactOptions) (*AffineShellCapContact, error) {
	if ctx == nil || faces[0] < 0 || faces[0] >= len(a.faces) || faces[1] < 0 || faces[1] >= len(c.faces) {
		return nil, geom.ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 64000000
	}
	if o.MaxOperations < 1 || o.MaxOperations > 128000000 {
		return nil, geom.ErrInvalid
	}
	budget := &nativePrismAffineBudget{ctx: ctx, max: o.MaxOperations, bits: 16384}
	r := &AffineShellCapContact{sources: [2]Solid{a, c}, faces: faces}
	for owner, s := range r.sources {
		if _, e := s.validateSplineRectangleEmbedding(ctx, o.Embedding, &r.charts[owner]); e != nil {
			return nil, e
		}
		for fi, ch := range r.charts[owner] {
			if budget.work >= budget.max {
				return nil, geom.ErrConvergence
			}
			chart, e := geom.NewAffineSurfaceChart(ctx, ch.patch, geom.AffineSurfaceChartOptions{MaxOperations: budget.max - budget.work, MaxCoefficientBits: 16384, MaxCells: 4096})
			if e != nil {
				return nil, e
			}
			budget.work += chart.ExactOperations()
			r.affine[owner] = append(r.affine[owner], chart)
			plane := affineCapPlane(budget, s, ch, s.faces[fi].Reversed)
			strict := false
			for _, v := range s.vertices {
				sign := affineMaterialValue(budget, plane, affineMaterialXYZ(v)).Sign()
				if sign > 0 {
					return nil, geom.ErrDomain
				}
				strict = strict || sign < 0
			}
			if budget.err != nil {
				return nil, budget.err
			}
			if !strict {
				return nil, geom.ErrDomain
			}
		}
	}
	plane := affineCapPlane(budget, a, r.charts[0][faces[0]], false)
	sides := [2]int{}
	for owner, s := range r.sources {
		cap := r.charts[owner][faces[owner]]
		onCap := map[int]bool{}
		for _, v := range cap.vertex {
			onCap[v] = true
		}
		for vi, v := range s.vertices {
			sign := affineMaterialValue(budget, plane, affineMaterialXYZ(v)).Sign()
			if onCap[vi] {
				if sign != 0 {
					return nil, geom.ErrDomain
				}
			} else {
				if sign == 0 || (sides[owner] != 0 && sides[owner] != sign) {
					return nil, geom.ErrDomain
				}
				sides[owner] = sign
			}
		}
	}
	if budget.err != nil {
		return nil, budget.err
	}
	if sides[0] == 0 || sides[1] != -sides[0] {
		return nil, geom.ErrDomain
	}
	pa, e := algebraicShellTracePatch(a.faces[faces[0]], r.charts[0][faces[0]].patch.Domain())
	if e != nil {
		return nil, e
	}
	pc, e := algebraicShellTracePatch(c.faces[faces[1]], r.charts[1][faces[1]].patch.Domain())
	if e != nil {
		return nil, e
	}
	if budget.work >= budget.max {
		return nil, geom.ErrConvergence
	}
	r.contact, e = intersect.IntersectAffinePatches(ctx, pa, pc, o.Patches, geom.AffineChartIntersectionOptions{MaxOperations: budget.max - budget.work, MaxCoefficientBits: 16384})
	if e != nil {
		return nil, e
	}
	budget.work += r.contact.ExactOperations()
	kind, e := r.contact.Kind()
	if e != nil {
		return nil, e
	}
	if kind != intersect.AffinePatchContactArea {
		return nil, geom.ErrDomain
	}
	area, _, _, e := r.contact.CoplanarArea()
	if e != nil || area.Sign() <= 0 {
		return nil, geom.ErrDomain
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	r.work = budget.work
	r.plane = plane
	r.sides = sides
	return r, nil
}
