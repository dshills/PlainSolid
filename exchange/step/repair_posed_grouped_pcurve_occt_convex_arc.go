package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// PosedGroupedPCurveRepairedOCCTConvexArcExtrusion retains original world STEP
// and world repair evidence while its exact native solid stays product-local.
// Descriptor.Frame is the represented placement; UV source records are never
// rotated or reparameterized by the pose normalization.
type PosedGroupedPCurveRepairedOCCTConvexArcExtrusion struct {
	base      GroupedPCurveRepairedOCCTConvexArcExtrusion
	rotation  geom.Affine
	reference int64
}

func (m PosedGroupedPCurveRepairedOCCTConvexArcExtrusion) OriginalSTEP() string {
	return m.base.OriginalSTEP()
}
func (m PosedGroupedPCurveRepairedOCCTConvexArcExtrusion) CorrectedModelSTEP() string {
	return m.base.CorrectedModelSTEP()
}
func (m PosedGroupedPCurveRepairedOCCTConvexArcExtrusion) Solid() trimmed.Solid {
	return m.base.Solid()
}
func (m PosedGroupedPCurveRepairedOCCTConvexArcExtrusion) Descriptor() ExactCircularLoopExtrusion {
	return m.base.Descriptor()
}
func (m PosedGroupedPCurveRepairedOCCTConvexArcExtrusion) Repairs() []ConvexArcRepairSite {
	return m.base.Repairs()
}
func (m PosedGroupedPCurveRepairedOCCTConvexArcExtrusion) Reversals() []ConvexArcRepairReversal {
	return m.base.Reversals()
}
func (m PosedGroupedPCurveRepairedOCCTConvexArcExtrusion) Tangencies() []ConvexArcTangentEvidence {
	return m.base.Tangencies()
}
func (m PosedGroupedPCurveRepairedOCCTConvexArcExtrusion) Provenance() ConvexArcRepairProvenance {
	return m.base.Provenance()
}
func (m PosedGroupedPCurveRepairedOCCTConvexArcExtrusion) PCurves() []ConvexArcPCurveAgreement {
	return m.base.PCurves()
}
func (m PosedGroupedPCurveRepairedOCCTConvexArcExtrusion) RationalCaps() []GroupedRationalCapEvidence {
	return m.base.RationalCaps()
}
func (m PosedGroupedPCurveRepairedOCCTConvexArcExtrusion) FiniteTrims() []GroupedCapFiniteTrim {
	return m.base.FiniteTrims()
}
func (m PosedGroupedPCurveRepairedOCCTConvexArcExtrusion) NormalizationRotation() geom.Affine {
	return m.rotation
}
func (m PosedGroupedPCurveRepairedOCCTConvexArcExtrusion) ReferenceCircleID() int64 {
	return m.reference
}

// RepairPosedGroupedPCurveOCCTConvexArcExtrusion adds an exactly represented
// proper signed-coordinate rotation to grouped default-pcurve repair. All UV
// graphs stay in their original owner charts; original translation remains in
// profile coordinates and Descriptor.Frame. No approximate pose is accepted.
func RepairPosedGroupedPCurveOCCTConvexArcExtrusion(ctx context.Context, r io.Reader, l Limits, o ConvexArcRepairOptions) (PosedGroupedPCurveRepairedOCCTConvexArcExtrusion, error) {
	var zero PosedGroupedPCurveRepairedOCCTConvexArcExtrusion
	if ctx == nil || r == nil {
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
	// This view is not acceptance: all removed owner records are authenticated
	// and subsequently proved in the unchanged full-graph UV mode below.
	view, _, e := convexPCurveModelView(f, 15, true)
	if e != nil {
		return zero, e
	}
	modelRaw, e := mappedPlanarSourceBytes(view)
	if e != nil {
		return zero, e
	}
	posed, e := RepairPosedGroupedOCCTConvexArcExtrusion(ctx, bytes.NewReader(modelRaw), l, o)
	if e != nil {
		return zero, e
	}
	body, e := nauoClosure(view, []int64{posed.Provenance().Body}, 16)
	if e != nil {
		return zero, e
	}
	var metadata map[int64]bool
	for _, id := range view.Order {
		rep, err := simple(view, id, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
		if err != nil {
			continue
		}
		items, err := listRefs(rep.Args[1], 2)
		if err != nil || len(items) != 2 {
			return zero, geom.ErrDomain
		}
		metadata, e = nauoClosure(view, []int64{items[0]}, 16)
		if e != nil {
			return zero, e
		}
	}
	if metadata == nil {
		return zero, geom.ErrDomain
	}
	local, e := convexArcRotateView(ctx, f, body, metadata, posed.rotation, true)
	if e != nil {
		return zero, e
	}
	localRaw, e := mappedPlanarSourceBytes(local)
	if e != nil {
		return zero, e
	}
	proof, e := RepairGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewReader(localRaw), l, o)
	if e != nil {
		return zero, e
	}
	// The independently proved world model and full local pcurve proof must
	// recover identical local native material and source topology identities.
	a, e := prismNativeBytes(posed.Solid())
	if e != nil {
		return zero, e
	}
	b, e := prismNativeBytes(proof.Solid())
	if e != nil {
		return zero, e
	}
	if !bytes.Equal(a, b) {
		return zero, geom.ErrDomain
	}
	pa, e := json.Marshal(posed.Provenance())
	if e != nil {
		return zero, e
	}
	pb, e := json.Marshal(proof.Provenance())
	if e != nil {
		return zero, e
	}
	if !bytes.Equal(pa, pb) {
		return zero, geom.ErrDomain
	}
	// The proper rotation transports all metric bounds unchanged. Existing
	// posed model repair separately checks each original world point and squared
	// displacement, then re-reads the corrected world analytic graph.
	proof.source = string(raw)
	proof.material = posed.base
	proof.finite.FullDomain.modelRepair = posed.base
	proof.finite.FullDomain.Repairs = posed.Repairs()
	proof.finite.FullDomain.Reversals = posed.Reversals()
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return PosedGroupedPCurveRepairedOCCTConvexArcExtrusion{proof, posed.rotation, posed.reference}, nil
}
