package iges

import (
	"context"
	"fmt"
	"io"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
)

// PlanarSolidOptions bounds the input and the independent exact material check.
type PlanarSolidOptions struct {
	Limits     Limits
	Validation collision.LocationOptions
}

// PlanarFaceSource identifies an original type 510 face and its owner entities.
// The first loop is the outer boundary. The remaining loops are holes.
type PlanarFaceSource struct {
	FaceID, PlaneID int
	LoopIDs         []int
	// ParameterCurves is present for affine spline supports. It follows
	// LoopIDs and the original use order in each file loop.
	ParameterCurves [][]int
}

// PlanarShellSource identifies one original type 514 shell and the half-open
// range of its faces in PlanarSolid.SourceFaces. The exterior precedes voids.
type PlanarShellSource struct {
	ShellID, FaceStart, FaceEnd int
	Void                        bool
}

// PlanarSolid transfers one IGES type 186, form 0 planar B-rep. SourceEdges
// has one original straight model curve per native edge. SourceFaces follows
// each original shell's face ordering. Coordinates remain in the file's units.
type PlanarSolid struct {
	Solid                    brep.Solid
	LengthUnit               float64
	SolidID, ShellID         int
	VertexListID, EdgeListID int
	SourceEdges              []int
	SourceFaces              []PlanarFaceSource
	SourceShells             []PlanarShellSource
}

const maxIGESPlanarVoids = 64

type igesPlanarVec [3]*big.Rat

func (a igesPlanarVec) sub(b igesPlanarVec) (r igesPlanarVec) {
	for i := range r {
		r[i] = new(big.Rat).Sub(a[i], b[i])
	}
	return r
}

func (a igesPlanarVec) cross(b igesPlanarVec) (r igesPlanarVec) {
	for i := range r {
		j, k := (i+1)%3, (i+2)%3
		r[i] = new(big.Rat).Sub(new(big.Rat).Mul(a[j], b[k]), new(big.Rat).Mul(a[k], b[j]))
	}
	return r
}

func (a igesPlanarVec) dot(b igesPlanarVec) *big.Rat {
	s := new(big.Rat)
	for i := range a {
		s.Add(s, new(big.Rat).Mul(a[i], b[i]))
	}
	return s
}

func (a igesPlanarVec) equal(b igesPlanarVec) bool {
	return a[0].Cmp(b[0]) == 0 && a[1].Cmp(b[1]) == 0 && a[2].Cmp(b[2]) == 0
}

func igesPlanarExact(v parameter) (*big.Rat, error) {
	s := strings.NewReplacer("D", "E", "d", "E").Replace(v.text)
	if v.stringValue || len(s) == 0 || len(s) > 2500 || !realLexeme(s) {
		return nil, geom.ErrUnrepresentable
	}
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		e, err := strconv.Atoi(s[i+1:])
		if err != nil || e < -4096 || e > 4096 {
			return nil, geom.ErrUnrepresentable
		}
	}
	q, ok := new(big.Rat).SetString(s)
	if !ok || q.Num().BitLen() > 8192 || q.Denom().BitLen() > 8192 {
		return nil, geom.ErrUnrepresentable
	}
	return q, nil
}

func igesPlanarCoordinates(a []parameter) (p igesPlanarVec, err error) {
	for i := range p {
		p[i], err = igesPlanarExact(a[i])
		if err != nil {
			return p, err
		}
	}
	return p, nil
}

func (p igesPlanarVec) represented() (v geom.Vec3, err error) {
	for i, x := range []*float64{&v.X, &v.Y, &v.Z} {
		*x, err = geom.RepresentedExactScalar(p[i])
		if err != nil {
			return v, err
		}
	}
	return v, nil
}

func igesPlanarFailure(id int, reason string) error {
	return fmt.Errorf("iges planar solid #%d: %s: %w", id, reason, geom.ErrDomain)
}

func igesPlanarCount(r topologyRecord, slot, start, stride, minimum int) (int, error) {
	if len(r.values) < start {
		return 0, igesPlanarFailure(r.id, "truncated entity")
	}
	n, err := intValue(r.values[slot])
	if err != nil || n < minimum || n > (len(r.values)-start)/stride {
		return 0, igesPlanarFailure(r.id, "invalid entity count")
	}
	return n, nil
}

type igesPlanarGraph struct {
	records map[int]topologyRecord
	used    map[int]bool
}

