package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type occtIGESDirectedEdge struct {
	from, to, curve int
	center          igesPlanarVec
	circular        bool
}

type occtIGESDirectedUse struct {
	edge, pcurve int
	forward      bool
}

type occtIGESDirectedLoop struct {
	id   int
	uses []occtIGESDirectedUse
}

type occtIGESDirectedFace struct {
	id, support int
	circular    bool
	reversed    bool
	origin      igesPlanarVec
	normal      igesPlanarVec
	x           igesPlanarVec
	radius      *big.Rat
	loops       []occtIGESDirectedLoop
}

// OCCTExactDirectedCircularLoopIGES includes the native model, original file
// and original IGES topology roles. Source use lists refer to the original
// edge list entries; the curve arrays retain directory pointers.
type OCCTExactDirectedCircularLoopIGES struct {
	ExactDirectedCircularLoopIGES
	Source                []byte
	SourceRimEdges        [2][]int
	SourceVerticalEdges   []int
	SourceParameterCurves map[int][]int
}

func occtIGESDirectedFailure(id int, detail string) error {
	return fmt.Errorf("iges: exact directed extrusion #%d: %s: %w", id, detail, geom.ErrDomain)
}

func occtIGESDirectedInts(g igesPlanarGraph, id, kind, n int) ([]int, error) {
	q, err := g.take(id, kind)
	if err != nil {
		return nil, err
	}
	if len(q.values) != n {
		return nil, occtIGESDirectedFailure(id, "unexpected parameter count")
	}
	return topologyInts(q, n)
}

func occtIGESDirectedVertices(g igesPlanarGraph, id int) ([]igesPlanarVec, []geom.Vec3, error) {
	r, err := g.take(id, 502)
	if err != nil {
		return nil, nil, err
	}
	n, err := igesPlanarCount(r, 1, 2, 3, 6)
	if err != nil || n > 128 || len(r.values) != 2+3*n {
		return nil, nil, occtIGESDirectedFailure(id, "unexpected vertex list")
	}
	exact, represented := make([]igesPlanarVec, n), make([]geom.Vec3, n)
	seen := map[geom.Vec3]bool{}
	for i := range exact {
		exact[i], err = igesPlanarCoordinates(r.values[2+3*i : 5+3*i])
		if err != nil {
			return nil, nil, err
		}
		represented[i], err = exact[i].represented()
		if err != nil {
			return nil, nil, err
		}
		if seen[represented[i]] {
			return nil, nil, occtIGESDirectedFailure(id, "duplicate model vertex")
		}
		seen[represented[i]] = true
	}
	return exact, represented, nil
}

func occtIGESDirectedCircle(g igesPlanarGraph, c topologyRecord) ([3]igesPlanarVec, error) {
	var points [3]igesPlanarVec
	if len(c.values) != 8 {
		return points, occtIGESDirectedFailure(c.id, "invalid model circle")
	}
	p := c.values
	for i := range points {
		q, err := igesPlanarCoordinates([]parameter{p[2+2*i], p[3+2*i], p[1]})
		if err != nil {
			return points, err
		}
		points[i] = q
	}
	if c.transform == 0 {
		return points, nil
	}
	t, err := g.take(c.transform, 124)
	if err != nil {
		return points, err
	}
	if len(t.values) != 13 {
		return points, occtIGESDirectedFailure(t.id, "invalid circle transform")
	}
	m := geom.Affine{}
	q := [3][4]*big.Rat{}
	for i := range q {
		for j := range q[i] {
			q[i][j], err = igesPlanarExact(t.values[1+4*i+j])
			if err == nil {
				m[i][j], err = geom.RepresentedExactScalar(q[i][j])
			}
			if err != nil {
				return points, err
			}
		}
	}
	if m[2][0] != 0 || m[2][1] != 0 || m[2][2] != 1 || m[0][2] != 0 || m[1][2] != 0 || exactRigidPlacement(&m) != nil {
		return points, occtIGESDirectedFailure(t.id, "circle transform is not proper rigid XY")
	}
	for i, p := range points {
		for j, row := range q {
			points[i][j] = new(big.Rat).Set(row[3])
			for k := 0; k < 3; k++ {
				points[i][j].Add(points[i][j], new(big.Rat).Mul(row[k], p[k]))
			}
		}
	}
	return points, nil
}

