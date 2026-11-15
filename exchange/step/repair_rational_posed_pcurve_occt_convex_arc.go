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
	// PCurveParameterScale maps model parameters to the original owner
	// parameter. An absent value denotes the identity map.
	PCurveParameterScale *big.Rat `json:",omitempty"`
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
	spans     []ConvexArcCylinderSpanOwner
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
	return copyFiniteConvexArcLineOwners(m.finite)
}

func copyFiniteConvexArcLineOwners(src []FiniteConvexArcLineOwner) []FiniteConvexArcLineOwner {
	out := append([]FiniteConvexArcLineOwner(nil), src...)
	for i, q := range out {
		out[i].StartParameter = new(big.Rat).Set(q.StartParameter)
		out[i].EndParameter = new(big.Rat).Set(q.EndParameter)
		if q.PCurveParameterScale != nil {
			out[i].PCurveParameterScale = new(big.Rat).Set(q.PCurveParameterScale)
		}
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
	src, e := readRationalConvexArcSource(ctx, r, l, o, false)
	if e != nil {
		return zero, e
	}
	tol := new(big.Rat).SetFloat64(o.MaxDistance)
	tol.Mul(tol, tol)
	var finiteRanges []FiniteConvexArcLineOwner
	var spans []ConvexArcCylinderSpanOwner
	for i := range src.owners {
		if e = ctx.Err(); e != nil {
			return zero, e
		}
		q := &src.owners[i]
		sc, _ := simple(src.original, q.SurfaceCurveID, "SURFACE_CURVE", 4)
		q.Kind, q.DistanceSquaredUpper, e = boundConvexOwnerPCurve(src.local, src.corrected, sc.Args[1].Ref, q.GeometryID, q.SupportID)
		if e != nil {
			if src.conversion != nil && src.local.Entities[q.GeometryID].Records[0].Type == "B_SPLINE_CURVE_WITH_KNOTS" {
				var span ConvexArcCylinderSpanOwner
				span, q.Kind, q.DistanceSquaredUpper, e = boundConvertedConvexArcCylinderSpan(src.local, src.corrected, *q, sc.Args[1].Ref, tol, src.conversion, nil, nil)
				spans = append(spans, span)
			} else {
				var domain FiniteConvexArcLineOwner
				if src.conversion != nil {
					domain, q.DistanceSquaredUpper, e = boundMappedConvexArcLineOwner(src.local, src.corrected, *q, sc.Args[1].Ref, tol, src.conversion, nil)
				}
				if e != nil {
					domain, q.DistanceSquaredUpper, e = boundFiniteConvexArcLineOwner(src.local, src.corrected, *q, sc.Args[1].Ref, tol)
				}
				q.Kind = "finite-plane-line"
				finiteRanges = append(finiteRanges, domain)
			}
			if e != nil {
				return zero, fmt.Errorf("source pcurve #%d: %w", q.PCurveID, e)
			}
		}
		if q.DistanceSquaredUpper.Cmp(tol) > 0 {
			return zero, fmt.Errorf("source pcurve #%d exceeds repair distance: %w", q.PCurveID, geom.ErrDomain)
		}
	}
	world, pose, e := src.placeCorrectedModel(ctx)
	if e != nil {
		return zero, e
	}
	base := PCurveRepairedOCCTConvexArcExtrusion{source: src.raw, material: src.material, agreements: src.owners, tolerance: o.MaxDistance}
	return RationalPosedPCurveRepairedOCCTConvexArcExtrusion{
		base: base, corrected: world, pose: pose, reference: src.reference, finite: finiteRanges, spans: spans,
	}, nil
}

type rationalConvexArcSource struct {
	raw                        string
	original, local, corrected *File
	rotation                   rigidplacement.RationalRigid
	reference                  int64
	body, metadata             map[int64]bool
	owners                     []ConvexArcPCurveAgreement
	material                   RepairedOCCTConvexArcExtrusion
	limits                     Limits
	conversion                 *convertedConvexArcLength
}

func readRationalConvexArcSource(ctx context.Context, r io.Reader, l Limits, o ConvexArcRepairOptions, grouped bool) (*rationalConvexArcSource, error) {
	if ctx == nil || r == nil || !finiteNumber(o.MaxDistance) || o.MaxDistance <= 0 {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	l, e := l.defaults()
	if e != nil {
		return nil, e
	}
	l.MaxBytes = min(l.MaxBytes, 1<<20)
	l.MaxEntities = min(l.MaxEntities, 1024)
	l.MaxValues = min(l.MaxValues, 8192)
	l.MaxDepth = min(l.MaxDepth, 16)
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return nil, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return nil, geom.ErrConvergence
	}
	f, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return nil, e
	}
	fillets := 1
	if grouped {
		fillets = 2
	}
	model, owners, e := convexPCurveModelView(f, 3*(3+fillets), grouped)
	converted := false
	if e != nil {
		if grouped {
			fillets = 3
		}
		model, owners, e = convexPCurveModelView(f, 3*(3+fillets), true)
		converted = !grouped && e == nil
	}
	if e != nil {
		return nil, e
	}
	if grouped {
		// The grouped native-unit dialect already admits complex rational
		// cap owners. A simple cylindrical spline marks the converted chart.
		for _, owner := range owners {
			if _, err := simple(f, owner.GeometryID, "B_SPLINE_CURVE_WITH_KNOTS", 9); err == nil {
				if _, err = simple(f, owner.SupportID, "CYLINDRICAL_SURFACE", 3); err == nil {
					converted = true
					break
				}
			}
		}
	}
	pose, reference, body, metadata, e := rationalConvexArcPose(model, grouped)
	if e != nil {
		return nil, e
	}
	src := &rationalConvexArcSource{
		raw: string(raw), original: f, rotation: pose, reference: reference,
		body: body, metadata: metadata, owners: owners, limits: l,
	}
	localModel, e := src.transformView(ctx, model, true)
	if e != nil {
		return nil, e
	}
	modelRaw, e := mappedPlanarSourceBytes(localModel)
	if e != nil {
		return nil, e
	}
	src.material, e = repairOCCTConvexArcModel(ctx, bytes.NewReader(modelRaw), l, o, fillets, grouped)
	if e != nil {
		return nil, fmt.Errorf("rational pose model: %w", e)
	}
	if fillets == 3 && (converted || src.material.Descriptor().LengthUnit != .001) {
		return nil, geom.ErrDomain
	}
	if converted {
		// The model reader has already verified the exact unit declaration.
		switch src.material.Descriptor().LengthUnit {
		case 1:
			src.conversion = &convertedConvexArcLength{ownerScale: big.NewRat(1, 1000), modelSpeed: big.NewRat(1, 1000)}
		case .0254:
			// OCCT writes 3.937007874016E-02 for the model LINE vector.
			// The actual owner conversion is the declared 5/127 inch per mm.
			src.conversion = &convertedConvexArcLength{ownerScale: big.NewRat(5, 127), modelSpeed: big.NewRat(123031496063, 3125000000000)}
		default:
			return nil, geom.ErrDomain
		}
	}
	// Only the proved model closure moves. The original numerically authored
	// UV records retain their original charts and parameter semantics.
	src.local, e = src.transformView(ctx, f, true)
	if e != nil {
		return nil, e
	}
	src.corrected, e = Parse(bytes.NewBufferString(src.material.CorrectedSTEP()), l)
	if e != nil {
		return nil, e
	}
	return src, nil
}

