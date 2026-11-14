package step

import (
	"context"
	"io"
	"plainsolid/geom"
)

const normalizedToroidalFormat = "plainsolid-step-normalized-independent-toroidal-materials"

// WriteNormalizedIndependentToroidalMaterialsJSON uses a separate version-one
// source-bound tag. Both original and normalized torus seam/vertex proofs and
// every product's native/ancestry claims are checked before any output.
func WriteNormalizedIndependentToroidalMaterialsJSON(ctx context.Context, w io.Writer, m NormalizedIndependentMaterials) error {
	if len(m.OriginalUnits) < 1 || len(m.OriginalUnits) > 16 || len(m.OriginalUnits) != len(m.Members) {
		return geom.ErrInvalid
	}
	q := NormalizedIndependentRoundMaterials{OriginalSource: m.OriginalSource, NormalizedSource: m.NormalizedSource, Members: m.Members, TargetUnit: RoundMaterialSIUnit(m.TargetUnit), tolerance: m.tolerance}
	for _, u := range m.OriginalUnits {
		q.OriginalUnits = append(q.OriginalUnits, RoundMaterialSIUnit(u))
	}
	return writeNormalizedMaterialsJSON(ctx, w, q, false, true, true)
}

// ReadNormalizedIndependentToroidalMaterialsJSON reconstructs from original STEP
// through the strict torus-inclusive normalization pipeline and compares all
// stored graphs and native claims. At least one authenticated torus is required.
func ReadNormalizedIndependentToroidalMaterialsJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (NormalizedIndependentMaterials, error) {
	m, e := readNormalizedMaterialsJSON(ctx, r, l, false, true, true)
	if e != nil {
		return NormalizedIndependentMaterials{}, e
	}
	q := NormalizedIndependentMaterials{OriginalSource: m.OriginalSource, NormalizedSource: m.NormalizedSource, Members: m.Members, TargetUnit: IndependentMaterialSIUnit(m.TargetUnit), tolerance: m.tolerance}
	for _, u := range m.OriginalUnits {
		q.OriginalUnits = append(q.OriginalUnits, IndependentMaterialSIUnit(u))
	}
	return q, nil
}
