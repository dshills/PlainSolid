package step

import "errors"

// normalizeAffineIsoPCurves changes only the proof view, after proving a
// bounded linear spline p-curve is exactly (t,c) or (c,t) on the owning model
// curve's full parameter domain. No point fitting or sampled comparison occurs.
func normalizeAffineIsoPCurves(f *File, maxPoints int) error {
	fail := func() error { return errors.New("step: unsupported affine spline p-curve") }
	next := int64(-1)
	emit := func(r Record) int64 {
		for {
			if _, ok := f.Entities[next]; !ok {
				break
			}
			next--
		}
		id := next
		next--
		f.Entities[id] = Entity{ID: id, Records: []Record{r}}
		return id
	}
	normalized := map[int64][2]float64{}
	modelDomains := map[int64][2]float64{}
	modelDomain := func(id int64) ([2]float64, error) {
		if d, ok := modelDomains[id]; ok {
			return d, nil
		}
		model, e := DecodeBSplineCurve(f, id, maxPoints)
		if e != nil {
			return [2]float64{}, e
		}
		lo, hi := model.Geometry.Domain()
		d := [2]float64{lo, hi}
		modelDomains[id] = d
		return d, nil
	}

	num := func(x float64) Value { return Value{Kind: Number, Text: real(x)} }
	for _, id := range f.Order {
		sc, e := simple(f, id, "SURFACE_CURVE", 4)
		if e != nil {
			continue
		}
		modelID, e := ref(sc.Args[1])
		if e != nil {
			return fail()
		}
		pcs, e := listRefs(sc.Args[2], 1)
		if e != nil {
			return fail()
		}
		for _, pcid := range pcs {
			pc, e := simple(f, pcid, "PCURVE", 3)
			if e != nil {
				return fail()
			}
			rid, e := ref(pc.Args[2])
			if e != nil {
				return fail()
			}
			rep, e := simple(f, rid, "DEFINITIONAL_REPRESENTATION", 3)
			if e != nil {
				return fail()
			}
			ids, e := listRefs(rep.Args[1], 1)
			if e != nil || len(ids) != 1 {
				return fail()
			}
			cid := ids[0]
			if _, e := simple(f, cid, "LINE", 3); e == nil {
				if domain, ok := normalized[cid]; ok {
					actual, e := modelDomain(modelID)
					if e != nil {
						return fail()
					}
					if actual != domain {
						return fail()
					}
				}
				continue
			}
			curve, e := simple(f, cid, "B_SPLINE_CURVE_WITH_KNOTS", 9)
			if e != nil {
				return fail()
			}
			if _, e := textValue(curve.Args[0]); e != nil {
				return fail()
			}
			degree, e := integer(curve.Args[1])
			if e != nil || degree != 1 {
				return fail()
			}
			pids, e := listRefs(curve.Args[2], 2)
			if e != nil || len(pids) != 2 {
				return fail()
			}
			for _, k := range []int{4, 5} {
				b, e := boolean(curve.Args[k])
				if e != nil || b {
					return fail()
				}
			}
			if curve.Args[3].Kind != Enumeration || curve.Args[3].Text != "UNSPECIFIED" || curve.Args[6].Kind != Aggregate || len(curve.Args[6].Items) != 2 || curve.Args[7].Kind != Aggregate || len(curve.Args[7].Items) != 2 {
				return fail()
			}
			for _, v := range curve.Args[6].Items {
				n, e := integer(v)
				if e != nil || n != 2 {
					return fail()
				}
			}
			if curve.Args[8].Kind != Enumeration {
				return fail()
			}
			switch curve.Args[8].Text {
			case "UNIFORM_KNOTS", "QUASI_UNIFORM_KNOTS", "PIECEWISE_BEZIER_KNOTS", "UNSPECIFIED":
			default:
				return fail()
			}
			knots := [2]float64{}
			points := [2][2]float64{}
			for i := 0; i < 2; i++ {
				knots[i], e = numeric(curve.Args[7].Items[i])
				if e != nil {
					return fail()
				}
				point, e := simple(f, pids[i], "CARTESIAN_POINT", 2)
				if e != nil || point.Args[1].Kind != Aggregate || len(point.Args[1].Items) != 2 {
					return fail()
				}
				if _, e := textValue(point.Args[0]); e != nil {
					return fail()
				}
				for k := 0; k < 2; k++ {
					points[i][k], e = numeric(point.Args[1].Items[k])
					if e != nil {
						return fail()
					}
				}
			}
			if knots[0] >= knots[1] {
				return fail()
			}
			axis := -1
			for k := 0; k < 2; k++ {
				if points[0][k] == knots[0] && points[1][k] == knots[1] && points[0][1-k] == points[1][1-k] {
					axis = k
					break
				}
			}
			if axis < 0 {
				return fail()
			}
			actual, e := modelDomain(modelID)
			if e != nil {
				return fail()
			}
			if actual != knots {
				return fail()
			}
			normalized[cid] = knots
			fixed := points[0][1-axis]
			p, d := [2]float64{}, [2]float64{}
			p[1-axis] = fixed
			d[axis] = 1
			point := emit(Record{Type: "CARTESIAN_POINT", Args: []Value{{Kind: String}, {Kind: Aggregate, Items: []Value{num(p[0]), num(p[1])}}}})
			direction := emit(Record{Type: "DIRECTION", Args: []Value{{Kind: String}, {Kind: Aggregate, Items: []Value{num(d[0]), num(d[1])}}}})
			vector := emit(Record{Type: "VECTOR", Args: []Value{{Kind: String}, {Kind: Reference, Ref: direction}, num(1)}})
			f.Entities[cid] = Entity{ID: cid, Records: []Record{{Type: "LINE", Args: []Value{{Kind: String}, {Kind: Reference, Ref: point}, {Kind: Reference, Ref: vector}}}}}
		}
	}
	return nil
}
