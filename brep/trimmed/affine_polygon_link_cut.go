//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// ConvexAffinePolygonLinkCutOptions explicitly opts into a topologically
// normalized boundary with isolated self-contact. It does not assert embedding.
// MaxFacePairs defaults to 8192 (maximum 65536), MaxContactPoints to 8 (maximum 64).
// Material and Sew retain their independent arithmetic/evaluation limits.
type ConvexAffinePolygonLinkCutOptions struct {
	Material                       ConvexAffinePolygonMaterialOptions
	MaxFacePairs, MaxContactPoints int
}

// ConvexAffinePolygonVertexContact binds distinct native vertex identities to
// one exact original paired junction. Each vertex has its own circular link.
// IncidentFaces indexes the returned Solid and matches Vertices elementwise.
type ConvexAffinePolygonVertexContact struct {
	SectionVertex int
	Point         [3]*big.Rat
	Vertices      []int
	IncidentFaces [][]int
}

// ConvexAffinePolygonLinkCut is a connected closed indexed boundary of A-B.
// Its physical image may self-contact only at the explicitly proved points.
// It is not an embedded manifold certificate. Each native Solid codec reload
// revalidates topology/curves; the separate global contact ledger is not stored.
type ConvexAffinePolygonLinkCut struct {
	material *ConvexAffinePolygonMaterial
	contacts []ConvexAffinePolygonVertexContact
	pairs    int
	work     int
}

func (r *ConvexAffinePolygonLinkCut) Solid() Solid {
	if r == nil {
		return Solid{}
	}
	return r.material.Solid()
}
func (r *ConvexAffinePolygonLinkCut) ExactVolume() *big.Rat {
	if r == nil {
		return nil
	}
	return r.material.ExactVolume()
}
func (r *ConvexAffinePolygonLinkCut) ExactOperations() int {
	if r == nil {
		return 0
	}
	return r.work
}
func (r *ConvexAffinePolygonLinkCut) CompleteFacePairs() int {
	if r == nil {
		return 0
	}
	return r.pairs
}
func (r *ConvexAffinePolygonLinkCut) OriginalCrossing() *ConvexAffinePolygonCrossing {
	if r == nil {
		return nil
	}
	return r.material.OriginalCrossing()
}
func (r *ConvexAffinePolygonLinkCut) FaceSources() []AffineShellMaterialFace {
	if r == nil {
		return nil
	}
	return r.material.FaceSources()
}
func (r *ConvexAffinePolygonLinkCut) Fragments() []AffineShellMaterialFace {
	if r == nil {
		return nil
	}
	return r.material.Fragments()
}
func (r *ConvexAffinePolygonLinkCut) SourceFaceFragments(owner, face int) *AffineImprintFaceFragments {
	if r == nil {
		return nil
	}
	return r.material.SourceFaceFragments(owner, face)
}
func (r *ConvexAffinePolygonLinkCut) IsolatedVertexContacts() []ConvexAffinePolygonVertexContact {
	if r == nil {
		return nil
	}
	out := append([]ConvexAffinePolygonVertexContact(nil), r.contacts...)
	for i := range out {
		for k := range out[i].Point {
			out[i].Point[k] = new(big.Rat).Set(out[i].Point[k])
		}
		out[i].Vertices = append([]int(nil), out[i].Vertices...)
		out[i].IncidentFaces = make([][]int, len(r.contacts[i].IncidentFaces))
		for j, f := range r.contacts[i].IncidentFaces {
			out[i].IncidentFaces[j] = append([]int(nil), f...)
		}
	}
	return out
}