func (g igesPlanarGraph) take(id, typ int) (topologyRecord, error) {
	r, ok := g.records[id]
	if !ok || r.kind != typ || g.used[id] || len(r.values) == 0 {
		return topologyRecord{}, igesPlanarFailure(id, "missing, differently typed, or repeated source owner")
	}
	if kind, err := intValue(r.values[0]); err != nil || kind != typ {
		return topologyRecord{}, igesPlanarFailure(id, "parameter type disagrees with directory")
	}
	g.used[id] = true
	return r, nil
}

// ReadPlanarSolid transfers a closed planar material with one exterior shell
// and up to 64 direct void shells. Type 190 planes and affine degree-one
// type 128 charts are supported with exact straight model edges. Every
// affine chart coedge must have an exactly agreeing linear type 126 pcurve.
// Concave outer wires and through-holes are supported. Exact vertex
// agreement, orientation, embedding and each declared cavity are proved
// before publishing a solid. Assembly transformations are not supported.
func ReadPlanarSolid(ctx context.Context, r io.Reader, o PlanarSolidOptions) (PlanarSolid, error) {
	if ctx == nil {
		return PlanarSolid{}, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return PlanarSolid{}, err
	}
	d, err := readPlanarTopologySource(r, o.Limits)
	if err != nil {
		return PlanarSolid{}, err
	}
	return certifyIGESPlanarSolid(ctx, d, o.Validation)
}

func readPlanarTopologySource(r io.Reader, l Limits) (topologySource, error) {
	return readTopologyRecordsWithPlanarShells(r, l, true, false, false, false, false, false, false, true)
}

func certifyIGESPlanarSolid(ctx context.Context, d topologySource, o collision.LocationOptions) (PlanarSolid, error) {
	m := PlanarSolid{LengthUnit: d.unit}
	g := igesPlanarGraph{records: make(map[int]topologyRecord, len(d.entities)), used: make(map[int]bool, len(d.entities))}
	group := 0
	for _, r := range d.entities {
		g.records[r.id] = r
		var id *int
		switch r.kind {
		case 402:
			id = &group
		case 186:
			id = &m.SolidID
		case 502:
			id = &m.VertexListID
		case 504:
			id = &m.EdgeListID
		}
		if id != nil {
			if *id != 0 {
				return PlanarSolid{}, igesPlanarFailure(r.id, "multiple root, vertex or edge lists")
			}
			*id = r.id
		}
	}
	s, err := g.take(m.SolidID, 186)
	if err != nil {
		return PlanarSolid{}, err
	}
	if group != 0 {
		association, err := g.take(group, 402)
		if err != nil {
			return PlanarSolid{}, err
		}
		shape, err := topologyInts(association, 3)
		if err != nil || shape[1] != 1 || shape[2] != s.id {
			return PlanarSolid{}, igesPlanarFailure(group, "association must name the one material root")
		}
	}
	lists, err := g.planarLists(ctx, m.VertexListID, m.EdgeListID, false)
	if err != nil {
		return PlanarSolid{}, err
	}
	faces, err := g.planarMaterialFaces(ctx, s, &m, lists)
	if err != nil {
		return PlanarSolid{}, err
	}
	if len(g.used) != len(g.records) {
		return PlanarSolid{}, igesPlanarFailure(m.SolidID, "unclaimed source entities")
	}
	m.SourceEdges = lists.sourceEdges
	return verifyIGESPlanarMaterial(ctx, m, lists.vertices, lists.edges, faces, o)
}

type igesPlanarLists struct {
	vertices    []geom.Vec3
	exact       []igesPlanarVec
	edges       []brep.Edge
	sourceEdges []int
}

