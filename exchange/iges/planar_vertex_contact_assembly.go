package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"
	"sort"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
)

// PlanarVertexContactAssemblyOptions bounds source data and the exact
// cross-component vertex-contact and disjointness proof.
type PlanarVertexContactAssemblyOptions struct {
	Limits Limits
	Proof  collision.VertexContactAssemblyOptions
}

// PlanarAssemblyBody retains original shared-list indices alongside the
// body's own native vertex and edge numbering. The IGES indices are one-based.
type PlanarAssemblyBody struct {
	PlanarSolid
	SourceVertexIndices, SourceEdgeIndices []int
}

// PlanarVertexContactAssembly contains separate IGES material roots and the
// complete graph of their isolated original vertex contacts. The declaration
// order is preserved. Contact vertex IDs are local to each native body.
type PlanarVertexContactAssembly struct {
	GroupID    int
	LengthUnit float64
	Bodies     []PlanarAssemblyBody
	Proof      collision.VertexContactAssemblyReport
}

// WritePlanarVertexContactAssembly writes a flat IGES group with separate
// material roots and distinct source list entries even at shared coordinates.
// It reopens the complete file and verifies topology, physical units and the
// contact ledger before publishing any bytes.
func WritePlanarVertexContactAssembly(ctx context.Context, w io.Writer, solids []brep.Solid, unit float64, o PlanarVertexContactAssemblyOptions) (collision.VertexContactAssemblyReport, error) {
	var zero collision.VertexContactAssemblyReport
	if ctx == nil || w == nil || len(solids) < 2 {
		return zero, geom.ErrInvalid
	}
	proof, err := collision.DiscoverVertexContactAssembly(ctx, solids, o.Proof)
	if err != nil {
		return zero, err
	}
	raw, expected, err := authorPlanarIGES(ctx, solids, unit, o.Limits, o.Proof.Validation)
	if err != nil {
		return zero, err
	}
	reopened, err := ReadPlanarVertexContactAssembly(ctx, bytes.NewReader(raw), o)
	if err != nil {
		return zero, fmt.Errorf("iges planar assembly readback: %w", err)
	}
	if reopened.LengthUnit != unit || len(reopened.Bodies) != len(solids) ||
		proof.SignedVolume.Cmp(reopened.Proof.SignedVolume) != 0 || !reflect.DeepEqual(proof.Contacts, reopened.Proof.Contacts) {
		return zero, igesPlanarFailure(0, "readback changed component identity, units or contact ledger")
	}
	for i, body := range reopened.Bodies {
		if !samePlanarIGESBody(solids[i], body.Solid, expected[i]) {
			return zero, igesPlanarFailure(body.SolidID, "readback changed original component topology")
		}
	}
	if err := publishPlanarIGES(ctx, w, raw); err != nil {
		return zero, err
	}
	return proof, nil
}

// ReadPlanarVertexContactAssembly proves a flat type 402 form 1 group of
// independently owned type 186 planar bodies. The group uses one shared
// vertex list and one shared edge list; membership of individual records is
// derived from the original directed coedges. Each body has one outward shell.
// Equal coordinates may occur in separate vertex records, but a source vertex
// or edge record may not be owned by two bodies. Boundary contact is accepted
// only at original vertices. Assemblies with transformations, void shells,
// or other kinds of geometric contact are outside this entry point.
func ReadPlanarVertexContactAssembly(ctx context.Context, r io.Reader, o PlanarVertexContactAssemblyOptions) (PlanarVertexContactAssembly, error) {
	if ctx == nil || o.Proof.MaxSolids < 0 {
		return PlanarVertexContactAssembly{}, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return PlanarVertexContactAssembly{}, err
	}
	d, err := readPlanarTopologySource(r, o.Limits)
	if err != nil {
		return PlanarVertexContactAssembly{}, err
	}
	return certifyIGESPlanarAssembly(ctx, d, o.Proof)
}

