package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type occtLoopFace struct {
	bound, loop  int64
	oriented     []int64
	id, surface  int64
	sense        bool
	uses         []analyticUse
	kind         string
	origin, z, x [3]*big.Rat
	radius       *big.Rat
	trims        []occtLoopTrim
}
type occtLoopTrim struct {
	bound, loop int64
	oriented    []int64
	uses        []analyticUse
	outer       bool
}
type occtLoopEdge struct {
	id, a, b, model int64
	kind            string
	same            bool
}

func occtLoopTriple(f *File, id int64, kind string) ([3]*big.Rat, error) {
	var q [3]*big.Rat
	r, e := simple(f, id, kind, 2)
	if e != nil || r.Args[1].Kind != Aggregate || len(r.Args[1].Items) != 3 {
		return q, geom.ErrDomain
	}
	for i, v := range r.Args[1].Items {
		q[i], e = liftSourceRat(v)
		if e != nil || q[i].Num().BitLen() > 256 || q[i].Denom().BitLen() > 256 {
			return q, geom.ErrDomain
		}
	}
	return q, nil
}
func occtLoopSub(a, b [3]*big.Rat) (q [3]*big.Rat) {
	for i := range q {
		q[i] = new(big.Rat).Sub(a[i], b[i])
	}
	return
}
func occtLoopDot(a, b [3]*big.Rat) *big.Rat {
	q := new(big.Rat)
	for i := range a {
		q.Add(q, new(big.Rat).Mul(a[i], b[i]))
	}
	return q
}
func occtLoopParallel(a, b [3]*big.Rat) bool {
	if occtLoopDot(a, a).Sign() == 0 || occtLoopDot(b, b).Sign() == 0 {
		return false
	}
	for i := 0; i < 3; i++ {
		j := (i + 1) % 3
		if new(big.Rat).Mul(a[i], b[j]).Cmp(new(big.Rat).Mul(a[j], b[i])) != 0 {
			return false
		}
	}
	return true
}
func occtLoopVec(v geom.Vec3) [3]*big.Rat {
	return [3]*big.Rat{sideRat(v.X), sideRat(v.Y), sideRat(v.Z)}
}
func occtLoopFrame(f *File, id int64) (o, z, x [3]*big.Rat, e error) {
	r, e := simple(f, id, "AXIS2_PLACEMENT_3D", 4)
	if e != nil {
		return
	}
	o, e = occtLoopTriple(f, r.Args[1].Ref, "CARTESIAN_POINT")
	if e != nil {
		return
	}
	z, e = occtLoopTriple(f, r.Args[2].Ref, "DIRECTION")
	if e != nil {
		return
	}
	x, e = occtLoopTriple(f, r.Args[3].Ref, "DIRECTION")
	if e == nil && (occtLoopDot(z, z).Sign() == 0 || occtLoopDot(x, x).Sign() == 0 || occtLoopDot(z, x).Sign() != 0) {
		e = geom.ErrDomain
	}
	return
}
func occtLoopReadFace(f *File, id int64) (q occtLoopFace, e error) {
	return occtLoopReadFaceMode(f, id, false)
}

func occtLoopReadFaceMode(f *File, id int64, directed bool) (q occtLoopFace, e error) {
	q.id = id
	r, e := simple(f, id, "ADVANCED_FACE", 4)
	if e != nil {
		return
	}
	q.sense, e = boolean(r.Args[3])
	if e != nil {
		return
	}
	q.surface = r.Args[2].Ref
	en := f.Entities[q.surface]
	if len(en.Records) != 1 {
		e = geom.ErrDomain
		return
	}
	s := en.Records[0]
	q.kind = s.Type
	if (s.Type != "PLANE" || len(s.Args) != 2) && (s.Type != "CYLINDRICAL_SURFACE" || len(s.Args) != 3) {
		e = geom.ErrDomain
		return
	}
	q.origin, q.z, q.x, e = occtLoopFrame(f, s.Args[1].Ref)
	if e != nil {
		return
	}
	if s.Type == "CYLINDRICAL_SURFACE" {
		q.radius, e = liftSourceRat(s.Args[2])
		if e != nil || q.radius.Sign() <= 0 {
			e = geom.ErrDomain
			return
		}
	}
	bs, e := listRefs(r.Args[1], 1)
	if e != nil || (!directed && len(bs) != 1) || len(bs) > 16 {
		e = geom.ErrDomain
		return
	}
	for _, bid := range bs {
		trim, err := occtLoopReadTrim(f, bid)
		if err != nil {
			return q, err
		}
		q.trims = append(q.trims, trim)
	}
	q.bound, q.loop, q.oriented, q.uses = q.trims[0].bound, q.trims[0].loop, q.trims[0].oriented, q.trims[0].uses
	return
}

