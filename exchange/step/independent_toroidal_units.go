package step

import (
	"context"
	"io"
	"plainsolid/geom"
)

// ReadNormalizedIndependentToroidalMaterials adds only the proven model-only
// natural ring torus to heterogeneous SI normalization. At least one torus is
// required. Both radii, all model points and seam-circle radii scale exactly;
// no angular or pcurve fields are admitted or repaired for torus products.
// Existing normalization and persistence APIs retain their previous families.
func ReadNormalizedIndependentToroidalMaterials(ctx context.Context, r io.Reader, target IndependentMaterialSIUnit, l Limits, tol float64) (NormalizedIndependentMaterials, error) {
	m, e := readNormalizedIndependentUnits(ctx, r, RoundMaterialSIUnit(target), l, tol, false, true, true)
	if e != nil {
		return NormalizedIndependentMaterials{}, e
	}
	found := false
	for _, p := range m.Members {
		found = found || p.Torus != nil
	}
	if !found {
		return NormalizedIndependentMaterials{}, geom.ErrInvalid
	}
	q := NormalizedIndependentMaterials{OriginalSource: m.OriginalSource, NormalizedSource: m.NormalizedSource, Members: m.Members, TargetUnit: target, tolerance: m.tolerance}
	for _, u := range m.OriginalUnits {
		q.OriginalUnits = append(q.OriginalUnits, IndependentMaterialSIUnit(u))
	}
	return q, nil
}
