package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"unicode/utf8"

	"plainsolid/geom"
)

// MixedConicalAP242OccurrenceEdit updates one original STEP relationship.
// A nil field retains the original value. Translation specifies the exact new
// child-to-parent origin in file units; the original rotation is preserved.
type MixedConicalAP242OccurrenceEdit struct {
	NAUOID      int64
	Name        *string
	Translation *[3]*big.Rat
}

// NestedMixedConicalAP242OccurrenceEdit is the same relationship edit on the
// original two-level nested source family.
type NestedMixedConicalAP242OccurrenceEdit = MixedConicalAP242OccurrenceEdit

// WriteNestedMixedConicalAP242Edits rewrites names and translations in the
// proved source hierarchy. Every edit applies to all expanded uses of its
// original relationship. The writer keeps local geometry and proves the full
// edited graph and exact local poses before publishing any bytes.
func WriteNestedMixedConicalAP242Edits(ctx context.Context, w io.Writer, original io.Reader, edits []NestedMixedConicalAP242OccurrenceEdit, l Limits, options geom.SurfacePatchOptions) error {
	if ctx == nil || w == nil || original == nil || len(edits) > 16 {
		return geom.ErrInvalid
	}
	read := func(r io.Reader) (mixedConicalEditInput, error) {
		m, err := ReadNestedMixedConicalAP242Assembly(ctx, r, l, options)
		return mixedConicalEditInput{m.Assembly, m.Occurrences, len(m.Instances)}, err
	}
	return writeMixedConicalAP242Edits(ctx, w, original, edits, read)
}

// WriteMixedConicalAP242DAGEdits transfers names and exact translations to
// proved original STEP relationships in a bounded product DAG. Edits apply
// consistently to all expanded uses of each relationship. The full result
// is re-proved with the same limits before any bytes are written.
func WriteMixedConicalAP242DAGEdits(ctx context.Context, w io.Writer, original io.Reader, edits []MixedConicalAP242OccurrenceEdit, l MixedConicalAP242DAGLimits, options geom.SurfacePatchOptions) error {
	if ctx == nil || w == nil || original == nil || len(edits) > 64 {
		return geom.ErrInvalid
	}
	read := func(r io.Reader) (mixedConicalEditInput, error) {
		m, err := ReadMixedConicalAP242DAG(ctx, r, l, options)
		return mixedConicalEditInput{m.Assembly, m.Occurrences, len(m.Instances)}, err
	}
	return writeMixedConicalAP242Edits(ctx, w, original, edits, read)
}

type mixedConicalEditInput struct {
	a           Assembly
	occurrences []ExactNAUOOccurrence
	instances   int
}

func writeMixedConicalAP242Edits(ctx context.Context, w io.Writer, original io.Reader, edits []MixedConicalAP242OccurrenceEdit, read func(io.Reader) (mixedConicalEditInput, error)) error {
	base, err := read(original)
	if err != nil {
		return err
	}
	f := base.a.Source
	known := make(map[int64]ExactNAUOOccurrence, len(base.occurrences))
	for _, o := range base.occurrences {
		known[o.NAUOID] = o
	}
	products := make(map[int64]int64, len(base.a.Products))
	for _, p := range base.a.Products {
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
		fresh, err := read(bytes.NewReader(raw))
		if err != nil {
			return err
		}
		if len(fresh.occurrences) != len(known) || fresh.instances != base.instances || fresh.a.LengthUnit != base.a.LengthUnit {
			return geom.ErrUnrepresentable
		}
		for _, got := range fresh.occurrences {
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
