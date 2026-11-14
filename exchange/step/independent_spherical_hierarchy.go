package step

import (
	"context"
	"io"
)

// IndependentSphericalHierarchy adds an optional exact sphere local product to
// the required planar/cylinder/torus hierarchy. All local products must be used.
// It retains original product/NAUO IDs, shared assets and expanded rigid paths.
type IndependentSphericalHierarchy IndependentAnalyticDAGAssembly

// IndependentSphericalHierarchyModel accepts authenticated local source graphs
// and nil Source for assembly nodes; geometry is never borrowed between products.
type IndependentSphericalHierarchyModel IndependentTorusAssemblyModel

// ReadIndependentSphericalHierarchy retains the existing limits (depth8,
// 64 expanded uses, 16 products) with zero or one original exact sphere leaf.
// Original source units must be common metre/mm and frames proper signed rigid.
func ReadIndependentSphericalHierarchy(ctx context.Context, r io.Reader, l Limits, tol float64) (IndependentSphericalHierarchy, error) {
	m, e := readIndependentTorusAssembly(ctx, r, l, tol, true, true, true, true)
	if e != nil {
		return IndependentSphericalHierarchy{}, e
	}
	paths, e := independentAnalyticHierarchyPaths(ctx, m, true)
	if e != nil {
		return IndependentSphericalHierarchy{}, e
	}
	return IndependentSphericalHierarchy{Assembly: m.Assembly, Products: m.Products, Instances: paths}, nil
}

// WriteIndependentSphericalHierarchy stages all original local geometry and
// standard relationships, and re-proves the complete graph before output.
func WriteIndependentSphericalHierarchy(ctx context.Context, w io.Writer, m IndependentSphericalHierarchyModel, l Limits, tol float64) error {
	return writeIndependentTorusAssembly(ctx, w, IndependentTorusAssemblyModel(m), l, tol, true, true, true, true)
}

// WriteIndependentSphericalHierarchySource re-proves and preserves original IDs.
func WriteIndependentSphericalHierarchySource(ctx context.Context, w io.Writer, source *File, l Limits, tol float64) error {
	return writeIndependentTorusAssemblySource(ctx, w, source, l, tol, true, true, true, true)
}
