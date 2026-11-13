package step

import "errors"

// Analytic support codecs retain only length units and use radians for their
// parameter conventions. Do not silently accept other angular units or malformed
// dimensional metadata through the more permissive legacy context reader.
func analyticSupportContextUnits(f *File, id int64) (float64, error) {
	fail := func() (float64, error) { return 0, errors.New("step: unsupported analytic support unit context") }
	unit, e := contextUnits(f, id)
	if e != nil {
		return 0, e
	}
	ent := f.Entities[id]
	if len(ent.Records) != 3 {
		return fail()
	}
	g, ok := component(ent, "GEOMETRIC_REPRESENTATION_CONTEXT")
	if !ok || len(g.Args) != 1 {
		return fail()
	}
	dim, e := integer(g.Args[0])
	if e != nil || dim != 3 {
		return fail()
	}
	u, ok := component(ent, "GLOBAL_UNIT_ASSIGNED_CONTEXT")
	if !ok || len(u.Args) != 1 {
		return fail()
	}
	ids, e := listRefs(u.Args[0], 1)
	if e != nil || len(ids) != 3 {
		return fail()
	}
	seen := map[string]bool{}
	var lengthID int64
	for _, uid := range ids {
		x := f.Entities[uid]
		category := ""
		for _, kind := range []string{"LENGTH_UNIT", "PLANE_ANGLE_UNIT", "SOLID_ANGLE_UNIT"} {
			if _, ok := component(x, kind); ok {
				if category != "" {
					return fail()
				}
				category = kind
			}
		}
		if category == "" || seen[category] {
			return fail()
		}
		seen[category] = true
		if category == "LENGTH_UNIT" {
			lengthID = uid
			continue
		}
		c, _ := component(x, category)
		n, nok := component(x, "NAMED_UNIT")
		si, sok := component(x, "SI_UNIT")
		want := "RADIAN"
		if category == "SOLID_ANGLE_UNIT" {
			want = "STERADIAN"
		}
		if len(x.Records) != 3 || len(c.Args) != 0 || !nok || len(n.Args) != 1 || n.Args[0].Kind != Derived || !sok || len(si.Args) != 2 || si.Args[0].Kind != Omitted || si.Args[1].Kind != Enumeration || si.Args[1].Text != want {
			return fail()
		}
	}
	// lengthUnit validates graph shape and cycles; inspect each conversion's
	// dimensional exponents without rounding near-integers to admissible values.
	seenLength := map[int64]bool{}
	for depth := 0; ; depth++ {
		if depth > 64 || seenLength[lengthID] {
			return fail()
		}
		seenLength[lengthID] = true
		x := f.Entities[lengthID]
		if _, ok := component(x, "SI_UNIT"); ok {
			break
		}
		n, ok := component(x, "NAMED_UNIT")
		c, cok := component(x, "CONVERSION_BASED_UNIT")
		if !ok || len(n.Args) != 1 || !cok || len(c.Args) != 2 {
			return fail()
		}
		did, e := ref(n.Args[0])
		if e != nil {
			return fail()
		}
		d, e := simple(f, did, "DIMENSIONAL_EXPONENTS", 7)
		if e != nil {
			return fail()
		}
		for i, v := range d.Args {
			value, e := cylinderSupportNumber(v)
			want := 0.
			if i == 0 {
				want = 1
			}
			if e != nil || value != want {
				return fail()
			}
		}
		mid, e := ref(c.Args[1])
		if e != nil {
			return fail()
		}
		m := f.Entities[mid]
		if len(m.Records) != 1 || len(m.Records[0].Args) != 2 {
			return fail()
		}
		lengthID, e = ref(m.Records[0].Args[1])
		if e != nil {
			return fail()
		}
	}
	return unit, nil
}
