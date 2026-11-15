package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"slices"

	rigid "plainsolid/brep/placement"
	"plainsolid/brep/trimmed"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// ConvexArcThroughBoreRim identifies one complete shared original rim and its
// cap and cylindrical owner curves in their authored association order.
type ConvexArcThroughBoreRim struct {
	CapFaceID, BoundID, LoopID, EdgeID, VertexID, ModelID int64
	Owners                                                [2]ConvexArcPCurveAgreement
	AxialStation                                          *big.Rat
}

// ConvexArcThroughBoreEvidence proves one inward cylindrical wall between the
// two outer cap planes. The center is in the repaired exterior's local XY
// frame. The ordered seam owners retain both distinct original UV branches.
type ConvexArcThroughBoreEvidence struct {
	WallFaceID, WallSupportID, WallBoundID, WallLoopID, SeamEdgeID, SeamModelID int64
	Rims                                                                        [2]ConvexArcThroughBoreRim
	SeamOwners                                                                  [2]ConvexArcPCurveAgreement
	SeamParameters, SeamAngles, TurnDefect                                      [2]*big.Rat
	Center                                                                      [2]*big.Rat
	Radius                                                                      *big.Rat
}

// RationalPosedConvexArcThroughBore retains an independently checked original
// interior wire on each planar cap with the repaired exterior proof. The
// corrected STEP carries the two wires and the inward bore wall.
type RationalPosedConvexArcThroughBore struct {
	original, corrected string
	outer               RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion
	Bore                ConvexArcThroughBoreEvidence
}

func (m RationalPosedConvexArcThroughBore) OriginalSTEP() string       { return m.original }
func (m RationalPosedConvexArcThroughBore) CorrectedModelSTEP() string { return m.corrected }
func (m RationalPosedConvexArcThroughBore) Pose() rigid.RationalRigid  { return m.outer.Pose() }
func (m RationalPosedConvexArcThroughBore) ExteriorRepairs() []ConvexArcRepairSite {
	return m.outer.Repairs()
}

// InspectRationalPosedConvexArcThroughBore proves a bounded original two-fillet
// millimetre product with two interior cap wires around one axial through-hole.
// It repairs proven exterior sites and bounds the original bore seam owner
// discrepancy in source units. The original bore model and pcurves remain in
// the corrected exchange graph. The result certifies the source and exchange
// graph for both cap wires.
func InspectRationalPosedConvexArcThroughBore(ctx context.Context, r io.Reader, l Limits, o ConvexArcRepairOptions) (RationalPosedConvexArcThroughBore, error) {
	var zero RationalPosedConvexArcThroughBore
	if ctx == nil || r == nil || !finiteNumber(o.MaxDistance) || o.MaxDistance <= 0 {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	l, err := l.defaults()
	if err != nil {
		return zero, err
	}
	// The admitted source has eight faces, twelve vertices and 36 directed
	// edge uses. Apply the caller's budget before selecting the exterior.
	if l.MaxFaces < 8 || l.MaxVertices < 12 || l.MaxEdgeUses < 36 {
		return zero, geom.ErrConvergence
	}
	l.MaxBytes, l.MaxEntities, l.MaxValues, l.MaxDepth = min(l.MaxBytes, 1<<20), min(l.MaxEntities, 1024), min(l.MaxValues, 8192), min(l.MaxDepth, 16)
	raw, err := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if err != nil {
		return zero, err
	}
	if int64(len(raw)) > l.MaxBytes {
		return zero, geom.ErrConvergence
	}
	f, err := Parse(bytes.NewReader(raw), l)
	if err != nil {
		return zero, err
	}
	bore, exterior, err := selectConvexArcBore(f)
	if err != nil {
		return zero, fmt.Errorf("through-bore topology: %w", err)
	}
	outer, err := RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(exterior), l, o)
	if err != nil {
		return zero, fmt.Errorf("through-bore exterior: %w", err)
	}
	provenance := outer.Provenance()
	if outer.LocalDescriptor().LengthUnit != .001 || provenance.Shell != bore.shell || len(outer.LocalDescriptor().Segments) != 5 {
		return zero, geom.ErrDomain
	}
	orderedCaps := [2]convexArcBoreLoop{}
	for i, face := range provenance.Caps {
		for _, candidate := range bore.caps {
			if candidate.face == face {
				orderedCaps[i] = candidate
			}
		}
		if orderedCaps[i].face == 0 {
			return zero, geom.ErrDomain
		}
	}
	bore.caps = orderedCaps
	closure, err := nauoClosure(f, []int64{bore.shell}, 16)
	if err != nil {
		return zero, err
	}
	for id := range closure {
		for _, r := range f.Entities[id].Records {
			if (r.Type == "CARTESIAN_POINT" || r.Type == "DIRECTION") && len(r.Args) == 2 && len(r.Args[1].Items) == 2 {
				delete(closure, id) // Original UV charts retain their stored coordinates.
			}
		}
	}
	local, err := rationalConvexArcView(ctx, f, closure, nil, outer.Pose(), true)
	if err != nil {
		return zero, err
	}
	evidence, err := proveConvexArcBore(local, bore, outer.LocalDescriptor().Height, o.MaxDistance)
	if err != nil {
		return zero, fmt.Errorf("through-bore owners: %w", err)
	}
	if err = proveConvexArcBoreSeparation(ctx, outer.Solid(), evidence); err != nil {
		return zero, fmt.Errorf("through-bore separation: %w", err)
	}
	corrected, err := restoredConvexArcBoreSTEP(f, bore, outer.CorrectedModelSTEP(), l)
	if err != nil {
		return zero, err
	}
	if err = ctx.Err(); err != nil {
		return zero, err
	}
	return RationalPosedConvexArcThroughBore{original: string(raw), corrected: corrected, outer: outer, Bore: evidence}, nil
}

