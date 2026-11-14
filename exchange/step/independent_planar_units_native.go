package step

import (
	"context"
	"io"
	"plainsolid/geom"
)

const normalizedPlanarFormat = "plainsolid-step-normalized-independent-planar-materials"

// WriteNormalizedIndependentPlanarMaterialsJSON re-proves the original planar
// source and exact dimensional normalization before comparing every source,
// pcurve/speed, product, native and ancestry claim. Its version-one tag is
// deliberately separate from round-material and unnormalized envelopes.
func WriteNormalizedIndependentPlanarMaterialsJSON(ctx context.Context, w io.Writer, m NormalizedIndependentPlanarMaterials) error {
	if len(m.OriginalUnits) < 1 || len(m.OriginalUnits) > 16 || len(m.OriginalUnits) != len(m.Members) {
		return geom.ErrInvalid
	}
	q := NormalizedIndependentRoundMaterials{OriginalSource: m.OriginalSource, NormalizedSource: m.NormalizedSource, Members: m.Members, TargetUnit: RoundMaterialSIUnit(m.TargetUnit), tolerance: m.tolerance}
	for _, u := range m.OriginalUnits {
		q.OriginalUnits = append(q.OriginalUnits, RoundMaterialSIUnit(u))
	}
	return writeNormalizedMaterialsJSON(ctx, w, q, true)
}

// ReadNormalizedIndependentPlanarMaterialsJSON invokes the bounded planar
// normalization pipeline once, reconstructing from OriginalSource rather than
// trusting stored normalized/native geometry. All stored graph claims must match.
func ReadNormalizedIndependentPlanarMaterialsJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (NormalizedIndependentPlanarMaterials, error) {
	m, e := readNormalizedMaterialsJSON(ctx, r, l, true)
	if e != nil {
		return NormalizedIndependentPlanarMaterials{}, e
	}
	q := NormalizedIndependentPlanarMaterials{OriginalSource: m.OriginalSource, NormalizedSource: m.NormalizedSource, Members: m.Members, TargetUnit: PlanarMaterialSIUnit(m.TargetUnit), tolerance: m.tolerance}
	for _, u := range m.OriginalUnits {
		q.OriginalUnits = append(q.OriginalUnits, PlanarMaterialSIUnit(u))
	}
	return q, nil
}
