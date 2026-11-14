package step

import (
	"context"
	"io"
)

const independentSphereNativeFormat = "plainsolid-step-independent-sphere-materials"

// WriteIndependentSphereMaterialsJSON writes a separate version-one envelope
// after one strict source proof, comparing every native geometry and ancestry
// claim. The source must contain a qualified natural sphere product.
func WriteIndependentSphereMaterialsJSON(ctx context.Context, w io.Writer, m IndependentExactMaterialCollection) error {
	return writeIndependentMaterialsJSON(ctx, w, m, true, true)
}

// ReadIndependentSphereMaterialsJSON reconstructs only from the retained STEP
// source, then checks native bytes and all product/shape/role metadata. Older
// envelope tags remain unsupported here and do not accept this new family.
func ReadIndependentSphereMaterialsJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (IndependentExactMaterialCollection, error) {
	return readIndependentMaterialsJSON(ctx, r, l, true, true)
}