func (src *rationalConvexArcSource) transformView(ctx context.Context, source *File, inverse bool) (*File, error) {
	return rationalConvexArcView(ctx, source, src.body, src.metadata, src.rotation, inverse)
}

func rationalConvexArcView(ctx context.Context, f *File, body, metadata map[int64]bool, pose rigidplacement.RationalRigid, inverse bool) (*File, error) {
	return convexArcTransformView(ctx, f, body, metadata, func(kind string, p [3]*big.Rat) ([3]*big.Rat, error) {
		if kind == "DIRECTION" {
			return pose.TransformVector(ctx, p, inverse, 512)
		}
		return pose.TransformPoint(ctx, p, inverse, 512)
	})
}

func (src *rationalConvexArcSource) placeCorrectedModel(ctx context.Context) (string, rigidplacement.RationalRigid, error) {
	var zero rigidplacement.RationalRigid
	worldView, e := src.transformView(ctx, src.corrected, false)
	if e != nil {
		return "", zero, e
	}
	world, e := mappedPlanarSourceBytes(worldView)
	if e != nil {
		return "", zero, e
	}
	if _, e = Parse(bytes.NewReader(world), src.limits); e != nil {
		return "", zero, e
	}
	for i := range src.material.sites {
		q := &src.material.sites[i]
		q.Original, e = src.rotation.TransformPoint(ctx, q.Original, false, 512)
		if e != nil {
			return "", zero, e
		}
		q.Repaired, e = src.rotation.TransformPoint(ctx, q.Repaired, false, 512)
		if e != nil {
			return "", zero, e
		}
		rawPoint, e := occtLoopTriple(src.original, q.PointID, "CARTESIAN_POINT")
		if e != nil || !convexUVSame(rawPoint, q.Original) {
			return "", zero, geom.ErrDomain
		}
		d := occtLoopSub(q.Repaired, q.Original)
		if occtLoopDot(d, d).Cmp(q.SquaredDistance) != 0 {
			return "", zero, geom.ErrDomain
		}
	}
	for i := range src.material.tangencies {
		q := &src.material.tangencies[i]
		q.Point, e = src.rotation.TransformPoint(ctx, q.Point, false, 512)
		if e != nil {
			return "", zero, e
		}
	}
	if e = ctx.Err(); e != nil {
		return "", zero, e
	}
	// The local material proof keeps its lateral shift in the profile and
	// stores the represented axial origin separately from the native solid.
	frame := src.material.Descriptor().Frame
	if frame.XAxis() != (geom.Vec3{X: 1}) || frame.ZAxis() != (geom.Vec3{Z: 1}) {
		return "", zero, geom.ErrDomain
	}
	matrix, e := src.rotation.Matrix()
	if e != nil {
		return "", zero, e
	}
	origin, e := src.rotation.TransformPoint(ctx, occtLoopVec(frame.Origin()), false, 512)
	if e != nil {
		return "", zero, e
	}
	for i := range matrix {
		matrix[i][3] = origin[i]
	}
	worldPlacement, e := rigidplacement.NewRationalRigid(matrix, 512)
	if e != nil {
		return "", zero, e
	}
	return string(world), worldPlacement, nil
}

func rationalConvexArcPose(f *File, grouped bool) (pose rigidplacement.RationalRigid, reference int64, body, metadata map[int64]bool, err error) {
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
	var axial [3]*big.Rat
	if grouped {
		for _, id := range f.Order {
			if !body[id] {
				continue
			}
			c, e := simple(f, id, "CYLINDRICAL_SURFACE", 3)
			if e != nil {
				continue
			}
			_, _, _, axial, e = convexUVFrame(f, c.Args[1].Ref)
			if e != nil {
				return pose, 0, nil, nil, e
			}
			break
		}
		if axial[0] == nil {
			return pose, 0, nil, nil, geom.ErrDomain
		}
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
		if grouped {
			z = axial
			y = convexUVCross(z, x)
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
