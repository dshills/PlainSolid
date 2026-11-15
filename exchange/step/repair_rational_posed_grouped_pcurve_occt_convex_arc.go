package step

import (
	"context"
	"fmt"
	"io"

	rigidplacement "plainsolid/brep/placement"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion retains the original
// grouped source, its complete owner and finite-trim evidence, and an exact
// rational world placement for the reconstructed product-local solid.
type RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion struct {
	base      GroupedPCurveRepairedOCCTConvexArcExtrusion
	corrected string
	pose      rigidplacement.RationalRigid
	reference int64
}

func (m RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion) OriginalSTEP() string {
	return m.base.OriginalSTEP()
}
func (m RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion) CorrectedModelSTEP() string {
	return m.corrected
}
func (m RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion) Solid() trimmed.Solid {
	return m.base.Solid()
}

// LocalDescriptor describes the model recipe in the normalized proof view.
func (m RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion) LocalDescriptor() ExactCircularLoopExtrusion {
	return m.base.Descriptor()
}

// Pose directly places Solid in the original world frame.
func (m RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion) Pose() rigidplacement.RationalRigid {
	return m.pose
}
func (m RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion) ReferenceCircleID() int64 {
	return m.reference
}
func (m RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion) Provenance() ConvexArcRepairProvenance {
	return m.base.Provenance()
}
func (m RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion) Repairs() []ConvexArcRepairSite {
	return m.base.Repairs()
}
func (m RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion) Tangencies() []ConvexArcTangentEvidence {
	return m.base.Tangencies()
}
func (m RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion) Reversals() []ConvexArcRepairReversal {
	return m.base.Reversals()
}
func (m RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion) PCurves() []ConvexArcPCurveAgreement {
	return m.base.PCurves()
}
func (m RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion) FiniteLineOwners() []FiniteConvexArcLineOwner {
	return copyFiniteConvexArcLineOwners(m.base.finite.FullDomain.finiteLineOwners)
}
func (m RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion) AxialTrims() []ConvexArcAxialTrimRepair {
	out := make([]ConvexArcAxialTrimRepair, len(m.base.material.axialTrims))
	for i, q := range m.base.material.axialTrims {
		out[i] = q.copied()
	}
	return out
}
func (m RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion) RationalCaps() []GroupedRationalCapEvidence {
	return m.base.RationalCaps()
}
func (m RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion) FiniteTrims() []GroupedCapFiniteTrim {
	return m.base.FiniteTrims()
}

// RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion repairs a complete
// default two-fillet source under a source-derived rational proper pose.
// In addition to the original model, pcurve and rational cap proofs, it
// bounds axial endpoint station shifts and finite planar LINE owner slopes.
func RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(ctx context.Context, r io.Reader, l Limits, o ConvexArcRepairOptions) (RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion, error) {
	var zero RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion
	src, e := readRationalConvexArcSource(ctx, r, l, o, true)
	if e != nil {
		return zero, e
	}
	full, e := proveGroupedConvexArcOwners(ctx, src.local, src.corrected, src.owners, src.material, o.MaxDistance, true)
	if e != nil {
		return zero, fmt.Errorf("rational pose owners: %w", e)
	}
	finite, e := proveGroupedConvexArcCapTrims(ctx, src.local, src.corrected, full, o, geom.ExactCurvePlaneEventOptions{})
	if e != nil {
		return zero, fmt.Errorf("rational pose finite caps: %w", e)
	}
	world, pose, e := src.placeCorrectedModel(ctx)
	if e != nil {
		return zero, e
	}
	finite.FullDomain.modelRepair = src.material
	finite.FullDomain.Repairs = src.material.Repairs()
	base := GroupedPCurveRepairedOCCTConvexArcExtrusion{
		source: src.raw, material: src.material, finite: finite, tolerance: o.MaxDistance,
	}
	return RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion{
		base: base, corrected: world, pose: pose, reference: src.reference,
	}, nil
}
