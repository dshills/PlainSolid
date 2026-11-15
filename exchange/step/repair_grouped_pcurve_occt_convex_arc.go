package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// GroupedPCurveRepairedOCCTConvexArcExtrusion owns explicitly reconstructed
// material. Original rational cap curves correspond through bounded radial
// projection and nonlinear reparameterization, never equal source parameters.
type GroupedPCurveRepairedOCCTConvexArcExtrusion struct {
	source    string
	material  RepairedOCCTConvexArcExtrusion
	finite    GroupedFiniteTrimEvidence
	tolerance float64
}

func (m GroupedPCurveRepairedOCCTConvexArcExtrusion) OriginalSTEP() string { return m.source }
func (m GroupedPCurveRepairedOCCTConvexArcExtrusion) CorrectedModelSTEP() string {
	return m.material.CorrectedSTEP()
}
func (m GroupedPCurveRepairedOCCTConvexArcExtrusion) Solid() trimmed.Solid { return m.material.Solid() }
func (m GroupedPCurveRepairedOCCTConvexArcExtrusion) Descriptor() ExactCircularLoopExtrusion {
	return m.material.Descriptor()
}
func (m GroupedPCurveRepairedOCCTConvexArcExtrusion) Repairs() []ConvexArcRepairSite {
	return m.material.Repairs()
}
func (m GroupedPCurveRepairedOCCTConvexArcExtrusion) Reversals() []ConvexArcRepairReversal {
	return m.material.Reversals()
}
func (m GroupedPCurveRepairedOCCTConvexArcExtrusion) Tangencies() []ConvexArcTangentEvidence {
	return m.material.Tangencies()
}
func (m GroupedPCurveRepairedOCCTConvexArcExtrusion) Provenance() ConvexArcRepairProvenance {
	return m.material.Provenance()
}
func (m GroupedPCurveRepairedOCCTConvexArcExtrusion) PCurves() []ConvexArcPCurveAgreement {
	out := append([]ConvexArcPCurveAgreement(nil), m.finite.FullDomain.OtherOwners...)
	for i := range out {
		out[i].DistanceSquaredUpper = new(big.Rat).Set(out[i].DistanceSquaredUpper)
	}
	return out
}
func (m GroupedPCurveRepairedOCCTConvexArcExtrusion) FiniteTrims() []GroupedCapFiniteTrim {
	out := append([]GroupedCapFiniteTrim(nil), m.finite.Trims...)
	for i := range out {
		q := &out[i]
		q.CylinderOwner.DistanceSquaredUpper = new(big.Rat).Set(q.CylinderOwner.DistanceSquaredUpper)
		q.EndPolynomial = append([]*big.Rat(nil), q.EndPolynomial...)
		for j := range q.EndPolynomial {
			q.EndPolynomial[j] = new(big.Rat).Set(q.EndPolynomial[j])
		}
		for j := 0; j < 2; j++ {
			q.FirstSpan[j] = new(big.Rat).Set(q.FirstSpan[j])
			q.EndpointDistanceUpper[j] = new(big.Rat).Set(q.EndpointDistanceUpper[j])
		}
		q.SourceStart = new(big.Rat).Set(q.SourceStart)
		q.SourceEnd = geom.ExactParameterInterval{Lower: new(big.Rat).Set(q.SourceEnd.Lower), Upper: new(big.Rat).Set(q.SourceEnd.Upper)}
	}
	return out
}

// RepairGroupedPCurveOCCTConvexArcExtrusion is a separate explicit two-fillet
// repair. It requires complete model topology, every original owner lift, and
// uniquely selected rational cap intervals with caller-bounded displacements.
// Output is the already proved exact model-only material. Original authoritative
// UV records are retained as evidence, not silently rewritten or discarded.
func RepairGroupedPCurveOCCTConvexArcExtrusion(ctx context.Context, r io.Reader, l Limits, o ConvexArcRepairOptions) (GroupedPCurveRepairedOCCTConvexArcExtrusion, error) {
	var zero GroupedPCurveRepairedOCCTConvexArcExtrusion
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
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return zero, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return zero, geom.ErrConvergence
	}
	proof, e := InspectGroupedConvexArcFiniteTrims(ctx, bytes.NewReader(raw), l, o, geom.ExactCurvePlaneEventOptions{})
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return GroupedPCurveRepairedOCCTConvexArcExtrusion{source: string(raw), material: proof.FullDomain.modelRepair, finite: proof, tolerance: o.MaxDistance}, nil
}

// RationalCaps returns the two original full-domain radial certificates. These
// bounds are geometrically reparameterized; PCurves returns the other twenty-
// eight same-parameter owner bounds.
func (m GroupedPCurveRepairedOCCTConvexArcExtrusion) RationalCaps() []GroupedRationalCapEvidence {
	out := append([]GroupedRationalCapEvidence(nil), m.finite.FullDomain.Caps...)
	for i := range out {
		q := &out[i]
		q.firstSpan = [3][]*big.Rat{}
		q.PoleIDs = append([]int64(nil), q.PoleIDs...)
		q.Knots = append([]*big.Rat(nil), q.Knots...)
		q.Weights = append([]*big.Rat(nil), q.Weights...)
		for j := range q.Knots {
			q.Knots[j] = new(big.Rat).Set(q.Knots[j])
		}
		for j := range q.Weights {
			q.Weights[j] = new(big.Rat).Set(q.Weights[j])
		}
		for j := range q.Domain {
			q.Domain[j] = new(big.Rat).Set(q.Domain[j])
		}
		q.RadialDistanceUpper = new(big.Rat).Set(q.RadialDistanceUpper)
		q.WitnessParameter = new(big.Rat).Set(q.WitnessParameter)
		q.WitnessDistanceSquaredLower = new(big.Rat).Set(q.WitnessDistanceSquaredLower)
	}
	return out
}