type convexArcBoreLoop struct {
	face, bound, loop int64
	uses              []analyticUse
}

type convexArcBoreTopology struct {
	shell int64
	caps  [2]convexArcBoreLoop
	wall  convexArcBoreLoop
}

func selectConvexArcBore(f *File) (convexArcBoreTopology, []byte, error) {
	var q convexArcBoreTopology
	for _, id := range f.Order {
		if _, err := simple(f, id, "CLOSED_SHELL", 2); err == nil {
			if q.shell != 0 {
				return q, nil, geom.ErrDomain
			}
			q.shell = id
		}
	}
	shell, err := simple(f, q.shell, "CLOSED_SHELL", 2)
	if err != nil {
		return q, nil, err
	}
	faces, err := listRefs(shell.Args[1], 8)
	if err != nil || len(faces) != 8 {
		return q, nil, geom.ErrDomain
	}
	view := &File{Header: f.Header, Entities: make(map[int64]Entity, len(f.Entities))}
	for id, entity := range f.Entities {
		view.Entities[id] = entity
	}
	before, err := nauoClosure(f, []int64{q.shell}, 16)
	if err != nil {
		return q, nil, err
	}
	seenFaces, seenUses := map[int64]bool{}, map[int64]bool{}
	var caps int
	var exteriorFaces []Value
	for _, id := range faces {
		face, err := simple(f, id, "ADVANCED_FACE", 4)
		if err != nil || seenFaces[id] {
			return q, nil, geom.ErrDomain
		}
		seenFaces[id] = true
		bounds, err := listRefs(face.Args[1], 1)
		if err != nil || len(bounds) > 2 {
			return q, nil, geom.ErrDomain
		}
		loops := make([]convexArcBoreLoop, len(bounds))
		for i, bound := range bounds {
			loops[i], err = readConvexArcBoreLoop(f, id, bound, seenUses)
			if err != nil {
				return q, nil, err
			}
		}
		if len(loops) == 2 {
			if caps == 2 {
				return q, nil, geom.ErrDomain
			}
			if _, err := simple(f, face.Args[2].Ref, "PLANE", 2); err != nil {
				return q, nil, err
			}
			if len(loops[0].uses) == 1 {
				loops[0], loops[1] = loops[1], loops[0]
			}
			if len(loops[0].uses) != 5 || len(loops[1].uses) != 1 {
				return q, nil, geom.ErrDomain
			}
			q.caps[caps] = loops[1]
			caps++
			setConvexArcBoreRefs(view, id, []Value{{Kind: Reference, Ref: loops[0].bound}})
		} else if u := loops[0].uses; len(u) == 4 && (u[0].id == u[2].id || u[1].id == u[3].id) {
			if q.wall.face != 0 {
				return q, nil, geom.ErrDomain
			}
			q.wall = loops[0]
			continue
		}
		exteriorFaces = append(exteriorFaces, Value{Kind: Reference, Ref: id})
	}
	if caps != 2 || q.wall.face == 0 {
		return q, nil, geom.ErrDomain
	}
	setConvexArcBoreRefs(view, q.shell, exteriorFaces)
	keep, err := nauoClosure(view, []int64{q.shell}, 16)
	if err != nil {
		return q, nil, err
	}
	// Preserve all metadata and any unrelated records so the existing complete
	// product proof can reject unowned geometry. Remove just the bore closure.
	for _, id := range f.Order {
		if before[id] && !keep[id] {
			delete(view.Entities, id)
			continue
		}
		view.Order = append(view.Order, id)
	}
	raw, err := mappedPlanarSourceBytes(view)
	return q, raw, err
}

