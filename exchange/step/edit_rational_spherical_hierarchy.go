package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"reflect"

	"plainsolid/geom"
)

// RationalSphericalHierarchyEdit specifies an exact child-to-parent pose and
// optionally a new name for one original relationship. All expanded paths
// through the same NAUO adopt the edit. Caller rationals are copied.
type RationalSphericalHierarchyEdit struct {
	NAUOID int64
	Matrix [3][4]*big.Rat
	Name   *string
}

// WriteRationalSphericalHierarchyEdits updates the source under the original
// neutral-child convention, preserving local material, product and relationship
// IDs. The result is fully re-proved before any data is written.
func WriteRationalSphericalHierarchyEdits(ctx context.Context, w io.Writer, source *File, edits []RationalSphericalHierarchyEdit, l Limits, tol float64) error {
	return writeRationalSphericalHierarchyEdits(ctx, w, source, edits, l, tol, false)
}

// WriteOCCTRationalSphericalHierarchyEdits uses the nonneutral OCCT convention.
// The original child datum is retained; its parent target is set to
// from*desired. Shared datums are copied before modification.
func WriteOCCTRationalSphericalHierarchyEdits(ctx context.Context, w io.Writer, source *File, edits []RationalSphericalHierarchyEdit, l Limits, tol float64) error {
	return writeRationalSphericalHierarchyEdits(ctx, w, source, edits, l, tol, true)
}

func rationalSphericalEditBasis(m [3][4]*big.Rat) (mappedRationalBasis, error) {
	var b mappedRationalBasis
	for i, row := range m {
		for _, q := range row {
			if q == nil || q.Num().BitLen() > 256 || q.Denom().BitLen() > 256 {
				return b, geom.ErrInvalid
			}
		}
		b.X[i], b.Y[i], b.Z[i], b.Origin[i] = new(big.Rat).Set(row[0]), new(big.Rat).Set(row[1]), new(big.Rat).Set(row[2]), new(big.Rat).Set(row[3])
	}
	if err := rationalHierarchyFrameBound(b); err != nil {
		return b, err
	}
	for _, axis := range [][3]*big.Rat{b.X, b.Y, b.Z} {
		if mappedRationalDot(axis, axis).Cmp(big.NewRat(1, 1)) != 0 {
			return b, geom.ErrUnrepresentable
		}
	}
	y := mappedRationalCross(b.Z, b.X)
	for i := range y {
		if y[i].Cmp(b.Y[i]) != 0 {
			return b, geom.ErrUnrepresentable
		}
	}
	return b, nil
}

func sameSphericalEditBasis(a, b mappedRationalBasis) bool {
	for _, pair := range [4][2][3]*big.Rat{{a.X, b.X}, {a.Y, b.Y}, {a.Z, b.Z}, {a.Origin, b.Origin}} {
		for j, q := range pair[0] {
			if q.Cmp(pair[1][j]) != 0 {
				return false
			}
		}
	}
	return true
}

func writeRationalSphericalHierarchyEdits(ctx context.Context, w io.Writer, source *File, edits []RationalSphericalHierarchyEdit, l Limits, tol float64, vendor bool) error {
	if ctx == nil || w == nil || len(edits) > 64 || !independentNativeSourceBound(source, 4096) {
		return geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	raw, err := mappedPlanarSourceBytes(source)
	if err != nil {
		return err
	}
	base, err := readRationalSphericalHierarchy(ctx, bytes.NewReader(raw), l, tol, vendor)
	if err != nil {
		return err
	}
	if len(edits) > 0 {
		known, err := applyRationalHierarchyEdits(ctx, base.Source, base, edits, vendor)
		if err != nil {
			return err
		}
		raw, err = mappedPlanarSourceBytes(base.Source)
		if err != nil {
			return err
		}
		fresh, err := readRationalSphericalHierarchy(ctx, bytes.NewReader(raw), l, tol, vendor)
		if err != nil {
			return err
		}
		if !matchRationalHierarchyEdits(base, fresh, known) {
			return geom.ErrUnrepresentable
		}
	}
	return writeRationalHierarchyEditBytes(ctx, w, raw)
}

func applyRationalHierarchyEdits(ctx context.Context, f *File, base RationalSphericalHierarchy, edits []RationalSphericalHierarchyEdit, vendor bool) (map[int64]ExactNAUOOccurrence, error) {
	known := make(map[int64]ExactNAUOOccurrence, len(base.Relationships))
	parent := make(map[int64]int64, len(base.Definitions))
	for _, r := range base.Relationships {
		known[r.NAUOID] = r
	}
	for _, p := range base.Definitions {
		parent[p.DefinitionID] = p.Representations[0]
	}
	selected := make(map[int64]bool, len(edits))
	for _, edit := range edits {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		r, ok := known[edit.NAUOID]
		if !ok {
			return nil, geom.ErrUnrepresentable
		}
		if selected[edit.NAUOID] {
			return nil, geom.ErrInvalid
		}
		selected[edit.NAUOID] = true
		desired, err := rationalSphericalEditBasis(edit.Matrix)
		if err != nil {
			return nil, err
		}
		if !sameSphericalEditBasis(r.pose, desired) {
			target := desired
			if vendor {
				from, err := mappedRationalFrame(f, r.FromPlacementID)
				if err != nil {
					return nil, err
				}
				target = composeMappedPose(from, desired)
			}
			if err := rationalHierarchyFrameBound(target); err != nil {
				return nil, err
			}
			if err := setSphericalHierarchyTarget(f, r.TransformationID, parent[r.ParentID], target); err != nil {
				return nil, err
			}
			r.ToPlacementID = f.Entities[r.TransformationID].Records[0].Args[3].Ref
			r.pose = desired
		}
		if edit.Name != nil {
			r.Name = *edit.Name
			f.Entities[r.NAUOID].Records[0].Args[1] = Value{Kind: String, Text: r.Name}
		}
		known[r.NAUOID] = r
	}
	return known, nil
}

func matchRationalHierarchyEdits(base, fresh RationalSphericalHierarchy, known map[int64]ExactNAUOOccurrence) bool {
	if len(fresh.Relationships) != len(known) || !reflect.DeepEqual(fresh.Definitions, base.Definitions) || !reflect.DeepEqual(fresh.Roots, base.Roots) {
		return false
	}
	for _, r := range fresh.Relationships {
		expect, ok := known[r.NAUOID]
		if !ok || !sameSphericalEditBasis(expect.pose, r.pose) {
			return false
		}
		expect.pose, r.pose = mappedRationalBasis{}, mappedRationalBasis{}
		if expect != r {
			return false
		}
	}
	return true
}

func writeRationalHierarchyEditBytes(ctx context.Context, w io.Writer, raw []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	n, err := w.Write(raw)
	if err == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return err
}
