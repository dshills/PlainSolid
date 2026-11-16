package trimmed

import (
	"context"
	"math/big"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SubdivideClippedCommonLens returns the same mixed-contact Common boundary
// with an additional exact seam on the original lens roof. The seam joins the
// two already represented branch midpoints, so each of the resulting wires
// contains one higher-order contact. Original vertices, edges, supports,
// and coedges retain their identities. The original lens face index now
// represents the lower free interval and the upper half is appended.
func (r *AlignedPolynomialEnvelope) SubdivideClippedCommonLens(ctx context.Context, o geom.GraphPointContactOptions) (Solid, error) {
	if ctx == nil || r == nil || r.kind != nativeClippedPolynomialCommon || r.clippedFootprint == nil {
		return Solid{}, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return Solid{}, err
	}
	orders := [2]int{r.contacts[0].Order, r.contacts[1].Order}
	if orders != ([2]int{3, 2}) && orders != ([2]int{2, 3}) {
		return Solid{}, geom.ErrDomain
	}

	s := r.solid
	// The source arrangement orders exterior, left strip, interior lens,
	// right strip, exterior. The two interior branches already have exact
	// midpoint splits in their original owner charts.
	const lensRegion = 2
	faceID, owner := r.history.RoofFaces[lensRegion], r.history.RoofOwners[lensRegion]
	f := s.faces[faceID]
	if f.Surface != r.sources[owner+1] || len(f.Outer) != 4 || len(f.Holes) != 0 {
		return Solid{}, geom.ErrConvergence
	}
	wireEdges := [4]int{r.history.Edges[1][0], r.history.Edges[1][1], r.history.Edges[4][0], r.history.Edges[4][1]}
	midpoints := r.history.Midpoints
	station := new(big.Rat).Add(r.contacts[0].Point[r.axis], r.contacts[1].Point[r.axis])
	station.Quo(station, big.NewRat(2, 1))
	points := [2]geom.Vec3{s.vertices[midpoints[0]], s.vertices[midpoints[1]]}
	for _, p := range points {
		free := [2]float64{p.X, p.Y}[r.axis]
		if ratInset(free).Cmp(station) != 0 {
			return Solid{}, geom.ErrConvergence
		}
	}
	dependent := 1 - r.axis
	if ([2]float64{points[0].X, points[0].Y})[dependent] == ([2]float64{points[1].X, points[1].Y})[dependent] {
		return Solid{}, geom.ErrConvergence
	}

	var vertices [4]int
	at := [2]int{-1, -1}
	for i, use := range f.Outer {
		if !slices.Contains(wireEdges[:], use.Edge) {
			return Solid{}, geom.ErrConvergence
		}
		edge := s.edges[use.Edge]
		vertices[i] = edge.Start
		if use.Reversed {
			vertices[i] = edge.End
		}
		for j, id := range midpoints {
			if vertices[i] == id {
				at[j] = i
			}
		}
	}
	if at[0] < 0 || at[1] != (at[0]+2)%len(vertices) {
		return Solid{}, geom.ErrConvergence
	}

	// The open slab has strict branch order by the source certificate. The
	// original owner projects affinely onto XY, so the straight UV segment
	// covers exactly the fiber between the two boundaries at this station.
	a, b := f.Outer[at[0]].UVStart, f.Outer[at[1]].UVStart
	edges := s.Edges()
	seam := len(edges)
	edges = append(edges, parametric.Edge{Start: midpoints[0], End: midpoints[1], Range: parametric.Interval{Max: 1},
		Curve: surfaceTrimCurve{surface: f.Surface, a: a, b: b}})
	halves := [2][]Use{
		{f.Outer[at[0]], f.Outer[(at[0]+1)%4], {Edge: seam, Reversed: true, UVStart: b, UVEnd: a}},
		{f.Outer[at[1]], f.Outer[(at[1]+1)%4], {Edge: seam, UVStart: a, UVEnd: b}},
	}
	contacts := [2]int{vertices[(at[0]+1)%4], vertices[(at[1]+1)%4]}
	if contacts[0] == r.contacts[1].Vertex {
		halves[0], halves[1] = halves[1], halves[0]
		contacts[0], contacts[1] = contacts[1], contacts[0]
	}
	if contacts != ([2]int{r.contacts[0].Vertex, r.contacts[1].Vertex}) {
		return Solid{}, geom.ErrConvergence
	}
	faces := s.Faces()
	faces[faceID].Outer, f.Outer = halves[0], halves[1]
	faces = append(faces, f)
	out, err := newSolidContext(ctx, s.vertices, edges, faces, s.tolerance, false)
	if err != nil {
		return Solid{}, err
	}
	if err = validateExactGraphPartitionEdges(ctx, out, o); err != nil {
		return Solid{}, err
	}
	if err = affineMaterialVertexLinks(ctx, out); err != nil {
		return Solid{}, err
	}
	if err = ctx.Err(); err != nil {
		return Solid{}, err
	}
	return out, nil
}