func setConvexArcBoreRefs(f *File, id int64, refs []Value) {
	entity := f.Entities[id]
	old := entity.Records[0]
	old.Args = append([]Value(nil), old.Args...)
	old.Args[1] = Value{Kind: Aggregate, Items: refs}
	entity.Records = []Record{old}
	f.Entities[id] = entity
}

func readConvexArcBoreLoop(f *File, face, bound int64, seen map[int64]bool) (convexArcBoreLoop, error) {
	q := convexArcBoreLoop{face: face, bound: bound}
	b, err := simple(f, bound, "FACE_BOUND", 3)
	if err != nil {
		return q, err
	}
	q.loop = b.Args[1].Ref
	forward, err := boolean(b.Args[2])
	if err != nil {
		return q, err
	}
	loop, err := simple(f, q.loop, "EDGE_LOOP", 2)
	if err != nil {
		return q, err
	}
	ids, err := listRefs(loop.Args[1], 1)
	if err != nil || len(ids) > 5 {
		return q, geom.ErrDomain
	}
	for _, id := range append([]int64{bound, q.loop}, ids...) {
		if seen[id] {
			return q, geom.ErrDomain
		}
		seen[id] = true
	}
	q.uses = make([]analyticUse, len(ids))
	for i, id := range ids {
		u, err := simple(f, id, "ORIENTED_EDGE", 5)
		if err != nil || u.Args[1].Kind != Derived || u.Args[2].Kind != Derived || u.Args[3].Kind != Reference {
			return q, geom.ErrDomain
		}
		sense, err := boolean(u.Args[4])
		if err != nil {
			return q, err
		}
		if !forward {
			i, sense = len(ids)-1-i, !sense
		}
		q.uses[i] = analyticUse{u.Args[3].Ref, sense}
	}
	return q, nil
}

func restoredConvexArcBoreSTEP(original *File, q convexArcBoreTopology, repaired string, l Limits) (string, error) {
	out, err := Parse(bytes.NewBufferString(repaired), l)
	if err != nil {
		return "", err
	}
	// Reinstall the original cap bound lists and closed-shell face list. The
	// surrounding entity IDs, corrected 3D geometry and support IDs survived.
	for _, id := range []int64{q.shell, q.caps[0].face, q.caps[1].face} {
		if _, ok := out.Entities[id]; !ok {
			return "", geom.ErrDomain
		}
		setConvexArcBoreRefs(out, id, original.Entities[id].Records[0].Args[1].Items)
	}
	bore, err := nauoClosure(original, []int64{q.wall.face, q.caps[0].bound, q.caps[1].bound}, 16)
	if err != nil {
		return "", err
	}
	for _, id := range original.Order {
		if !bore[id] {
			continue
		}
		if _, ok := out.Entities[id]; ok {
			if _, err := equalCircularExtrusionGraph(original, id, out, id); err != nil {
				return "", err
			}
			continue
		}
		out.Entities[id] = original.Entities[id]
		out.Order = append(out.Order, id)
	}
	slices.Sort(out.Order)
	raw, err := mappedPlanarSourceBytes(out)
	if err != nil {
		return "", err
	}
	if _, err = Parse(bytes.NewReader(raw), l); err != nil {
		return "", err
	}
	return string(raw), nil
}

