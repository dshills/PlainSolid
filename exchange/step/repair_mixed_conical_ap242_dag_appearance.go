package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"

	"plainsolid/geom"
)

// RepairedMixedConicalAP242DAGAppearance retains the original styled source,
// its strictly reopenable correction and the proofs of both local materials.
type RepairedMixedConicalAP242DAGAppearance struct {
	repair                    RepairedMixedConicalAP242DAG
	productStyles, faceStyles []MixedConicalAP242SurfaceStyle
	pathOverride              DeepSHUOColor
}

// OriginalSTEP returns the unmodified, independently authored input.
func (m RepairedMixedConicalAP242DAGAppearance) OriginalSTEP() string { return m.repair.original }

// CorrectedModelSTEP returns the strictly reopenable styled correction.
func (m RepairedMixedConicalAP242DAGAppearance) CorrectedModelSTEP() string {
	return m.repair.corrected
}

// Appearance returns the corrected exact assembly and its original style
// owners. Its OriginalSTEP is the corrected styled carrier.
func (m RepairedMixedConicalAP242DAGAppearance) Appearance() MixedConicalAP242DAGAppearance {
	a := m.repair.Assembly()
	if m.repair.corrected != "" {
		// Return a private source index. The stored carrier already passed
		// this parser under the caller's limits during strict verification.
		a.Assembly.Source, _ = Parse(strings.NewReader(m.repair.corrected), Limits{})
	}
	return MixedConicalAP242DAGAppearance{
		DAG: a, ProductStyles: slices.Clone(m.productStyles), FaceStyles: slices.Clone(m.faceStyles),
		PathOverride: m.pathOverride, original: m.repair.corrected,
	}
}

// PartRepairs returns the source-bound proofs for the two original products.
func (m RepairedMixedConicalAP242DAGAppearance) PartRepairs() [2]NestedMixedConicalAP242PartRepair {
	return m.repair.PartRepairs()
}

// RepairMixedConicalAP242DAGAppearance proves the original styled hierarchy
// and its complete override path, then corrects each local solid once under
// the requested displacement bound in file units. Presentation and original
// usage IDs remain fixed. The corrected file must pass the strict styled reader.
func RepairMixedConicalAP242DAGAppearance(ctx context.Context, r io.Reader, l MixedConicalAP242DAGLimits, o MixedConicalAP242DAGRepairOptions) (RepairedMixedConicalAP242DAGAppearance, error) {
	var zero RepairedMixedConicalAP242DAGAppearance
	if ctx == nil || r == nil || !finiteNumber(o.MaxDistance) || o.MaxDistance <= 0 {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	l, err := mixedConicalAP242DAGLimits(l)
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
	h, err := readMixedConicalAP242HierarchyMode(bytes.NewReader(style.unstyled), l.Source, mixedConicalAP242DAG)
	if err != nil {
		return zero, fmt.Errorf("step: authenticate rounded styled AP242 hierarchy: %w", err)
	}
	worlds, err := mixedConicalAP242DAGWorlds(ctx, h.assembly.Roots[0], h.records.occurrences, l)
	if err != nil {
		return zero, err
	}
	hierarchy := MixedConicalAP242DAG{Assembly: h.assembly, Occurrences: h.records.occurrences, Instances: worlds}
	overrideProduct := int64(0)
	for definition, body := range h.layout.parts {
		if body == style.override.BodyID {
			overrideProduct = definition
		}
	}
	if _, err := bindMixedConicalDAGSHUO(hierarchy, overrideProduct, style.shuo, style.override); err != nil {
		return zero, err
	}
	corrected, parts, err := repairMixedConicalAP242PartsInto(ctx, h, style.file, l.Source, o.MaxDistance)
	if err != nil {
		return zero, err
	}
	a, err := ReadMixedConicalAP242DAGAppearance(ctx, bytes.NewReader(corrected), l, geom.SurfacePatchOptions{})
	if err != nil {
		return zero, fmt.Errorf("step: verify corrected styled AP242 DAG: %w", err)
	}
	post := MixedConicalAP242Assembly{Assembly: a.DAG.Assembly, Parts: a.DAG.Parts, Occurrences: a.DAG.Occurrences}
	if err := verifyMixedConicalAP242Repair(h, post, parts); err != nil {
		return zero, err
	}
	if !reflect.DeepEqual(worlds, a.DAG.Instances) {
		return zero, geom.ErrInvalid
	}
	// The merge kept the exact original body and face IDs. Use their proved
	// native indices to compare the full original and corrected style owners.
	before, err := style.bind(a.DAG, string(raw))
	if err != nil {
		return zero, err
	}
	if !slices.Equal(before.ProductStyles, a.ProductStyles) || !slices.Equal(before.FaceStyles, a.FaceStyles) ||
		before.PathOverride != a.PathOverride {
		return zero, errMixedConicalAP242DAGAppearance
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return RepairedMixedConicalAP242DAGAppearance{
		repair: RepairedMixedConicalAP242DAG{original: string(raw), corrected: string(corrected), maxDistance: o.MaxDistance,
			assembly: a.DAG, parts: parts},
		productStyles: a.ProductStyles, faceStyles: a.FaceStyles, pathOverride: a.PathOverride,
	}, nil
}
