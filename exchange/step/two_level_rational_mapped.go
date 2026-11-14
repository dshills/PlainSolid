package step

import (
	"bytes"
	"io"
	"plainsolid/geom"
)

// TwoLevelExactMappedOccurrence preserves three original mapping steps and their
// exact composed pose. A reused leaf item ID alone is not occurrence identity.
type TwoLevelExactMappedOccurrence struct {
	Occurrence ExactMappedOccurrence
	Path       [3]MappedSourceStep
}
type TwoLevelRationalMappedPlanarMaterials struct {
	Source      *File
	Name        string
	LengthUnit  float64
	Bodies      map[int64]Body
	MapOrder    []int64
	Occurrences []TwoLevelExactMappedOccurrence
}

// ReadTwoLevelRationalMappedPlanarMaterials accepts exactly two shared group
// levels above the existing local material maps. Each level has one shared group
// definition, at most16 uses, identity map origin and the same source context.
// Expansion is bounded to128 occurrences. Existing shallower APIs stay strict.
func ReadTwoLevelRationalMappedPlanarMaterials(r io.Reader, l Limits) (TwoLevelRationalMappedPlanarMaterials, error) {
	fail := func() (TwoLevelRationalMappedPlanarMaterials, error) {
		return TwoLevelRationalMappedPlanarMaterials{}, mappedPlanarFailure()
	}
	lim, e := l.defaults()
	if e != nil {
		return fail()
	}
	f, e := Parse(r, l)
	if e != nil {
		return TwoLevelRationalMappedPlanarMaterials{}, e
	}
	var sdrID int64
	sdrCount, shapeCount := 0, 0
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) != 1 {
			continue
		}
		switch en.Records[0].Type {
		case "SHAPE_DEFINITION_REPRESENTATION":
			sdrID = id
			sdrCount++
		case "SHAPE_REPRESENTATION":
			shapeCount++
		}
	}
	if sdrCount != 1 || shapeCount != 3 {
		return fail()
	}
	sdr, e := simple(f, sdrID, "SHAPE_DEFINITION_REPRESENTATION", 2)
	if e != nil {
		return fail()
	}
	rootID := sdr.Args[1].Ref
	root, e := simple(f, rootID, "SHAPE_REPRESENTATION", 3)
	if e != nil {
		return fail()
	}
	ids, e := listRefs(root.Args[1], 1)
	if e != nil || len(ids) > 16 {
		return fail()
	}
	name, e := textValue(root.Args[0])
	if e != nil {
		return fail()
	}
	var mid, gid int64
	outer := []ExactMappedOccurrence{}
	seen := map[int64]bool{}
	for _, id := range ids {
		if seen[id] {
			return fail()
		}
		seen[id] = true
		item, e := simple(f, id, "MAPPED_ITEM", 3)
		if e != nil {
			return fail()
		}
		if mid != 0 && mid != item.Args[1].Ref {
			return fail()
		}
		mid = item.Args[1].Ref
		mapping, e := simple(f, mid, "REPRESENTATION_MAP", 2)
		if e != nil {
			return fail()
		}
		origin, e := mappedExactFrame(f, mapping.Args[0].Ref)
		if e != nil || frameTransform(origin) != geom.Identity() {
			return fail()
		}
		gid = mapping.Args[1].Ref
		if gid == rootID {
			return fail()
		}
		pose, e := mappedRationalOperator(f, item.Args[2].Ref)
		if e != nil {
			return fail()
		}
		n, e := textValue(item.Args[0])
		if e != nil {
			return fail()
		}
		outer = append(outer, ExactMappedOccurrence{Name: n, ItemID: id, TargetID: item.Args[2].Ref, MapID: mid, pose: pose})
	}
	group, e := simple(f, gid, "SHAPE_REPRESENTATION", 3)
	if e != nil || group.Args[2].Ref != root.Args[2].Ref {
		return fail()
	}
	if _, e = textValue(group.Args[0]); e != nil {
		return fail()
	}
	children, e := listRefs(group.Args[1], 1)
	if e != nil {
		return fail()
	}
	whole, e := mappedPlanarPartition(f, rootID, ids, lim)
	if e != nil || len(whole.Entities) != len(f.Entities) {
		return fail()
	}
	// Redirect only a private product-root view to the middle representation. All
	// removed outer records have already been inspected; source remains untouched.
	raw, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return fail()
	}
	view, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return fail()
	}
	sd := view.Entities[sdrID]
	sd.Records[0].Args[1] = Value{Kind: Reference, Ref: gid}
	view.Entities[sdrID] = sd
	view, e = mappedPlanarPartition(view, gid, children, lim)
	if e != nil {
		return fail()
	}
	raw, e = mappedPlanarSourceBytes(view)
	if e != nil {
		return fail()
	}
	inner, e := ReadNestedRationalMappedPlanarMaterials(bytes.NewReader(raw), l)
	if e != nil || len(inner.Occurrences)*len(outer) > 128 {
		return fail()
	}
	// Every removed non-metadata source record must belong to the explicitly
	// inspected outer layer, not a second hidden branch. Reachability and the
	// one-shared-map rule above restrict the removed closure to those records.
	out := TwoLevelRationalMappedPlanarMaterials{Source: f, Name: name, LengthUnit: inner.LengthUnit, Bodies: inner.Bodies, MapOrder: inner.MapOrder}
	for _, a := range outer {
		for _, b := range inner.Occurrences {
			leaf := b.Occurrence
			leaf.pose = composeMappedPose(a.pose, leaf.pose)
			out.Occurrences = append(out.Occurrences, TwoLevelExactMappedOccurrence{Occurrence: leaf, Path: [3]MappedSourceStep{{Name: a.Name, ItemID: a.ItemID, TargetID: a.TargetID, MapID: a.MapID, RepresentationID: gid}, b.Path[0], b.Path[1]}})
		}
	}
	return out, nil
}
func WriteTwoLevelRationalMappedPlanarSource(w io.Writer, source *File, l Limits) error {
	if w == nil || source == nil {
		return mappedPlanarFailure()
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return e
	}
	if _, e = ReadTwoLevelRationalMappedPlanarMaterials(bytes.NewReader(raw), l); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
