package step

import (
	"context"
	"io"
)

// IndependentAnalyticAssembly has one planar, one exact finite split-cylinder,
// and one model-only natural ring torus product under one flat assembly root.
// All geometry stays local and each occurrence retains its original NAUO ID.
// Unlike IndependentTorusAssembly, this separate API requires all three families.
type IndependentAnalyticAssembly IndependentTorusAssembly

type IndependentAnalyticAssemblyModel IndependentTorusAssemblyModel

// ReadIndependentAnalyticAssembly authenticates exactly three independently
// owned local products and four to sixteen proper signed-coordinate occurrences.
// The torus is reused at least twice; each other product is used at least once.
// Cylinder topology is exactly the previously proved two-half/four-quarter
// model-only family. Rounded pcurves, nesting and other supports are rejected.
func ReadIndependentAnalyticAssembly(ctx context.Context, r io.Reader, l Limits, tol float64) (IndependentAnalyticAssembly, error) {
	m, e := readIndependentTorusAssembly(ctx, r, l, tol, true)
	return IndependentAnalyticAssembly(m), e
}

// WriteIndependentAnalyticAssembly authors and re-proves a standard flat
// product graph from separately authenticated local STEP product sources.
func WriteIndependentAnalyticAssembly(ctx context.Context, w io.Writer, m IndependentAnalyticAssemblyModel, l Limits, tol float64) error {
	return writeIndependentTorusAssembly(ctx, w, IndependentTorusAssemblyModel(m), l, tol, true)
}

// WriteIndependentAnalyticAssemblySource retains original product/occurrence
// and local support IDs after re-proving the complete accepted graph.
func WriteIndependentAnalyticAssemblySource(ctx context.Context, w io.Writer, source *File, l Limits, tol float64) error {
	return writeIndependentTorusAssemblySource(ctx, w, source, l, tol, true)
}
