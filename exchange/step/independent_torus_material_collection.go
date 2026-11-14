package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// IndependentTorusMaterial authenticates a natural ring torus with two exact
// model-only CIRCLE seams. Native analytic patches canonicalize angular seams;
// no rational zero-discrepancy trigonometric evaluation is claimed.
type IndependentTorusMaterial struct {
	Frame                                                      geom.Frame
	MajorRadius, MinorRadius                                   float64
	SourceBodyID, SourceShellID, SourceFaceID, SourceSurfaceID int64
	SourceBoundID, SourceLoopID, SourceVertexID, SourcePointID int64
	SourceCircleIDs, SourceEdgeIDs                             [2]int64
	SourceUseIDs                                               [4]int64
	NativeFaceSourceIDs                                        []int64
}

func ReadIndependentToroidalMaterials(ctx context.Context, r io.Reader, l Limits, tol float64) (IndependentExactMaterialCollection, error) {
	m, e := readIndependentExactMaterialCollection(ctx, r, l, tol, 1, 16, true, true, true, false, false, true)
	if e != nil {
		return IndependentExactMaterialCollection{}, e
	}
	for _, p := range m.Members {
		if p.Torus != nil {
			return m, nil
		}
	}
	return IndependentExactMaterialCollection{}, geom.ErrInvalid
}
func WriteIndependentToroidalMaterials(ctx context.Context, w io.Writer, sources []*File, l Limits, tol float64) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	var staged bytes.Buffer
	if e := writeIndependentExactMaterialCollection(ctx, &staged, sources, l, tol, 1, 16, true, true, false, false, true); e != nil {
		return e
	}
	if _, e := ReadIndependentToroidalMaterials(ctx, bytes.NewReader(staged.Bytes()), l, tol); e != nil {
		return e
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(staged.Bytes())
	if e == nil && n != staged.Len() {
		e = io.ErrShortWrite
	}
	return e
}
func readIndependentTorusProduct(ctx context.Context, f *File, bodyID int64, l Limits, tol float64) (IndependentExactMaterial, error) {
	bad := func() (IndependentExactMaterial, error) { return IndependentExactMaterial{}, geom.ErrInvalid }
	keep, e := nauoClosure(f, []int64{bodyID}, l.MaxDepth)
	if e != nil || len(keep) > 40 {
		return bad()
	}
	allowed := map[string]bool{}
	for _, k := range []string{"MANIFOLD_SOLID_BREP", "CLOSED_SHELL", "ADVANCED_FACE", "FACE_BOUND", "EDGE_LOOP", "ORIENTED_EDGE", "EDGE_CURVE", "VERTEX_POINT", "CARTESIAN_POINT", "AXIS2_PLACEMENT_3D", "DIRECTION", "CIRCLE", "TOROIDAL_SURFACE"} {
		allowed[k] = true
	}
	counts := map[string]int{}
	for id := range keep {
		en := f.Entities[id]
		if len(en.Records) != 1 || !allowed[en.Records[0].Type] {
			return bad()
		}
		r := en.Records[0]
		if len(r.Args) == 0 || r.Args[0].Kind != String {
			return bad()
		}
		counts[r.Type]++
	}
	for k, n := range map[string]int{"MANIFOLD_SOLID_BREP": 1, "CLOSED_SHELL": 1, "ADVANCED_FACE": 1, "FACE_BOUND": 1, "EDGE_LOOP": 1, "ORIENTED_EDGE": 4, "EDGE_CURVE": 2, "VERTEX_POINT": 1, "CIRCLE": 2, "TOROIDAL_SURFACE": 1} {
		if counts[k] != n {
			return bad()
		}
	}
	body, e := simple(f, bodyID, "MANIFOLD_SOLID_BREP", 2)
	if e != nil || body.Args[1].Kind != Reference {
		return bad()
	}
	shellID := body.Args[1].Ref
	shell, e := simple(f, shellID, "CLOSED_SHELL", 2)
	if e != nil {
		return bad()
	}
	faces, e := listRefs(shell.Args[1], 1)
	if e != nil || len(faces) != 1 {
		return bad()
	}
	face, e := simple(f, faces[0], "ADVANCED_FACE", 4)
	if e != nil {
		return bad()
	}
	sense, e := boolean(face.Args[3])
	if e != nil || !sense || face.Args[2].Kind != Reference {
		return bad()
	}
	sid := face.Args[2].Ref
	support, e := simple(f, sid, "TOROIDAL_SURFACE", 4)
	if e != nil || support.Args[1].Kind != Reference {
		return bad()
	}
	frame, e := mappedExactFrame(f, support.Args[1].Ref)
	if e != nil {
		return bad()
	}
	R, e := cylinderSupportNumber(support.Args[2])
	if e != nil {
		return bad()
	}
	r, e := cylinderSupportNumber(support.Args[3])
	if e != nil || r <= 0 || R <= r {
		return bad()
	}
	sum := new(big.Rat).Add(new(big.Rat).SetFloat64(R), new(big.Rat).SetFloat64(r))
	outer, exact := sum.Float64()
	if !exact || !finiteNumber(outer) {
		return bad()
	}
	offset := func(distance float64) (geom.Vec3, error) {
		o, x := frame.Origin(), frame.XAxis()
		a, b := [3]float64{o.X, o.Y, o.Z}, [3]float64{x.X, x.Y, x.Z}
		v := [3]float64{}
		for i := range v {
			q := new(big.Rat).Add(new(big.Rat).SetFloat64(a[i]), new(big.Rat).Mul(new(big.Rat).SetFloat64(b[i]), new(big.Rat).SetFloat64(distance)))
			var ok bool
			v[i], ok = q.Float64()
			if !ok || !finiteNumber(v[i]) {
				return geom.Vec3{}, geom.ErrUnrepresentable
			}
		}
		return geom.Vec3{X: v[0], Y: v[1], Z: v[2]}, nil
	}
	minorCenter, e := offset(R)
	if e != nil {
		return bad()
	}
	seamPoint, e := offset(outer)
	if e != nil {
		return bad()
	}
	bounds, e := listRefs(face.Args[1], 1)
	if e != nil || len(bounds) != 1 {
		return bad()
	}
	bound, e := simple(f, bounds[0], "FACE_BOUND", 3)
	if e != nil || bound.Args[1].Kind != Reference {
		return bad()
	}
	sense, e = boolean(bound.Args[2])
	if e != nil || !sense {
		return bad()
	}
	loopID := bound.Args[1].Ref
	loop, e := simple(f, loopID, "EDGE_LOOP", 2)
	if e != nil {
		return bad()
	}
	uses, e := listRefs(loop.Args[1], 4)
	if e != nil || len(uses) != 4 {
		return bad()
	}
	q := IndependentTorusMaterial{Frame: frame, MajorRadius: R, MinorRadius: r, SourceBodyID: bodyID, SourceShellID: shellID, SourceFaceID: faces[0], SourceSurfaceID: sid, SourceBoundID: bounds[0], SourceLoopID: loopID}
	seen := map[int64]bool{}
	for i, uid := range uses {
		if seen[uid] {
			return bad()
		}
		seen[uid] = true
		q.SourceUseIDs[i] = uid
		use, e := simple(f, uid, "ORIENTED_EDGE", 5)
		if e != nil || use.Args[1].Kind != Derived || use.Args[2].Kind != Derived || use.Args[3].Kind != Reference {
			return bad()
		}
		sense, e := boolean(use.Args[4])
		if e != nil || sense != (i >= 2) {
			return bad()
		}
		eid := use.Args[3].Ref
		role := i % 2
		if i >= 2 {
			if eid != q.SourceEdgeIDs[role] {
				return bad()
			}
			continue
		}
		q.SourceEdgeIDs[role] = eid
		if role == 1 && eid == q.SourceEdgeIDs[0] {
			return bad()
		}
		edge, e := simple(f, eid, "EDGE_CURVE", 5)
		if e != nil || edge.Args[1].Kind != Reference || edge.Args[2].Kind != Reference || edge.Args[1].Ref != edge.Args[2].Ref || edge.Args[3].Kind != Reference {
			return bad()
		}
		sense, e = boolean(edge.Args[4])
		if e != nil || !sense {
			return bad()
		}
		vid := edge.Args[1].Ref
		if role == 1 && vid != q.SourceVertexID {
			return bad()
		}
		q.SourceVertexID = vid
		vertex, e := simple(f, vid, "VERTEX_POINT", 2)
		if e != nil || vertex.Args[1].Kind != Reference {
			return bad()
		}
		q.SourcePointID = vertex.Args[1].Ref
		point, e := splitTriple(f, q.SourcePointID, "CARTESIAN_POINT")
		if e != nil || point != seamPoint {
			return bad()
		}
		cid := edge.Args[3].Ref
		q.SourceCircleIDs[role] = cid
		circle, e := simple(f, cid, "CIRCLE", 3)
		if e != nil || circle.Args[1].Kind != Reference {
			return bad()
		}
		cf, e := mappedExactFrame(f, circle.Args[1].Ref)
		if e != nil {
			return bad()
		}
		radius, e := cylinderSupportNumber(circle.Args[2])
		if e != nil {
			return bad()
		}
		if role == 0 {
			if radius != outer || cf.Origin() != frame.Origin() || cf.XAxis() != frame.XAxis() || cf.ZAxis() != frame.ZAxis() {
				return bad()
			}
		} else {
			if radius != r || cf.Origin() != minorCenter || cf.XAxis() != frame.XAxis() || cf.ZAxis() != frame.YAxis().Scale(-1) {
				return bad()
			}
		}
	}
	native, e := parametric.Torus(frame, R, r)
	if e != nil {
		return bad()
	}
	solid, e := trimmed.FromParametric(native)
	if e != nil {
		return bad()
	}
	for range solid.Faces() {
		q.NativeFaceSourceIDs = append(q.NativeFaceSourceIDs, q.SourceFaceID)
	}
	unit := 0.0
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) == 1 && en.Records[0].Type == "ADVANCED_BREP_SHAPE_REPRESENTATION" {
			cid, e := ref(en.Records[0].Args[2])
			if e != nil {
				return bad()
			}
			unit, e = contextUnits(f, cid)
			if e != nil {
				return bad()
			}
		}
	}
	if unit <= 0 {
		return bad()
	}
	if e = ctx.Err(); e != nil {
		return IndependentExactMaterial{}, e
	}
	return IndependentExactMaterial{Name: body.Args[0].Text, LengthUnit: unit, Source: f, Solid: solid, Torus: &q, SourceBodyID: bodyID}, nil
}
