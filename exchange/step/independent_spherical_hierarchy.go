package step

import (
	"context"
	"io"

	"plainsolid/geom"
)

// IndependentSphericalHierarchy adds an optional exact sphere local product to
// the required planar/cylinder/torus hierarchy. All local products must be used.
// It retains original product/NAUO IDs, shared assets and expanded rigid paths.
type IndependentSphericalHierarchy IndependentAnalyticDAGAssembly

// IndependentSphericalHierarchyModel accepts authenticated local source graphs
// and nil Source for assembly nodes; geometry is never borrowed between products.
type IndependentSphericalHierarchyModel IndependentTorusAssemblyModel

// SphericalHierarchyNativeFaceSources returns an owned index-to-entity map
// for one already authenticated material. Multiple canonical native faces may
// realize a single original analytic STEP face. The body's local geometry and
// entity graph must come from the same hierarchy read.
func SphericalHierarchyNativeFaceSources(body IndependentExactMaterial) ([]int64, error) {
	var ids []int64
	n := len(body.Solid.Faces())
	switch {
	case body.Planar != nil:
		if body.Source == nil {
			return nil, geom.ErrInvalid
		}
		manifold, e := simple(body.Source, body.SourceBodyID, "MANIFOLD_SOLID_BREP", 2)
		if e != nil {
			return nil, e
		}
		shell, e := simple(body.Source, manifold.Args[1].Ref, "CLOSED_SHELL", 2)
		if e != nil {
			return nil, e
		}
		// The advanced planar decoder constructs faces in the shell's
		// ordered reference list. The native index is retained verbatim.
		ids, e = listRefs(shell.Args[1], 1)
		if e != nil {
			return nil, e
		}
		n = len(body.Planar.Solid.Faces())
	case body.Cylinder != nil:
		ids = body.Cylinder.NativeFaceSourceIDs[:]
	case body.Torus != nil:
		ids = body.Torus.NativeFaceSourceIDs
	case body.Sphere != nil:
		ids = body.Sphere.NativeFaceSourceIDs
	default:
		return nil, geom.ErrUnrepresentable
	}
	if n == 0 || n != len(ids) {
		return nil, geom.ErrUnrepresentable
	}
	for _, id := range ids {
		if id <= 0 {
			return nil, geom.ErrUnrepresentable
		}
	}
	return append([]int64(nil), ids...), nil
}

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
