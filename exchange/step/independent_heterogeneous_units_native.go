package step

import (
	"context"
	"io"
	"plainsolid/geom"
)

const normalizedMixedFormat = "plainsolid-step-normalized-independent-materials"

// WriteNormalizedIndependentMaterialsJSON writes the distinct heterogeneous v1
// envelope after complete original/normalized family-specific source reproof.
// It retains original SI declarations, root roles, native geometry and exact
// planar speeds without applying that speed allowance to round products.
func WriteNormalizedIndependentMaterialsJSON(ctx context.Context, w io.Writer, m NormalizedIndependentMaterials) error {
	if len(m.OriginalUnits) < 1 || len(m.OriginalUnits) > 16 || len(m.OriginalUnits) != len(m.Members) {
		return geom.ErrInvalid
	}
	q := NormalizedIndependentRoundMaterials{OriginalSource: m.OriginalSource, NormalizedSource: m.NormalizedSource, Members: m.Members, TargetUnit: RoundMaterialSIUnit(m.TargetUnit), tolerance: m.tolerance}
	for _, u := range m.OriginalUnits {
		q.OriginalUnits = append(q.OriginalUnits, RoundMaterialSIUnit(u))
	}
	return writeNormalizedMaterialsJSON(ctx, w, q, false, true)
}

// ReadNormalizedIndependentMaterialsJSON reconstructs from original STEP once
// through heterogeneous normalization. Neither normalized source nor native
// payload is trusted as reconstruction input; every stored claim must match.
func ReadNormalizedIndependentMaterialsJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (NormalizedIndependentMaterials, error) {
	m, e := readNormalizedMaterialsJSON(ctx, r, l, false, true)
	if e != nil {
		return NormalizedIndependentMaterials{}, e
	}
	q := NormalizedIndependentMaterials{OriginalSource: m.OriginalSource, NormalizedSource: m.NormalizedSource, Members: m.Members, TargetUnit: IndependentMaterialSIUnit(m.TargetUnit), tolerance: m.tolerance}
	for _, u := range m.OriginalUnits {
		q.OriginalUnits = append(q.OriginalUnits, IndependentMaterialSIUnit(u))
	}
	return q, nil
}
