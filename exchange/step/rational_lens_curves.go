package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/geom"
)

// RationalLensCurve retains represented positive source weights without maximum
// normalization. Its homogeneous products are evaluated exactly, not through a
// rounded BSpline weight conversion. UV poles use Z=0.
type RationalLensCurve struct {
	Degree         int
	Poles          []geom.Vec3
	Knots, Weights []float64
}

// RationalSplineLens retains original support charts and independently refined
// raw-weight model/upper-UV/lower-UV curves for every certified seam piece.
type RationalSplineLens struct {
	PartitionedSplineLens
	Curves [][3]RationalLensCurve
	Style  LensStyle
}

// ReadRationalSplineLensBody proves raw-weight source curves without changing
// their coefficient scale, preserving concurrent support and seam partitions.
func ReadRationalSplineLensBody(r io.Reader, l Limits, o CurvedOvalOptions) (RationalSplineLens, error) {
	f, e := Parse(r, l)
	if e != nil {
		return RationalSplineLens{}, e
	}
	m, e := readSplineLensBody(bytes.NewReader(serializeWrappedProofFile(f)), l, o, true, true, true)
	if e != nil {
		return RationalSplineLens{}, e
	}
	styles, e := readLensStyles(f, map[int64]SplineLensBody{m.SourceBodyID: m}, m.LengthUnit)
	if e != nil {
		return RationalSplineLens{}, e
	}
	return RationalSplineLens{PartitionedSplineLens: PartitionedSplineLens{Body: m, Pieces: m.sourcePieces}, Curves: m.sourceRationalCurves, Style: styles[m.SourceBodyID]}, nil
}

// WriteRationalSplineLensBody re-proves source curves and material before any
// output. Unrepresentable or inconsistent supplied coefficients are rejected.
func WriteRationalSplineLensBody(w io.Writer, m RationalSplineLens, o CurvedOvalOptions) error {
	if len(m.Curves) == 0 {
		return geom.ErrInvalid
	}
	if w == nil {
		return geom.ErrInvalid
	}
	var b bytes.Buffer
	if e := writePartitionedSplineLensBody(&b, m.Body, o, nil, m.Curves); e != nil {
		return e
	}
	f, e := Parse(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		return e
	}
	proved, e := readSplineLensBody(bytes.NewReader(b.Bytes()), Limits{}, o, true, true, true)
	if e != nil {
		return e
	}
	return appendLensStyles(w, b.Bytes(), f, map[int64]SplineLensBody{proved.SourceBodyID: proved}, map[int64]LensStyle{proved.SourceBodyID: m.Style})
}

