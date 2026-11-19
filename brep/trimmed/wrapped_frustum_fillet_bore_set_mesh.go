//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// CertifiedFilletedBoreCutMesh maps both original cylinder ends, including a
// blind floor end, to shared display IDs and original model-edge parameters.
type CertifiedFilletedBoreCutMesh struct {
	Rims       [2][]int
	Edges      [2][]int
	Parameters [2][]*big.Rat
}

// CertifiedFilletedFrustumBoreSetMesh certifies the entire source boundary in
// both Hausdorff directions, including multi-hole caps and separate blind
// floors. Planar fill bounds are set-wise, not a triangle parameterization.
// Closed incidence, positive volume and global nonincidence are independently
// checked. No angular or certified volume-error claim is made.
type CertifiedFilletedFrustumBoreSetMesh struct {
	Mesh                                   mesh.Mesh
	Source                                 Solid
	History                                WrappedFilletedFrustumBoreSet
	OriginalFace                           []int
	OuterRims                              [2][]int
	OuterEdges                             [2][]int
	OuterParameters                        [2][]*big.Rat
	Cuts                                   []CertifiedFilletedBoreCutMesh
	SquaredDeflection, SignedDisplayVolume *big.Rat
	Collision                              collision.Report
	TriangleChecks, Predicates             int
}

