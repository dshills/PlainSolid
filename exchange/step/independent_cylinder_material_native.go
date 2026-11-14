package step

import (
	"context"
	"io"
)

const independentCylinderNativeFormat = "plainsolid-step-independent-cylinder-materials"

// WriteIndependentCylinderMaterialsJSON writes a distinct version-one envelope
// after one complete source reproof and comparison of native geometry and all
// product, shape and ancestry claims. Failure before output leaves w untouched.
func WriteIndependentCylinderMaterialsJSON(ctx context.Context, w io.Writer, m IndependentExactMaterialCollection) error {
	return writeIndependentMaterialsJSON(ctx, w, m, true)
}

// ReadIndependentCylinderMaterialsJSON reconstructs from STEP exactly once and
// checks the stored native geometry and ancestry. At least one cylinder product
// is required. The older two-family envelope is deliberately not accepted.
func ReadIndependentCylinderMaterialsJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (IndependentExactMaterialCollection, error) {
	return readIndependentMaterialsJSON(ctx, r, l, true)
}
