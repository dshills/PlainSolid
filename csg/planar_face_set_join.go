package csg

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
)

// PlanarFaceSetJoinHistory maps original incidences into the joined material.
// An original face, edge or vertex that was wholly inside the deleted
// interface has no result index and maps to -1.
type PlanarFaceSetJoinHistory struct {
	Owners  [2]PlanarFaceJoinOwner
	Contact collision.PlanarFaceContactSet
}

// JoinPlanarFaceSet makes a regularized union along several complete original
// faces. The interface may fold across original edges. Only entities that
// still belong to the outer boundary or to a cavity are retained.
func JoinPlanarFaceSet(ctx context.Context, a, b brep.Solid, pairs [][2]int, o collision.PlanarFaceContactOptions) (brep.Solid, PlanarFaceSetJoinHistory, error) {
	p, err := collision.ValidatePlanarFaceContactSet(ctx, a, b, pairs, o)
	if err != nil {
		return brep.Solid{}, PlanarFaceSetJoinHistory{}, err
	}
	return joinPlanarFaceSet(ctx, a, b, p, o.Validation)
}

// JoinMatchingPlanarFaces discovers the complete original interface before
// joining the two bodies, with the same topology history as JoinPlanarFaceSet.
func JoinMatchingPlanarFaces(ctx context.Context, a, b brep.Solid, o collision.PlanarFaceDiscoveryOptions) (brep.Solid, PlanarFaceSetJoinHistory, error) {
	p, err := collision.DiscoverPlanarFaceContactSet(ctx, a, b, o)
	if err != nil {
		return brep.Solid{}, PlanarFaceSetJoinHistory{}, err
	}
	return joinPlanarFaceSet(ctx, a, b, p, o.Contact.Validation)
}

func joinPlanarFaceSet(ctx context.Context, a, b brep.Solid, proof collision.PlanarFaceContactSet, o collision.LocationOptions) (brep.Solid, PlanarFaceSetJoinHistory, error) {
	var in planarInterface
	in.volume = proof.SignedVolume
	for _, f := range proof.Faces {
		in.faces[0] = append(in.faces[0], f.FaceA)
		in.faces[1] = append(in.faces[1], f.FaceB)
		in.vertices = append(in.vertices, f.Vertices...)
		in.edges = append(in.edges, f.Edges...)
	}
	result, owners, err := joinPlanarInterface(ctx, a, b, in, o)
	if err != nil {
		return brep.Solid{}, PlanarFaceSetJoinHistory{}, err
	}
	return result, PlanarFaceSetJoinHistory{Owners: owners, Contact: proof}, nil
}

type planarInterface struct {
	faces           [2][]int
	vertices, edges [][2]int
	volume          *big.Rat
}

