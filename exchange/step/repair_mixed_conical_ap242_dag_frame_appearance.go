package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"slices"

	"plainsolid/geom"
)

// MixedConicalAP242DAGFrameAppearanceRepairOptions selects one rounded
// contextual occurrence frame and a separate budget for repairing the
// original local surfaces. The frame displacement bound applies to the
// repaired exact local solid in original file units.
type MixedConicalAP242DAGFrameAppearanceRepairOptions struct {
	Frame    MixedConicalAP242FrameRepairOptions
	Geometry MixedConicalAP242DAGRepairOptions
}

// RepairedMixedConicalAP242DAGFrameAppearance retains the original styled
// source, its repaired occurrence frame and local solids, and the strictly
// reopenable carrier with the original appearance owners.
type RepairedMixedConicalAP242DAGFrameAppearance struct {
	original string
	options  MixedConicalAP242DAGFrameAppearanceRepairOptions
	frame    MixedConicalAP242FrameRepairEvidence
	geometry RepairedMixedConicalAP242DAGAppearance
}

// OriginalSTEP returns the original bytes before either correction.
func (m RepairedMixedConicalAP242DAGFrameAppearance) OriginalSTEP() string { return m.original }

// CorrectedModelSTEP returns the strict styled carrier after both repairs.
func (m RepairedMixedConicalAP242DAGFrameAppearance) CorrectedModelSTEP() string {
	return m.geometry.CorrectedModelSTEP()
}

// Appearance returns the corrected exact DAG and its original style owners.
func (m RepairedMixedConicalAP242DAGFrameAppearance) Appearance() MixedConicalAP242DAGAppearance {
	return m.geometry.Appearance()
}

// FrameEvidence returns the original relationship IDs and certified correction.
func (m RepairedMixedConicalAP242DAGFrameAppearance) FrameEvidence() MixedConicalAP242FrameRepairEvidence {
	return m.frame
}

// PartRepairs returns the two local material proofs with original owner IDs.
func (m RepairedMixedConicalAP242DAGFrameAppearance) PartRepairs() [2]NestedMixedConicalAP242PartRepair {
	return m.geometry.PartRepairs()
}

// RepairMixedConicalAP242DAGFrameAppearance explicitly repairs the rounded
// terminal rolling-rim frame used by the original SHUO context and then
// repairs the two rounded local solids. Both budgets must pass and the final
// carrier must strictly reopen with the original hierarchy and style owners.
func RepairMixedConicalAP242DAGFrameAppearance(ctx context.Context, r io.Reader, l MixedConicalAP242DAGLimits, o MixedConicalAP242DAGFrameAppearanceRepairOptions) (RepairedMixedConicalAP242DAGFrameAppearance, error) {
	var zero RepairedMixedConicalAP242DAGFrameAppearance
	if ctx == nil || r == nil || !finiteNumber(o.Geometry.MaxDistance) || o.Geometry.MaxDistance <= 0 {
		return zero, geom.ErrInvalid
	}
	frame, err := normalizeMixedConicalAP242FrameRepairOptions(o.Frame)
	if err != nil {
		return zero, err
	}
	o.Frame = frame
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	l, err = mixedConicalAP242DAGLimits(l)
	if err != nil {
		return zero, err
	}
	raw, err := io.ReadAll(io.LimitReader(r, l.Source.MaxBytes+1))
	if err != nil {
		return zero, err
	}
	if int64(len(raw)) > l.Source.MaxBytes {
		return zero, geom.ErrConvergence
	}
	style, err := readMixedConicalDAGStyleSource(raw, l.Source)
	if err != nil {
		return zero, fmt.Errorf("step: authenticate original AP242 styles: %w", err)
	}
	a, err := ReadAssembly(bytes.NewReader(style.unstyled), l.Source)
	if err != nil {
		return zero, err
	}
	pending, err := inferMixedConicalAP242FrameRepair(ctx, a, o.Frame)
	if err != nil {
		return zero, err
	}
	// The unstyled view established the two ordinary incoming references
	// to the placement. The authenticated SHUO context is the only extra
	// owner allowed in the full source, and neither direction may be shared.
	use := pending.occ
	directions := pending.evidence.DirectionIDs
	incoming := conicalRepairIncoming(style.file)
	if use.NAUOID != style.shuo.path[2] || use.ToPlacementID != style.override.PlacementID ||
		incoming[use.ToPlacementID] != 3 || incoming[directions[0]] != 1 || incoming[directions[1]] != 1 {
		return zero, fmt.Errorf("step: original occurrence frame is outside the scoped SHUO ownership: %w", geom.ErrUnrepresentable)
	}
	pending.apply(style.file)
	carrier, err := mappedPlanarSourceBytes(style.file)
	if err != nil {
		return zero, err
	}
	geometry, err := RepairMixedConicalAP242DAGAppearance(ctx, bytes.NewReader(carrier), l, o.Geometry)
	if err != nil {
		return zero, err
	}
	model := geometry.repair.assembly
	evidence, err := pending.prove(ctx, model, o.Frame.MaxDisplacement)
	if err != nil {
		return zero, err
	}
	original, err := style.bind(model, string(raw))
	if err != nil {
		return zero, err
	}
	if !slices.Equal(original.ProductStyles, geometry.productStyles) || !slices.Equal(original.FaceStyles, geometry.faceStyles) ||
		original.PathOverride != geometry.pathOverride {
		return zero, errMixedConicalAP242DAGAppearance
	}
	return RepairedMixedConicalAP242DAGFrameAppearance{original: string(raw), options: o, frame: evidence, geometry: geometry}, nil
}
