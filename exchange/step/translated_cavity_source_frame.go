package step

import (
	"plainsolid/geom"
)

// A separate related coordinate representation avoids STEP readers treating
// an axis item in the solid representation as an additional shape transform.
func cavitySourceFrame(f *File, add, remove bool) (int64, error) {
	var mainID, relationID int64
	next := int64(1)
	for _, id := range f.Order {
		if id >= next {
			next = id + 1
		}
		if _, e := simple(f, id, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3); e == nil {
			if mainID != 0 {
				return 0, geom.ErrInvalid
			}
			mainID = id
		}
		if _, e := simple(f, id, "REPRESENTATION_RELATIONSHIP", 4); e == nil {
			if relationID != 0 {
				return 0, geom.ErrInvalid
			}
			relationID = id
		}
	}
	main, e := simple(f, mainID, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
	if e != nil {
		return 0, e
	}
	refv := func(id int64) Value { return Value{Kind: Reference, Ref: id} }
	put := func(typ string, args []Value) int64 {
		id := next
		next++
		f.Entities[id] = Entity{ID: id, Records: []Record{{Type: typ, Args: args}}}
		f.Order = append(f.Order, id)
		return id
	}
	if add {
		if relationID != 0 {
			return 0, geom.ErrInvalid
		}
		point := func(typ string, xyz [3]string) int64 {
			items := []Value{}
			for _, s := range xyz {
				items = append(items, Value{Kind: Number, Text: s})
			}
			return put(typ, []Value{{Kind: String}, {Kind: Aggregate, Items: items}})
		}
		p := point("CARTESIAN_POINT", [3]string{"0.", "0.", "0."})
		z := point("DIRECTION", [3]string{"0.", "0.", "1."})
		x := point("DIRECTION", [3]string{"1.", "0.", "0."})
		frame := put("AXIS2_PLACEMENT_3D", []Value{{Kind: String}, refv(p), refv(z), refv(x)})
		rep := put("SHAPE_REPRESENTATION", []Value{{Kind: String}, {Kind: Aggregate, Items: []Value{refv(frame)}}, main.Args[2]})
		put("REPRESENTATION_RELATIONSHIP", []Value{{Kind: String}, {Kind: String}, refv(mainID), refv(rep)})
		return frame, nil
	}
	relation, e := simple(f, relationID, "REPRESENTATION_RELATIONSHIP", 4)
	if e != nil {
		return 0, e
	}
	a, e := ref(relation.Args[2])
	if e != nil || a != mainID {
		return 0, geom.ErrInvalid
	}
	repID, e := ref(relation.Args[3])
	if e != nil {
		return 0, e
	}
	rep, e := simple(f, repID, "SHAPE_REPRESENTATION", 3)
	if e != nil {
		return 0, e
	}
	if rep.Args[1].Kind != Aggregate || len(rep.Args[1].Items) != 1 {
		return 0, geom.ErrInvalid
	}
	frame, e := ref(rep.Args[1].Items[0])
	if e != nil {
		return 0, e
	}
	fr, e := simple(f, frame, "AXIS2_PLACEMENT_3D", 4)
	if e != nil {
		return 0, e
	}
	if !remove {
		return frame, nil
	}
	ids := map[int64]bool{frame: true, repID: true, relationID: true}
	for _, v := range fr.Args[1:] {
		id, e := ref(v)
		if e != nil {
			return 0, e
		}
		ids[id] = true
	}
	if len(ids) != 6 {
		return 0, geom.ErrInvalid
	}
	for id := range ids {
		delete(f.Entities, id)
	}
	order := []int64{}
	for _, id := range f.Order {
		if !ids[id] {
			order = append(order, id)
		}
	}
	f.Order = order
	return frame, nil
}