func joinPlanarInterface(ctx context.Context, a, b brep.Solid, proof planarInterface, o collision.LocationOptions) (brep.Solid, [2]PlanarFaceJoinOwner, error) {
	var zero, owners [2]PlanarFaceJoinOwner
	points := [2][]geom.Vec3{a.Vertices(), b.Vertices()}
	lines := [2][]brep.Edge{a.Edges(), b.Edges()}
	sides := [2][]brep.Face{a.Faces(), b.Faces()}
	used := [2][]bool{make([]bool, len(lines[0])), make([]bool, len(lines[1]))}
	removed := [2][]bool{make([]bool, len(sides[0])), make([]bool, len(sides[1]))}
	makeIDs := func(n, offset int) []int {
		ids := make([]int, n)
		for i := range ids {
			ids[i] = i + offset
		}
		return ids
	}
	repsV := [2][]int{makeIDs(len(points[0]), 0), makeIDs(len(points[1]), len(points[0]))}
	repsE := [2][]int{makeIDs(len(lines[0]), 0), makeIDs(len(lines[1]), len(lines[0]))}
	for part, fs := range proof.faces {
		for _, f := range fs {
			removed[part][f] = true
		}
	}
	for _, v := range proof.vertices {
		repsV[1][v[1]] = v[0]
	}
	for _, e := range proof.edges {
		repsE[1][e[1]] = e[0]
	}
	for part, fs := range sides {
		for i, f := range fs {
			if err := ctx.Err(); err != nil {
				return brep.Solid{}, zero, err
			}
			if removed[part][i] {
				continue
			}
			for _, wire := range append([][]brep.EdgeUse{f.Boundary}, f.Holes...) {
				for _, u := range wire {
					used[part][u.Edge] = true
				}
			}
		}
	}
	unassigned := func(n int) []int {
		r := make([]int, n)
		for i := range r {
			r[i] = -1
		}
		return r
	}
	vIndex := unassigned(len(points[0]) + len(points[1]))
	eIndex := unassigned(len(lines[0]) + len(lines[1]))
	neededV := make([]bool, len(vIndex))
	for part, es := range lines {
		for i, e := range es {
			if used[part][i] {
				neededV[repsV[part][e.Start]], neededV[repsV[part][e.End]] = true, true
			}
		}
	}
	var vertices []geom.Vec3
	var edges []brep.Edge
	var faces []brep.Face
	for part, ps := range points {
		m := &owners[part]
		m.Vertices = unassigned(len(ps))
		for i, p := range ps {
			k := repsV[part][i]
			if neededV[k] && vIndex[k] < 0 {
				vIndex[k] = len(vertices)
				vertices = append(vertices, p)
			}
			m.Vertices[i] = vIndex[k]
		}
	}
	// Build only edges used by retained faces. Each merged seam keeps the
	// first available original direction, including a reversed second owner.
	for part, es := range lines {
		vmap := owners[part].Vertices
		for i, e := range es {
			k := repsE[part][i]
			if used[part][i] && eIndex[k] < 0 {
				eIndex[k] = len(edges)
				edges = append(edges, brep.Edge{Start: vmap[e.Start], End: vmap[e.End]})
			}
		}
	}
	for part, es := range lines {
		m := &owners[part]
		m.Edges = make([]brep.EdgeUse, len(es))
		for i, e := range es {
			k := eIndex[repsE[part][i]]
			m.Edges[i].Edge = k
			if k < 0 {
				continue
			}
			u, v := m.Vertices[e.Start], m.Vertices[e.End]
			stored := edges[k]
			if stored.Start == v && stored.End == u {
				m.Edges[i].Reversed = true
			} else if stored.Start != u || stored.End != v {
				return brep.Solid{}, zero, fmt.Errorf("%w: joined edge map disagrees with original endpoints", collision.ErrInvalid)
			}
		}
	}
	for part, fs := range sides {
		m := &owners[part]
		m.Faces = unassigned(len(fs))
		for i, f := range fs {
			if err := ctx.Err(); err != nil {
				return brep.Solid{}, zero, err
			}
			if removed[part][i] {
				continue
			}
			remap := func(wire []brep.EdgeUse) {
				for i, u := range wire {
					v := m.Edges[u.Edge]
					v.Reversed = v.Reversed != u.Reversed
					wire[i] = v
				}
			}
			remap(f.Boundary)
			for _, w := range f.Holes {
				remap(w)
			}
			m.Faces[i] = len(faces)
			faces = append(faces, f)
		}
	}
	result, err := brep.NewSolid(vertices, edges, faces)
	if err != nil {
		return brep.Solid{}, zero, fmt.Errorf("csg: cannot rebuild the original face join: %w", err)
	}
	loc, err := collision.NewPointLocator(ctx, result, o)
	if err != nil {
		return brep.Solid{}, zero, err
	}
	roots := 0
	volume := new(big.Rat)
	for _, s := range loc.Shells() {
		if s.Depth == 0 {
			roots++
		}
		if s.Depth > 1 {
			return brep.Solid{}, zero, fmt.Errorf("%w: joined material contains an enclosed island", collision.ErrInvalid)
		}
		volume.Add(volume, s.SignedVolume)
	}
	if roots != 1 || volume.Cmp(proof.volume) != 0 {
		return brep.Solid{}, zero, fmt.Errorf("%w: joined boundary must enclose the complete combined material", collision.ErrInvalid)
	}
	return result, owners, nil
}