func proveConvexArcBore(f *File, q convexArcBoreTopology, height, budget float64) (ConvexArcThroughBoreEvidence, error) {
	var out ConvexArcThroughBoreEvidence
	wall, err := simple(f, q.wall.face, "ADVANCED_FACE", 4)
	if err != nil {
		return out, err
	}
	forward, err := boolean(wall.Args[3])
	if err != nil || forward {
		return out, geom.ErrDomain
	}
	incoming := convexArcBoreIncoming(f)
	support := wall.Args[2].Ref
	cylinder, err := simple(f, support, "CYLINDRICAL_SURFACE", 3)
	if err != nil {
		return out, err
	}
	o, x, _, z, err := convexUVFrame(f, cylinder.Args[1].Ref)
	if err != nil || !convexUVSame(z, occtLoopVec(geom.Vec3{Z: 1})) {
		return out, geom.ErrDomain
	}
	radius, err := liftSourceRat(cylinder.Args[2])
	tolerance, span := new(big.Rat).SetFloat64(budget), new(big.Rat).SetFloat64(height)
	if err != nil || radius.Sign() <= 0 || tolerance.Cmp(new(big.Rat).Quo(radius, big.NewRat(1000000, 1))) > 0 {
		return out, geom.ErrDomain
	}
	if _, err := circularRepairVector([3]*big.Rat{o[0], o[1], radius}); err != nil {
		return out, err
	}
	out.WallFaceID, out.WallSupportID, out.WallBoundID, out.WallLoopID = q.wall.face, support, q.wall.bound, q.wall.loop
	out.Center, out.Radius = [2]*big.Rat{o[0], o[1]}, radius
	var vertices [2][3]*big.Rat
	for i, cap := range q.caps {
		zCap := new(big.Rat)
		if i == 1 {
			zCap.Set(span)
		}
		rim, point, err := proveConvexArcBoreRim(f, incoming, cap, q.wall.face, support, o, x, z, radius, zCap, i == 0)
		if err != nil {
			return out, err
		}
		out.Rims[i], vertices[i] = rim, point
	}
	if out.Rims[0].EdgeID == out.Rims[1].EdgeID || out.Rims[0].VertexID == out.Rims[1].VertexID {
		return out, geom.ErrDomain
	}
	// Expected material-oriented wall cycle starts at the lower rim. Its
	// uses of the two full circles oppose the original cap uses.
	wallUses := q.wall.uses
	start := slices.Index(wallUses, analyticUse{out.Rims[0].EdgeID, false})
	if start < 0 {
		return out, geom.ErrDomain
	}
	seamID := wallUses[(start+1)%4].id
	want := [4]analyticUse{{out.Rims[0].EdgeID, false}, {seamID, true}, {out.Rims[1].EdgeID, true}, {seamID, false}}
	for i, use := range want {
		if wallUses[(start+i)%4] != use {
			return out, geom.ErrDomain
		}
	}
	a, b, seam, err := analyticEdge(f, seamID)
	if err != nil || a != out.Rims[0].VertexID || b != out.Rims[1].VertexID {
		return out, geom.ErrDomain
	}
	model, owners, err := originalConvexArcBoreOwners(f, incoming, seamID, seam, "SEAM_CURVE", [2]int64{q.wall.face, q.wall.face}, [2]int64{support, support})
	if err != nil {
		return out, err
	}
	p, d, err := convexUVLine(f, model, 3)
	if err != nil || !convexUVSame([3]*big.Rat(d), z) || !convexUVSame([3]*big.Rat(p), boreCirclePoint(o, x, radius)) {
		return out, geom.ErrDomain
	}
	for i, v := range vertices {
		s := out.Rims[i].AxialStation
		point := [3]*big.Rat{p[0], p[1], new(big.Rat).Add(p[2], s)}
		if !convexUVSame(v, point) {
			return out, geom.ErrDomain
		}
		out.SeamParameters[i] = s
	}
	for i, owner := range owners {
		uv, speed, err := convexUVLine(f, owner.GeometryID, 2)
		if err != nil || uv[1].Sign() != 0 || speed[0].Sign() != 0 || speed[1].Cmp(big.NewRat(1, 1)) != 0 ||
			i == 0 && uv[0].Sign() <= 0 || i == 1 && uv[0].Sign() != 0 {
			return out, geom.ErrDomain
		}
		out.SeamAngles[i] = uv[0]
		owners[i].Kind = "cylinder-line"
		owners[i].DistanceSquaredUpper = new(big.Rat)
	}
	lo, hi := pcurvePiBounds()
	two := big.NewRat(2, 1)
	out.TurnDefect = [2]*big.Rat{new(big.Rat).Sub(out.SeamAngles[0], new(big.Rat).Mul(two, hi)),
		new(big.Rat).Sub(out.SeamAngles[0], new(big.Rat).Mul(two, lo))}
	angle := new(big.Rat).Abs(out.TurnDefect[0])
	if upper := new(big.Rat).Abs(out.TurnDefect[1]); upper.Cmp(angle) > 0 {
		angle = upper
	}
	// The two affine axial laws agree everywhere. The radial mismatch is
	// bounded by radius times the enclosed phase discrepancy from 2*pi.
	distance := new(big.Rat).Mul(radius, angle)
	if distance.Cmp(tolerance) > 0 {
		return out, geom.ErrDomain
	}
	owners[0].DistanceSquaredUpper.Mul(distance, distance)
	out.SeamEdgeID, out.SeamModelID, out.SeamOwners = seamID, model, owners
	return out, nil
}

