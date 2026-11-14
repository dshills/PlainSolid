package step

import (
	"bytes"
	"fmt"
	"io"
)

// GeneratrixParameterTrim preserves the actual two-level source orientation and
// finite parameter selectors. Basis geometry remains in Cylinder.Curves.
type GeneratrixParameterTrim struct {
	Entity, Basis                 int64
	Name                          string
	Parameters                    [2]float64
	SenseAgreement, EdgeSameSense bool
}

// TrimmedGeneratrixCylinderProduct retains explicit source trim provenance.
// Cylinder.SameSense contains effective basis direction, not EDGE_CURVE's flag;
// the original independent flags are retained in Trims, keyed by EDGE_CURVE ID.
type TrimmedGeneratrixCylinderProduct struct {
	Cylinder SplineGeneratrixCylinderProduct
	Trims    map[int64]GeneratrixParameterTrim
}

// ReadTrimmedGeneratrixCylinderProduct accepts two independently owned full-
// domain PARAMETER trims over exact degree-one spline generatrices of the strict
// split-cylinder product. Both trim senses are supported; partial trims, point
// selectors, nested curves, pcurves and tolerance-based endpoint repair are not.
func ReadTrimmedGeneratrixCylinderProduct(r io.Reader, l Limits) (TrimmedGeneratrixCylinderProduct, error) {
	fail := func() (TrimmedGeneratrixCylinderProduct, error) {
		return TrimmedGeneratrixCylinderProduct{}, fmt.Errorf("step: unsupported exact parameter-trimmed generatrix cylinder")
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	f, e := Parse(r, l)
	if e != nil {
		return TrimmedGeneratrixCylinderProduct{}, e
	}
	if len(f.Entities) > 240 {
		return fail()
	}
	counts := map[int64]int{}
	var count func(Value)
	count = func(v Value) {
		if v.Kind == Reference {
			counts[v.Ref]++
		}
		for _, x := range v.Items {
			count(x)
		}
	}
	for _, en := range f.Entities {
		for _, rr := range en.Records {
			for _, v := range rr.Args {
				count(v)
			}
		}
	}
	trims := map[int64]GeneratrixParameterTrim{}
	drop := map[int64]bool{}
	for _, id := range f.Order {
		ent := f.Entities[id]
		if len(ent.Records) != 1 || ent.Records[0].Type != "EDGE_CURVE" {
			continue
		}
		_, _, cid, edgeSense, e := cylinderCellEdge(f, id)
		if e != nil {
			return fail()
		}
		rr, ok := component(f.Entities[cid], "TRIMMED_CURVE")
		if !ok {
			continue
		}
		if len(trims) >= 2 || len(f.Entities[cid].Records) != 1 || len(rr.Args) != 6 || counts[cid] != 1 {
			return fail()
		}
		name, e := textValue(rr.Args[0])
		if e != nil {
			return fail()
		}
		basis, e := ref(rr.Args[1])
		if e != nil || counts[basis] != 1 {
			return fail()
		}
		sense, e := boolean(rr.Args[4])
		if e != nil || rr.Args[5].Kind != Enumeration || rr.Args[5].Text != "PARAMETER" {
			return fail()
		}
		c, e := DecodeBSplineCurve(f, basis, 2)
		if e != nil || c.Geometry.Degree() != 1 {
			return fail()
		}
		lo, hi := c.Geometry.Domain()
		bounds := [2]float64{lo, hi}
		for i, v := range rr.Args[2:4] {
			if v.Kind != Aggregate || len(v.Items) != 1 {
				return fail()
			}
			p := v.Items[0]
			if p.Kind != Typed || p.Text != "PARAMETER_VALUE" || len(p.Items) != 1 {
				return fail()
			}
			x, e := cylinderSupportNumber(p.Items[0])
			if e != nil || x != bounds[i] {
				return fail()
			}
		}
		trims[id] = GeneratrixParameterTrim{cid, basis, name, bounds, sense, edgeSense}
		drop[cid] = true
		rec := ent.Records[0]
		rec.Args[3] = Value{Kind: Reference, Ref: basis}
		flag := "F"
		if sense == edgeSense {
			flag = "T"
		}
		rec.Args[4] = Value{Kind: Enumeration, Text: flag}
		f.Entities[id] = Entity{ID: id, Records: []Record{rec}}
	}
	if len(trims) != 2 {
		return fail()
	}
	out, e := ReadSplineGeneratrixCylinderProduct(bytes.NewReader(splitProductFile(f, drop, nil)), l)
	if e != nil {
		return fail()
	}
	for id := range out.Curves {
		if _, ok := trims[id]; !ok {
			return fail()
		}
	}
	return TrimmedGeneratrixCylinderProduct{out, trims}, nil
}
