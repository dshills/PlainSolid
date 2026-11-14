package step

import (
	"bytes"
	"fmt"
	"io"
)

// SplineGeneratrixCylinderProduct retains original full-domain spline edge
// geometry alongside the exactly equivalent canonical four-face cylinder.
// Curves is keyed by source EDGE_CURVE ID; curve senses remain in SameSense.
type SplineGeneratrixCylinderProduct struct {
	Product   SplitCylinderProduct
	Curves    map[int64]SplineCurve
	SameSense map[int64]bool
}

// ReadSplineGeneratrixCylinderProduct accepts a rooted half-split cylinder with
// exactly two clamped degree-one spline generatrices and model-only circle rims.
// Exact positive rational weights may change parameter speed, never the line
// segment. All source poles must equal the directed topological endpoints.
// Other readers and vendor pcurve acceptance are unchanged.
func ReadSplineGeneratrixCylinderProduct(r io.Reader, l Limits) (SplineGeneratrixCylinderProduct, error) {
	fail := func() (SplineGeneratrixCylinderProduct, error) {
		return SplineGeneratrixCylinderProduct{}, fmt.Errorf("step: unsupported exact spline-generatrix cylinder product")
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	f, e := Parse(r, l)
	if e != nil {
		return SplineGeneratrixCylinderProduct{}, e
	}
	if len(f.Entities) > 240 {
		return fail()
	}
	refs := map[int64]int{}
	var visit func(Value)
	visit = func(v Value) {
		if v.Kind == Reference {
			refs[v.Ref]++
		}
		for _, a := range v.Items {
			visit(a)
		}
	}
	var max int64
	for id, en := range f.Entities {
		if id > max {
			max = id
		}
		for _, rr := range en.Records {
			for _, v := range rr.Args {
				visit(v)
			}
		}
	}
	curves := map[int64]SplineCurve{}
	senses := map[int64]bool{}
	drop := map[int64]bool{}
	extra := map[int64]string{}
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) != 1 || en.Records[0].Type != "EDGE_CURVE" {
			continue
		}
		a, b, cid, sense, e := cylinderCellEdge(f, id)
		if e != nil {
			return fail()
		}
		ce := f.Entities[cid]
		_, spline := component(ce, "B_SPLINE_CURVE_WITH_KNOTS")
		if !spline {
			continue
		}
		if refs[cid] != 1 || len(curves) >= 2 {
			return fail()
		}
		c, e := DecodeBSplineCurve(f, cid, 2)
		if e != nil || c.Geometry.Degree() != 1 {
			return fail()
		}
		var poles []int64
		for _, rr := range ce.Records {
			var base []Value
			if rr.Type == "B_SPLINE_CURVE" {
				base = rr.Args
			} else if rr.Type == "B_SPLINE_CURVE_WITH_KNOTS" && len(rr.Args) == 9 {
				base = rr.Args[1:6]
			}
			if base != nil {
				if len(base) != 5 || base[2].Kind != Enumeration || base[2].Text != "UNSPECIFIED" || base[3].Text != "F" || base[4].Text != "U" {
					return fail()
				}
				poles, e = listRefs(base[1], 2)
				if e != nil || len(poles) != 2 {
					return fail()
				}
			}
			if rr.Type == "B_SPLINE_CURVE_WITH_KNOTS" {
				v := rr.Args[len(rr.Args)-1]
				if v.Kind != Enumeration || v.Text != "UNSPECIFIED" {
					return fail()
				}
			}
			var exact func(Value) bool
			exact = func(v Value) bool {
				if v.Kind == Number {
					_, err := cylinderSupportNumber(v)
					return err == nil
				}
				for _, x := range v.Items {
					if !exact(x) {
						return false
					}
				}
				return true
			}
			for _, v := range rr.Args {
				if !exact(v) {
					return fail()
				}
			}
		}
		if len(poles) != 2 {
			return fail()
		}
		if e = exactSplineWeightNormalization(f, cid, "RATIONAL_B_SPLINE_CURVE"); e != nil {
			return fail()
		}
		points := c.Geometry.Poles()
		for i, pid := range poles {
			p, e := splitTriple(f, pid, "CARTESIAN_POINT")
			if e != nil || p != points[i] {
				return fail()
			}
			rr, _ := simple(f, pid, "CARTESIAN_POINT", 2)
			name, e := textValue(rr.Args[0])
			if e != nil || name != "" {
				return fail()
			}
			if refs[pid] == 1 {
				drop[pid] = true
			}
		}
		vertex := func(vid int64) (int64, error) {
			v, e := simple(f, vid, "VERTEX_POINT", 2)
			if e != nil {
				return 0, e
			}
			return ref(v.Args[1])
		}
		pa, e := vertex(a)
		if e != nil {
			return fail()
		}
		pb, e := vertex(b)
		if e != nil {
			return fail()
		}
		va, e := splitTriple(f, pa, "CARTESIAN_POINT")
		if e != nil {
			return fail()
		}
		vb, e := splitTriple(f, pb, "CARTESIAN_POINT")
		if e != nil {
			return fail()
		}
		start, end := points[0], points[1]
		origin := pa
		if !sense {
			va, vb = vb, va
		}
		if start != va || end != vb || start == end {
			return fail()
		}
		coordsA := [3]float64{start.X, start.Y, start.Z}
		coordsB := [3]float64{end.X, end.Y, end.Z}
		dir := [3]int{}
		nonzero := 0
		for i := range dir {
			if coordsA[i] != coordsB[i] {
				nonzero++
				dir[i] = 1
				if coordsB[i] < coordsA[i] {
					dir[i] = -1
				}
			}
		}
		if nonzero != 1 {
			return fail()
		}
		did := max + int64(len(extra)) + 1
		extra[did] = fmt.Sprintf("DIRECTION('',(%d.,%d.,%d.))", dir[0], dir[1], dir[2])
		vid := did + 1
		extra[vid] = fmt.Sprintf("VECTOR('',#%d,1.)", did)
		// Keep the original curve entity ID so all authored coedge ownership survives.
		f.Entities[cid] = Entity{ID: cid, Records: []Record{{Type: "LINE", Args: []Value{{Kind: String, Text: ""}, {Kind: Reference, Ref: origin}, {Kind: Reference, Ref: vid}}}}}
		curves[id] = c
		senses[id] = sense
	}
	if len(curves) != 2 {
		return fail()
	}
	// All generated dependencies are local, bounded and semantically checked by
	// the unchanged whole-graph cylinder reader.
	var staged bytes.Buffer
	staged.WriteString("ISO-10303-21;HEADER;FILE_DESCRIPTION((''),'2;1');FILE_NAME('','',(''),(''),'','','');FILE_SCHEMA(('CONFIG_CONTROL_DESIGN'));ENDSEC;DATA;")
	for id := max + 1; id <= max+int64(len(extra)); id++ {
		fmt.Fprintf(&staged, "#%d=%s;", id, extra[id])
	}
	staged.WriteString("ENDSEC;END-ISO-10303-21;")
	parsed, err := Parse(&staged, Limits{})
	if err != nil {
		return fail()
	}
	for _, id := range parsed.Order {
		f.Entities[id] = parsed.Entities[id]
		f.Order = append(f.Order, id)
	}
	raw := splitProductFile(f, drop, nil)
	p, e := ReadSplitCylinderProduct(bytes.NewReader(raw), l)
	if e != nil {
		return fail()
	}
	// Exactly the two axial edges must have carried the source spline records.
	if _, ok := curves[p.Model.Edges[4]]; !ok {
		return fail()
	}
	if _, ok := curves[p.Model.Edges[5]]; !ok {
		return fail()
	}
	return SplineGeneratrixCylinderProduct{p, curves, senses}, nil
}