func (g igesPlanarGraph) planarLists(ctx context.Context, vertexList, edgeList int, allowCoincident bool) (igesPlanarLists, error) {
	vertexRecord, err := g.take(vertexList, 502)
	if err != nil {
		return igesPlanarLists{}, err
	}
	nv, err := igesPlanarCount(vertexRecord, 1, 2, 3, 4)
	if err != nil || noPointers(vertexRecord.values[2+3*nv:]) != nil {
		return igesPlanarLists{}, igesPlanarFailure(vertexRecord.id, "invalid vertex list")
	}
	vertices, exact := make([]geom.Vec3, nv), make([]igesPlanarVec, nv)
	positions := make(map[geom.Vec3]bool, nv)
	for i := range vertices {
		if err := ctx.Err(); err != nil {
			return igesPlanarLists{}, err
		}
		exact[i], err = igesPlanarCoordinates(vertexRecord.values[2+3*i : 5+3*i])
		if err == nil {
			vertices[i], err = exact[i].represented()
		}
		if err != nil {
			return igesPlanarLists{}, fmt.Errorf("iges planar vertex %d: %w", i+1, err)
		}
		if !allowCoincident && positions[vertices[i]] {
			return igesPlanarLists{}, igesPlanarFailure(vertexRecord.id, "distinct vertices share an exact location")
		}
		positions[vertices[i]] = true
	}
	edgeRecord, err := g.take(edgeList, 504)
	if err != nil {
		return igesPlanarLists{}, err
	}
	ne, err := igesPlanarCount(edgeRecord, 1, 2, 5, 6)
	if err != nil {
		return igesPlanarLists{}, err
	}
	ev, err := topologyInts(edgeRecord, 2+5*ne)
	if err != nil {
		return igesPlanarLists{}, err
	}
	edges := make([]brep.Edge, ne)
	sourceEdges := make([]int, ne)
	pairs := make(map[[2]int]bool, ne)
	for i := range edges {
		if err := ctx.Err(); err != nil {
			return igesPlanarLists{}, err
		}
		a := ev[2+5*i : 7+5*i]
		if a[1] != vertexList || a[3] != vertexList || a[2] < 1 || a[2] > nv || a[4] < 1 || a[4] > nv || a[2] == a[4] {
			return igesPlanarLists{}, igesPlanarFailure(edgeRecord.id, "invalid model-edge vertices")
		}
		key := [2]int{min(a[2], a[4]), max(a[2], a[4])}
		if pairs[key] {
			return igesPlanarLists{}, igesPlanarFailure(edgeRecord.id, "duplicate model-edge endpoints")
		}
		pairs[key] = true
		edges[i] = brep.Edge{Start: a[2] - 1, End: a[4] - 1}
		sourceEdges[i] = a[0]
		line, err := g.planarModelSegment(a[0])
		if err != nil {
			return igesPlanarLists{}, err
		}
		if !line[0].equal(exact[edges[i].Start]) {
			return igesPlanarLists{}, igesPlanarFailure(a[0], "original line start disagrees with edge vertex")
		}
		if !line[1].equal(exact[edges[i].End]) {
			return igesPlanarLists{}, igesPlanarFailure(a[0], "original line end disagrees with edge vertex")
		}
	}
	return igesPlanarLists{vertices, exact, edges, sourceEdges}, nil
}

func (g igesPlanarGraph) planarMaterialFaces(ctx context.Context, s topologyRecord, m *PlanarSolid, lists igesPlanarLists) ([]brep.Face, error) {
	nvoid, err := igesPlanarCount(s, 3, 4, 2, 0)
	if err != nil || nvoid > maxIGESPlanarVoids {
		return nil, igesPlanarFailure(s.id, "invalid void shell count")
	}
	root, err := topologyInts(s, 4+2*nvoid)
	if err != nil {
		return nil, err
	}
	shellRefs := [][2]int{{root[1], root[2]}}
	for i := range nvoid {
		shellRefs = append(shellRefs, [2]int{root[4+2*i], root[5+2*i]})
	}
	m.ShellID = root[1]
	var faces []brep.Face
	for i, ref := range shellRefs {
		if ref[1] != 0 && ref[1] != 1 {
			return nil, igesPlanarFailure(ref[0], "invalid shell orientation")
		}
		fs, source, err := g.planarShell(ctx, ref[0], m.EdgeListID, lists.exact, lists.edges, ref[1] == 0)
		if err != nil {
			return nil, err
		}
		m.SourceShells = append(m.SourceShells, PlanarShellSource{ref[0], len(faces), len(faces) + len(fs), i != 0})
		faces, m.SourceFaces = append(faces, fs...), append(m.SourceFaces, source...)
	}
	return faces, nil
}