func occtLoopReadTrim(f *File, id int64) (q occtLoopTrim, e error) {
	q.bound = id
	b, e := simple(f, id, "FACE_BOUND", 3)
	if e != nil {
		b, e = simple(f, id, "FACE_OUTER_BOUND", 3)
		q.outer = e == nil
	}
	if e != nil {
		return
	}
	forward, e := boolean(b.Args[2])
	if e != nil {
		return
	}
	q.loop = b.Args[1].Ref
	loop, e := simple(f, b.Args[1].Ref, "EDGE_LOOP", 2)
	if e != nil {
		return
	}
	us, e := listRefs(loop.Args[1], 3)
	if e != nil || len(us) > 64 {
		e = geom.ErrDomain
		return
	}
	q.oriented = append([]int64(nil), us...)
	for _, uid := range us {
		u, err := simple(f, uid, "ORIENTED_EDGE", 5)
		if err != nil || u.Args[1].Kind != Derived || u.Args[2].Kind != Derived {
			e = geom.ErrDomain
			return
		}
		sense, err := boolean(u.Args[4])
		if err != nil {
			e = err
			return
		}
		q.uses = append(q.uses, analyticUse{u.Args[3].Ref, sense})
	}
	if !forward {
		for i, j := 0, len(q.uses)-1; i < j; i, j = i+1, j-1 {
			q.uses[i], q.uses[j] = q.uses[j], q.uses[i]
		}
		for i := range q.uses {
			q.uses[i].sense = !q.uses[i].sense
		}
	}
	return
}

// OCCTExactCircularLoopExtrusion retains a genuine single-product model-only
// AP214 source. The material is normalized to positive local Z height, with no
// inferred signed construction history. Original product and topology IDs stay
// separate from generated native identities.
type OCCTExactCircularLoopExtrusion struct {
	ExactCircularLoopExtrusionProduct
	Product AssemblyProduct
}

// ReadOCCTExactCircularLoopExtrusion proves the finite LINE/CIRCLE extrusion
// from original model edges and material-oriented loops. This explicit mode
// admits exact direction ratios and arbitrary coplanar frame origins, but only
// common Z-axis cylinders, binary-lossless source vertices/radii and one simple
// loop per face. It admits metre and millimetre SI units and OCCT's exact inch
// declaration. No tolerances or source pcurves are used to repair geometry.
func ReadOCCTExactCircularLoopExtrusion(ctx context.Context, r io.Reader, l Limits) (OCCTExactCircularLoopExtrusion, error) {
	a, body, l, e := readOCCTCircularLoopSource(ctx, r, l)
	if e != nil {
		return OCCTExactCircularLoopExtrusion{}, e
	}
	return authenticateOCCTLoopBody(ctx, a.Source, l, a.Products[0], a.LengthUnit, body)
}

func readOCCTCircularLoopSource(ctx context.Context, r io.Reader, l Limits) (Assembly, int64, Limits, error) {
	var zero Assembly
	if ctx == nil || r == nil {
		return zero, 0, l, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, 0, l, e
	}
	l, e := occurrenceColorSourceLimits(l)
	if e != nil {
		return zero, 0, l, e
	}
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return zero, 0, l, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return zero, 0, l, geom.ErrConvergence
	}
	f, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return zero, 0, l, e
	}
	var sdrID int64
	for _, id := range f.Order {
		if _, err := simple(f, id, "SHAPE_DEFINITION_REPRESENTATION", 2); err == nil {
			if sdrID != 0 {
				return zero, 0, l, geom.ErrDomain
			}
			sdrID = id
		}
	}
	if sdrID == 0 {
		return zero, 0, l, geom.ErrDomain
	}
	pid, pd, name, e := independentMaterialProduct(f, sdrID)
	if e != nil {
		return zero, 0, l, e
	}
	sdr, _ := simple(f, sdrID, "SHAPE_DEFINITION_REPRESENTATION", 2)
	repID := sdr.Args[1].Ref
	rep, e := simple(f, repID, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
	if e != nil {
		return zero, 0, l, e
	}
	unit, e := contextUnits(f, rep.Args[2].Ref)
	if e != nil {
		return zero, 0, l, e
	}
	product, _ := simple(f, pid, "PRODUCT", 4)
	identifier, _ := textValue(product.Args[0])
	description, _ := textValue(product.Args[2])
	a := Assembly{Source: f, LengthUnit: unit, Products: []AssemblyProduct{{DefinitionID: pd, ProductID: pid, Identifier: identifier, Name: name, Description: description, Representations: []int64{repID}}}, Roots: []int64{pd}}
	bodyID, e := occtLoopProductGraph(f, a)
	if e != nil {
		return zero, 0, l, fmt.Errorf("step: exact loop product: %w", e)
	}
	return a, bodyID, l, nil
}

