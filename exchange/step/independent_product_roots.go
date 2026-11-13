package step

import (
	"fmt"
)

// independentProductRoots recognizes the complete registered AP203 product
// envelope. It does not infer assembly relationships or discard loose metadata.
func independentProductRoots(f *File, rep int64) ([]int64, error) {
	fail := func() ([]int64, error) {
		return nil, fmt.Errorf("step: incomplete or unsupported independent product envelope")
	}
	var sdr int64
	for id, en := range f.Entities {
		for _, r := range en.Records {
			if r.Type == "SHAPE_DEFINITION_REPRESENTATION" {
				if len(en.Records) != 1 || len(r.Args) != 2 || r.Args[1].Kind != Reference {
					return fail()
				}
				if r.Args[1].Ref == rep {
					if sdr != 0 {
						return fail()
					}
					sdr = id
				}
			}
		}
	}
	if sdr == 0 {
		return nil, nil
	}
	ids := []int64{sdr}
	next := func(v Value, kind string, n int) (Record, bool) {
		id, e := ref(v)
		if e != nil {
			return Record{}, false
		}
		r, e := simple(f, id, kind, n)
		if e != nil {
			return Record{}, false
		}
		ids = append(ids, id)
		return r, true
	}
	text := func(v Value, s string) bool { return v.Kind == String && v.Text == s }
	r, _ := simple(f, sdr, "SHAPE_DEFINITION_REPRESENTATION", 2)
	pds, ok := next(r.Args[0], "PRODUCT_DEFINITION_SHAPE", 3)
	if !ok || !text(pds.Args[0], "") || !text(pds.Args[1], "") {
		return fail()
	}
	pd, ok := next(pds.Args[2], "PRODUCT_DEFINITION", 4)
	if !ok || !text(pd.Args[0], "design") || !text(pd.Args[1], "") {
		return fail()
	}
	formation, ok := next(pd.Args[2], "PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE", 4)
	if !ok || !text(formation.Args[0], "") || !text(formation.Args[1], "") || formation.Args[3].Kind != Enumeration || formation.Args[3].Text != "NOT_KNOWN" {
		return fail()
	}
	product, ok := next(formation.Args[2], "PRODUCT", 4)
	if !ok || product.Args[0].Kind != String || product.Args[1].Kind != String || !text(product.Args[2], "") || product.Args[3].Kind != Aggregate || len(product.Args[3].Items) != 1 {
		return fail()
	}
	mc, ok := next(product.Args[3].Items[0], "MECHANICAL_CONTEXT", 3)
	if !ok || !text(mc.Args[0], "") || !text(mc.Args[2], "mechanical") {
		return fail()
	}
	dc, ok := next(pd.Args[3], "DESIGN_CONTEXT", 3)
	if !ok || !text(dc.Args[0], "") || !text(dc.Args[2], "design") || dc.Args[1].Kind != Reference || mc.Args[1].Kind != Reference || dc.Args[1].Ref != mc.Args[1].Ref {
		return fail()
	}
	app, ok := next(mc.Args[1], "APPLICATION_CONTEXT", 1)
	if !ok || !text(app.Args[0], "configuration controlled 3d designs of mechanical parts and assemblies") {
		return fail()
	}
	var apd int64
	for id, en := range f.Entities {
		for _, r := range en.Records {
			if r.Type == "APPLICATION_PROTOCOL_DEFINITION" {
				if len(en.Records) != 1 || len(r.Args) != 4 {
					return fail()
				}
				if r.Args[3].Kind == Reference && r.Args[3].Ref == mc.Args[1].Ref {
					if apd != 0 || !text(r.Args[0], "international standard") || !text(r.Args[1], "config_control_design") || r.Args[2].Kind != Number || len(r.Args[2].Text) > 16 {
						return fail()
					}
					year, err := cylinderSupportNumber(r.Args[2])
					if err != nil || year != 1994 {
						return fail()
					}
					apd = id
				}
			}
		}
	}
	if apd == 0 {
		return fail()
	}
	ids = append(ids, apd)
	seen := map[int64]bool{}
	for _, id := range ids {
		if seen[id] {
			return fail()
		}
		seen[id] = true
	}
	return []int64{sdr, apd}, nil
}

// Only declarative context/unit nodes can be shared by independent model roots.
// In particular products, placements, points, curves and topology cannot be.
func independentShareable(e Entity) bool {
	allowed := map[string]bool{
		"APPLICATION_CONTEXT": true, "APPLICATION_PROTOCOL_DEFINITION": true, "MECHANICAL_CONTEXT": true, "DESIGN_CONTEXT": true,
		"GEOMETRIC_REPRESENTATION_CONTEXT": true, "GLOBAL_UNIT_ASSIGNED_CONTEXT": true, "GLOBAL_UNCERTAINTY_ASSIGNED_CONTEXT": true, "REPRESENTATION_CONTEXT": true,
		"NAMED_UNIT": true, "LENGTH_UNIT": true, "PLANE_ANGLE_UNIT": true, "SOLID_ANGLE_UNIT": true, "SI_UNIT": true, "CONVERSION_BASED_UNIT": true, "DIMENSIONAL_EXPONENTS": true, "LENGTH_MEASURE_WITH_UNIT": true, "PLANE_ANGLE_MEASURE_WITH_UNIT": true, "MEASURE_WITH_UNIT": true, "UNCERTAINTY_MEASURE_WITH_UNIT": true,
	}
	for _, r := range e.Records {
		if !allowed[r.Type] {
			return false
		}
	}
	return len(e.Records) > 0
}
