package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"unicode/utf8"

	"plainsolid/geom"
)

// NestedMixedConicalAP242OccurrenceEdit updates one original STEP relationship.
// A nil field retains the original value. Translation specifies the exact new
// child-to-parent origin in file units; the original rotation is preserved.
type NestedMixedConicalAP242OccurrenceEdit struct {
	NAUOID      int64
	Name        *string
	Translation *[3]*big.Rat
}

// WriteNestedMixedConicalAP242Edits rewrites names and translations in the
// proved source hierarchy. Every edit applies to all expanded uses of its
// original relationship. The writer keeps local geometry and proves the full
// edited graph and exact local poses before publishing any bytes.
func WriteNestedMixedConicalAP242Edits(ctx context.Context, w io.Writer, original io.Reader, edits []NestedMixedConicalAP242OccurrenceEdit, l Limits, options geom.SurfacePatchOptions) error {
	if ctx == nil || w == nil || original == nil || len(edits) > 16 {
		return geom.ErrInvalid
	}
	base, err := ReadNestedMixedConicalAP242Assembly(ctx, original, l, options)
	if err != nil {
		return err
	}
	f := base.Assembly.Source
	known := make(map[int64]ExactNAUOOccurrence, len(base.Occurrences))
	for _, o := range base.Occurrences {
		known[o.NAUOID] = o
	}
	products := make(map[int64]int64, len(base.Assembly.Products))
	for _, p := range base.Assembly.Products {
		// The strict reader has already proved one representation per product.
		products[p.DefinitionID] = p.Representations[0]
	}
	seen := make(map[int64]bool, len(edits))
	for _, e := range edits {
		if err := ctx.Err(); err != nil {
			return err
		}
		o, ok := known[e.NAUOID]
		if !ok || seen[e.NAUOID] || e.Name == nil && e.Translation == nil {
			return geom.ErrInvalid
		}
		seen[e.NAUOID] = true
		if e.Name != nil {
			name := *e.Name
			if len(name) > 4096 || !utf8.ValidString(name) {
				return geom.ErrInvalid
			}
			o.Name = name
			f.Entities[o.NAUOID].Records[0].Args[1] = Value{Kind: String, Text: name}
		}
		if e.Translation != nil {
			changed := false
			for i, q := range *e.Translation {
				if q == nil || q.Num().BitLen() > 256 || q.Denom().BitLen() > 256 {
					return geom.ErrUnrepresentable
				}
				changed = changed || q.Cmp(o.pose.Origin[i]) != 0
				o.pose.Origin[i] = new(big.Rat).Set(q)
			}
			if changed {
				// The source's child frame is the exact identity. Its proved
				// target frame therefore is the requested child-to-parent pose.
				if err := setAssemblyOccurrenceTarget(f, o.TransformationID, products[o.ParentID], o.pose); err != nil {
					return err
				}
				o.ToPlacementID = f.Entities[o.TransformationID].Records[0].Args[3].Ref
			}
		}
		known[o.NAUOID] = o
	}
	raw, err := mappedPlanarSourceBytes(f)
	if err != nil {
		return err
	}
	if len(edits) != 0 {
		fresh, err := ReadNestedMixedConicalAP242Assembly(ctx, bytes.NewReader(raw), l, options)
		if err != nil {
			return err
		}
		if len(fresh.Occurrences) != len(known) || len(fresh.Instances) != len(base.Instances) || fresh.Assembly.LengthUnit != base.Assembly.LengthUnit {
			return geom.ErrUnrepresentable
		}
		for _, got := range fresh.Occurrences {
			want, ok := known[got.NAUOID]
			if !ok || !equalMixedConicalOccurrences(got, want) {
				return geom.ErrUnrepresentable
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	n, err := w.Write(raw)
	if err == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return err
}

func equalMixedConicalOccurrences(a, b ExactNAUOOccurrence) bool {
	ma, mb := a.Matrix(), b.Matrix()
	a.pose, b.pose = mappedRationalBasis{}, mappedRationalBasis{}
	if a != b {
		return false
	}
	for i := range ma {
		for j := range ma[i] {
			if ma[i][j].Cmp(mb[i][j]) != 0 {
				return false
			}
		}
	}
	return true
}