func proveConvexArcBoreRim(f *File, incoming map[int64][]int64, cap convexArcBoreLoop, wall, cylinder int64, origin, x, z [3]*big.Rat, radius, zCap *big.Rat, low bool) (ConvexArcThroughBoreRim, [3]*big.Rat, error) {
	var out ConvexArcThroughBoreRim
	var zero [3]*big.Rat
	face, err := simple(f, cap.face, "ADVANCED_FACE", 4)
	if err != nil || cap.uses[0].sense != low || (face.Args[3].Text == "F") != low {
		return out, zero, geom.ErrDomain
	}
	plane, err := simple(f, face.Args[2].Ref, "PLANE", 2)
	if err != nil {
		return out, zero, err
	}
	po, _, _, pz, err := convexUVFrame(f, plane.Args[1].Ref)
	if err != nil || !convexUVSame(z, pz) || po[2].Cmp(zCap) != 0 {
		return out, zero, geom.ErrDomain
	}
	edgeID := cap.uses[0].id
	a, b, sc, err := analyticEdge(f, edgeID)
	if err != nil || a != b {
		return out, zero, geom.ErrDomain
	}
	model, owners, err := originalConvexArcBoreOwners(f, incoming, edgeID, sc, "SURFACE_CURVE", [2]int64{cap.face, wall}, [2]int64{face.Args[2].Ref, cylinder})
	if err != nil {
		return out, zero, err
	}
	circle, err := simple(f, model, "CIRCLE", 3)
	if err != nil {
		return out, zero, err
	}
	center, mx, _, mz, err := convexUVFrame(f, circle.Args[1].Ref)
	if err != nil || !convexUVSame(x, mx) || !convexUVSame(z, mz) || center[0].Cmp(origin[0]) != 0 || center[1].Cmp(origin[1]) != 0 || center[2].Cmp(zCap) != 0 {
		return out, zero, geom.ErrDomain
	}
	r, err := liftSourceRat(circle.Args[2])
	point, vertexErr := circularNAUOVertex(f, a)
	if err != nil || vertexErr != nil || r.Cmp(radius) != 0 || !convexUVSame(point, boreCirclePoint(center, x, radius)) {
		return out, zero, geom.ErrDomain
	}
	for i, owner := range owners {
		kind, distance, err := boundConvexOwnerPCurve(f, f, model, owner.GeometryID, owner.SupportID)
		if err != nil || distance.Sign() != 0 {
			return out, zero, geom.ErrDomain
		}
		owners[i].Kind, owners[i].DistanceSquaredUpper = kind, distance
	}
	out = ConvexArcThroughBoreRim{CapFaceID: cap.face, BoundID: cap.bound, LoopID: cap.loop, EdgeID: edgeID, VertexID: a, ModelID: model,
		Owners: owners, AxialStation: new(big.Rat).Sub(zCap, origin[2])}
	return out, point, nil
}