func occtIGESDirectedEdges(ctx context.Context, g igesPlanarGraph, id int) (int, []igesPlanarVec, []geom.Vec3, []occtIGESDirectedEdge, error) {
	fail := func(err error) (int, []igesPlanarVec, []geom.Vec3, []occtIGESDirectedEdge, error) {
		return 0, nil, nil, nil, err
	}
	r, err := g.take(id, 504)
	if err != nil {
		return fail(err)
	}
	n, err := igesPlanarCount(r, 1, 2, 5, 9)
	if err != nil || n > 192 || len(r.values) != 2+5*n {
		return fail(occtIGESDirectedFailure(id, "invalid model edge list"))
	}
	v, err := topologyInts(r, 2+5*n)
	if err != nil {
		return fail(err)
	}
	vertices := v[3]
	exact, represented, err := occtIGESDirectedVertices(g, vertices)
	if err != nil {
		return fail(err)
	}
	edges := make([]occtIGESDirectedEdge, n)
	for i := range edges {
		if err = ctx.Err(); err != nil {
			return fail(err)
		}
		k := v[2+5*i : 7+5*i]
		if k[1] != vertices || k[3] != vertices || min(k[2], k[4]) < 1 || max(k[2], k[4]) > len(exact) || k[2] == k[4] {
			return fail(occtIGESDirectedFailure(id, "invalid edge vertex ownership"))
		}
		e := occtIGESDirectedEdge{from: k[2] - 1, to: k[4] - 1, curve: k[0]}
		switch g.records[e.curve].kind {
		case 110:
			c, er := g.take(e.curve, 110)
			if er != nil {
				return fail(er)
			}
			if len(c.values) != 7 {
				return fail(occtIGESDirectedFailure(c.id, "invalid model line"))
			}
			p, ee := igesPlanarCoordinates(c.values[1:4])
			q, ef := igesPlanarCoordinates(c.values[4:7])
			if ee != nil || ef != nil || !p.equal(exact[e.from]) || !q.equal(exact[e.to]) {
				return fail(occtIGESDirectedFailure(c.id, "model line endpoints differ from original vertices"))
			}
		case 100:
			c, er := g.take(e.curve, 100)
			if er != nil {
				return fail(er)
			}
			p, er := occtIGESDirectedCircle(g, c)
			if er != nil {
				return fail(er)
			}
			if !p[1].equal(exact[e.from]) || !p[2].equal(exact[e.to]) {
				return fail(occtIGESDirectedFailure(c.id, "circular endpoints differ from original vertices"))
			}
			u, w := p[1].sub(p[0]), p[2].sub(p[0])
			if u[2].Sign() != 0 || w[2].Sign() != 0 || u.cross(w)[2].Sign() <= 0 || u.dot(w).Sign() != 0 || u.dot(u).Cmp(w.dot(w)) != 0 {
				return fail(occtIGESDirectedFailure(c.id, "source circle is not an exact positive quarter arc"))
			}
			e.center, e.circular = p[0], true
		default:
			return fail(occtIGESDirectedFailure(e.curve, "unsupported model edge"))
		}
		edges[i] = e
	}
	return vertices, exact, represented, edges, nil
}

