package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// CertifiedFilletedFrustumBoreMesh is a closed represented display of one
// eccentric through bore. Rims[end][0/1] retain outer/inner cap rows, with
// actual native edge ownership. SquaredDeflection bounds both directions
// between the entire source boundary and mesh, including planar annular fill.
// It does not assert a per-cap-triangle source parameterization or angular bound.
type CertifiedFilletedFrustumBoreMesh struct {
	Mesh                                   mesh.Mesh
	Source                                 Solid
	History                                WrappedFilletedFrustumBoreHistory
	OriginalFace                           []int
	Rims                                   [2][2][]int
	RimEdges                               [2][2][]int
	RimParameters                          [2][2][]*big.Rat // original increasing model-edge parameter
	SquaredDeflection, SignedDisplayVolume *big.Rat
	Collision                              collision.Report
	TriangleChecks, Predicates             int
}

// TessellateFilletedFrustumThroughBoreCertified retains certified outer and
// inner rim samples without tolerance welding. The exact source circles are
// strictly nested by the native constructor. Independently validated displayed
// loops and exact area/boundary-chain triangulation prove annular coverage.
// Boundary homotopy within the certified chord error preserves winding away
// from that neighborhood, proving the same two-sided filled-cap bound. Each
// support uses its original native parameterization; no rounded surrogate is
// introduced. Triangle checks and polygon predicates are cumulative; default
// trim-loop validation and collision work limits remain independently bounded.
func TessellateFilletedFrustumThroughBoreCertified(ctx context.Context, source Solid, lower, upper float64, center geom.Vec2, radius float64, o CertifiedWrappedFrustumMeshOptions) (out CertifiedFilletedFrustumBoreMesh, err error) {
	defer func() {
		if err != nil {
			out = CertifiedFilletedFrustumBoreMesh{}
		}
	}()
	if ctx == nil {
		return out, geom.ErrInvalid
	}
	if o.Patch.MaxTriangleChecks == 0 {
		o.Patch.MaxTriangleChecks = 100000
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 100000
	}
	if o.MaxPredicates == 0 {
		o.MaxPredicates = 1000000
	}
	outer, e := TessellateFilletedFrustumClosedCertified(ctx, source, lower, upper, o)
	if e != nil {
		return out, e
	}
	body, h, e := FilletAndBoreWrappedFrustum(ctx, source, lower, upper, center, radius, o.Embedding)
	if e != nil {
		return out, e
	}
	face := body.faces[h.BoreFace]
	w, ok := face.Surface.(*geom.WrappedSurfacePatch)
	if !ok {
		return out, geom.ErrConvergence
	}
	po := o.Patch
	po.MaxTriangleChecks -= outer.TriangleChecks
	if po.MaxTriangleChecks < 1 {
		return out, geom.ErrConvergence
	}
	inner, e := TessellateWrappedKnotPatchCertified(ctx, w, po)
	if e != nil {
		return out, e
	}
	if !inner.BothDirections {
		return out, geom.ErrConvergence
	}
	out.Source, out.History = body, h
	out.TriangleChecks = outer.TriangleChecks + inner.TriangleChecks
	out.Predicates = outer.Predicates
	out.SquaredDeflection = new(big.Rat).Set(outer.SquaredDeflection)
	for _, bound := range inner.SquaredErrors {
		if bound.Cmp(out.SquaredDeflection) > 0 {
			out.SquaredDeflection.Set(bound)
		}
	}
	// Only supported wall triangles are transferred; old disk center vertices
	// are omitted, so every final vertex belongs to the actual closed shell.
	om := make([]int, len(outer.Mesh.Vertices))
	for i := range om {
		om[i] = -1
	}
	for i, tri := range outer.Mesh.Triangles {
		owner := outer.OriginalFace[i]
		if owner == h.Fillets.Caps[0] || owner == h.Fillets.Caps[1] {
			continue
		}
		var t [3]int
		for j, id := range tri {
			if om[id] < 0 {
				om[id] = len(out.Mesh.Vertices)
				out.Mesh.Vertices = append(out.Mesh.Vertices, outer.Mesh.Vertices[id])
			}
			t[j] = om[id]
		}
		out.Mesh.Triangles = append(out.Mesh.Triangles, t)
		out.OriginalFace = append(out.OriginalFace, owner)
	}
	offset := len(out.Mesh.Vertices)
	out.Mesh.Vertices = append(out.Mesh.Vertices, inner.Mesh.Vertices...)
	for _, tri := range inner.Mesh.Triangles {
		t := [3]int{offset + tri[0], offset + tri[1], offset + tri[2]}
		if face.Reversed {
			t[1], t[2] = t[2], t[1]
		}
		out.Mesh.Triangles = append(out.Mesh.Triangles, t)
		out.OriginalFace = append(out.OriginalFace, h.BoreFace)
	}
	nu, nv := len(inner.UParameters)-1, len(inner.VParameters)-1
	if nu < 4 || nv < 1 || len(inner.Mesh.Vertices) != nu*(nv+1) {
		return out, geom.ErrConvergence
	}
	for end := 0; end < 2; end++ {
		for i, id := range outer.Rims[end][0] {
			if om[id] < 0 {
				return out, geom.ErrConvergence
			}
			out.Rims[end][0] = append(out.Rims[end][0], om[id])
			out.RimParameters[end][0] = append(out.RimParameters[end][0], big.NewRat(int64(4*i), int64(len(outer.Rims[end][0]))))
			quadrant := i / (len(outer.Rims[end][0]) / 4)
			out.RimEdges[end][0] = append(out.RimEdges[end][0], h.Fillets.Rims[end][0][quadrant])
		}
		for i := 0; i < nu; i++ {
			out.Rims[end][1] = append(out.Rims[end][1], offset+i*(nv+1)+end*nv)
			u := inner.UParameters[i]
			out.RimParameters[end][1] = append(out.RimParameters[end][1], ratInset(u))
			edge := -1
			for _, id := range h.BoreRims[end] {
				r := body.edges[id].Range
				if u >= r.Min && u < r.Max {
					edge = id
					break
				}
			}
			if edge < 0 {
				return out, geom.ErrConvergence
			}
			out.RimEdges[end][1] = append(out.RimEdges[end][1], edge)
		}
		var points [][2]*big.Rat
		var ids []int
		loops := make([][]int, 2)
		polygons := make([]trimvalidate.Loop, 2)
		plane, ok := body.faces[h.Fillets.Caps[end]].Surface.(geom.PlaneSurface)
		if !ok {
			return out, geom.ErrConvergence
		}
		z := plane.Frame().Origin().Z
		for role, row := range out.Rims[end] {
			ordered := append([]int(nil), row...)
			if role == 1 {
				for i, j := 0, len(ordered)-1; i < j; i, j = i+1, j-1 {
					ordered[i], ordered[j] = ordered[j], ordered[i]
				}
			}
			for i, id := range ordered {
				p := out.Mesh.Vertices[id]
				if p.Z != z {
					return out, geom.ErrConvergence
				}
				loops[role] = append(loops[role], len(points))
				points = append(points, [2]*big.Rat{ratInset(p.X), ratInset(p.Y)})
				ids = append(ids, id)
				next := out.Mesh.Vertices[ordered[(i+1)%len(ordered)]]
				polygons[role] = append(polygons[role], trimvalidate.Segment{Start: geom.Vec2{X: p.X, Y: p.Y}, End: geom.Vec2{X: next.X, Y: next.Y}})
			}
		}
		proof, e := trimvalidate.ValidateLoops(ctx, polygons[0], polygons[1:], trimvalidate.Options{})
		if e != nil {
			return out, e
		}
		if proof.Status != trimvalidate.Certified {
			return out, geom.ErrConvergence
		}
		if len(out.Mesh.Triangles)+len(points) > o.MaxTriangles {
			return out, geom.ErrConvergence
		}
		remaining := o.MaxPredicates - out.Predicates
		if remaining < 1 {
			return out, geom.ErrConvergence
		}
		tris, work, e := exactHoledFootprintTriangulation(ctx, points, loops, remaining)
		if e != nil {
			return out, e
		}
		out.Predicates += work
		for _, tri := range tris {
			t := [3]int{ids[tri[0]], ids[tri[1]], ids[tri[2]]}
			if end == 0 {
				t[1], t[2] = t[2], t[1]
			}
			out.Mesh.Triangles = append(out.Mesh.Triangles, t)
			out.OriginalFace = append(out.OriginalFace, h.Fillets.Caps[end])
		}
	}
	if len(out.Mesh.Triangles) > o.MaxTriangles {
		return out, geom.ErrConvergence
	}
	out.SignedDisplayVolume, out.Collision, e = checkClosedPeriodicMesh(ctx, out.Mesh, o.Collision)
	if e != nil {
		return out, e
	}
	return out, nil
}
