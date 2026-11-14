package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/geom"
	"reflect"
)

// NormalizedIndependentAnalyticHierarchy separates original per-representation
// SI evidence from the exact common-unit graph and its native local materials.
// Geometry and occurrences in Hierarchy are expressed in TargetUnit.
type NormalizedIndependentAnalyticHierarchy struct {
	OriginalSource *File
	Hierarchy      IndependentAnalyticHierarchy
	OriginalUnits  map[int64]IndependentMaterialSIUnit // original representation ID
	TargetUnit     IndependentMaterialSIUnit
	tolerance      float64
}

// ReadNormalizedIndependentAnalyticHierarchy normalizes only metre/mm contexts.
// Placement origins belong to their respective from/to representations; they
// are scaled before rigid transforms are composed. Child mapping frames must
// be neutral; nonzero origins or nonidentity child axes are outside this independently qualified mode. All dimensional model and
// planar UV scalars must normalize without binary64 rounding. The unchanged
// hierarchy proof authenticates topology, ownership and every occurrence path.
func ReadNormalizedIndependentAnalyticHierarchy(ctx context.Context, r io.Reader, target IndependentMaterialSIUnit, l Limits, tol float64) (NormalizedIndependentAnalyticHierarchy, error) {
	bad := func() (NormalizedIndependentAnalyticHierarchy, error) {
		return NormalizedIndependentAnalyticHierarchy{}, geom.ErrInvalid
	}
	if ctx == nil || r == nil || (target != IndependentMaterialSIUnit(RoundMetre) && target != IndependentMaterialSIUnit(RoundMillimetre)) || !finiteNumber(tol) || tol <= 0 {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return NormalizedIndependentAnalyticHierarchy{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 1<<20)
	l.MaxEntities = min(l.MaxEntities, 4096)
	l.MaxValues = min(l.MaxValues, 65536)
	l.MaxDepth = min(l.MaxDepth, 16)
	source, e := Parse(r, l)
	if e != nil {
		return NormalizedIndependentAnalyticHierarchy{}, e
	}
	// Mixed-unit nonzero child mapping origins have not been independently
	// qualified under the consumer's rotated relationship convention.
	for _, en := range source.Entities {
		for _, rec := range en.Records {
			if rec.Type == "ITEM_DEFINED_TRANSFORMATION" {
				if len(rec.Args) != 4 || rec.Args[2].Kind != Reference {
					return bad()
				}
				basis, err := mappedRationalFrame(source, rec.Args[2].Ref)
				if err != nil {
					return bad()
				}
				for j, axis := range [3][3]*big.Rat{basis.X, basis.Y, basis.Z} {
					for k, x := range axis {
						want := int64(0)
						if j == k {
							want = 1
						}
						if x.Cmp(big.NewRat(want, 1)) != 0 {
							return bad()
						}
					}
				}
				fr, e := simple(source, rec.Args[2].Ref, "AXIS2_PLACEMENT_3D", 4)
				if e != nil {
					return bad()
				}
				pt, e := simple(source, fr.Args[1].Ref, "CARTESIAN_POINT", 2)
				if e != nil || pt.Args[1].Kind != Aggregate || len(pt.Args[1].Items) != 3 {
					return bad()
				}
				for _, v := range pt.Args[1].Items {
					q, e := liftSourceRat(v)
					if e != nil || q.Sign() != 0 {
						return bad()
					}
				}
			}
		}
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return bad()
	}
	view, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return bad()
	}
	units := map[int64]IndependentMaterialSIUnit{}
	changed := map[int64]bool{}
	for _, id := range source.Order {
		en := source.Entities[id]
		if len(en.Records) != 1 {
			continue
		}
		rec := en.Records[0]
		if rec.Type != "SHAPE_REPRESENTATION" && rec.Type != "ADVANCED_BREP_SHAPE_REPRESENTATION" {
			continue
		}
		if len(rec.Args) != 3 || rec.Args[2].Kind != Reference || len(units) >= 16 {
			return bad()
		}
		prefix, e := nauoSIPrefix(source, rec.Args[2].Ref)
		if e != nil || (prefix != "$" && prefix != "MILLI") {
			return bad()
		}
		from := IndependentMaterialSIUnit(RoundMetre)
		if prefix == "MILLI" {
			from = IndependentMaterialSIUnit(RoundMillimetre)
		}
		units[id] = from
		ratio := big.NewRat(1, 1)
		if from != target {
			if from == IndependentMaterialSIUnit(RoundMetre) {
				ratio.SetInt64(1000)
			} else {
				ratio.SetFrac64(1, 1000)
			}
		}
		closure, e := nauoClosure(source, []int64{id}, l.MaxDepth)
		if e != nil {
			return bad()
		}
		// Clone from the original, never an already scaled shared record.
		candidate, e := Parse(bytes.NewReader(raw), l)
		if e != nil {
			return bad()
		}
		for _, key := range source.Order {
			if !closure[key] {
				continue
			}
			if e = ctx.Err(); e != nil {
				return NormalizedIndependentAnalyticHierarchy{}, e
			}
			item := candidate.Entities[key]
			// A nested length unit (including uncertainty metadata) must agree
			// with this representation, rather than being silently relabelled.
			for _, rr := range item.Records {
				if rr.Type == "LENGTH_UNIT" {
					for _, uu := range item.Records {
						if uu.Type == "SI_UNIT" {
							if len(uu.Args) != 2 || uu.Args[1].Kind != Enumeration || uu.Args[1].Text != "METRE" {
								return bad()
							}
							if prefix == "$" && uu.Args[0].Kind != Omitted {
								return bad()
							}
							if prefix == "MILLI" && (uu.Args[0].Kind != Enumeration || uu.Args[0].Text != "MILLI") {
								return bad()
							}
						}
					}
				}
			}
			if e = scaleHierarchyUnitEntity(&item, ratio, target); e != nil {
				return NormalizedIndependentAnalyticHierarchy{}, e
			}
			if changed[key] && !reflect.DeepEqual(view.Entities[key], item) {
				return bad()
			}
			changed[key] = true
			view.Entities[key] = item
		}
	}
	if len(units) < 4 {
		return bad()
	}
	normalized, e := mappedPlanarSourceBytes(view)
	if e != nil {
		return bad()
	}
	proof, e := ReadIndependentAnalyticHierarchy(ctx, bytes.NewReader(normalized), l, tol)
	if e != nil {
		return NormalizedIndependentAnalyticHierarchy{}, e
	}
	// Every representation was independently assigned an original unit.
	if len(proof.Assembly.Products) != len(units) {
		return bad()
	}
	return NormalizedIndependentAnalyticHierarchy{OriginalSource: source, Hierarchy: proof, OriginalUnits: units, TargetUnit: target, tolerance: tol}, nil
}