func originalConvexArcBoreOwners(f *File, incoming map[int64][]int64, edge, wrapper int64, kind string, faces, supports [2]int64) (int64, [2]ConvexArcPCurveAgreement, error) {
	var out [2]ConvexArcPCurveAgreement
	w, err := simple(f, wrapper, kind, 4)
	if err != nil || !slices.Equal(incoming[wrapper], []int64{edge}) || len(incoming[edge]) != 2 || w.Args[3].Kind != Enumeration || w.Args[3].Text != "PCURVE_S1" {
		return 0, out, geom.ErrDomain
	}
	model, err := ref(w.Args[1])
	if err != nil || !slices.Equal(incoming[model], []int64{wrapper}) {
		return 0, out, geom.ErrDomain
	}
	pcs, err := listRefs(w.Args[2], 2)
	if err != nil || len(pcs) != 2 || pcs[0] == pcs[1] {
		return 0, out, geom.ErrDomain
	}
	for i, id := range pcs {
		p, err := simple(f, id, "PCURVE", 3)
		if err != nil || !slices.Equal(incoming[id], []int64{wrapper}) || p.Args[1].Ref != supports[i] {
			return 0, out, geom.ErrDomain
		}
		dr, err := simple(f, p.Args[2].Ref, "DEFINITIONAL_REPRESENTATION", 3)
		if err != nil || !slices.Equal(incoming[p.Args[2].Ref], []int64{id}) || validatePCurveContext(f, dr.Args[2].Ref) != nil {
			return 0, out, geom.ErrDomain
		}
		g, err := listRefs(dr.Args[1], 1)
		if err != nil || len(g) != 1 || !slices.Equal(incoming[g[0]], []int64{p.Args[2].Ref}) {
			return 0, out, geom.ErrDomain
		}
		out[i] = ConvexArcPCurveAgreement{EdgeID: edge, SurfaceCurveID: wrapper, PCurveID: id, FaceID: faces[i], SupportID: supports[i], GeometryID: g[0]}
	}
	return model, out, nil
}

func convexArcBoreIncoming(f *File) map[int64][]int64 {
	refs := make(map[int64][]int64, len(f.Order))
	var collect func(int64, Value)
	collect = func(owner int64, value Value) {
		if value.Kind == Reference {
			refs[value.Ref] = append(refs[value.Ref], owner)
		}
		for _, child := range value.Items {
			collect(owner, child)
		}
	}
	for _, id := range f.Order {
		for _, record := range f.Entities[id].Records {
			for _, value := range record.Args {
				collect(id, value)
			}
		}
	}
	return refs
}

func boreCirclePoint(center, x [3]*big.Rat, radius *big.Rat) [3]*big.Rat {
	out := convexUVZero()
	for i := range out {
		out[i].Mul(x[i], radius).Add(out[i], center[i])
	}
	return out
}

func proveConvexArcBoreSeparation(ctx context.Context, exterior trimmed.Solid, bore ConvexArcThroughBoreEvidence) error {
	var lo, hi [2]*big.Rat
	for i, c := range bore.Center {
		lo[i], hi[i] = new(big.Rat).Sub(c, bore.Radius), new(big.Rat).Add(c, bore.Radius)
	}
	// The exterior is an exact straight extrusion. The bore's enclosing
	// square strictly inside each cap certifies separation along the span.
	for _, face := range exterior.Faces()[:2] {
		location, proof, err := trimmed.LocateRationalBoxInFaceTrim(ctx, face, lo, hi, trimvalidate.Options{})
		if err != nil {
			return err
		}
		if proof.Status != trimvalidate.Certified || location != trimvalidate.Inside {
			return geom.ErrDomain
		}
	}
	return nil
}
