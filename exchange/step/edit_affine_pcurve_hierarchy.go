package step

import (
	"bytes"
	"context"
	"io"
	"reflect"

	"plainsolid/geom"
)

// WriteEditedSTEP applies an exact local occurrence edit directly to the
// authored source graph. Original spline pcurves and their record IDs are
// retained. All local material, occurrence paths, and affine evidence are
// re-proved against the staged STEP before publication.
func (m AffinePCurveHierarchy) WriteEditedSTEP(ctx context.Context, w io.Writer, edits []RationalSphericalHierarchyEdit, l Limits) error {
	if ctx == nil || w == nil || len(edits) > 64 || m.original == "" {
		return geom.ErrInvalid
	}
	base, err := readAffinePCurveHierarchy(ctx, bytes.NewBufferString(m.original), l, m.tolerance, m.vendor)
	if err != nil {
		return err
	}
	if len(edits) == 0 {
		return writeRationalHierarchyEditBytes(ctx, w, []byte(m.original))
	}
	// The proof view has replacement LINEs and synthetic vectors. It can be
	// used for hierarchy facts only; edit the separate original parse tree.
	f, err := Parse(bytes.NewBufferString(m.original), l)
	if err != nil {
		return err
	}
	known, err := applyRationalHierarchyEdits(ctx, f, base.model, edits, m.vendor)
	if err != nil {
		return err
	}
	raw, err := mappedPlanarSourceBytes(f)
	if err != nil {
		return err
	}
	fresh, err := readAffinePCurveHierarchy(ctx, bytes.NewReader(raw), l, m.tolerance, m.vendor)
	if err != nil {
		return err
	}
	if !matchRationalHierarchyEdits(base.model, fresh.model, known) || !reflect.DeepEqual(fresh.evidence, base.evidence) ||
		!sameAffineHierarchyProducts(base.model, fresh.model) {
		return geom.ErrUnrepresentable
	}
	return writeRationalHierarchyEditBytes(ctx, w, raw)
}

func sameAffineHierarchyProducts(before, after RationalSphericalHierarchy) bool {
	if before.LengthUnit != after.LengthUnit || len(before.Products) != len(after.Products) || len(before.Instances) != len(after.Instances) {
		return false
	}
	for i, v := range before.Instances {
		a := after.Instances[i]
		if a.ProductID != v.ProductID || !reflect.DeepEqual(a.Path, v.Path) {
			return false
		}
	}
	for id, p := range before.Products {
		a, ok := after.Products[id]
		if !ok || p.SourceProductID != a.SourceProductID || p.SourceBodyID != a.SourceBodyID {
			return false
		}
		faces, err := SphericalHierarchyNativeFaceSources(p)
		if err != nil {
			return false
		}
		other, err := SphericalHierarchyNativeFaceSources(a)
		if err != nil || !reflect.DeepEqual(faces, other) {
			return false
		}
	}
	return true
}