func scaleHierarchyUnitEntity(en *Entity, ratio *big.Rat, target IndependentMaterialSIUnit) error {
	scale := func(v *Value, binary bool) error {
		if v.Kind != Number || len(v.Text) > 128 {
			return geom.ErrInvalid
		}
		q, e := liftSourceRat(*v)
		if e != nil || q.Num().BitLen() > 256 || q.Denom().BitLen() > 256 {
			return geom.ErrInvalid
		}
		q.Mul(q, ratio)
		if q.Num().BitLen() > 256 || q.Denom().BitLen() > 256 {
			return geom.ErrInvalid
		}
		if binary {
			f, ok := q.Float64()
			if !ok || !finiteNumber(f) {
				return geom.ErrUnrepresentable
			}
		}
		s, e := extrusionDirectionDecimal(q)
		if e != nil {
			return e
		}
		v.Text = s
		return nil
	}
	length := false
	for _, r := range en.Records {
		length = length || r.Type == "LENGTH_UNIT"
	}
	for j := range en.Records {
		r := &en.Records[j]
		switch r.Type {
		case "CARTESIAN_POINT":
			if len(r.Args) != 2 || r.Args[1].Kind != Aggregate || (len(r.Args[1].Items) != 2 && len(r.Args[1].Items) != 3) {
				return geom.ErrInvalid
			}
			for k := range r.Args[1].Items {
				if e := scale(&r.Args[1].Items[k], true); e != nil {
					return e
				}
			}
		case "VECTOR", "CIRCLE", "CYLINDRICAL_SURFACE":
			if len(r.Args) != 3 {
				return geom.ErrInvalid
			}
			if e := scale(&r.Args[2], true); e != nil {
				return e
			}
		case "TOROIDAL_SURFACE":
			if len(r.Args) != 4 {
				return geom.ErrInvalid
			}
			for k := 2; k < 4; k++ {
				if e := scale(&r.Args[k], true); e != nil {
					return e
				}
			}
		case "UNCERTAINTY_MEASURE_WITH_UNIT":
			if len(r.Args) != 4 || r.Args[0].Kind != Typed || r.Args[0].Text != "LENGTH_MEASURE" || len(r.Args[0].Items) != 1 {
				return geom.ErrInvalid
			}
			if e := scale(&r.Args[0].Items[0], false); e != nil {
				return e
			}
		case "SI_UNIT":
			if length {
				if len(r.Args) != 2 {
					return geom.ErrInvalid
				}
				if target == IndependentMaterialSIUnit(RoundMetre) {
					r.Args[0] = Value{Kind: Omitted}
				} else {
					r.Args[0] = Value{Kind: Enumeration, Text: "MILLI"}
				}
			}
		}
	}
	return nil
}

// WriteNormalizedIndependentAnalyticHierarchySource re-proves and re-emits the
// original mixed-unit records and IDs, rather than the normalized graph. To
// emit normalized geometry use WriteIndependentAnalyticHierarchySource with
// the returned Hierarchy.Assembly.Source.
func WriteNormalizedIndependentAnalyticHierarchySource(ctx context.Context, w io.Writer, source *File, target IndependentMaterialSIUnit, l Limits, tol float64) error {
	if ctx == nil || w == nil || !independentNativeSourceBound(source, 4096) {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return e
	}
	if _, e = ReadNormalizedIndependentAnalyticHierarchy(ctx, bytes.NewReader(raw), target, l, tol); e != nil {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}
