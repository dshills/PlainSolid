package step

import (
	"context"
	"io"
)

const independentTorusNativeFormat = "plainsolid-step-independent-toroidal-materials"

// WriteIndependentToroidalMaterialsJSON writes the distinct version-one torus-
// inclusive envelope after one complete source proof and comparison of native
// geometry, original seam/vertex roles and every product/shape ancestry claim.
func WriteIndependentToroidalMaterialsJSON(ctx context.Context, w io.Writer, m IndependentExactMaterialCollection) error {
	return writeIndependentMaterialsJSON(ctx, w, m, true, true, true)
}

// ReadIndependentToroidalMaterialsJSON reconstructs only through the strict
// model-only torus collection codec. Rounded vendor seam charts remain rejected.
func ReadIndependentToroidalMaterialsJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (IndependentExactMaterialCollection, error) {
	return readIndependentMaterialsJSON(ctx, r, l, true, true, true)
}
