package step

import (
	"context"
	"io"
)

const independentAnalyticAssemblyNativeFormat = "plainsolid-step-independent-analytic-assembly"

// WriteIndependentAnalyticAssemblyJSON stores three shared local materials and
// all original source/occurrence claims under a separate version-one tag. One
// complete assembly proof compares source, native geometry, local frames and IDs.
// The prior two-product torus assembly format retains its exact meaning/bytes.
func WriteIndependentAnalyticAssemblyJSON(ctx context.Context, w io.Writer, m IndependentAnalyticAssembly) error {
	return writeIndependentTorusAssemblyJSON(ctx, w, IndependentTorusAssembly(m), true)
}

// ReadIndependentAnalyticAssemblyJSON trusts only the reconstructed original
// STEP graph and compares all stored claims/native encodings before returning.
func ReadIndependentAnalyticAssemblyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (IndependentAnalyticAssembly, error) {
	m, e := readIndependentTorusAssemblyJSON(ctx, r, l, true)
	return IndependentAnalyticAssembly(m), e
}
