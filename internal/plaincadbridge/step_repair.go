package plaincadbridge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"strings"

	rigidplacement "plainsolid/brep/placement"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

type stepRepairRequest struct {
	MaxDistance number `json:"maxDistance"`
	Native      bool   `json:"native,omitempty"`
}

type stepRepairEvidence struct {
	Family           string                         `json:"family"`
	SourceSHA256     string                         `json:"sourceSHA256"`
	LengthUnit       float64                        `json:"lengthUnit"`
	MaxDistance      float64                        `json:"maxDistance"`
	Provenance       step.ConvexArcRepairProvenance `json:"provenance"`
	Repairs          []step.ConvexArcRepairSite     `json:"repairs"`
	PCurveOwners     int                            `json:"pcurveOwners"`
	RationalCaps     int                            `json:"rationalCaps,omitempty"`
	FiniteLineOwners int                            `json:"finiteLineOwners,omitempty"`
	AxialTrims       int                            `json:"axialTrims,omitempty"`
	WorldPose        *[3][4]*big.Rat                `json:"worldPose,omitempty"`
}

type stepRepairMaterial interface {
	OriginalSTEP() string
	Solid() trimmed.Solid
	Provenance() step.ConvexArcRepairProvenance
	Repairs() []step.ConvexArcRepairSite
	PCurves() []step.ConvexArcPCurveAgreement
}

func (b *Bridge) importSTEPWithRepair(ctx context.Context, source string, repair *stepRepairRequest) (Shape, error) {
	if repair == nil {
		return b.importSTEP(source)
	}
	maxDistance := float64(repair.MaxDistance)
	if !finite(maxDistance) || maxDistance <= 0 {
		return Shape{}, errors.New("stepRepair requires a positive finite maxDistance in source length units")
	}
	shape, err := b.importSTEP(source)
	var strictErr *failureError
	if !errors.As(err, &strictErr) || strictErr.code != "UNSUPPORTED" {
		return shape, err
	}
	options := step.ConvexArcRepairOptions{MaxDistance: maxDistance}
	if material, err := step.RepairPCurveOCCTConvexArcExtrusion(ctx, strings.NewReader(source), stepLimits(), options); err == nil {
		return b.putSTEPRepair(ctx, material, *repair)
	}
	if material, err := step.RepairGroupedPCurveOCCTConvexArcExtrusion(ctx, strings.NewReader(source), stepLimits(), options); err == nil {
		return b.putSTEPRepair(ctx, material, *repair)
	}
	if material, err := step.RepairPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, strings.NewReader(source), stepLimits(), options); err == nil {
		return b.putSTEPRepair(ctx, material, *repair)
	}
	if material, err := step.RepairRationalPosedPCurveOCCTConvexArcExtrusion(ctx, strings.NewReader(source), stepLimits(), options); err == nil {
		return b.putSTEPRepair(ctx, material, *repair)
	}
	if material, err := step.RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, strings.NewReader(source), stepLimits(), options); err == nil {
		return b.putSTEPRepair(ctx, material, *repair)
	}
	return Shape{}, unsupportedEdge("STEP source has no qualified default OCCT pcurve repair within maxDistance")
}

func (b *Bridge) putSTEPRepair(ctx context.Context, material stepRepairMaterial, q stepRepairRequest) (Shape, error) {
	evidence := stepRepairEvidence{
		MaxDistance:  float64(q.MaxDistance),
		Provenance:   material.Provenance(),
		Repairs:      material.Repairs(),
		PCurveOwners: len(material.PCurves()),
	}
	digest := sha256.Sum256([]byte(material.OriginalSTEP()))
	evidence.SourceSHA256 = hex.EncodeToString(digest[:])
	var proof bytes.Buffer
	var d step.ExactCircularLoopExtrusion
	var pose *rigidplacement.RationalRigid
	var err error
	switch m := material.(type) {
	case step.PCurveRepairedOCCTConvexArcExtrusion:
		d = m.Descriptor()
		evidence.Family = "occt-single-pcurve-convex-arc"
		if q.Native {
			err = step.WritePCurveRepairedOCCTConvexArcJSON(ctx, &proof, m)
		}
	case step.GroupedPCurveRepairedOCCTConvexArcExtrusion:
		d = m.Descriptor()
		evidence.Family = "occt-grouped-pcurve-convex-arc"
		evidence.RationalCaps = len(m.RationalCaps())
		if q.Native {
			err = step.WriteGroupedPCurveRepairedOCCTConvexArcJSON(ctx, &proof, m)
		}
	case step.PosedGroupedPCurveRepairedOCCTConvexArcExtrusion:
		d = m.Descriptor()
		evidence.Family = "occt-posed-grouped-pcurve-convex-arc"
		evidence.RationalCaps = len(m.RationalCaps())
		if q.Native {
			err = step.WritePosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, &proof, m)
		}
	case step.RationalPosedPCurveRepairedOCCTConvexArcExtrusion:
		d = m.LocalDescriptor()
		p := m.Pose()
		pose = &p
		evidence.Family = "occt-rational-posed-single-pcurve-convex-arc"
		evidence.FiniteLineOwners = len(m.FiniteLineOwners())
		if q.Native {
			err = step.WriteRationalPosedPCurveRepairedOCCTConvexArcJSON(ctx, &proof, m)
		}
	case step.RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion:
		d = m.LocalDescriptor()
		p := m.Pose()
		pose = &p
		evidence.Family = "occt-rational-posed-grouped-pcurve-convex-arc"
		evidence.RationalCaps = len(m.RationalCaps())
		evidence.FiniteLineOwners = len(m.FiniteLineOwners())
		evidence.AxialTrims = len(m.AxialTrims())
		if q.Native {
			err = step.WriteRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx, &proof, m)
		}
	default:
		return Shape{}, errors.New("unrecognized STEP repair material")
	}
	if err != nil {
		return Shape{}, err
	}
	if proof.Len() > MaxRequestBytes {
		return Shape{}, &failureError{"LIMIT", "STEP repair native proof exceeds 1 MiB"}
	}
	evidence.LengthUnit = d.LengthUnit
	var shape Shape
	if pose != nil {
		var matrix [3][4]*big.Rat
		matrix, err = pose.Matrix()
		if err != nil {
			return Shape{}, err
		}
		evidence.WorldPose = &matrix
		// The rational pose already includes the local descriptor's axial origin.
		d.Frame, _ = geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		shape, err = b.putRationalSTEPArcLoop(ctx, material.Solid(), d, *pose)
	} else {
		shape, err = b.putExactSTEPArcLoop(ctx, material.Solid(), d)
	}
	if err != nil {
		return Shape{}, err
	}
	shape.RepairEvidence = &evidence
	if q.Native {
		raw := json.RawMessage(proof.Bytes())
		shape.NativeProof = &raw
	}
	stored := b.shapes[shape.Handle]
	stored.result = shape
	b.shapes[shape.Handle] = stored
	return shape, nil
}