func verifyIGESPlanarMaterial(ctx context.Context, m PlanarSolid, vertices []geom.Vec3, edges []brep.Edge, faces []brep.Face, o collision.LocationOptions) (PlanarSolid, error) {
	solid, err := brep.NewSolid(vertices, edges, faces)
	if err != nil {
		return PlanarSolid{}, err
	}
	m.Solid = solid
	shells, err := igesPlanarEmbeddedShells(ctx, m.Solid, o)
	if err != nil {
		return PlanarSolid{}, err
	}
	if len(shells) != len(m.SourceShells) {
		return PlanarSolid{}, igesPlanarFailure(m.SolidID, "declared shells disagree with embedded material")
	}
	for i, shell := range m.SourceShells {
		if len(shells[i]) != shell.FaceEnd-shell.FaceStart {
			return PlanarSolid{}, igesPlanarFailure(shell.ShellID, "declared shell has different face membership")
		}
		for j, face := range shells[i] {
			if face != shell.FaceStart+j {
				return PlanarSolid{}, igesPlanarFailure(shell.ShellID, "declared shell has different face membership")
			}
		}
	}
	return m, nil
}

func (g igesPlanarGraph) planarShell(ctx context.Context, id, edgesID int, exact []igesPlanarVec, edges []brep.Edge, reversed bool) ([]brep.Face, []PlanarFaceSource, error) {
	shell, err := g.take(id, 514)
	if err != nil {
		return nil, nil, err
	}
	n, err := igesPlanarCount(shell, 1, 2, 2, 4)
	if err != nil {
		return nil, nil, err
	}
	sv, err := topologyInts(shell, 2+2*n)
	if err != nil {
		return nil, nil, err
	}
	faces, source := make([]brep.Face, n), make([]PlanarFaceSource, n)
	for i := range faces {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		sense := sv[3+2*i]
		if sense < 0 || sense > 1 {
			return nil, nil, igesPlanarFailure(id, "invalid face orientation")
		}
		faces[i], source[i], err = g.planarFace(ctx, sv[2+2*i], edgesID, exact, edges, reversed != (sense == 0))
		if err != nil {
			return nil, nil, err
		}
	}
	return faces, source, nil
}

func igesPlanarEmbeddedShells(ctx context.Context, s brep.Solid, o collision.LocationOptions) ([][]int, error) {
	n := s.Topology().Shells
	if n < 1 || n > 1+maxIGESPlanarVoids {
		return nil, igesPlanarFailure(0, "unsupported embedded shell count")
	}
	triangleFaces := s.TriangleFaces()
	locator, err := collision.NewMeshPointLocator(ctx, s.Tessellate(), triangleFaces, o)
	if err != nil {
		return nil, fmt.Errorf("iges planar solid embedding: %w", err)
	}
	shells := locator.Shells()
	outer := -1
	for i, shell := range shells {
		if shell.Depth == 0 {
			if outer != -1 {
				return nil, igesPlanarFailure(0, "material has more than one exterior")
			}
			outer = i
		}
	}
	if outer < 0 {
		return nil, igesPlanarFailure(0, "material has no exterior")
	}
	result := make([][]int, 1, len(shells))
	faceOwners := make(map[int]int)
	for i, shell := range shells {
		if i != outer && (shell.Depth != 1 || shell.Parent != outer) {
			return nil, igesPlanarFailure(0, "every void must be directly enclosed by the exterior")
		}
		var faces []int
		for _, tri := range shell.Triangles {
			face := triangleFaces[tri]
			owner, seen := faceOwners[face]
			if seen && owner != i {
				return nil, igesPlanarFailure(0, "face belongs to more than one embedded shell")
			}
			if !seen {
				faceOwners[face] = i
				faces = append(faces, face)
			}
		}
		sort.Ints(faces)
		if i == outer {
			result[0] = faces
		} else {
			result = append(result, faces)
		}
	}
	return result, nil
}

func (g igesPlanarGraph) datum(id, typ int) (igesPlanarVec, error) {
	r, err := g.take(id, typ)
	if err != nil {
		return igesPlanarVec{}, err
	}
	last := 4
	if typ == 116 {
		last = 5
	}
	if len(r.values) < last || noPointers(r.values[4:]) != nil {
		return igesPlanarVec{}, igesPlanarFailure(r.id, "invalid plane datum")
	}
	return igesPlanarCoordinates(r.values[1:4])
}

