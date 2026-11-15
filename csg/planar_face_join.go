package csg

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep"
	"plainsolid/collision"
)

// PlanarFaceJoinOwner maps the original vertex, edge, and face indices of one
// input to the result. EdgeUse.Reversed compares the stored directions. The
// removed interface face maps to -1; all other entities have a result index.
type PlanarFaceJoinOwner struct {
	Vertices []int
	Edges    []brep.EdgeUse
	Faces    []int
}

// PlanarFaceJoinHistory records the two original owners and their certified
// interface. Only the perimeter vertices and edges have two original owners.
type PlanarFaceJoinHistory struct {
	Owners  [2]PlanarFaceJoinOwner
	Contact collision.PlanarFaceContact
}

// JoinPlanarFacePair returns the regularized union of two outward planar
// single-shell parts that meet only along a complete, oppositely oriented
// original face. The shared face may be concave and have matching inner wires.
// Neighboring faces and all other original incidences are preserved. Exact
// embedding and material volume are checked before returning the result.
func JoinPlanarFacePair(ctx context.Context, a, b brep.Solid, faceA, faceB int, o collision.PlanarFaceContactOptions) (brep.Solid, PlanarFaceJoinHistory, error) {
	proof, err := collision.ValidatePlanarFaceContact(ctx, a, b, faceA, faceB, o)
	if err != nil {
		return brep.Solid{}, PlanarFaceJoinHistory{}, err
	}
	h := PlanarFaceJoinHistory{Contact: proof}
	vertices, edges := a.Vertices(), a.Edges()
	bv, be := b.Vertices(), b.Edges()
	for owner, n := range [2]int{len(vertices), len(bv)} {
		h.Owners[owner].Vertices = make([]int, n)
		for i := range n {
			if owner == 0 {
				h.Owners[owner].Vertices[i] = i
			} else {
				h.Owners[owner].Vertices[i] = -1
			}
		}
	}
	for _, v := range proof.Vertices {
		h.Owners[1].Vertices[v[1]] = v[0]
	}
	for i, v := range bv {
		if h.Owners[1].Vertices[i] < 0 {
			h.Owners[1].Vertices[i] = len(vertices)
			vertices = append(vertices, v)
		}
	}
	h.Owners[0].Edges = make([]brep.EdgeUse, len(edges))
	h.Owners[1].Edges = make([]brep.EdgeUse, len(be))
	for i := range edges {
		h.Owners[0].Edges[i].Edge = i
	}
	for i := range be {
		h.Owners[1].Edges[i].Edge = -1
	}
	bmap := h.Owners[1].Vertices
	for _, pair := range proof.Edges {
		x, y := edges[pair[0]], be[pair[1]]
		mapped := brep.Edge{Start: bmap[y.Start], End: bmap[y.End]}
		edge := brep.EdgeUse{Edge: pair[0]}
		if x != mapped {
			if x.Start != mapped.End || x.End != mapped.Start {
				return brep.Solid{}, PlanarFaceJoinHistory{}, fmt.Errorf("%w: original perimeter edge map disagrees with its endpoints", collision.ErrInvalid)
			}
			edge.Reversed = true
		}
		h.Owners[1].Edges[pair[1]] = edge
	}
	for i, e := range be {
		if h.Owners[1].Edges[i].Edge < 0 {
			h.Owners[1].Edges[i].Edge = len(edges)
			edges = append(edges, brep.Edge{Start: bmap[e.Start], End: bmap[e.End]})
		}
	}
	af, bf := a.Faces(), b.Faces()
	faces := make([]brep.Face, 0, len(af)+len(bf)-2)
	for owner, source := range [2][]brep.Face{af, bf} {
		mapping := &h.Owners[owner]
		mapping.Faces = make([]int, len(source))
		remap := func(wire []brep.EdgeUse) {
			for i, u := range wire {
				q := mapping.Edges[u.Edge]
				q.Reversed = q.Reversed != u.Reversed
				wire[i] = q
			}
		}
		for i, f := range source {
			if err := ctx.Err(); err != nil {
				return brep.Solid{}, PlanarFaceJoinHistory{}, err
			}
			if i == [2]int{faceA, faceB}[owner] {
				mapping.Faces[i] = -1
				continue
			}
			remap(f.Boundary)
			for _, wire := range f.Holes {
				remap(wire)
			}
			mapping.Faces[i] = len(faces)
			faces = append(faces, f)
		}
	}
	out, err := brep.NewSolid(vertices, edges, faces)
	if err != nil {
		return brep.Solid{}, PlanarFaceJoinHistory{}, fmt.Errorf("csg: cannot rebuild the original face join: %w", err)
	}
	loc, err := collision.NewPointLocator(ctx, out, o.Validation)
	if err != nil {
		return brep.Solid{}, PlanarFaceJoinHistory{}, err
	}
	shells := loc.Shells()
	roots := 0
	volume := new(big.Rat)
	for _, s := range shells {
		if s.Depth == 0 {
			roots++
		}
		if s.Depth > 1 {
			return brep.Solid{}, PlanarFaceJoinHistory{}, fmt.Errorf("%w: joined material contains an enclosed island", collision.ErrInvalid)
		}
		volume.Add(volume, s.SignedVolume)
	}
	if roots != 1 || volume.Cmp(proof.SignedVolume) != 0 {
		return brep.Solid{}, PlanarFaceJoinHistory{}, fmt.Errorf("%w: joined boundary must enclose the complete combined material", collision.ErrInvalid)
	}
	return out, h, nil
}