func certifyIGESPlanarAssembly(ctx context.Context, d topologySource, o collision.VertexContactAssemblyOptions) (PlanarVertexContactAssembly, error) {
	m := PlanarVertexContactAssembly{LengthUnit: d.unit}
	fail := func(e error) (PlanarVertexContactAssembly, error) { return PlanarVertexContactAssembly{}, e }
	g := igesPlanarGraph{records: make(map[int]topologyRecord, len(d.entities)), used: make(map[int]bool, len(d.entities))}
	vertexList, edgeList, roots := 0, 0, 0
	for _, r := range d.entities {
		g.records[r.id] = r
		var id *int
		switch r.kind {
		case 402:
			id = &m.GroupID
		case 502:
			id = &vertexList
		case 504:
			id = &edgeList
		case 186:
			roots++
		}
		if id != nil {
			if *id != 0 {
				return fail(igesPlanarFailure(r.id, "assembly requires one flat group and common vertex and edge lists"))
			}
			*id = r.id
		}
	}
	group, err := g.take(m.GroupID, 402)
	if err != nil {
		return fail(err)
	}
	n, err := igesPlanarCount(group, 1, 2, 1, 2)
	if err != nil || n != roots {
		return fail(igesPlanarFailure(group.id, "assembly group must account for all original solid roots"))
	}
	max := o.MaxSolids
	if max == 0 {
		max = 256
	}
	if n > max {
		return fail(collision.ErrLimit)
	}
	ids, err := topologyInts(group, 2+n)
	if err != nil {
		return fail(err)
	}
	lists, err := g.planarLists(ctx, vertexList, edgeList, true)
	if err != nil {
		return fail(err)
	}
	owners := igesPlanarAssemblyOwners{make(map[int]bool), make(map[int]bool)}
	solids := make([]brep.Solid, n)
	m.Bodies = make([]PlanarAssemblyBody, n)
	for i, id := range ids[2:] {
		root, err := g.take(id, 186)
		if err != nil {
			return fail(err)
		}
		body := PlanarSolid{LengthUnit: d.unit, SolidID: id, VertexListID: vertexList, EdgeListID: edgeList}
		faces, err := g.planarMaterialFaces(ctx, root, &body, lists)
		if err != nil {
			return fail(err)
		}
		if len(body.SourceShells) != 1 {
			return fail(igesPlanarFailure(id, "vertex-contact assembly bodies require one outward shell"))
		}
		m.Bodies[i], err = owners.extract(ctx, body, faces, lists, o.Validation)
		if err != nil {
			return fail(err)
		}
		solids[i] = m.Bodies[i].Solid
	}
	if len(g.used) != len(g.records) || len(owners.vertices) != len(lists.vertices) || len(owners.edges) != len(lists.edges) {
		return fail(igesPlanarFailure(group.id, "unowned source entities or list entries"))
	}
	m.Proof, err = collision.DiscoverVertexContactAssembly(ctx, solids, o)
	if err != nil {
		return fail(err)
	}
	return m, nil
}

type igesPlanarAssemblyOwners struct {
	vertices, edges map[int]bool
}

func (o igesPlanarAssemblyOwners) extract(ctx context.Context, body PlanarSolid, faces []brep.Face, lists igesPlanarLists, validation collision.LocationOptions) (PlanarAssemblyBody, error) {
	fail := func(e error) (PlanarAssemblyBody, error) { return PlanarAssemblyBody{}, e }
	memberEdges, memberVertices := make(map[int]bool), make(map[int]bool)
	var edgeOrder, vertexOrder []int
	for _, f := range faces {
		for _, loop := range append([][]brep.EdgeUse{f.Boundary}, f.Holes...) {
			for _, u := range loop {
				if err := ctx.Err(); err != nil {
					return fail(err)
				}
				if memberEdges[u.Edge] {
					continue
				}
				if o.edges[u.Edge] {
					return fail(igesPlanarFailure(body.SolidID, "edge list entry used by multiple bodies"))
				}
				memberEdges[u.Edge] = true
				edgeOrder = append(edgeOrder, u.Edge)
				edge := lists.edges[u.Edge]
				for _, v := range []int{edge.Start, edge.End} {
					if o.vertices[v] {
						return fail(igesPlanarFailure(body.SolidID, "vertex list entry used by multiple bodies"))
					}
					if !memberVertices[v] {
						memberVertices[v] = true
						vertexOrder = append(vertexOrder, v)
					}
				}
			}
		}
	}
	// Retain order from the file lists irrespective of face or loop traversal.
	sort.Ints(vertexOrder)
	sort.Ints(edgeOrder)
	vlocal, elocal := make(map[int]int, len(vertexOrder)), make(map[int]int, len(edgeOrder))
	vertices := make([]geom.Vec3, len(vertexOrder))
	positions := make(map[geom.Vec3]bool, len(vertexOrder))
	for i, v := range vertexOrder {
		p := lists.vertices[v]
		if positions[p] {
			return fail(igesPlanarFailure(body.SolidID, "distinct vertices share an exact location in one body"))
		}
		positions[p], vertices[i], vlocal[v], o.vertices[v] = true, p, i, true
		vertexOrder[i]++
	}
	edges := make([]brep.Edge, len(edgeOrder))
	body.SourceEdges = make([]int, len(edgeOrder))
	for i, e := range edgeOrder {
		edge := lists.edges[e]
		edges[i], elocal[e], o.edges[e] = brep.Edge{Start: vlocal[edge.Start], End: vlocal[edge.End]}, i, true
		body.SourceEdges[i] = lists.sourceEdges[e]
		edgeOrder[i]++
	}
	for _, f := range faces {
		for _, loop := range append([][]brep.EdgeUse{f.Boundary}, f.Holes...) {
			for i := range loop {
				loop[i].Edge = elocal[loop[i].Edge]
			}
		}
	}
	validated, err := verifyIGESPlanarMaterial(ctx, body, vertices, edges, faces, validation)
	if err != nil {
		return fail(fmt.Errorf("iges planar assembly body #%d: %w", body.SolidID, err))
	}
	return PlanarAssemblyBody{validated, vertexOrder, edgeOrder}, nil
}