// ReadOCCTExactDirectedCircularLoopExtrusion authenticates one original
// type-186 positive-Z extrusion. Circular rims must be exact quarter arcs;
// each cylindrical coedge has an explicit cardinal linear pcurve. Original
// type-124 arc transforms, face and loop identities, and descending or
// shifted cylinder charts are retained in the original source.
func ReadOCCTExactDirectedCircularLoopExtrusion(ctx context.Context, r io.Reader, l Limits) (OCCTExactDirectedCircularLoopIGES, error) {
	var zero OCCTExactDirectedCircularLoopIGES
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	l, err := l.defaults()
	if err != nil {
		return zero, err
	}
	l.MaxBytes = min(l.MaxBytes, 1<<20)
	l.MaxRecords = min(l.MaxRecords, 8192)
	l.MaxEntities = min(l.MaxEntities, 1024)
	l.MaxValues = min(l.MaxValues, 32768)
	doc, err := readTopologyRecordPolicy(r, l, true, false, false, false, false, false, true, false, true, true)
	if err != nil {
		return zero, err
	}
	g := igesPlanarGraph{records: make(map[int]topologyRecord, len(doc.entities)), used: map[int]bool{}}
	root, edgeList := 0, 0
	for _, a := range doc.entities {
		if a.kind != 100 && a.transform != 0 {
			return zero, occtIGESDirectedFailure(a.id, "only original circle transforms are supported")
		}
		switch a.kind {
		case 186:
			if root != 0 {
				return zero, occtIGESDirectedFailure(a.id, "second original solid")
			}
			root = a.id
		case 504:
			if edgeList != 0 {
				return zero, occtIGESDirectedFailure(a.id, "second original edge list")
			}
			edgeList = a.id
		}
		g.records[a.id] = a
	}
	m, err := occtIGESDirectedInts(g, root, 186, 4)
	if err != nil {
		return zero, err
	}
	if m[2] != 0 && m[2] != 1 || m[3] != 0 {
		return zero, occtIGESDirectedFailure(root, "invalid exterior shell orientation or void shells")
	}
	sh, err := g.take(m[1], 514)
	if err != nil {
		return zero, err
	}
	count, err := igesPlanarCount(sh, 1, 2, 2, 5)
	if err != nil || count > 66 || len(sh.values) != 2+2*count {
		return zero, occtIGESDirectedFailure(sh.id, "invalid source face count")
	}
	shell, err := topologyInts(sh, 2+2*count)
	if err != nil {
		return zero, err
	}
	vertexList, exact, points, edges, err := occtIGESDirectedEdges(ctx, g, edgeList)
	if err != nil {
		return zero, err
	}
	if len(points) != 2*(count-2) || len(edges) != 3*(count-2) {
		return zero, occtIGESDirectedFailure(edgeList, "not one cap-to-cap sweep")
	}
	faces := map[int]occtIGESDirectedFace{}
	owners := make([][]int, len(edges))
	var caps []occtIGESDirectedFace
	pcurves := map[int][]int{}
	for i := 0; i < count; i++ {
		id, orientation := shell[2+2*i], shell[3+2*i]
		if orientation != 0 && orientation != 1 {
			return zero, occtIGESDirectedFailure(sh.id, "invalid face orientation")
		}
		v, err := occtIGESDirectedFaceIDs(g, id)
		if err != nil {
			return zero, err
		}
		f, err := occtIGESDirectedReadFace(ctx, g, v, id, edgeList, m[2] != orientation, exact, edges)
		if err != nil {
			return zero, err
		}
		if !f.circular && f.normal[0].Sign() == 0 && f.normal[1].Sign() == 0 {
			caps = append(caps, f)
		}
		faces[id] = f
		for _, ring := range f.loops {
			for _, u := range ring.uses {
				owners[u.edge] = append(owners[u.edge], id)
				if u.pcurve != 0 {
					pcurves[id] = append(pcurves[id], u.pcurve)
				}
			}
		}
	}
	if len(g.used) != len(g.records) {
		return zero, occtIGESDirectedFailure(root, "unclaimed original directory entity")
	}
	if len(caps) != 2 || len(caps[0].loops) != len(caps[1].loops) {
		return zero, occtIGESDirectedFailure(sh.id, "two caps with matching ring counts are required")
	}
	if caps[0].origin[2].Cmp(caps[1].origin[2]) > 0 {
		caps[0], caps[1] = caps[1], caps[0]
	}
	d, flat, offsets, err := occtIGESDirectedDescriptor(caps, points, exact, edges, doc.unit)
	if err != nil {
		return zero, err
	}
	native, err := trimmed.ExtrudeExactDirectedCircularLoops(ctx, d.Outer, d.Holes, d.Bottom, d.Top, trimmed.ExactCircularLoopOptions{})
	if err != nil {
		return zero, err
	}
	out := OCCTExactDirectedCircularLoopIGES{Source: doc.bytes, SourceParameterCurves: pcurves, ExactDirectedCircularLoopIGES: ExactDirectedCircularLoopIGES{
		Descriptor: d, Native: native, SourceSolid: root, SourceShell: sh.id, SourceEdges: edgeList, SourceVertices: vertexList,
		SourceCaps: [2]int{caps[0].id, caps[1].id}, RingOffsets: offsets,
	}}
	if err = occtIGESDirectedBind(&out, flat, points, edges, faces, owners); err != nil {
		return zero, err
	}
	if err = ctx.Err(); err != nil {
		return zero, err
	}
	return out, nil
}

