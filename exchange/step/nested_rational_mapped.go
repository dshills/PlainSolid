package step

import (
	"bytes"
	"io"
	"math/big"
	"plainsolid/geom"
)

// MappedSourceStep records one original mapping edge, outermost first.
type MappedSourceStep struct {
	Name                                      string
	ItemID, TargetID, MapID, RepresentationID int64
}

// NestedExactMappedOccurrence preserves the two source uses and their composed
// immutable exact pose. Repeated group uses share original local body maps.
type NestedExactMappedOccurrence struct {
	Occurrence ExactMappedOccurrence
	Path       [2]MappedSourceStep
}
type NestedRationalMappedPlanarMaterials struct {
	Source      *File
	Name        string
	LengthUnit  float64
	Bodies      map[int64]Body
	MapOrder    []int64
	Occurrences []NestedExactMappedOccurrence
}

func composeMappedPose(outer, inner mappedRationalBasis) (b mappedRationalBasis) {
	apply := func(v [3]*big.Rat, point bool) (q [3]*big.Rat) {
		for i := range q {
			q[i] = new(big.Rat)
			if point {
				q[i].Set(outer.Origin[i])
			}
			for j, a := range [][3]*big.Rat{outer.X, outer.Y, outer.Z} {
				q[i].Add(q[i], new(big.Rat).Mul(a[i], v[j]))
			}
		}
		return
	}
	b.Origin = apply(inner.Origin, true)
	b.X = apply(inner.X, false)
	b.Y = apply(inner.Y, false)
	b.Z = apply(inner.Z, false)
	return
}

// ReadNestedRationalMappedPlanarMaterials accepts exactly one intermediate shared
// SHAPE_REPRESENTATION, with identity mapping origins and a common source unit
// context. At most sixteen outer/inner uses each and64 expanded uses are allowed.
// It retains source ancestry; no generic recursive assembly inference is made.
func ReadNestedRationalMappedPlanarMaterials(r io.Reader, l Limits) (NestedRationalMappedPlanarMaterials, error) {
	fail := func() (NestedRationalMappedPlanarMaterials, error) {
		return NestedRationalMappedPlanarMaterials{}, mappedPlanarFailure()
	}
	lim, e := l.defaults()
	if e != nil {
		return fail()
	}
	f, e := Parse(r, l)
	if e != nil {
		return NestedRationalMappedPlanarMaterials{}, e
	}
	sdrs, shapes := []int64{}, []int64{}
	mappedCount := 0
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) != 1 {
			continue
		}
		switch en.Records[0].Type {
		case "SHAPE_DEFINITION_REPRESENTATION":
			sdrs = append(sdrs, id)
		case "SHAPE_REPRESENTATION":
			shapes = append(shapes, id)
		case "MAPPED_ITEM":
			mappedCount++
		}
	}
	if len(sdrs) != 1 || len(shapes) != 2 {
		return fail()
	}
	sdr, e := simple(f, sdrs[0], "SHAPE_DEFINITION_REPRESENTATION", 2)
	if e != nil {
		return fail()
	}
	rootID := sdr.Args[1].Ref
	root, e := simple(f, rootID, "SHAPE_REPRESENTATION", 3)
	if e != nil {
		return fail()
	}
	outerIDs, e := listRefs(root.Args[1], 1)
	if e != nil || len(outerIDs) > 16 {
		return fail()
	}
	name, e := textValue(root.Args[0])
	if e != nil {
		return fail()
	}
	var groupMap, groupID int64
	outer := []ExactMappedOccurrence{}
	seen := map[int64]bool{}
	for _, id := range outerIDs {
		if seen[id] {
			return fail()
		}
		seen[id] = true
		item, e := simple(f, id, "MAPPED_ITEM", 3)
		if e != nil {
			return fail()
		}
		mid := item.Args[1].Ref
		if groupMap != 0 && mid != groupMap {
			return fail()
		}
		groupMap = mid
		mp, e := simple(f, mid, "REPRESENTATION_MAP", 2)
		if e != nil {
			return fail()
		}
		origin, e := mappedExactFrame(f, mp.Args[0].Ref)
		if e != nil || frameTransform(origin) != geom.Identity() {
			return fail()
		}
		groupID = mp.Args[1].Ref
		if groupID == rootID {
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
	group, e := simple(f, groupID, "SHAPE_REPRESENTATION", 3)
	if e != nil || group.Args[2].Ref != root.Args[2].Ref {
		return fail()
	}
	if _, e = textValue(group.Args[0]); e != nil {
		return fail()
	}
	innerIDs, e := listRefs(group.Args[1], 1)
	if e != nil || len(innerIDs) > 16 || len(innerIDs)*len(outer) > 64 || mappedCount != len(innerIDs)+len(outer) {
		return fail()
	}
	for _, id := range innerIDs {
		if seen[id] {
			return fail()
		}
		seen[id] = true
	}
	// Account for every original record before removing validated group wrappers
	// from the private flat proof view. No orphan or cyclic dependency is ignored.
	all, e := mappedPlanarPartition(f, rootID, outerIDs, lim)
	if e != nil || len(all.Entities) != len(f.Entities) {
		return fail()
	}
	flat, e := mappedPlanarPartition(f, rootID, innerIDs, lim)
	if e != nil {
		return fail()
	}
	raw, e := mappedPlanarSourceBytes(flat)
	if e != nil {
		return fail()
	}
	local, e := ReadRationalMappedPlanarMaterials(bytes.NewReader(raw), l)
	if e != nil {
		return fail()
	}
	if len(local.Occurrences) != len(innerIDs) {
		return fail()
	}
	out := NestedRationalMappedPlanarMaterials{Source: f, Name: name, LengthUnit: local.LengthUnit, Bodies: local.Bodies, MapOrder: local.MapOrder}
	for _, a := range outer {
		for _, b := range local.Occurrences {
			pose := composeMappedPose(a.pose, b.pose)
			leaf := b
			leaf.pose = pose
			out.Occurrences = append(out.Occurrences, NestedExactMappedOccurrence{Occurrence: leaf, Path: [2]MappedSourceStep{{Name: a.Name, ItemID: a.ItemID, TargetID: a.TargetID, MapID: a.MapID, RepresentationID: groupID}, {Name: b.Name, ItemID: b.ItemID, TargetID: b.TargetID, MapID: b.MapID, RepresentationID: f.Entities[b.MapID].Records[0].Args[1].Ref}}})
		}
	}
	return out, nil
}
func WriteNestedRationalMappedPlanarSource(w io.Writer, source *File, l Limits) error {
	if w == nil || source == nil {
		return mappedPlanarFailure()
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return e
	}
	if _, e = ReadNestedRationalMappedPlanarMaterials(bytes.NewReader(raw), l); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
