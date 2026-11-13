package section

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
)

// Corner identifies one exact intersection with a mesh edge. MeshEdge is sorted
// by original vertex index and Parameter follows MeshEdge[0]->MeshEdge[1]. Edge
// names an original BRep edge or -1 for a tessellation diagonal; EdgeParameter
// follows that original edge's stored Start->End direction, and is nil for a
// diagonal. Point and both parameters are caller-owned rationals.
type Corner struct {
	Point         [3]*big.Rat
	MeshEdge      [2]int
	Parameter     *big.Rat
	Edge          int
	EdgeParameter *big.Rat
}

// Segment is a directed unsimplified section edge naming its source face and
// triangle. Start/End index Corners. Multiple segments can descend from one face;
// collinear simplification in Loops deliberately does not erase this graph.
type Segment struct{ Start, End, Face, Triangle int }
type Ancestry struct {
	Loops    []Loop
	Corners  []Corner
	Segments []Segment
}

// PlanarWithAncestry preserves the exact transverse intersection graph alongside
// Planar's rounded simplified loops. Corner order is first occurrence in source
// tessellation order. The same source-vertex contact rejection applies. The
// graph pass is linear in triangles/edges and uses the same maxTriangles cap;
// cancellation is checked per source entity. No welding or Boolean is performed.
func PlanarWithAncestry(ctx context.Context, s brep.Solid, p geom.Plane, maxTriangles int) (Ancestry, error) {
	empty := Ancestry{}
	loops, e := Planar(ctx, s, p, maxTriangles)
	if e != nil {
		return empty, e
	}
	r := Ancestry{Loops: loops}
	m := s.Tessellate()
	faces := s.TriangleFaces()
	normal, origin := p.Normal(), p.Origin()
	nr := [3]*big.Rat{new(big.Rat).SetFloat64(normal.X), new(big.Rat).SetFloat64(normal.Y), new(big.Rat).SetFloat64(normal.Z)}
	ov := [3]float64{origin.X, origin.Y, origin.Z}
	points := make([][3]*big.Rat, len(m.Vertices))
	dist := make([]*big.Rat, len(m.Vertices))
	for i, v := range m.Vertices {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		xyz := [3]float64{v.X, v.Y, v.Z}
		d := new(big.Rat)
		for k, x := range xyz {
			points[i][k] = new(big.Rat).SetFloat64(x)
			d.Add(d, new(big.Rat).Mul(new(big.Rat).Sub(points[i][k], new(big.Rat).SetFloat64(ov[k])), nr[k]))
		}
		dist[i] = d
	}
	original := map[[2]int]int{}
	edges := s.Edges()
	for i, e := range edges {
		if er := ctx.Err(); er != nil {
			return empty, er
		}
		original[[2]int{min(e.Start, e.End), max(e.Start, e.End)}] = i
	}
	corners := map[[2]int]int{}
	crossing := func(a, b int) int {
		key := [2]int{min(a, b), max(a, b)}
		if i, ok := corners[key]; ok {
			return i
		}
		a, b = key[0], key[1]
		t := new(big.Rat).Quo(dist[a], new(big.Rat).Sub(dist[a], dist[b]))
		corner := Corner{MeshEdge: key, Parameter: new(big.Rat).Set(t), Edge: -1}
		for k := range corner.Point {
			corner.Point[k] = new(big.Rat).Add(points[a][k], new(big.Rat).Mul(t, new(big.Rat).Sub(points[b][k], points[a][k])))
		}
		if edge, ok := original[key]; ok {
			corner.Edge = edge
			corner.EdgeParameter = new(big.Rat).Set(t)
			if edges[edge].Start != a {
				corner.EdgeParameter.Sub(big.NewRat(1, 1), t)
			}
		}
		id := len(r.Corners)
		r.Corners = append(r.Corners, corner)
		corners[key] = id
		return id
	}
	for ti, tr := range m.Triangles {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		start, end := -1, -1
		for j, a := range tr {
			b := tr[(j+1)%3]
			if dist[a].Sign() == dist[b].Sign() {
				continue
			}
			id := crossing(a, b)
			if dist[a].Sign() > 0 {
				start = id
			} else {
				end = id
			}
		}
		if start >= 0 {
			r.Segments = append(r.Segments, Segment{start, end, faces[ti], ti})
		}
	}
	return r, nil
}