func exactRationalLensCurve(c RationalLensCurve, o CurvedOvalOptions) (*geom.ExactSplineCurve, error) {
	if c.Degree < 1 || c.Degree > 16 || len(c.Poles) < 2 || len(c.Poles) > 64 || len(c.Poles) > o.MaxPoles || len(c.Weights) != len(c.Poles) || len(c.Knots) != len(c.Poles)+c.Degree+1 {
		return nil, geom.ErrInvalid
	}
	// The ordinary constructor is used only to validate represented pole/knot
	// grammar; its weights are all one, so no source weight normalization occurs.
	template, e := geom.NewBSpline(c.Degree, c.Poles, c.Knots, nil)
	if e != nil {
		return nil, e
	}
	a, b := template.Domain()
	if a != 0 || b != 1 {
		return nil, geom.ErrDomain
	}
	net := make([][4]*big.Rat, len(c.Poles))
	for i, p := range c.Poles {
		w := new(big.Rat).SetFloat64(c.Weights[i])
		if w == nil || w.Sign() <= 0 {
			return nil, geom.ErrInvalid
		}
		net[i] = [4]*big.Rat{new(big.Rat).Mul(new(big.Rat).SetFloat64(p.X), w), new(big.Rat).Mul(new(big.Rat).SetFloat64(p.Y), w), new(big.Rat).Mul(new(big.Rat).SetFloat64(p.Z), w), w}
	}
	return constructRationalLensSpline(net, c.Degree, c.Knots, o)
}
func constructRationalLensSpline(net [][4]*big.Rat, degree int, knots []float64, o CurvedOvalOptions) (*geom.ExactSplineCurve, error) {
	return geom.NewExactHomogeneousSpline(context.Background(), degree, net, knots, o.patch())
}
func decodeRationalLensCurve(f *File, id int64, two bool, o CurvedOvalOptions) (RationalLensCurve, *geom.ExactSplineCurve, error) {
	empty := RationalLensCurve{}
	base, ok := component(f.Entities[id], "B_SPLINE_CURVE")
	if !ok {
		base, ok = component(f.Entities[id], "B_SPLINE_CURVE_WITH_KNOTS")
		if !ok || len(base.Args) < 6 {
			return empty, nil, geom.ErrInvalid
		}
		base.Args = base.Args[1:]
	}
	if len(base.Args) < 5 || base.Args[3].Text == "T" || base.Args[4].Text == "T" {
		return empty, nil, geom.ErrInvalid
	}
	ids, e := listRefs(base.Args[1], 2)
	if e != nil || len(ids) > 64 {
		return empty, nil, geom.ErrInvalid
	}
	local := cloneOvalFile(f)
	for _, pid := range ids {
		p, e := ovalRecord(f, pid, "CARTESIAN_POINT", 2)
		dimension := 3
		if two {
			dimension = 2
		}
		if e != nil || p.Args[1].Kind != Aggregate || len(p.Args[1].Items) != dimension {
			return empty, nil, geom.ErrInvalid
		}
		if two {
			p.Args[1] = Value{Kind: Aggregate, Items: append(append([]Value(nil), p.Args[1].Items...), Value{Kind: Number, Text: "0."})}
		}
		ovalSet(local, pid, p)
	}
	// Replace weights only in the private grammar view; raw records below remain authoritative.
	en := local.Entities[id]
	en.Records = append([]Record(nil), en.Records...)
	weights := make([]float64, len(ids))
	for i := range weights {
		weights[i] = 1
	}
	for i, r := range en.Records {
		if r.Type == "RATIONAL_B_SPLINE_CURVE" {
			if len(r.Args) != 1 || r.Args[0].Kind != Aggregate || len(r.Args[0].Items) != len(ids) {
				return empty, nil, geom.ErrInvalid
			}
			items := make([]Value, len(ids))
			for j, v := range r.Args[0].Items {
				weights[j], e = numeric(v)
				if e != nil || weights[j] <= 0 {
					return empty, nil, geom.ErrInvalid
				}
				items[j] = Value{Kind: Number, Text: "1."}
			}
			r.Args = []Value{{Kind: Aggregate, Items: items}}
			en.Records[i] = r
		}
	}
	local.Entities[id] = en
	q, e := DecodeBSplineCurve(local, id, 64)
	if e != nil {
		return empty, nil, e
	}
	c := RationalLensCurve{q.Geometry.Degree(), q.Geometry.Poles(), q.Geometry.Knots(), weights}
	exact, e := exactRationalLensCurve(c, o)
	return c, exact, e
}
func stageRationalLensCurves(curves [3]RationalLensCurve, unit float64, o CurvedOvalOptions) (*File, [3]int64, error) {
	var ids [3]int64
	var b bytes.Buffer
	m := CurveModel{LengthUnit: unit}
	for role, c := range curves {
		if _, e := exactRationalLensCurve(c, o); e != nil {
			return nil, ids, e
		}
		if role > 0 {
			for _, p := range c.Poles {
				if p.Z != 0 {
					return nil, ids, geom.ErrInvalid
				}
			}
		}
		template, e := geom.NewBSpline(c.Degree, c.Poles, c.Knots, nil)
		if e != nil {
			return nil, ids, e
		}
		m.Curves = append(m.Curves, SplineCurve{Geometry: template, Closed: "F", SelfIntersect: "F"})
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
		en := f.Entities[id]
		en.Records = append([]Record(nil), en.Records...)
		for i, r := range en.Records {
			if r.Type == "RATIONAL_B_SPLINE_CURVE" {
				items := []Value{}
				for _, w := range curves[index].Weights {
					items = append(items, Value{Kind: Number, Text: real(w)})
				}
				r.Args = []Value{{Kind: Aggregate, Items: items}}
				en.Records[i] = r
			}
		}
		f.Entities[id] = en
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
		index++
	}
	if index != 3 {
		return nil, ids, geom.ErrInvalid
	}
	return f, ids, nil
}