// authenticateOCCTLoopBody requires independently authenticated ownership and units.
func authenticateOCCTLoopBody(ctx context.Context, f *File, l Limits, product AssemblyProduct, unit float64, bodyID int64) (OCCTExactCircularLoopExtrusion, error) {
	var zero OCCTExactCircularLoopExtrusion
	m, e := readOCCTLoopModel(ctx, f, l, bodyID, false)
	if e != nil {
		return zero, e
	}
	lo := m.faces[m.lo]
	n := len(lo.uses)
	if len(m.faces[m.hi].uses) != n {
		return zero, geom.ErrDomain
	}
	z0, height := m.z0, m.height
	frame, _ := geom.NewFrame(geom.Vec3{Z: z0}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	out := OCCTExactCircularLoopExtrusion{Product: product}
	out.Source = f
	out.Body = bodyID
	out.Shell = m.shell
	out.Caps = [2]int64{m.lo, m.hi}
	out.Descriptor = ExactCircularLoopExtrusion{Name: product.Name, Frame: frame, Height: height, LengthUnit: unit}
	// Reverse the outward low cap cycle to obtain the required CCW source loop.
	for i := n - 1; i >= 0; i-- {
		u := lo.uses[i]
		edge := m.edges[u.id]
		start, end := edge.b, edge.a
		if !u.sense {
			start, end = edge.a, edge.b
		}
		p, q := m.points[start], m.points[end]
		if p.Z != z0 || q.Z != z0 {
			return zero, geom.ErrDomain
		}
		seg := trimmed.ExactCircularLoopSegment{Start: geom.Vec2{X: p.X, Y: p.Y}, End: geom.Vec2{X: q.X, Y: q.Y}}
		if edge.kind == "CIRCLE" {
			c, err := simple(f, edge.model, "CIRCLE", 3)
			if err != nil {
				return zero, err
			}
			o, z, x, e := occtLoopFrame(f, c.Args[1].Ref)
			if e != nil || !occtLoopPositiveAxis(z, 2) || !occtLoopPositiveAxis(x, 0) || o[2].Cmp(lo.origin[2]) != 0 {
				return zero, geom.ErrDomain
			}
			center, e := obliqueNURBSFloatPoint(o)
			if e != nil {
				return zero, e
			}
			rad, e := cylinderSupportNumber(c.Args[2])
			if e != nil || rad <= 0 {
				return zero, geom.ErrDomain
			}
			seg.Center = geom.Vec2{X: center.X, Y: center.Y}
			seg.Radius = rad
		}
		out.Descriptor.Segments = append(out.Descriptor.Segments, seg)
	}
	native, e := trimmed.ExtrudeExactCircularLoop(ctx, out.Descriptor.Segments, height, trimmed.ExactCircularLoopOptions{})
	if e != nil {
		return zero, e
	}
	out.Native = native
	if e = occtLoopBindNative(f, &out, m.faces, m.edges, m.points, m.owners); e != nil {
		return zero, fmt.Errorf("step: exact loop native binding: %w", e)
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}

type occtLoopModel struct {
	shell, lo, hi int64
	faces         map[int64]occtLoopFace
	edges         map[int64]occtLoopEdge
	points        map[int64]geom.Vec3
	owners        map[int64][]int64
	z0, height    float64
}

func readOCCTLoopModel(ctx context.Context, f *File, l Limits, bodyID int64, directed bool) (occtLoopModel, error) {
	var zero occtLoopModel
	body, e := simple(f, bodyID, "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return zero, e
	}
	shell, e := simple(f, body.Args[1].Ref, "CLOSED_SHELL", 2)
	if e != nil {
		return zero, e
	}
	ids, e := listRefs(shell.Args[1], 5)
	if e != nil || len(ids) > 66 || len(ids) > l.MaxFaces {
		return zero, geom.ErrDomain
	}
	faces := map[int64]occtLoopFace{}
	edges := map[int64]occtLoopEdge{}
	points := map[int64]geom.Vec3{}
	owners := map[int64][]int64{}
	uses := map[int64][]bool{}
	var capIDs []int64
	axis := occtLoopVec(geom.Vec3{Z: 1})
	ownedUses := map[int64]bool{}
	for _, fid := range ids {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		if _, ok := faces[fid]; ok {
			return zero, geom.ErrDomain
		}
		face, e := occtLoopReadFaceMode(f, fid, directed)
		if e != nil {
			return zero, e
		}
		for _, t := range face.trims {
			for _, id := range append([]int64{t.bound, t.loop}, t.oriented...) {
				if ownedUses[id] {
					return zero, geom.ErrDomain
				}
				ownedUses[id] = true
			}
		}
		faces[fid] = face
		if face.kind == "PLANE" && occtLoopParallel(face.z, axis) {
			capIDs = append(capIDs, fid)
		}
		for _, t := range face.trims {
			for _, u := range t.uses {
				owners[u.id] = append(owners[u.id], fid)
				uses[u.id] = append(uses[u.id], u.sense)
				if _, ok := edges[u.id]; ok {
					continue
				}
				er, e := simple(f, u.id, "EDGE_CURVE", 5)
				if e != nil {
					return zero, e
				}
				same, e := boolean(er.Args[4])
				if e != nil || (!directed && !same) {
					return zero, geom.ErrDomain
				}
				curve := f.Entities[er.Args[3].Ref]
				if len(curve.Records) != 1 || (curve.Records[0].Type != "LINE" && curve.Records[0].Type != "CIRCLE") {
					return zero, geom.ErrDomain
				}
				edges[u.id] = occtLoopEdge{u.id, er.Args[1].Ref, er.Args[2].Ref, er.Args[3].Ref, curve.Records[0].Type, same}
				for _, vid := range []int64{er.Args[1].Ref, er.Args[2].Ref} {
					vr, e := simple(f, vid, "VERTEX_POINT", 2)
					if e != nil {
						return zero, e
					}
					v, e := occtLoopTriple(f, vr.Args[1].Ref, "CARTESIAN_POINT")
					if e != nil {
						return zero, e
					}
					p, e := obliqueNURBSFloatPoint(v)
					if e != nil {
						return zero, e
					}
					points[vid] = p
				}
			}
		}
	}
	if len(points) > l.MaxVertices || len(edges)*2 > l.MaxEdgeUses {
		return zero, geom.ErrConvergence
	}
	if len(capIDs) != 2 {
		return zero, geom.ErrDomain
	}
	if faces[capIDs[0]].origin[2].Cmp(faces[capIDs[1]].origin[2]) > 0 {
		capIDs[0], capIDs[1] = capIDs[1], capIDs[0]
	}
	lo, hi := faces[capIDs[0]], faces[capIDs[1]]
	z0, ok := lo.origin[2].Float64()
	if !ok {
		return zero, geom.ErrUnrepresentable
	}
	height, ok := new(big.Rat).Sub(hi.origin[2], lo.origin[2]).Float64()
	if !ok || height <= 0 {
		return zero, geom.ErrDomain
	}
	n := 0
	for _, t := range lo.trims {
		n += len(t.uses)
	}
	if len(hi.trims) != len(lo.trims) || n < 3 || len(ids) != n+2 || len(edges) != 3*n || len(points) != 2*n {
		return zero, geom.ErrDomain
	}
	for eid, os := range owners {
		if len(os) != 2 || os[0] == os[1] || uses[eid][0] == uses[eid][1] {
			return zero, geom.ErrDomain
		}
	}
	for fid, face := range faces {
		if fid != capIDs[0] && fid != capIDs[1] && len(face.trims) != 1 {
			return zero, geom.ErrDomain
		}
		for _, t := range face.trims {
			for i, u := range t.uses {
				nextUse := t.uses[(i+1)%len(t.uses)]
				edge, next := edges[u.id], edges[nextUse.id]
				end, start := edge.b, next.a
				if !u.sense {
					end = edge.a
				}
				if !nextUse.sense {
					start = next.b
				}
				if end != start {
					return zero, geom.ErrDomain
				}
			}
		}
	}
	return occtLoopModel{body.Args[1].Ref, capIDs[0], capIDs[1], faces, edges, points, owners, z0, height}, nil
}
func occtLoopPositiveAxis(v [3]*big.Rat, k int) bool {
	for j, q := range v {
		if j == k {
			if q.Sign() <= 0 {
				return false
			}
		} else if q.Sign() != 0 {
			return false
		}
	}
	return true
}

func occtLoopBindNative(f *File, out *OCCTExactCircularLoopExtrusion, faces map[int64]occtLoopFace, edges map[int64]occtLoopEdge, points map[int64]geom.Vec3, owners map[int64][]int64) error {
	m := occtLoopModel{faces: faces, edges: edges, points: points, owners: owners}
	b := ExactDirectedCircularLoopExtrusionProduct{
		Descriptor: ExactDirectedCircularLoopExtrusion{Frame: out.Descriptor.Frame},
		Native:     out.Native, Caps: out.Caps,
	}
	if err := occtLoopBindSource(f, &b, out.Descriptor.Segments, m, false); err != nil {
		return err
	}
	out.Walls, out.VerticalEdges, out.Rims, out.Vertices = b.Walls, b.VerticalEdges, b.Rims, b.Vertices
	return nil
}

func occtLoopBindSource(f *File, out *ExactDirectedCircularLoopExtrusionProduct, segments []trimmed.ExactCircularLoopSegment, m occtLoopModel, directed bool) error {
	faces, edges, points, owners := m.faces, m.edges, m.points, m.owners
	vs, es, fs := out.Native.Vertices(), out.Native.Edges(), out.Native.Faces()
	n := len(segments)
	vertexIDs := make([]int64, len(vs))
	world := make([]geom.Vec3, len(vs))
	seen := map[int64]bool{}
	for i, p := range vs {
		q, e := sideDiskWorld(out.Descriptor.Frame, p)
		if e != nil {
			return e
		}
		world[i] = q
		for id, v := range points {
			if v == q {
				if vertexIDs[i] != 0 {
					return geom.ErrDomain
				}
				vertexIDs[i] = id
			}
		}
		if vertexIDs[i] == 0 || seen[vertexIDs[i]] {
			return geom.ErrDomain
		}
		seen[vertexIDs[i]] = true
	}
	edgeIDs := make([]int64, len(es))
	forward := make([]bool, len(es))
	seen = map[int64]bool{}
	for i, edge := range es {
		a, b := vertexIDs[edge.Start], vertexIDs[edge.End]
		for id, s := range edges {
			if (s.a == a && s.b == b) || (s.a == b && s.b == a) {
				if edgeIDs[i] != 0 {
					return geom.ErrDomain
				}
				edgeIDs[i] = id
				forward[i] = s.a == a
			}
		}
		id := edgeIDs[i]
		if id == 0 || seen[id] {
			return geom.ErrDomain
		}
		seen[id] = true
		s := edges[id]
		start, end := occtLoopVec(points[s.a]), occtLoopVec(points[s.b])
		curve := f.Entities[s.model].Records[0]
		circle := i < 2*n && segments[i%n].Radius > 0
		if circle {
			if s.kind != "CIRCLE" || (!directed && !forward[i]) || len(curve.Args) != 3 {
				return geom.ErrDomain
			}
			seg := segments[i%n]
			o, z, x, e := occtLoopFrame(f, curve.Args[1].Ref)
			if e != nil || (!directed && (!occtLoopPositiveAxis(z, 2) || !occtLoopPositiveAxis(x, 0))) || !occtLoopParallel(z, occtLoopVec(geom.Vec3{Z: 1})) || o[0].Cmp(sideRat(seg.Center.X)) != 0 || o[1].Cmp(sideRat(seg.Center.Y)) != 0 || o[2].Cmp(start[2]) != 0 {
				return geom.ErrDomain
			}
			if directed {
				// SameSense states whether the finite original edge follows the
				// circle's model axis; disagreement selects the major arc.
				positive := z[2].Sign() > 0
				if !s.same {
					positive = !positive
				}
				if !forward[i] {
					positive = !positive
				}
				if (occtLoopArcCross(seg) > 0) != positive {
					return geom.ErrDomain
				}
			}
			radius, e := liftSourceRat(curve.Args[2])
			if e != nil || radius.Cmp(sideRat(seg.Radius)) != 0 {
				return geom.ErrDomain
			}
		} else {
			if s.kind != "LINE" || len(curve.Args) != 3 {
				return geom.ErrDomain
			}
			o, e := occtLoopTriple(f, curve.Args[1].Ref, "CARTESIAN_POINT")
			if e != nil {
				return e
			}
			v, e := simple(f, curve.Args[2].Ref, "VECTOR", 3)
			if e != nil {
				return e
			}
			d, e := occtLoopTriple(f, v.Args[1].Ref, "DIRECTION")
			if e != nil {
				return e
			}
			mag, e := liftSourceRat(v.Args[2])
			delta := occtLoopSub(end, start)
			if e != nil || mag.Sign() <= 0 || !occtLoopParallel(d, delta) || (occtLoopDot(d, delta).Sign() > 0) != s.same {
				return geom.ErrDomain
			}
			offset := occtLoopSub(start, o)
			if occtLoopDot(offset, offset).Sign() != 0 && !occtLoopParallel(offset, d) {
				return geom.ErrDomain
			}
		}
	}
	out.Rims = [2][]int64{append([]int64(nil), edgeIDs[:n]...), append([]int64(nil), edgeIDs[n:2*n]...)}
	out.VerticalEdges = append([]int64(nil), edgeIDs[2*n:]...)
	out.Vertices = [2][]int64{append([]int64(nil), vertexIDs[:n]...), append([]int64(nil), vertexIDs[n:]...)}
	faceIDs := []int64{out.Caps[0], out.Caps[1]}
	seen = map[int64]bool{out.Caps[0]: true, out.Caps[1]: true}
	for i := 0; i < n; i++ {
		var wall int64
		for _, fid := range owners[edgeIDs[i]] {
			if fid != out.Caps[0] {
				wall = fid
			}
		}
		if wall == 0 || seen[wall] {
			return geom.ErrDomain
		}
		seen[wall] = true
		faceIDs = append(faceIDs, wall)
		out.Walls = append(out.Walls, wall)
	}
	if len(seen) != len(faces) {
		return geom.ErrDomain
	}
	for i, native := range fs {
		source := faces[faceIDs[i]]
		if !occtLoopMatchTrims(native, source, edgeIDs, forward, directed) {
			return fmt.Errorf("face #%d trims: %w", source.id, geom.ErrDomain)
		}
		if i >= 2 && segments[i-2].Radius > 0 {
			seg := segments[i-2]
			if source.kind != "CYLINDRICAL_SURFACE" || source.sense != (!directed || occtLoopArcCross(seg) > 0) || !occtLoopParallel(source.z, occtLoopVec(geom.Vec3{Z: 1})) || (!directed && !occtLoopPositiveAxis(source.x, 0)) || source.origin[0].Cmp(sideRat(seg.Center.X)) != 0 || source.origin[1].Cmp(sideRat(seg.Center.Y)) != 0 || source.radius.Cmp(sideRat(seg.Radius)) != 0 {
				return geom.ErrDomain
			}
		} else {
			if source.kind != "PLANE" {
				return geom.ErrDomain
			}
			normal := occtLoopVec(geom.Vec3{Z: 1})
			if i == 0 {
				normal[2].Neg(normal[2])
			}
			if i >= 2 {
				seg := segments[i-2]
				normal = [3]*big.Rat{new(big.Rat).Sub(sideRat(seg.End.Y), sideRat(seg.Start.Y)), new(big.Rat).Sub(sideRat(seg.Start.X), sideRat(seg.End.X)), new(big.Rat)}
			}
			if !occtLoopParallel(normal, source.z) {
				return geom.ErrDomain
			}
			sense := occtLoopDot(normal, source.z).Sign() > 0
			if sense != source.sense {
				return geom.ErrDomain
			}
			for _, ring := range append([][]trimmed.Use{native.Outer}, native.Holes...) {
				for _, u := range ring {
					p := occtLoopVec(world[es[u.Edge].Start])
					if occtLoopDot(occtLoopSub(p, source.origin), source.z).Sign() != 0 {
						return geom.ErrDomain
					}
				}
			}
		}
	}
	return nil
}

// occtLoopArcCross gives the exact orientation of two endpoint radii.
func occtLoopArcCross(seg trimmed.ExactCircularLoopSegment) int {
	ax := new(big.Rat).Sub(sideRat(seg.Start.X), sideRat(seg.Center.X))
	ay := new(big.Rat).Sub(sideRat(seg.Start.Y), sideRat(seg.Center.Y))
	bx := new(big.Rat).Sub(sideRat(seg.End.X), sideRat(seg.Center.X))
	by := new(big.Rat).Sub(sideRat(seg.End.Y), sideRat(seg.Center.Y))
	return new(big.Rat).Sub(ax.Mul(ax, by), ay.Mul(ay, bx)).Sign()
}

func occtLoopMatchTrims(native trimmed.Face, source occtLoopFace, edges []int64, forward []bool, directed bool) bool {
	rings := append([][]trimmed.Use{native.Outer}, native.Holes...)
	if len(rings) != len(source.trims) {
		return false
	}
	used := make([]bool, len(rings))
	explicit := false
	for _, q := range source.trims {
		explicit = explicit || q.outer
	}
	for i, ring := range rings {
		expected := make([]analyticUse, len(ring))
		for j, u := range ring {
			k := j
			if native.Reversed {
				k = len(ring) - 1 - j
			}
			expected[k] = analyticUse{edges[u.Edge], forward[u.Edge] != (u.Reversed != native.Reversed)}
		}
		match := false
		for k, q := range source.trims {
			if used[k] || (directed && explicit && q.outer != (i == 0)) || len(q.uses) != len(expected) {
				continue
			}
			for shift := range expected {
				ok := true
				for j, u := range expected {
					if u != q.uses[(j+shift)%len(expected)] {
						ok = false
						break
					}
				}
				if ok {
					used[k], match = true, true
					break
				}
			}
			if match {
				break
			}
		}
		if !match {
			return false
		}
	}
	return true
}

func occtLoopProductGraph(f *File, a Assembly) (int64, error) {
	bad := func() (int64, error) { return 0, geom.ErrDomain }
	repID := a.Products[0].Representations[0]
	rep, e := simple(f, repID, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
	if e != nil {
		return bad()
	}
	items, e := listRefs(rep.Args[1], 2)
	if e != nil || len(items) != 2 {
		return bad()
	}
	neutral, e := sideDiskReadFrame(f, items[0])
	if e != nil || frameTransform(neutral) != geom.Identity() {
		return bad()
	}
	if _, e = simple(f, items[1], "MANIFOLD_SOLID_BREP", 2); e != nil {
		return bad()
	}
	inches := a.LengthUnit == .0254
	if !inches && a.LengthUnit != .001 && a.LengthUnit != 1 {
		return bad()
	}
	allowed := map[string]bool{}
	for _, k := range []string{"APPLICATION_PROTOCOL_DEFINITION", "APPLICATION_CONTEXT", "SHAPE_DEFINITION_REPRESENTATION", "PRODUCT_DEFINITION_SHAPE", "PRODUCT_DEFINITION", "PRODUCT_DEFINITION_FORMATION", "PRODUCT", "PRODUCT_CONTEXT", "PRODUCT_DEFINITION_CONTEXT", "PRODUCT_RELATED_PRODUCT_CATEGORY", "ADVANCED_BREP_SHAPE_REPRESENTATION", "AXIS2_PLACEMENT_3D", "CARTESIAN_POINT", "DIRECTION", "MANIFOLD_SOLID_BREP", "CLOSED_SHELL", "ADVANCED_FACE", "FACE_BOUND", "FACE_OUTER_BOUND", "EDGE_LOOP", "ORIENTED_EDGE", "EDGE_CURVE", "VERTEX_POINT", "LINE", "VECTOR", "CIRCLE", "PLANE", "CYLINDRICAL_SURFACE", "GEOMETRIC_REPRESENTATION_CONTEXT", "GLOBAL_UNCERTAINTY_ASSIGNED_CONTEXT", "GLOBAL_UNIT_ASSIGNED_CONTEXT", "REPRESENTATION_CONTEXT", "LENGTH_UNIT", "NAMED_UNIT", "SI_UNIT", "PLANE_ANGLE_UNIT", "SOLID_ANGLE_UNIT", "UNCERTAINTY_MEASURE_WITH_UNIT"} {
		allowed[k] = true
	}
	inchRecords := []string{"CONVERSION_BASED_UNIT", "DIMENSIONAL_EXPONENTS", "LENGTH_MEASURE_WITH_UNIT"}
	if inches {
		for _, k := range inchRecords {
			allowed[k] = true
		}
	}
	geometryNames := map[string]bool{}
	for _, k := range []string{"AXIS2_PLACEMENT_3D", "CARTESIAN_POINT", "DIRECTION", "MANIFOLD_SOLID_BREP", "CLOSED_SHELL", "ADVANCED_FACE", "FACE_BOUND", "FACE_OUTER_BOUND", "EDGE_LOOP", "ORIENTED_EDGE", "EDGE_CURVE", "VERTEX_POINT", "LINE", "VECTOR", "CIRCLE", "PLANE", "CYLINDRICAL_SURFACE"} {
		geometryNames[k] = true
	}
	if e := occtLoopContextUnits(f, rep.Args[2].Ref, false, inches); e != nil {
		return bad()
	}
	schemas := 0
	for _, h := range f.Header {
		if h.Type == "FILE_SCHEMA" {
			schemas++
			if len(h.Args) != 1 || h.Args[0].Kind != Aggregate || len(h.Args[0].Items) != 1 || h.Args[0].Items[0].Kind != String || (h.Args[0].Items[0].Text != "AUTOMOTIVE_DESIGN" && h.Args[0].Items[0].Text != "AUTOMOTIVE_DESIGN { 1 0 10303 214 1 1 1 1 }") {
				return bad()
			}
		}
	}
	if schemas != 1 {
		return bad()
	}
	counts := map[string]int{}
	var roots []int64
	var sdr, apd, category int64
	for _, id := range f.Order {
		for _, r := range f.Entities[id].Records {
			if !allowed[r.Type] {
				return bad()
			}
			if (geometryNames[r.Type] || r.Type == "ADVANCED_BREP_SHAPE_REPRESENTATION") && (len(r.Args) == 0 || r.Args[0].Kind != String) {
				return bad()
			}
			counts[r.Type]++
			switch r.Type {
			case "SHAPE_DEFINITION_REPRESENTATION":
				sdr = id
				roots = append(roots, id)
			case "APPLICATION_PROTOCOL_DEFINITION":
				apd = id
				roots = append(roots, id)
			case "PRODUCT_RELATED_PRODUCT_CATEGORY":
				category = id
				roots = append(roots, id)
			}
		}
	}
	for _, k := range []string{"APPLICATION_PROTOCOL_DEFINITION", "APPLICATION_CONTEXT", "SHAPE_DEFINITION_REPRESENTATION", "PRODUCT_DEFINITION_SHAPE", "PRODUCT_DEFINITION", "PRODUCT_DEFINITION_FORMATION", "PRODUCT", "PRODUCT_CONTEXT", "PRODUCT_DEFINITION_CONTEXT", "PRODUCT_RELATED_PRODUCT_CATEGORY", "ADVANCED_BREP_SHAPE_REPRESENTATION", "MANIFOLD_SOLID_BREP", "CLOSED_SHELL"} {
		if counts[k] != 1 {
			return bad()
		}
	}
	if inches {
		for _, k := range inchRecords {
			if counts[k] != 1 {
				return bad()
			}
		}
	}
	pid, pd, _, e := independentMaterialProduct(f, sdr)
	if e != nil || pid != a.Products[0].ProductID || pd != a.Products[0].DefinitionID {
		return bad()
	}
	sd, _ := simple(f, sdr, "SHAPE_DEFINITION_REPRESENTATION", 2)
	if sd.Args[1].Ref != repID {
		return bad()
	}
	c, e := simple(f, category, "PRODUCT_RELATED_PRODUCT_CATEGORY", 3)
	if e != nil || c.Args[0].Kind != String || c.Args[0].Text != "part" || c.Args[1].Kind != Omitted {
		return bad()
	}
	ps, e := listRefs(c.Args[2], 1)
	if e != nil || len(ps) != 1 || ps[0] != pid {
		return bad()
	}
	ad, e := simple(f, apd, "APPLICATION_PROTOCOL_DEFINITION", 4)
	if e != nil || ad.Args[0].Kind != String || ad.Args[1].Kind != String || ad.Args[0].Text != "international standard" || ad.Args[1].Text != "automotive_design" {
		return bad()
	}
	year, e := liftSourceRat(ad.Args[2])
	if e != nil || year.Cmp(big.NewRat(2000, 1)) != 0 {
		return bad()
	}
	app, e := simple(f, ad.Args[3].Ref, "APPLICATION_CONTEXT", 1)
	if e != nil || app.Args[0].Kind != String || app.Args[0].Text != "core data for automotive mechanical design processes" {
		return bad()
	}
	for _, k := range []string{"PRODUCT_CONTEXT", "PRODUCT_DEFINITION_CONTEXT"} {
		for _, id := range f.Order {
			r := f.Entities[id]
			if len(r.Records) == 1 && r.Records[0].Type == k {
				if r.Records[0].Args[1].Ref != ad.Args[3].Ref {
					return bad()
				}
			}
		}
	}
	closure, e := nauoClosure(f, roots, 16)
	if e != nil || len(closure) != len(f.Entities) {
		return bad()
	}
	geometry, e := nauoClosure(f, items, 16)
	if e != nil {
		return bad()
	}
	for id, en := range f.Entities {
		for _, r := range en.Records {
			if geometryNames[r.Type] && !geometry[id] {
				return bad()
			}
		}
	}
	return items[1], nil
}

func occtLoopContextMode(f *File, id int64, optionalUncertainty bool) error {
	return occtLoopContextUnits(f, id, optionalUncertainty, false)
}

func occtLoopContextUnits(f *File, id int64, optionalUncertainty, inches bool) error {
	en := f.Entities[id]
	if len(en.Records) != 4 && !(optionalUncertainty && len(en.Records) == 3) {
		return geom.ErrDomain
	}
	seen := map[string]bool{}
	var unitIDs []int64
	var uncertainty int64
	for _, r := range en.Records {
		if seen[r.Type] {
			return geom.ErrDomain
		}
		seen[r.Type] = true
		switch r.Type {
		case "GEOMETRIC_REPRESENTATION_CONTEXT":
			if len(r.Args) != 1 {
				return geom.ErrDomain
			}
			q, e := liftSourceRat(r.Args[0])
			if e != nil || q.Cmp(big.NewRat(3, 1)) != 0 {
				return geom.ErrDomain
			}
		case "REPRESENTATION_CONTEXT":
			if len(r.Args) != 2 || r.Args[0].Kind != String || r.Args[1].Kind != String {
				return geom.ErrDomain
			}
		case "GLOBAL_UNIT_ASSIGNED_CONTEXT":
			if len(r.Args) != 1 {
				return geom.ErrDomain
			}
			var e error
			unitIDs, e = listRefs(r.Args[0], 3)
			if e != nil || len(unitIDs) != 3 {
				return geom.ErrDomain
			}
		case "GLOBAL_UNCERTAINTY_ASSIGNED_CONTEXT":
			if len(r.Args) != 1 {
				return geom.ErrDomain
			}
			ids, e := listRefs(r.Args[0], 1)
			if e != nil || len(ids) != 1 {
				return geom.ErrDomain
			}
			uncertainty = ids[0]
		default:
			return geom.ErrDomain
		}
	}
	kinds := map[string]bool{}
	var length int64
	for _, uid := range unitIDs {
		en := f.Entities[uid]
		if len(en.Records) != 3 {
			return geom.ErrDomain
		}
		if _, conversion := component(en, "CONVERSION_BASED_UNIT"); inches && conversion {
			if kinds["LENGTH_UNIT"] || !occtLoopInch(f, en) {
				return geom.ErrDomain
			}
			kinds["LENGTH_UNIT"], length = true, uid
			continue
		}
		kind := ""
		var si Record
		named := false
		for _, r := range en.Records {
			switch r.Type {
			case "LENGTH_UNIT", "PLANE_ANGLE_UNIT", "SOLID_ANGLE_UNIT":
				if kind != "" || len(r.Args) != 0 {
					return geom.ErrDomain
				}
				kind = r.Type
			case "NAMED_UNIT":
				if named || len(r.Args) != 1 || r.Args[0].Kind != Derived {
					return geom.ErrDomain
				}
				named = true
			case "SI_UNIT":
				if si.Type != "" || len(r.Args) != 2 {
					return geom.ErrDomain
				}
				si = r
			default:
				return geom.ErrDomain
			}
		}
		if !named || kind == "" || si.Type == "" || kinds[kind] || si.Args[1].Kind != Enumeration {
			return geom.ErrDomain
		}
		kinds[kind] = true
		switch kind {
		case "LENGTH_UNIT":
			length = uid
			if si.Args[1].Text != "METRE" || !(si.Args[0].Kind == Omitted || (si.Args[0].Kind == Enumeration && si.Args[0].Text == "MILLI")) {
				return geom.ErrDomain
			}
		case "PLANE_ANGLE_UNIT":
			if si.Args[0].Kind != Omitted || si.Args[1].Text != "RADIAN" {
				return geom.ErrDomain
			}
		case "SOLID_ANGLE_UNIT":
			if si.Args[0].Kind != Omitted || si.Args[1].Text != "STERADIAN" {
				return geom.ErrDomain
			}
		}
	}
	if uncertainty == 0 && optionalUncertainty && len(en.Records) == 3 {
		return nil
	}
	u, e := simple(f, uncertainty, "UNCERTAINTY_MEASURE_WITH_UNIT", 4)
	if e != nil || u.Args[0].Kind != Typed || u.Args[0].Text != "LENGTH_MEASURE" || len(u.Args[0].Items) != 1 || u.Args[1].Ref != length || u.Args[2].Kind != String || u.Args[3].Kind != String {
		return geom.ErrDomain
	}
	q, e := liftSourceRat(u.Args[0].Items[0])
	if e != nil || q.Sign() < 0 {
		return geom.ErrDomain
	}
	return nil
}

// OCCT's AP214 inch declaration is exactly 127/5 millimetres. Authenticate
// that original conversion chain before accepting its rounded float64 unit.
func occtLoopInch(f *File, unit Entity) bool {
	length, ok := component(unit, "LENGTH_UNIT")
	named, hasName := component(unit, "NAMED_UNIT")
	conversion, converted := component(unit, "CONVERSION_BASED_UNIT")
	if !ok || len(length.Args) != 0 || !hasName || len(named.Args) != 1 || named.Args[0].Kind != Reference ||
		!converted || len(conversion.Args) != 2 || conversion.Args[0].Kind != String || conversion.Args[0].Text != "INCH" || conversion.Args[1].Kind != Reference {
		return false
	}
	dim, err := simple(f, named.Args[0].Ref, "DIMENSIONAL_EXPONENTS", 7)
	if err != nil {
		return false
	}
	for i, value := range dim.Args {
		want := int64(0)
		if i == 0 {
			want = 1
		}
		q, err := liftSourceRat(value)
		if err != nil || q.Cmp(big.NewRat(want, 1)) != 0 {
			return false
		}
	}
	m, err := simple(f, conversion.Args[1].Ref, "LENGTH_MEASURE_WITH_UNIT", 2)
	if err != nil || m.Args[1].Kind != Reference {
		return false
	}
	q, err := liftSourceRat(m.Args[0])
	if err != nil || q.Cmp(big.NewRat(127, 5)) != 0 {
		return false
	}
	base := f.Entities[m.Args[1].Ref]
	length, ok = component(base, "LENGTH_UNIT")
	named, hasName = component(base, "NAMED_UNIT")
	si, isSI := component(base, "SI_UNIT")
	return len(base.Records) == 3 && ok && len(length.Args) == 0 && hasName && len(named.Args) == 1 && named.Args[0].Kind == Derived &&
		isSI && len(si.Args) == 2 && si.Args[0].Kind == Enumeration && si.Args[0].Text == "MILLI" &&
		si.Args[1].Kind == Enumeration && si.Args[1].Text == "METRE"
}

// WriteOCCTExactCircularLoopSource re-proves and re-emits the authenticated
// original source graph, keeping its product metadata and original entity IDs.
func WriteOCCTExactCircularLoopSource(ctx context.Context, w io.Writer, f *File, l Limits) error {
	if ctx == nil || w == nil || !independentNativeSourceRecordBound(f, 4096, 16) {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	b, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return e
	}
	if len(b) > 1<<20 {
		return geom.ErrConvergence
	}
	if _, e = ReadOCCTExactCircularLoopExtrusion(ctx, bytes.NewReader(b), l); e != nil {
		return e
	}
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		return io.ErrShortWrite
	}
	return e
}