func (g igesPlanarGraph) planarFace(ctx context.Context, id, edgeList int, vertices []igesPlanarVec, edges []brep.Edge, reversed bool) (brep.Face, PlanarFaceSource, error) {
	fail := func(err error) (brep.Face, PlanarFaceSource, error) { return brep.Face{}, PlanarFaceSource{}, err }
	r, err := g.take(id, 510)
	if err != nil {
		return fail(err)
	}
	n, err := igesPlanarCount(r, 2, 4, 1, 1)
	if err != nil {
		return fail(err)
	}
	fv, err := topologyInts(r, 4+n)
	if err != nil || fv[3] != 1 {
		return fail(igesPlanarFailure(id, "explicit original outer loop required"))
	}
	source := PlanarFaceSource{FaceID: id, PlaneID: fv[1], LoopIDs: fv[4:]}
	support, err := g.planarFaceGeometry(source.PlaneID)
	if err != nil {
		return fail(err)
	}
	f := brep.Face{}
	for i, loop := range source.LoopIDs {
		pcurves := []int(nil)
		uses, err := g.planarLoop(ctx, loop, edgeList, vertices, edges, support, i == 0, &pcurves)
		if err != nil {
			return fail(err)
		}
		if support.chart != nil {
			source.ParameterCurves = append(source.ParameterCurves, pcurves)
		}
		if reversed {
			for j, k := 0, len(uses)-1; j < k; j, k = j+1, k-1 {
				uses[j], uses[k] = uses[k], uses[j]
			}
			for j := range uses {
				uses[j].Reversed = !uses[j].Reversed
			}
		}
		if i == 0 {
			f.Boundary = uses
		} else {
			f.Holes = append(f.Holes, uses)
		}
	}
	return f, source, nil
}

func (g igesPlanarGraph) planarLoop(ctx context.Context, id, edgeList int, vertices []igesPlanarVec, edges []brep.Edge, support igesPlanarFaceGeometry, outer bool, pcurves *[]int) ([]brep.EdgeUse, error) {
	r, err := g.take(id, 508)
	if err != nil {
		return nil, err
	}
	stride := 5
	if support.chart != nil {
		stride = 7
	}
	n, err := igesPlanarCount(r, 1, 2, stride, 3)
	if err != nil {
		return nil, err
	}
	v, err := topologyInts(r, 2+stride*n)
	if err != nil {
		return nil, err
	}
	uses, points := make([]brep.EdgeUse, n), make([]igesPlanarVec, n)
	prev, first := -1, -1
	seen := make(map[int]bool, n)
	for i := range uses {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		start := 2 + stride*i
		u := v[start : start+stride]
		if u[0] != 0 || u[1] != edgeList || u[2] < 1 || u[2] > len(edges) || u[3] < 0 || u[3] > 1 {
			return nil, igesPlanarFailure(id, "invalid directed loop incidence")
		}
		uses[i] = brep.EdgeUse{Edge: u[2] - 1, Reversed: u[3] == 0}
		e := edges[uses[i].Edge]
		if support.chart == nil {
			if u[4] != 0 {
				return nil, igesPlanarFailure(id, "explicit plane pcurves are outside this source dialect")
			}
		} else {
			if u[4] != 1 || u[5] != 0 {
				return nil, igesPlanarFailure(id, "affine chart use requires one independent linear pcurve")
			}
			trim, err := g.planarSplineSegment(u[6])
			if err != nil {
				return nil, err
			}
			for j, v := range []int{e.Start, e.End} {
				p, ok := support.lift(trim[j])
				if !ok || !p.equal(vertices[v]) {
					return nil, igesPlanarFailure(u[6], "pcurve does not exactly lift to the original directed edge")
				}
			}
			*pcurves = append(*pcurves, u[6])
		}
		start, end := e.Start, e.End
		if uses[i].Reversed {
			start, end = end, start
		}
		if seen[start] || prev != -1 && start != prev {
			return nil, igesPlanarFailure(id, "original loop is not one directed vertex cycle")
		}
		seen[start], prev = true, end
		if i == 0 {
			first = start
		}
		points[i] = vertices[start]
		if points[i].sub(support.origin).dot(support.normal).Sign() != 0 {
			return nil, igesPlanarFailure(id, "original face vertex does not lie on the declared plane exactly")
		}
	}
	if prev != first {
		return nil, igesPlanarFailure(id, "original loop does not close")
	}
	area := new(big.Rat)
	for i := 1; i+1 < n; i++ {
		u, v := points[i].sub(points[0]), points[i+1].sub(points[0])
		area.Add(area, u.cross(v).dot(support.normal))
	}
	if area.Sign() == 0 || (area.Sign() > 0) != outer {
		return nil, igesPlanarFailure(id, "original outer and inner orientations disagree with the plane")
	}
	return uses, nil
}
