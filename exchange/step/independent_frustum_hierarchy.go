package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/geom"
)

// IndependentFrustumHierarchy adds one original rational tapered wrapped
// material to the signed-coordinate planar/cylinder/torus/sphere hierarchy.
// This is not a CONICAL_SURFACE angle-to-slope conversion.
type IndependentFrustumHierarchy IndependentAnalyticDAGAssembly
type IndependentFrustumHierarchyModel IndependentTorusAssemblyModel

func ReadIndependentFrustumHierarchy(ctx context.Context, r io.Reader, l Limits, tol float64) (IndependentFrustumHierarchy, error) {
	m, e := readIndependentTorusAssembly(ctx, r, l, tol, true, true, true, true, true)
	if e != nil {
		return IndependentFrustumHierarchy{}, e
	}
	paths, e := independentAnalyticHierarchyPaths(ctx, m, true, true)
	if e != nil {
		return IndependentFrustumHierarchy{}, e
	}
	return IndependentFrustumHierarchy{Assembly: m.Assembly, Products: m.Products, Instances: paths}, nil
}
func WriteIndependentFrustumHierarchy(ctx context.Context, w io.Writer, m IndependentFrustumHierarchyModel, l Limits, tol float64) error {
	return writeIndependentTorusAssembly(ctx, w, IndependentTorusAssemblyModel(m), l, tol, true, true, true, true, true)
}
func WriteIndependentFrustumHierarchySource(ctx context.Context, w io.Writer, f *File, l Limits, tol float64) error {
	return writeIndependentTorusAssemblySource(ctx, w, f, l, tol, true, true, true, true, true)
}
func hasIndependentFrustumSupport(f *File) bool {
	for _, e := range f.Entities {
		for _, r := range e.Records {
			if r.Type == "B_SPLINE_SURFACE_WITH_KNOTS" {
				return true
			}
		}
	}
	return false
}
func readIndependentFrustumOrMaterial(ctx context.Context, f *File, l Limits, tol float64, cylinder, sphere, frustum bool) (IndependentExactMaterial, error) {
	if !frustum || !hasIndependentFrustumSupport(f) {
		return readIndependentExactMaterial(ctx, f, l, tol, true, cylinder, sphere, true)
	}
	if e := ctx.Err(); e != nil {
		return IndependentExactMaterial{}, e
	}
	raw, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return IndependentExactMaterial{}, e
	}
	m, e := ReadWrappedFrusta(bytes.NewReader(raw), l, WrappedSurfaceOptions{MaxSurfaces: 1, MaxPoles: 256, MaxOperations: 1000000, MaxCoefficientBits: 4096})
	if e != nil {
		return IndependentExactMaterial{}, e
	}
	if e = ctx.Err(); e != nil {
		return IndependentExactMaterial{}, e
	}
	if len(m.Bodies) != 1 || !m.Bodies[0].Embedding.Complete {
		return IndependentExactMaterial{}, geom.ErrInvalid
	}
	var body int64
	for _, id := range f.Order {
		for _, r := range f.Entities[id].Records {
			if r.Type == "MANIFOLD_SOLID_BREP" {
				if body != 0 {
					return IndependentExactMaterial{}, geom.ErrInvalid
				}
				body = id
			}
		}
	}
	if body == 0 {
		return IndependentExactMaterial{}, geom.ErrInvalid
	}
	b := m.Bodies[0]
	return IndependentExactMaterial{Name: b.Name, LengthUnit: m.LengthUnit, Source: f, Solid: b.Solid, SourceBodyID: body, Frustum: &b}, nil
}
func stageIndependentFrustumMaterial(ctx context.Context, f *File, raw []byte, l Limits, tol float64, allow bool) (IndependentExactMaterialCollection, error) {
	if allow && hasIndependentFrustumSupport(f) {
		m, e := readIndependentFrustumOrMaterial(ctx, f, l, tol, true, true, true)
		if e != nil {
			return IndependentExactMaterialCollection{}, e
		}
		return IndependentExactMaterialCollection{Source: f, LengthUnit: m.LengthUnit, Members: []IndependentExactMaterial{m}}, nil
	}
	return readIndependentExactMaterialCollection(ctx, bytes.NewReader(raw), l, tol, 1, 1, true, true, true, false, false, true)
}
