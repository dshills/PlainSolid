package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// ConvexAffinePolygonCapOptions preserves separately bounded face/edge stages
// and cumulatively bounds both whole-shell authenticators and cap arithmetic.
// MaxOperations defaults to 64M and cannot exceed 128M.
type ConvexAffinePolygonCapOptions struct {
	Shell         ConvexAffinePolygonShellOptions
	MaxOperations int
}

// ConvexAffinePolygonCapContact proves that the complete intersection of two
// convex native shells is this positive-area original cap region. All noncap
// vertices lie strictly on opposite sides of its plane, hence no volume or
// additional off-plane contacts exist. It does not construct a fused shell.
type ConvexAffinePolygonCapContact struct {
	shells  [2]*ConvexAffinePolygonShell
	faces   [2]int
	contact *intersect.AffinePolygonContact
	plane   [4]*big.Rat
	work    int
}

func (r *ConvexAffinePolygonCapContact) Contact() *intersect.AffinePolygonContact {
	if r == nil {
		return nil
	}
	return r.contact
}
func (r *ConvexAffinePolygonCapContact) OriginalFaces() [2]int {
	if r == nil {
		return [2]int{-1, -1}
	}
	return r.faces
}
func (r *ConvexAffinePolygonCapContact) OriginalShell(owner int) (*ConvexAffinePolygonShell, error) {
	if r == nil || owner < 0 || owner > 1 {
		return nil, geom.ErrInvalid
	}
	return r.shells[owner], nil
}

// SupportingPlane returns N·P+D=0, oriented outward from original owner zero.
func (r *ConvexAffinePolygonCapContact) SupportingPlane() [4]*big.Rat {
	p := [4]*big.Rat{}
	if r != nil {
		for i := range p {
			p[i] = new(big.Rat).Set(r.plane[i])
		}
	}
	return p
}
func (r *ConvexAffinePolygonCapContact) ExactCommonVolume() *big.Rat {
	if r == nil {
		return nil
	}
	return new(big.Rat)
}
func (r *ConvexAffinePolygonCapContact) ExactOperations() int {
	if r == nil {
		return 0
	}
	return r.work
}

func CertifyConvexAffinePolygonCapContact(ctx context.Context, a, c Solid, faces [2]int, o ConvexAffinePolygonCapOptions) (*ConvexAffinePolygonCapContact, error) {
	if ctx == nil || faces[0] < 0 || faces[0] >= len(a.faces) || faces[1] < 0 || faces[1] >= len(c.faces) {
		return nil, geom.ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 64000000
	}
	if o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.Shell.MaxOperations < 0 || o.Shell.MaxOperations > 128000000 {
		return nil, geom.ErrInvalid
	}
	b := &nativePrismAffineBudget{ctx: ctx, max: o.MaxOperations, bits: 16384}
	r := &ConvexAffinePolygonCapContact{faces: faces}
	for owner, s := range [2]Solid{a, c} {
		if b.work >= b.max {
			return nil, geom.ErrConvergence
		}
		so := o.Shell
		if so.MaxOperations == 0 || so.MaxOperations > b.max-b.work {
			so.MaxOperations = b.max - b.work
		}
		shell, e := CertifyConvexAffinePolygonShell(ctx, s, so)
		if e != nil {
			return nil, e
		}
		r.shells[owner] = shell
		b.work += shell.ExactOperations()
	}
	r.plane = r.shells[0].queries[faces[0]].Plane()
	plane := affineMaterialPlane{n: [3]*big.Rat{r.plane[0], r.plane[1], r.plane[2]}, d: new(big.Rat).Neg(r.plane[3])}
	loops := [2][][2]*big.Rat{}
	for owner, shell := range r.shells {
		s := shell.source
		f := s.faces[faces[owner]]
		onCap := map[int]bool{}
		for _, u := range f.Outer {
			edge := s.edges[u.Edge]
			onCap[edge.Start] = true
			onCap[edge.End] = true
			loops[owner] = append(loops[owner], [2]*big.Rat{ratInset(u.UVStart.X), ratInset(u.UVStart.Y)})
		}
		for vi, p := range s.vertices {
			sign := affineMaterialValue(b, plane, affineMaterialXYZ(p)).Sign()
			if b.err != nil {
				return nil, b.err
			}
			if onCap[vi] {
				if sign != 0 {
					return nil, geom.ErrDomain
				}
			} else if (owner == 0 && sign != -1) || (owner == 1 && sign != 1) {
				return nil, geom.ErrDomain
			}
		}
	}
	if b.work >= b.max {
		return nil, geom.ErrConvergence
	}
	pa, e := algebraicShellTracePatch(a.faces[faces[0]], r.shells[0].queries[faces[0]].chart.Domain())
	if e != nil {
		return nil, e
	}
	pc, e := algebraicShellTracePatch(c.faces[faces[1]], r.shells[1].queries[faces[1]].chart.Domain())
	if e != nil {
		return nil, e
	}
	r.contact, e = intersect.IntersectConvexAffinePatches(ctx, pa, pc, loops, o.Shell.Face.Patch, intersect.AffinePolygonContactOptions{MaxEdges: len(loops[0]) + len(loops[1]), MaxOperations: b.max - b.work, MaxCoefficientBits: 16384})
	if e != nil {
		return nil, e
	}
	contact := r.contact.Contact()
	b.work += contact.ExactOperations()
	kind, e := contact.Kind()
	if e != nil {
		return nil, e
	}
	if kind != intersect.AffinePatchContactArea {
		return nil, geom.ErrDomain
	}
	area, _, _, e := contact.CoplanarArea()
	if e != nil || area.Sign() <= 0 {
		return nil, geom.ErrDomain
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	r.work = b.work
	return r, nil
}