func occtIGESDirectedDescriptor(caps []occtIGESDirectedFace, points []geom.Vec3, exact []igesPlanarVec, edges []occtIGESDirectedEdge, unit float64) (ExactDirectedCircularLoopExtrusion, []trimmed.ExactCircularLoopSegment, []int, error) {
	var d ExactDirectedCircularLoopExtrusion
	fail := func(err error) (ExactDirectedCircularLoopExtrusion, []trimmed.ExactCircularLoopSegment, []int, error) {
		return ExactDirectedCircularLoopExtrusion{}, nil, nil, err
	}
	var err error
	d.Bottom, err = geom.RepresentedExactScalar(caps[0].origin[2])
	if err != nil {
		return fail(err)
	}
	d.Top, err = geom.RepresentedExactScalar(caps[1].origin[2])
	if err != nil {
		return fail(err)
	}
	d.Unit = unit
	var flat []trimmed.ExactCircularLoopSegment
	offsets := []int{0}
	for i, loop := range caps[0].loops {
		ring := make([]trimmed.ExactCircularLoopSegment, len(loop.uses))
		for j := range ring {
			// Reverse physical low-cap winding to define the source profile.
			u := occtIGESDirectedUseAt(loop.uses, j, !caps[0].reversed)
			e := edges[u.edge]
			p, q := e.from, e.to
			if !u.forward {
				p, q = q, p
			}
			s := trimmed.ExactCircularLoopSegment{Start: geom.Vec2{X: points[p].X, Y: points[p].Y}, End: geom.Vec2{X: points[q].X, Y: points[q].Y}}
			if e.circular {
				center, er := e.center.represented()
				if er != nil {
					return fail(er)
				}
				s.Center = geom.Vec2{X: center.X, Y: center.Y}
				delta := exact[p].sub(e.center)
				r2 := delta.dot(delta)
				num, den := new(big.Int).Sqrt(r2.Num()), new(big.Int).Sqrt(r2.Denom())
				r := new(big.Rat).SetFrac(num, den)
				if new(big.Rat).Mul(r, r).Cmp(r2) != 0 {
					return fail(occtIGESDirectedFailure(e.curve, "radius is not an exact rational scalar"))
				}
				s.Radius, err = geom.RepresentedExactScalar(r)
				if err != nil {
					return fail(err)
				}
			}
			ring[j] = s
		}
		if i == 0 {
			d.Outer = ring
		} else {
			d.Holes = append(d.Holes, ring)
		}
		flat = append(flat, ring...)
		offsets = append(offsets, len(flat))
	}
	return d, flat, offsets, nil
}

// WriteOCCTExactDirectedCircularLoopSource emits the original bytes only
// after independently re-proving their exact rings, model and trim owners.
func WriteOCCTExactDirectedCircularLoopSource(ctx context.Context, w io.Writer, source []byte, l Limits) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	m, err := ReadOCCTExactDirectedCircularLoopExtrusion(ctx, bytes.NewReader(source), l)
	if err != nil {
		return err
	}
	n, err := w.Write(m.Source)
	if err == nil && n != len(m.Source) {
		return io.ErrShortWrite
	}
	return err
}
