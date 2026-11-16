package step

import (
	"math"
	"reflect"
	"slices"
)

// Merge a corrected standalone view into its authenticated owning body. The
// original part closures are disjoint, but a standalone generated ID can
// already belong to another part or to the assembly occurrence graph.
func mergeNestedMixedConicalPart(full, before, after *File, bodyID int64, incoming map[int64]int, freshID *int64, l Limits) (map[int64]int64, error) {
	if !reflect.DeepEqual(before.Header, after.Header) {
		return nil, errNestedMixedConicalAP242Repair
	}
	oldGeometry, err := nauoClosure(before, []int64{bodyID}, l.MaxDepth)
	if err != nil {
		return nil, err
	}
	newGeometry, err := nauoClosure(after, []int64{bodyID}, l.MaxDepth)
	if err != nil {
		return nil, err
	}
	localIncoming := conicalRepairIncoming(before)
	ids := make(map[int64]int64, len(after.Entities))
	generated := map[int64]int64{}
	for _, id := range after.Order {
		ids[id] = id
		if _, ok := before.Entities[id]; ok {
			continue
		}
		if *freshID == math.MaxInt64 || !newGeometry[id] || !validNestedMixedConicalNewEntity(after.Entities[id]) {
			return nil, errNestedMixedConicalAP242Repair
		}
		(*freshID)++
		ids[id], generated[id] = *freshID, *freshID
	}
	for _, id := range before.Order {
		old := before.Entities[id]
		current, kept := after.Entities[id]
		if kept && reflect.DeepEqual(old, current) {
			continue
		}
		// An incoming reference from outside this view would let a local
		// correction change neighboring definition or placement data.
		if !oldGeometry[id] || incoming[id] != localIncoming[id] ||
			!reflect.DeepEqual(full.Entities[id], old) || (kept && !newGeometry[id]) {
			return nil, errNestedMixedConicalAP242Repair
		}
		if !kept {
			delete(full.Entities, id)
			continue
		}
		updated, err := remapNestedMixedConicalEntity(current, ids)
		if err != nil {
			return nil, err
		}
		full.Entities[id] = updated
	}
	for _, id := range after.Order {
		key, added := generated[id]
		if !added {
			continue
		}
		updated, err := remapNestedMixedConicalEntity(after.Entities[id], ids)
		if err != nil {
			return nil, err
		}
		full.Entities[key] = updated
		full.Order = append(full.Order, key)
	}
	return generated, nil
}

func validNestedMixedConicalNewEntity(e Entity) bool {
	if len(e.Records) != 1 {
		return false
	}
	return e.Records[0].Type == "AXIS1_PLACEMENT" || e.Records[0].Type == "B_SPLINE_CURVE_WITH_KNOTS"
}

func remapNestedMixedConicalEntity(e Entity, ids map[int64]int64) (Entity, error) {
	e.ID = ids[e.ID]
	if e.ID == 0 {
		return Entity{}, errNestedMixedConicalAP242Repair
	}
	e.Records = slices.Clone(e.Records)
	for i := range e.Records {
		var err error
		e.Records[i].Args, err = remapNestedMixedConicalValues(e.Records[i].Args, ids)
		if err != nil {
			return Entity{}, err
		}
	}
	return e, nil
}

func remapNestedMixedConicalValues(in []Value, ids map[int64]int64) ([]Value, error) {
	out := slices.Clone(in)
	for i, v := range out {
		if v.Kind == Reference {
			out[i].Ref = ids[v.Ref]
			if out[i].Ref == 0 {
				return nil, errNestedMixedConicalAP242Repair
			}
		}
		if len(v.Items) != 0 {
			var err error
			out[i].Items, err = remapNestedMixedConicalValues(v.Items, ids)
			if err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}
