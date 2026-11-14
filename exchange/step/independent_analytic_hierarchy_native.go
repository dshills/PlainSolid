package step

import (
	"context"
	"io"
)

const independentAnalyticHierarchyNativeFormat = "plainsolid-step-independent-analytic-hierarchy"
const independentAnalyticHierarchyClaimsFormat = "plainsolid-step-independent-analytic-hierarchy-claims"

func readIndependentAnalyticPathProof(ctx context.Context, r io.Reader, l Limits, tol float64, hierarchy bool) (IndependentAnalyticDAGAssembly, error) {
	if hierarchy {
		m, e := ReadIndependentAnalyticHierarchy(ctx, r, l, tol)
		return IndependentAnalyticDAGAssembly(m), e
	}
	return ReadIndependentAnalyticDAGAssembly(ctx, r, l, tol)
}

// WriteIndependentAnalyticHierarchyJSON uses a separate version-one source-bound
// tag, re-proving local materials, graph ownership and all bounded leaf paths.
func WriteIndependentAnalyticHierarchyJSON(ctx context.Context, w io.Writer, m IndependentAnalyticHierarchy) error {
	return writeIndependentAnalyticDAGJSON(ctx, w, IndependentAnalyticDAGAssembly(m), true)
}

// ReadIndependentAnalyticHierarchyJSON reconstructs only from the original STEP;
// native bytes, original IDs and exact composed paths must match that proof.
func ReadIndependentAnalyticHierarchyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (IndependentAnalyticHierarchy, error) {
	m, e := readIndependentAnalyticDAGJSON(ctx, r, l, true)
	return IndependentAnalyticHierarchy(m), e
}
