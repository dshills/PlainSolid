package step

import (
	"bytes"
	"plainsolid/geom"
	"reflect"
)

// UnusedOccurrenceContext retains an original, unstyled duplicate occurrence
// context emitted by OCCT beside the styled one. Both have identical records
// and the same original NAUO PDS; no geometry/style references the duplicate.
type UnusedOccurrenceContext struct{ DefinitionRepresentationID, RepresentationID, ProductShapeID, EquivalentRepresentationID int64 }

func combinedOccurrenceProofView(f *File, l Limits) ([]byte, []UnusedOccurrenceContext, error) {
	raw, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return nil, nil, e
	}
	view, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return nil, nil, e
	}
	incoming := map[int64][]int64{}
	var walk func(int64, Value)
	walk = func(id int64, v Value) {
		if v.Kind == Reference {
			incoming[v.Ref] = append(incoming[v.Ref], id)
		}
		for _, q := range v.Items {
			walk(id, q)
		}
	}
	for id, en := range f.Entities {
		for _, r := range en.Records {
			for _, v := range r.Args {
				walk(id, v)
			}
		}
	}
	type pair struct {
		sdr, pds, rep int64
		record        Record
	}
	var unused, styled []pair
	for _, id := range f.Order {
		s, e := simple(f, id, "SHAPE_DEFINITION_REPRESENTATION", 2)
		if e != nil {
			continue
		}
		refs, e := namedReferences(s, 0, 1)
		if e != nil {
			continue
		}
		p, e := simple(f, refs[0], "PRODUCT_DEFINITION_SHAPE", 3)
		if e != nil || p.Args[2].Kind != Reference {
			continue
		}
		if _, e = simple(f, p.Args[2].Ref, "NEXT_ASSEMBLY_USAGE_OCCURRENCE", 6); e != nil {
			continue
		}
		rep, e := simple(f, refs[1], "SHAPE_REPRESENTATION", 3)
		if e != nil {
			continue
		}
		owners := incoming[refs[1]]
		if len(owners) == 1 && owners[0] == id {
			unused = append(unused, pair{id, refs[0], refs[1], rep})
			continue
		}
		if len(owners) == 2 {
			a := owners[0]
			if a == id {
				a = owners[1]
			}
			as, e := simple(f, a, "PRESENTATION_STYLE_BY_CONTEXT", 2)
			if e == nil && as.Args[1].Kind == Reference && as.Args[1].Ref == refs[1] {
				styled = append(styled, pair{id, refs[0], refs[1], rep})
			}
		}
	}
	if len(unused) > 64 {
		return nil, nil, geom.ErrInvalid
	}
	remove := map[int64]bool{}
	var evidence []UnusedOccurrenceContext
	for _, u := range unused {
		var match int64
		for _, s := range styled {
			if u.pds == s.pds && reflect.DeepEqual(u.record, s.record) {
				if match != 0 {
					return nil, nil, geom.ErrInvalid
				}
				match = s.rep
			}
		}
		if match == 0 || len(incoming[u.sdr]) != 0 {
			return nil, nil, geom.ErrInvalid
		}
		remove[u.sdr] = true
		remove[u.rep] = true
		evidence = append(evidence, UnusedOccurrenceContext{u.sdr, u.rep, u.pds, match})
	}
	var order []int64
	for _, id := range view.Order {
		if remove[id] {
			delete(view.Entities, id)
		} else {
			order = append(order, id)
		}
	}
	view.Order = order
	raw, e = mappedPlanarSourceBytes(view)
	return raw, evidence, e
}