// TessellateFilletedFrustumBoreSetCertified accepts the same bounded mixed cuts
// and exact separations as the native constructor. Every source wall is
// certified independently, with cumulative triangle work; each planar fill
// validates its actual oriented loops before exact triangulation under shared
// predicate limits. Boundary correspondence/winding homotopy transfers the
// largest rim error to each closed planar region, including hole boundaries.
// No sampled classification or tolerance weld decides topology. Failure is
// atomic, including final global contact failure.
func TessellateFilletedFrustumBoreSetCertified(ctx context.Context, source Solid, lower, upper float64, cuts []FilletedFrustumBoreCut, o CertifiedWrappedFrustumMeshOptions) (out CertifiedFilletedFrustumBoreSetMesh, err error) {
	defer func() {
		if err != nil {
			out = CertifiedFilletedFrustumBoreSetMesh{}
		}
	}()
	if ctx == nil || len(cuts) == 0 || len(cuts) > 16 {
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
	native, e := FilletAndBoreWrappedFrustumSet(ctx, source, lower, upper, cuts, o.Embedding)
	if e != nil {
		return out, e
	}
	outer, e := TessellateFilletedFrustumClosedCertified(ctx, source, lower, upper, o)
	if e != nil {
		return out, e
	}
	out.Source, out.History = native.Solid, native
	out.SquaredDeflection = new(big.Rat).Set(outer.SquaredDeflection)
	out.TriangleChecks, out.Predicates = outer.TriangleChecks, outer.Predicates
	om := make([]int, len(outer.Mesh.Vertices))
	for i := range om {
		om[i] = -1
	}
	for i, tri := range outer.Mesh.Triangles {
		owner := outer.OriginalFace[i]
		if owner == native.Fillets.Caps[0] || owner == native.Fillets.Caps[1] {
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
	for end, row := range outer.Rims {
		for i, id := range row[0] {
			if om[id] < 0 {
				return out, geom.ErrConvergence
			}
			out.OuterRims[end] = append(out.OuterRims[end], om[id])
			out.OuterEdges[end] = append(out.OuterEdges[end], native.Fillets.Rims[end][0][i/(len(row[0])/4)])
			out.OuterParameters[end] = append(out.OuterParameters[end], big.NewRat(int64(4*i), int64(len(row[0]))))
		}
	}
	for _, h := range native.Cuts {
		face := native.Solid.faces[h.BoreFace]
		w, ok := face.Surface.(*geom.WrappedSurfacePatch)
		if !ok {
			return out, geom.ErrConvergence
		}
		po := o.Patch
		po.MaxTriangleChecks -= out.TriangleChecks
		if po.MaxTriangleChecks < 1 {
			return out, geom.ErrConvergence
		}
		grid, e := TessellateWrappedKnotPatchCertified(ctx, w, po)
		if e != nil {
			return out, e
		}
		if !grid.BothDirections {
			return out, geom.ErrConvergence
		}
		out.TriangleChecks += grid.TriangleChecks
		for _, bound := range grid.SquaredErrors {
			if bound.Cmp(out.SquaredDeflection) > 0 {
				out.SquaredDeflection.Set(bound)
			}
		}
		if len(out.Mesh.Triangles)+len(grid.Mesh.Triangles) > o.MaxTriangles {
			return out, geom.ErrConvergence
		}
		offset := len(out.Mesh.Vertices)
		out.Mesh.Vertices = append(out.Mesh.Vertices, grid.Mesh.Vertices...)
		for _, tri := range grid.Mesh.Triangles {
			t := [3]int{offset + tri[0], offset + tri[1], offset + tri[2]}
			if face.Reversed {
				t[1], t[2] = t[2], t[1]
			}
			out.Mesh.Triangles = append(out.Mesh.Triangles, t)
			out.OriginalFace = append(out.OriginalFace, h.BoreFace)
		}
		nu, nv := len(grid.UParameters)-1, len(grid.VParameters)-1
		if nu < 4 || nv < 1 || len(grid.Mesh.Vertices) != nu*(nv+1) {
			return out, geom.ErrConvergence
		}
		c := CertifiedFilletedBoreCutMesh{}
		for end := 0; end < 2; end++ {
			for i := 0; i < nu; i++ {
				u := grid.UParameters[i]
				edge := -1
				for _, id := range h.BoreRims[end] {
					r := native.Solid.edges[id].Range
					if u >= r.Min && u < r.Max {
						edge = id
						break
					}
				}
				if edge < 0 {
					return out, geom.ErrConvergence
				}
				c.Rims[end] = append(c.Rims[end], offset+i*(nv+1)+end*nv)
				c.Edges[end] = append(c.Edges[end], edge)
				c.Parameters[end] = append(c.Parameters[end], ratInset(u))
			}
		}
		out.Cuts = append(out.Cuts, c)
	}
	fill := func(owner int, rows [][]int) error {
		face := native.Solid.faces[owner]
		plane, ok := face.Surface.(geom.PlaneSurface)
		if !ok || plane.Frame().ZAxis() != (geom.Vec3{Z: 1}) {
			return geom.ErrConvergence
		}
		z := plane.Frame().Origin().Z
		var points [][2]*big.Rat
		var ids []int
		loops := make([][]int, len(rows))
		polygons := make([]trimvalidate.Loop, len(rows))
		for role, row := range rows {
			ordered := append([]int(nil), row...)
			if role > 0 {
				for i, j := 0, len(ordered)-1; i < j; i, j = i+1, j-1 {
					ordered[i], ordered[j] = ordered[j], ordered[i]
				}
			}
			for i, id := range ordered {
				p := out.Mesh.Vertices[id]
				if p.Z != z {
					return geom.ErrConvergence
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
			return e
		}
		if proof.Status != trimvalidate.Certified {
			return geom.ErrConvergence
		}
		remaining := o.MaxPredicates - out.Predicates
		if remaining < 1 {
			return geom.ErrConvergence
		}
		if len(out.Mesh.Triangles)+len(points)+2*(len(rows)-1)-2 > o.MaxTriangles {
			return geom.ErrConvergence
		}
		triangles, work, e := exactHoledFootprintTriangulation(ctx, points, loops, remaining)
		if e != nil {
			return e
		}
		out.Predicates += work
		for _, tri := range triangles {
			t := [3]int{ids[tri[0]], ids[tri[1]], ids[tri[2]]}
			if face.Reversed {
				t[1], t[2] = t[2], t[1]
			}
			out.Mesh.Triangles = append(out.Mesh.Triangles, t)
			out.OriginalFace = append(out.OriginalFace, owner)
		}
		return nil
	}
	for end, owner := range native.Fillets.Caps {
		rows := [][]int{out.OuterRims[end]}
		for i, c := range cuts {
			if c.Depth == 0 || (c.FromUpper == (end == 1)) {
				rows = append(rows, out.Cuts[i].Rims[end])
			}
		}
		if e = fill(owner, rows); e != nil {
			return out, e
		}
	}
	for i, h := range native.Cuts {
		if h.FloorFace < 0 {
			continue
		}
		end := 1
		if cuts[i].FromUpper {
			end = 0
		}
		if e = fill(h.FloorFace, [][]int{out.Cuts[i].Rims[end]}); e != nil {
			return out, e
		}
	}
	out.SignedDisplayVolume, out.Collision, e = checkClosedPeriodicMesh(ctx, out.Mesh, o.Collision)
	if e != nil {
		return out, e
	}
	return out, nil
}
