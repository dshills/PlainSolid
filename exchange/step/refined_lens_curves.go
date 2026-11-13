package step

import (
	"bytes"
	"fmt"
	"io"
	"plainsolid/geom"
)

// RefinedSplineLens retains independent model/upper-UV/lower-UV spline bases for
// each exact shared seam piece, in canonical piece order. Geometry and native
// material follow PartitionedSplineLens; these source curves are independently
// re-proved on write. Their positive weights must normalize without rounding.
type RefinedSplineLens struct {
	PartitionedSplineLens
	Curves [][3]*geom.BSpline
}

// ReadRefinedSplineLensBody proves complete model/owner agreement while
// retaining independent continuous clamped unit-domain spline bases.
func ReadRefinedSplineLensBody(r io.Reader, l Limits, o CurvedOvalOptions) (RefinedSplineLens, error) {
	m, e := readSplineLensBody(r, l, o, true, true)
	if e != nil {
		return RefinedSplineLens{}, e
	}
	return RefinedSplineLens{PartitionedSplineLens: PartitionedSplineLens{Body: m, Pieces: m.sourcePieces}, Curves: m.sourceCurves}, nil
}

// WriteRefinedSplineLensBody re-proves all supplied source curves and retains
// their bases; cached material and source identifiers are not proof inputs.
func WriteRefinedSplineLensBody(w io.Writer, m RefinedSplineLens, o CurvedOvalOptions) error {
	if len(m.Curves) == 0 {
		return geom.ErrInvalid
	}
	return writePartitionedSplineLensBody(w, m.Body, o, m.Curves)
}
func decodeRefinedLensCurve(f *File, id int64, two bool, o CurvedOvalOptions) (*geom.BSpline, *geom.ExactSplineCurve, error) {
	if e := exactSplineWeightNormalization(f, id, "RATIONAL_B_SPLINE_CURVE"); e != nil {
		return nil, nil, e
	}
	local := f
	if two {
		base, ok := component(f.Entities[id], "B_SPLINE_CURVE")
		if !ok {
			base, ok = component(f.Entities[id], "B_SPLINE_CURVE_WITH_KNOTS")
			if !ok || len(base.Args) < 3 {
				return nil, nil, geom.ErrInvalid
			}
			base.Args = base.Args[1:]
		}
		if len(base.Args) < 2 {
			return nil, nil, geom.ErrInvalid
		}
		ids, e := listRefs(base.Args[1], 2)
		if e != nil || len(ids) > 64 {
			return nil, nil, geom.ErrInvalid
		}
		local = cloneOvalFile(f)
		for _, pid := range ids {
			p, e := ovalRecord(f, pid, "CARTESIAN_POINT", 2)
			if e != nil || p.Args[1].Kind != Aggregate || len(p.Args[1].Items) != 2 {
				return nil, nil, geom.ErrInvalid
			}
			p.Args[1] = Value{Kind: Aggregate, Items: append(append([]Value(nil), p.Args[1].Items...), Value{Kind: Number, Text: "0."})}
			ovalSet(local, pid, p)
		}
	}
	q, e := DecodeBSplineCurve(local, id, 64)
	if e != nil {
		return nil, nil, fmt.Errorf("decode #%d: %w", id, e)
	}
	c := q.Geometry
	a, b := c.Domain()
	if q.Closed == "T" || q.SelfIntersect == "T" || a != 0 || b != 1 || c.Degree() < 1 || c.Degree() > 16 || len(c.Poles()) > o.MaxPoles {
		return nil, nil, geom.ErrInvalid
	}
	exact, e := geom.NewExactSplineCurve(c, o.patch())
	if e != nil {
		return nil, nil, fmt.Errorf("exact #%d: %w", id, e)
	}
	return c, exact, e
}
func stageRefinedLensCurves(curves [3]*geom.BSpline, unit float64, o CurvedOvalOptions) (*File, [3]int64, error) {
	var ids [3]int64
	for i, c := range curves {
		if c == nil {
			return nil, ids, geom.ErrInvalid
		}
		if i > 0 {
			for _, p := range c.Poles() {
				if p.Z != 0 {
					return nil, ids, geom.ErrInvalid
				}
			}
		}
	}
	var b bytes.Buffer
	m := CurveModel{LengthUnit: unit}
	for _, c := range curves {
		m.Curves = append(m.Curves, SplineCurve{Geometry: c, Closed: "F", SelfIntersect: "F"})
	}
	if e := WriteCurves(&b, m); e != nil {
		return nil, ids, e
	}
	f, e := Parse(&b, Limits{})
	if e != nil {
		return nil, ids, e
	}
	index := 0
	for _, id := range f.Order {
		base, ok := component(f.Entities[id], "B_SPLINE_CURVE")
		if !ok {
			continue
		}
		if index >= 3 {
			return nil, ids, geom.ErrInvalid
		}
		ids[index] = id
		if index > 0 {
			for _, v := range base.Args[1].Items {
				p, e := ovalRecord(f, v.Ref, "CARTESIAN_POINT", 2)
				if e != nil {
					return nil, ids, e
				}
				p.Args[1].Items = p.Args[1].Items[:2]
				ovalSet(f, v.Ref, p)
			}
		}
		if _, _, e = decodeRefinedLensCurve(f, id, index > 0, o); e != nil {
			return nil, ids, e
		}
		index++
	}
	if index != 3 {
		return nil, ids, geom.ErrInvalid
	}
	return f, ids, nil
}
