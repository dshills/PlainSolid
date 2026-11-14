package step

import (
	"bytes"
	"io"
	"math"
	"math/big"
	"plainsolid/geom"
	"unicode/utf8"
)

// WriteEditedMappedPlanarMaterials authors new placements and occurrence names
// over the unchanged, independently revalidated local products in source. Edits
// must identify every original item and target exactly once. New target records
// are private to each occurrence; original shared datum records are never edited.
// Geometry, units, map identity and occurrence membership cannot change here.
// Output is staged and fully re-read before publication. This is not source-only
// write-back and does not create a NAUO product assembly.
func WriteEditedMappedPlanarMaterials(w io.Writer, source *File, name string, edits []MappedPlanarOccurrence, l Limits) error {
	if w == nil || source == nil || !utf8.ValidString(name) || len(name) > 1024 || len(edits) > 16 {
		return mappedPlanarFailure()
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return e
	}
	original, e := ReadMappedPlanarMaterials(bytes.NewReader(raw), l)
	if e != nil {
		return e
	}
	f := original.Source // parser-owned copy; no caller data is mutated
	known := map[int64]MappedPlanarOccurrence{}
	for _, m := range original.Materials {
		for _, u := range m.Occurrences {
			known[u.ItemID] = u
		}
	}
	if len(edits) != len(known) {
		return mappedPlanarFailure()
	}
	var next, rootID int64
	for _, id := range f.Order {
		if id > next {
			next = id
		}
		if len(f.Entities[id].Records) == 1 && f.Entities[id].Records[0].Type == "SHAPE_REPRESENTATION" {
			rootID = id
		}
	}
	if next > math.MaxInt64-80 {
		return mappedPlanarFailure()
	}
	emit := func(kind string, args ...Value) int64 {
		next++
		f.Order = append(f.Order, next)
		f.Entities[next] = Entity{ID: next, Records: []Record{{Type: kind, Args: args}}}
		return next
	}
	str := func(s string) Value { return Value{Kind: String, Text: s} }
	refv := func(id int64) Value { return Value{Kind: Reference, Ref: id} }
	coordinate := func(kind string, v geom.Vec3) int64 {
		values := []Value{}
		for _, x := range []float64{v.X, v.Y, v.Z} {
			q := new(big.Rat).SetFloat64(x)
			values = append(values, Value{Kind: Number, Text: q.FloatString(q.Denom().BitLen() - 1)})
		}
		return emit(kind, str(""), Value{Kind: Aggregate, Items: values})
	}
	seen := map[int64]bool{}
	for _, edit := range edits {
		old, ok := known[edit.ItemID]
		if !ok || seen[edit.ItemID] || old.TargetID != edit.TargetID || !utf8.ValidString(edit.Name) || len(edit.Name) > 1024 || !edit.Transform.IsFinite() {
			return mappedPlanarFailure()
		}
		seen[edit.ItemID] = true
		t := edit.Transform
		x, y, z := geom.Vec3{X: t[0][0], Y: t[1][0], Z: t[2][0]}, geom.Vec3{X: t[0][1], Y: t[1][1], Z: t[2][1]}, geom.Vec3{X: t[0][2], Y: t[1][2], Z: t[2][2]}
		if !cylinderSupportUnit(x) || !cylinderSupportUnit(y) || !cylinderSupportUnit(z) || x.Cross(y) != z {
			return mappedPlanarFailure()
		}
		xi, yi, zi := coordinate("DIRECTION", x), coordinate("DIRECTION", y), coordinate("DIRECTION", z)
		p := coordinate("CARTESIAN_POINT", geom.Vec3{X: t[0][3], Y: t[1][3], Z: t[2][3]})
		target := emit("CARTESIAN_TRANSFORMATION_OPERATOR_3D", str(""), str(""), str(""), refv(xi), refv(yi), refv(p), Value{Kind: Number, Text: "1."}, refv(zi))
		en := f.Entities[edit.ItemID]
		en.Records[0].Args[0] = str(edit.Name)
		en.Records[0].Args[2] = refv(target)
		f.Entities[edit.ItemID] = en
	}
	root := f.Entities[rootID]
	root.Records[0].Args[0] = str(name)
	f.Entities[rootID] = root
	items, e := listRefs(root.Records[0].Args[1], 1)
	if e != nil {
		return e
	}
	limits, e := l.defaults()
	if e != nil {
		return e
	}
	// Remove old now-unreferenced operator datums without changing any product
	// geometry. A datum still used by a local product remains in its closure.
	f, e = mappedPlanarPartition(f, rootID, items, limits)
	if e != nil {
		return e
	}
	raw, e = mappedPlanarSourceBytes(f)
	if e != nil {
		return e
	}
	if _, e = ReadMappedPlanarMaterials(bytes.NewReader(raw), l); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
