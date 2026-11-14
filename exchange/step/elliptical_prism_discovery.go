package step

// ellipsePrismDiscovery discovers roles only. Numeric values are NOT proved here.
// Its output must pass the complete exact final graph comparison.
func ellipsePrismDiscovery(want, f *File, roots [][2]int64) (map[int64]int64, bool) {
	mapping, reverse := map[int64]int64{}, map[int64]int64{}
	var matchEntity func(int64, int64) bool
	var matchValue func(Value, Value) bool
	matchValue = func(w, v Value) bool {
		if w.Kind != v.Kind {
			return false
		}
		switch w.Kind {
		case Reference:
			return matchEntity(w.Ref, v.Ref)
		case String:
			return len(v.Text) <= 1024
		case Number:
			_, e := liftSourceRat(v)
			return e == nil
		}
		if w.Text != v.Text || len(w.Items) != len(v.Items) {
			return false
		}
		for i := range w.Items {
			if !matchValue(w.Items[i], v.Items[i]) {
				return false
			}
		}
		return true
	}
	matchEntity = func(wid, id int64) bool {
		if old, ok := mapping[wid]; ok {
			return old == id
		}
		if _, ok := reverse[id]; ok {
			return false
		}
		w, ok := want.Entities[wid]
		if !ok {
			return false
		}
		v, ok := f.Entities[id]
		if !ok || len(w.Records) != len(v.Records) {
			return false
		}
		mapping[wid] = id
		reverse[id] = wid
		for i, wr := range w.Records {
			vr := v.Records[i]
			if wr.Type != vr.Type || len(wr.Args) != len(vr.Args) {
				return false
			}
			for j := range wr.Args {
				if !matchValue(wr.Args[j], vr.Args[j]) {
					return false
				}
			}
		}
		return true
	}
	for _, r := range roots {
		if !matchEntity(r[0], r[1]) {
			return nil, false
		}
	}
	return mapping, len(mapping) == len(want.Entities) && len(mapping) == len(f.Entities)
}
