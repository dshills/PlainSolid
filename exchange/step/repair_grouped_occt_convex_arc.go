package step

import (
	"context"
	"io"
	"math/big"
	"plainsolid/geom"
)

// ConvexArcRepairReversal records a support-identical circle axis change paired
// with reversal of the original edge and both original coedges. Original IDs
// remain unchanged; edge direction must be interpreted using this evidence.
type ConvexArcRepairReversal struct {
	EdgeID, CircleID, AxisDirectionID, OriginalStart, OriginalEnd int64
	OrientedUseIDs                                                [2]int64
}

func (m RepairedOCCTConvexArcExtrusion) Reversals() []ConvexArcRepairReversal {
	return append([]ConvexArcRepairReversal(nil), m.reversals...)
}
func (m PosedRepairedOCCTConvexArcExtrusion) Reversals() []ConvexArcRepairReversal {
	return m.base.Reversals()
}

// ConvexArcTangentEvidence identifies the independently selected original
// cylinder, axial plane and cap for each unique exact tangent point.
type ConvexArcTangentEvidence struct {
	VertexID, PointID, CylinderFaceID, CapFaceID, PlaneFaceID int64
	Point                                                     [3]*big.Rat
}

func (m RepairedOCCTConvexArcExtrusion) Tangencies() []ConvexArcTangentEvidence {
	out := append([]ConvexArcTangentEvidence(nil), m.tangencies...)
	for i := range out {
		for j := range out[i].Point {
			out[i].Point[j] = new(big.Rat).Set(out[i].Point[j])
		}
	}
	return out
}
func (m PosedRepairedOCCTConvexArcExtrusion) Tangencies() []ConvexArcTangentEvidence {
	return m.base.Tangencies()
}

// RepairGroupedOCCTConvexArcExtrusion is a distinct exactly-two-fillet mode:
// five planes, two outward cylinders, five convex profile segments. Each
// endpoint must have its own unique cylinder/plane tangency. Matched reversed
// circle/edge/coedge encodings are admitted only by a complete exact proof.
func RepairGroupedOCCTConvexArcExtrusion(ctx context.Context, r io.Reader, l Limits, o ConvexArcRepairOptions) (RepairedOCCTConvexArcExtrusion, error) {
	return repairOCCTConvexArcExtrusion(ctx, r, l, o, true)
}

// RepairPosedGroupedOCCTConvexArcExtrusion additionally preserves proper
// signed-coordinate poses and reports repairs in original world coordinates.
func RepairPosedGroupedOCCTConvexArcExtrusion(ctx context.Context, r io.Reader, l Limits, o ConvexArcRepairOptions) (PosedRepairedOCCTConvexArcExtrusion, error) {
	return repairPosedOCCTConvexArcExtrusion(ctx, r, l, o, true)
}

func normalizeGroupedConvexArcReversals(f *File, refs map[int64][]int64) ([]ConvexArcRepairReversal, error) {
	var out []ConvexArcRepairReversal
	for _, id := range f.Order {
		circle, e := simple(f, id, "CIRCLE", 3)
		if e != nil {
			continue
		}
		origin, z, _, e := occtLoopFrame(f, circle.Args[1].Ref)
		if e != nil {
			return nil, e
		}
		if occtLoopPositiveAxis(z, 2) {
			continue
		}
		if z[0].Sign() != 0 || z[1].Sign() != 0 || z[2].Sign() >= 0 {
			return nil, geom.ErrDomain
		}
		// No reference may borrow this circle or its frame/axis for another role.
		if len(refs[id]) != 1 {
			return nil, geom.ErrDomain
		}
		eid := refs[id][0]
		edge, e := simple(f, eid, "EDGE_CURVE", 5)
		if e != nil || edge.Args[3].Ref != id {
			return nil, geom.ErrDomain
		}
		same, e := boolean(edge.Args[4])
		if e != nil || !same {
			return nil, geom.ErrDomain
		}
		frame, e := simple(f, circle.Args[1].Ref, "AXIS2_PLACEMENT_3D", 4)
		if e != nil {
			return nil, e
		}
		axis := frame.Args[2].Ref
		if len(refs[circle.Args[1].Ref]) != 1 || refs[circle.Args[1].Ref][0] != id || len(refs[axis]) != 1 || refs[axis][0] != circle.Args[1].Ref {
			return nil, geom.ErrDomain
		}
		radius, e := liftSourceRat(circle.Args[2])
		if e != nil || radius.Sign() <= 0 {
			return nil, geom.ErrDomain
		}
		r2 := new(big.Rat).Mul(radius, radius)
		var p [2][3]*big.Rat
		for i, vid := range []int64{edge.Args[1].Ref, edge.Args[2].Ref} {
			v, e := simple(f, vid, "VERTEX_POINT", 2)
			if e != nil {
				return nil, e
			}
			point, e := occtLoopTriple(f, v.Args[1].Ref, "CARTESIAN_POINT")
			if e != nil {
				return nil, e
			}
			p[i] = occtLoopSub(point, origin)
			if p[i][2].Sign() != 0 || occtLoopDot(p[i], p[i]).Cmp(r2) != 0 {
				return nil, geom.ErrDomain
			}
		}
		// Original -Z, same-sense edge must select the strict clockwise minor arc.
		// Swapping endpoints and +Z then selects exactly its reverse, not complement.
		det := new(big.Rat).Sub(new(big.Rat).Mul(p[0][0], p[1][1]), new(big.Rat).Mul(p[0][1], p[1][0]))
		if det.Sign() >= 0 {
			return nil, geom.ErrDomain
		}
		if len(refs[eid]) != 2 {
			return nil, geom.ErrDomain
		}
		q := ConvexArcRepairReversal{EdgeID: eid, CircleID: id, AxisDirectionID: axis, OriginalStart: edge.Args[1].Ref, OriginalEnd: edge.Args[2].Ref}
		n := 0
		// Record source order for deterministic copied evidence.
		for _, uid := range f.Order {
			if uid != refs[eid][0] && uid != refs[eid][1] {
				continue
			}
			u, e := simple(f, uid, "ORIENTED_EDGE", 5)
			if e != nil || u.Args[3].Ref != eid || u.Args[1].Kind != Derived || u.Args[2].Kind != Derived {
				return nil, geom.ErrDomain
			}
			sense, e := boolean(u.Args[4])
			if e != nil {
				return nil, e
			}
			u.Args[4].Text = "T"
			if sense {
				u.Args[4].Text = "F"
			}
			f.Entities[uid] = Entity{ID: uid, Records: []Record{u}}
			if n >= 2 {
				return nil, geom.ErrDomain
			}
			q.OrientedUseIDs[n] = uid
			n++
		}
		if n != 2 {
			return nil, geom.ErrDomain
		}
		edge.Args[1], edge.Args[2] = edge.Args[2], edge.Args[1]
		f.Entities[eid] = Entity{ID: eid, Records: []Record{edge}}
		d, e := simple(f, axis, "DIRECTION", 2)
		if e != nil {
			return nil, e
		}
		d.Args[1].Items = []Value{{Kind: Number, Text: "0."}, {Kind: Number, Text: "0."}, {Kind: Number, Text: "1."}}
		f.Entities[axis] = Entity{ID: axis, Records: []Record{d}}
		out = append(out, q)
	}
	return out, nil
}
