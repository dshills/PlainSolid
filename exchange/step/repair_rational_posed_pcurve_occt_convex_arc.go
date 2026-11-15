package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"

	rigidplacement "plainsolid/brep/placement"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// FiniteConvexArcLineOwner retains the directed source model parameter
// endpoints for a planar LINE owner that required a finite-interval bound.
// PCurves reports its continuous squared distance upper bound on this range.
type FiniteConvexArcLineOwner struct {
	PCurveID                     int64
	StartParameter, EndParameter *big.Rat
}

// RationalPosedPCurveRepairedOCCTConvexArcExtrusion keeps product-local native
// geometry and its exact source-derived proper placement into world space.
// Its original STEP includes every pcurve. CorrectedModelSTEP carries the
// explicitly repaired model graph in original world coordinates.
type RationalPosedPCurveRepairedOCCTConvexArcExtrusion struct {
	base      PCurveRepairedOCCTConvexArcExtrusion
	corrected string
	pose      rigidplacement.RationalRigid
	reference int64
	finite    []FiniteConvexArcLineOwner
}

func (m RationalPosedPCurveRepairedOCCTConvexArcExtrusion) OriginalSTEP() string {
	return m.base.OriginalSTEP()
}
func (m RationalPosedPCurveRepairedOCCTConvexArcExtrusion) CorrectedModelSTEP() string {
	return m.corrected
}
func (m RationalPosedPCurveRepairedOCCTConvexArcExtrusion) Solid() trimmed.Solid {
	return m.base.Solid()
}

// LocalDescriptor retains the model recipe in the normalized proof view.
func (m RationalPosedPCurveRepairedOCCTConvexArcExtrusion) LocalDescriptor() ExactCircularLoopExtrusion {
	return m.base.Descriptor()
}

// Pose places Solid directly in original world coordinates. It composes the
// local descriptor's axial origin with the original reference-circle rotation.
func (m RationalPosedPCurveRepairedOCCTConvexArcExtrusion) Pose() rigidplacement.RationalRigid {
	return m.pose
}
func (m RationalPosedPCurveRepairedOCCTConvexArcExtrusion) ReferenceCircleID() int64 {
	return m.reference
}
func (m RationalPosedPCurveRepairedOCCTConvexArcExtrusion) Provenance() ConvexArcRepairProvenance {
	return m.base.Provenance()
}
func (m RationalPosedPCurveRepairedOCCTConvexArcExtrusion) Repairs() []ConvexArcRepairSite {
	return m.base.Repairs()
}
func (m RationalPosedPCurveRepairedOCCTConvexArcExtrusion) Tangencies() []ConvexArcTangentEvidence {
	return m.base.Tangencies()
}
func (m RationalPosedPCurveRepairedOCCTConvexArcExtrusion) PCurves() []ConvexArcPCurveAgreement {
	return m.base.PCurves()
}
func (m RationalPosedPCurveRepairedOCCTConvexArcExtrusion) FiniteLineOwners() []FiniteConvexArcLineOwner {
	out := append([]FiniteConvexArcLineOwner(nil), m.finite...)
	for i, q := range out {
		out[i].StartParameter = new(big.Rat).Set(q.StartParameter)
		out[i].EndParameter = new(big.Rat).Set(q.EndParameter)
	}
	return out
}