// CutWithSeparatedVertexLinks joins exact edge mates before identifying their
// endpoint occurrences. Coincident model points are never enough to merge two
// vertex links. Complete original occupancy and all output triangle contacts
// are re-proved; unmatched edges, unowned point contacts, interval/area contacts
// outside declared topology, disconnected shells and noncircular links reject.
// Existing Combine/CombineComponents retain their strict physical-link gate.
func (r *ConvexAffinePolygonCrossing) CutWithSeparatedVertexLinks(ctx context.Context, o ConvexAffinePolygonLinkCutOptions) (*ConvexAffinePolygonLinkCut, error) {
	if o.MaxFacePairs == 0 {
		o.MaxFacePairs = 8192
	}
	if o.MaxContactPoints == 0 {
		o.MaxContactPoints = 8
	}
	if o.MaxFacePairs < 1 || o.MaxFacePairs > 65536 || o.MaxContactPoints < 1 || o.MaxContactPoints > 64 {
		return nil, geom.ErrInvalid
	}
	selection, faces, options, e := r.selectPolygonMaterial(ctx, intersect.AffineRingDifference, o.Material)
	if e != nil {
		return nil, e
	}
	if len(faces)*(len(faces)-1)/2 > o.MaxFacePairs {
		return nil, geom.ErrConvergence
	}
	b := &nativePrismAffineBudget{ctx: ctx, max: options.MaxOperations, bits: 16384, work: selection.work}
	solid, e := sewPolygonEdgeOccurrences(ctx, faces, options.Sew)
	if e != nil {
		return nil, e
	}
	if !affinePolygonConnectedFaces(solid) {
		return nil, geom.ErrDomain
	}
	if e = affineMaterialVertexLinks(ctx, solid); e != nil {
		return nil, e
	}
	if e = r.verifyPolygonMaterialSolid(ctx, b, solid, selection.FaceSources(), options); e != nil {
		return nil, e
	}
	contacts, e := r.polygonCoincidentVertices(ctx, solid, o.MaxContactPoints)
	if e != nil {
		return nil, e
	}
	if len(contacts) == 0 {
		return nil, geom.ErrDomain
	}
	pairs, e := r.verifyPolygonContactTopology(ctx, b, solid, selection.FaceSources(), contacts)
	if e != nil {
		return nil, e
	}
	if b.err != nil {
		return nil, b.err
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	selection.solid = solid
	selection.work = b.work
	return &ConvexAffinePolygonLinkCut{material: &ConvexAffinePolygonMaterial{crossing: r, material: selection}, contacts: contacts, pairs: pairs, work: b.work}, nil
}

type polygonModelEdgeKey struct{ a, c geom.Vec3 }

func polygonModelLess(a, c geom.Vec3) bool {
	if a.X != c.X {
		return a.X < c.X
	}
	if a.Y != c.Y {
		return a.Y < c.Y
	}
	return a.Z < c.Z
}
func polygonModelKey(a, c geom.Vec3) polygonModelEdgeKey {
	if polygonModelLess(c, a) {
		a, c = c, a
	}
	return polygonModelEdgeKey{a, c}
}

// Only called on source-authenticated affine triangles. Unlike general Sew,
// endpoint equivalence is induced by proved paired edges, never a point map.
func sewPolygonEdgeOccurrences(ctx context.Context, input []SewFace, o SewOptions) (Solid, error) {
	if o.MaxUses == 0 {
		o.MaxUses = 4096
	}
	if ctx == nil || o.MaxUses < 1 || o.MaxUses > 1<<20 || len(input) > o.MaxFaces {
		return Solid{}, geom.ErrInvalid
	}
	if 3*len(input) > o.MaxUses {
		return Solid{}, parametric.ErrBudget
	}
	type occurrence struct {
		face, edge int
		use        Use
	}
	offsets := make([]int, len(input))
	n := 0
	edges := map[polygonModelEdgeKey][]occurrence{}
	for fi, f := range input {
		if e := ctx.Err(); e != nil {
			return Solid{}, e
		}
		if len(f.Vertices) != 3 || len(f.Edges) != 3 || len(f.Face.Outer) != 3 || len(f.Face.Holes) != 0 {
			return Solid{}, geom.ErrDomain
		}
		offsets[fi] = n
		n += 3
		for _, u := range f.Face.Outer {
			if u.Edge < 0 || u.Edge >= 3 {
				return Solid{}, geom.ErrDomain
			}
			edge := f.Edges[u.Edge]
			if edge.Start < 0 || edge.Start >= 3 || edge.End < 0 || edge.End >= 3 || edge.Start == edge.End {
				return Solid{}, geom.ErrDomain
			}
			a, c := f.Vertices[edge.Start], f.Vertices[edge.End]
			if a == c {
				return Solid{}, geom.ErrDomain
			}
			key := polygonModelKey(a, c)
			edges[key] = append(edges[key], occurrence{fi, u.Edge, u})
			if len(edges[key]) > 2 {
				return Solid{}, geom.ErrDomain
			}
		}
	}
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	join := func(a, c int) {
		a, c = find(a), find(c)
		if a < c {
			parent[c] = a
		} else {
			parent[a] = c
		}
	}
	// Iteration order cannot affect the minimal representative or published order.
	for key, uses := range edges {
		if len(uses) != 2 || uses[0].face == uses[1].face {
			return Solid{}, geom.ErrDomain
		}
		x, y := uses[0], uses[1]
		fx, fy := input[x.face], input[y.face]
		a, c := fx.Edges[x.edge], fy.Edges[y.edge]
		reversed := fx.Vertices[a.Start] != fy.Vertices[c.Start]
		if (x.use.Reversed != fx.Face.Reversed) == ((y.use.Reversed != fy.Face.Reversed) != reversed) {
			return Solid{}, geom.ErrDomain
		}
		same := sameSewModelEdge(a, c, reversed) || sameSewStraightTrimEdges(a, c, reversed)
		if !same {
			var e error
			same, e = compareSewSplineEdges(ctx, a, c, reversed, o.Proof)
			if e != nil {
				return Solid{}, e
			}
		}
		if !same {
			return Solid{}, geom.ErrDomain
		}
		cs, ce := c.Start, c.End
		if reversed {
			cs, ce = ce, cs
		}
		if fx.Vertices[a.Start] != fy.Vertices[cs] || fx.Vertices[a.End] != fy.Vertices[ce] || key.a == key.c {
			return Solid{}, geom.ErrDomain
		}
		join(offsets[x.face]+a.Start, offsets[y.face]+cs)
		join(offsets[x.face]+a.End, offsets[y.face]+ce)
	}
	vertices := []geom.Vec3{}
	ids := map[int]int{}
	local := make([][]int, len(input))
	for fi, f := range input {
		local[fi] = make([]int, 3)
		for vi, p := range f.Vertices {
			root := find(offsets[fi] + vi)
			id, ok := ids[root]
			if !ok {
				id = len(vertices)
				ids[root] = id
				vertices = append(vertices, p)
			} else if vertices[id] != p {
				return Solid{}, geom.ErrDomain
			}
			local[fi][vi] = id
		}
	}
	nativeEdges := []parametric.Edge{}
	nativeFaces := []Face{}
	edgeIDs := map[polygonModelEdgeKey]int{}
	for fi, f := range input {
		face := cloneFace(f.Face)
		for ui, u := range face.Outer {
			edge := f.Edges[u.Edge]
			key := polygonModelKey(f.Vertices[edge.Start], f.Vertices[edge.End])
			edge.Start, edge.End = local[fi][edge.Start], local[fi][edge.End]
			id, ok := edgeIDs[key]
			if !ok {
				id = len(nativeEdges)
				edgeIDs[key] = id
				nativeEdges = append(nativeEdges, edge)
			} else {
				prior := nativeEdges[id]
				rev := prior.Start != edge.Start
				if rev {
					if prior.Start != edge.End || prior.End != edge.Start {
						return Solid{}, geom.ErrDomain
					}
				} else if prior.End != edge.End {
					return Solid{}, geom.ErrDomain
				}
				u.Reversed = u.Reversed != rev
			}
			u.Edge = id
			face.Outer[ui] = u
		}
		nativeFaces = append(nativeFaces, face)
	}
	return NewSolidContext(ctx, vertices, nativeEdges, nativeFaces, o.Tolerance)
}
