package step

// ellipseFaceGraph proves an injective dependency graph rooted at all supplied roles.
func ellipseFaceGraph(want, f *File, roots [][2]int64) (map[int64]int64, bool) {
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
			wr, we := liftSourceRat(w)
			vr, ve := liftSourceRat(v)
			return we == nil && ve == nil && wr.Cmp(vr) == 0
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

// ellipseFaceSourceBound rejects excessive or cyclic caller-owned Value trees
// before source serialization; parsed files already satisfy parser limits.
func ellipseFaceSourceBound(f *File) bool {
	if f == nil || len(f.Entities) != 47 || len(f.Order) != 47 || len(f.Header) > 64 {
		return false
	}
	budget, text := 4096, 32768
	var check func(Value, int) bool
	check = func(v Value, depth int) bool {
		budget--
		text -= len(v.Text)
		if budget < 0 || text < 0 || depth > 16 || len(v.Text) > 1024 {
			return false
		}
		for _, a := range v.Items {
			if !check(a, depth+1) {
				return false
			}
		}
		return true
	}
	records := append([]Record(nil), f.Header...)
	for _, en := range f.Entities {
		if len(en.Records) > 4 {
			return false
		}
		records = append(records, en.Records...)
	}
	for _, r := range records {
		if len(r.Type) > 128 {
			return false
		}
		for _, a := range r.Args {
			if !check(a, 0) {
				return false
			}
		}
	}
	return true
}