// RepairRationalPosedPCurveOCCTConvexArcExtrusion accepts independently
// authored single-fillet STEP whose original reference circle defines an
// exactly orthonormal rational frame. It additionally bounds original planar
// LINE-owner speed differences over their directed finite model intervals.
// Orientation, source ownership and all other owner proofs remain exact.
func RepairRationalPosedPCurveOCCTConvexArcExtrusion(ctx context.Context, r io.Reader, l Limits, o ConvexArcRepairOptions) (RationalPosedPCurveRepairedOCCTConvexArcExtrusion, error) {
	var zero RationalPosedPCurveRepairedOCCTConvexArcExtrusion
	if ctx == nil || r == nil || !finiteNumber(o.MaxDistance) || o.MaxDistance <= 0 {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	l, e := l.defaults()
	if e != nil {
		return zero, e
	}
	l.MaxBytes = min(l.MaxBytes, 1<<20)
	l.MaxEntities = min(l.MaxEntities, 1024)
	l.MaxValues = min(l.MaxValues, 8192)
	l.MaxDepth = min(l.MaxDepth, 16)
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return zero, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return zero, geom.ErrConvergence
	}
	f, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return zero, e
	}
	model, agreements, e := convexPCurveModelView(f, 12, false)
	if e != nil {
		return zero, e
	}
	pose, reference, body, metadata, e := rationalConvexArcPose(model)
	if e != nil {
		return zero, e
	}
	view := func(source *File, inverse bool) (*File, error) {
		return convexArcTransformView(ctx, source, body, metadata, func(kind string, p [3]*big.Rat) ([3]*big.Rat, error) {
			if kind == "DIRECTION" {
				return pose.TransformVector(ctx, p, inverse, 512)
			}
			return pose.TransformPoint(ctx, p, inverse, 512)
		})
	}
	localModel, e := view(model, true)
	if e != nil {
		return zero, e
	}
	modelRaw, e := mappedPlanarSourceBytes(localModel)
	if e != nil {
		return zero, e
	}
	material, e := RepairOCCTConvexArcExtrusion(ctx, bytes.NewReader(modelRaw), l, o)
	if e != nil {
		return zero, fmt.Errorf("rational pose model: %w", e)
	}
	// Only the proved model closure moves. The original numerically authored
	// UV records retain their original charts and parameter semantics.
	localFull, e := view(f, true)
	if e != nil {
		return zero, e
	}
	corrected, e := Parse(bytes.NewBufferString(material.CorrectedSTEP()), l)
	if e != nil {
		return zero, e
	}
	tol := new(big.Rat).SetFloat64(o.MaxDistance)
	tol.Mul(tol, tol)
	var finiteRanges []FiniteConvexArcLineOwner
	for i := range agreements {
		if e = ctx.Err(); e != nil {
			return zero, e
		}
		q := &agreements[i]
		sc, _ := simple(f, q.SurfaceCurveID, "SURFACE_CURVE", 4)
		q.Kind, q.DistanceSquaredUpper, e = boundConvexOwnerPCurve(localFull, corrected, sc.Args[1].Ref, q.GeometryID, q.SupportID)
		if e != nil {
			var domain FiniteConvexArcLineOwner
			domain, q.DistanceSquaredUpper, e = boundFiniteConvexArcLineOwner(localFull, corrected, *q, sc.Args[1].Ref, tol)
			if e != nil {
				return zero, fmt.Errorf("source pcurve #%d: %w", q.PCurveID, e)
			}
			q.Kind = "finite-plane-line"
			finiteRanges = append(finiteRanges, domain)
		}
		if q.DistanceSquaredUpper.Cmp(tol) > 0 {
			return zero, fmt.Errorf("source pcurve #%d exceeds repair distance: %w", q.PCurveID, geom.ErrDomain)
		}
	}
	worldView, e := view(corrected, false)
	if e != nil {
		return zero, e
	}
	world, e := mappedPlanarSourceBytes(worldView)
	if e != nil {
		return zero, e
	}
	if _, e = Parse(bytes.NewReader(world), l); e != nil {
		return zero, e
	}
	for i := range material.sites {
		q := &material.sites[i]
		q.Original, e = pose.TransformPoint(ctx, q.Original, false, 512)
		if e != nil {
			return zero, e
		}
		q.Repaired, e = pose.TransformPoint(ctx, q.Repaired, false, 512)
		if e != nil {
			return zero, e
		}
		rawPoint, e := occtLoopTriple(f, q.PointID, "CARTESIAN_POINT")
		if e != nil || !convexUVSame(rawPoint, q.Original) {
			return zero, geom.ErrDomain
		}
		d := occtLoopSub(q.Repaired, q.Original)
		if occtLoopDot(d, d).Cmp(q.SquaredDistance) != 0 {
			return zero, geom.ErrDomain
		}
	}
	for i := range material.tangencies {
		q := &material.tangencies[i]
		q.Point, e = pose.TransformPoint(ctx, q.Point, false, 512)
		if e != nil {
			return zero, e
		}
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	// The local material proof keeps its lateral shift in the profile and
	// stores the represented axial origin separately from the native solid.
	frame := material.Descriptor().Frame
	if frame.XAxis() != (geom.Vec3{X: 1}) || frame.ZAxis() != (geom.Vec3{Z: 1}) {
		return zero, geom.ErrDomain
	}
	matrix, e := pose.Matrix()
	if e != nil {
		return zero, e
	}
	origin, e := pose.TransformPoint(ctx, occtLoopVec(frame.Origin()), false, 512)
	if e != nil {
		return zero, e
	}
	for i := range matrix {
		matrix[i][3] = origin[i]
	}
	worldPlacement, e := rigidplacement.NewRationalRigid(matrix, 512)
	if e != nil {
		return zero, e
	}
	base := PCurveRepairedOCCTConvexArcExtrusion{source: string(raw), material: material, agreements: agreements, tolerance: o.MaxDistance}
	return RationalPosedPCurveRepairedOCCTConvexArcExtrusion{
		base: base, corrected: string(world), pose: worldPlacement, reference: reference, finite: finiteRanges,
	}, nil
}

func rationalConvexArcPose(f *File) (pose rigidplacement.RationalRigid, reference int64, body, metadata map[int64]bool, err error) {
	var repID int64
	for _, id := range f.Order {
		if _, e := simple(f, id, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3); e == nil {
			if repID != 0 {
				return pose, 0, nil, nil, geom.ErrDomain
			}
			repID = id
		}
	}
	rep, e := simple(f, repID, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
	if e != nil {
		return pose, 0, nil, nil, e
	}
	items, e := listRefs(rep.Args[1], 2)
	if e != nil || len(items) != 2 {
		return pose, 0, nil, nil, geom.ErrDomain
	}
	neutral, e := sideDiskReadFrame(f, items[0])
	if e != nil || frameTransform(neutral) != geom.Identity() {
		return pose, 0, nil, nil, geom.ErrDomain
	}
	if _, e = simple(f, items[1], "MANIFOLD_SOLID_BREP", 2); e != nil {
		return pose, 0, nil, nil, e
	}
	body, e = nauoClosure(f, []int64{items[1]}, 16)
	if e != nil {
		return pose, 0, nil, nil, e
	}
	metadata, e = nauoClosure(f, []int64{items[0]}, 16)
	if e != nil {
		return pose, 0, nil, nil, e
	}
	for _, id := range f.Order {
		if !body[id] {
			continue
		}
		c, e := simple(f, id, "CIRCLE", 3)
		if e != nil {
			continue
		}
		_, x, y, z, e := convexUVFrame(f, c.Args[1].Ref)
		if e != nil {
			return pose, 0, nil, nil, e
		}
		var matrix [3][4]*big.Rat
		for i := range matrix {
			matrix[i] = [4]*big.Rat{x[i], y[i], z[i], new(big.Rat)}
		}
		pose, e = rigidplacement.NewRationalRigid(matrix, 512)
		return pose, id, body, metadata, e
	}
	return pose, 0, nil, nil, geom.ErrDomain
}
