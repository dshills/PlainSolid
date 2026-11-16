package step

import (
	"bytes"
	"context"
	"io"
	"math"
	"math/big"
	"strings"

	"plainsolid/geom"
)

// SphericalHierarchyOccurrenceEdit sets the name and exact signed-coordinate
// child-to-parent transform of one original relationship across all its uses.
type SphericalHierarchyOccurrenceEdit struct {
	NAUOID    int64
	Name      string
	Transform geom.Affine
}

// WriteIndependentSphericalHierarchyEdits re-proves a retained independent
// assembly source, applies selected occurrence edits, and re-proves the result
// before writing. Original product, body, face and relationship IDs are retained.
// An exclusively owned target frame is updated in place; a shared frame is
// left untouched and the selected relationship receives a private target. A
// placement edit must agree under both source frame composition conventions.
func WriteIndependentSphericalHierarchyEdits(ctx context.Context, w io.Writer, source *File, edits []SphericalHierarchyOccurrenceEdit, l Limits, tol float64) error {
	if ctx == nil || w == nil || len(edits) > 64 || !independentNativeSourceBound(source, 4096) {
		return geom.ErrInvalid
	}
	raw, err := mappedPlanarSourceBytes(source)
	if err != nil {
		return err
	}
	base, err := ReadIndependentSphericalHierarchy(ctx, bytes.NewReader(raw), l, tol)
	if err != nil {
		return err
	}
	f := base.Assembly.Source
	wanted := make(map[int64]AssemblyOccurrence, len(base.Assembly.Occurrences))
	for _, u := range base.Assembly.Occurrences {
		wanted[u.ID] = u
	}
	selected := make(map[int64]SphericalHierarchyOccurrenceEdit, len(edits))
	for _, edit := range edits {
		if _, ok := wanted[edit.NAUOID]; !ok || !planarSplineRigid(edit.Transform) {
			return geom.ErrUnrepresentable
		}
		if _, duplicate := selected[edit.NAUOID]; duplicate {
			return geom.ErrInvalid
		}
		selected[edit.NAUOID] = edit
	}
	if len(edits) != 0 {
		for _, id := range f.Order {
			if err := ctx.Err(); err != nil {
				return err
			}
			en := f.Entities[id]
			if len(en.Records) != 1 || en.Records[0].Type != "CONTEXT_DEPENDENT_SHAPE_REPRESENTATION" {
				continue
			}
			cd := en.Records[0]
			pds := f.Entities[cd.Args[1].Ref].Records[0]
			edit, ok := selected[pds.Args[2].Ref]
			if !ok {
				continue
			}
			old := wanted[edit.NAUOID]
			if edit.Transform != old.Transform {
				var trID, parentRep int64
				for _, r := range f.Entities[cd.Args[0].Ref].Records {
					switch r.Type {
					case "REPRESENTATION_RELATIONSHIP":
						parentRep = r.Args[3].Ref
					case "REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION":
						trID = r.Args[0].Ref
					}
				}
				if err := editSphericalHierarchyFrame(f, trID, parentRep, edit.Transform); err != nil {
					return err
				}
			}
			if edit.Name != old.Name {
				rec := f.Entities[edit.NAUOID].Records[0]
				rec.Args[1] = Value{Kind: String, Text: edit.Name}
				old.Name = edit.Name
			}
			old.Transform = edit.Transform
			wanted[old.ID] = old
			delete(selected, edit.NAUOID)
		}
		if len(selected) != 0 {
			return geom.ErrUnrepresentable
		}
		raw, err = mappedPlanarSourceBytes(f)
		if err != nil {
			return err
		}
		fresh, err := ReadIndependentSphericalHierarchy(ctx, bytes.NewReader(raw), l, tol)
		if err != nil {
			return err
		}
		if len(fresh.Assembly.Occurrences) != len(wanted) {
			return geom.ErrUnrepresentable
		}
		for _, got := range fresh.Assembly.Occurrences {
			if got != wanted[got.ID] {
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

// The frame and representation ownership is known from the strict source
// proof; count the remaining incoming references before touching any datum.
func editSphericalHierarchyFrame(f *File, trID, parentRep int64, pose geom.Affine) error {
	tr := f.Entities[trID].Records[0]
	from, err := mappedRationalFrame(f, tr.Args[2].Ref)
	if err != nil {
		return err
	}
	var desired mappedRationalBasis
	for i := range 3 {
		desired.X[i], desired.Y[i], desired.Z[i], desired.Origin[i] = new(big.Rat).SetFloat64(pose[i][0]), new(big.Rat).SetFloat64(pose[i][1]), new(big.Rat).SetFloat64(pose[i][2]), new(big.Rat).SetFloat64(pose[i][3])
	}
	target := composeMappedPose(desired, from)
	// A rounded target would be rejected on re-import. Check it before emitting.
	if _, err := independentTorusAssemblyPose(target); err != nil {
		return err
	}
	// Verify compatible placement in readers that compose inverse(from)*to,
	// including OCCT. The two orders always agree for neutral child frames.
	if alternate, err := independentTorusAssemblyPose(composeMappedPose(inverseMappedPose(from), target)); err != nil || alternate != pose {
		return geom.ErrUnrepresentable
	}
	return setAssemblyOccurrenceTarget(f, trID, parentRep, target)
}

func setAssemblyOccurrenceTarget(f *File, trID, parentRep int64, target mappedRationalBasis) error {
	tr := f.Entities[trID].Records[0]
	toID := tr.Args[3].Ref
	frame := f.Entities[toID].Records[0]
	children := [3]int64{frame.Args[1].Ref, frame.Args[2].Ref, frame.Args[3].Ref}
	var incoming [4]int
	owners := map[int64]int{}
	var maxID int64
	for id, en := range f.Entities {
		maxID = max(maxID, id)
		var visit func(Value)
		visit = func(v Value) {
			if v.Kind == Reference {
				for i, q := range [4]int64{toID, children[0], children[1], children[2]} {
					if v.Ref == q {
						incoming[i]++
						if i == 0 {
							owners[id]++
						}
					}
				}
			}
			for _, item := range v.Items {
				visit(item)
			}
		}
		for _, rec := range en.Records {
			for _, v := range rec.Args {
				visit(v)
			}
		}
	}
	private := incoming == [4]int{2, 1, 1, 1} && owners[trID] == 1 && owners[parentRep] == 1
	if !private && maxID > math.MaxInt64-4 {
		return geom.ErrUnrepresentable
	}
	var newItems [3]Value
	for i, q := range [3][3]*big.Rat{target.Origin, target.Z, target.X} {
		items := make([]Value, 3)
		for j, component := range q {
			token, err := mappedTerminatingDecimal(component)
			if err != nil {
				return err
			}
			if !strings.Contains(token, ".") {
				token += "."
			}
			items[j] = Value{Kind: Number, Text: token}
		}
		en := f.Entities[children[i]]
		old := en.Records[0]
		if private {
			old.Args[1] = Value{Kind: Aggregate, Items: items}
		} else {
			maxID++
			en.ID = maxID
			en.Records = []Record{{Type: old.Type, Args: []Value{old.Args[0], {Kind: Aggregate, Items: items}}}}
			f.Entities[maxID] = en
			f.Order = append(f.Order, maxID)
			newItems[i] = Value{Kind: Reference, Ref: maxID}
		}
	}
	if !private {
		maxID++
		en := f.Entities[toID]
		en.ID = maxID
		en.Records = []Record{{Type: frame.Type, Args: []Value{frame.Args[0], newItems[0], newItems[1], newItems[2]}}}
		f.Entities[maxID] = en
		f.Order = append(f.Order, maxID)
		targetRef := Value{Kind: Reference, Ref: maxID}
		rep := f.Entities[parentRep].Records[0]
		rep.Args[1].Items = append(rep.Args[1].Items, targetRef)
		tr.Args[3] = targetRef
	}
	return nil
}
